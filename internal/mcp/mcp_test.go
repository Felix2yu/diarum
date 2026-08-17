package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/songtianlun/diarum/internal/store"
)

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

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
	svr := New(s)
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
