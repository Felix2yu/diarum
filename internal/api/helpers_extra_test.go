package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/store"
)

func TestParseAnalysisMarkdown(t *testing.T) {
	valid := `# Analysis: 2024-01-01 ~ 2024-01-31

**Period:** 2024-01
**Diary Count:** 12
**Keywords:** mood, work
**Created:** 2024-02-01T00:00:00Z

This is the summary text.
It spans multiple lines.
`
	a := parseAnalysisMarkdown([]byte(valid))
	if a == nil {
		t.Fatal("expected non-nil analysis")
	}
	if a.StartDate != "2024-01-01" || a.EndDate != "2024-01-31" {
		t.Errorf("start/end date = %q/%q", a.StartDate, a.EndDate)
	}
	if a.Period != "2024-01" {
		t.Errorf("period = %q", a.Period)
	}
	if a.DiaryCount != 12 {
		t.Errorf("diary count = %d", a.DiaryCount)
	}
	if a.Keywords != "mood, work" {
		t.Errorf("keywords = %q", a.Keywords)
	}
	if a.Created != "2024-02-01T00:00:00Z" {
		t.Errorf("created = %q", a.Created)
	}
	if !strings.Contains(a.Summary, "This is the summary text.") || !strings.Contains(a.Summary, "It spans multiple lines.") {
		t.Errorf("summary = %q", a.Summary)
	}

	// Missing start/end header → returns nil.
	noHeader := `# Analysis

**Period:** 2024-01
`
	if parseAnalysisMarkdown([]byte(noHeader)) != nil {
		t.Error("expected nil when start/end dates missing")
	}

	// Empty input → nil.
	if parseAnalysisMarkdown([]byte("")) != nil {
		t.Error("expected nil for empty input")
	}
}

func TestGenerateAnalysisMarkdownRoundTrip(t *testing.T) {
	in := exportAnalysis{
		Period:     "2024-01",
		StartDate:  "2024-01-01",
		EndDate:    "2024-01-31",
		DiaryCount: 7,
		Keywords:   "happy, calm",
		Created:    "2024-02-01",
		Summary:    "A quiet month.",
	}
	out := generateAnalysisMarkdown(in)
	for _, want := range []string{"# Analysis: 2024-01-01 ~ 2024-01-31", "**Period:** 2024-01", "**Diary Count:** 7", "**Keywords:** happy, calm", "**Created:** 2024-02-01", "A quiet month."} {
		if !strings.Contains(out, want) {
			t.Errorf("generated markdown missing %q\n---got---\n%s", want, out)
		}
	}

	// Round-trip back through the parser.
	back := parseAnalysisMarkdown([]byte(out))
	if back == nil {
		t.Fatal("round-trip parse returned nil")
	}
	if back.StartDate != in.StartDate || back.EndDate != in.EndDate || back.DiaryCount != in.DiaryCount || back.Keywords != in.Keywords {
		t.Errorf("round-trip mismatch: %+v", back)
	}

	// No keywords path.
	noKw := exportAnalysis{StartDate: "2024-03-01", EndDate: "2024-03-31", Period: "2024-03", DiaryCount: 1, Created: "now"}
	if strings.Contains(generateAnalysisMarkdown(noKw), "**Keywords:**") {
		t.Error("expected no Keywords line when empty")
	}
}

func TestParseMarkdownFileChinese(t *testing.T) {
	// Mood + weather + mood states + scenarios parsing (Chinese prefixes).
	rich := parseMarkdownFile("2024-03-01.md", []byte("# 2024-03-01\n**心情：** 😊\n**心情状态：** 开心, 平静\n**情景：** 工作, 家庭\n**天气：** 晴\n\nBody text.\n"))
	if rich == nil {
		t.Fatal("expected non-nil")
	}
	if rich.Mood == 0 {
		t.Error("expected mood to be parsed")
	}
	if rich.Weather != "晴" {
		t.Errorf("weather = %q", rich.Weather)
	}
	if len(rich.MoodStates) != 2 || rich.MoodStates[0] != "开心" {
		t.Errorf("mood states = %v", rich.MoodStates)
	}
	if len(rich.Scenarios) != 2 || rich.Scenarios[0] != "工作" {
		t.Errorf("scenarios = %v", rich.Scenarios)
	}
	if !strings.Contains(rich.Content, "Body text.") {
		t.Errorf("content = %q", rich.Content)
	}

	// English-prefix variants.
	en := parseMarkdownFile("2024-03-02.md", []byte("# 2024-03-02\n**Mood:** happy\n**mood_states:** calm, focused\n**scenarios:** work\n**weather:** rainy\n\nx\n"))
	if en == nil {
		t.Fatal("expected non-nil for english prefixes")
	}
	if en.Weather != "rainy" {
		t.Errorf("weather = %q", en.Weather)
	}
	if len(en.MoodStates) != 2 || en.MoodStates[1] != "focused" {
		t.Errorf("mood states = %v", en.MoodStates)
	}

	// Filename with invalid date prefix → fall through to heading.
	badName := parseMarkdownFile("not-a-date_notes.md", []byte("# 2024-04-09\n\nx\n"))
	if badName == nil || badName.Date != "2024-04-09" {
		t.Errorf("expected heading date after bad filename, got %+v", badName)
	}

	// No date anywhere → nil.
	if parseMarkdownFile("untitled.md", []byte("Just some thoughts.")) != nil {
		t.Error("expected nil when no date found")
	}
}

