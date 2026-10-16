package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/embedding"
	"github.com/songtianlun/diarum/internal/logger"
	mcpserver "github.com/songtianlun/diarum/internal/mcp"
	"github.com/songtianlun/diarum/internal/store"
)

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestGetDataDir(t *testing.T) {
	t.Setenv("DIARUM_DATA_PATH", "")
	t.Setenv("DIARIA_DATA_PATH", "")
	if got := getDataDir(); got != "./diarum_data" {
		t.Fatalf("getDataDir default = %q, want ./diarum_data", got)
	}

	t.Setenv("DIARIA_DATA_PATH", "/legacy")
	if got := getDataDir(); got != "/legacy" {
		t.Fatalf("getDataDir legacy env = %q, want /legacy", got)
	}

	t.Setenv("DIARUM_DATA_PATH", "/preferred")
	if got := getDataDir(); got != "/preferred" {
		t.Fatalf("getDataDir preferred env = %q, want /preferred", got)
	}
}

func TestServeSPA(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":          &fstest.MapFile{Data: []byte("root-index")},
		"assets/app.js":       &fstest.MapFile{Data: []byte("console.log('ok')")},
		"nested/index.html":   &fstest.MapFile{Data: []byte("nested-index")},
		"nested/ignored.txt":  &fstest.MapFile{Data: []byte("ignored")},
		"not-a-dir/file.html": &fstest.MapFile{Data: []byte("file-html")},
	}

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
		wantErr    error
	}{
		{name: "api path", path: "/api/v1/test", wantErr: echo.ErrNotFound},
		{name: "exact file", path: "/assets/app.js", wantStatus: http.StatusOK, wantBody: "console.log('ok')"},
		{name: "directory index", path: "/nested/", wantStatus: http.StatusOK, wantBody: "nested-index"},
		{name: "fallback index", path: "/missing", wantStatus: http.StatusOK, wantBody: "root-index"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := serveSPA(c, fs.FS(fsys))
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("serveSPA error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("serveSPA: %v", err)
			}
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if body := rec.Body.String(); body != tt.wantBody {
				t.Fatalf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestVersionGlobals(t *testing.T) {
	if Version == "" || Name == "" {
		t.Fatalf("Version/Name should not be empty: %q / %q", Version, Name)
	}
}

func TestRunVersionAndUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"version"}, &out); err != nil {
		t.Fatalf("run version: %v", err)
	}
	if !strings.Contains(out.String(), Version) {
		t.Fatalf("version output = %q", out.String())
	}

	if err := run([]string{"unknown"}, io.Discard); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("run unknown command error = %v", err)
	}
	if err := run([]string{"version"}, failingWriter{}); err == nil || err.Error() != "write failed" {
		t.Fatalf("run version writer error = %v, want write failed", err)
	}
}

func TestServeSPAEdgeCases(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":          &fstest.MapFile{Data: []byte("root")},
		"empty-dir/index.txt": &fstest.MapFile{Data: []byte("txt")},
		"bad-dir/index.html":  &fstest.MapFile{Data: []byte("bad-dir-index")},
	}

	e := echo.New()

	t.Run("root path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := serveSPA(c, fs.FS(fsys)); err != nil {
			t.Fatalf("serveSPA root: %v", err)
		}
		if body := rec.Body.String(); body != "root" {
			t.Fatalf("root body = %q, want root", body)
		}
	})

	t.Run("empty path becomes root", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = ""
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := serveSPA(c, fs.FS(fsys)); err != nil {
			t.Fatalf("serveSPA empty path: %v", err)
		}
	})

	t.Run("dir with trailing slash gets index", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bad-dir/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := serveSPA(c, fs.FS(fsys)); err != nil {
			t.Fatalf("serveSPA bad-dir/ : %v", err)
		}
		if body := rec.Body.String(); body != "bad-dir-index" {
			t.Fatalf("bad-dir/ body = %q, want bad-dir-index", body)
		}
	})

	t.Run("dir without trailing slash falls back", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/empty-dir", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := serveSPA(c, fs.FS(fsys)); err != nil {
			t.Fatalf("serveSPA empty-dir: %v", err)
		}
		if body := rec.Body.String(); body != "root" {
			t.Fatalf("empty-dir body = %q, want root fallback", body)
		}
	})

	t.Run("missing index.html", func(t *testing.T) {
		emptyFS := fstest.MapFS{}
		req := httptest.NewRequest(http.MethodGet, "/anything", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		err := serveSPA(c, fs.FS(emptyFS))
		if err != echo.ErrNotFound {
			t.Fatalf("serveSPA empty FS error = %v, want echo.ErrNotFound", err)
		}
	})

	t.Run("api path returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		err := serveSPA(c, fs.FS(fsys))
		if err != echo.ErrNotFound {
			t.Fatalf("serveSPA /api/ error = %v, want echo.ErrNotFound", err)
		}
	})

}

