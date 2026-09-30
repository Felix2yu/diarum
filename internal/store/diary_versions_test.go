package store

import (
	"errors"
	"testing"
	"time"
)

// 同一编辑会话只记一个版本，不同会话各记一个；快照内容为会话首次修改前的旧正文。
func TestDiaryVersionPerEditSession(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)

	if _, _, err := s.UpsertDiary(user.ID, "2026-09-01", "original", nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("seed diary: %v", err)
	}

	// 会话 1 首次修改正文 → 快照 "original"
	if _, _, err := s.UpsertDiaryWithSession(user.ID, "2026-09-01", "v1", nil, nil, nil, nil, nil, nil, nil, nil, "session-1"); err != nil {
		t.Fatalf("upsert session-1: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	// 会话 1 内再次保存（自动保存）→ 不再新增版本
	if _, _, err := s.UpsertDiaryWithSession(user.ID, "2026-09-01", "v2", nil, nil, nil, nil, nil, nil, nil, nil, "session-1"); err != nil {
		t.Fatalf("upsert session-1 second save: %v", err)
	}

	diary, err := s.GetDiaryByDate(user.ID, "2026-09-01 00:00:00.000Z", "2026-09-01 23:59:59.999Z")
	if err != nil {
		t.Fatalf("get diary: %v", err)
	}
	versions, err := s.ListDiaryVersions(user.ID, diary.ID, 30)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version after session-1, got %d", len(versions))
	}
	if versions[0].Preview != "original" {
		t.Fatalf("version preview = %q, want %q", versions[0].Preview, "original")
	}
	full, err := s.GetDiaryVersion(user.ID, versions[0].ID)
	if err != nil || full.Content != "original" {
		t.Fatalf("version content = %q, err %v", full.Content, err)
	}

	// 新会话 2 → 快照当时正文 "v2"
	time.Sleep(5 * time.Millisecond)
	if _, _, err := s.UpsertDiaryWithSession(user.ID, "2026-09-01", "v3", nil, nil, nil, nil, nil, nil, nil, nil, "session-2"); err != nil {
		t.Fatalf("upsert session-2: %v", err)
	}
	versions, err = s.ListDiaryVersions(user.ID, diary.ID, 30)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions after session-2, got %d", len(versions))
	}
	full, err = s.GetDiaryVersion(user.ID, versions[0].ID)
	if err != nil {
		t.Fatalf("get newest version: %v", err)
	}
	if full.Content != "v2" {
		t.Fatalf("newest version content = %q, want %q", full.Content, "v2")
	}
}

// 无会话 ID、正文未变化、新建日记都不产生版本。
func TestDiaryVersionNotRecordedWithoutSessionOrChange(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)

	// 无会话（memos/MCP/导入路径）
	if _, _, err := s.UpsertDiary(user.ID, "2026-09-02", "first", nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// 带会话的新建日记（无旧内容可快照）
	if _, _, err := s.UpsertDiaryWithSession(user.ID, "2026-09-03", "new day", nil, nil, nil, nil, nil, nil, nil, nil, "session-a"); err != nil {
		t.Fatalf("insert with session: %v", err)
	}
	// 带会话但正文未变化
	if _, _, err := s.UpsertDiaryWithSession(user.ID, "2026-09-02", "first", nil, nil, nil, nil, nil, nil, nil, nil, "session-b"); err != nil {
		t.Fatalf("upsert same content: %v", err)
	}
	// 带会话但只改 mood（正文不变）
	if _, _, err := s.UpsertDiaryWithSession(user.ID, "2026-09-02", "first", intPtr(4), nil, nil, nil, nil, nil, nil, nil, "session-c"); err != nil {
		t.Fatalf("upsert mood only: %v", err)
	}

	for _, date := range []string{"2026-09-02", "2026-09-03"} {
		diary, err := s.GetDiaryByDate(user.ID, date+" 00:00:00.000Z", date+" 23:59:59.999Z")
		if err != nil {
			t.Fatalf("get diary %s: %v", date, err)
		}
		versions, err := s.ListDiaryVersions(user.ID, diary.ID, 30)
		if err != nil {
			t.Fatalf("list versions %s: %v", date, err)
		}
		if len(versions) != 0 {
			t.Fatalf("expected 0 versions for %s, got %d", date, len(versions))
		}
	}
}

