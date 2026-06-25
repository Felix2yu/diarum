package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/songtianlun/diarum/internal/api"
	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/embedding"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/static"
	"github.com/songtianlun/diarum/internal/store"
)

var startServer = func(e *echo.Echo, addr string) error {
	return e.Start(addr)
}

func getDataDir() string {
	if dataDir := os.Getenv("DIARUM_DATA_PATH"); dataDir != "" {
		return dataDir
	}
	if dataDir := os.Getenv("DIARIA_DATA_PATH"); dataDir != "" {
		return dataDir
	}
	return "./diarum_data"
}

func acceptEncoding(r *http.Request, encoding string) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), encoding)
}

func serveSPA(c *echo.Context, fsys fs.FS) error {
	path := c.Request().URL.Path
	if strings.HasPrefix(path, "/api/") {
		return echo.ErrNotFound
	}
	path = filepath.Clean(path)
	if path == "." {
		path = "/"
	}
	path = strings.TrimPrefix(path, "/")

	for _, enc := range []struct{ ext, header string }{
		{".zst", "zstd"},
		{".br", "br"},
	} {
		if acceptEncoding(c.Request(), enc.header) {
			compressed := path + enc.ext
			if f, err := fsys.Open(compressed); err == nil {
				defer f.Close()
				if stat, err := f.Stat(); err == nil && !stat.IsDir() {
					data, _ := io.ReadAll(f)
					c.Response().Header().Set(echo.HeaderContentEncoding, enc.header)
					c.Response().Header().Set(echo.HeaderContentType, mimeByExtension(path))
					return c.Blob(http.StatusOK, mimeByExtension(path), data)
				}
			}
		}
	}

	file, err := fsys.Open(path)
	if err == nil {
		defer file.Close()
		stat, err := file.Stat()
		if err != nil {
			return err
		}
		if stat.IsDir() {
			file.Close()
			file, err = fsys.Open(filepath.Join(path, "index.html"))
			if err == nil {
				defer file.Close()
				stat, err = file.Stat()
				if err != nil {
					return err
				}
			}
		}
		if err == nil {
			data, _ := io.ReadAll(file)
			return c.Blob(http.StatusOK, mimeByExtension(stat.Name()), data)
		}
	}
	indexFile, err := fsys.Open("index.html")
	if err != nil {
		return echo.ErrNotFound
	}
	defer indexFile.Close()
	data, _ := io.ReadAll(indexFile)
	return c.HTMLBlob(http.StatusOK, data)
}

