package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/push"
	"github.com/songtianlun/diarum/internal/store"
)

func newPushRouter(t *testing.T, s *store.Store, u *store.User) (*echo.Echo, *push.Scheduler) {
	t.Helper()
	e := echo.New()
	cfg := config.NewConfigService(s)
	sender := push.NewSender(s)
	sc := push.NewScheduler(s, cfg, sender)
	RegisterPushRoutes(e, s, authMiddlewareFor(u), sc)
	return e, sc
}

const validSubJSON = `{"endpoint":"https://push.example/x","keys":{"p256dh":"BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM","auth":"PTFm9pC-W6LdLpqR8BOaKg"}}`

func TestPushVapidPublicKey(t *testing.T) {
	s := newTestStore(t)
	u := newTestUser(t, s)
	e, _ := newPushRouter(t, s, u)

	rec := performRequest(t, e, http.MethodGet, "/api/v1/push/vapid-public-key", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	data := decodeJSONBody(t, rec)
	if key, _ := data["public_key"].(string); key == "" {
		t.Fatalf("public_key empty")
	}
}

func TestPushSubscribeDelete(t *testing.T) {
	s := newTestStore(t)
	u := newTestUser(t, s)
	e, _ := newPushRouter(t, s, u)

	rec := performRequest(t, e, http.MethodPost, "/api/v1/push/subscriptions", strings.NewReader(validSubJSON), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("subscribe status=%d body=%s", rec.Code, rec.Body.String())
	}

	subs, err := s.ListPushSubscriptions(u.ID)
	if err != nil || len(subs) != 1 {
		t.Fatalf("subs after save: %v, %v", len(subs), err)
	}

	rec = performRequest(t, e, http.MethodDelete, "/api/v1/push/subscriptions", strings.NewReader(`{"endpoint":"https://push.example/x"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status=%d", rec.Code)
	}
	subs, _ = s.ListPushSubscriptions(u.ID)
	if len(subs) != 0 {
		t.Fatalf("subs after delete = %d, want 0", len(subs))
	}
}

func TestPushSubscribeInvalid(t *testing.T) {
	s := newTestStore(t)
	u := newTestUser(t, s)
	e, _ := newPushRouter(t, s, u)

	rec := performRequest(t, e, http.MethodPost, "/api/v1/push/subscriptions", strings.NewReader(`{"endpoint":""}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rec.Code)
	}
}

func TestPushSchedule(t *testing.T) {
	s := newTestStore(t)
	u := newTestUser(t, s)
	e, _ := newPushRouter(t, s, u)

	body := `{"enabled":true,"time":"21:30","timezone":"Asia/Shanghai","message":"该写日记啦"}`
	rec := performRequest(t, e, http.MethodPost, "/api/v1/push/schedule", strings.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("schedule status=%d body=%s", rec.Code, rec.Body.String())
	}

	cfg := config.NewConfigService(s)
	enabled, _ := cfg.GetBool(u.ID, "webpush.enabled")
	if !enabled {
		t.Fatalf("webpush.enabled not persisted")
	}
	timeStr, _ := cfg.GetString(u.ID, "webpush.time")
	if timeStr != "21:30" {
		t.Fatalf("webpush.time=%q", timeStr)
	}
}

func TestPushTestNoSub(t *testing.T) {
	s := newTestStore(t)
	u := newTestUser(t, s)
	e, _ := newPushRouter(t, s, u)

	rec := performRequest(t, e, http.MethodPost, "/api/v1/push/test", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rec.Code)
	}
}

func TestPushTestWithSub(t *testing.T) {
	s := newTestStore(t)
	u := newTestUser(t, s)
	e, _ := newPushRouter(t, s, u)

	rec := performRequest(t, e, http.MethodPost, "/api/v1/push/subscriptions", strings.NewReader(validSubJSON), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("subscribe status=%d", rec.Code)
	}

	withMockTransport(t, func(r *http.Request) (*http.Response, error) {
		return httpResponse(http.StatusCreated, "{}"), nil
	})

	rec = performRequest(t, e, http.MethodPost, "/api/v1/push/test", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("test status=%d body=%s", rec.Code, rec.Body.String())
	}

	// confirm no leftover malformed body handling path
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	_ = io.Discard
}
