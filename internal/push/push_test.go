package push

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SherClockHolmes/webpush-go"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
)

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

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
	if _, ok := sc.timer.Timer(u.ID); ok {
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
		{"origin preferred", "https://diarum.example.com", "localhost:1323", "", "webpush@diarum.example.com"},
		{"host fallback", "", "diarum.example.com", "", "webpush@diarum.example.com"},
		{"origin localhost rejected, host used", "https://localhost:1323", "diarum.example.com", "", "webpush@diarum.example.com"},
		{"ip rejected, host used", "", "192.168.1.5", "", SubscriberEmail},
		{"localhost rejected", "", "localhost", "", SubscriberEmail},
		{"override mailto stripped", "https://a.example.com", "b.example.com", "mailto:admin@diarum.app", "admin@diarum.app"},
		{"override bare email kept", "", "", "admin@diarum.app", "admin@diarum.app"},
		{"override https kept", "", "", "https://diarum.yufei.im", "https://diarum.yufei.im"},
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

func TestSetSiteFromRequest(t *testing.T) {
	t.Cleanup(func() { SiteHost, SiteOrigin = "", "" })

	// Origin wins.
	SetSiteFromRequest("https://diarum.example.com", "", "", "", "localhost:1323")
	if SiteHost != "diarum.example.com" || SiteOrigin != "https://diarum.example.com" {
		t.Errorf("origin case: host=%q origin=%q", SiteHost, SiteOrigin)
	}

	// No Origin, but X-Forwarded-Host/Proto from a reverse proxy.
	SetSiteFromRequest("", "", "https", "diarum.example.com", "internal:1323")
	if SiteHost != "diarum.example.com" || SiteOrigin != "https://diarum.example.com" {
		t.Errorf("forwarded case: host=%q origin=%q", SiteHost, SiteOrigin)
	}

	// Referer fallback.
	SetSiteFromRequest("", "https://diarum.example.com/settings", "", "", "localhost")
	if SiteHost != "diarum.example.com" {
		t.Errorf("referer case: host=%q", SiteHost)
	}

	// Raw host fallback; non-public hosts do not set SiteOrigin.
	SetSiteFromRequest("", "", "", "", "diarum:1323")
	if SiteHost != "diarum" {
		t.Errorf("raw host case: host=%q", SiteHost)
	}

	// Everything empty.
	SetSiteFromRequest("", "", "", "", "")
	if SiteHost != "" {
		t.Errorf("empty case: host=%q", SiteHost)
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

func TestNoDoubleMailtoPrefix(t *testing.T) {
	// Regression: webpush-go prepends "mailto:" to bare e-mail subjects, so the
	// exact subject passed must not already carry the prefix (which produced
	// "mailto:mailto:..." and a 403 BadJwtToken from Apple).
	_, u, _, sender, _ := newHarness(t)
	t.Cleanup(func() { SiteHost, SiteOrigin, SubscriberOverride = "", "", "" })
	SiteOrigin = "https://diarum.yufei.im"
	SubscriberOverride = "mailto:webpush@diarum.yufei.im"

	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	if err := sender.store.SavePushSubscription(u.ID, srv.URL, "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM", "PTFm9pC-W6LdLpqR8BOaKg"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := sender.SendNotification(u.ID, "t", "b"); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Extract the JWT (t parameter) and decode its payload.
	i := strings.Index(auth, "t=")
	if i < 0 {
		t.Fatalf("no t= in authorization: %q", auth)
	}
	rest := auth[i+2:]
	if j := strings.Index(rest, ","); j >= 0 {
		rest = rest[:j]
	}
	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		t.Fatalf("malformed jwt: %q", rest)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims struct {
		Sub string `json:"sub"`
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := strings.Count(claims.Sub, "mailto:"); got != 1 {
		t.Errorf("sub %q has %d mailto: prefixes, want exactly 1", claims.Sub, got)
	}
	if claims.Sub != "mailto:webpush@diarum.yufei.im" {
		t.Errorf("sub=%q", claims.Sub)
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

func TestSendNoTopicForApple(t *testing.T) {
	_, u, _, sender, _ := newHarness(t)
	if err := sender.EnsureVAPIDKeys(); err != nil {
		t.Fatalf("keys: %v", err)
	}
	t.Cleanup(func() { SiteHost = "" })
	SiteHost = "diarum.example.com"

	var gotTopic string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTopic = r.Header.Get("Topic")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	// Route the Apple-style endpoint through a transport that dials our
	// local test server so the headers can be asserted.
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

	// Regression: Apple rejects a dotted-hostname Topic with 400 BadWebPushTopic,
	// so we must NOT send a Topic header at all.
	if err := sender.SendNotificationWithClient(u.ID, "t", "b", &http.Client{Transport: transport}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotTopic != "" {
		t.Fatalf("Topic=%q want empty (BadWebPushTopic regression)", gotTopic)
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
	if _, _, err := s.UpsertDiary(u.ID, "2026-03-10", "已写", intPtr(4), nil, nil, nil, nil, nil, nil, nil); err != nil {
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
	if _, ok := sc.timer.Timer(u.ID); !ok {
		t.Fatalf("no timer after Start")
	}
	sc.Stop()
	if _, ok := sc.timer.Timer(u.ID); ok {
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

func TestEnsureVAPIDKeys_PreExisting(t *testing.T) {
	s, _, _, sender, _ := newHarness(t)

	// Pre-populate both keys in the store
	if err := s.SetVAPIDKey("public", "stored-public-key"); err != nil {
		t.Fatalf("set public: %v", err)
	}
	if err := s.SetVAPIDKey("private", "stored-private-key"); err != nil {
		t.Fatalf("set private: %v", err)
	}

	if err := sender.EnsureVAPIDKeys(); err != nil {
		t.Fatalf("EnsureVAPIDKeys pre-existing: %v", err)
	}
	if sender.pubKey != "stored-public-key" || sender.privKey != "stored-private-key" {
		t.Fatalf("keys not loaded: pub=%q priv=%q", sender.pubKey, sender.privKey)
	}

	// Second call should short-circuit via s.loaded
	if err := sender.EnsureVAPIDKeys(); err != nil {
		t.Fatalf("second EnsureVAPIDKeys: %v", err)
	}
}

func TestEnsureVAPIDKeys_MissingPrivate(t *testing.T) {
	s, _, _, sender, _ := newHarness(t)

	if err := s.SetVAPIDKey("public", "orphan-public"); err != nil {
		t.Fatalf("set public: %v", err)
	}

	err := sender.EnsureVAPIDKeys()
	if err == nil || !strings.Contains(err.Error(), "private key is missing") {
		t.Fatalf("expected missing private key error, got: %v", err)
	}
}

func TestPushSchedulerStart_WithUsers(t *testing.T) {
	s, u, cfg, _, sc := newHarness(t)
	_ = cfg.Set(u.ID, "webpush.enabled", true)

	// Start should ensure VAPID keys and register timers
	sc.Start()

	// VAPID keys should now exist
	pub, err := s.GetVAPIDKey("public")
	if err != nil || pub == "" {
		t.Fatalf("VAPID public key should be set: pub=%q err=%v", pub, err)
	}
}
