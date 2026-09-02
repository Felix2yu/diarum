package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
)

// mockWeatherTransportError returns a 200 with invalid JSON for the weather
// fetch hosts so provider fetch fails with a decode error (no retry/sleep),
// while geocoding still succeeds so the route error branch is reached quickly.
func mockWeatherTransportError(t *testing.T) {
	t.Helper()
	withMockTransport(t, func(req *http.Request) (*http.Response, error) {
		host := req.URL.Host
		switch {
		case strings.Contains(host, "geocoding-api.open-meteo.com"):
			return httpResponse(http.StatusOK, `{"results":[{"name":"Beijing","latitude":39.9042,"longitude":116.4074,"country":"China","admin1":"Beijing"}]}`), nil
		default:
			// qweather + open-meteo weather fetch: 200 but garbage → decode error, no retry.
			return httpResponse(http.StatusOK, `not-json`), nil
		}
	})
}

func TestWeatherGetErrorPath(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterWeatherRoutes(e, s, authMiddlewareFor(user))
	mockWeatherTransportError(t)

	rec := performRequest(t, e, "GET", "/api/v1/weather?city=Beijing", nil, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET /weather error status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWeatherCoordsErrorPath(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterWeatherRoutes(e, s, authMiddlewareFor(user))
	mockWeatherTransportError(t)

	rec := performRequest(t, e, "GET", "/api/v1/weather/coords?city=Beijing&lat=39.9&lon=116.4", nil, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET /weather/coords error status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWeatherCitiesMissingQuery(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterWeatherRoutes(e, s, authMiddlewareFor(user))
	mockWeatherTransport(t)

	// Already covered by weather_routes_test.go for success; verify the
	// empty-query branch explicitly returns 400 with the right shape.
	rec := performRequest(t, e, "GET", "/api/v1/weather/cities", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("cities missing q status = %d", rec.Code)
	}
}

func setupBackfillUser(t *testing.T, s *store.Store, user *store.User) {
	t.Helper()
	cfg := config.NewConfigService(s)
	if err := cfg.Set(user.ID, "weather.default_city", "Beijing"); err != nil {
		t.Fatalf("set default city: %v", err)
	}
}

// createDiaryOnDate inserts a diary with explicit date + weather for backfill.
func createDiaryOnDate(t *testing.T, s *store.Store, user *store.User, date, weather, content string) {
	t.Helper()
	if _, err := s.InsertImportedDiary(user.ID, "", date, content, 0, nil, nil, weather, nil, "", 0, 0); err != nil {
		t.Fatalf("InsertImportedDiary: %v", err)
	}
}

func TestWeatherBackfillSkipValidWeather(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterWeatherRoutes(e, s, authMiddlewareFor(user))
	mockWeatherTransport(t)
	setupBackfillUser(t, s, user)

	past := time.Now().AddDate(0, 0, -5).Format("2006-01-02")
	createDiaryOnDate(t, s, user, past, "12", "has weather already")

	body, _ := json.Marshal(map[string]any{"start_date": past})
	rec := performRequest(t, e, "POST", "/api/v1/weather/backfill", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("backfill status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "event: skipped") {
		t.Errorf("expected skipped event, body=%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "已有有效天气代码") {
		t.Errorf("expected valid-weather skip reason, body=%s", rec.Body.String())
	}
}

func TestWeatherBackfillSkipFutureDate(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterWeatherRoutes(e, s, authMiddlewareFor(user))
	mockWeatherTransport(t)
	setupBackfillUser(t, s, user)

	future := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	createDiaryOnDate(t, s, user, future, "", "future diary")

	body, _ := json.Marshal(map[string]any{"start_date": future})
	rec := performRequest(t, e, "POST", "/api/v1/weather/backfill", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("backfill status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "未来日期") {
		t.Errorf("expected future-date skip reason, body=%s", rec.Body.String())
	}
}

func TestWeatherBackfillSkipEmptyContent(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterWeatherRoutes(e, s, authMiddlewareFor(user))
	mockWeatherTransport(t)
	setupBackfillUser(t, s, user)

	past := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	createDiaryOnDate(t, s, user, past, "", "")

	body, _ := json.Marshal(map[string]any{"start_date": past, "skip_empty": true})
	rec := performRequest(t, e, "POST", "/api/v1/weather/backfill", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("backfill status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "无内容") {
		t.Errorf("expected empty-content skip reason, body=%s", rec.Body.String())
	}
}

func TestWeatherBackfillOverwriteOldEmoji(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterWeatherRoutes(e, s, authMiddlewareFor(user))
	mockWeatherTransport(t)
	setupBackfillUser(t, s, user)

	past := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	// Old emoji weather ("☀️") is not a numeric WMO code → should be overwritten.
	createDiaryOnDate(t, s, user, past, "☀️", "sunny day")

	body, _ := json.Marshal(map[string]any{"start_date": past})
	rec := performRequest(t, e, "POST", "/api/v1/weather/backfill", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("backfill status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "event: updated") {
		t.Errorf("expected updated event, body=%s", rec.Body.String())
	}
}