func TestRunServe(t *testing.T) {
	originalStartServer := startServer
	originalLevel := logger.GetLevel()
	defer func() { startServer = originalStartServer }()
	defer logger.SetLevel(originalLevel)

	var capturedAddr string
	startServer = func(e *echo.Echo, addr string) error {
		capturedAddr = addr
		if len(e.Router().Routes()) == 0 {
			t.Fatal("server should register routes before starting")
		}
		return http.ErrServerClosed
	}

	if err := run([]string{"serve", "-data-dir", t.TempDir(), "-http", ":9191"}, io.Discard); err != nil {
		t.Fatalf("run serve: %v", err)
	}
	if capturedAddr != ":9191" {
		t.Fatalf("capturedAddr = %q, want :9191", capturedAddr)
	}

	startServer = func(e *echo.Echo, addr string) error {
		return errors.New("boom")
	}
	if err := run([]string{"serve", "-data-dir", t.TempDir(), "-http", ":9292"}, io.Discard); err == nil || err.Error() != "boom" {
		t.Fatalf("run serve error = %v, want boom", err)
	}

	logger.SetLevel(logger.LevelDebug)
	startServer = func(e *echo.Echo, addr string) error {
		foundDocs := false
		for _, route := range e.Router().Routes() {
			if route.Path == "/api/docs" {
				foundDocs = true
				break
			}
		}
		if !foundDocs {
			t.Fatal("debug mode should register OpenAPI docs route")
		}
		return http.ErrServerClosed
	}
	if err := run([]string{"serve", "-data-dir", t.TempDir()}, io.Discard); err != nil {
		t.Fatalf("run serve debug docs: %v", err)
	}

	filePath := filepath.Join(t.TempDir(), "file-data-dir")
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile data-dir file: %v", err)
	}
	if err := run([]string{"serve", "-data-dir", filePath}, io.Discard); err == nil {
		t.Fatal("run serve should fail when data-dir points to a file")
	}

	t.Run("default command and data dir", func(t *testing.T) {
		startServer = func(e *echo.Echo, addr string) error {
			return http.ErrServerClosed
		}
		if err := run(nil, io.Discard); err != nil {
			t.Fatalf("run nil args: %v", err)
		}
	})

	t.Run("vector db init failure", func(t *testing.T) {
		startServer = func(e *echo.Echo, addr string) error {
			return http.ErrServerClosed
		}
		tmpDir := t.TempDir()
		vectorDir := filepath.Join(tmpDir, "vectors")
		if err := os.WriteFile(vectorDir, []byte("block"), 0o600); err != nil {
			t.Fatalf("WriteFile block vector dir: %v", err)
		}
		if err := run([]string{"serve", "-data-dir", tmpDir, "-http", ":9393"}, io.Discard); err != nil {
			t.Fatalf("run serve vector db failure should be non-fatal: %v", err)
		}
	})
}

