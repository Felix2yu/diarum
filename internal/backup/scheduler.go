package backup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/scheduler"
	"github.com/songtianlun/diarum/internal/store"
)

// ExportFn builds a ZIP archive for the given user
type ExportFn func(userID string) (*bytes.Buffer, error)

// Scheduler manages per-user backup timers
type Scheduler struct {
	store         *store.Store
	configService *config.ConfigService
	dataDir       string
	exportFn      ExportFn
	timer         *scheduler.Timer
}

// NewScheduler creates a new backup scheduler
func NewScheduler(s *store.Store, cfg *config.ConfigService, dataDir string, exportFn ExportFn) *Scheduler {
	sc := &Scheduler{
		store:         s,
		configService: cfg,
		dataDir:       dataDir,
		exportFn:      exportFn,
	}
	sc.timer = scheduler.NewTimer(sc.listUserIDs)
	sc.timer.Enabled = func(userID string) bool {
		enabled, _ := sc.configService.GetBool(userID, "backup.enabled")
		return enabled
	}
	sc.timer.Next = sc.nextBackupTime
	sc.timer.Run = func(userID string) { _ = sc.execute(userID) }
	return sc
}

func (sc *Scheduler) listUserIDs() ([]string, error) {
	users, err := sc.store.ListUsers()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids, nil
}

// Start initializes timers for all users with backup enabled
func (sc *Scheduler) Start() {
	users, err := sc.store.ListUsers()
	if err != nil {
		logger.Error("[Backup] failed to list users: %v", err)
		return
	}
	for _, u := range users {
		sc.Refresh(u.ID)
	}
	logger.Info("[Backup] scheduler started for %d users", len(users))
}

// Stop cancels all timers
func (sc *Scheduler) Stop() {
	sc.timer.Stop()
}

// Refresh recalculates and resets the timer for a user
func (sc *Scheduler) Refresh(userID string) {
	sc.timer.Refresh(userID)
}

// RunNow triggers an immediate backup for a user
func (sc *Scheduler) RunNow(userID string) error {
	return sc.execute(userID)
}

func (sc *Scheduler) execute(userID string) error {
	logger.Info("[Backup] executing backup for user %s", userID)

	buf, err := sc.exportFn(userID)
	if err != nil {
		logger.Error("[Backup] export failed for user %s: %v", userID, err)
		return err
	}

	// Save to local disk
	backupDir := filepath.Join(sc.dataDir, "backups", userID)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		logger.Error("[Backup] failed to create dir %s: %v", backupDir, err)
		return err
	}

	filename := fmt.Sprintf("diarum_backup_%s.zip", time.Now().UTC().Format("20060102_150405"))
	filePath := filepath.Join(backupDir, filename)
	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		logger.Error("[Backup] failed to write %s: %v", filePath, err)
		return err
	}

	// Upload to S3 if configured
	s3Key := ""
	var s3Err error
	uploadS3, _ := sc.configService.GetBool(userID, "backup.upload_s3")
	if uploadS3 {
		s3Key = fmt.Sprintf("backups/%s/%s", userID, filename)
		if err := sc.store.UploadToS3(userID, s3Key, buf.Bytes()); err != nil {
			logger.Error("[Backup] S3 upload failed for user %s: %v", userID, err)
			s3Err = err
			s3Key = ""
		}
	}

	// Create backup record
	_, err = sc.store.CreateBackup(userID, filename, filePath, int64(buf.Len()), s3Key)
	if err != nil {
		logger.Error("[Backup] failed to create record for user %s: %v", userID, err)
		return err
	}

	// Cleanup old backups
	retentionDays, _ := sc.configService.GetInt(userID, "backup.retention_days")
	if retentionDays > 0 {
		removed, _ := sc.store.CleanupOldBackups(userID, retentionDays)
		for _, b := range removed {
			_ = os.Remove(b.Filepath)
			if b.S3Key != "" {
				if err := sc.store.DeleteObjectFromS3(userID, b.S3Key); err != nil {
					logger.Error("[Backup] failed to delete S3 object %s for user %s: %v", b.S3Key, userID, err)
				}
			}
		}
		if len(removed) > 0 {
			logger.Info("[Backup] cleaned up %d old backups for user %s", len(removed), userID)
		}
	}

	if s3Err != nil {
		// The local backup succeeded, but surfacing the S3 error lets a manual
		// trigger report it instead of silently succeeding.
		logger.Error("[Backup] local backup saved but S3 upload failed for user %s: %v", userID, s3Err)
		return fmt.Errorf("local backup saved but S3 upload failed: %w", s3Err)
	}

	logger.Info("[Backup] backup completed for user %s: %s (%d bytes)", userID, filename, buf.Len())

	// Schedule next backup
	sc.Refresh(userID)
	return nil
}

// nextBackupTime calculates when the next backup should run
func (sc *Scheduler) nextBackupTime(userID string) time.Time {
	frequency, _ := sc.configService.GetString(userID, "backup.frequency")
	timeStr, _ := sc.configService.GetString(userID, "backup.time")
	if timeStr == "" {
		timeStr = "00:00"
	}

	hour, minute := parseTime(timeStr)
	now := time.Now().UTC()

	switch frequency {
	case "daily":
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		return next

	case "weekly":
		dow, _ := sc.configService.GetInt(userID, "backup.day_of_week")
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
		daysAhead := dow - int(next.Weekday())
		if daysAhead < 0 {
			daysAhead += 7
		}
		if daysAhead == 0 && !next.After(now) {
			daysAhead = 7
		}
		next = next.AddDate(0, 0, daysAhead)
		return next

	case "monthly":
		dom, _ := sc.configService.GetInt(userID, "backup.day_of_month")
		dom = max(dom, 1)
		next := time.Date(now.Year(), now.Month(), dom, hour, minute, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.AddDate(0, 1, 0)
		}
		return next
	}

	return time.Time{}
}

func parseTime(s string) (int, int) {
	parts := strings.Split(s, ":")
	hour := 0
	minute := 0
	if len(parts) >= 1 {
		hour, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		minute, _ = strconv.Atoi(parts[1])
	}
	return hour, minute
}
