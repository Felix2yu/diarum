package push

import (
	"strconv"
	"strings"
	"time"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/scheduler"
	"github.com/songtianlun/diarum/internal/store"
)

// Scheduler manages per-user reminder timers that send a push notification at
// the configured daily time, skipping days that already have a written diary.
type Scheduler struct {
	store         *store.Store
	configService *config.ConfigService
	sender        *Sender

	now   func() time.Time
	timer *scheduler.Timer
}

// NewScheduler creates a push reminder scheduler.
func NewScheduler(s *store.Store, cfg *config.ConfigService, sender *Sender) *Scheduler {
	sc := &Scheduler{
		store:         s,
		configService: cfg,
		sender:        sender,
		now:           time.Now,
	}
	sc.timer = scheduler.NewTimer(sc.listUserIDs, scheduler.WithNow(sc.now))
	sc.timer.Enabled = func(userID string) bool {
		enabled, _ := sc.configService.GetBool(userID, "webpush.enabled")
		return enabled
	}
	sc.timer.Next = sc.nextNotifyTime
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

// Start initializes timers for all users with reminders enabled.
func (sc *Scheduler) Start() {
	if err := sc.sender.EnsureVAPIDKeys(); err != nil {
		logger.Error("[Push] failed to ensure VAPID keys: %v", err)
		return
	}
	users, err := sc.store.ListUsers()
	if err != nil {
		logger.Error("[Push] failed to list users: %v", err)
		return
	}
	for _, u := range users {
		sc.Refresh(u.ID)
	}
	logger.Info("[Push] reminder scheduler started for %d users", len(users))
}

// Stop cancels all timers.
func (sc *Scheduler) Stop() {
	sc.timer.Stop()
}

// Refresh recalculates and resets the timer for a user.
func (sc *Scheduler) Refresh(userID string) {
	sc.timer.Refresh(userID)
}

// RunNow triggers an immediate reminder for a user (used by the test endpoint).
func (sc *Scheduler) RunNow(userID string) error {
	return sc.execute(userID)
}

func (sc *Scheduler) execute(userID string) error {
	// Skip when today's diary has already been written (in the user's timezone).
	loc := sc.location(userID)
	today := sc.now().In(loc).Format(time.DateOnly)
	written, err := sc.store.HasDiaryContent(userID, today)
	if err != nil {
		logger.Error("[Push] failed to check diary for user %s: %v", userID, err)
	}
	if written {
		logger.Info("[Push] skipping reminder for %s: diary for %s already written", userID, today)
		sc.Refresh(userID)
		return nil
	}

	message, _ := sc.configService.GetString(userID, "webpush.message")
	if message == "" {
		message = "该写今天的日记啦 ✍️"
	}

	logger.Info("[Push] sending reminder to user %s", userID)
	if err := sc.sender.SendNotification(userID, "吾身 · 日记提醒", message); err != nil {
		logger.Error("[Push] failed to send reminder for user %s: %v", userID, err)
	}

	// Schedule the next daily reminder.
	sc.Refresh(userID)
	return nil
}

// nextNotifyTime computes the next daily reminder instant in the user's timezone.
func (sc *Scheduler) nextNotifyTime(userID string) time.Time {
	timeStr, _ := sc.configService.GetString(userID, "webpush.time")
	hour, minute := parseTime(timeStr)

	loc := sc.location(userID)
	now := sc.now().In(loc)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// location resolves the user's configured IANA timezone (falls back to local).
func (sc *Scheduler) location(userID string) *time.Location {
	tz, _ := sc.configService.GetString(userID, "webpush.tz")
	if tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return time.Local
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