func TestMimeByExtension(t *testing.T) {
	tests := []struct{ path, want string }{
		{"app.js", "application/javascript"},
		{"style.css", "text/css"},
		{"page.html", "text/html"},
		{"icon.svg", "image/svg+xml"},
		{"photo.png", "image/png"},
		{"pic.jpg", "image/jpeg"},
		{"pic.jpeg", "image/jpeg"},
		{"img.webp", "image/webp"},
		{"font.woff2", "font/woff2"},
		{"font.woff", "font/woff"},
		{"data.json", "application/json"},
		{"file.txt", "application/octet-stream"},
		{"", "application/octet-stream"},
	}
	for _, tt := range tests {
		if got := mimeByExtension(tt.path); got != tt.want {
			t.Errorf("mimeByExtension(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestServeSPACompressed(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("plain")},
		"app.js":        &fstest.MapFile{Data: []byte("plain-js")},
		"app.js.zst":    &fstest.MapFile{Data: []byte("zstd-js")},
		"app.js.br":     &fstest.MapFile{Data: []byte("br-js")},
		"style.css":     &fstest.MapFile{Data: []byte("plain-css")},
		"style.css.br":  &fstest.MapFile{Data: []byte("br-css")},
		"image.png":     &fstest.MapFile{Data: []byte("plain-png")},
		"image.png.zst": &fstest.MapFile{Data: []byte("zst-png")},
	}

	tests := []struct {
		name       string
		path       string
		acceptEnc  string
		wantBody   string
		wantEncHdr string
	}{
		{name: "zstd preferred", path: "/app.js", acceptEnc: "zstd, br", wantBody: "zstd-js", wantEncHdr: "zstd"},
		{name: "br fallback", path: "/app.js", acceptEnc: "br", wantBody: "br-js", wantEncHdr: "br"},
		{name: "no accept-encoding", path: "/app.js", acceptEnc: "", wantBody: "plain-js", wantEncHdr: ""},
		{name: "br preferred for css", path: "/style.css", acceptEnc: "br, zstd", wantBody: "br-css", wantEncHdr: "br"},
		{name: "zstd for png", path: "/image.png", acceptEnc: "zstd", wantBody: "zst-png", wantEncHdr: "zstd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.acceptEnc != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEnc)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			if err := serveSPA(c, fs.FS(fsys)); err != nil {
				t.Fatalf("serveSPA: %v", err)
			}
			if body := rec.Body.String(); body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
			if got := rec.Header().Get("Content-Encoding"); got != tt.wantEncHdr {
				t.Errorf("Content-Encoding = %q, want %q", got, tt.wantEncHdr)
			}
		})
	}
}

func TestServeSPAStatError(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("root")},
		"dir/":       &fstest.MapFile{Mode: fs.ModeDir},
	}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/dir/noindex", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := serveSPA(c, fs.FS(fsys)); err != nil {
		t.Fatalf("serveSPA dir noindex: %v", err)
	}
}

func TestMainFunction(t *testing.T) {
	if err := run([]string{"version"}, io.Discard); err != nil {
		t.Fatalf("main version: %v", err)
	}
}

// ---- fsys mocks for serveSPA stat-error branches ----

type errStatFile struct{}

func (errStatFile) Stat() (fs.FileInfo, error) { return nil, errors.New("stat boom") }
func (errStatFile) Read([]byte) (int, error)   { return 0, io.EOF }
func (errStatFile) Close() error               { return nil }

type errStatFS struct{}

func (errStatFS) Open(name string) (fs.File, error) { return errStatFile{}, nil }

type fakeDirFile struct{}

func (fakeDirFile) Stat() (fs.FileInfo, error) { return fakeDirInfo{}, nil }
func (fakeDirFile) Read([]byte) (int, error)   { return 0, fs.ErrInvalid }
func (fakeDirFile) Close() error               { return nil }

type fakeDirInfo struct{}

func (fakeDirInfo) Name() string { return "d" }
func (fakeDirInfo) IsDir() bool  { return true }
func (fakeDirInfo) Size() int64  { return 0 }

func (fakeDirInfo) Mode() fs.FileMode  { return fs.ModeDir }
func (fakeDirInfo) ModTime() time.Time { return time.Time{} }
func (fakeDirInfo) Sys() any           { return nil }

type dirThenErrFS struct{}

func (dirThenErrFS) Open(name string) (fs.File, error) {
	if strings.HasSuffix(name, "index.html") {
		return errStatFile{}, nil
	}
	return fakeDirFile{}, nil
}

