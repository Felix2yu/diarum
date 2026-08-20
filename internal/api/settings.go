package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/store"
	"github.com/songtianlun/diarum/internal/weather"
)

var weatherSettingsKeys = map[string]bool{
	"weather.auto_fetch":      true,
	"weather.auto_fetch_time": true,
	"weather.default_city":    true,
	"weather.enabled":         true,
}

type settingResponse struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type successResponse struct {
	Success bool `json:"success"`
}

type apiTokenStatusResponse struct {
	Exists  bool   `json:"exists"`
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
}

type apiTokenToggleResponse struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
}

type settingsBatchResponse struct {
	Settings map[string]any `json:"settings"`
}

// generateToken generates a random 32-character hex token
func generateToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func getSettingHandler(configService *config.ConfigService) echo.HandlerFunc {
	return func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		key := c.Param("key")

		if _, ok := config.GetConfigMeta(key); !ok {
			return badRequest("Unknown setting key: "+key, nil)
		}

		value, err := configService.Get(userId, key)
		if err != nil {
			return badRequest("Failed to get setting", err)
		}
		if config.IsEncrypted(key) {
			if s, ok := value.(string); ok {
				value = maskSecret(s)
			}
		}

		return c.JSON(http.StatusOK, settingResponse{
			Key:   key,
			Value: value,
		})
	}
}

