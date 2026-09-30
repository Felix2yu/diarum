package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/config"
)

// upsert 带 edit_session_id 才记版本；同一会话只记一次；
// 列表/详情/恢复/越权访问/过期清理的完整链路。
func TestDiaryVersionRoutes(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	other := newTestUser(t, s)
	e := echo.New()
	RegisterDiaryRoutes(e, s, authMiddlewareFor(user), nil)

	jsonHeaders := map[string]string{"Content-Type": "application/json"}

	// seed 日记（无会话 → 不产生版本）
	rec := performRequest(t, e, http.MethodPost, "/api/v1/diaries/upsert",
		strings.NewReader(`{"date":"2026-10-01","content":"original","mood":3}`), jsonHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed upsert status = %d body=%s", rec.Code, rec.Body.String())
	}
	diaryID, _ := decodeJSONBody(t, rec)["id"].(string)
	if diaryID == "" {
		t.Fatalf("seed response missing id: %s", rec.Body.String())
	}

	// 编辑会话 1 首次保存 → 快照 "original"
	rec = performRequest(t, e, http.MethodPost, "/api/v1/diaries/upsert",
		strings.NewReader(`{"date":"2026-10-01","content":"edited","edit_session_id":"sess-1"}`), jsonHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit upsert status = %d body=%s", rec.Code, rec.Body.String())
	}
	// 同会话再次保存 → 不新增
	rec = performRequest(t, e, http.MethodPost, "/api/v1/diaries/upsert",
		strings.NewReader(`{"date":"2026-10-01","content":"edited more","edit_session_id":"sess-1"}`), jsonHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("second save status = %d body=%s", rec.Code, rec.Body.String())
	}

	// 列表：1 个版本，摘要不含完整正文字段之外的泄露
	listPath := "/api/v1/diaries/" + diaryID + "/versions"
	rec = performRequest(t, e, http.MethodGet, listPath, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list versions status = %d body=%s", rec.Code, rec.Body.String())
	}
	listBody := decodeJSONBody(t, rec)
	if total, _ := listBody["total"].(float64); total != 1 {
		t.Fatalf("versions total = %v, want 1", listBody["total"])
	}
	versions, _ := listBody["versions"].([]any)
	first, _ := versions[0].(map[string]any)
	versionID, _ := first["id"].(string)
	if versionID == "" {
		t.Fatalf("version list missing id: %#v", listBody)
	}
	if preview, _ := first["preview"].(string); preview != "original" {
		t.Fatalf("version preview = %q, want %q", preview, "original")
	}
	if content, ok := first["content"].(string); ok && content != "" {
		t.Fatalf("version list should not include full content, got %q", content)
	}

	// 详情：完整内容
	rec = performRequest(t, e, http.MethodGet, listPath+"/"+versionID, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("version detail status = %d body=%s", rec.Code, rec.Body.String())
	}
	detail := decodeJSONBody(t, rec)
	if detail["content"] != "original" || detail["diary_id"] != diaryID {
		t.Fatalf("version detail = %#v", detail)
	}

	// 不存在的版本 → 404
	rec = performRequest(t, e, http.MethodGet, listPath+"/missing-version", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing version status = %d, want 404", rec.Code)
	}

	// 恢复 → 内容回写，且恢复前当前内容被快照（版本数变 2）
	rec = performRequest(t, e, http.MethodPost, listPath+"/"+versionID+"/restore", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore status = %d body=%s", rec.Code, rec.Body.String())
	}
	if restored := decodeJSONBody(t, rec)["content"]; restored != "original" {
		t.Fatalf("restored content = %v, want original", restored)
	}
	rec = performRequest(t, e, http.MethodGet, listPath, nil, nil)
	if total, _ := decodeJSONBody(t, rec)["total"].(float64); total != 2 {
		t.Fatalf("versions total after restore = %v, want 2", decodeJSONBody(t, rec)["total"])
	}

	// 越权：其他用户看不到版本，也不能恢复
	eOther := echo.New()
	RegisterDiaryRoutes(eOther, s, authMiddlewareFor(other), nil)
	rec = performRequest(t, eOther, http.MethodGet, listPath, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("other list status = %d", rec.Code)
	}
	if total, _ := decodeJSONBody(t, rec)["total"].(float64); total != 0 {
		t.Fatalf("other versions total = %v, want 0", decodeJSONBody(t, rec)["total"])
	}
	rec = performRequest(t, eOther, http.MethodGet, listPath+"/"+versionID, nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other detail status = %d, want 404", rec.Code)
	}
	rec = performRequest(t, eOther, http.MethodPost, listPath+"/"+versionID+"/restore", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other restore status = %d, want 404", rec.Code)
	}
}

// 列表惰性清理遵循 diary.version_retention_days 配置（默认 30 天）。
func TestDiaryVersionListPurgesExpired(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterDiaryRoutes(e, s, authMiddlewareFor(user), nil)

	if _, _, err := s.UpsertDiary(user.ID, "2026-10-02", "current", nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	diary, err := s.GetDiaryByDate(user.ID, "2026-10-02 00:00:00.000Z", "2026-10-02 23:59:59.999Z")
	if err != nil {
		t.Fatalf("get diary: %v", err)
	}
	// 手工插入 40 天前的过期版本
	expiredCreated := time.Now().UTC().AddDate(0, 0, -40).Format("2006-01-02 15:04:05.000Z")
	if _, err := s.DB.Exec(
		`INSERT INTO diary_versions(id, owner, diary_id, date, edit_session_id, content, mood, mood_states, scenarios, weather, city, temp_min, temp_max, tags, created)
		 VALUES('expired-api', ?, ?, '2026-10-02', 'expired', 'old', 0, '[]', '[]', '', '', 0, 0, '[]', ?)`,
		user.ID, diary.ID, expiredCreated,
	); err != nil {
		t.Fatalf("insert expired: %v", err)
	}

	rec := performRequest(t, e, http.MethodGet, "/api/v1/diaries/"+diary.ID+"/versions", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}
	if total, _ := decodeJSONBody(t, rec)["total"].(float64); total != 0 {
		t.Fatalf("expired version leaked: total = %v", decodeJSONBody(t, rec)["total"])
	}

	// 保留期调大后重新插入的近期版本可见（配置链路生效）
	cfg := config.NewConfigService(s)
	if err := cfg.Set(user.ID, "diary.version_retention_days", 60); err != nil {
		t.Fatalf("set retention: %v", err)
	}
	if _, err := s.DB.Exec(
		`INSERT INTO diary_versions(id, owner, diary_id, date, edit_session_id, content, mood, mood_states, scenarios, weather, city, temp_min, temp_max, tags, created)
		 VALUES('recent-api', ?, ?, '2026-10-02', 'recent', 'newer', 0, '[]', '[]', '', '', 0, 0, '[]', ?)`,
		user.ID, diary.ID, time.Now().UTC().Format("2006-01-02 15:04:05.000Z"),
	); err != nil {
		t.Fatalf("insert recent: %v", err)
	}
	rec = performRequest(t, e, http.MethodGet, "/api/v1/diaries/"+diary.ID+"/versions", nil, nil)
	if total, _ := decodeJSONBody(t, rec)["total"].(float64); total != 1 {
		t.Fatalf("retention 60 days list total = %v, want 1", decodeJSONBody(t, rec)["total"])
	}
}
