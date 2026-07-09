package api

import (
	"fmt"
	"net/http"

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
}
