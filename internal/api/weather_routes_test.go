package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/config"
)

// mockWeatherTransport intercepts every external weather call. QWeather hosts are
// forced to 404 so the code deterministically falls back to the mocked Open-Meteo /
// Nominatim endpoints (no real network, no flakiness).
func mockWeatherTransport(t *testing.T) {
	t.Helper()
	withMockTransport(t, func(req *http.Request) (*http.Response, error) {
		host := req.URL.Host
		switch {
		case strings.Contains(host, "qweather.com"):
			return httpResponse(http.StatusNotFound, `{"code":"404"}`), nil
		case strings.Contains(host, "geocoding-api.open-meteo.com"):
			return httpResponse(http.StatusOK, `{"results":[{"name":"Beijing","latitude":39.9042,"longitude":116.4074,"country":"China","admin1":"Beijing"}]}`), nil
		case strings.Contains(host, "open-meteo.com"):
			return httpResponse(http.StatusOK, `{"daily":{"time":["2024-01-10"],"weather_code":[0],"temperature_2m_max":[20],"temperature_2m_min":[10]}}`), nil
		case strings.Contains(host, "nominatim.openstreetmap.org/reverse"):
			return httpResponse(http.StatusOK, `{"lat":"39.9042","lon":"116.4074","display_name":"北京市, 北京市, 中国","address":{"city":"北京市","state":"北京市","country":"中国"}}`), nil
		case strings.Contains(host, "nominatim.openstreetmap.org"):
			return httpResponse(http.StatusOK, `[{"lat":"39.9042","lon":"116.4074","display_name":"北京市, 北京市, 中国"}]`), nil
		default:
			return httpResponse(http.StatusNotFound, "not found"), nil
		}
	})
}

func TestWeatherProviderAndCitiesRoutes(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterWeatherRoutes(e, s, authMiddlewareFor(user))

	rec := performRequest(t, e, http.MethodGet, "/api/v1/weather/provider", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/weather/provider status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Missing query -> 400
	rec = performRequest(t, e, http.MethodGet, "/api/v1/weather/cities", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("/weather/cities missing q status = %d, want 400", rec.Code)
	}

	mockWeatherTransport(t)
	rec = performRequest(t, e, http.MethodGet, "/api/v1/weather/cities?q=Beijing", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/weather/cities status = %d body=%s", rec.Code, rec.Body.String())
	}
	var cities []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &cities); err != nil || len(cities) == 0 {
		t.Fatalf("/weather/cities decode = %v body=%s", err, rec.Body.String())
	}
}

func TestWeatherGetAndCoordsRoutes(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterWeatherRoutes(e, s, authMiddlewareFor(user))

	// Missing city -> 400
	rec := performRequest(t, e, http.MethodGet, "/api/v1/weather", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("/weather missing city status = %d, want 400", rec.Code)
	}
	// Missing required params -> 400
	rec = performRequest(t, e, http.MethodGet, "/api/v1/weather/coords", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("/weather/coords missing params status = %d, want 400", rec.Code)
	}
	// Invalid lat -> 400
	rec = performRequest(t, e, http.MethodGet, "/api/v1/weather/coords?city=B&lat=abc&lon=116", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("/weather/coords invalid lat status = %d, want 400", rec.Code)
	}

	mockWeatherTransport(t)
	rec = performRequest(t, e, http.MethodGet, "/api/v1/weather?city=Beijing&date=2024-01-10", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/weather status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/weather/coords?city=Beijing&lat=39.9&lon=116.4&date=2024-01-10", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/weather/coords status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWeatherBackfillRoutes(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	cfg := config.NewConfigService(s)
	e := echo.New()
	RegisterWeatherRoutes(e, s, authMiddlewareFor(user))

	// No default city configured -> 400
	rec := performRequest(t, e, http.MethodPost, "/api/v1/weather/backfill", strings.NewReader(`{"start_date":"2024-01-01"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("/weather/backfill no default city status = %d, want 400", rec.Code)
	}

	if err := cfg.Set(user.ID, "weather.default_city", "Beijing"); err != nil {
		t.Fatalf("set weather.default_city: %v", err)
	}
	// A diary with no valid weather code in range -> triggers fetch.
	if _, _, err := s.UpsertDiary(user.ID, "2024-01-10", "A quiet day.", intPtr(4), nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("UpsertDiary: %v", err)
	}

	mockWeatherTransport(t)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/weather/backfill", strings.NewReader(`{"start_date":"2024-01-01"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("/weather/backfill status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "complete") {
		t.Fatalf("/weather/backfill stream missing complete event: %s", rec.Body.String())
	}
}
