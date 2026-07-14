package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
	"github.com/songtianlun/diarum/internal/weather"
)

func RegisterWeatherRoutes(e *echo.Echo, s *store.Store, authMiddleware echo.MiddlewareFunc) {
	configService := config.NewConfigService(s)

	group := e.Group("/api/v1/weather", authMiddleware)

	group.GET("", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID

		mcpURL, _ := configService.GetString(userID, "weather.mcp_url")
		if mcpURL == "" {
			mcpURL = "http://localhost:8080"
		}
		useMCP, _ := configService.GetBool(userID, "weather.use_mcp")

		svc := weather.NewService(mcpURL, useMCP)

		city := c.QueryParam("city")
		if city == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "city parameter is required",
			})
		}

		date := c.QueryParam("date")

		result, err := svc.GetWeather(city, date)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}

		return c.JSON(http.StatusOK, result)
	})

	group.GET("/coords", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID

		mcpURL, _ := configService.GetString(userID, "weather.mcp_url")
		if mcpURL == "" {
			mcpURL = "http://localhost:8080"
		}
		useMCP, _ := configService.GetBool(userID, "weather.use_mcp")

		svc := weather.NewService(mcpURL, useMCP)

		city := c.QueryParam("city")
		lat := c.QueryParam("lat")
		lon := c.QueryParam("lon")

		if city == "" || lat == "" || lon == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "city, lat, and lon parameters are required",
			})
		}

		var latF, lonF float64
		if _, err := fmt.Sscanf(lat, "%f", &latF); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid lat parameter",
			})
		}
		if _, err := fmt.Sscanf(lon, "%f", &lonF); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid lon parameter",
			})
		}

		date := c.QueryParam("date")

		result, err := svc.GetWeatherByCoords(city, latF, lonF, date)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}

		return c.JSON(http.StatusOK, result)
	})

	// Batch backfill weather for diaries missing weather data (SSE stream)
	group.POST("/backfill", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID

		var body struct {
			StartDate string `json:"start_date"`
			SkipEmpty bool   `json:"skip_empty"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}

		mcpURL, _ := configService.GetString(userID, "weather.mcp_url")
		if mcpURL == "" {
			mcpURL = "http://localhost:8080"
		}
		useMCP, _ := configService.GetBool(userID, "weather.use_mcp")
		svc := weather.NewService(mcpURL, useMCP)

		defaultCity, _ := configService.GetString(userID, "weather.default_city")
		if defaultCity == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "请先设置默认城市",
			})
		}

		// Build date range for query
		start := ""
		if body.StartDate != "" {
			start = body.StartDate + " 00:00:00.000Z"
		}

		// Get all diaries in date range
		diaries, err := s.ListDiaries(userID, start, "", "-date", 0)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to fetch diaries",
			})
		}

		// Set SSE headers
		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Connection", "keep-alive")
		c.Response().Header().Set("X-Accel-Buffering", "no")

		writer := &sseWriter{w: c.Response()}
		sendEvent := func(event string, data any) {
			jsonData, _ := json.Marshal(data)
			fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, jsonData)
			writer.Flush()
		}

		updated := 0
		skipped := 0
		failed := 0

		today := time.Now().Format("2006-01-02")

		// Send initial progress
		sendEvent("progress", map[string]any{
			"current": 0,
			"total":   len(diaries),
			"status":  "开始补全天气...",
		})

		for i, diary := range diaries {
			date := store.DateOnly(diary.Date)

			// Skip if already has weather data
			if diary.Weather != "" {
				skipped++
				sendEvent("skipped", map[string]any{
					"date":   date,
					"reason": "已有天气数据",
				})
				continue
			}

			// Skip future dates
			if date > today {
				skipped++
				sendEvent("skipped", map[string]any{
					"date":   date,
					"reason": "未来日期",
				})
				continue
			}

			// Skip empty content if requested
			if body.SkipEmpty && diary.Content == "" {
				skipped++
				sendEvent("skipped", map[string]any{
					"date":   date,
					"reason": "无内容",
				})
				continue
			}

			// Fetch weather for this date
			city := diary.City
			if city == "" {
				city = defaultCity
			}

			// Send progress before API call
			sendEvent("progress", map[string]any{
				"current": i + 1,
				"total":   len(diaries),
				"date":    date,
				"status":  fmt.Sprintf("正在获取 %s 的天气...", date),
			})

			// Rate limit: wait between API calls to avoid throttling
			time.Sleep(150 * time.Millisecond)

			result, err := svc.GetWeather(city, date)
			if err != nil {
				failed++
				sendEvent("error", map[string]any{
					"date":  date,
					"error": err.Error(),
				})
				continue
			}

			// Update diary with weather data
			_, _, err = s.UpsertDiary(
				userID,
				date,
				diary.Content,
				diary.Mood,
				diary.MoodStates,
				diary.Scenarios,
				fmt.Sprintf("%d", result.WMOCode),
				diary.Tags,
				city,
				result.TempMin,
				result.TempMax,
			)
			if err != nil {
				failed++
				sendEvent("error", map[string]any{
					"date":  date,
					"error": err.Error(),
				})
				continue
			}

			updated++
			sendEvent("updated", map[string]any{
				"date":    date,
				"weather": fmt.Sprintf("%d", result.WMOCode),
				"updated": updated,
			})
		}

		// Send completion event
		sendEvent("complete", map[string]any{
			"total":   len(diaries),
			"updated": updated,
			"skipped": skipped,
			"failed":  failed,
		})

		return nil
	})
}
