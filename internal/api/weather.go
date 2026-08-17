package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
	"github.com/songtianlun/diarum/internal/weather"
)

// backfillLocks 防止同一用户并发发起补全任务（避免重复调用外部 API）
var backfillLocks sync.Map

func RegisterWeatherRoutes(e *echo.Echo, s *store.Store, authMiddleware echo.MiddlewareFunc) {
	configService := config.NewConfigService(s)

	group := e.Group("/api/v1/weather", authMiddleware)

	// Weather provider status
	group.GET("/provider", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"qweather_enabled": weather.QWeatherEnabled(),
		})
	})

	// Search cities by name (uses Open-Meteo geocoding API)
	group.GET("/cities", func(c *echo.Context) error {
		query := c.QueryParam("q")
		if query == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "q parameter is required",
			})
		}

		cities, err := weather.SearchCities(query)
		if err != nil {
			// Return appropriate status code for Nominatim rate limiting
			var nomErr *weather.NominatimError
			if errors.As(err, &nomErr) {
				status := http.StatusBadGateway
				if nomErr.StatusCode == http.StatusTooManyRequests {
					status = http.StatusTooManyRequests
				}
				return c.JSON(status, map[string]string{
					"error": "定位服务暂时不可用，请稍后重试或手动选择城市",
				})
			}
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}

		return c.JSON(http.StatusOK, cities)
	})

	group.GET("", func(c *echo.Context) error {
		svc := weather.NewService()

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
		svc := weather.NewService()

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

		svc := weather.NewService()

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

		// 同一用户已有补全任务在运行时直接返回，避免重复消耗外部 API
		if _, inFlight := backfillLocks.LoadOrStore(userID, true); inFlight {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "天气补全任务已在运行中，请稍候",
			})
		}
		defer backfillLocks.Delete(userID)

		writer := &sseWriter{w: c.Response()}
		// 返回 error：写入失败（客户端断开）时立即中止，不再继续请求外部 API
		sendEvent := func(event string, data any) error {
			jsonData, err := json.Marshal(data)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, jsonData); err != nil {
				return err
			}
			writer.Flush()
			return nil
		}

		updated := 0
		skipped := 0
		failed := 0

		today := time.Now().Format("2006-01-02")

		// Send initial progress
		if err := sendEvent("progress", map[string]any{
			"current": 0,
			"total":   len(diaries),
			"status":  "开始补全天气...",
		}); err != nil {
			return nil
		}

		for i, diary := range diaries {
			// 客户端断开连接时立即停止循环，避免继续调用外部 API
			if c.Request().Context().Err() != nil {
				return nil
			}
			date := store.DateOnly(diary.Date)

			// Check if weather is a valid WMO code (numeric)
			weatherCode := 0
			hasValidWeather := false
			if diary.Weather != "" {
				n, err := fmt.Sscanf(diary.Weather, "%d", &weatherCode)
				if err == nil && n == 1 && weatherCode >= 0 && weatherCode <= 99 {
					hasValidWeather = true
				}
			}

			// Skip if already has valid weather data
			if hasValidWeather {
				skipped++
				if err := sendEvent("skipped", map[string]any{
					"date":   date,
					"reason": "已有有效天气代码",
				}); err != nil {
					return nil
				}
				continue
			}

			// Skip future dates
			if date > today {
				skipped++
				if err := sendEvent("skipped", map[string]any{
					"date":   date,
					"reason": "未来日期",
				}); err != nil {
					return nil
				}
				continue
			}

			// Skip empty content if requested
			if body.SkipEmpty && diary.Content == "" {
				skipped++
				if err := sendEvent("skipped", map[string]any{
					"date":   date,
					"reason": "无内容",
				}); err != nil {
					return nil
				}
				continue
			}

			// If has old emoji data, note it will be overwritten
			if diary.Weather != "" {
				if err := sendEvent("progress", map[string]any{
					"current": i + 1,
					"total":   len(diaries),
					"date":    date,
					"status":  fmt.Sprintf("替换旧天气数据: %s", diary.Weather),
				}); err != nil {
					return nil
				}
			}

			// Fetch weather for this date
			city := diary.City
			if city == "" {
				city = defaultCity
			}

			// Send progress before API call
			if err := sendEvent("progress", map[string]any{
				"current": i + 1,
				"total":   len(diaries),
				"date":    date,
				"status":  fmt.Sprintf("正在获取 %s 的天气...", date),
			}); err != nil {
				return nil
			}

			// Rate limit: wait between API calls to avoid throttling
			time.Sleep(150 * time.Millisecond)

			result, err := svc.GetWeather(city, date)
			if err != nil {
				failed++
				if err := sendEvent("error", map[string]any{
					"date":  date,
					"error": err.Error(),
				}); err != nil {
					return nil
				}
				continue
			}

			// 仅更新天气字段，避免用查询时点的快照覆盖用户随后编辑的 content/mood/tags
			_, err = s.UpsertDiaryWeather(
				userID,
				date,
				fmt.Sprintf("%d", result.WMOCode),
				city,
				result.TempMin,
				result.TempMax,
			)
			if err != nil {
				failed++
				if err := sendEvent("error", map[string]any{
					"date":  date,
					"error": err.Error(),
				}); err != nil {
					return nil
				}
				continue
			}

			updated++
			if err := sendEvent("updated", map[string]any{
				"date":    date,
				"weather": fmt.Sprintf("%d", result.WMOCode),
				"updated": updated,
			}); err != nil {
				return nil
			}
		}

		// Send completion event
		if err := sendEvent("complete", map[string]any{
			"total":   len(diaries),
			"updated": updated,
			"skipped": skipped,
			"failed":  failed,
		}); err != nil {
			return nil
		}

		return nil
	})
}
