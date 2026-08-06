package api

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/store"
)

const pingTimeout = 3 * time.Second

// RegisterHealthRoutes registers health check endpoints for Docker/k8s health checks.
func RegisterHealthRoutes(e *echo.Echo, s *store.Store) {
	handler := func(c *echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), pingTimeout)
		defer cancel()

		if err := s.DB.PingContext(ctx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "error",
				"error":  err.Error(),
			})
		}

		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	}

	e.GET("/healthz", handler)
	e.GET("/health", handler)
}