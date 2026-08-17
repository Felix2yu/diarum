package api

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/backup"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/store"
)

// RegisterBackupRoutes registers backup-related API endpoints
func RegisterBackupRoutes(e *echo.Echo, s *store.Store, authMiddleware echo.MiddlewareFunc, scheduler *backup.Scheduler, cfg *config.ConfigService) {
	group := e.Group("/api/v1/backups", authMiddleware)

	// List backups
	group.GET("", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID
		page, _ := strconv.Atoi(c.QueryParam("page"))
		if page < 1 {
			page = 1
		}
		perPage, _ := strconv.Atoi(c.QueryParam("per_page"))
		if perPage < 1 {
			perPage = 20
		}
		backups, total, err := s.ListBackups(userID, page, perPage)
		if err != nil {
			return badRequest("Failed to list backups", err)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"backups":  backups,
			"total":    total,
			"page":     page,
			"per_page": perPage,
			"pages":    store.TotalPages(total, perPage),
		})
	})

	// Get backup details
	group.GET("/:id", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID
		id := c.Param("id")
		b, err := s.GetBackupByID(id)
		if err != nil || b.Owner != userID {
			return notFound("Backup not found")
		}
		return c.JSON(http.StatusOK, b)
	})

	// Download backup file
	group.GET("/:id/download", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID
		id := c.Param("id")
		b, err := s.GetBackupByID(id)
		if err != nil || b.Owner != userID {
			return notFound("Backup not found")
		}
		// Backups are stored at absolute filesystem paths. echo's c.File/c.FileFS
		// open through c.echo.Filesystem (default os.DirFS(".")) which cannot
		// resolve absolute paths, so serve the file directly instead.
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", b.Filename))
		c.Response().Header().Set(echo.HeaderContentType, "application/zip")
		http.ServeFile(c.Response(), c.Request(), b.Filepath)
		return nil
	})

	// Delete backup
	group.DELETE("/:id", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID
		id := c.Param("id")
		b, err := s.GetBackupByID(id)
		if err != nil || b.Owner != userID {
			return notFound("Backup not found")
		}
		// Remove file from disk
		_ = os.Remove(b.Filepath)
		if b.S3Key != "" {
			if err := s.DeleteObjectFromS3(userID, b.S3Key); err != nil {
				logger.Warn("[Backup] failed to delete S3 object %s for user %s: %v", b.S3Key, userID, err)
			}
		}
		// Remove DB record
		if err := s.DeleteBackup(id, userID); err != nil {
			return badRequest("Failed to delete backup", err)
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
	})

	// Manual trigger backup
	group.POST("/trigger", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID
		if err := scheduler.RunNow(userID); err != nil {
			return badRequest("Backup failed: "+err.Error(), nil)
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "backup completed"})
	})

	// Save backup settings and refresh scheduler
	group.POST("/settings", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID
		var body struct {
			Enabled       *bool   `json:"enabled"`
			Frequency     *string `json:"frequency"`
			Time          *string `json:"time"`
			DayOfWeek     *int    `json:"day_of_week"`
			DayOfMonth    *int    `json:"day_of_month"`
			RetentionDays *int    `json:"retention_days"`
			UploadS3      *bool   `json:"upload_s3"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		if body.Enabled != nil {
			_ = cfg.Set(userID, "backup.enabled", *body.Enabled)
		}
		if body.Frequency != nil {
			_ = cfg.Set(userID, "backup.frequency", *body.Frequency)
		}
		if body.Time != nil {
			_ = cfg.Set(userID, "backup.time", *body.Time)
		}
		if body.DayOfWeek != nil {
			_ = cfg.Set(userID, "backup.day_of_week", *body.DayOfWeek)
		}
		if body.DayOfMonth != nil {
			_ = cfg.Set(userID, "backup.day_of_month", *body.DayOfMonth)
		}
		if body.RetentionDays != nil {
			_ = cfg.Set(userID, "backup.retention_days", *body.RetentionDays)
		}
		if body.UploadS3 != nil {
			_ = cfg.Set(userID, "backup.upload_s3", *body.UploadS3)
		}
		scheduler.Refresh(userID)
		return c.JSON(http.StatusOK, map[string]string{"status": "saved"})
	})

	// Get backup settings
	group.GET("/settings", func(c *echo.Context) error {
		userID := auth.CurrentUser(c).ID
		enabled, _ := cfg.GetBool(userID, "backup.enabled")
		frequency, _ := cfg.GetString(userID, "backup.frequency")
		timeStr, _ := cfg.GetString(userID, "backup.time")
		dow, _ := cfg.GetInt(userID, "backup.day_of_week")
		dom, _ := cfg.GetInt(userID, "backup.day_of_month")
		retention, _ := cfg.GetInt(userID, "backup.retention_days")
		uploadS3, _ := cfg.GetBool(userID, "backup.upload_s3")
		return c.JSON(http.StatusOK, map[string]any{
			"enabled":        enabled,
			"frequency":      frequency,
			"time":           timeStr,
			"day_of_week":    dow,
			"day_of_month":   dom,
			"retention_days": retention,
			"upload_s3":      uploadS3,
		})
	})
}
