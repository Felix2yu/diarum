package push

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SherClockHolmes/webpush-go"
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

func TestNormalizeHost(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"diarum.example.com", "diarum.example.com"},
		{"diarum.example.com:443", "diarum.example.com"},
		{"EXAMPLE.com:8443", "example.com"},
		{"", ""},
		{"  192.168.1.5:8080  ", "192.168.1.5"},
	} {
		if got := NormalizeHost(c.in); got != c.want {
			t.Errorf("NormalizeHost(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestSubscriberResolution(t *testing.T) {
	_, _, _, sender, _ := newHarness(t)
	t.Cleanup(func() {
		SiteHost, SiteOrigin, SubscriberOverride = "", "", ""
	})

	cases := []struct {
		name     string
		origin   string
		host     string
		override string
		want     string
	}{
		{"origin preferred", "https://diarum.example.com", "localhost:1323", "", "mailto:webpush@diarum.example.com"},
		{"host fallback", "", "diarum.example.com", "", "mailto:webpush@diarum.example.com"},
		{"origin localhost rejected, host used", "https://localhost:1323", "diarum.example.com", "", "mailto:webpush@diarum.example.com"},
		{"ip rejected, host used", "", "192.168.1.5", "", SubscriberEmail},
		{"localhost rejected", "", "localhost", "", SubscriberEmail},
		{"override wins", "https://a.example.com", "b.example.com", "mailto:admin@diarum.app", "mailto:admin@diarum.app"},
		{"empty all", "", "", "", SubscriberEmail},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			SiteOrigin, SiteHost, SubscriberOverride = c.origin, c.host, c.override
			if got := sender.subscriber(); got != c.want {
				t.Errorf("subscriber()=%q want %q", got, c.want)
			}
		})
	}
}

func TestOriginHost(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"https://diarum.example.com", "diarum.example.com"},
		{"https://diarum.example.com:8443", "diarum.example.com"},
		{"https://localhost:1323", "localhost"},
		{"not a url", ""},
		{"", ""},
	} {
		if got := OriginHost(c.in); got != c.want {
			t.Errorf("OriginHost(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestIsPublicHost(t *testing.T) {
	for _, c := range []struct {
		in   string
		want bool
	}{
		{"diarum.example.com", true},
		{"", false},
		{"localhost", false},
		{"127.0.0.1", false},
		{"192.168.1.5", false},
		{"myhost.local", false},
		{"myhost.lan", false},
	} {
		if got := isPublicHost(c.in); got != c.want {
			t.Errorf("isPublicHost(%q)=%v want %v", c.in, got, c.want)
		}
	}
}

func TestSendWithoutExplicitKeyLoad(t *testing.T) {
	// A fresh Sender that has NOT had EnsureVAPIDKeys called must still send:
	// keys are loaded lazily inside SendNotification. Regression for
	// "ecdsa: private key scalar is zero or negative".
	s, u, err := func() (*store.Store, *store.User, error) {
		st, err := store.Open(t.TempDir())
		if err != nil {
			return nil, nil, err
		}
		us, err := st.CreateUser("tester", "t@example.com", "hash")
		return st, us, err
	}()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	sender := NewSender(s)
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
	if !sender.loaded {
		t.Fatalf("scheduler not marked loaded after send")
	}
}

func TestSendSetsTopicForApple(t *testing.T) {
	_, u, _, sender, _ := newHarness(t)
	if err := sender.EnsureVAPIDKeys(); err != nil {
		t.Fatalf("keys: %v", err)
	}
	SiteHost = "diarum.example.com"
	defer func() { SiteHost = "" }()

	var gotTopic string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTopic = r.Header.Get("Topic")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	// Route the Apple-style endpoint through a transport that dials our
	// local test server so the Topic header can be asserted.
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial(network, net.JoinHostPort("127.0.0.1", port))
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	sub := webpush.Subscription{
		Endpoint: "https://web.push.apple.com/ab8c-def0",
		Keys: webpush.Keys{
			Auth:   "PTFm9pC-W6LdLpqR8BOaKg",
			P256dh: "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM",
		},
	}
	if err := sender.store.SavePushSubscription(u.ID, sub.Endpoint, sub.Keys.P256dh, sub.Keys.Auth); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Send with a sender that shares keys but a custom client+options path:
	// reuse SendNotification but inject the transport via a direct send.
	if err := sender.SendNotificationWithClient(u.ID, "t", "b", &http.Client{Transport: transport}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotTopic != "diarum.example.com" {
		t.Fatalf("Topic=%q want diarum.example.com", gotTopic)
	}
}

func TestSendNoTopicForNonApple(t *testing.T) {
	_, u, _, sender, _ := newHarness(t)
	if err := sender.EnsureVAPIDKeys(); err != nil {
		t.Fatalf("keys: %v", err)
	}
	var gotTopic string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTopic = r.Header.Get("Topic")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	if err := sender.store.SavePushSubscription(u.ID, srv.URL, "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM", "PTFm9pC-W6LdLpqR8BOaKg"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := sender.SendNotification(u.ID, "t", "b"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotTopic != "" {
		t.Fatalf("Topic=%q want empty for non-Apple endpoint", gotTopic)
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
