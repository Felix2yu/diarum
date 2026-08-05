package api

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/push"
	"github.com/songtianlun/diarum/internal/store"
)

// RegisterPushRoutes registers web push notification endpoints.
func RegisterPushRoutes(e *echo.Echo, s *store.Store, authMiddleware echo.MiddlewareFunc, scheduler *push.Scheduler) {
	sender := push.NewSender(s)
	configService := config.NewConfigService(s)
	group := e.Group("/api/v1/push", authMiddleware)

	// Record the deployment hostname/origin on each authenticated request so
	// push notifications can build a valid VAPID subject and Topic header
	// (required by Apple's push service). Origin is preferred over Host
	// because it carries the true public origin the browser subscribed under.
	group.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			origin := c.Request().Header.Get("Origin")
			push.SiteOrigin = origin
			if host := push.OriginHost(origin); host != "" {
				push.SiteHost = host
			} else {
				push.SiteHost = push.NormalizeHost(c.Request().Host)
			}
			return next(c)
		}
	})

	// GET the VAPID public key used to create a push subscription.
	group.GET("/vapid-public-key", func(c *echo.Context) error {
		key, err := sender.PublicKey()
		if err != nil {
			return badRequest("Failed to load VAPID keys", err)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"public_key": key,
		})
	})

	// POST a browser push subscription to register it for reminders.
	group.POST("/subscriptions", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID

		var body struct {
			Endpoint string `json:"endpoint"`
			Keys     struct {
				P256dh string `json:"p256dh"`
				Auth   string `json:"auth"`
			} `json:"keys"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		if body.Endpoint == "" || body.Keys.P256dh == "" || body.Keys.Auth == "" {
			return badRequest("endpoint, keys.p256dh and keys.auth are required", nil)
		}

		if err := s.SavePushSubscription(userID, body.Endpoint, body.Keys.P256dh, body.Keys.Auth); err != nil {
			return badRequest("Failed to save subscription", err)
		}

		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	// DELETE a browser push subscription (e.g. when the user unsubscribes).
	group.DELETE("/subscriptions", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID

		var body struct {
			Endpoint string `json:"endpoint"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		if body.Endpoint == "" {
			return badRequest("endpoint is required", nil)
		}

		if err := s.DeletePushSubscription(userID, body.Endpoint); err != nil {
			return badRequest("Failed to delete subscription", err)
		}

		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	// POST a test notification to all of the user's subscriptions.
	group.POST("/test", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID
		subs, err := s.ListPushSubscriptions(userID)
		if err != nil {
			return badRequest("Failed to list subscriptions", err)
		}
		if len(subs) == 0 {
			return badRequest("No active notification subscription on this browser", nil)
		}
		if err := sender.SendNotification(userID, "吾身 · 测试通知", "这是一条测试通知 🎉"); err != nil {
			return badRequest("Failed to send test notification", err)
		}
		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	// POST/refresh the reminder schedule (enabled/time/timezone/message).
	group.POST("/schedule", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID

		var body struct {
			Enabled  bool   `json:"enabled"`
			Time     string `json:"time"`
			TimeZone string `json:"timezone"`
			Message  string `json:"message"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}

		settings := map[string]any{
			"webpush.enabled": body.Enabled,
			"webpush.time":    body.Time,
			"webpush.tz":      body.TimeZone,
			"webpush.message": body.Message,
		}
		if err := configService.SetBatch(userID, settings); err != nil {
			return badRequest("Failed to save reminder settings", err)
		}
		scheduler.Refresh(userID)

		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})
}