func mimeByExtension(path string) string {
	switch {
	case strings.HasSuffix(path, ".js"):
		return "application/javascript"
	case strings.HasSuffix(path, ".css"):
		return "text/css"
	case strings.HasSuffix(path, ".html"):
		return "text/html"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".webp"):
		return "image/webp"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(path, ".woff"):
		return "font/woff"
	case strings.HasSuffix(path, ".json"):
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, stdout io.Writer) error {
	command := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	if command == "version" {
		_, err := fmt.Fprintf(stdout, "%s version %s\n", Name, Version)
		return err
	}
	if command != "serve" {
		return fmt.Errorf("unknown command: %s", command)
	}

	serveFlags := flag.NewFlagSet("serve", flag.ExitOnError)
	dataDir := serveFlags.String("data-dir", getDataDir(), "the directory to store application data")
	httpAddr := serveFlags.String("http", ":8090", "HTTP listen address")
	if err := serveFlags.Parse(args); err != nil {
		return err
	}

	appStore, err := store.Open(*dataDir)
	if err != nil {
		return err
	}
	defer appStore.Close()

	absDataDir, err := filepath.Abs(appStore.DataDir)
	if err != nil {
		log.Printf("Data directory: %s", appStore.DataDir)
	} else {
		log.Printf("Data directory: %s", absDataDir)
	}

	vectorDB, err := embedding.NewVectorDB(appStore.DataDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize vector database: %v", err)
	}
	var embeddingService *embedding.EmbeddingService
	if vectorDB != nil {
		embeddingService = embedding.NewEmbeddingService(appStore, vectorDB)
	}

	configService := config.NewConfigService(appStore)
	authService := auth.NewService(appStore)
	e := echo.New()
	e.Use(middleware.Recover())

	authMiddleware := authService.Middleware
	onDiaryChanged := func(userID string) {
		enabled, _ := configService.GetBool(userID, "ai.enabled")
		if !enabled || embeddingService == nil {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			logger.Info("[AutoVectorBuild] triggered for user: %s", userID)
			result, err := embeddingService.BuildIncrementalVectors(ctx, userID)
			if err != nil {
				logger.Error("[AutoVectorBuild] failed for user %s: %v", userID, err)
				return
			}
			logger.Info("[AutoVectorBuild] completed for user %s: %d built, %d failed", userID, result.Success, result.Failed)
		}()
	}

	api.RegisterAuthRoutes(e, appStore, authService)
	api.RegisterDiaryRoutes(e, appStore, authMiddleware, onDiaryChanged)
	api.RegisterMediaRoutes(e, appStore, authMiddleware)
	api.RegisterImageUploadRoutes(e, appStore, authMiddleware)
	api.RegisterSettingsRoutes(e, appStore, authMiddleware)
	api.RegisterMemosRoutes(e, appStore, authMiddleware, onDiaryChanged)
	api.RegisterAIRoutes(e, appStore, authMiddleware, embeddingService)
	api.RegisterExportImportRoutes(e, appStore, authMiddleware, embeddingService)
	api.RegisterCheveretoRoutes(e, appStore, authMiddleware)
	api.RegisterPublicRoutes(e, appStore)
	api.RegisterVersionRoutes(e, Version, Name)
	api.RegisterMetricsRoutes(e)
	if logger.GetLevel() <= logger.LevelDebug {
		api.RegisterOpenAPIRoutes(e, Version, Name)
		logger.Info("[Docs] OpenAPI docs enabled in debug mode at /api/docs and /api/v1/docs")
	}

	staticFS, err := static.GetFS()
	if err != nil {
		log.Printf("Warning: Failed to get embedded static files: %v", err)
	} else {
		defaultHandler := e.HTTPErrorHandler
		e.HTTPErrorHandler = func(c *echo.Context, err error) {
			log.Printf("HTTPErrorHandler called: path=%s err=%v", c.Request().URL.Path, err)
			if strings.HasPrefix(c.Request().URL.Path, "/api/") {
				log.Printf(" -> defaultHandler for API path")
				defaultHandler(c, err)
				return
			}
			w := c.Response()
			path := c.Request().URL.Path
			path = filepath.Clean(path)
			if path == "." {
				path = "/"
			}
			path = strings.TrimPrefix(path, "/")
			if path == "" {
				path = "index.html"
			}
			if f, ferr := staticFS.Open(path); ferr == nil {
				stat, _ := f.Stat()
				if stat != nil && !stat.IsDir() {
					data, _ := io.ReadAll(f)
					f.Close()
					log.Printf(" -> serving file %s (%d bytes)", path, len(data))
					w.Header().Set(echo.HeaderContentType, mimeByExtension(path))
					w.WriteHeader(http.StatusOK)
					w.Write(data)
					return
				}
				f.Close()
			}
			if f, ferr := staticFS.Open("index.html"); ferr == nil {
				data, _ := io.ReadAll(f)
				f.Close()
				log.Printf(" -> serving index.html (%d bytes)", len(data))
				w.Header().Set(echo.HeaderContentType, "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				w.Write(data)
				return
			} else {
				log.Printf(" -> index.html open failed: %v", ferr)
			}
			log.Printf(" -> falling back to default handler")
			defaultHandler(c, err)
		}
	}

	if err := startServer(e, *httpAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
