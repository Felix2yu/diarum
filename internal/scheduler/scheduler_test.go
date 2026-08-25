package scheduler

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewTimerDefaults(t *testing.T) {
	var listed int
	tm := NewTimer(func() ([]string, error) {
		listed++
		return []string{"u1"}, nil
	})
	if tm.userTimers == nil {
		t.Fatal("userTimers map should be initialized")
	}
	if tm.now == nil {
		t.Fatal("now should default to a clock function")
	}
	if tm.Len() != 0 {
		t.Fatalf("new timer should be empty, got %d", tm.Len())
	}
	if listed != 0 {
		t.Fatalf("listUsers should not be called before Start, calls = %d", listed)
	}
}

func TestWithNowOverridesClock(t *testing.T) {
	fixed := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	called := false
	tm := NewTimer(func() ([]string, error) { return nil, nil }, WithNow(func() time.Time {
		called = true
		return fixed
	}))
	tm.Next = func(string) time.Time { return fixed.Add(time.Hour) }
	tm.Refresh("u1")
	if !called {
		t.Fatal("injected clock was not used by Refresh")
	}
	if tm.Len() != 1 {
		t.Fatalf("timer count = %d, want 1", tm.Len())
	}
	tm.Stop()
}

func TestStartSchedulesAllUsers(t *testing.T) {
	tm := NewTimer(func() ([]string, error) { return []string{"u1", "u2"}, nil })
	tm.Enabled = func(string) bool { return true }
	tm.Next = func(string) time.Time { return time.Now().Add(time.Hour) }
	tm.Start()
	if tm.Len() != 2 {
		t.Fatalf("timer count after Start = %d, want 2", tm.Len())
	}
	if _, ok := tm.Timer("u1"); !ok {
		t.Fatal("u1 should have an active timer")
	}
	if _, ok := tm.Timer("u2"); !ok {
		t.Fatal("u2 should have an active timer")
	}
	tm.Stop()
}

func TestStartListUsersError(t *testing.T) {
	tm := NewTimer(func() ([]string, error) { return nil, errors.New("boom") })
	tm.Enabled = func(string) bool { return true }
	tm.Next = func(string) time.Time { return time.Now().Add(time.Hour) }
	tm.Start()
	if tm.Len() != 0 {
		t.Fatalf("no timers should exist when listing users fails, got %d", tm.Len())
	}
}

func TestRefreshNilEnabledSchedules(t *testing.T) {
	tm := NewTimer(func() ([]string, error) { return nil, nil })
	tm.Next = func(string) time.Time { return time.Now().Add(time.Hour) }
	tm.Refresh("u1")
	if tm.Len() != 1 {
		t.Fatalf("nil Enabled should schedule, timers = %d", tm.Len())
	}
	tm.Stop()
}

func TestRefreshDisabledSkips(t *testing.T) {
	tm := NewTimer(func() ([]string, error) { return nil, nil })
	tm.Enabled = func(userID string) bool { return userID == "enabled" }
	tm.Next = func(string) time.Time { return time.Now().Add(time.Hour) }

	tm.Refresh("disabled-user")
	if _, ok := tm.Timer("disabled-user"); ok {
		t.Fatal("disabled user should not get a timer")
	}

	tm.Refresh("enabled")
	if tm.Len() != 1 {
		t.Fatalf("enabled user should have exactly one timer, got %d", tm.Len())
	}
	tm.Stop()
}

func TestRefreshZeroNextSkips(t *testing.T) {
	tm := NewTimer(func() ([]string, error) { return nil, nil })
	tm.Enabled = func(string) bool { return true }
	tm.Next = func(string) time.Time { return time.Time{} }
	tm.Refresh("u1")
	if tm.Len() != 0 {
		t.Fatalf("zero Next should not schedule, timers = %d", tm.Len())
	}
}

func TestRefreshPastDueRunsImmediately(t *testing.T) {
	runCalled := make(chan string, 1)
	tm := NewTimer(func() ([]string, error) { return nil, nil })
	tm.Enabled = func(string) bool { return true }
	tm.Next = func(string) time.Time { return time.Now().Add(-time.Minute) }
	tm.Run = func(userID string) { runCalled <- userID }

	tm.Refresh("u1")

	select {
	case id := <-runCalled:
		if id != "u1" {
			t.Fatalf("run received %q, want u1", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("past-due job was not run")
	}
	tm.Stop()
}

func TestRefreshReplacesExistingTimer(t *testing.T) {
	tm := NewTimer(func() ([]string, error) { return nil, nil })
	tm.Enabled = func(string) bool { return true }
	tm.Next = func(string) time.Time { return time.Now().Add(time.Hour) }

	tm.Refresh("u1")
	first, ok := tm.Timer("u1")
	if !ok {
		t.Fatal("first refresh did not create a timer")
	}

	tm.Refresh("u1")
	second, ok := tm.Timer("u1")
	if !ok {
		t.Fatal("second refresh lost the timer")
	}
	if first == second {
		t.Fatal("refresh should replace the timer with a new one")
	}
	if tm.Len() != 1 {
		t.Fatalf("refresh should keep exactly one timer, got %d", tm.Len())
	}
	tm.Stop()
}

func TestStopCancelsAllTimers(t *testing.T) {
	var runs atomic.Int32
	tm := NewTimer(func() ([]string, error) { return nil, nil })
	tm.Enabled = func(string) bool { return true }
	tm.Next = func(string) time.Time { return time.Now().Add(50 * time.Millisecond) }
	tm.Run = func(string) { runs.Add(1) }

	tm.Refresh("u1")
	tm.Refresh("u2")
	tm.Stop()

	if tm.Len() != 0 {
		t.Fatalf("stop should clear all timers, got %d", tm.Len())
	}
	time.Sleep(150 * time.Millisecond)
	if runs.Load() != 0 {
		t.Fatalf("stopped timers should never fire, runs = %d", runs.Load())
	}
}

func TestInjectReplacesTimer(t *testing.T) {
	tm := NewTimer(func() ([]string, error) { return nil, nil })
	tm.Enabled = func(string) bool { return true }
	tm.Next = func(string) time.Time { return time.Now().Add(time.Hour) }

	tm.Refresh("u1")
	old, _ := tm.Timer("u1")

	injected := time.AfterFunc(time.Hour, func() {})
	tm.Inject("u1", injected)

	got, ok := tm.Timer("u1")
	if !ok || got != injected {
		t.Fatal("inject should replace the active timer")
	}
	if old == nil {
		t.Fatal("previous timer should have existed")
	}

	tm.Inject("fresh", injected)
	if _, ok := tm.Timer("fresh"); !ok {
		t.Fatal("inject should add timers for unknown users")
	}
	tm.Stop()
}