func TestServeSPAStatErrorOnFile(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := serveSPA(c, errStatFS{})
	if err == nil || !strings.Contains(err.Error(), "stat boom") {
		t.Fatalf("serveSPA stat error = %v, want stat boom", err)
	}
}

func TestServeSPAStatErrorOnDirIndex(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/d", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := serveSPA(c, dirThenErrFS{})
	if err == nil || !strings.Contains(err.Error(), "stat boom") {
		t.Fatalf("serveSPA dir index stat error = %v, want stat boom", err)
	}
}

func TestMCPAuthMiddleware(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	defer s.Close()

	user, err := s.CreateUser("mcpuser", "mcp@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := s.SetSetting(user.ID, "api.enabled", true, false); err != nil {
		t.Fatalf("set api.enabled: %v", err)
	}
	if err := s.SetSetting(user.ID, "api.token", "good-token", false); err != nil {
		t.Fatalf("set api.token: %v", err)
	}

	next := func(c *echo.Context) error {
		user, _ := c.Request().Context().Value(mcpserver.UserIDKey).(string)
		return c.String(http.StatusOK, user)
	}
	handler := newMCPAuth(s)(next)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantBody   string
	}{
		{name: "missing header", authHeader: "", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", authHeader: "Basic abc", wantStatus: http.StatusUnauthorized},
		{name: "unknown token", authHeader: "Bearer nope", wantStatus: http.StatusUnauthorized},
		{name: "valid token", authHeader: "Bearer good-token", wantStatus: http.StatusOK, wantBody: user.ID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := handler(c); err != nil {
				httpErr, ok := err.(*echo.HTTPError)
				if !ok {
					t.Fatalf("handler error = %v, want HTTPError", err)
				}
				if httpErr.Code != tt.wantStatus {
					t.Fatalf("status = %d, want %d", httpErr.Code, tt.wantStatus)
				}
				return
			}
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if body := rec.Body.String(); body != tt.wantBody {
				t.Fatalf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestDiaryChangedHook(t *testing.T) {
	dataDir := t.TempDir()
	s, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	defer s.Close()

	configSvc := config.NewConfigService(s)

	hookUser, err := s.CreateUser("hookuser", "hook@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Run("disabled returns immediately", func(t *testing.T) {
		hook := newDiaryChangedHook(configSvc, nil)
		hook("user-disabled")
	})

	t.Run("enabled builds vectors async", func(t *testing.T) {
		vectorDB, err := embedding.NewVectorDB(dataDir)
		if err != nil {
			t.Fatalf("vector db: %v", err)
		}
		svc := embedding.NewEmbeddingService(s, vectorDB)
		hook := newDiaryChangedHook(configSvc, svc)

		if err := configSvc.Set(hookUser.ID, "ai.enabled", true); err != nil {
			t.Fatalf("set ai.enabled: %v", err)
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			hook(hookUser.ID)
		}()
		<-done

		time.Sleep(300 * time.Millisecond)
	})
}

func TestRunServeRequestHandling(t *testing.T) {
	originalStartServer := startServer
	defer func() { startServer = originalStartServer }()

	t.Setenv("DIARUM_PUSH_SUBSCRIBER", "ci-subscriber")

	var spaFallback bool
	startServer = func(e *echo.Echo, addr string) error {
		spaFallback = false

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/some-spa-route", nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Errorf("SPA fallback status = %d content-type = %q, want 200 text/html", rec.Code, rec.Header().Get("Content-Type"))
		} else {
			spaFallback = true
		}

		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("unknown api route status = %d, want 404", rec.Code)
		}

		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/test", nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("well-known status = %d, want 404", rec.Code)
		}

		return http.ErrServerClosed
	}

	if err := run([]string{"serve", "-data-dir", t.TempDir(), "-http", ":9494"}, io.Discard); err != nil {
		t.Fatalf("run serve: %v", err)
	}
	if !spaFallback {
		t.Fatal("SPA fallback handler was not exercised")
	}
}
