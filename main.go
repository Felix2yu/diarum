package main

import (
	"bytes"
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
	"github.com/songtianlun/diarum/internal/backup"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/embedding"
	"github.com/songtianlun/diarum/internal/logger"
	mcpserver "github.com/songtianlun/diarum/internal/mcp"
	"github.com/songtianlun/diarum/internal/static"
	"github.com/songtianlun/diarum/internal/store"
	"github.com/songtianlun/diarum/internal/weather"
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
	// Initialize weather service (needed by settings routes for scheduler notifications)
	weatherSvc := weather.NewService()
	weatherScheduler := weather.NewScheduler(appStore, configService, weatherSvc)
	api.RegisterSettingsRoutes(e, appStore, authMiddleware, weatherScheduler)
	api.RegisterMemosRoutes(e, appStore, authMiddleware, onDiaryChanged)
	api.RegisterAIRoutes(e, appStore, authMiddleware, embeddingService)
	api.RegisterExportImportRoutes(e, appStore, authMiddleware, embeddingService)
	api.RegisterCheveretoRoutes(e, appStore, authMiddleware)
	api.RegisterWeatherRoutes(e, appStore, authMiddleware)
	api.RegisterPublicRoutes(e, appStore)
	api.RegisterVersionRoutes(e, Version, Name)
	api.RegisterMetricsRoutes(e)

	// Initialize MCP server
	mcpSrv := mcpserver.New(appStore)
	mcpHandler := mcpSrv.GetStreamableHTTPServer()

	// MCP auth middleware — inject user_id into request context via Echo middleware chain
	mcpAuth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
			}
			rawToken := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))

			userID, err := appStore.ValidateAPIToken(rawToken)
			if err != nil || userID == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
			}
			ctx := context.WithValue(c.Request().Context(), mcpserver.UserIDKey, userID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}

	e.Any("/mcp", echo.WrapHandler(mcpHandler), mcpAuth)
	logger.Info("[MCP] Streamable HTTP server enabled at /mcp")

	// Initialize backup scheduler
	backupScheduler := backup.NewScheduler(appStore, configService, *dataDir, func(userID string) (*bytes.Buffer, error) {
		req := api.ExportRequest{DateRange: "all", IncludeDiaries: true, IncludeMedia: true, IncludeConversations: true, IncludeAnalysis: true}
		buf, _, err := api.BuildExportZip(appStore, userID, req)
		return buf, err
	})
	api.RegisterBackupRoutes(e, appStore, authMiddleware, backupScheduler, configService)
	backupScheduler.Start()
	defer backupScheduler.Stop()

	// Start weather auto-fetch scheduler
	weatherScheduler.Start()
	defer weatherScheduler.Stop()

	staticFS, err := static.GetFS()
	if err != nil {
		log.Printf("Warning: Failed to get embedded static files: %v", err)
	} else {
		defaultHandler := e.HTTPErrorHandler
		e.HTTPErrorHandler = func(c *echo.Context, err error) {
			if strings.HasPrefix(c.Request().URL.Path, "/api/") || strings.HasPrefix(c.Request().URL.Path, "/mcp") || strings.HasPrefix(c.Request().URL.Path, "/.well-known") {
				defaultHandler(c, err)
				return
			}
			serveSPA(c, staticFS)
		}
	}

	if err := startServer(e, *httpAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
