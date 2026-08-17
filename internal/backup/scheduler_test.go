package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newTestUser(t *testing.T, s *store.Store) *store.User {
	t.Helper()
	u, err := s.CreateUser("testuser", "test@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func testExportFn(userID string) (*bytes.Buffer, error) {
	return bytes.NewBufferString("zip-content"), nil
}

func TestSchedulerNewStartStop(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	sc := NewScheduler(s, cfg, t.TempDir(), testExportFn)
	if sc == nil || sc.userTimers == nil {
		t.Fatal("NewScheduler did not initialize userTimers")
	}
	sc.Start()
	sc.Stop()
}

func TestSchedulerRefreshDisabled(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	sc := NewScheduler(s, cfg, t.TempDir(), testExportFn)
	u := newTestUser(t, s)

	sc.Refresh(u.ID)
	if _, ok := sc.userTimers[u.ID]; ok {
		t.Fatal("timer should not exist when backup disabled")
	}
}

func TestSchedulerRefreshEnabled(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	sc := NewScheduler(s, cfg, t.TempDir(), testExportFn)
	u := newTestUser(t, s)

	if err := cfg.Set(u.ID, "backup.enabled", true); err != nil {
		t.Fatalf("Set enabled: %v", err)
	}
	sc.Refresh(u.ID)
	if _, ok := sc.userTimers[u.ID]; !ok {
		t.Fatal("timer should exist when backup enabled")
	}
	// 未配置频率 → nextBackupTime 为零值 → timer 被移除
	sc = NewScheduler(s, cfg, t.TempDir(), testExportFn)
	sc.Refresh(u.ID)
	sc.Stop()
}

func TestSchedulerRunNow(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	dataDir := t.TempDir()
	sc := NewScheduler(s, cfg, dataDir, testExportFn)
	u := newTestUser(t, s)

	if err := sc.RunNow(u.ID); err != nil {
		t.Fatalf("RunNow: %v", err)
	}

	// 备份文件存在
	files, err := os.ReadDir(filepath.Join(dataDir, "backups", u.ID))
	if err != nil || len(files) != 1 {
		t.Fatalf("backup files = %v, %v", files, err)
	}

	// 备份记录存在
	backups, total, err := s.ListBackups(u.ID, 1, 50)
	if err != nil || total != 1 || len(backups) != 1 {
		t.Fatalf("backup records = %v, total=%d, %v", backups, total, err)
	}
	if backups[0].Filename != files[0].Name() {
		t.Errorf("record filename %q != disk filename %q", backups[0].Filename, files[0].Name())
	}
}

func TestSchedulerRunNowExportError(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	sc := NewScheduler(s, cfg, t.TempDir(), func(userID string) (*bytes.Buffer, error) {
		return nil, os.ErrPermission
	})
	u := newTestUser(t, s)

	if err := sc.RunNow(u.ID); err == nil {
		t.Fatal("expected export error")
	}
}

func TestSchedulerRunNowWriteError(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	sc := NewScheduler(s, cfg, t.TempDir(), testExportFn)
	u := newTestUser(t, s)

	// dataDir 指向一个文件，MkdirAll 会失败
	f := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	sc.dataDir = f
	if err := sc.RunNow(u.ID); err == nil {
		t.Fatal("expected dir creation error")
	}
}

func TestSchedulerRunNowRetentionCleanup(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	dataDir := t.TempDir()
	sc := NewScheduler(s, cfg, dataDir, testExportFn)
	u := newTestUser(t, s)

	if err := cfg.Set(u.ID, "backup.retention_days", "0"); err != nil {
		t.Fatalf("Set retention: %v", err)
	}
	if err := sc.RunNow(u.ID); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
}

func TestSchedulerRunNowS3Failure(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	sc := NewScheduler(s, cfg, t.TempDir(), testExportFn)
	u := newTestUser(t, s)

	// upload_s3 开启但未配置 S3 → UploadToS3 失败 → s3Err 分支
	if err := cfg.Set(u.ID, "backup.upload_s3", true); err != nil {
		t.Fatalf("Set upload_s3: %v", err)
	}
	if err := sc.RunNow(u.ID); err == nil {
		t.Fatal("expected S3 upload error")
	}
}

func TestSchedulerNextBackupTime(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	sc := NewScheduler(s, cfg, t.TempDir(), testExportFn)
	u := newTestUser(t, s)

	// 未显式配置 frequency → 注册默认值为 "daily"，应返回未来时间
	if sc.nextBackupTime(u.ID).IsZero() {
		t.Fatal("expected default (daily) next time without explicit frequency")
	}

	// daily
	if err := cfg.Set(u.ID, "backup.frequency", "daily"); err != nil {
		t.Fatalf("Set frequency: %v", err)
	}
	next := sc.nextBackupTime(u.ID)
	if !next.After(time.Now()) {
		t.Fatalf("daily next = %v, want in future", next)
	}

	// weekly
	if err := cfg.Set(u.ID, "backup.frequency", "weekly"); err != nil {
		t.Fatalf("Set frequency: %v", err)
	}
	if err := cfg.Set(u.ID, "backup.day_of_week", "1"); err != nil {
		t.Fatalf("Set dow: %v", err)
	}
	next = sc.nextBackupTime(u.ID)
	if !next.After(time.Now()) {
		t.Fatalf("weekly next = %v, want in future", next)
	}

	// monthly
	if err := cfg.Set(u.ID, "backup.frequency", "monthly"); err != nil {
		t.Fatalf("Set frequency: %v", err)
	}
	if err := cfg.Set(u.ID, "backup.day_of_month", "28"); err != nil {
		t.Fatalf("Set dom: %v", err)
	}
	next = sc.nextBackupTime(u.ID)
	if !next.After(time.Now()) {
		t.Fatalf("monthly next = %v, want in future", next)
	}
}

func TestParseTime(t *testing.T) {
	cases := []struct{ in string; h, m int }{
		{"00:00", 0, 0},
		{"07:30", 7, 30},
		{"23:59", 23, 59},
		{"12", 12, 0},
		{"abc", 0, 0},
		{"", 0, 0},
		{"1:2:3", 1, 2},
	}
	for _, c := range cases {
		h, m := parseTime(c.in)
		if h != c.h || m != c.m {
			t.Errorf("parseTime(%q) = %d:%d, want %d:%d", c.in, h, m, c.h, c.m)
		}
	}
}