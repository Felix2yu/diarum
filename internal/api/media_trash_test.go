package api

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	iauth "github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/store"
)

func TestMediaUnreferencedRoute(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	e := echo.New()
	RegisterMediaRoutes(e, s, authMiddlewareFor(user), iauth.NewService(s))

	media, err := s.CreateMedia(user.ID, "photo.png", "Photo", "", nil)
	if err != nil {
		t.Fatalf("CreateMedia: %v", err)
	}
	if _, err := s.CreateMedia(user.ID, "photo2.png", "Photo2", "", nil); err != nil {
		t.Fatalf("CreateMedia: %v", err)
	}

	// 一篇引用其中一张的日记，未引用列表应只剩另一张
	content := "hello\n![](" + "/api/v1/files/media/" + media.ID + "/photo.png" + ")"
	if _, _, err := s.UpsertDiary(user.ID, "2026-03-01", content, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("UpsertDiary: %v", err)
	}

	rec := performRequest(t, e, http.MethodGet, "/api/v1/media/unreferenced", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /media/unreferenced status = %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONBody(t, rec)
	if total, _ := payload["totalItems"].(float64); total != 1 {
		t.Fatalf("unreferenced totalItems = %v, want 1", payload["totalItems"])
	}
}

func TestPurgeExpiredTrashedMedia(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)

	media, err := s.CreateMedia(user.ID, "old.png", "Old", "", nil)
	if err != nil {
		t.Fatalf("CreateMedia: %v", err)
	}
	if err := s.SaveUploadedMedia(media, strings.NewReader("media-body")); err != nil {
		t.Fatalf("SaveUploadedMedia: %v", err)
	}
	if err := s.SoftDeleteMedia(media.ID, user.ID); err != nil {
		t.Fatalf("SoftDeleteMedia: %v", err)
	}
	past := time.Now().UTC().AddDate(0, 0, -40).Format("2006-01-02 15:04:05.000Z")
	if _, err := s.DB.Exec(`UPDATE media SET deleted_at = ? WHERE id = ?`, past, media.ID); err != nil {
		t.Fatalf("backdate deleted_at: %v", err)
	}
	filePath := s.MediaFilePath(media)

	deleted, err := PurgeExpiredTrashedMedia(s, user.ID, time.Now().UTC().Add(-store.MediaTrashRetention))
	if err != nil {
		t.Fatalf("PurgeExpiredTrashedMedia: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("purged = %d, want 1", deleted)
	}
	if _, err := s.GetMedia(media.ID, user.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetMedia after purge error = %v, want not found", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("media file still exists after purge: %v", err)
	}
}
