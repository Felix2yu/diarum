// Package scheduler provides a reusable per-user timer manager that backs the
// backup, weather and push schedulers. It owns the timer map and its lifecycle
// (Start/Stop/Refresh) while the per-user behavior is supplied through the
// Enabled, Next and Run callbacks, eliminating the near-identical duplicated
// code that previously lived in those three packages.
package scheduler

import (
	"sync"
	"time"
)

// Timer schedules recurring per-user callbacks. The per-user behavior is
// supplied via the Enabled, Next and Run callbacks; Timer owns the mutex and
// the timer map so callers never touch shared state directly.
type Timer struct {
	mu         sync.Mutex
	userTimers map[string]*time.Timer

	// Enabled reports whether scheduling is enabled for a user.
	Enabled func(userID string) bool
	// Next computes the next execution time for a user; a zero time means
	// "do not schedule anything".
	Next func(userID string) time.Time
	// Run executes the scheduled job for a user.
	Run func(userID string)

	listUsers func() ([]string, error)
	now       func() time.Time
}

// Option configures a Timer.
type Option func(*Timer)

// WithNow overrides the clock used to compute delays (mainly for tests).
func WithNow(now func() time.Time) Option {
	return func(t *Timer) { t.now = now }
}

// NewTimer creates a Timer. listUsers returns the set of user IDs to schedule on
// Start.
func NewTimer(listUsers func() ([]string, error), opts ...Option) *Timer {
	t := &Timer{
		userTimers: make(map[string]*time.Timer),
		listUsers:  listUsers,
		now:        time.Now,
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// Start (re)computes timers for every user.
func (t *Timer) Start() {
	users, err := t.listUsers()
	if err != nil {
		return
	}
	for _, u := range users {
		t.Refresh(u)
	}
}

// Stop cancels all timers and clears the map.
func (t *Timer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, timer := range t.userTimers {
		timer.Stop()
	}
	clear(t.userTimers)
}

// Refresh recalculates and resets the timer for a single user.
func (t *Timer) Refresh(userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if old, ok := t.userTimers[userID]; ok {
		old.Stop()
		delete(t.userTimers, userID)
	}

	if t.Enabled != nil && !t.Enabled(userID) {
		return
	}

	next := t.Next(userID)
	if next.IsZero() {
		return
	}

	delay := max(next.Sub(t.now()), 0)
	t.userTimers[userID] = time.AfterFunc(delay, func() {
		t.Run(userID)
	})
}

// Timer returns the active timer for userID and whether one exists.
func (t *Timer) Timer(userID string) (*time.Timer, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	timer, ok := t.userTimers[userID]
	return timer, ok
}

// Len returns the number of active timers.
func (t *Timer) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.userTimers)
}

// Inject replaces the timer for userID. It is used by tests to install fake
// timers without going through Refresh.
func (t *Timer) Inject(userID string, timer *time.Timer) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if old, ok := t.userTimers[userID]; ok {
		old.Stop()
	}
	t.userTimers[userID] = timer
}