func putSettingHandler(configService *config.ConfigService) echo.HandlerFunc {
	return func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		key := c.Param("key")

		if _, ok := config.GetConfigMeta(key); !ok {
			return badRequest("Unknown setting key: "+key, nil)
		}

		var body struct {
			Value any `json:"value"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}

		if err := configService.Set(userId, key, body.Value); err != nil {
			return badRequest("Failed to save setting", err)
		}

		return c.JSON(http.StatusOK, successResponse{Success: true})
	}
}

func deleteSettingHandler(configService *config.ConfigService) echo.HandlerFunc {
	return func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		key := c.Param("key")

		if _, ok := config.GetConfigMeta(key); !ok {
			return badRequest("Unknown setting key: "+key, nil)
		}

		if err := configService.Delete(userId, key); err != nil {
			return badRequest("Failed to delete setting", err)
		}

		return c.JSON(http.StatusOK, successResponse{Success: true})
	}
}

// RegisterSettingsRoutes registers settings-related API endpoints
func RegisterSettingsRoutes(e *echo.Echo, store *store.Store, authMiddleware echo.MiddlewareFunc, weatherScheduler *weather.Scheduler) {
	configService := config.NewConfigService(store)
	group := e.Group("/api/v1/settings", authMiddleware)

	// Get API token status and value
	group.GET("/api-token", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID

		token, err := configService.GetString(userId, "api.token")
		if err != nil {
			logger.Debug("[GET /api/v1/settings/api-token] error getting token: %v", err)
		}
		enabled, err := configService.GetBool(userId, "api.enabled")
		if err != nil {
			logger.Debug("[GET /api/v1/settings/api-token] error getting enabled: %v", err)
		}

		// 不返回明文 token，只返回状态；token 仅在创建/重置时返回一次
		return c.JSON(http.StatusOK, apiTokenStatusResponse{
			Exists:  token != "",
			Enabled: enabled && token != "",
			Token:   "",
		})
	})

	// Toggle API token enabled/disabled
	group.POST("/api-token/toggle", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID

		token, err := configService.GetString(userId, "api.token")
		if err != nil {
			logger.Debug("[POST /api/v1/settings/api-token/toggle] error getting token: %v", err)
		}
		enabled, err := configService.GetBool(userId, "api.enabled")
		if err != nil {
			logger.Debug("[POST /api/v1/settings/api-token/toggle] error getting enabled: %v", err)
		}

		if token == "" {
			// No token exists, create one and enable it
			newToken, err := generateToken()
			if err != nil {
				return badRequest("Failed to generate token", err)
			}

			if err := configService.Set(userId, "api.token", newToken); err != nil {
				return badRequest("Failed to save token", err)
			}
			if err := configService.Set(userId, "api.enabled", true); err != nil {
				return badRequest("Failed to save enabled status", err)
			}

			return c.JSON(http.StatusOK, apiTokenToggleResponse{
				Enabled: true,
				Token:   newToken,
			})
		}

		// Toggle existing token
		newEnabled := !enabled
		if err := configService.Set(userId, "api.enabled", newEnabled); err != nil {
			return badRequest("Failed to update token", err)
		}

		// 已存在的 token 不再回读明文
		return c.JSON(http.StatusOK, apiTokenToggleResponse{
			Enabled: newEnabled,
			Token:   "",
		})
	})

	// Reset API token (generate new one)
	group.POST("/api-token/reset", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID

		// Generate new token
		newToken, err := generateToken()
		if err != nil {
			return badRequest("Failed to generate token", err)
		}

		// Save new token
		if err := configService.Set(userId, "api.token", newToken); err != nil {
			return badRequest("Failed to save token", err)
		}

		// Ensure enabled is set (default to true for reset)
		enabled, err := configService.GetBool(userId, "api.enabled")
		if err != nil {
			logger.Debug("[POST /api/v1/settings/api-token/reset] error getting enabled: %v", err)
		}
		if !enabled {
			if err := configService.Set(userId, "api.enabled", true); err != nil {
				logger.Debug("[POST /api/v1/settings/api-token/reset] error setting enabled: %v", err)
			}
			enabled = true
		}

		return c.JSON(http.StatusOK, apiTokenToggleResponse{
			Enabled: enabled,
			Token:   newToken,
		})
	})

	// Get all settings (new v1 API)
	group.GET("", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID

		settings, err := configService.GetBatch(userId)
		if err != nil {
			return badRequest("Failed to get settings", err)
		}

		// 加密键的明文不返回给客户端，只返回掩码
		for key, value := range settings {
			if config.IsEncrypted(key) {
				if s, ok := value.(string); ok {
					settings[key] = maskSecret(s)
				}
			}
		}

		return c.JSON(http.StatusOK, settingsBatchResponse{
			Settings: settings,
		})
	})

	// Batch update settings (new v1 API)
	group.PUT("/batch", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID

		var body struct {
			Settings map[string]any `json:"settings"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}

		// Validate keys against registry
		for key := range body.Settings {
			if _, ok := config.GetConfigMeta(key); !ok {
				return badRequest("Unknown setting key: "+key, nil)
			}
		}

		if err := configService.SetBatch(userId, body.Settings); err != nil {
			return badRequest("Failed to save settings", err)
		}

		// Notify weather scheduler when weather-related settings change
		for key := range body.Settings {
			if weatherSettingsKeys[key] {
				weatherScheduler.Refresh(userId)
				break
			}
		}

		return c.JSON(http.StatusOK, successResponse{Success: true})
	})

	// Get single setting by key
	group.GET("/:key", getSettingHandler(configService))

	// Update single setting by key
	group.PUT("/:key", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		key := c.Param("key")

		if _, ok := config.GetConfigMeta(key); !ok {
			return badRequest("Unknown setting key: "+key, nil)
		}

		var body struct {
			Value any `json:"value"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}

		if err := configService.Set(userId, key, body.Value); err != nil {
			return badRequest("Failed to save setting", err)
		}

		if weatherSettingsKeys[key] {
			weatherScheduler.Refresh(userId)
		}

		return c.JSON(http.StatusOK, successResponse{Success: true})
	})

	// Delete single setting by key
	group.DELETE("/:key", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		key := c.Param("key")

		if _, ok := config.GetConfigMeta(key); !ok {
			return badRequest("Unknown setting key: "+key, nil)
		}

		if err := configService.Delete(userId, key); err != nil {
			return badRequest("Failed to delete setting", err)
		}

		if weatherSettingsKeys[key] {
			weatherScheduler.Refresh(userId)
		}

		return c.JSON(http.StatusOK, successResponse{Success: true})
	})
}
