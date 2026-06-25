package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/store"
)

// RegisterDiaryRoutes registers custom API endpoints for diary operations.
func RegisterDiaryRoutes(e *echo.Echo, s *store.Store, authMiddleware echo.MiddlewareFunc, onDiaryChanged func(string)) {
	group := e.Group("/api/v1/diaries", authMiddleware)

	group.POST("/upsert", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		var body struct {
			Date       string   `json:"date"`
			Content    string   `json:"content"`
			Mood       int      `json:"mood"`
			MoodStates []string `json:"mood_states"`
			Scenarios  []string `json:"scenarios"`
			Weather    string   `json:"weather"`
			Tags       []string `json:"tags"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		if body.Date == "" {
			return badRequest("date is required", nil)
		}
		if body.Tags == nil {
			body.Tags = []string{}
		}

		diary, _, err := s.UpsertDiary(user.ID, body.Date, body.Content, body.Mood, body.MoodStates, body.Scenarios, body.Weather, body.Tags)
		if err != nil {
			return badRequest("Failed to save diary", err)
		}
		if onDiaryChanged != nil {
			onDiaryChanged(user.ID)
		}
		return c.JSON(http.StatusOK, diaryResponse(diary, body.Date, true))
	})

	group.GET("/by-date/:date", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		dateStr := c.PathParam("date")
		start, end := dateStr+" 00:00:00.000Z", dateStr+" 23:59:59.999Z"
		diary, err := s.GetDiaryByDate(user.ID, start, end)
		if err != nil {
			return c.JSON(http.StatusOK, map[string]any{"date": dateStr, "content": "", "exists": false})
		}
		return c.JSON(http.StatusOK, diaryResponse(diary, dateStr, true))
	})

	// 往年今日: 返回所有相同月-日但年份不同的日记
	group.GET("/on-this-day", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		date := c.QueryParam("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		diaries, err := s.GetDiariesByMonthDay(user.ID, date)
		if err != nil {
			return serverError("Failed to query on-this-day diaries", err)
		}
		result := make([]map[string]any, 0, len(diaries))
		for _, diary := range diaries {
			result = append(result, diaryResponse(diary, store.DateOnly(diary.Date), true))
		}
		return c.JSON(http.StatusOK, map[string]any{"date": date, "total": len(result), "diaries": result})
	})

	// 随机穿越: 从用户有内容的日记中随机挑选一条返回
	group.GET("/random", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		exclude := c.QueryParam("exclude_date")
		diary, err := s.GetRandomDiary(user.ID, exclude)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return c.JSON(http.StatusOK, map[string]any{"exists": false})
			}
			return serverError("Failed to pick a random diary", err)
		}
		return c.JSON(http.StatusOK, diaryResponse(diary, store.DateOnly(diary.Date), true))
	})

	group.GET("/exists", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		start := c.QueryParam("start")
		end := c.QueryParam("end")
		if start == "" || end == "" {
			now := time.Now()
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			end = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		}
		diaries, err := s.ListDiaries(user.ID, start+" 00:00:00.000Z", end+" 23:59:59.999Z", "-date", 0)
		if err != nil {
			return serverError("Failed to query diaries", err)
		}
		dates := make([]string, 0, len(diaries))
		entries := make([]map[string]any, 0, len(diaries))
		for _, diary := range diaries {
			date := store.DateOnly(diary.Date)
			dates = append(dates, date)
			entries = append(entries, map[string]any{"date": date, "mood": diary.Mood, "weather": diary.Weather})
		}
		return c.JSON(http.StatusOK, map[string]any{"dates": dates, "entries": entries})
	})

	group.GET("/stats", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		tz := c.QueryParam("tz")
		loc := time.UTC
		if tz != "" {
			if parsed, err := time.LoadLocation(tz); err == nil {
				loc = parsed
			}
		}
		total := s.CountDiaries(user.ID)
		now := time.Now().In(loc)
		oneYearAgo := now.AddDate(-1, 0, 0).Format("2006-01-02")
		diaries, _ := s.ListDiaries(user.ID, oneYearAgo+" 00:00:00.000Z", "", "-date", 365)
		dateSet := make(map[string]bool, len(diaries))
		for _, diary := range diaries {
			dateSet[store.DateOnly(diary.Date)] = true
		}
		streak := 0
		today := now.Format("2006-01-02")
		yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
		var checkDate time.Time
		if dateSet[today] {
			checkDate = now
		} else if dateSet[yesterday] {
			checkDate = now.AddDate(0, 0, -1)
		}
		for !checkDate.IsZero() {
			if !dateSet[checkDate.Format("2006-01-02")] {
				break
			}
			streak++
			checkDate = checkDate.AddDate(0, 0, -1)
		}
		return c.JSON(http.StatusOK, map[string]any{"total": total, "streak": streak})
	})

	group.GET("/search", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		query := c.QueryParam("q")
		scenario := c.QueryParam("scenario")
		if query == "" && scenario == "" {
			return badRequest("Query parameter 'q' or 'scenario' is required", nil)
		}
		diaries, err := s.SearchDiaries(user.ID, query, scenario, 50)
		if err != nil {
			return serverError("Search failed", err)
		}
		results := make([]map[string]any, 0, len(diaries))
		for _, diary := range diaries {
			snippet := diary.Content
			if runes := []rune(snippet); len(runes) > 200 {
				snippet = string(runes[:200]) + "..."
			}
			tags := diary.Tags
			if tags == nil {
				tags = []string{}
			}
			results = append(results, map[string]any{
				"id":        diary.ID,
				"date":      store.DateOnly(diary.Date),
				"snippet":   snippet,
				"mood":      diary.Mood,
				"scenarios": diary.Scenarios,
				"weather":   diary.Weather,
				"tags":      tags,
			})
		}
		return c.JSON(http.StatusOK, map[string]any{"results": results, "total": len(results)})
	})

	// Filter by mood and/or scenario
	group.GET("/filter", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		moodStr := c.QueryParam("mood")
		scenario := c.QueryParam("scenario")
		var mood int
		if moodStr != "" {
			fmt.Sscanf(moodStr, "%d", &mood)
		}
		diaries, err := s.FilterDiaries(user.ID, mood, scenario, 100)
		if err != nil {
			return serverError("Filter failed", err)
		}
		results := make([]map[string]any, 0, len(diaries))
		for _, diary := range diaries {
			snippet := diary.Content
			if runes := []rune(snippet); len(runes) > 200 {
				snippet = string(runes[:200]) + "..."
			}
			tags := diary.Tags
			if tags == nil {
				tags = []string{}
			}
			results = append(results, map[string]any{
				"id":        diary.ID,
				"date":      store.DateOnly(diary.Date),
				"snippet":   snippet,
				"mood":      diary.Mood,
				"scenarios": diary.Scenarios,
				"weather":   diary.Weather,
				"tags":      tags,
			})
		}
		return c.JSON(http.StatusOK, map[string]any{"results": results, "total": len(results)})
	})

	group.POST("/by-ids", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		var body struct {
			IDs []string `json:"ids"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		result := make([]map[string]any, 0, len(body.IDs))
		for _, id := range body.IDs {
			diary, err := s.GetDiaryByID(id)
			if err != nil || diary.Owner != user.ID {
				continue
			}
			result = append(result, diaryResponse(diary, store.DateOnly(diary.Date), true))
		}
		return c.JSON(http.StatusOK, map[string]any{"diaries": result})
	})

	group.GET("/recent", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		limit := 5
		if raw := c.QueryParam("limit"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil {
				limit = parsed
			}
		}
		if limit <= 0 {
			limit = 5
		}
		if limit > 100 {
			limit = 100
		}
		diaries, err := s.ListDiaries(user.ID, "", "", "-date", limit)
		if err != nil {
			return badRequest("Failed to fetch recent diaries", err)
		}
		result := make([]map[string]any, 0, len(diaries))
		for _, diary := range diaries {
			result = append(result, map[string]any{"id": diary.ID, "date": store.DateOnly(diary.Date), "content": diary.Content})
		}
		return c.JSON(http.StatusOK, map[string]any{"diaries": result})
	})

	group.GET("/:id", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		diary, err := s.GetDiaryByID(c.PathParam("id"))
		if err != nil {
			return notFound("Diary not found")
		}
		if diary.Owner != user.ID {
			return forbidden("Access denied")
		}
		return c.JSON(http.StatusOK, diaryResponse(diary, store.DateOnly(diary.Date), true))
	})

	group.DELETE("/:id", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		if err := s.DeleteDiary(c.PathParam("id"), user.ID); err != nil {
			return notFound("Diary not found")
		}
		if onDiaryChanged != nil {
			onDiaryChanged(user.ID)
		}
		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	group.GET("/tags", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		tags, err := s.ListTagCounts(user.ID)
		if err != nil {
			return serverError("Failed to fetch tags", err)
		}
		return c.JSON(http.StatusOK, map[string]any{"tags": tags, "total": len(tags)})
	})

	group.GET("/by-tag/:tag", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		tag := c.PathParam("tag")
		diaries, err := s.ListDiariesByTag(user.ID, tag)
		if err != nil {
			return serverError("Failed to fetch diaries by tag", err)
		}
		result := make([]map[string]any, 0, len(diaries))
		for _, diary := range diaries {
			result = append(result, diaryResponse(diary, store.DateOnly(diary.Date), true))
		}
		return c.JSON(http.StatusOK, map[string]any{"tag": tag, "diaries": result, "total": len(result)})
	})
}

func diaryResponse(diary *store.Diary, date string, exists bool) map[string]any {
	tags := diary.Tags
	if tags == nil {
		tags = []string{}
	}
	moodStates := diary.MoodStates
	if moodStates == nil {
		moodStates = []string{}
	}
	scenarios := diary.Scenarios
	if scenarios == nil {
		scenarios = []string{}
	}
	return map[string]any{
		"id":         diary.ID,
		"date":       date,
		"content":    diary.Content,
		"mood":       diary.Mood,
		"mood_states": moodStates,
		"scenarios":  scenarios,
		"weather":    diary.Weather,
		"tags":       tags,
		"owner":      diary.Owner,
		"created":    diary.Created,
		"updated":    diary.Updated,
		"exists":     exists,
	}
}