// 恢复：内容回写、恢复前当前内容被快照（可撤销）、相同内容 no-op、越权不可见。
func TestDiaryVersionRestore(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)
	other := newTestUser(t, s)

	if _, _, err := s.UpsertDiary(user.ID, "2026-09-04", "original", intPtr(3), nil, nil, &[]string{"#a"}, strPtr("sunny"), nil, nil, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, _, err := s.UpsertDiaryWithSession(user.ID, "2026-09-04", "edited", intPtr(5), nil, nil, &[]string{"#b"}, strPtr("rain"), nil, nil, nil, "session-1"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	diary, err := s.GetDiaryByDate(user.ID, "2026-09-04 00:00:00.000Z", "2026-09-04 23:59:59.999Z")
	if err != nil {
		t.Fatalf("get diary: %v", err)
	}
	versions, err := s.ListDiaryVersions(user.ID, diary.ID, 30)
	if err != nil || len(versions) != 1 {
		t.Fatalf("versions = %d, err %v", len(versions), err)
	}
	original := versions[0]

	// 越权不可见
	if _, err := s.GetDiaryVersion(other.ID, original.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other owner get version err = %v, want ErrNotFound", err)
	}

	// 恢复（间隔确保 created 时间戳可排序）
	time.Sleep(5 * time.Millisecond)
	restored, err := s.RestoreDiaryVersion(user.ID, original.ID)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Content != "original" || restored.Mood != 3 || restored.Weather != "sunny" || len(restored.Tags) != 1 || restored.Tags[0] != "#a" {
		t.Fatalf("restored = %+v", restored)
	}

	// 恢复前当前内容已被快照 → 版本数 +1，且内容为 "edited"
	versions, err = s.ListDiaryVersions(user.ID, diary.ID, 30)
	if err != nil {
		t.Fatalf("list after restore: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions after restore, got %d", len(versions))
	}
	snapshotted := false
	for _, v := range versions {
		full, err := s.GetDiaryVersion(user.ID, v.ID)
		if err != nil {
			t.Fatalf("get version %s: %v", v.ID, err)
		}
		if full.Content == "edited" {
			snapshotted = true
		}
	}
	if !snapshotted {
		t.Fatalf("snapshot before restore missing; versions = %+v", versions)
	}

	// 再次恢复同一版本 → 内容相同，no-op 不新增版本
	if _, err := s.RestoreDiaryVersion(user.ID, original.ID); err != nil {
		t.Fatalf("restore again: %v", err)
	}
	versions, err = s.ListDiaryVersions(user.ID, diary.ID, 30)
	if err != nil || len(versions) != 2 {
		t.Fatalf("expected 2 versions after no-op restore, got %d, err %v", len(versions), err)
	}
}

// 按保留期过期清理；retentionDays <= 0 不清理。
func TestDiaryVersionPurgeExpired(t *testing.T) {
	s := newTestStore(t)
	user := newTestUser(t, s)

	if _, _, err := s.UpsertDiary(user.ID, "2026-09-05", "keep me", nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	diary, err := s.GetDiaryByDate(user.ID, "2026-09-05 00:00:00.000Z", "2026-09-05 23:59:59.999Z")
	if err != nil {
		t.Fatalf("get diary: %v", err)
	}
	// 手工插入一条 40 天前的过期版本
	expiredCreated := time.Now().UTC().AddDate(0, 0, -40).Format("2006-01-02 15:04:05.000Z")
	if _, err := s.DB.Exec(
		`INSERT INTO diary_versions(id, owner, diary_id, date, edit_session_id, content, mood, mood_states, scenarios, weather, city, temp_min, temp_max, tags, created)
		 VALUES('expired-v', ?, ?, '2026-09-05', 'expired-session', 'old', 0, '[]', '[]', '', '', 0, 0, '[]', ?)`,
		user.ID, diary.ID, expiredCreated,
	); err != nil {
		t.Fatalf("insert expired version: %v", err)
	}
	// 现有版本
	if _, _, err := s.UpsertDiaryWithSession(user.ID, "2026-09-05", "changed", nil, nil, nil, nil, nil, nil, nil, nil, "session-1"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// retentionDays=0 → 不清理
	if err := s.PurgeExpiredDiaryVersions(user.ID, 0); err != nil {
		t.Fatalf("purge with 0: %v", err)
	}
	versions, err := s.ListDiaryVersions(user.ID, diary.ID, 0)
	if err != nil || len(versions) != 2 {
		t.Fatalf("expected 2 versions with retention 0, got %d, err %v", len(versions), err)
	}

	// retentionDays=30 → 过期的被清理
	versions, err = s.ListDiaryVersions(user.ID, diary.ID, 30)
	if err != nil || len(versions) != 1 {
		t.Fatalf("expected 1 version after purge, got %d, err %v", len(versions), err)
	}
	if _, err := s.GetDiaryVersion(user.ID, "expired-v"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired version still visible: %v", err)
	}
}
