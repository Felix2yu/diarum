package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

type Diary struct {
	ID         string   `json:"id"`
	Date       string   `json:"date"`
	Content    string   `json:"content"`
	Mood       int      `json:"mood"`
	MoodStates []string `json:"mood_states"`
	Scenarios  []string `json:"scenarios"`
	Weather    string   `json:"weather"`
	Tags       []string `json:"tags"`
	Owner      string   `json:"owner"`
	Created    string   `json:"created"`
	Updated    string   `json:"updated"`
}

type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type Stats struct {
	Total  int `json:"total"`
	Streak int `json:"streak"`
}

type PeriodAnalysis struct {
	ID          string `json:"id"`
	Period      string `json:"period"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	DiaryCount  int    `json:"diary_count"`
	Summary     string `json:"summary"`
	CreatedAt   string `json:"created_at"`
}

type Media struct {
	ID       string   `json:"id"`
	FilePath string   `json:"file_path"`
	Name     string   `json:"name"`
	Alt      string   `json:"alt"`
	Owner    string   `json:"owner"`
	Diaries  []string `json:"diaries"`
	Created  string   `json:"created"`
	Updated  string   `json:"updated"`
}

type SearchResult struct {
	ID       string `json:"id"`
	Date     string `json:"date"`
	Snippet  string `json:"snippet"`
	Mood     int    `json:"mood"`
	Tags     []string `json:"tags"`
}

type CalendarEntry struct {
	Date   string `json:"date"`
	Exists bool   `json:"exists"`
}

func New(baseURL, apiToken string) *Client {
	return &Client{
		baseURL:  baseURL,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Diary CRUD operations

type UpsertDiaryRequest struct {
	Date       string   `json:"date"`
	Content    string   `json:"content,omitempty"`
	Mood       *int     `json:"mood,omitempty"`
	MoodStates []string `json:"mood_states,omitempty"`
	Scenarios  []string `json:"scenarios,omitempty"`
	Weather    string   `json:"weather,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

func (c *Client) UpsertDiary(req UpsertDiaryRequest) (*Diary, bool, error) {
	respBody, err := c.do("POST", "/api/v1/diaries/upsert", req)
	if err != nil {
		return nil, false, err
	}

	var result struct {
		Diary   Diary `json:"diary"`
		Created bool  `json:"created"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, false, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Diary, result.Created, nil
}

func (c *Client) GetDiaryByID(id string) (*Diary, error) {
	respBody, err := c.do("GET", "/api/v1/diaries/"+id, nil)
	if err != nil {
		return nil, err
	}

	var diary Diary
	if err := json.Unmarshal(respBody, &diary); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &diary, nil
}

func (c *Client) GetDiaryByDate(date string) (*Diary, error) {
	respBody, err := c.do("GET", "/api/v1/diaries/by-date/"+date, nil)
	if err != nil {
		return nil, err
	}

	var diary Diary
	if err := json.Unmarshal(respBody, &diary); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &diary, nil
}

func (c *Client) DeleteDiary(id string) error {
	_, err := c.do("DELETE", "/api/v1/diaries/"+id, nil)
	return err
}

func (c *Client) ListRecentDiaries(limit int) ([]Diary, error) {
	if limit <= 0 {
		limit = 10
	}
	path := fmt.Sprintf("/api/v1/diaries/recent?limit=%d", limit)
	respBody, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var diaries []Diary
	if err := json.Unmarshal(respBody, &diaries); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return diaries, nil
}

func (c *Client) GetOnThisDay(date string) ([]Diary, error) {
	path := "/api/v1/diaries/on-this-day"
	if date != "" {
		path += "?date=" + date
	}
	respBody, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var diaries []Diary
	if err := json.Unmarshal(respBody, &diaries); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return diaries, nil
}

// Search and filter operations

func (c *Client) SearchDiaries(query string, scenario string, limit int) ([]SearchResult, error) {
	path := fmt.Sprintf("/api/v1/diaries/search?q=%s", query)
	if scenario != "" {
		path += "&scenario=" + scenario
	}
	if limit > 0 {
		path += fmt.Sprintf("&limit=%d", limit)
	}

	respBody, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return results, nil
}

func (c *Client) FilterDiaries(mood *int, scenario string, limit int) ([]Diary, error) {
	path := "/api/v1/diaries/filter?"
	if mood != nil {
		path += fmt.Sprintf("mood=%d&", *mood)
	}
	if scenario != "" {
		path += "scenario=" + scenario + "&"
	}
	if limit > 0 {
		path += fmt.Sprintf("limit=%d", limit)
	}

	respBody, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var diaries []Diary
	if err := json.Unmarshal(respBody, &diaries); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return diaries, nil
}

func (c *Client) GetTags() ([]TagCount, error) {
	respBody, err := c.do("GET", "/api/v1/diaries/tags", nil)
	if err != nil {
		return nil, err
	}

	var tags []TagCount
	if err := json.Unmarshal(respBody, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return tags, nil
}

func (c *Client) GetDiariesByTag(tag string) ([]Diary, error) {
	respBody, err := c.do("GET", "/api/v1/diaries/by-tag/"+tag, nil)
	if err != nil {
		return nil, err
	}

	var diaries []Diary
	if err := json.Unmarshal(respBody, &diaries); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return diaries, nil
}

// Statistics operations

func (c *Client) GetStats(timezone string) (*Stats, error) {
	path := "/api/v1/diaries/stats"
	if timezone != "" {
		path += "?tz=" + timezone
	}

	respBody, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var stats Stats
	if err := json.Unmarshal(respBody, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &stats, nil
}

func (c *Client) GetCalendarHeatmap(start, end string) ([]CalendarEntry, error) {
	path := fmt.Sprintf("/api/v1/diaries/exists?start=%s&end=%s", start, end)
	respBody, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var entries []CalendarEntry
	if err := json.Unmarshal(respBody, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return entries, nil
}

// AI operations

type AnalyzePeriodRequest struct {
	Period       string   `json:"period"`
	StartDate    string   `json:"start_date"`
	EndDate      string   `json:"end_date"`
	Keywords     []string `json:"keywords,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
}

func (c *Client) AnalyzePeriod(req AnalyzePeriodRequest) (*PeriodAnalysis, error) {
	respBody, err := c.do("POST", "/api/v1/ai/analysis", req)
	if err != nil {
		return nil, err
	}

	var analysis PeriodAnalysis
	if err := json.Unmarshal(respBody, &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &analysis, nil
}

func (c *Client) ListAnalyses(period string) ([]PeriodAnalysis, error) {
	path := "/api/v1/ai/analyses"
	if period != "" {
		path += "?period=" + period
	}

	respBody, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var analyses []PeriodAnalysis
	if err := json.Unmarshal(respBody, &analyses); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return analyses, nil
}

type ChatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id,omitempty"`
}

type ChatResponse struct {
	Reply           string   `json:"reply"`
	ConversationID  string   `json:"conversation_id"`
	ReferencedDiaries []string `json:"referenced_diaries"`
}

func (c *Client) Chat(req ChatRequest) (*ChatResponse, error) {
	respBody, err := c.do("POST", "/api/v1/ai/chat", req)
	if err != nil {
		return nil, err
	}

	var response ChatResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

func (c *Client) BuildEmbeddings() error {
	_, err := c.do("POST", "/api/v1/ai/vectors/build", nil)
	return err
}

type VectorStats struct {
	DiaryCount    int `json:"diary_count"`
	IndexedCount  int `json:"indexed_count"`
	OutdatedCount int `json:"outdated_count"`
	PendingCount  int `json:"pending_count"`
}

func (c *Client) GetVectorStats() (*VectorStats, error) {
	respBody, err := c.do("GET", "/api/v1/ai/vectors/stats", nil)
	if err != nil {
		return nil, err
	}

	var stats VectorStats
	if err := json.Unmarshal(respBody, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &stats, nil
}

// Media operations

func (c *Client) ListMedia(limit, offset int) ([]Media, error) {
	path := "/api/v1/media?"
	if limit > 0 {
		path += fmt.Sprintf("limit=%d&", limit)
	}
	if offset > 0 {
		path += fmt.Sprintf("offset=%d", offset)
	}

	respBody, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var media []Media
	if err := json.Unmarshal(respBody, &media); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return media, nil
}

// Export/Import operations

type ExportRequest struct {
	Format string `json:"format"`
	Start  string `json:"start,omitempty"`
	End    string `json:"end,omitempty"`
}

func (c *Client) ExportDiaries(req ExportRequest) ([]byte, error) {
	return c.do("POST", "/api/v1/export", req)
}
