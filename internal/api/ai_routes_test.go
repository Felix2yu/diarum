package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/config"
)

// mockAITransport answers the OpenAI-compatible endpoints used by the AI routes:
// /v1/models (model listing) and /v1/chat/completions (chat/polish/analysis).
func mockAITransport(t *testing.T) {
	t.Helper()
	withMockTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/v1/models"):
			return httpResponse(http.StatusOK, `{"data":[{"id":"mock-model-1"},{"id":"mock-model-2"}]}`), nil
		case strings.HasSuffix(req.URL.Path, "/v1/chat/completions"):
			return httpResponse(http.StatusOK, `{"choices":[{"message":{"content":"MOCK_RESULT"}}]}`), nil
		default:
			return httpResponse(http.StatusNotFound, "not found"), nil
		}
	})
}

func TestAIRoutesConversations(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	// embeddingService nil also exercises the "Embedding service not initialized" branches.
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	rec := performRequest(t, e, http.MethodGet, "/api/v1/ai/conversations", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list conversations status = %d body=%s", rec.Code, rec.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("list conversations body is not an array: %v body=%s", err, rec.Body.String())
	}
	if len(list) != 0 {
		t.Fatalf("expected empty conversation list, got %d", len(list))
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/conversations", strings.NewReader(`{"title":"My chat"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("create conversation status = %d body=%s", rec.Code, rec.Body.String())
	}
	convID := decodeJSONBody(t, rec)["id"].(string)

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/conversations/"+convID, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get conversation status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/ai/conversations/"+convID, strings.NewReader(`{"title":"Renamed"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("update conversation status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["title"] != "Renamed" {
		t.Fatalf("update conversation payload = %#v", payload)
	}

	rec = performRequest(t, e, http.MethodDelete, "/api/v1/ai/conversations/"+convID, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete conversation status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/conversations/missing", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET missing conversation status = %d, want 404", rec.Code)
	}
	rec = performRequest(t, e, http.MethodDelete, "/api/v1/ai/conversations/missing", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("DELETE missing conversation status = %d, want 404", rec.Code)
	}
	rec = performRequest(t, e, http.MethodPut, "/api/v1/ai/conversations/missing", strings.NewReader(`{"title":"x"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("PUT missing conversation status = %d, want 404", rec.Code)
	}
}

func TestAIRoutesVectorsNilService(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	for _, path := range []string{
		"/api/v1/ai/vectors/build",
		"/api/v1/ai/vectors/build-incremental",
	} {
		rec := performRequest(t, e, http.MethodPost, path, nil, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("POST %s status = %d, want 400 (embedding service nil)", path, rec.Code)
		}
	}
	rec := performRequest(t, e, http.MethodGet, "/api/v1/ai/vectors/stats", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET vectors/stats status = %d, want 400 (embedding service nil)", rec.Code)
	}
}

func TestAIRoutesSettings(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	cfg := config.NewConfigService(s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	// Pre-set a real API key so the placeholder-keeping branch has something to fall back to.
	if err := cfg.Set(user.ID, "ai.api_key", "real-secret-key"); err != nil {
		t.Fatalf("set ai.api_key: %v", err)
	}

	rec := performRequest(t, e, http.MethodGet, "/api/v1/ai/settings", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET ai/settings status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["api_key"] == nil || payload["enabled"] == nil {
		t.Fatalf("GET ai/settings payload = %#v", payload)
	}

	// Disabled save (no required fields) should succeed.
	rec = performRequest(t, e, http.MethodPut, "/api/v1/ai/settings", strings.NewReader(`{"enabled":false,"base_url":"https://x.test","chat_model":"m","embedding_model":"e"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT ai/settings disabled status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Enabled but missing required fields should be rejected.
	rec = performRequest(t, e, http.MethodPut, "/api/v1/ai/settings", strings.NewReader(`{"enabled":true,"base_url":"https://x.test"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT ai/settings enabled-missing-fields status = %d, want 400", rec.Code)
	}

	// Placeholder secret should keep the existing real key.
	rec = performRequest(t, e, http.MethodPut, "/api/v1/ai/settings", strings.NewReader(`{"enabled":false,"api_key":"********","base_url":"https://x.test","chat_model":"m","embedding_model":"e"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT ai/settings placeholder-secret status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["success"] != true {
		t.Fatalf("PUT ai/settings placeholder-secret payload = %#v", payload)
	}
}

func TestAIRoutesAnalysisGetAndList(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	// Invalid period
	rec := performRequest(t, e, http.MethodGet, "/api/v1/ai/analysis?period=bad&start=2024-01-01&end=2024-01-31", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("analysis GET bad period status = %d, want 400", rec.Code)
	}
	// Missing key
	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analysis?period=week", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("analysis GET missing key status = %d, want 400", rec.Code)
	}
	// No saved analysis -> found:false (sql.ErrNoRows branch)
	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analysis?period=week&key=2024-W01", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("analysis GET found:false status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["found"] != false {
		t.Fatalf("analysis GET found = %#v, want false", payload["found"])
	}

	// List analyses
	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analyses?period=all", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("analyses list status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["items"] == nil {
		t.Fatalf("analyses list payload = %#v", payload)
	}
	// Invalid list period
	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analyses?period=bad", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("analyses list bad period status = %d, want 400", rec.Code)
	}
}

func TestAIRoutesModels(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	// Missing api key / base url
	rec := performRequest(t, e, http.MethodPost, "/api/v1/ai/models", strings.NewReader(`{}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("models missing creds status = %d, want 400", rec.Code)
	}

	mockAITransport(t)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/models", strings.NewReader(`{"api_key":"k","base_url":"https://mock.local"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("models status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); len(payload["models"].([]any)) != 2 {
		t.Fatalf("models payload = %#v", payload)
	}
}

func TestAIRoutesPolish(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	cfg := config.NewConfigService(s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	// Missing content
	rec := performRequest(t, e, http.MethodPost, "/api/v1/ai/polish", strings.NewReader(`{}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("polish missing content status = %d, want 400", rec.Code)
	}
	// Invalid mode
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/polish", strings.NewReader(`{"content":"x","mode":"bad"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("polish bad mode status = %d, want 400", rec.Code)
	}
	// Custom mode missing prompt (AI not configured -> 503, consistent with
	// the analysis route so the frontend can surface an "AI 未配置" notice).
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/polish", strings.NewReader(`{"content":"x","mode":"custom"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("polish custom missing prompt status = %d, want 503", rec.Code)
	}

	// Configure AI and mock chat completions -> success.
	for key, value := range map[string]any{
		"ai.enabled":    true,
		"ai.api_key":    "k",
		"ai.base_url":   "https://mock.local",
		"ai.chat_model": "m",
	} {
		if err := cfg.Set(user.ID, key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	mockAITransport(t)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/polish", strings.NewReader(`{"content":"hello world","mode":"medium"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("polish status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["content"] != "MOCK_RESULT" {
		t.Fatalf("polish payload = %#v", payload)
	}
}

func TestAIRoutesChatValidation(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	// Missing conversation_id / content
	rec := performRequest(t, e, http.MethodPost, "/api/v1/ai/chat", strings.NewReader(`{}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("chat missing fields status = %d, want 400", rec.Code)
	}
	// Conversation not found
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/chat", strings.NewReader(`{"conversation_id":"missing","content":"hi"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("chat missing conversation status = %d, want 404", rec.Code)
	}
}

func TestAIRoutesTranscribeValidation(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	cfg := config.NewConfigService(s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	// Speech provider not configured -> rejected early.
	rec := performRequest(t, e, http.MethodPost, "/api/v1/ai/transcribe", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("transcribe no provider status = %d, want 400", rec.Code)
	}

	// Configure speech (falls back to shared AI base url/key) but post without a file.
	for key, value := range map[string]any{
		"ai.speech.provider": "openai",
		"ai.base_url":        "https://mock.local",
		"ai.api_key":         "k",
	} {
		if err := cfg.Set(user.ID, key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	emptyBody, emptyCT := multipartRequestBody(t, "file", "audio.webm", []byte{}, nil)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/transcribe", emptyBody, map[string]string{"Content-Type": emptyCT})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("transcribe missing file status = %d, want 400 (got body=%s)", rec.Code, rec.Body.String())
	}
}

func TestAIRoutesAnalysisPost(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	cfg := config.NewConfigService(s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	// Validation: bad period
	rec := performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"bad"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("analysis POST bad period status = %d, want 400", rec.Code)
	}
	// Missing start/end
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"week"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("analysis POST missing range status = %d, want 400", rec.Code)
	}
	// Bad date format
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"week","start":"nope","end":"2024-01-31"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("analysis POST bad date status = %d, want 400", rec.Code)
	}

	// Create a diary inside the range, configure AI, mock chat completions -> success.
	if _, _, err := s.UpsertDiary(user.ID, "2024-01-15", "A productive day at work.", intPtr(4), nil, nil, nil, strPtr("sunny"), nil, nil, nil); err != nil {
		t.Fatalf("UpsertDiary: %v", err)
	}
	for key, value := range map[string]any{
		"ai.enabled":    true,
		"ai.api_key":    "k",
		"ai.base_url":   "https://mock.local",
		"ai.chat_model": "m",
	} {
		if err := cfg.Set(user.ID, key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	mockAITransport(t)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"custom","start":"2024-01-01","end":"2024-01-31","keywords":"work"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("analysis POST status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	if payload["summary"] != "MOCK_RESULT" {
		t.Fatalf("analysis POST summary = %#v", payload["summary"])
	}
	if payload["count"] == nil || int(payload["count"].(float64)) < 1 {
		t.Fatalf("analysis POST count = %#v", payload["count"])
	}
}

func TestAIRoutesAnalysisManualSave(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	// Validation: bad period / missing key / bad key / empty summary
	for _, tc := range []struct {
		name string
		body string
	}{
		{"bad period", `{"period":"bad","key":"2026-W02","summary":"x"}`},
		{"missing key", `{"period":"week","summary":"x"}`},
		{"bad key format", `{"period":"week","key":"2026-W2","summary":"x"}`},
		{"out-of-range week", `{"period":"week","key":"2027-W53","summary":"x"}`},
		{"bad month key", `{"period":"month","key":"2026-13","summary":"x"}`},
		{"empty summary", `{"period":"week","key":"2026-W02","summary":"   "}`},
	} {
		rec := performRequest(t, e, http.MethodPut, "/api/v1/ai/analysis", strings.NewReader(tc.body), map[string]string{"Content-Type": "application/json"})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("analysis PUT %s status = %d, want 400 (body=%s)", tc.name, rec.Code, rec.Body.String())
		}
	}

	if _, _, err := s.UpsertDiary(user.ID, "2026-01-07", "周中的一次长跑记录", nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("UpsertDiary: %v", err)
	}

	// 2026-W02: 2026-01-05 ~ 2026-01-11 (ISO 8601). Manual save without AI config.
	body := `{"period":"week","key":"2026-W02","summary":"本周运动三次，睡眠改善。"}`
	rec := performRequest(t, e, http.MethodPut, "/api/v1/ai/analysis", strings.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("analysis PUT status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	if payload["summary"] != "本周运动三次，睡眠改善。" {
		t.Fatalf("analysis PUT summary = %#v", payload["summary"])
	}
	if payload["start"] != "2026-01-05" || payload["end"] != "2026-01-11" {
		t.Fatalf("analysis PUT derived range = %v ~ %v, want 2026-01-05 ~ 2026-01-11", payload["start"], payload["end"])
	}
	if count, ok := payload["count"].(float64); !ok || int(count) != 1 {
		t.Fatalf("analysis PUT count = %#v, want 1", payload["count"])
	}

	// Saved manual report must be retrievable via the GET endpoint by key.
	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analysis?period=week&key=2026-W02", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("analysis GET status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload = decodeJSONBody(t, rec)
	if payload["found"] != true {
		t.Fatalf("analysis GET found = %#v, want true", payload["found"])
	}
	if payload["summary"] != "本周运动三次，睡眠改善。" {
		t.Fatalf("analysis GET summary = %#v", payload["summary"])
	}
	if payload["key"] != "2026-W02" {
		t.Fatalf("analysis GET key = %#v, want 2026-W02", payload["key"])
	}

	// The saved report should appear in the history list for period=week.
	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analyses?period=week", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("analyses GET status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload = decodeJSONBody(t, rec)
	items, _ := payload["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("analyses GET items = %#v, want 1 item", payload["items"])
	}
}

func TestAIRoutesAnalysisPostByKey(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	cfg := config.NewConfigService(s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	// Validation: keyed POST without key / with bad key
	rec := performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"year"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("analysis POST missing key status = %d, want 400", rec.Code)
	}
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"week","key":"2026-W99"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("analysis POST bad key status = %d, want 400", rec.Code)
	}

	// Configure AI + mock transport, then generate a yearly report by key.
	if _, _, err := s.UpsertDiary(user.ID, "2026-03-15", "年度里的一个普通日子", nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("UpsertDiary: %v", err)
	}
	for key, value := range map[string]any{
		"ai.enabled":    true,
		"ai.api_key":    "k",
		"ai.base_url":   "https://mock.local",
		"ai.chat_model": "m",
	} {
		if err := cfg.Set(user.ID, key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	mockAITransport(t)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"year","key":"2026"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("analysis POST status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	if payload["summary"] != "MOCK_RESULT" {
		t.Fatalf("analysis POST summary = %#v", payload["summary"])
	}
	if payload["key"] != "2026" || payload["start"] != "2026-01-01" || payload["end"] != "2026-12-31" {
		t.Fatalf("analysis POST key/range = %v %v %v", payload["key"], payload["start"], payload["end"])
	}
	if count, ok := payload["count"].(float64); !ok || int(count) != 1 {
		t.Fatalf("analysis POST count = %#v, want 1", payload["count"])
	}

	// Retrieved again by key from GET.
	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analysis?period=year&key=2026", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("analysis GET status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload = decodeJSONBody(t, rec); payload["found"] != true || payload["summary"] != "MOCK_RESULT" {
		t.Fatalf("analysis GET payload = %#v", payload)
	}
}
