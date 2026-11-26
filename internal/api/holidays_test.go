package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/holidays"
	"github.com/songtianlun/diarum/internal/store"
)

func TestHolidayRoutes(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	authSvc := auth.NewService(s)
	user, err := s.CreateUser("alice", "alice@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := authSvc.IssueToken(user)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2026.json":
			_ = json.NewEncoder(w).Encode(holidays.YearData{
				Year: 2026,
				Days: []holidays.Day{{Name: "元旦", Date: "2026-01-01", IsOffDay: true}},
			})
		case "/2027.json":
			_ = json.NewEncoder(w).Encode(holidays.YearData{
				Year: 2027,
				Days: []holidays.Day{{Name: "元旦", Date: "2027-01-01", IsOffDay: true}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	svc := holidays.NewService(s, holidays.Options{
		Client:  upstream.Client(),
		Sources: []string{upstream.URL + "/%d.json"},
		Now:     func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) },
	})

	e := echo.New()
	RegisterHolidayRoutes(e, nil, authSvc.Middleware)
	RegisterHolidayRoutes(e, svc, authSvc.Middleware)

	doReq := func(method, target string, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, target, &buf)
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	// 功能默认关闭：status 报告 false，数据查询返回空
	rec := doReq(http.MethodGet, "/api/v1/holidays/cn/status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}
	var status struct {
		Enabled bool `json:"enabled"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &status)
	if status.Enabled {
		t.Fatal("expected enabled=false before feature is enabled")
	}

	rec = doReq(http.MethodGet, "/api/v1/holidays/cn?years=2026", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET disabled batch = %d", rec.Code)
	}
	var disabledBatch struct {
		Enabled bool                              `json:"enabled"`
		Years   map[string]*holidays.YearResponse `json:"years"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &disabledBatch)
	if disabledBatch.Enabled || len(disabledBatch.Years) != 0 {
		t.Fatalf("expected empty response when disabled: %+v", disabledBatch)
	}
	if rec := doReq(http.MethodGet, "/api/v1/holidays/cn?year=2026", nil); rec.Code != http.StatusOK {
		t.Fatalf("GET disabled single = %d", rec.Code)
	}

	// 开启功能（同时验证畸形 JSON）
	badReq := httptest.NewRequest(http.MethodPut, "/api/v1/holidays/cn/settings", bytes.NewBufferString("{bad"))
	badReq.Header.Set("Authorization", "Bearer "+token)
	badReq.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	e.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed settings JSON, got %d", badRec.Code)
	}

	rec = doReq(http.MethodPut, "/api/v1/holidays/cn/settings", map[string]any{"enabled": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT settings = %d, body = %s", rec.Code, rec.Body.String())
	}

	// 添加自定义休息日
	rec = doReq(http.MethodPut, "/api/v1/holidays/cn/overrides", map[string]any{
		"date":     "2026-07-01",
		"name":     "公司放假",
		"isOffDay": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT override = %d, body = %s", rec.Code, rec.Body.String())
	}

	// 覆盖参数校验
	if rec := doReq(http.MethodPut, "/api/v1/holidays/cn/overrides", map[string]any{
		"date": "bad", "name": "x", "isOffDay": true,
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad override date, got %d", rec.Code)
	}

	// 单年查询返回数据并包含自定义覆盖
	rec = doReq(http.MethodGet, "/api/v1/holidays/cn?year=2026", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET single year status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var single holidays.YearResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &single); err != nil {
		t.Fatalf("decode single: %v", err)
	}
	if single.Year != 2026 || !single.Published || len(single.Days) == 0 {
		t.Fatalf("unexpected single response: %+v", single)
	}
	foundCustom := false
	for _, d := range single.Days {
		if d.Date == "2026-07-01" && d.Custom && d.IsOffDay {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Fatalf("expected custom override 2026-07-01 in response")
	}

	// 批量年份查询（重复年份去重）
	rec = doReq(http.MethodGet, "/api/v1/holidays/cn?years=2025,2026,2026", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET batch years status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var batch struct {
		Region  string                            `json:"region"`
		Enabled bool                              `json:"enabled"`
		Years   map[string]*holidays.YearResponse `json:"years"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &batch); err != nil {
		t.Fatalf("decode batch: %v", err)
	}
	if !batch.Enabled || len(batch.Years) != 2 || !batch.Years["2025"].Published || !batch.Years["2026"].Published {
		t.Fatalf("unexpected batch response: %+v", batch)
	}

	// 非法查询参数
	if rec := doReq(http.MethodGet, "/api/v1/holidays/cn?year=1800", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid year, got %d", rec.Code)
	}
	if rec := doReq(http.MethodGet, "/api/v1/holidays/cn?years=2025,abc", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid batch year, got %d", rec.Code)
	}
	if rec := doReq(http.MethodGet, "/api/v1/holidays/cn?years=2001,2002,2003,2004,2005,2006,2007,2008,2009,2010,2011,2012,2013,2014,2015,2016,2017,2018,2019,2020,2021", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for >20 years, got %d", rec.Code)
	}
	if rec := doReq(http.MethodGet, "/api/v1/holidays/cn", nil); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for default year, got %d", rec.Code)
	}

	// 管理概览、年度预览、刷新与删除覆盖
	if rec := doReq(http.MethodGet, "/api/v1/holidays/cn/settings", nil); rec.Code != http.StatusOK {
		t.Fatalf("GET settings overview = %d", rec.Code)
	}
	if rec := doReq(http.MethodGet, "/api/v1/holidays/cn/year?year=2026", nil); rec.Code != http.StatusOK {
		t.Fatalf("GET year preview = %d", rec.Code)
	}
	if rec := doReq(http.MethodGet, "/api/v1/holidays/cn/year?year=1800", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid preview year, got %d", rec.Code)
	}

	rec = doReq(http.MethodPost, "/api/v1/holidays/cn/refresh", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST refresh = %d, body = %s", rec.Code, rec.Body.String())
	}
	var refreshed struct {
		LastFetchedAt string         `json:"last_fetched_at"`
		Overrides     []holidays.Day `json:"overrides"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &refreshed)
	if refreshed.LastFetchedAt == "" || len(refreshed.Overrides) != 1 {
		t.Fatalf("unexpected refresh response: %+v", refreshed)
	}

	if rec := doReq(http.MethodDelete, "/api/v1/holidays/cn/overrides/2026-07-01", nil); rec.Code != http.StatusOK {
		t.Fatalf("DELETE override = %d", rec.Code)
	}
	if rec := doReq(http.MethodDelete, "/api/v1/holidays/cn/overrides/2026-07-01", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 deleting missing override, got %d", rec.Code)
	}
	if rec := doReq(http.MethodDelete, "/api/v1/holidays/cn/overrides/bad-date", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 deleting invalid date, got %d", rec.Code)
	}

	// 上游失败时刷新返回 502
	badUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer badUpstream.Close()
	failSvc := holidays.NewService(s, holidays.Options{
		Client:  badUpstream.Client(),
		Sources: []string{badUpstream.URL + "/%d.json"},
		Now:     func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) },
	})
	e2 := echo.New()
	RegisterHolidayRoutes(e2, failSvc, authSvc.Middleware)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/holidays/cn/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	failRec := httptest.NewRecorder()
	e2.ServeHTTP(failRec, req)
	if failRec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on failed refresh, got %d", failRec.Code)
	}

	// 离线 JSON 导入（raw JSON + multipart）
	if rec := doReq(http.MethodPost, "/api/v1/holidays/cn/import", map[string]any{"year": 2030, "days": []any{}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid import payload, got %d", rec.Code)
	}

	var mpBuf bytes.Buffer
	mw := multipart.NewWriter(&mpBuf)
	fw, err := mw.CreateFormFile("file", "2030.json")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write([]byte(`{"year":2030,"papers":["https://www.gov.cn/2030"],"days":[{"name":"元旦","date":"2030-01-01","isOffDay":true}]}`))
	_ = mw.Close()

	mpReq := httptest.NewRequest(http.MethodPost, "/api/v1/holidays/cn/import", &mpBuf)
	mpReq.Header.Set("Authorization", "Bearer "+token)
	mpReq.Header.Set("Content-Type", mw.FormDataContentType())
	mpRec := httptest.NewRecorder()
	e.ServeHTTP(mpRec, mpReq)
	if mpRec.Code != http.StatusOK {
		t.Fatalf("multipart import status = %d, body = %s", mpRec.Code, mpRec.Body.String())
	}
	var importResp struct {
		Imported holidays.YearResponse `json:"imported"`
	}
	if err := json.Unmarshal(mpRec.Body.Bytes(), &importResp); err != nil {
		t.Fatalf("decode importResp: %v", err)
	}
	if importResp.Imported.Year != 2030 || len(importResp.Imported.Days) != 1 {
		t.Fatalf("unexpected importResp: %+v", importResp)
	}
}
