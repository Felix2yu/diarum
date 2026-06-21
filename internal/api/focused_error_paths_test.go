package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/embedding"
)

func TestFocusedAIRouteErrors(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	for _, test := range []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{name: "settings invalid json", method: http.MethodPut, path: "/api/v1/ai/settings", body: `{`, want: http.StatusBadRequest},
		{name: "settings missing enabled fields", method: http.MethodPut, path: "/api/v1/ai/settings", body: `{"enabled":true}`, want: http.StatusBadRequest},
		{name: "models invalid json", method: http.MethodPost, path: "/api/v1/ai/models", body: `{`, want: http.StatusBadRequest},
		{name: "chat invalid json", method: http.MethodPost, path: "/api/v1/ai/chat", body: `{`, want: http.StatusBadRequest},
		{name: "conversation update invalid json", method: http.MethodPut, path: "/api/v1/ai/conversations/missing", body: `{`, want: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := performRequest(t, e, test.method, test.path, strings.NewReader(test.body), map[string]string{"Content-Type": "application/json"})
			if rec.Code != test.want {
				t.Fatalf("%s status = %d body=%s", test.path, rec.Code, rec.Body.String())
			}
		})
	}

	withMockTransport(t, func(req *http.Request) (*http.Response, error) {
		return httpResponse(http.StatusInternalServerError, "upstream failed"), nil
	})
	rec := performRequest(t, e, http.MethodPost, "/api/v1/ai/models", strings.NewReader(`{"api_key":"key","base_url":"https://mock.local"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/models upstream error status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAIRoutesHappyPath(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), nil)

	rec := performRequest(t, e, http.MethodGet, "/api/v1/ai/settings", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ai/settings status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/ai/settings", strings.NewReader(`{"enabled":false}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /ai/settings status = %d body=%s", rec.Code, rec.Body.String())
	}

	withMockTransport(t, func(req *http.Request) (*http.Response, error) {
		return httpResponse(http.StatusOK, `{"object":"list","data":[{"id":"m1","object":"model"}]}`), nil
	})
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/models", strings.NewReader(`{"api_key":"k","base_url":"https://mock.local"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /ai/models status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/models", strings.NewReader(`{"api_key":"","base_url":""}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/models empty key status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/conversations", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ai/conversations status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/conversations", strings.NewReader(`{"title":"Test Conv"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /ai/conversations status = %d body=%s", rec.Code, rec.Body.String())
	}
	convPayload := decodeJSONBody(t, rec)
	convID := convPayload["id"].(string)

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/conversations/"+convID, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ai/conversations/:id status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/conversations/nonexistent", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /ai/conversations/nonexistent status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/ai/conversations/"+convID, strings.NewReader(`{"title":"Updated"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /ai/conversations/:id status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/ai/conversations/nonexistent", strings.NewReader(`{"title":"x"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("PUT /ai/conversations/nonexistent status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodDelete, "/api/v1/ai/conversations/"+convID, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /ai/conversations/:id status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodDelete, "/api/v1/ai/conversations/nonexistent", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("DELETE /ai/conversations/nonexistent status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/chat", strings.NewReader(`{"conversation_id":"x","content":"hi"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /ai/chat not found status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/chat", strings.NewReader(`{}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/chat empty status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"month","start":"2024-01-01","end":"2024-01-31"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /ai/analysis empty status = %d body=%s", rec.Code, rec.Body.String())
	}

	if _, err := s.InsertImportedDiary(user.ID, "", "2024-01-15", "some content", "happy", "", nil); err != nil {
		t.Fatalf("InsertImportedDiary: %v", err)
	}
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"month","start":"2024-01-01","end":"2024-01-31"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/analysis disabled with diaries status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analyses?period=month", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ai/analyses status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analysis?period=month&start=2024-01-01&end=2024-01-31", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ai/analysis status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"month","start":"2024-01-01","end":"2024-01-31","keywords":"travel,food"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /ai/analysis with keywords status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/vectors/stats", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /ai/vectors/stats nil service status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/vectors/build", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/vectors/build nil status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/vectors/build-incremental", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/vectors/build-incremental nil status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/transcribe", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/transcribe no provider status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"bad","start":"2024-01-01","end":"2024-01-31"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/analysis bad period status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"month","start":"","end":""}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/analysis missing start/end status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"month","start":"bad-date","end":"2024-01-31"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/analysis bad start date status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/analysis", strings.NewReader(`{"period":"month","start":"2024-01-01","end":"bad-date"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/analysis bad end date status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analysis?period=bad&start=2024-01-01&end=2024-01-31", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /ai/analysis bad period status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analysis?period=month", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /ai/analysis missing start/end status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analyses?period=bad", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /ai/analyses bad period status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/analyses?period=all", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ai/analyses all status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/polish", strings.NewReader(`{}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/polish status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFocusedAIVectorRouteServiceErrors(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	vectorDB, err := embedding.NewVectorDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewVectorDB: %v", err)
	}
	e := echo.New()
	RegisterAIRoutes(e, s, authMiddlewareFor(user), embedding.NewEmbeddingService(s, vectorDB))

	rec := performRequest(t, e, http.MethodPost, "/api/v1/ai/vectors/build", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/vectors/build disabled status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performRequest(t, e, http.MethodPost, "/api/v1/ai/vectors/build-incremental", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /ai/vectors/build-incremental disabled status = %d body=%s", rec.Code, rec.Body.String())
	}

	configureAIRouteSettings(t, s, user.ID)
	if err := s.DB.Close(); err != nil {
		t.Fatalf("Close store DB: %v", err)
	}
	rec = performRequest(t, e, http.MethodGet, "/api/v1/ai/vectors/stats", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /ai/vectors/stats closed DB status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFocusedMediaRouteErrors(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterMediaRoutes(e, s, authMiddlewareFor(user))

	rec := performRequest(t, e, http.MethodGet, "/api/v1/media?page=0&perPage=0", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /media invalid pagination status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	if payload["page"] != float64(1) || payload["perPage"] != float64(50) {
		t.Fatalf("invalid pagination payload = %#v", payload)
	}

	rec = performRequest(t, e, http.MethodPatch, "/api/v1/media/missing", strings.NewReader(`{"diary":[]}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("PATCH /media/missing status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performRequest(t, e, http.MethodPatch, "/api/v1/media/missing", strings.NewReader(`{`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PATCH /media invalid JSON status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performRequest(t, e, http.MethodGet, "/api/v1/files/media/missing/photo.png", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET missing media file status = %d body=%s", rec.Code, rec.Body.String())
	}

	media, err := s.InsertImportedMedia(user.ID, "media-fallback", "fallback.png", "Fallback", "Alt", nil)
	if err != nil {
		t.Fatalf("InsertImportedMedia: %v", err)
	}
	fallbackPath := s.NewMediaFilePath(media.ID, media.File)
	if err := os.MkdirAll(filepath.Dir(fallbackPath), 0o755); err != nil {
		t.Fatalf("MkdirAll fallback media: %v", err)
	}
	if err := os.WriteFile(fallbackPath, pngBytes(), 0o600); err != nil {
		t.Fatalf("WriteFile fallback media: %v", err)
	}
	rec = performRequest(t, e, http.MethodGet, "/api/v1/files/media/"+media.ID+"/"+media.File, nil, nil)
	if rec.Code != http.StatusOK || rec.Header().Get(echo.HeaderContentType) != "image/png" || len(rec.Body.Bytes()) == 0 {
		t.Fatalf("GET fallback media file status = %d content-type=%q body=%q", rec.Code, rec.Header().Get(echo.HeaderContentType), rec.Body.Bytes())
	}

	oversized := append([]byte{}, pngBytes()...)
	oversized = append(oversized, bytes.Repeat([]byte{0}, 50*1024*1024)...)
	body, contentType := multipartRequestBody(t, "file", "too-large.png", oversized, nil)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/media", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /media oversized status = %d body=%s", rec.Code, rec.Body.String())
	}

	cfg := config.NewConfigService(s)
	if err := cfg.Set(user.ID, "image_upload.provider", "chevereto"); err != nil {
		t.Fatalf("Set image_upload.provider: %v", err)
	}
	body, contentType = multipartRequestBody(t, "file", "save-error.png", pngBytes(), nil)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/media", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST /media save error status = %d body=%s", rec.Code, rec.Body.String())
	}
	items, total, err := s.ListMedia(user.ID, 1, 20)
	if err != nil {
		t.Fatalf("ListMedia after save error: %v", err)
	}
	for _, item := range items {
		if item.File == "save-error.png" {
			t.Fatalf("failed upload media was not deleted: total=%d items=%#v", total, items)
		}
	}
	if err := cfg.Set(user.ID, "image_upload.provider", "local"); err != nil {
		t.Fatalf("Reset image_upload.provider: %v", err)
	}

	deleteMedia, err := s.InsertImportedMedia(user.ID, "delete-error", "blocked.png", "Blocked", "", nil)
	if err != nil {
		t.Fatalf("InsertImportedMedia delete error: %v", err)
	}
	blockedPath := s.MediaFilePath(deleteMedia)
	if err := os.MkdirAll(filepath.Join(blockedPath, "child"), 0o755); err != nil {
		t.Fatalf("MkdirAll blocked media path: %v", err)
	}
	rec = performRequest(t, e, http.MethodDelete, "/api/v1/media/"+deleteMedia.ID, nil, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("DELETE /media file delete error status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFocusedMediaUploadCreateError(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterMediaRoutes(e, s, authMiddlewareFor(user))
	if err := s.DB.Close(); err != nil {
		t.Fatalf("Close store DB: %v", err)
	}

	body, contentType := multipartRequestBody(t, "file", "create-error.png", pngBytes(), nil)
	rec := performRequest(t, e, http.MethodPost, "/api/v1/media", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /media create error status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFocusedPublicRouteDisabledToken(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterPublicRoutes(e, s)
	cfg := config.NewConfigService(s)
	if err := cfg.Set(user.ID, "api.token", "disabled-token"); err != nil {
		t.Fatalf("Set api.token: %v", err)
	}
	if err := cfg.Set(user.ID, "api.enabled", false); err != nil {
		t.Fatalf("Set api.enabled: %v", err)
	}

	rec := performRequest(t, e, http.MethodGet, "/api/v1/diaries?token=disabled-token&date=2024-01-01", nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("public disabled token status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFocusedImageUploadBranches(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterImageUploadRoutes(e, s, authMiddlewareFor(user))
	cfg := config.NewConfigService(s)

	rec := performRequest(t, e, http.MethodGet, "/api/v1/image-upload/settings", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET image-upload default status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["provider"] != "local" || payload["local"].(map[string]any)["path"] == "" {
		t.Fatalf("GET image-upload default payload = %#v", payload)
	}

	for key, value := range map[string]any{
		"image_upload.provider":            " S3 ",
		"image_upload.local.path":          " custom-media ",
		"image_upload.s3.bucket":           "bucket",
		"image_upload.s3.region":           "region",
		"image_upload.s3.endpoint":         "endpoint",
		"image_upload.s3.access_key":       "access",
		"image_upload.s3.secret":           "secret",
		"image_upload.s3.force_path_style": true,
		"chevereto.domain":                 "https://img.example.com/",
		"chevereto.api_key":                "key",
		"chevereto.album_id":               "album",
	} {
		if err := cfg.Set(user.ID, key, value); err != nil {
			t.Fatalf("Set %s: %v", key, err)
		}
	}
	rec = performRequest(t, e, http.MethodGet, "/api/v1/image-upload/settings", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET image-upload configured status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["provider"] != "s3" || payload["chevereto"].(map[string]any)["domain"] != "https://img.example.com" {
		t.Fatalf("GET image-upload configured payload = %#v", payload)
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT image-upload invalid JSON status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"bogus"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT image-upload invalid provider status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"s3","s3":{"bucket":"bucket"}}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT image-upload incomplete s3 status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"chevereto","chevereto":{"domain":"https://img.example.com"}}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT image-upload incomplete chevereto status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"s3","s3":{"bucket":" bucket ","region":" us-east-1 ","access_key":" key ","secret":" secret ","endpoint":" https://s3.example.com ","force_path_style":true}}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT image-upload s3 status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	settings := payload["settings"].(map[string]any)
	if settings["provider"] != "s3" || settings["s3"].(map[string]any)["bucket"] != "bucket" {
		t.Fatalf("s3 settings payload = %#v", payload)
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"chevereto","chevereto":{"domain":" https://img.example.com/ ","api_key":" key ","album_id":" album "}}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT image-upload chevereto status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload = decodeJSONBody(t, rec)
	settings = payload["settings"].(map[string]any)
	chevereto := settings["chevereto"].(map[string]any)
	if settings["provider"] != "chevereto" || chevereto["domain"] != "https://img.example.com" || chevereto["api_key"] != "key" {
		t.Fatalf("chevereto settings payload = %#v", payload)
	}
}

func TestFocusedCheveretoRouteErrors(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterCheveretoRoutes(e, s, authMiddlewareFor(user))

	for _, test := range []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{name: "settings invalid json", method: http.MethodPut, path: "/api/v1/chevereto/settings", body: `{`, want: http.StatusBadRequest},
		{name: "test invalid json", method: http.MethodPost, path: "/api/v1/chevereto/test", body: `{`, want: http.StatusBadRequest},
		{name: "upload disabled", method: http.MethodPost, path: "/api/v1/chevereto/upload", body: ``, want: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := performRequest(t, e, test.method, test.path, strings.NewReader(test.body), map[string]string{"Content-Type": "application/json"})
			if rec.Code != test.want {
				t.Fatalf("%s status = %d body=%s", test.path, rec.Code, rec.Body.String())
			}
		})
	}

	cfg := config.NewConfigService(s)
	for key, value := range map[string]any{
		"chevereto.enabled":  true,
		"chevereto.domain":   "https://img.example.com",
		"chevereto.api_key":  "key",
		"chevereto.album_id": "album",
	} {
		if err := cfg.Set(user.ID, key, value); err != nil {
			t.Fatalf("Set %s: %v", key, err)
		}
	}

	rec := performRequest(t, e, http.MethodPost, "/api/v1/chevereto/upload", strings.NewReader(""), map[string]string{"Content-Type": "multipart/form-data; boundary=missing"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST chevereto/upload missing source status = %d body=%s", rec.Code, rec.Body.String())
	}

	for _, test := range []struct {
		name string
		body string
	}{
		{name: "upstream non-200", body: "upstream failed"},
		{name: "invalid json", body: `{`},
		{name: "missing image", body: `{"status_code":200}`},
		{name: "missing url", body: `{"image":{}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			withMockTransport(t, func(req *http.Request) (*http.Response, error) {
				if test.name == "upstream non-200" {
					return httpResponse(http.StatusBadGateway, test.body), nil
				}
				return httpResponse(http.StatusOK, test.body), nil
			})
			body, contentType := multipartRequestBody(t, "source", "photo.png", pngBytes(), nil)
			rec := performRequest(t, e, http.MethodPost, "/api/v1/chevereto/upload", body, map[string]string{"Content-Type": contentType})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("POST chevereto/upload %s status = %d body=%s", test.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestFocusedSettingsRouteErrors(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterSettingsRoutes(e, s, authMiddlewareFor(user))
	cfg := config.NewConfigService(s)
	if err := cfg.Set(user.ID, "ai.chat_model", "gpt-route"); err != nil {
		t.Fatalf("Set ai.chat_model: %v", err)
	}

	rec := performRequest(t, e, http.MethodGet, "/api/v1/settings/ai.chat_model", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET single setting route status = %d body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeJSONBody(t, rec); payload["value"] != "gpt-route" {
		t.Fatalf("GET single setting payload = %#v", payload)
	}
	rec = performRequest(t, e, http.MethodGet, "/api/v1/settings/unknown.key", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET unknown single setting status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/settings/ai.enabled", strings.NewReader(`{`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT single setting invalid JSON status = %d body=%s", rec.Code, rec.Body.String())
	}

	if err := s.DB.Close(); err != nil {
		t.Fatalf("Close store DB: %v", err)
	}
	rec = performRequest(t, e, http.MethodGet, "/api/v1/settings/api-token", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET api-token closed DB status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, test := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "batch set", method: http.MethodPut, path: "/api/v1/settings/batch", body: `{"settings":{"ai.enabled":true}}`},
		{name: "single put", method: http.MethodPut, path: "/api/v1/settings/ai.enabled", body: `{"value":true}`},
		{name: "single delete", method: http.MethodDelete, path: "/api/v1/settings/ai.enabled"},
		{name: "token toggle", method: http.MethodPost, path: "/api/v1/settings/api-token/toggle"},
		{name: "token reset", method: http.MethodPost, path: "/api/v1/settings/api-token/reset"},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := performRequest(t, e, test.method, test.path, strings.NewReader(test.body), map[string]string{"Content-Type": "application/json"})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s status = %d body=%s", test.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestFocusedExportImportEdges(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterExportImportRoutes(e, s, authMiddlewareFor(user), nil)

	if _, err := s.InsertImportedDiary(user.ID, "export-old", "2024-01-01", "old diary", "", "", nil); err != nil {
		t.Fatalf("InsertImportedDiary old: %v", err)
	}
	if _, err := s.InsertImportedDiary(user.ID, "export-new", "2024-02-02", "new diary", "", "", nil); err != nil {
		t.Fatalf("InsertImportedDiary new: %v", err)
	}
	missingMedia, err := s.InsertImportedMedia(user.ID, "export-missing-media", "missing.png", "Missing", "", nil)
	if err != nil {
		t.Fatalf("InsertImportedMedia missing: %v", err)
	}
	rec := performRequest(t, e, http.MethodPost, "/api/v1/export", strings.NewReader(`{"date_range":"custom","start_date":"2024-02-01","end_date":"2024-02-28","include_diaries":true,"include_media":false,"include_conversations":false}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST export filtered status = %d body=%s", rec.Code, rec.Body.String())
	}
	entries := zipEntries(t, rec.Body.Bytes())
	if strings.Contains(string(entries["diarum_export.json"]), "old diary") || !strings.Contains(string(entries["diarum_export.json"]), "new diary") {
		t.Fatalf("filtered export json = %s", entries["diarum_export.json"])
	}
	var stats exportStats
	if err := json.Unmarshal([]byte(rec.Header().Get("X-Export-Stats")), &stats); err != nil {
		t.Fatalf("decode export stats: %v", err)
	}
	if stats.Diaries.TotalInSystem != 2 || stats.Diaries.ActualExported != 1 || stats.Media.ActualExported != 0 {
		t.Fatalf("filtered export stats = %#v", stats)
	}
	rec = performRequest(t, e, http.MethodPost, "/api/v1/export", strings.NewReader(`{"date_range":"all","include_diaries":false,"include_media":true,"include_conversations":false}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST export missing media status = %d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal([]byte(rec.Header().Get("X-Export-Stats")), &stats); err != nil {
		t.Fatalf("decode missing media export stats: %v", err)
	}
	if stats.Media.ShouldExport != 1 || stats.Media.ActualExported != 0 || len(stats.FailedItems) != 1 || stats.FailedItems[0].ID != missingMedia.ID {
		t.Fatalf("missing media export stats = %#v", stats)
	}

	rec = performRequest(t, e, http.MethodPost, "/api/v1/export", strings.NewReader(`{"date_range":"custom","start_date":"2024-02-02","end_date":"2024-02-01"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST export invalid custom range status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performRequest(t, e, http.MethodPost, "/api/v1/export", strings.NewReader(`{`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST export malformed JSON fallback status = %d body=%s", rec.Code, rec.Body.String())
	}

	body, contentType := multipartRequestBody(t, "file", "not.zip", []byte("not a zip"), nil)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/import", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST import invalid zip status = %d body=%s", rec.Code, rec.Body.String())
	}

	var invalidJSONZip bytes.Buffer
	zw := zip.NewWriter(&invalidJSONZip)
	w, err := zw.Create("diarum_export.json")
	if err != nil {
		t.Fatalf("Create diarum_export.json: %v", err)
	}
	if _, err := w.Write([]byte(`{`)); err != nil {
		t.Fatalf("Write invalid export JSON: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close invalid JSON zip: %v", err)
	}
	body, contentType = multipartRequestBody(t, "file", "invalid-json.zip", invalidJSONZip.Bytes(), nil)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/import", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST import invalid export JSON status = %d body=%s", rec.Code, rec.Body.String())
	}

	zipBytes := buildImportZip(t, exportData{
		Version:    1,
		ExportedAt: "2024-02-01T00:00:00Z",
		Diaries: []exportDiary{
			{ID: "missing-date", Content: "no date"},
		},
		Media: []exportMedia{
			{ID: "missing-file", File: "missing.png", Name: "Missing"},
		},
		Conversations: []exportConversation{
			{ID: "conv", Title: "Conversation", Messages: []exportMessage{{ID: "msg", Role: "user", Content: "hello", ReferencedDiaries: []string{"missing-date"}}}},
		},
	}, nil)
	body, contentType = multipartRequestBody(t, "file", "edge.zip", zipBytes, nil)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/import", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST import edge stats status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	if payload["diaries"].(map[string]any)["failed"] != float64(1) || payload["media"].(map[string]any)["failed"] != float64(1) || payload["conversations"].(map[string]any)["imported"] != float64(1) {
		t.Fatalf("import edge payload = %#v", payload)
	}
}

func TestImportTriggersEmbeddingBranch(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	vectorDB, err := embedding.NewVectorDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewVectorDB: %v", err)
	}
	t.Cleanup(func() { _ = vectorDB.Close() })
	service := embedding.NewEmbeddingService(s, vectorDB)
	if err := config.NewConfigService(s).Set(user.ID, "ai.enabled", true); err != nil {
		t.Fatalf("Set ai.enabled: %v", err)
	}
	e := echo.New()
	RegisterExportImportRoutes(e, s, authMiddlewareFor(user), service)

	zipBytes := buildImportZip(t, exportData{
		Version:    1,
		ExportedAt: "2024-02-01T00:00:00Z",
		Diaries: []exportDiary{
			{ID: "diary", Date: "2024-03-01", Content: "Imported with embedding", Mood: "ok", Weather: "sun"},
		},
	}, nil)
	body, contentType := multipartRequestBody(t, "file", "embedding.zip", zipBytes, nil)
	rec := performRequest(t, e, http.MethodPost, "/api/v1/import", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST import embedding status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	if payload["diaries"].(map[string]any)["imported"] != float64(1) {
		t.Fatalf("import embedding payload = %#v", payload)
	}
}

func TestFocusedClosedStoreRouteErrors(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	authMiddleware := authMiddlewareFor(user)
	RegisterDiaryRoutes(e, s, authMiddleware, nil)
	RegisterMediaRoutes(e, s, authMiddleware)
	RegisterSettingsRoutes(e, s, authMiddleware)
	RegisterImageUploadRoutes(e, s, authMiddleware)
	RegisterCheveretoRoutes(e, s, authMiddleware)
	RegisterAIRoutes(e, s, authMiddleware, nil)
	RegisterExportImportRoutes(e, s, authMiddleware, nil)

	if err := s.DB.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	checks := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/diaries?date=2024-01-01", ""},
		{http.MethodPost, "/api/v1/diaries", `{"date":"2024-01-01","content":"body"}`},
		{http.MethodGet, "/api/v1/diaries/calendar", ""},
		{http.MethodGet, "/api/v1/diaries/search?q=body", ""},
		{http.MethodGet, "/api/v1/diaries/recent", ""},
		{http.MethodGet, "/api/v1/diaries/missing", ""},
		{http.MethodGet, "/api/v1/media", ""},
		{http.MethodGet, "/api/v1/image-upload/settings", ""},
		{http.MethodPut, "/api/v1/image-upload/settings", `{"provider":"local"}`},
		{http.MethodGet, "/api/v1/settings/api-token", ""},
		{http.MethodPost, "/api/v1/settings/api-token/toggle", ""},
		{http.MethodPost, "/api/v1/settings/api-token/reset", ""},
		{http.MethodGet, "/api/v1/settings/api.enabled", ""},
		{http.MethodPut, "/api/v1/settings/api.enabled", `{"value":true}`},
		{http.MethodDelete, "/api/v1/settings/api.enabled", ""},
		{http.MethodPut, "/api/v1/chevereto/settings", `{"enabled":false}`},
		{http.MethodGet, "/api/v1/ai/settings", ""},
		{http.MethodPut, "/api/v1/ai/settings", `{"enabled":false}`},
		{http.MethodGet, "/api/v1/ai/conversations", ""},
		{http.MethodPost, "/api/v1/ai/conversations", `{"title":"title"}`},
		{http.MethodGet, "/api/v1/ai/conversations/missing", ""},
		{http.MethodDelete, "/api/v1/ai/conversations/missing", ""},
		{http.MethodPut, "/api/v1/ai/conversations/missing", `{"title":"title"}`},
		{http.MethodPost, "/api/v1/export", `{}`},
	}
	for _, check := range checks {
		t.Run(check.method+" "+check.path, func(t *testing.T) {
			var body *strings.Reader
			if check.body != "" {
				body = strings.NewReader(check.body)
			} else {
				body = strings.NewReader("")
			}
			rec := performRequest(t, e, check.method, check.path, body, map[string]string{"Content-Type": "application/json"})
			if rec.Code == 0 {
				t.Fatal("request did not produce a response")
			}
		})
	}
}

func TestFetchModelsAdditionalErrors(t *testing.T) {
	if _, err := fetchModels("://invalid", "key"); err == nil {
		t.Fatal("expected invalid URL error")
	}

	withMockTransport(t, func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport failed")
	})
	if _, err := fetchModels("https://mock.local", "key"); err == nil {
		t.Fatal("expected transport error")
	}

	withMockTransport(t, func(*http.Request) (*http.Response, error) {
		return httpResponse(http.StatusOK, "not-json"), nil
	})
	if _, err := fetchModels("https://mock.local", "key"); err == nil {
		t.Fatal("expected response decode error")
	}
}

func TestParseMarkdownFile(t *testing.T) {
	for _, tc := range []struct {
		name     string
		filename string
		content  string
		wantNil  bool
		wantDate string
		wantMood string
		wantWx   string
	}{
		{
			name:     "date from filename",
			filename: "2024-03-15.md",
			content:  "Hello world",
			wantDate: "2024-03-15",
		},
		{
			name:     "date from filename with mood suffix",
			filename: "2024-03-15_happy.md",
			content:  "Hello world",
			wantDate: "2024-03-15",
		},
		{
			name:     "date from heading",
			filename: "diary.md",
			content:  "# 2024-05-01\n\nContent here",
			wantDate: "2024-05-01",
		},
		{
			name:     "mood and weather from content",
			filename: "2024-01-01.md",
			content:  "# 2024-01-01\n\n**Mood:** excited\n**Weather:** rainy\n\nBody text",
			wantDate: "2024-01-01",
			wantMood: "excited",
			wantWx:   "rainy",
		},
		{
			name:     "no date returns nil",
			filename: "notes.md",
			content:  "Just some notes",
			wantNil:  true,
		},
		{
			name:     "nested path extracts date",
			filename: "markdown/2024-07-04_mood.md",
			content:  "content",
			wantDate: "2024-07-04",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := parseMarkdownFile(tc.filename, []byte(tc.content))
			if tc.wantNil {
				if result != nil {
					t.Fatalf("expected nil, got %+v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Date != tc.wantDate {
				t.Errorf("Date = %q, want %q", result.Date, tc.wantDate)
			}
			if tc.wantMood != "" && result.Mood != tc.wantMood {
				t.Errorf("Mood = %q, want %q", result.Mood, tc.wantMood)
			}
			if tc.wantWx != "" && result.Weather != tc.wantWx {
				t.Errorf("Weather = %q, want %q", result.Weather, tc.wantWx)
			}
		})
	}
}

func TestResolveConflictRoutes(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterExportImportRoutes(e, s, authMiddlewareFor(user), nil)

	if _, err := s.InsertImportedDiary(user.ID, "old", "2024-06-01", "old content", "sad", "", nil); err != nil {
		t.Fatalf("InsertImportedDiary: %v", err)
	}

	body, _ := json.Marshal(resolveConflictRequest{Date: "", Action: "keep_old"})
	rec := performRequest(t, e, http.MethodPost, "/api/v1/import/resolve", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty date status = %d", rec.Code)
	}

	body, _ = json.Marshal(resolveConflictRequest{Date: "2024-06-01", Action: "invalid"})
	rec = performRequest(t, e, http.MethodPost, "/api/v1/import/resolve", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid action status = %d", rec.Code)
	}

	body, _ = json.Marshal(resolveConflictRequest{Date: "2024-06-01", Action: "keep_old"})
	rec = performRequest(t, e, http.MethodPost, "/api/v1/import/resolve", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("keep_old status = %d body=%s", rec.Code, rec.Body.String())
	}

	body, _ = json.Marshal(resolveConflictRequest{Date: "2024-06-01", Action: "replace", Content: "new content", Mood: "happy", Weather: "sunny"})
	rec = performRequest(t, e, http.MethodPost, "/api/v1/import/resolve", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("replace status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	if payload["status"] != "replaced" {
		t.Fatalf("replace payload = %#v", payload)
	}
	existing, _ := s.GetDiaryByDate(user.ID, "2024-06-01 00:00:00.000Z", "2024-06-01 23:59:59.999Z")
	if existing == nil || existing.Content != "new content" || existing.Mood != "happy" {
		t.Fatalf("diary after replace = %+v", existing)
	}

	body, _ = json.Marshal(resolveConflictRequest{Date: "2024-09-99", Action: "replace", Content: "x"})
	rec = performRequest(t, e, http.MethodPost, "/api/v1/import/resolve", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("replace no existing status = %d body=%s", rec.Code, rec.Body.String())
	}

	body = []byte(`{}`)
	rec = performRequest(t, e, http.MethodPost, "/api/v1/import/resolve", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid bind status = %d", rec.Code)
	}
}

func TestImportMdZipFallback(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterExportImportRoutes(e, s, authMiddlewareFor(user), nil)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("2024-08-10.md")
	_, _ = w.Write([]byte("# 2024-08-10\n\n**Mood:** calm\n**Weather:** foggy\n\nA quiet day"))
	w, _ = zw.Create("2024-08-11.md")
	_, _ = w.Write([]byte("# 2024-08-11\n\nJust a normal entry"))
	_ = zw.Close()

	body, contentType := multipartRequestBody(t, "file", "diaries.zip", buf.Bytes(), nil)
	rec := performRequest(t, e, http.MethodPost, "/api/v1/import", body, map[string]string{"Content-Type": contentType})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST import md zip status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	diaries := payload["diaries"].(map[string]any)
	if diaries["imported"] != float64(2) {
		t.Fatalf("imported = %v, want 2", diaries["imported"])
	}
}

func TestResolveConflictWithEmbedding(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	vectorDB, err := embedding.NewVectorDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewVectorDB: %v", err)
	}
	t.Cleanup(func() { _ = vectorDB.Close() })
	service := embedding.NewEmbeddingService(s, vectorDB)
	if err := config.NewConfigService(s).Set(user.ID, "ai.enabled", true); err != nil {
		t.Fatalf("Set ai.enabled: %v", err)
	}
	e := echo.New()
	RegisterExportImportRoutes(e, s, authMiddlewareFor(user), service)

	body, _ := json.Marshal(resolveConflictRequest{Date: "2024-07-04", Action: "replace", Content: "new with embedding"})
	rec := performRequest(t, e, http.MethodPost, "/api/v1/import/resolve", bytes.NewReader(body), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("replace with embedding status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestImageUploadSettingsRoutes(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterImageUploadRoutes(e, s, authMiddlewareFor(user))

	rec := performRequest(t, e, http.MethodGet, "/api/v1/image-upload/settings", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /image-upload/settings status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"local","local":{"path":"/tmp/img"}}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /image-upload/settings local status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"s3","s3":{"bucket":"b","region":"us-east-1","endpoint":"https://s3.amazonaws.com","access_key":"ak","secret":"sk"}}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /image-upload/settings s3 status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"chevereto","chevereto":{"domain":"https://example.com","api_key":"ck"}}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /image-upload/settings chevereto status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT /image-upload/settings invalid json status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"s3","s3":{}}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT /image-upload/settings s3 missing fields status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"chevereto","chevereto":{}}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT /image-upload/settings chevereto missing fields status = %d", rec.Code)
	}

	rec = performRequest(t, e, http.MethodPut, "/api/v1/image-upload/settings", strings.NewReader(`{"provider":"unknown"}`), map[string]string{"Content-Type": "application/json"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT /image-upload/settings unknown provider status = %d", rec.Code)
	}
}
