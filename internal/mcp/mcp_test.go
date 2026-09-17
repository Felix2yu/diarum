package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/songtianlun/diarum/internal/chat"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
)

func intPtr(v int) *int           { return &v }
func strPtr(v string) *string     { return &v }
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
	svr := New(s, nil, nil, nil, nil, nil)
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
		"on_this_day", "random_diary", "get_diaries_by_ids",
		"search_diaries", "get_tags", "get_stats", "get_weather",
		"update_diary", "batch_update_diaries", "batch_delete_diaries", "list_diaries",
		"polish_diary", "transcribe_audio", "correct_voice_diary",
		"batch_create_diaries",
		"get_period_analysis", "list_period_analyses", "save_period_analysis", "generate_period_analysis",
		"build_vectors", "vector_stats",
		"has_diary_content", "list_diaries_by_tag", "upsert_diary_weather", "weather_backfill",
		"list_conversations", "get_conversation", "create_conversation", "update_conversation",
		"delete_conversation", "list_conversation_messages", "chat",
		"get_settings", "get_setting", "set_setting", "delete_setting",
		"list_backups", "get_backup", "trigger_backup", "delete_backup",
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
		"period":     "custom",
		"date_start": "2026-02-01",
		"date_end":   "2026-02-15",
		"keywords":   "旅行",
		"summary":    "旅行半月记",
	})
	if res.IsError {
		t.Fatalf("save custom: %s", res.Content)
	}
	res = callTool(t, svr, uid, "get_period_analysis", map[string]any{
		"period":     "custom",
		"date_start": "2026-02-01",
		"date_end":   "2026-02-15",
		"keywords":   "旅行",
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

func TestOnThisDay(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	// Create a diary on a specific date in a previous year
	if _, _, err := s.UpsertDiary(uid, "2024-09-05", "last year today", intPtr(3), nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Today's date should be excluded by GetDiariesByMonthDay
	res := callTool(t, svr, uid, "on_this_day", map[string]any{"date": "2026-09-05"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	var out struct {
		Date    string         `json:"date"`
		Diaries []*store.Diary `json:"diaries"`
		Count   int            `json:"count"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Count != 1 {
		t.Fatalf("count = %d, want 1 (last year's entry)", out.Count)
	}

	// Invalid date format
	if !callTool(t, svr, uid, "on_this_day", map[string]any{"date": "2026/09/05"}).IsError {
		t.Fatal("expected invalid date error")
	}
}

func TestRandomDiary(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	if _, _, err := s.UpsertDiary(uid, "2026-03-01", "hello", intPtr(4), nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	res := callTool(t, svr, uid, "random_diary", nil)
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
}

func TestGetDiariesByIDs(t *testing.T) {
	svr, s, uid, cleanup := newTestServer(t)
	defer cleanup()

	d1, _, _ := s.UpsertDiary(uid, "2026-05-01", "a", intPtr(1), nil, nil, nil, nil, nil, nil, nil)
	d2, _, _ := s.UpsertDiary(uid, "2026-05-02", "b", intPtr(1), nil, nil, nil, nil, nil, nil, nil)
	res := callTool(t, svr, uid, "get_diaries_by_ids", map[string]any{
		"ids": []any{d1.ID, d2.ID, "nonexistent"},
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
		t.Fatalf("count = %d, want 2 (nonexistent filtered out)", out.Count)
	}
}

func TestVectorToolsRequireEmbeddingService(t *testing.T) {
	svr, _, uid, cleanup := newTestServer(t)
	defer cleanup()

	// embeddingService is nil in test server
	res := callTool(t, svr, uid, "vector_stats", nil)
	if !res.IsError {
		t.Fatal("expected error: embedding service not initialized")
	}
	res = callTool(t, svr, uid, "build_vectors", map[string]any{"mode": "incremental"})
	if !res.IsError {
		t.Fatal("expected error: embedding service not initialized")
	}
}

func TestValidatePatch(t *testing.T) {
	tests := []struct {
		name    string
		patch   diaryPatchArgs
		wantErr string
	}{
		{name: "empty patch ok", patch: diaryPatchArgs{}},
		{name: "valid mood", patch: diaryPatchArgs{Mood: intPtr(3)}},
		{name: "mood too low", patch: diaryPatchArgs{Mood: intPtr(0)}, wantErr: "mood must be between 1 and 5"},
		{name: "mood too high", patch: diaryPatchArgs{Mood: intPtr(6)}, wantErr: "mood must be between 1 and 5"},
		{name: "valid content_format text", patch: diaryPatchArgs{ContentFormat: "text"}},
		{name: "valid content_format html", patch: diaryPatchArgs{ContentFormat: "html"}},
		{name: "bad content_format", patch: diaryPatchArgs{ContentFormat: "markdown"}, wantErr: "content_format must be 'text' or 'html'"},
		{name: "valid tags_op merge", patch: diaryPatchArgs{TagsOp: "merge"}},
		{name: "valid tags_op remove", patch: diaryPatchArgs{TagsOp: "remove"}},
		{name: "bad tags_op", patch: diaryPatchArgs{TagsOp: "append"}, wantErr: "tags_op must be 'replace', 'merge' or 'remove'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePatch(tt.patch)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validatePatch() = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("validatePatch() = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestPatchFromArgsUnescapesContent(t *testing.T) {
	content := "第一行\\n第二行"
	patch := patchFromArgs(diaryPatchArgs{Content: &content})
	if patch.Content == nil || *patch.Content != "第一行\n第二行" {
		t.Fatalf("Content = %v, want unescaped newline", patch.Content)
	}
	if patch.TagsOp != "" || patch.ContentFormat != "" {
		t.Fatalf("TagsOp/ContentFormat = %q/%q, want passthrough", patch.TagsOp, patch.ContentFormat)
	}
}

func TestBuildBatchTargets(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()

	tests := []struct {
		name  string
		args  diaryTargetsArgs
		check func(t *testing.T, got store.BatchTargets)
	}{
		{
			name: "empty",
			args: diaryTargetsArgs{},
			check: func(t *testing.T, got store.BatchTargets) {
				if got.IDs != nil || got.DateRange != nil || got.Tag != "" || got.Scenario != "" || got.Query != "" {
					t.Fatalf("got %#v, want zero value", got)
				}
			},
		},
		{
			name: "ids win",
			args: diaryTargetsArgs{IDs: []string{"a", "b"}},
			check: func(t *testing.T, got store.BatchTargets) {
				if len(got.IDs) != 2 || got.IDs[0] != "a" {
					t.Fatalf("IDs = %v", got.IDs)
				}
			},
		},
		{
			name: "date range",
			args: diaryTargetsArgs{DateStart: " 2026-01-01 ", DateEnd: " 2026-01-31 "},
			check: func(t *testing.T, got store.BatchTargets) {
				if got.DateRange == nil || got.DateRange.Start != "2026-01-01" || got.DateRange.End != "2026-01-31" {
					t.Fatalf("DateRange = %+v", got.DateRange)
				}
			},
		},
		{
			name: "date range missing end ignored",
			args: diaryTargetsArgs{DateStart: "2026-01-01"},
			check: func(t *testing.T, got store.BatchTargets) {
				if got.DateRange != nil {
					t.Fatalf("DateRange = %+v, want nil", got.DateRange)
				}
			},
		},
		{
			name: "trim tag/scenario/query",
			args: diaryTargetsArgs{Tag: " 工作 ", Scenario: " 通勤 ", Query: " 会议 "},
			check: func(t *testing.T, got store.BatchTargets) {
				if got.Tag != "工作" || got.Scenario != "通勤" || got.Query != "会议" {
					t.Fatalf("got %q/%q/%q", got.Tag, got.Scenario, got.Query)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, svr.buildBatchTargets(tt.args))
		})
	}
}

func TestPeriodKeyExample(t *testing.T) {
	now := time.Now().UTC()
	y, w := now.ISOWeek()
	tests := []struct {
		period string
		want   string
	}{
		{"week", fmt.Sprintf("%d-W%d", y, w)},
		{"month", now.Format("2006-01")},
		{"year", now.Format("2006")},
		{"custom", ""},
		{"other", ""},
	}
	for _, tt := range tests {
		if got := periodKeyExample(tt.period); got != tt.want {
			t.Errorf("periodKeyExample(%q) = %q, want %q", tt.period, got, tt.want)
		}
	}
}

func TestPeriodKeyLabel(t *testing.T) {
	tests := []struct {
		period, key, want string
	}{
		{"week", "2026-W36", "2026年第36周"},
		{"week", "bad", "bad"},
		{"month", "2026-09", "2026年9月"},
		{"month", "bad", "bad"},
		{"year", "2026", "2026年"},
		{"year", "bad", "bad"},
		{"custom", "anything", "anything"},
	}
	for _, tt := range tests {
		if got := periodKeyLabel(tt.period, tt.key); got != tt.want {
			t.Errorf("periodKeyLabel(%q, %q) = %q, want %q", tt.period, tt.key, got, tt.want)
		}
	}
}

func TestIsoWeekStart(t *testing.T) {
	tests := []struct {
		year, week int
		want       string
	}{
		{2026, 1, "2025-12-29"},
		{2026, 36, "2026-08-31"},
		{2024, 1, "2024-01-01"},
	}
	for _, tt := range tests {
		got := isoWeekStart(tt.year, tt.week).Format("2006-01-02")
		if got != tt.want {
			t.Errorf("isoWeekStart(%d, %d) = %s, want %s", tt.year, tt.week, got, tt.want)
		}
	}
}

func TestDerivePeriodRange(t *testing.T) {
	tests := []struct {
		name       string
		period     string
		key        string
		start, end string
		wantErr    string
	}{
		{name: "week", period: "week", key: "2026-W36", start: "2026-08-31", end: "2026-09-06"},
		{name: "week 1", period: "week", key: "2024-W1", start: "2024-01-01", end: "2024-01-07"},
		{name: "bad week", period: "week", key: "2026-W54", wantErr: "invalid week period_key"},
		{name: "week parse error", period: "week", key: "nope", wantErr: "invalid week period_key"},
		{name: "month", period: "month", key: "2026-09", start: "2026-09-01", end: "2026-09-30"},
		{name: "leap february", period: "month", key: "2024-02", start: "2024-02-01", end: "2024-02-29"},
		{name: "bad month", period: "month", key: "2026-13", wantErr: "invalid month period_key"},
		{name: "month parse error", period: "month", key: "x", wantErr: "invalid month period_key"},
		{name: "year", period: "year", key: "2026", start: "2026-01-01", end: "2026-12-31"},
		{name: "bad year", period: "year", key: "abc", wantErr: "invalid year period_key"},
		{name: "custom unsupported", period: "custom", key: "", wantErr: "custom period requires explicit date_start/date_end"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := derivePeriodRange(tt.period, tt.key)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want contains %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("derivePeriodRange: %v", err)
			}
			if start != tt.start || end != tt.end {
				t.Fatalf("got %s..%s, want %s..%s", start, end, tt.start, tt.end)
			}
		})
	}
}

func TestResolvePeriodRange(t *testing.T) {
	tests := []struct {
		name                        string
		period, key, start, end     string
		wantStart, wantEnd, wantErr string
	}{
		{name: "invalid period", period: "decade", wantErr: "period must be one of"},
		{name: "custom missing dates", period: "custom", wantErr: "custom period requires date_start and date_end"},
		{name: "custom bad start", period: "custom", start: "2026/01/01", end: "2026-01-31", wantErr: "date_start must be YYYY-MM-DD"},
		{name: "custom bad end", period: "custom", start: "2026-01-01", end: "2026/01/31", wantErr: "date_end must be YYYY-MM-DD"},
		{name: "custom explicit dates", period: "custom", start: "2026-01-01", end: "2026-01-31", wantStart: "2026-01-01", wantEnd: "2026-01-31"},
		{name: "month from key", period: "month", key: "2026-09", wantStart: "2026-09-01", wantEnd: "2026-09-30"},
		{name: "month missing key", period: "month", wantErr: "period_key is required"},
		{name: "month override dates", period: "month", key: "2026-09", start: "2026-09-05", end: "2026-09-10", wantStart: "2026-09-05", wantEnd: "2026-09-10"},
		{name: "bad key propagates", period: "week", key: "xx", wantErr: "invalid week period_key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := resolvePeriodRange(tt.period, tt.key, tt.start, tt.end)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want contains %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePeriodRange: %v", err)
			}
			if start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("got %s..%s, want %s..%s", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestBufferWriter(t *testing.T) {
	var w bufferWriter
	n, err := w.Write([]byte("你好"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len("你好") {
		t.Fatalf("n = %d", n)
	}
	w.Write([]byte("world"))
	w.Flush()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if w.buf.String() != "你好world" {
		t.Fatalf("buf = %q", w.buf.String())
	}
}

func TestAIChatConfig(t *testing.T) {
	t.Run("nil config service", func(t *testing.T) {
		svr, _, _, cleanup := newTestServer(t)
		defer cleanup()
		if _, ok := svr.aiChatConfig("user1"); ok {
			t.Fatal("ok = true, want false for nil configService")
		}
	})

	t.Run("resolved values", func(t *testing.T) {
		s, err := store.Open(t.TempDir())
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		defer func() { _ = s.Close() }()
		user, err := s.CreateUser("cfguser", "cfg@example.com", "hash")
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		for key, value := range map[string]any{
			"ai.enabled":    true,
			"ai.api_key":    "sk-mcp",
			"ai.base_url":   "https://ai.example.com",
			"ai.chat_model": "chat-x",
		} {
			if err := s.SetSetting(user.ID, key, value, false); err != nil {
				t.Fatalf("SetSetting %s: %v", key, err)
			}
		}
		svr := New(s, config.NewConfigService(s), nil, nil, nil, nil)
		cfg, ok := svr.aiChatConfig(user.ID)
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if !cfg.Enabled || cfg.APIKey != "sk-mcp" || cfg.BaseURL != "https://ai.example.com" || cfg.Model != "chat-x" {
			t.Fatalf("cfg = %#v", cfg)
		}
	})
}

func TestSpeechConfig(t *testing.T) {
	t.Run("nil config service", func(t *testing.T) {
		svr, _, _, cleanup := newTestServer(t)
		defer cleanup()
		if _, ok := svr.speechConfig("user1"); ok {
			t.Fatal("ok = true, want false for nil configService")
		}
	})

	newServerWithUser := func(t *testing.T) (*Server, *store.Store, string) {
		t.Helper()
		s, err := store.Open(t.TempDir())
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		t.Cleanup(func() { _ = s.Close() })
		user, err := s.CreateUser("speechuser", "speech@example.com", "hash")
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		return New(s, config.NewConfigService(s), nil, nil, nil, nil), s, user.ID
	}

	set := func(t *testing.T, s *store.Store, userID string, values map[string]any) {
		t.Helper()
		for key, value := range values {
			if err := s.SetSetting(userID, key, value, false); err != nil {
				t.Fatalf("SetSetting %s: %v", key, err)
			}
		}
	}

	t.Run("provider none", func(t *testing.T) {
		svr, s, userID := newServerWithUser(t)
		set(t, s, userID, map[string]any{"ai.speech.provider": "none"})
		if _, ok := svr.speechConfig(userID); ok {
			t.Fatal("ok = true, want false when provider is none")
		}
	})

	t.Run("provider missing", func(t *testing.T) {
		svr, _, userID := newServerWithUser(t)
		if _, ok := svr.speechConfig(userID); ok {
			t.Fatal("ok = true, want false when provider unset")
		}
	})

	t.Run("dedicated credentials", func(t *testing.T) {
		svr, s, userID := newServerWithUser(t)
		set(t, s, userID, map[string]any{
			"ai.speech.provider": "openai",
			"ai.speech.base_url": "https://speech.example.com",
			"ai.speech.api_key":  "sk-speech",
			"ai.speech.model":    "whisper-2",
		})
		cfg, ok := svr.speechConfig(userID)
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if !cfg.Enabled || cfg.BaseURL != "https://speech.example.com" || cfg.APIKey != "sk-speech" || cfg.Model != "whisper-2" {
			t.Fatalf("cfg = %#v", cfg)
		}
	})

	t.Run("fallback to shared ai credentials", func(t *testing.T) {
		svr, s, userID := newServerWithUser(t)
		set(t, s, userID, map[string]any{
			"ai.speech.provider": "openai",
			"ai.base_url":        "https://shared.example.com",
			"ai.api_key":         "sk-shared",
		})
		cfg, ok := svr.speechConfig(userID)
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if cfg.BaseURL != "https://shared.example.com" || cfg.APIKey != "sk-shared" {
			t.Fatalf("cfg = %#v", cfg)
		}
	})
}

func callPrompt(t *testing.T, svr *Server, name string, args map[string]string) *mcp.GetPromptResult {
	t.Helper()
	sp := svr.mcpServer.ListPrompts()[name]
	if sp == nil {
		t.Fatalf("prompt %q not registered", name)
	}
	req := mcp.GetPromptRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := sp.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("prompt %q handler error: %v", name, err)
	}
	return res
}

func promptText(t *testing.T, res *mcp.GetPromptResult) string {
	t.Helper()
	if len(res.Messages) == 0 {
		t.Fatal("prompt has no messages")
	}
	content, ok := res.Messages[0].Content.(mcp.TextContent)
	if !ok {
		t.Fatalf("message content type = %T", res.Messages[0].Content)
	}
	return content.Text
}

func TestRegisterPrompts(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()

	t.Run("write_diary with topic and city", func(t *testing.T) {
		res := callPrompt(t, svr, "write_diary", map[string]string{
			"topic": "测试主题", "date": "2026-09-17", "city": "北京",
		})
		text := promptText(t, res)
		for _, want := range []string{"2026-09-17", "测试主题", "city=北京", "create_diary"} {
			if !strings.Contains(text, want) {
				t.Errorf("prompt text missing %q: %s", want, text)
			}
		}
	})

	t.Run("write_diary defaults date and no topic", func(t *testing.T) {
		res := callPrompt(t, svr, "write_diary", nil)
		text := promptText(t, res)
		wantDate := time.Now().Format("2006-01-02")
		if !strings.Contains(text, wantDate) {
			t.Errorf("prompt text missing default date %q", wantDate)
		}
		if !strings.Contains(text, "自行构思") {
			t.Errorf("prompt text missing no-topic guidance")
		}
	})

	t.Run("analyze_period", func(t *testing.T) {
		res := callPrompt(t, svr, "analyze_period", map[string]string{
			"period": "month", "period_key": "2026-09", "keywords": "工作,运动",
		})
		text := promptText(t, res)
		for _, want := range []string{"month", "period_key=2026-09", "工作,运动", "generate_period_analysis"} {
			if !strings.Contains(text, want) {
				t.Errorf("prompt text missing %q", want)
			}
		}
	})

	t.Run("chat_about_diary new and existing conversation", func(t *testing.T) {
		res := callPrompt(t, svr, "chat_about_diary", map[string]string{"question": "最近心情如何"})
		if !strings.Contains(promptText(t, res), "最近心情如何") {
			t.Error("missing question")
		}
		if !strings.Contains(promptText(t, res), "开启一个新对话") {
			t.Error("missing new-conversation hint")
		}
		res = callPrompt(t, svr, "chat_about_diary", map[string]string{"question": "q", "conversation_id": "conv123"})
		if !strings.Contains(promptText(t, res), "conversation_id=conv123") {
			t.Error("missing conversation_id")
		}
	})

	t.Run("mood_review default days", func(t *testing.T) {
		res := callPrompt(t, svr, "mood_review", nil)
		if !strings.Contains(promptText(t, res), "30 天") {
			t.Error("missing default 30 days")
		}
		res = callPrompt(t, svr, "mood_review", map[string]string{"days": "7"})
		if !strings.Contains(promptText(t, res), "7 天") {
			t.Error("missing custom 7 days")
		}
	})

	t.Run("today_summary default date", func(t *testing.T) {
		res := callPrompt(t, svr, "today_summary", nil)
		if !strings.Contains(promptText(t, res), time.Now().Format("2006-01-02")) {
			t.Error("missing default today date")
		}
		res = callPrompt(t, svr, "today_summary", map[string]string{"date": "2026-01-01"})
		if !strings.Contains(promptText(t, res), "2026-01-01") {
			t.Error("missing custom date")
		}
	})
}

func TestSettingsTools(t *testing.T) {
	newServerWithUser := func(t *testing.T) (*Server, *store.Store, string) {
		t.Helper()
		s, err := store.Open(t.TempDir())
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		t.Cleanup(func() { _ = s.Close() })
		user, err := s.CreateUser("setuser", "set@example.com", "hash")
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		return New(s, config.NewConfigService(s), nil, nil, nil, nil), s, user.ID
	}

	t.Run("config service not initialized", func(t *testing.T) {
		svr, _, _, cleanup := newTestServer(t)
		defer cleanup()
		for _, name := range []string{"get_settings", "get_setting", "set_setting", "delete_setting"} {
			res := callTool(t, svr, "user1", name, map[string]any{"key": "k", "value": "v"})
			if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Config service not initialized") {
				t.Errorf("%s: res = %v, want config error", name, res)
			}
		}
	})

	t.Run("auth required", func(t *testing.T) {
		svr, _, _, cleanup := newTestServer(t)
		defer cleanup()
		for _, name := range []string{"get_settings", "get_setting", "set_setting", "delete_setting"} {
			res := callTool(t, svr, "", name, nil)
			if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Authentication required") {
				t.Errorf("%s: res = %v, want auth error", name, res)
			}
		}
	})

	t.Run("get_settings prefix filter", func(t *testing.T) {
		svr, s, userID := newServerWithUser(t)
		for _, kv := range [][2]string{{"ai.api_key", "sk-1"}, {"weather.default_city", "北京"}, {"backup.enabled", "true"}} {
			if err := s.SetSetting(userID, kv[0], kv[1], false); err != nil {
				t.Fatalf("SetSetting: %v", err)
			}
		}
		res := callTool(t, svr, userID, "get_settings", map[string]any{"prefix": "weather"})
		if res.IsError {
			t.Fatalf("get_settings error: %v", res.Content[0].(mcp.TextContent).Text)
		}
		var out struct {
			Settings map[string]string `json:"settings"`
			Count    int               `json:"count"`
		}
		if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.Count != 1 || out.Settings["weather.default_city"] != "北京" {
			t.Fatalf("out = %#v", out)
		}
	})

	t.Run("get_setting missing key error", func(t *testing.T) {
		svr, _, userID := newServerWithUser(t)
		res := callTool(t, svr, userID, "get_setting", nil)
		if !res.IsError {
			t.Fatal("want error for empty key")
		}
	})

	t.Run("set and get roundtrip", func(t *testing.T) {
		svr, _, userID := newServerWithUser(t)
		res := callTool(t, svr, userID, "set_setting", map[string]any{"key": "ai.chat_model", "value": "gpt-x"})
		if res.IsError {
			t.Fatalf("set_setting error: %v", res.Content[0].(mcp.TextContent).Text)
		}
		res = callTool(t, svr, userID, "get_setting", map[string]any{"key": "ai.chat_model"})
		var out struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.Key != "ai.chat_model" || out.Value != "gpt-x" {
			t.Fatalf("out = %#v", out)
		}
	})

	t.Run("set_setting empty key", func(t *testing.T) {
		svr, _, userID := newServerWithUser(t)
		res := callTool(t, svr, userID, "set_setting", map[string]any{"key": " ", "value": "v"})
		if !res.IsError {
			t.Fatal("want error for blank key")
		}
	})

	t.Run("delete_setting roundtrip", func(t *testing.T) {
		svr, s, userID := newServerWithUser(t)
		if err := s.SetSetting(userID, "tmp.key", "v", false); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
		res := callTool(t, svr, userID, "delete_setting", map[string]any{"key": "tmp.key"})
		if res.IsError {
			t.Fatalf("delete_setting error: %v", res.Content[0].(mcp.TextContent).Text)
		}
		res = callTool(t, svr, userID, "delete_setting", map[string]any{"key": "no-such-key"})
		if res.IsError {
			t.Fatal("delete of missing key should succeed silently")
		}
	})
}

func TestChatToolsWithoutServiceAndAuth(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()

	for _, name := range []string{"list_conversations", "get_conversation", "create_conversation", "update_conversation", "delete_conversation"} {
		res := callTool(t, svr, "", name, nil)
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Authentication required") {
			t.Errorf("%s: want auth error, got %v", name, res)
		}
	}
}

func TestChatConversationToolsRoundtrip(t *testing.T) {
	svr, _, userID, cleanup := newTestServer(t)
	defer cleanup()
	svr.chatService = chat.NewChatService(svr.store, nil)

	convID := ""
	t.Run("create", func(t *testing.T) {
		res := callTool(t, svr, userID, "create_conversation", map[string]any{"title": "测试会话"})
		if res.IsError {
			t.Fatalf("create_conversation error: %v", res.Content[0].(mcp.TextContent).Text)
		}
		var c store.Conversation
		if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &c); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if c.ID == "" {
			t.Fatal("empty conversation id")
		}
		convID = c.ID
	})

	t.Run("list", func(t *testing.T) {
		res := callTool(t, svr, userID, "list_conversations", map[string]any{"limit": 5})
		if res.IsError {
			t.Fatalf("list_conversations error: %v", res.Content[0].(mcp.TextContent).Text)
		}
		if !strings.Contains(res.Content[0].(mcp.TextContent).Text, "测试会话") {
			t.Fatal("created conversation not listed")
		}
	})

	t.Run("get", func(t *testing.T) {
		res := callTool(t, svr, userID, "get_conversation", map[string]any{"id": convID})
		if res.IsError {
			t.Fatalf("get_conversation error: %v", res.Content[0].(mcp.TextContent).Text)
		}
	})

	t.Run("get missing", func(t *testing.T) {
		res := callTool(t, svr, userID, "get_conversation", map[string]any{"id": "no-such"})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Conversation not found") {
			t.Fatalf("want not-found error, got %v", res)
		}
	})

	t.Run("rename", func(t *testing.T) {
		res := callTool(t, svr, userID, "update_conversation", map[string]any{"id": convID, "title": "改名"})
		if res.IsError {
			t.Fatalf("update_conversation error: %v", res.Content[0].(mcp.TextContent).Text)
		}
	})

	t.Run("rename empty title", func(t *testing.T) {
		res := callTool(t, svr, userID, "update_conversation", map[string]any{"id": convID, "title": " "})
		if !res.IsError {
			t.Fatal("want error for empty title")
		}
	})

	t.Run("delete", func(t *testing.T) {
		res := callTool(t, svr, userID, "delete_conversation", map[string]any{"id": convID})
		if res.IsError {
			t.Fatalf("delete_conversation error: %v", res.Content[0].(mcp.TextContent).Text)
		}
	})
}

func TestBackupToolsAuthAndNotFound(t *testing.T) {
	svr, _, _, cleanup := newTestServer(t)
	defer cleanup()

	for _, name := range []string{"list_backups", "get_backup", "trigger_backup", "delete_backup"} {
		res := callTool(t, svr, "", name, nil)
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Authentication required") {
			t.Errorf("%s: want auth error, got %v", name, res)
		}
	}

	res := callTool(t, svr, "user1", "trigger_backup", nil)
	if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Backup scheduler not initialized") {
		t.Fatalf("trigger_backup: want scheduler error, got %v", res)
	}

	res = callTool(t, svr, "user1", "get_backup", map[string]any{"id": "no-such"})
	if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Backup not found") {
		t.Fatalf("get_backup: want not-found error, got %v", res)
	}

	res = callTool(t, svr, "user1", "delete_backup", map[string]any{"id": "no-such"})
	if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Backup not found") {
		t.Fatalf("delete_backup: want not-found error, got %v", res)
	}
}

func TestBackupListAndDelete(t *testing.T) {
	svr, _, userID, cleanup := newTestServer(t)
	defer cleanup()

	if _, err := svr.store.CreateBackup(userID, "bak.zip", "", 100, ""); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	res := callTool(t, svr, userID, "list_backups", map[string]any{"limit": 10})
	if res.IsError {
		t.Fatalf("list_backups error: %v", res.Content[0].(mcp.TextContent).Text)
	}
	var out struct {
		Backups []map[string]any `json:"backups"`
		Total   int              `json:"total"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Total != 1 || len(out.Backups) != 1 {
		t.Fatalf("out = %#v", out)
	}
	id, _ := out.Backups[0]["id"].(string)

	res = callTool(t, svr, userID, "get_backup", map[string]any{"id": id})
	if res.IsError {
		t.Fatalf("get_backup error: %v", res.Content[0].(mcp.TextContent).Text)
	}

	res = callTool(t, svr, userID, "delete_backup", map[string]any{"id": id})
	if res.IsError {
		t.Fatalf("delete_backup error: %v", res.Content[0].(mcp.TextContent).Text)
	}

	res = callTool(t, svr, userID, "list_backups", nil)
	if !strings.Contains(res.Content[0].(mcp.TextContent).Text, `"count":0`) {
		t.Fatalf("backups not deleted: %s", res.Content[0].(mcp.TextContent).Text)
	}
}

func TestUpsertDiaryWeatherTool(t *testing.T) {
	svr, _, userID, cleanup := newTestServer(t)
	defer cleanup()

	t.Run("auth required", func(t *testing.T) {
		res := callTool(t, svr, "", "upsert_diary_weather", nil)
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Authentication required") {
			t.Fatalf("want auth error, got %v", res)
		}
	})

	t.Run("missing args", func(t *testing.T) {
		res := callTool(t, svr, userID, "upsert_diary_weather", map[string]any{"date": "2026-09-17"})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "'date' and 'city' are required") {
			t.Fatalf("want missing-args error, got %v", res)
		}
	})

	t.Run("bad date", func(t *testing.T) {
		res := callTool(t, svr, userID, "upsert_diary_weather", map[string]any{"date": "20260917", "city": "北京"})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "'date' must be YYYY-MM-DD") {
			t.Fatalf("want date error, got %v", res)
		}
	})

	t.Run("upsert ok and changed hook", func(t *testing.T) {
		changed := make(chan string, 1)
		s, err := store.Open(t.TempDir())
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		defer func() { _ = s.Close() }()
		user, err := s.CreateUser("wxuser", "wx@example.com", "hash")
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		svr2 := New(s, nil, func(uid string) { changed <- uid }, nil, nil, nil)
		res := callTool(t, svr2, user.ID, "upsert_diary_weather", map[string]any{
			"date": "2026-09-17", "city": "北京", "weather": "晴", "temp_min": 15.0, "temp_max": 28.0,
		})
		if res.IsError {
			t.Fatalf("upsert_diary_weather error: %v", res.Content[0].(mcp.TextContent).Text)
		}
		if !strings.Contains(res.Content[0].(mcp.TextContent).Text, `"status":"ok"`) {
			t.Fatalf("unexpected output: %s", res.Content[0].(mcp.TextContent).Text)
		}
		select {
		case uid := <-changed:
			if uid != user.ID {
				t.Fatalf("changed uid = %q", uid)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("onDiaryChanged not called")
		}
	})
}

func TestPolishDiaryTool(t *testing.T) {
	t.Run("auth required", func(t *testing.T) {
		svr, _, _, cleanup := newTestServer(t)
		defer cleanup()
		res := callTool(t, svr, "", "polish_diary", nil)
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Authentication required") {
			t.Fatalf("want auth error, got %v", res)
		}
	})

	t.Run("missing content", func(t *testing.T) {
		svr, _, userID, cleanup := newTestServer(t)
		defer cleanup()
		res := callTool(t, svr, userID, "polish_diary", map[string]any{"content": "  "})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "'content' is required") {
			t.Fatalf("want content error, got %v", res)
		}
	})

	t.Run("apply without target", func(t *testing.T) {
		svr, _, userID, cleanup := newTestServer(t)
		defer cleanup()
		res := callTool(t, svr, userID, "polish_diary", map[string]any{"content": "文本", "apply": true})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "target_diary_id") {
			t.Fatalf("want target error, got %v", res)
		}
	})

	t.Run("ai not configured", func(t *testing.T) {
		svr, _, userID, cleanup := newTestServer(t)
		defer cleanup()
		res := callTool(t, svr, userID, "polish_diary", map[string]any{"content": "文本"})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "AI service is not configured") {
			t.Fatalf("want ai config error, got %v", res)
		}
	})

	t.Run("bad mode", func(t *testing.T) {
		s, err := store.Open(t.TempDir())
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		defer func() { _ = s.Close() }()
		user, err := s.CreateUser("poluser", "pol@example.com", "hash")
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		set := map[string]any{"ai.enabled": true, "ai.api_key": "sk", "ai.base_url": "http://127.0.0.1:1", "ai.chat_model": "m"}
		for k, v := range set {
			if err := s.SetSetting(user.ID, k, v, false); err != nil {
				t.Fatalf("SetSetting: %v", err)
			}
		}
		svr := New(s, config.NewConfigService(s), nil, nil, nil, nil)
		res := callTool(t, svr, user.ID, "polish_diary", map[string]any{"content": "文本", "mode": "extreme"})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "unsupported polish mode") {
			t.Fatalf("want mode error, got %v", res)
		}
	})
}

