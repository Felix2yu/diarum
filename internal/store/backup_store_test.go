package store

import (
	"database/sql"
	"errors"
	"testing"
)

func newBackupStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestBackupCRUD(t *testing.T) {
	s := newBackupStore(t)
	u, err := s.CreateUser("backupuser", "backup@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	created, err := s.CreateBackup(u.ID, "dump.zip", "/data/dump.zip", 1024, "s3/key.zip")
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if created.ID == "" || created.Owner != u.ID || created.Filename != "dump.zip" {
		t.Fatalf("created backup = %+v", created)
	}

	got, err := s.GetBackupByID(created.ID)
	if err != nil {
		t.Fatalf("get backup: %v", err)
	}
	if got.Filepath != "/data/dump.zip" || got.Size != 1024 || got.S3Key != "s3/key.zip" {
		t.Fatalf("fetched backup = %+v", got)
	}

	if _, err := s.GetBackupByID("missing-id"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for missing backup, got %v", err)
	}

	for i := range 2 {
		if _, err := s.CreateBackup(u.ID, "b"+string(rune('a'+i))+".zip", "/p", int64(i), ""); err != nil {
			t.Fatalf("create extra backup %d: %v", i, err)
		}
	}

	list, total, err := s.ListBackups(u.ID, 1, 2)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if total != 3 || len(list) != 2 {
		t.Fatalf("page 1 = %d items, total %d, want 2 items total 3", len(list), total)
	}

	list, total, err = s.ListBackups(u.ID, 2, 2)
	if err != nil {
		t.Fatalf("list backups page 2: %v", err)
	}
	if total != 3 || len(list) != 1 {
		t.Fatalf("page 2 = %d items, total %d, want 1 item total 3", len(list), total)
	}

	if _, _, err := s.ListBackups(u.ID, 1, 0); err != nil {
		t.Fatalf("list backups with perPage=0 should default: %v", err)
	}

	if err := s.DeleteBackup(created.ID, u.ID); err != nil {
		t.Fatalf("delete backup: %v", err)
	}
	if err := s.DeleteBackup(created.ID, u.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleting removed backup should return sql.ErrNoRows, got %v", err)
	}
	if err := s.DeleteBackup(created.ID, "other-owner"); err == nil {
		t.Fatal("deleting another owner's backup should fail")
	}
}

func TestCleanupOldBackups(t *testing.T) {
	s := newBackupStore(t)
	u, err := s.CreateUser("cleanupuser", "cleanup@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if removed, err := s.CleanupOldBackups(u.ID, 0); err != nil || removed != nil {
		t.Fatalf("retention <= 0 should be a no-op, got %v / %v", removed, err)
	}
	if removed, err := s.CleanupOldBackups(u.ID, -1); err != nil || removed != nil {
		t.Fatalf("negative retention should be a no-op, got %v / %v", removed, err)
	}

	oldB, err := s.CreateBackup(u.ID, "old.zip", "/old", 1, "old-key")
	if err != nil {
		t.Fatalf("create old backup: %v", err)
	}
	newB, err := s.CreateBackup(u.ID, "new.zip", "/new", 1, "")
	if err != nil {
		t.Fatalf("create new backup: %v", err)
	}
	if _, err := s.DB.Exec(`UPDATE backups SET created = '2020-01-01T00:00:00.000Z' WHERE id = ?`, oldB.ID); err != nil {
		t.Fatalf("age old backup: %v", err)
	}

	removed, err := s.CleanupOldBackups(u.ID, 30)
	if err != nil {
		t.Fatalf("cleanup old backups: %v", err)
	}
	if len(removed) != 1 || removed[0].ID != oldB.ID || removed[0].S3Key != "old-key" {
		t.Fatalf("removed = %+v, want only the stale backup with its S3 key", removed)
	}

	if _, err := s.GetBackupByID(newB.ID); err != nil {
		t.Fatalf("fresh backup should survive cleanup: %v", err)
	}
	if _, err := s.GetBackupByID(oldB.ID); err == nil {
		t.Fatal("stale backup should be deleted")
	}
}

func TestUpsertDiaryWeather(t *testing.T) {
	s := newBackupStore(t)
	u, err := s.CreateUser("weatheruser", "weather@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	diary, err := s.UpsertDiaryWeather(u.ID, "2025-06-15", "晴", "北京", 22, 31)
	if err != nil {
		t.Fatalf("upsert insert path: %v", err)
	}
	if diary.Weather != "晴" || diary.City != "北京" || diary.TempMin != 22 || diary.TempMax != 31 {
		t.Fatalf("inserted diary weather = %+v", diary)
	}

	updated, err := s.UpsertDiaryWeather(u.ID, "2025-06-15", "多云", "上海", 18, 27)
	if err != nil {
		t.Fatalf("upsert update path: %v", err)
	}
	if updated.ID != diary.ID {
		t.Fatalf("update should reuse the same diary, got %s want %s", updated.ID, diary.ID)
	}
	if updated.Weather != "多云" || updated.City != "上海" || updated.TempMin != 18 || updated.TempMax != 27 {
		t.Fatalf("updated diary weather = %+v", updated)
	}
}

func TestListUsers(t *testing.T) {
	s := newBackupStore(t)
	users := []*User{}
	for _, name := range []string{"alpha", "beta"} {
		u, err := s.CreateUser(name, name+"@example.com", "hash")
		if err != nil {
			t.Fatalf("create user %s: %v", name, err)
		}
		users = append(users, u)
	}

	got, err := s.ListUsers()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(got) != len(users) {
		t.Fatalf("listed %d users, want %d", len(got), len(users))
	}
	seen := map[string]bool{}
	for _, u := range got {
		seen[u.ID] = true
	}
	for _, u := range users {
		if !seen[u.ID] {
			t.Fatalf("user %q missing from ListUsers output", u.ID)
		}
	}
}
