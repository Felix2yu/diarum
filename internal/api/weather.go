package api

import (
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

	// Batch backfill weather for diaries missing weather data
	group.POST("/backfill", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID

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

		// Get all diaries without weather data
		diaries, err := s.ListDiaries(userID, "", "", "-date", 0)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to fetch diaries",
			})
		}

		type BackfillResult struct {
			Date    string `json:"date"`
			Status  string `json:"status"`
			Weather string `json:"weather,omitempty"`
			Error   string `json:"error,omitempty"`
		}

		var results []BackfillResult
		updated := 0
		skipped := 0
		failed := 0

		for _, diary := range diaries {
			date := store.DateOnly(diary.Date)

			// Skip if already has weather data
			if diary.Weather != "" && diary.Weather != "0" {
				skipped++
				continue
			}

			// Skip future dates
			today := time.Now().Format("2006-01-02")
			if date > today {
				skipped++
				continue
			}

			// Fetch weather for this date
			city := diary.City
			if city == "" {
				city = defaultCity
			}

			result, err := svc.GetWeather(city, date)
			if err != nil {
				failed++
				results = append(results, BackfillResult{
					Date:   date,
					Status: "failed",
					Error:  err.Error(),
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
				results = append(results, BackfillResult{
					Date:   date,
					Status: "failed",
					Error:  err.Error(),
				})
				continue
			}

			updated++
			results = append(results, BackfillResult{
				Date:    date,
				Status:  "updated",
				Weather: fmt.Sprintf("%d", result.WMOCode),
			})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"total":   len(diaries),
			"updated": updated,
			"skipped": skipped,
			"failed":  failed,
			"results": results,
		})
	})
}