func TestIsBlankDiaryContent(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"   \n\t  ", true},
		{"<p></p>", true},
		{"<div><br/></div>", true},
		{"&nbsp;", true},
		{"&#160;", true},
		{"<p>hello</p>", false},
		{"just text", false},
		{"<b>bold</b> words", false},
		{"<p>&nbsp;</p>trailing text", false},
	}
	for _, c := range cases {
		if got := isBlankDiaryContent(c.in); got != c.want {
			t.Errorf("isBlankDiaryContent(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDiaryResponse(t *testing.T) {
	d := &store.Diary{
		ID:      "d1",
		Content: "hi",
		Mood:    3,
		Owner:   "u1",
	}
	resp := diaryResponse(d, "2024-01-01", true)
	if resp["id"] != "d1" || resp["date"] != "2024-01-01" || resp["exists"] != true {
		t.Errorf("basic fields wrong: %+v", resp)
	}
	// nil slices should be normalized to empty slices, not nil.
	if resp["tags"] == nil || resp["mood_states"] == nil || resp["scenarios"] == nil {
		t.Errorf("expected non-nil slices, got %+v", resp)
	}
	if _, ok := resp["tags"].([]string); !ok {
		t.Errorf("tags not []string: %T", resp["tags"])
	}

	// With populated slices.
	d2 := &store.Diary{
		ID:         "d2",
		Tags:       []string{"a", "b"},
		MoodStates: []string{"x"},
		Scenarios:  []string{"y"},
	}
	resp2 := diaryResponse(d2, "2024-02-02", false)
	if len(resp2["tags"].([]string)) != 2 {
		t.Errorf("tags not preserved: %+v", resp2["tags"])
	}
	if resp2["weather"] != "" {
		t.Errorf("weather = %v", resp2["weather"])
	}
}

func TestIsValidZipPath(t *testing.T) {
	if !isValidZipPath("diaries/2024-01-01.md") {
		t.Error("expected valid path")
	}
	if isValidZipPath("../escape") {
		t.Error("expected .. to be invalid")
	}
	if isValidZipPath("/abs/path") {
		t.Error("expected absolute path to be invalid")
	}
	if isValidZipPath("\\windows") {
		t.Error("expected backslash prefix to be invalid")
	}
}

func TestGenerateMarkdown(t *testing.T) {
	d := exportDiary{
		Date:       "2024-01-01",
		Content:    "body",
		Mood:       2,
		MoodStates: []string{"calm"},
		Scenarios:  []string{"home"},
		Weather:    "晴",
	}
	out := generateMarkdown(d)
	for _, want := range []string{"# 2024-01-01", "**心情：**", "**心情状态：** calm", "**情景：** home", "**天气：** 晴", "body"} {
		if !strings.Contains(out, want) {
			t.Errorf("generateMarkdown missing %q\n---got---\n%s", want, out)
		}
	}

	// Minimal: no mood/weather → no metadata block.
	min := exportDiary{Date: "2024-05-05", Content: "only body"}
	if strings.Contains(generateMarkdown(min), "**心情：**") {
		t.Error("expected no mood line for zero mood")
	}
	if !strings.Contains(generateMarkdown(min), "only body") {
		t.Error("expected body content")
	}
}

func TestBuildExportZipRoundTrip(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterDiaryRoutes(e, s, authMiddlewareFor(user), nil)

	// Create a diary so export has data.
	body, _ := json.Marshal(map[string]any{
		"date":    "2024-01-01",
		"content": "<p>hello world</p>",
		"mood":    3,
	})
	rec := performRequest(t, e, "POST", "/api/v1/diaries/upsert", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("create diary failed: %d", rec.Code)
	}

	buf, stats, err := BuildExportZip(s, user.ID, ExportRequest{
		DateRange:      "all",
		IncludeDiaries: true,
	})
	if err != nil {
		t.Fatalf("BuildExportZip error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty zip output")
	}
	if stats.Diaries.ActualExported < 1 {
		t.Errorf("expected at least 1 diary in stats, got %+v", stats)
	}
}