func TestTranscribeAudioTool(t *testing.T) {
	svr, _, userID, cleanup := newTestServer(t)
	defer cleanup()

	t.Run("auth required", func(t *testing.T) {
		res := callTool(t, svr, "", "transcribe_audio", nil)
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Authentication required") {
			t.Fatalf("want auth error, got %v", res)
		}
	})

	t.Run("missing audio", func(t *testing.T) {
		res := callTool(t, svr, userID, "transcribe_audio", map[string]any{"audio_base64": " "})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "'audio_base64' is required") {
			t.Fatalf("want audio error, got %v", res)
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		res := callTool(t, svr, userID, "transcribe_audio", map[string]any{"audio_base64": "!!!"})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "invalid base64") {
			t.Fatalf("want base64 error, got %v", res)
		}
	})

	t.Run("speech not configured", func(t *testing.T) {
		res := callTool(t, svr, userID, "transcribe_audio", map[string]any{"audio_base64": "aGk="})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Speech recognition is not configured") {
			t.Fatalf("want speech config error, got %v", res)
		}
	})
}

func TestCorrectVoiceDiaryTool(t *testing.T) {
	svr, _, userID, cleanup := newTestServer(t)
	defer cleanup()

	t.Run("auth required", func(t *testing.T) {
		res := callTool(t, svr, "", "correct_voice_diary", nil)
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "Authentication required") {
			t.Fatalf("want auth error, got %v", res)
		}
	})

	t.Run("no input", func(t *testing.T) {
		res := callTool(t, svr, userID, "correct_voice_diary", nil)
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "provide either 'audio_base64' or 'raw_text'") {
			t.Fatalf("want input error, got %v", res)
		}
	})

	t.Run("ai not configured", func(t *testing.T) {
		res := callTool(t, svr, userID, "correct_voice_diary", map[string]any{"raw_text": "呃 今天嗯测试"})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "AI service is not configured") {
			t.Fatalf("want ai config error, got %v", res)
		}
	})

	t.Run("apply unreachable ai fails before target check", func(t *testing.T) {
		s, err := store.Open(t.TempDir())
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		defer func() { _ = s.Close() }()
		user, err := s.CreateUser("cvuser", "cv@example.com", "hash")
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		for k, v := range map[string]any{"ai.enabled": true, "ai.api_key": "sk", "ai.base_url": "http://127.0.0.1:1", "ai.chat_model": "m"} {
			if err := s.SetSetting(user.ID, k, v, false); err != nil {
				t.Fatalf("SetSetting: %v", err)
			}
		}
		svr2 := New(s, config.NewConfigService(s), nil, nil, nil, nil)
		res := callTool(t, svr2, user.ID, "correct_voice_diary", map[string]any{"raw_text": "文本", "apply": true})
		if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "AI request failed") {
			t.Fatalf("want target error, got %v", res)
		}
	})
}
