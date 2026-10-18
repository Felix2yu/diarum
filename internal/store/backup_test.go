package store

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// sqliteMagic 是 SQLite 数据库文件的 16 字节头，外部备份工具按它校验产物。
const sqliteMagic = "SQLite format 3\x00"

func sha256File(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		t.Fatalf("hash %s: %v", path, err)
	}
	return string(sum.Sum(nil))
}

// openSnapshotFile 把快照字节落盘并作为数据库打开，模拟「恢复后应用读回备份」。
func openSnapshotFile(t *testing.T, data []byte) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "restored.db")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("ping snapshot: %v", err)
	}
	return db
}

// 快照先是合法的 SQLite 文件（头 16 字节），再能被读回：应用还在运行（写连接不关、
// 行还留在 WAL 里）时，刚提交的日记必须出现在快照里。
func TestWriteSQLiteSnapshotReadableWhileWriterLive(t *testing.T) {
	dataDir := t.TempDir()
	live, err := Open(dataDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer live.Close()

	u, err := live.CreateUser("snapuser", "snap@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	const wantContent = "written while the app keeps running"
	if _, _, err := live.UpsertDiary(u.ID, "2026-10-01", wantContent, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("upsert diary: %v", err)
	}
	exp, err := OpenForExport(dataDir)
	if err != nil {
		t.Fatalf("open for export: %v", err)
	}
	defer exp.Close()

	var buf bytes.Buffer
	if err := exp.WriteSQLiteSnapshot(&buf); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	snap := buf.Bytes()
	if len(snap) < 16 || string(snap[:16]) != sqliteMagic {
		t.Fatalf("snapshot header = %q, want %q", snap[:min(len(snap), 16)], sqliteMagic)
	}

	db := openSnapshotFile(t, snap)
	var gotOwner, gotContent string
	// date 列存的是 '2026-10-01 00:00:00.000Z' 形态，按内容取回这一行即可。
	if err := db.QueryRow(`SELECT owner, content FROM diaries WHERE owner = ?`, u.ID).Scan(&gotOwner, &gotContent); err != nil {
		t.Fatalf("read diary from snapshot: %v", err)
	}
	if gotOwner != u.ID || gotContent != wantContent {
		t.Fatalf("diary from snapshot = (%q, %q), want (%q, %q)", gotOwner, gotContent, u.ID, wantContent)
	}
	var check string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&check); err != nil {
		t.Fatalf("integrity check: %v", err)
	}
	if check != "ok" {
		t.Fatalf("integrity_check = %q, want ok", check)
	}
}

// 数据文件不存在就是路径配错：导出必须报错，绝不建目录、绝不凭空造一个空库。
func TestOpenForExportMissingDB(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := OpenForExport(dataDir); err == nil {
		t.Fatal("OpenForExport on missing database should fail")
	}
	if fileExists(filepath.Join(dataDir, DatabaseName)) {
		t.Fatal("failed export must not create the database file")
	}
}

// 旧版 data.db 不回退：应用一启动就会迁移出 diarum.db，回退只会备份一个
// 主进程早已不再使用的文件。
func TestOpenForExportDoesNotFallBackToLegacyName(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, LegacyDatabaseName), []byte("legacy"), 0o600); err != nil {
		t.Fatalf("write legacy db: %v", err)
	}
	if _, err := OpenForExport(dataDir); err == nil {
		t.Fatal("OpenForExport should refuse to back up only the legacy database")
	}
}

// 导出不迁移、不改写：旧 schema 的文件原样进、原样出，源文件 sha256 前后一致。
func TestOpenForExportDoesNotMigrate(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, DatabaseName)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE legacy_probe(id INTEGER PRIMARY KEY, note TEXT)`); err != nil {
		t.Fatalf("create fixture table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO legacy_probe(note) VALUES ('old schema')`); err != nil {
		t.Fatalf("insert fixture row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture db: %v", err)
	}

	before := sha256File(t, path)

	exp, err := OpenForExport(dataDir)
	if err != nil {
		t.Fatalf("open for export: %v", err)
	}
	var buf bytes.Buffer
	if err := exp.WriteSQLiteSnapshot(&buf); err != nil {
		exp.Close()
		t.Fatalf("write snapshot: %v", err)
	}
	if err := exp.Close(); err != nil {
		t.Fatalf("close export: %v", err)
	}

	if after := sha256File(t, path); after != before {
		t.Fatal("export connection rewrote the source database")
	}

	snap := openSnapshotFile(t, buf.Bytes())
	rows, err := snap.Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tables: %v", err)
	}
	if len(tables) != 1 || tables[0] != "legacy_probe" {
		t.Fatalf("snapshot tables = %v, want only [legacy_probe]（导出不得补建应用 schema）", tables)
	}
	var note string
	if err := snap.QueryRow(`SELECT note FROM legacy_probe WHERE id = 1`).Scan(&note); err != nil {
		t.Fatalf("read fixture row from snapshot: %v", err)
	}
	if note != "old schema" {
		t.Fatalf("fixture row = %q, want %q", note, "old schema")
	}
}
