package push

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
)

func newHarness(t *testing.T) (*store.Store, *store.User, *config.ConfigService, *Sender, *Scheduler) {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	u, err := s.CreateUser("tester", "t@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	cfg := config.NewConfigService(s)
	sender := NewSender(s)
	return s, u, cfg, sender, NewScheduler(s, cfg, sender)
}

func TestParseTime(t *testing.T) {
	for _, c := range []struct {
		in   string
		h, m int
	}{
		{"", 0, 0}, {"21", 21, 0}, {"21:30", 21, 30}, {"07:05", 7, 5}, {"nope", 0, 0},
	} {
		h, m := parseTime(c.in)
		if h != c.h || m != c.m {
			t.Errorf("parseTime(%q)=%d:%d want %d:%d", c.in, h, m, c.h, c.m)
		}
	}
}

func TestNextNotifyTimeSameDay(t *testing.T) {
	_, u, cfg, _, sc := newHarness(t)
	now := time.Date(2026, 3, 10, 20, 0, 0, 0, time.UTC)
	sc.now = func() time.Time { return now }
	_ = cfg.Set(u.ID, "webpush.time", "21:30")
	_ = cfg.Set(u.ID, "webpush.tz", "UTC")
	got := sc.nextNotifyTime(u.ID)
	want := time.Date(2026, 3, 10, 21, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("next=%s want %s", got, want)
	}
}

func TestNextNotifyTimeTomorrowWhenPast(t *testing.T) {
	_, u, cfg, _, sc := newHarness(t)
	now := time.Date(2026, 3, 10, 22, 30, 0, 0, time.UTC)
	sc.now = func() time.Time { return now }
	_ = cfg.Set(u.ID, "webpush.time", "21:30")
	_ = cfg.Set(u.ID, "webpush.tz", "UTC")
	got := sc.nextNotifyTime(u.ID)
	want := time.Date(2026, 3, 11, 21, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("next=%s want %s", got, want)
	}
}

func TestRefreshNoTimerWhenDisabled(t *testing.T) {
	_, u, _, _, sc := newHarness(t)
	sc.Refresh(u.ID)
	if _, ok := sc.userTimers[u.ID]; ok {
		t.Fatalf("timer exists when reminder disabled")
	}
}

func TestVAPIDGeneratedAndPersisted(t *testing.T) {
	s, _, _, sender, _ := newHarness(t)
	pub, err := sender.PublicKey()
	if err != nil || pub == "" {
		t.Fatalf("public key err=%v pub=%q", err, pub)
	}
	priv, err := s.GetVAPIDKey("private")
	if err != nil || priv == "" {
		t.Fatalf("private key not persisted: err=%v", err)
	}
}

func TestSendRemovesGoneSubscription(t *testing.T) {
	_, u, _, sender, _ := newHarness(t)
	if err := sender.EnsureVAPIDKeys(); err != nil {
		t.Fatalf("keys: %v", err)
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()
	if err := sender.store.SavePushSubscription(u.ID, srv.URL, "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM", "PTFm9pC-W6LdLpqR8BOaKg"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := sender.SendNotification(u.ID, "t", "b"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d want 1", calls)
	}
	subs, _ := sender.store.ListPushSubscriptions(u.ID)
	if len(subs) != 0 {
		t.Fatalf("gone sub not removed")
	}
}
func TestSendSuccess(t *testing.T) {
	_, u, _, sender, _ := newHarness(t)
	if err := sender.EnsureVAPIDKeys(); err != nil {
		t.Fatalf("keys: %v", err)
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	if err := sender.store.SavePushSubscription(u.ID, srv.URL, "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM", "PTFm9pC-W6LdLpqR8BOaKg"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := sender.SendNotification(u.ID, "t", "b"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d want 1", calls)
	}
}

func TestExecuteSkipsWhenDiaryWritten(t *testing.T) {
	s, u, cfg, sender, sc := newHarness(t)
	if err := sender.EnsureVAPIDKeys(); err != nil {
		t.Fatalf("keys: %v", err)
	}
	fixed := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	sc.now = func() time.Time { return fixed }
	_ = cfg.Set(u.ID, "webpush.enabled", true)
	_ = cfg.Set(u.ID, "webpush.time", "21:30")
	_ = cfg.Set(u.ID, "webpush.tz", "UTC")
	_ = cfg.Set(u.ID, "webpush.message", "写日记")

	// write today's diary directly through the store
	if _, _, err := s.UpsertDiary(u.ID, "2026-03-10", "已写", 4, nil, nil, "", nil, "", 0, 0); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if err := sender.store.SavePushSubscription(u.ID, srv.URL, "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM", "PTFm9pC-W6LdLpqR8BOaKg"); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := sc.RunNow(u.ID); err != nil {
		t.Fatalf("run now: %v", err)
	}
	if calls != 0 {
		t.Fatalf("push sent despite diary written (calls=%d)", calls)
	}
}

func TestStartAndStop(t *testing.T) {
	_, u, cfg, _, sc := newHarness(t)
	_ = cfg.Set(u.ID, "webpush.enabled", true)
	_ = cfg.Set(u.ID, "webpush.time", "21:00")
	_ = cfg.Set(u.ID, "webpush.tz", "UTC")
	sc.Start()
	if _, ok := sc.userTimers[u.ID]; !ok {
		t.Fatalf("no timer after Start")
	}
	sc.Stop()
	if _, ok := sc.userTimers[u.ID]; ok {
		t.Fatalf("timer remains after Stop")
	}
}

func TestExecuteSendsWhenNotWritten(t *testing.T) {
	_, u, cfg, sender, sc := newHarness(t)
	if err := sender.EnsureVAPIDKeys(); err != nil {
		t.Fatalf("keys: %v", err)
	}
	fixed := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	sc.now = func() time.Time { return fixed }
	_ = cfg.Set(u.ID, "webpush.enabled", true)
	_ = cfg.Set(u.ID, "webpush.time", "21:30")
	_ = cfg.Set(u.ID, "webpush.tz", "UTC")

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	if err := sender.store.SavePushSubscription(u.ID, srv.URL, "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM", "PTFm9pC-W6LdLpqR8BOaKg"); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := sc.RunNow(u.ID); err != nil {
		t.Fatalf("run now: %v", err)
	}
	if calls != 1 {
		t.Fatalf("push calls=%d want 1", calls)
	}
}
