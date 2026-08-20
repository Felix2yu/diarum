package backup

import (
	"testing"

	"github.com/songtianlun/diarum/internal/config"
)

// TestSchedulerRefreshCancelExisting ensures Refresh stops a previously-scheduled
// timer for the same user before installing a new one.
func TestSchedulerRefreshCancelExisting(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	sc := NewScheduler(s, cfg, t.TempDir(), testExportFn)
	u := newTestUser(t, s)

	if err := cfg.Set(u.ID, "backup.enabled", true); err != nil {
		t.Fatalf("Set enabled: %v", err)
	}

	sc.Refresh(u.ID)
	t1, ok := sc.timer.Timer(u.ID)
	if !ok {
		t.Fatal("timer should be set after first Refresh")
	}

	sc.Refresh(u.ID)
	t2, ok := sc.timer.Timer(u.ID)
	if !ok {
		t.Fatal("timer should still exist after re-Refresh")
	}
	if t1 == t2 {
		t.Error("expected a fresh timer after re-Refresh (old one should be cancelled)")
	}
	sc.Stop()
}

// TestSchedulerStartWithEnabledUser exercises Start's loop that refreshes a timer
// for every user that has backup enabled.
func TestSchedulerStartWithEnabledUser(t *testing.T) {
	s := newTestStore(t)
	cfg := config.NewConfigService(s)
	sc := NewScheduler(s, cfg, t.TempDir(), testExportFn)
	u := newTestUser(t, s)

	if err := cfg.Set(u.ID, "backup.enabled", true); err != nil {
		t.Fatalf("Set enabled: %v", err)
	}

	sc.Start()
	if _, ok := sc.timer.Timer(u.ID); !ok {
		t.Fatal("expected a timer for the enabled user after Start")
	}
	sc.Stop()

	// Disabled user must not get a timer even when present at Start.
	sc2 := NewScheduler(s, cfg, t.TempDir(), testExportFn)
	if err := cfg.Set(u.ID, "backup.enabled", false); err != nil {
		t.Fatalf("Set disabled: %v", err)
	}
	sc2.Start()
	if _, ok := sc2.timer.Timer(u.ID); ok {
		t.Fatal("disabled user should not get a timer at Start")
	}
	sc2.Stop()
}
