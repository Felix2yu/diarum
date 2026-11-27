package store

import (
	"fmt"
	"testing"
	"time"
)

func mediaFileURL(mediaID string) string {
	return fmt.Sprintf("/api/v1/files/media/%s/photo.png", mediaID)
}

func mustGetMedia(t *testing.T, s *Store, id, owner string) *Media {
	t.Helper()
	media, err := s.GetMedia(id, owner)
	if err != nil {
		t.Fatalf("GetMedia(%s): %v", id, err)
	}
	return media
}

func mustCreateMedia(t *testing.T, s *Store, owner, name string) *Media {
	t.Helper()
	media, err := s.CreateMedia(owner, name+".png", name, "", nil)
	if err != nil {
		t.Fatalf("CreateMedia(%s): %v", name, err)
	}
	return media
}

func TestDiarySaveSyncsMediaImages(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	other := newTestUser(t, s)

	first := mustCreateMedia(t, s, user.ID, "first")
	second := mustCreateMedia(t, s, user.ID, "second")
	foreign := mustCreateMedia(t, s, other.ID, "foreign")
	unused := mustCreateMedia(t, s, user.ID, "unused")

	content := "今天\n\n![](" + mediaFileURL(first.ID) + ")\n![](" + mediaFileURL(second.ID) + ")\n![](" + mediaFileURL(foreign.ID) + ")"
	if _, _, err := s.UpsertDiary(user.ID, "2026-02-01", content, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("UpsertDiary: %v", err)
	}

	if got := mustGetMedia(t, s, first.ID, user.ID).Diary; len(got) != 1 {
		t.Fatalf("first media diary = %#v, want 1 association", got)
	}
	if got := mustGetMedia(t, s, second.ID, user.ID).Diary; len(got) != 1 {
		t.Fatalf("second media diary = %#v, want 1 association", got)
	}
	// 不能把别的用户的媒体关联进来
	if got := mustGetMedia(t, s, foreign.ID, other.ID).Diary; len(got) != 0 {
		t.Fatalf("foreign media diary = %#v, want empty", got)
	}

	// 删除正文中 second 的引用后，关联随之消失
	content = "今天\n\n![](" + mediaFileURL(first.ID) + ")"
	if _, _, err := s.UpsertDiary(user.ID, "2026-02-01", content, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("UpsertDiary second save: %v", err)
	}
	if got := mustGetMedia(t, s, second.ID, user.ID).Diary; len(got) != 0 {
		t.Fatalf("second media diary after removal = %#v, want empty", got)
	}

	items, total, err := s.ListUnreferencedMedia(user.ID, 1, 50)
	if err != nil {
		t.Fatalf("ListUnreferencedMedia: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("unreferenced total/items = %d/%d, want 2/2", total, len(items))
	}
	for _, item := range items {
		if item.ID == first.ID || item.ID == foreign.ID {
			t.Fatalf("unreferenced media contains %s, want only %s and %s", item.ID, second.ID, unused.ID)
		}
	}

	mediaItems, mediaTotal, err := s.ListMedia(user.ID, 1, 50)
	if err != nil {
		t.Fatalf("ListMedia: %v", err)
	}
	if mediaTotal != 3 {
		t.Fatalf("ListMedia total = %d, want 3", mediaTotal)
	}
	for _, item := range mediaItems {
		if item.ID != first.ID {
			continue
		}
		if item.Expand == nil || len(item.Expand.Diary) != 1 {
			t.Fatalf("first media expand = %#v, want 1 diary", item.Expand)
		}
	}
}

func TestSoftDeleteUnreferencedMedia(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	other := newTestUser(t, s)

	referenced := mustCreateMedia(t, s, user.ID, "referenced")
	freeA := mustCreateMedia(t, s, user.ID, "free-a")
	freeB := mustCreateMedia(t, s, user.ID, "free-b")
	foreign := mustCreateMedia(t, s, other.ID, "foreign")

	content := "![](" + mediaFileURL(referenced.ID) + ")"
	if _, _, err := s.UpsertDiary(user.ID, "2026-04-01", content, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("UpsertDiary: %v", err)
	}

	deleted, err := s.SoftDeleteUnreferencedMedia(user.ID)
	if err != nil {
		t.Fatalf("SoftDeleteUnreferencedMedia: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if got := mustGetMedia(t, s, referenced.ID, user.ID); got.DeletedAt != "" {
		t.Fatalf("referenced media deleted_at = %q, want empty", got.DeletedAt)
	}
	for _, media := range []*Media{freeA, freeB} {
		if got := mustGetMedia(t, s, media.ID, user.ID); got.DeletedAt == "" {
			t.Fatalf("unreferenced media %s not moved to trash", media.ID)
		}
	}
	if got := mustGetMedia(t, s, foreign.ID, other.ID); got.DeletedAt != "" {
		t.Fatalf("foreign media deleted_at = %q, want empty", got.DeletedAt)
	}

	again, err := s.SoftDeleteUnreferencedMedia(user.ID)
	if err != nil {
		t.Fatalf("SoftDeleteUnreferencedMedia second run: %v", err)
	}
	if again != 0 {
		t.Fatalf("second run deleted = %d, want 0", again)
	}
}

func TestListTrashedMediaBefore(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)

	fresh := mustCreateMedia(t, s, user.ID, "fresh")
	expired := mustCreateMedia(t, s, user.ID, "expired")
	for _, media := range []*Media{fresh, expired} {
		if err := s.SoftDeleteMedia(media.ID, user.ID); err != nil {
			t.Fatalf("SoftDeleteMedia: %v", err)
		}
	}
	past := time.Now().UTC().AddDate(0, 0, -40).Format("2006-01-02 15:04:05.000Z")
	if _, err := s.DB.Exec(`UPDATE media SET deleted_at = ? WHERE id = ?`, past, expired.ID); err != nil {
		t.Fatalf("backdate deleted_at: %v", err)
	}

	cutoff := time.Now().UTC().Add(-MediaTrashRetention).Format("2006-01-02 15:04:05.000Z")
	items, err := s.ListTrashedMediaBefore(user.ID, cutoff)
	if err != nil {
		t.Fatalf("ListTrashedMediaBefore: %v", err)
	}
	if len(items) != 1 || items[0].ID != expired.ID {
		t.Fatalf("expired trash = %#v, want only %s", items, expired.ID)
	}
}

func TestBackfillDiaryImages(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)

	diary, err := s.InsertImportedDiary(user.ID, "", "2025-12-01", "body", 0, nil, nil, "", nil, "", 0, 0)
	if err != nil {
		t.Fatalf("InsertImportedDiary: %v", err)
	}
	// 旧数据：只有 media.diary JSON，没有 diary_images
	linked := mustCreateMedia(t, s, user.ID, "linked")
	if _, err := s.DB.Exec(`DELETE FROM diary_images WHERE media_id = ?`, linked.ID); err != nil {
		t.Fatalf("clear links: %v", err)
	}
	if _, err := s.DB.Exec(`UPDATE media SET diary = ? WHERE id = ?`, encodeJSON([]string{diary.ID}), linked.ID); err != nil {
		t.Fatalf("seed media.diary: %v", err)
	}
	// 正文里引用的媒体
	inline := mustCreateMedia(t, s, user.ID, "inline")
	if _, err := s.DB.Exec(`DELETE FROM diary_images WHERE media_id = ?`, inline.ID); err != nil {
		t.Fatalf("clear links: %v", err)
	}
	if _, err := s.DB.Exec(`UPDATE diaries SET content = ? WHERE id = ?`, "![]("+mediaFileURL(inline.ID)+")", diary.ID); err != nil {
		t.Fatalf("seed diary content: %v", err)
	}

	if err := backfillDiaryImages(s.DB); err != nil {
		t.Fatalf("backfillDiaryImages: %v", err)
	}
	if got := mustGetMedia(t, s, linked.ID, user.ID).Diary; len(got) != 1 || got[0] != diary.ID {
		t.Fatalf("linked media diary = %#v, want [%s]", got, diary.ID)
	}
	if got := mustGetMedia(t, s, inline.ID, user.ID).Diary; len(got) != 1 || got[0] != diary.ID {
		t.Fatalf("inline media diary = %#v, want [%s]", got, diary.ID)
	}

	// 幂等：重复执行不会产生重复关联
	if err := backfillDiaryImages(s.DB); err != nil {
		t.Fatalf("backfillDiaryImages second run: %v", err)
	}
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM diary_images WHERE diary_id = ?`, diary.ID).Scan(&count); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if count != 2 {
		t.Fatalf("diary_images rows = %d, want 2", count)
	}
}
