package api

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/imaging"
	"github.com/songtianlun/diarum/internal/store"
)

// mediaFileAuth 校验媒体文件访问权限，支持 Authorization header 或 ?token= query 参数。
// query 形式用于 <img> 标签等无法携带请求头的场景。
func mediaFileAuth(authService *auth.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			after, _ := strings.CutPrefix(c.Request().Header.Get("Authorization"), "Bearer ")
			token := strings.TrimSpace(after)
			if token == "" {
				token = c.QueryParam("token")
			}
			if token == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "The request requires valid authorization token.")
			}
			user, err := authService.ParseToken(token)
			if err != nil {
				return err
			}
			c.Set(auth.ContextUserKey, user)
			return next(c)
		}
	}
}

func RegisterMediaRoutes(e *echo.Echo, s *store.Store, authMiddleware echo.MiddlewareFunc, authService *auth.Service) {
	group := e.Group("/api/v1/media", authMiddleware)

	group.GET("", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		page := parsePositiveInt(c.QueryParam("page"), 1)
		perPage := parsePositiveInt(c.QueryParam("perPage"), 50)
		items, total, err := s.ListMedia(user.ID, page, perPage)
		if err != nil {
			return serverError("Failed to fetch media", err)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"page":       page,
			"perPage":    perPage,
			"totalItems": total,
			"totalPages": store.TotalPages(total, perPage),
			"items":      items,
		})
	})

	group.GET("/trash", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		page := parsePositiveInt(c.QueryParam("page"), 1)
		perPage := parsePositiveInt(c.QueryParam("perPage"), 50)
		items, total, err := s.ListTrashedMedia(user.ID, page, perPage)
		if err != nil {
			return serverError("Failed to fetch trashed media", err)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"page":       page,
			"perPage":    perPage,
			"totalItems": total,
			"totalPages": store.TotalPages(total, perPage),
			"items":      items,
		})
	})

	group.DELETE("/trash", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		items, err := s.ListAllTrashedMedia(user.ID)
		if err != nil {
			return serverError("Failed to load trashed media", err)
		}
		deleted := 0
		failed := 0
		for i := range items {
			if err := permanentlyDeleteMedia(s, user.ID, &items[i]); err != nil {
				failed++
				continue
			}
			deleted++
		}
		return c.JSON(http.StatusOK, map[string]any{
			"success": true,
			"deleted": deleted,
			"failed":  failed,
		})
	})

	group.GET("/:id", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		media, err := s.GetMedia(c.Param("id"), user.ID)
		if err != nil {
			return notFound("Media not found")
		}
		return c.JSON(http.StatusOK, media)
	})

	group.POST("", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		file, header, err := c.Request().FormFile("file")
		if err != nil {
			return badRequest("No file provided", err)
		}
		defer file.Close()

		buffer := make([]byte, 512)
		n, _ := file.Read(buffer)
		if seeker, ok := file.(interface {
			Seek(int64, int) (int64, error)
		}); ok {
			_, _ = seeker.Seek(0, 0)
		}
		if detected, allowed := config.IsAllowedMediaType(buffer[:n]); !allowed {
			return badRequest("Invalid file type: "+detected, nil)
		}
		if header.Size > 50*1024*1024 {
			return badRequest("File size exceeds 50MB limit", nil)
		}

		filename := store.SafeFilename(header.Filename)
		name := c.FormValue("name")
		if name == "" {
			name = filename
		}
		alt := c.FormValue("alt")
		diary := c.Request().Form["diary"]
		media, err := s.CreateMedia(user.ID, filename, name, alt, diary)
		if err != nil {
			return badRequest("Failed to create media", err)
		}
		if err := s.SaveUploadedMedia(media, file); err != nil {
			_ = s.DeleteMedia(media.ID, user.ID)
			return serverError("Failed to save media file", err)
		}
		return c.JSON(http.StatusOK, media)
	})

	group.PATCH("/:id", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		var body struct {
			Diary []string `json:"diary"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		media, err := s.UpdateMediaDiary(c.Param("id"), user.ID, body.Diary)
		if err != nil {
			return notFound("Media not found")
		}
		return c.JSON(http.StatusOK, media)
	})

	group.DELETE("/:id", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		if err := s.SoftDeleteMedia(c.Param("id"), user.ID); err != nil {
			return notFound("Media not found")
		}
		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	group.POST("/:id/restore", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		if err := s.RestoreMedia(c.Param("id"), user.ID); err != nil {
			return notFound("Media not found")
		}
		media, err := s.GetMedia(c.Param("id"), user.ID)
		if err != nil {
			return notFound("Media not found")
		}
		return c.JSON(http.StatusOK, media)
	})

	group.DELETE("/:id/permanent", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		media, err := s.GetMedia(c.Param("id"), user.ID)
		if err != nil {
			return notFound("Media not found")
		}
		if err := permanentlyDeleteMedia(s, user.ID, media); err != nil {
			return serverError("Failed to delete media file", err)
		}
		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	e.GET("/api/v1/files/media/:id/:filename", func(c *echo.Context) error {
		user := auth.CurrentUser(c)
		media, err := s.GetMedia(c.Param("id"), user.ID)
		if err != nil || media.File != c.Param("filename") {
			return notFound("File not found")
		}
		if variant, ok := mediaVariantFromRequest(c); ok {
			return serveMediaVariant(c, s, media, variant)
		}
		return serveOriginalMedia(c, s, media)
	}, mediaFileAuth(authService))
}

func mediaVariantCacheDir(s *store.Store, mediaID string) string {
	return filepath.Join(s.DataDir, "cache", "media-variants", mediaID)
}

func permanentlyDeleteMedia(s *store.Store, owner string, media *store.Media) error {
	if err := s.DeleteMediaFile(media); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := s.DeleteMedia(media.ID, owner); err != nil {
		return err
	}
	_ = os.RemoveAll(mediaVariantCacheDir(s, media.ID))
	return nil
}

func mediaVariantFromRequest(c *echo.Context) (imaging.Variant, bool) {
	if raw := strings.TrimSpace(c.QueryParam("variant")); raw != "" {
		return imaging.ResolveVariant(raw)
	}
	if raw := strings.TrimSpace(c.QueryParam("thumb")); raw != "" {
		return imaging.ResolveSize(raw)
	}
	return imaging.Variant{}, false
}

func serveMediaVariant(c *echo.Context, s *store.Store, media *store.Media, variant imaging.Variant) error {
	cacheDir := mediaVariantCacheDir(s, media.ID)
	for _, ext := range []string{"jpg", "png"} {
		cachePath := filepath.Join(cacheDir, variant.Name+"."+ext)
		fi, err := os.Stat(cachePath)
		if err != nil {
			continue
		}
		f, err := os.Open(cachePath)
		if err != nil {
			break
		}
		defer f.Close()
		c.Response().Header().Set("Cache-Control", "private, max-age=604800")
		http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), f)
		return nil
	}

	reader, err := s.OpenMediaFile(media)
	if err != nil {
		return notFound("File not found")
	}
	defer reader.Close()

	result, err := imaging.Generate(reader, variant)
	if err != nil {
		// SVG 等不支持解码的格式直接回退原图，避免灯箱与缩略图请求失败。
		return serveOriginalMedia(c, s, media)
	}

	cachePath := filepath.Join(cacheDir, variant.Name+"."+result.Extension)
	if err := writeFileAtomic(cachePath, result.Data); err != nil {
		// 缓存失败不影响本次请求，继续返回内存中的变体。
	}

	c.Response().Header().Set(echo.HeaderContentType, result.ContentType)
	c.Response().Header().Set("Cache-Control", "private, max-age=604800")
	c.Response().WriteHeader(http.StatusOK)
	_, err = c.Response().Write(result.Data)
	return err
}

func serveOriginalMedia(c *echo.Context, s *store.Store, media *store.Media) error {
	path := s.MediaFilePath(media)
	if fi, err := os.Stat(path); err == nil {
		f, err := os.Open(path)
		if err != nil {
			return notFound("File not found")
		}
		defer f.Close()
		c.Response().Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), f)
		return nil
	}

	reader, err := s.OpenMediaFile(media)
	if err != nil {
		return notFound("File not found")
	}
	defer reader.Close()

	head := make([]byte, 512)
	n, readErr := reader.Read(head)
	if readErr != nil && readErr != io.EOF {
		return serverError("Failed to read media file", readErr)
	}

	contentType := http.DetectContentType(head[:n])
	if guessed := mime.TypeByExtension(filepath.Ext(media.File)); guessed != "" {
		contentType = guessed
	}
	c.Response().Header().Set(echo.HeaderContentType, contentType)
	c.Response().Header().Set("Cache-Control", "private, max-age=3600")
	c.Response().WriteHeader(http.StatusOK)
	if n > 0 {
		if _, err := c.Response().Write(head[:n]); err != nil {
			return err
		}
	}
	_, err = io.Copy(c.Response(), reader)
	return err
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".variant-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func parsePositiveInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
