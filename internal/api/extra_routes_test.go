package api

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/backup"
	"github.com/songtianlun/diarum/internal/config"
)

func TestHealthRoutes(t *testing.T) {
	s := newTestStore(t)
	e := echo.New()
	RegisterHealthRoutes(e, s)

	rec := performRequest(t, e, http.MethodGet, "/healthz", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["status"] != "ok" {
		t.Fatalf("/healthz payload = %#v", payload)
	}

	// Close the DB to exercise the error branch.
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	rec = performRequest(t, e, http.MethodGet, "/health", nil, nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/health after close status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMetricsRoutes(t *testing.T) {
	e := echo.New()
	RegisterMetricsRoutes(e)

	rec := performRequest(t, e, http.MethodGet, "/api/v1/metrics", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/v1/metrics status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["go_version"] == nil || payload["uptime"] == nil {
		t.Fatalf("/api/v1/metrics payload = %#v", payload)
	}

	for _, path := range []string{"/api/metrics", "/metrics"} {
		rec = performRequest(t, e, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, rec.Code)
		}
	}
}

func TestBackupRoutes(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	cfg := config.NewConfigService(s)
	scheduler := backup.NewScheduler(s, cfg, t.TempDir(), func(userID string) (*bytes.Buffer, error) {
		return bytes.NewBufferString("zip"), nil
	})
	e := echo.New()
	RegisterBackupRoutes(e, s, authMiddlewareFor(user), scheduler, cfg)

	// List (empty)
	rec := performRequest(t, e, http.MethodGet, "/api/v1/backups", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list backups status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["total"] != float64(0) {
		t.Fatalf("list backups payload = %#v", payload)
	}

	// Get settings (defaults)
	rec = performRequest(t, e, http.MethodGet, "/api/v1/backups/settings", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get backup settings status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Save settings
	rec = performRequest(t, e, http.MethodPost, "/api/v1/backups/settings", bytes.NewBufferString(`{"enabled":true,"frequency":"daily","time":"07:00","day_of_week":1,"day_of_month":15,"retention_days":30,"upload_s3":false}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("save backup settings status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Verify persisted
	rec = performRequest(t, e, http.MethodGet, "/api/v1/backups/settings", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get backup settings after save status = %d", rec.Code)
	}
	if payload := decodeJSONBody(t, rec); payload["enabled"] != true || payload["frequency"] != "daily" {
		t.Fatalf("backup settings after save = %#v", payload)
	}

	// Trigger a backup
	rec = performRequest(t, e, http.MethodPost, "/api/v1/backups/trigger", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("trigger backup status = %d body=%s", rec.Code, rec.Body.String())
	}

	// List again — should have one backup
	rec = performRequest(t, e, http.MethodGet, "/api/v1/backups?page=1&per_page=10", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list backups after trigger status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	if payload["total"].(float64) != 1 {
		t.Fatalf("expected 1 backup, got %#v", payload)
	}
	backups := payload["backups"].([]any)
	backupID := backups[0].(map[string]any)["id"].(string)

	// Get backup detail
	rec = performRequest(t, e, http.MethodGet, "/api/v1/backups/"+backupID, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get backup detail status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Download backup file
	rec = performRequest(t, e, http.MethodGet, "/api/v1/backups/"+backupID+"/download", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("download backup status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Delete backup
	rec = performRequest(t, e, http.MethodDelete, "/api/v1/backups/"+backupID, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete backup status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Missing backup by other user
	other := newTestUser(t, s)
	e2 := echo.New()
	RegisterBackupRoutes(e2, s, authMiddlewareFor(other), scheduler, cfg)
	rec = performRequest(t, e2, http.MethodGet, "/api/v1/backups/"+backupID, nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get backup of other user status = %d, want 404", rec.Code)
	}
	rec = performRequest(t, e2, http.MethodDelete, "/api/v1/backups/"+backupID, nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete backup of other user status = %d, want 404", rec.Code)
	}
}

func TestBackupTriggerFailure(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	cfg := config.NewConfigService(s)
	// Export fn always fails → trigger returns error
	scheduler := backup.NewScheduler(s, cfg, t.TempDir(), func(userID string) (*bytes.Buffer, error) {
		return nil, bytes.ErrTooLarge
	})
	e := echo.New()
	RegisterBackupRoutes(e, s, authMiddlewareFor(user), scheduler, cfg)

	rec := performRequest(t, e, http.MethodPost, "/api/v1/backups/trigger", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("trigger backup failure status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBackupSettingsInvalidBody(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	cfg := config.NewConfigService(s)
	scheduler := backup.NewScheduler(s, cfg, t.TempDir(), func(userID string) (*bytes.Buffer, error) {
		return bytes.NewBufferString("zip"), nil
	})
	e := echo.New()
	RegisterBackupRoutes(e, s, authMiddlewareFor(user), scheduler, cfg)

	rec := performRequest(t, e, http.MethodPost, "/api/v1/backups/settings", bytes.NewBufferString(`{not-json`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("backup settings invalid body status = %d", rec.Code)
	}
}
