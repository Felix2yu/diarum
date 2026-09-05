package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/songtianlun/diarum/internal/store"
)

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
func floatPtr(v float64) *float64 { return &v }

func newTestServer(t *testing.T) (*Server, *store.Store, string, func()) {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	user, err := s.CreateUser("mcpuser", "mcp@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	svr := New(s, nil, nil)
	return svr, s, user.ID, func() { _ = s.Close() }
}

func callTool(t *testing.T, svr *Server, userID, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	tool := svr.mcpServer.GetTool(name)
	if tool == nil {
		t.Fatalf("tool %q not registered", name)
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	ctx := context.Background()
	if userID != "" {
		ctx = context.WithValue(ctx, UserIDKey, userID)
	}
	res, err := tool.Handler(ctx, req)
	if err != nil {
		t.Fatalf("tool %q handler error: %v", name, err)
	}
	return res
}

func TestNewRegistersTools(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()

	expected := []string{
		"create_diary", "get_diary", "delete_diary", "list_recent_diaries",
		"search_diaries", "get_tags", "get_stats", "get_weather",
		"update_diary", "batch_update_diaries", "batch_delete_diaries", "list_diaries",
		"polish_diary", "transcribe_audio", "correct_voice_diary",
		"batch_create_diaries",
		"get_period_analysis", "list_period_analyses", "save_period_analysis",
	}
	for _, name := range expected {
		if svr.mcpServer.GetTool(name) == nil {
			t.Errorf("tool %q should be registered", name)
		}
	}
}

func TestGetStreamableHTTPServer(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()
	h := svr.GetStreamableHTTPServer()
	if h == nil {
		t.Fatal("GetStreamableHTTPServer returned nil")
	}
}

func TestGetUserID(t *testing.T) {
	if got := getUserID(context.Background()); got != "" {
		t.Fatalf("getUserID empty ctx = %q, want empty", got)
	}
	if got := getUserID(context.WithValue(context.Background(), UserIDKey, "u1")); got != "u1" {
		t.Fatalf("getUserID = %q, want u1", got)
	}
}

func TestCreateDiaryAuthRequired(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()

	res := callTool(t, svr, "", "create_diary", map[string]any{
		"date":    "2026-01-01",
		"content": "hello",
	})
	if !res.IsError {
		t.Fatal("expected auth error result")
	}
}

func TestCreateDiarySuccess(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	res := callTool(t, svr, uid, "create_diary", map[string]any{
		"date":        "2026-01-01",
		"content":     "hello",
		"mood":        4,
		"mood_states": []any{"happy"},
		"scenarios":   []any{"work"},
		"weather":     "sunny",
		"tags":        []any{"life"},
		"city":        "Beijing",
		"temp_min":    "10",
		"temp_max":    "20",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}

	diaries, err := s.ListDiaries(uid, "", "", "-date", 10)
	if err != nil {
		t.Fatalf("list diaries: %v", err)
	}
	if len(diaries) != 1 {
		t.Fatalf("expected 1 diary, got %d", len(diaries))
	}
}

func TestCreateDiaryStoreError(t *testing.T) {
	svr, _, uid, cleanup := newTestServer(t)
	defer cleanup()
	// Close the underlying store so UpsertDiary fails.
	cleanup()

	res := callTool(t, svr, uid, "create_diary", map[string]any{
		"date": "2026-01-01",
	})
	if !res.IsError {
		t.Fatal("expected store error result")
	}
}

func TestGetDiaryAuthRequired(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()
	res := callTool(t, svr, "", "get_diary", map[string]any{"date": "2026-01-01"})
	if !res.IsError {
		t.Fatal("expected auth error result")
	}
}

func TestGetDiaryRequiresParams(t *testing.T) {
	svr, _, uid, cleanup := newTestServer(t)
	defer cleanup()
	res := callTool(t, svr, uid, "get_diary", map[string]any{})
	if !res.IsError {
		t.Fatal("expected missing param error result")
	}
}

func TestGetDiaryByDate(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()
	if _, _, err := s.UpsertDiary(uid, "2026-02-02", "hi", intPtr(3), nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	res := callTool(t, svr, uid, "get_diary", map[string]any{"date": "2026-02-02"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
}

func TestGetDiaryByID(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()
	d, _, err := s.UpsertDiary(uid, "2026-02-03", "hi", intPtr(3), nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	res := callTool(t, svr, uid, "get_diary", map[string]any{"id": d.ID})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
}

func TestGetDiaryNotFound(t *testing.T) {
	svr, _, uid, cleanup := newTestServer(t)
	defer cleanup()
	res := callTool(t, svr, uid, "get_diary", map[string]any{"id": "nope"})
	if !res.IsError {
		t.Fatal("expected not found error result")
	}
}

func TestDeleteDiaryAuthRequired(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()
	res := callTool(t, svr, "", "delete_diary", map[string]any{"id": "x"})
	if !res.IsError {
		t.Fatal("expected auth error result")
	}
}

func TestDeleteDiarySuccess(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()
	d, _, err := s.UpsertDiary(uid, "2026-02-04", "hi", intPtr(3), nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	res := callTool(t, svr, uid, "delete_diary", map[string]any{"id": d.ID})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	diaries, _ := s.ListDiaries(uid, "", "", "-date", 10)
	if len(diaries) != 0 {
		t.Fatalf("expected 0 diaries after delete, got %d", len(diaries))
	}
}

func TestListRecentDiaries(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()
	for _, date := range []string{"2026-03-01", "2026-03-02"} {
		if _, _, err := s.UpsertDiary(uid, date, "hi", intPtr(3), nil, nil, nil, nil, nil, nil, nil); err != nil {
			t.Fatalf("upsert %s: %v", date, err)
		}
	}
	res := callTool(t, svr, uid, "list_recent_diaries", map[string]any{})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
}

func TestSearchDiaries(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()
	res := callTool(t, svr, "", "search_diaries", map[string]any{"query": "x"})
	if !res.IsError {
		t.Fatal("expected auth error result")
	}
	res = callTool(t, svr, "mcpuser", "search_diaries", map[string]any{"query": "x"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
}

func TestGetTags(t *testing.T) {
	svr, _, uid, cleanup := newTestServer(t)
	defer cleanup()
	if callTool(t, svr, "", "get_tags", nil).IsError == false {
		t.Fatal("expected auth error result")
	}
	if callTool(t, svr, uid, "get_tags", nil).IsError {
		t.Fatal("unexpected error")
	}
}

func TestGetStats(t *testing.T) {
	svr, _, uid, cleanup := newTestServer(t)
	defer cleanup()
	if callTool(t, svr, "", "get_stats", nil).IsError == false {
		t.Fatal("expected auth error result")
	}
	if callTool(t, svr, uid, "get_stats", nil).IsError {
		t.Fatal("unexpected error")
	}
}

func TestGetWeatherAuthRequired(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()
	res := callTool(t, svr, "", "get_weather", map[string]any{"city": "Beijing"})
	if !res.IsError {
		t.Fatal("expected auth error result")
	}
}

func TestGetWeatherRequiresCity(t *testing.T) {
	svr, _, uid, cleanup := newTestServer(t)
	defer cleanup()
	res := callTool(t, svr, uid, "get_weather", map[string]any{})
	if !res.IsError {
		t.Fatal("expected missing city error result")
	}
}

func TestGetWeatherSuccess(t *testing.T) {
	svr, _, uid, cleanup := newTestServer(t)
	defer cleanup()
	res := callTool(t, svr, uid, "get_weather", map[string]any{"city": "Beijing"})
	// Network may be unavailable in test env; either outcome exercises the handler.
	if len(res.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	if _, err := json.Marshal(res); err != nil {
		t.Fatalf("marshal result: %v", err)
	}
}

func TestCreateDiaryPartialUpdatePreservesFields(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	// Step 1: Create diary with all fields
	_, _, err := s.UpsertDiary(uid, "2025-04-01", "original content",
		intPtr(4), &[]string{"happy"}, &[]string{"work"}, &[]string{"#daily"},
		strPtr("sunny"), strPtr("Shanghai"), floatPtr(10.0), floatPtr(22.0))
	if err != nil {
		t.Fatalf("seed diary: %v", err)
	}

	// Step 2: MCP call with only date + content (no mood/weather/tags)
	res := callTool(t, svr, uid, "create_diary", map[string]any{
		"date":    "2025-04-01",
		"content": "updated via MCP",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}

	// Step 3: Verify via store that all original fields are preserved
	diary, err := s.GetDiaryByDate(uid, "2025-04-01 00:00:00.000Z", "2025-04-01 23:59:59.999Z")
	if err != nil {
		t.Fatalf("GetDiaryByDate: %v", err)
	}
	if diary.Content != "updated via MCP" {
		t.Fatalf("content = %q, want 'updated via MCP'", diary.Content)
	}
	if diary.Mood != 4 {
		t.Fatalf("mood = %d, want 4 (preserved)", diary.Mood)
	}
	if len(diary.MoodStates) != 1 || diary.MoodStates[0] != "happy" {
		t.Fatalf("mood_states = %v, want [happy] (preserved)", diary.MoodStates)
	}
	if len(diary.Scenarios) != 1 || diary.Scenarios[0] != "work" {
		t.Fatalf("scenarios = %v, want [work] (preserved)", diary.Scenarios)
	}
	if len(diary.Tags) != 1 || diary.Tags[0] != "#daily" {
		t.Fatalf("tags = %v, want [#daily] (preserved)", diary.Tags)
	}
	if diary.Weather != "sunny" {
		t.Fatalf("weather = %q, want sunny (preserved)", diary.Weather)
	}
	if diary.City != "Shanghai" {
		t.Fatalf("city = %q, want Shanghai (preserved)", diary.City)
	}
	if diary.TempMin != 10.0 {
		t.Fatalf("temp_min = %f, want 10.0 (preserved)", diary.TempMin)
	}
	if diary.TempMax != 22.0 {
		t.Fatalf("temp_max = %f, want 22.0 (preserved)", diary.TempMax)
	}
}

func TestCreateDiaryExplicitOverwriteClearsFields(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	// Seed with mood=4, weather=sunny
	_, _, err := s.UpsertDiary(uid, "2025-04-02", "seed",
		intPtr(4), nil, nil, nil, strPtr("sunny"), nil, nil, nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// MCP call that explicitly sets mood=1 and weather="" (clear weather)
	res := callTool(t, svr, uid, "create_diary", map[string]any{
		"date":    "2025-04-02",
		"content": "new content",
		"mood":    float64(1),
		"weather": "",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}

	diary, err := s.GetDiaryByDate(uid, "2025-04-02 00:00:00.000Z", "2025-04-02 23:59:59.999Z")
	if err != nil {
		t.Fatalf("GetDiaryByDate: %v", err)
	}
	if diary.Mood != 1 {
		t.Fatalf("mood = %d, want 1 (explicitly overwritten)", diary.Mood)
	}
	if diary.Weather != "" {
		t.Fatalf("weather = %q, want empty (explicitly cleared)", diary.Weather)
	}
	if diary.Content != "new content" {
		t.Fatalf("content = %q, want 'new content'", diary.Content)
	}
}

func TestCreateDiaryMetadataOnlyPreservesContent(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	if _, _, err := s.UpsertDiary(uid, "2025-05-01", "keep me", intPtr(3), nil, nil, nil, strPtr("sunny"), nil, nil, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// MCP create_diary with only date + mood (no content) must NOT wipe content.
	res := callTool(t, svr, uid, "create_diary", map[string]any{
		"date": "2025-05-01",
		"mood": float64(2),
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	diary, err := s.GetDiaryByDate(uid, "2025-05-01 00:00:00.000Z", "2025-05-01 23:59:59.999Z")
	if err != nil {
		t.Fatalf("GetDiaryByDate: %v", err)
	}
	if diary.Content != "keep me" {
		t.Fatalf("content = %q, want 'keep me' (preserved on metadata-only edit)", diary.Content)
	}
	if diary.Mood != 2 {
		t.Fatalf("mood = %d, want 2", diary.Mood)
	}
}

func TestUpdateDiaryByIDPartial(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	d, _, err := s.UpsertDiary(uid, "2025-06-01", "original", intPtr(3), &[]string{"happy"}, &[]string{"work"}, &[]string{"#a"}, strPtr("sunny"), nil, nil, nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Update only mood + merge a tag; content/mood_states/scenarios/weather preserved.
	res := callTool(t, svr, uid, "update_diary", map[string]any{
		"id":      d.ID,
		"mood":    float64(5),
		"tags":    []any{"#b"},
		"tags_op": "merge",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	diary, err := s.GetDiaryByID(d.ID)
	if err != nil {
		t.Fatalf("GetDiaryByID: %v", err)
	}
	if diary.Mood != 5 {
		t.Fatalf("mood = %d, want 5", diary.Mood)
	}
	if diary.Content != "original" {
		t.Fatalf("content = %q, want preserved 'original'", diary.Content)
	}
	if len(diary.Tags) != 2 {
		t.Fatalf("tags = %v, want [#a #b] after merge", diary.Tags)
	}
}

func TestBatchUpdateDryRunDoesNotWrite(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	for _, date := range []string{"2025-07-01", "2025-07-02"} {
		if _, _, err := s.UpsertDiary(uid, date, "x", intPtr(1), nil, nil, &[]string{"#batch"}, nil, nil, nil, nil); err != nil {
			t.Fatalf("seed %s: %v", date, err)
		}
	}
	res := callTool(t, svr, uid, "batch_update_diaries", map[string]any{
		"targets": map[string]any{"tag": "#batch"},
		"patch":   map[string]any{"mood": float64(4)},
		"opts":    map[string]any{"dry_run": true},
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	// Ensure nothing changed.
	diaries, _ := s.ListDiariesByTag(uid, "#batch")
	for _, d := range diaries {
		if d.Mood != 1 {
			t.Fatalf("dry_run wrote mood=%d, want unchanged 1", d.Mood)
		}
	}
}

func TestBatchUpdateByTagApplies(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	for _, date := range []string{"2025-08-01", "2025-08-02"} {
		if _, _, err := s.UpsertDiary(uid, date, "x", intPtr(1), nil, nil, &[]string{"#tag"}, nil, nil, nil, nil); err != nil {
			t.Fatalf("seed %s: %v", date, err)
		}
	}
	res := callTool(t, svr, uid, "batch_update_diaries", map[string]any{
		"targets": map[string]any{"tag": "#tag"},
		"patch":   map[string]any{"mood": float64(5), "tags_op": "replace", "tags": []any{"#tag", "#done"}},
		"opts":    map[string]any{"dry_run": false, "continue_on_error": true},
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	diaries, _ := s.ListDiariesByTag(uid, "#tag")
	if len(diaries) != 2 {
		t.Fatalf("expected 2 diaries, got %d", len(diaries))
	}
	for _, d := range diaries {
		if d.Mood != 5 {
			t.Fatalf("mood = %d, want 5", d.Mood)
		}
	}
}

func TestBatchDeleteDiaries(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	d1, _, _ := s.UpsertDiary(uid, "2025-09-01", "a", intPtr(1), nil, nil, nil, nil, nil, nil, nil)
	d2, _, _ := s.UpsertDiary(uid, "2025-09-02", "b", intPtr(1), nil, nil, nil, nil, nil, nil, nil)
	res := callTool(t, svr, uid, "batch_delete_diaries", map[string]any{
		"ids": []any{d1.ID, d2.ID},
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	diaries, _ := s.ListDiaries(uid, "", "", "-date", 10)
	if len(diaries) != 0 {
		t.Fatalf("expected 0 diaries after batch delete, got %d", len(diaries))
	}
}

func TestListDiariesFiltered(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	for _, date := range []string{"2025-10-01", "2025-10-02", "2025-10-03"} {
		if _, _, err := s.UpsertDiary(uid, date, "x", intPtr(1), nil, nil, &[]string{"#f"}, nil, nil, nil, nil); err != nil {
			t.Fatalf("seed %s: %v", date, err)
		}
	}
	res := callTool(t, svr, uid, "list_diaries", map[string]any{
		"date_start": "2025-10-01",
		"date_end":   "2025-10-02",
		"limit":      float64(50),
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	var out struct {
		Diaries []*store.Diary `json:"diaries"`
		Count   int            `json:"count"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Count != 2 {
		t.Fatalf("count = %d, want 2 (within date range)", out.Count)
	}
}

func TestBatchCreateDiaries(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	res := callTool(t, svr, uid, "batch_create_diaries", map[string]any{
		"items": []any{
			map[string]any{
				"date":    "2026-04-01",
				"content": "plain day",
				"mood":    float64(4),
				"tags":    []any{"#import", "#trip"},
			},
			map[string]any{
				"date":           "2026-04-02",
				"content":        "第一段\n\n第二段",
				"content_format": "html",
				"scenarios":      []any{"travel"},
			},
		},
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	var out struct {
		Created int `json:"created"`
		Failed  int `json:"failed"`
		Total   int `json:"total"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Created != 2 || out.Failed != 0 || out.Total != 2 {
		t.Fatalf("created=%d failed=%d total=%d, want 2/0/2", out.Created, out.Failed, out.Total)
	}

	diaries, err := s.ListDiaries(uid, "2026-04-01 00:00:00.000Z", "2026-04-02 23:59:59.999Z", "date", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(diaries) != 2 {
		t.Fatalf("expected 2 diaries, got %d", len(diaries))
	}
	d1, err := s.GetDiaryByDate(uid, "2026-04-01 00:00:00.000Z", "2026-04-01 23:59:59.999Z")
	if err != nil {
		t.Fatalf("get 04-01: %v", err)
	}
	if d1.Content != "plain day" || d1.Mood != 4 {
		t.Fatalf("first = %+v", d1)
	}
	if len(d1.Tags) != 2 || d1.Tags[0] != "#import" || d1.Tags[1] != "#trip" {
		t.Fatalf("tags = %v, want [#import #trip]", d1.Tags)
	}
	d2, err := s.GetDiaryByDate(uid, "2026-04-02 00:00:00.000Z", "2026-04-02 23:59:59.999Z")
	if err != nil {
		t.Fatalf("get 04-02: %v", err)
	}
	if !strings.Contains(d2.Content, "<p>") {
		t.Fatalf("content_format=html should convert plain text to HTML, got %q", d2.Content)
	}
	if len(d2.Scenarios) != 1 || d2.Scenarios[0] != "travel" {
		t.Fatalf("scenarios = %v, want [travel]", d2.Scenarios)
	}
}

func TestBatchCreateDryRunDoesNotWrite(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	res := callTool(t, svr, uid, "batch_create_diaries", map[string]any{
		"items": []any{
			map[string]any{"date": "2026-04-03", "content": "x"},
			map[string]any{"date": "2026-04-04", "content": "y"},
		},
		"opts": map[string]any{"dry_run": true},
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	diaries, _ := s.ListDiaries(uid, "", "", "-date", 10)
	if len(diaries) != 0 {
		t.Fatalf("dry_run wrote %d diaries, want 0", len(diaries))
	}
}

func TestBatchCreateSkipsExistingDate(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	if _, _, err := s.UpsertDiary(uid, "2026-04-05", "original", intPtr(3), nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// UNIQUE(date, owner) means create-with-skip: same-date items are skipped.
	res := callTool(t, svr, uid, "batch_create_diaries", map[string]any{
		"items": []any{
			map[string]any{"date": "2026-04-05", "content": "imported", "mood": float64(2)},
			map[string]any{"date": "2026-04-06", "content": "new", "mood": float64(4)},
		},
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	var out struct {
		Results []store.BatchResult `json:"results"`
		Created int                 `json:"created"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Created != 1 {
		t.Fatalf("created = %d, want 1", out.Created)
	}
	skipped := 0
	for _, r := range out.Results {
		if r.Status == "skipped" {
			skipped++
		}
	}
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}
	// Existing diary untouched.
	d, err := s.GetDiaryByDate(uid, "2026-04-05 00:00:00.000Z", "2026-04-05 23:59:59.999Z")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if d.Content != "original" || d.Mood != 3 {
		t.Fatalf("existing diary changed: %+v", d)
	}
}

func TestBatchCreateValidationErrors(t *testing.T) {
	svr, _, uid, cleanup := newTestServer(t)
	defer cleanup()

	cases := []struct {
		name string
		args map[string]any
	}{
		{"empty items", map[string]any{"items": []any{}}},
		{"missing date", map[string]any{"items": []any{map[string]any{"content": "x"}}}},
		{"bad date", map[string]any{"items": []any{map[string]any{"date": "2026/04/01"}}}},
		{"bad mood", map[string]any{"items": []any{map[string]any{"date": "2026-04-01", "mood": float64(9)}}}},
		{"bad tags_op", map[string]any{"items": []any{map[string]any{"date": "2026-04-01", "tags_op": "append"}}}},
		{"bad content_format", map[string]any{"items": []any{map[string]any{"date": "2026-04-01", "content_format": "md"}}}},
	}
	for _, tc := range cases {
		res := callTool(t, svr, uid, "batch_create_diaries", tc.args)
		if !res.IsError {
			t.Fatalf("%s: expected error result", tc.name)
		}
	}
}

func TestPeriodAnalysisRoundtrip(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	// Save a month analysis with only period_key (range auto-derived).
	res := callTool(t, svr, uid, "save_period_analysis", map[string]any{
		"period":      "month",
		"period_key":  "2026-01",
		"diary_count": float64(10),
		"summary":     "一月总结",
	})
	if res.IsError {
		t.Fatalf("save: %s", res.Content)
	}

	// Get it back by period_key only.
	res = callTool(t, svr, uid, "get_period_analysis", map[string]any{
		"period":     "month",
		"period_key": "2026-01",
	})
	if res.IsError {
		t.Fatalf("get: %s", res.Content)
	}
	var got store.PeriodAnalysis
	if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.PeriodKey != "2026-01" || got.StartDate != "2026-01-01" || got.EndDate != "2026-01-31" {
		t.Fatalf("range not derived correctly: %+v", got)
	}
	if got.Summary != "一月总结" || got.DiaryCount != 10 {
		t.Fatalf("summary/count mismatch: %+v", got)
	}

	// Save again with the same key: overwrite, not duplicate.
	res = callTool(t, svr, uid, "save_period_analysis", map[string]any{
		"period":      "month",
		"period_key":  "2026-01",
		"diary_count": float64(11),
		"summary":     "一月总结（更新）",
	})
	if res.IsError {
		t.Fatalf("resave: %s", res.Content)
	}
	saved, err := s.ListSavedAnalyses(uid, "month", 100)
	if err != nil {
		t.Fatalf("list saved: %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("expected 1 saved analysis after overwrite, got %d", len(saved))
	}
	if saved[0].Summary != "一月总结（更新）" {
		t.Fatalf("summary = %q, want updated", saved[0].Summary)
	}

	// List via MCP with period filter.
	res = callTool(t, svr, uid, "list_period_analyses", map[string]any{"period": "month"})
	if res.IsError {
		t.Fatalf("list: %s", res.Content)
	}
	var listed struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listed.Count != 1 {
		t.Fatalf("list count = %d, want 1", listed.Count)
	}

	// Custom period roundtrip with keywords.
	res = callTool(t, svr, uid, "save_period_analysis", map[string]any{
		"period":    "custom",
		"date_start": "2026-02-01",
		"date_end":   "2026-02-15",
		"keywords":  "旅行",
		"summary":   "旅行半月记",
	})
	if res.IsError {
		t.Fatalf("save custom: %s", res.Content)
	}
	res = callTool(t, svr, uid, "get_period_analysis", map[string]any{
		"period":    "custom",
		"date_start": "2026-02-01",
		"date_end":   "2026-02-15",
		"keywords":  "旅行",
	})
	if res.IsError {
		t.Fatalf("get custom: %s", res.Content)
	}

	// Error paths: unknown period, missing period_key, not found, invalid week key.
	if !callTool(t, svr, uid, "get_period_analysis", map[string]any{"period": "decade"}).IsError {
		t.Fatal("expected invalid period error")
	}
	if !callTool(t, svr, uid, "save_period_analysis", map[string]any{"period": "month", "summary": "s"}).IsError {
		t.Fatal("expected missing period_key error")
	}
	if !callTool(t, svr, uid, "get_period_analysis", map[string]any{"period": "year", "period_key": "1999"}).IsError {
		t.Fatal("expected not-found error")
	}
	if !callTool(t, svr, uid, "get_period_analysis", map[string]any{"period": "week", "period_key": "2026-W99"}).IsError {
		t.Fatal("expected invalid week key error")
	}
}

func TestPeriodAnalysisAuthRequired(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()
	res := callTool(t, svr, "", "get_period_analysis", map[string]any{
		"period": "month", "period_key": "2026-01",
	})
	if !res.IsError {
		t.Fatal("expected auth error result")
	}
}
