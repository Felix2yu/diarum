package store

import (
	"archive/zip"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// 本文件服务于导出子命令 `diarum backup`：外部备份工具（docker-db-auto-backup 等）
// 经 docker exec 调起本进程，把 stdout 当备份产物，所以这里只读、不迁移、不建目录，
// 产物一律先落临时目录再流式输出。

// 压缩包内部的固定布局。回档时按同样的名字把各样东西放回原处，改这里要同步改 README。
const (
	// ZipDBEntry 是放在包根的库快照文件名。
	ZipDBEntry = "diarum.db"
	// ZipStorageDir 是本地媒体在包内的目录前缀（库外状态：media 字节）。
	ZipStorageDir = "storage/"
	// ZipVectorsDir 是向量库在包内的目录前缀（库外状态：chromem 持久化目录）。
	ZipVectorsDir = "vectors/"
)

// OpenForExport 以「只导出不改动」的方式打开数据目录里的数据库。
// 与 Open 的区别只有一点：不建目录、不跑迁移与种子写入。
//
// 导出进程由外部备份工具（docker exec）在应用运行时触发：表结构升级只应由应用
// 进程自己执行，一个「顺手打开数据库」的导出进程不该改 schema，更不该与主进程抢写。
// 数据文件不存在时直接报错——那是路径配错或应用尚未首次启动，导出永远不该凭空
// 造出一个空库；旧版 data.db 也不回退：应用一启动就会把它迁移成 diarum.db，
// 回退只会备份一个主进程早已不再使用的文件。
func OpenForExport(dataDir string) (*Store, error) {
	path := filepath.Join(dataDir, DatabaseName)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("数据文件不可用: %w", err)
	}
	// 刻意不设 journal_mode / foreign_keys：导出连接不回写库头；已有的 WAL 数据照常读取。
	// file: 前缀让 modernc 驱动把路径与参数分开解析（与 openSQLite 的裸路径拼法等价）。
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1) // 导出是顺序读，一条连接足够，也少开一份 -shm 句柄。
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	return &Store{DB: db, DataDir: dataDir}, nil
}

// WriteSQLiteSnapshot 把数据库以 SQLite 自身的一致性快照写到 w。
//
// 用 VACUUM INTO 而不是复制文件：热拷贝正在被写入的 db/-wal/-shm 不是原子操作，
// 可能拷出损坏的副本或漏掉 WAL 里已提交的数据；VACUUM INTO 由 SQLite 在事务里
// 生成一份自洽的快照，WAL 模式下也不阻塞写入。
//
// 快照先落在临时目录再流式输出：VACUUM INTO 要求目标不存在，且这样无论
// 调用方读到一半断开，都不会在数据目录里留下半成品。
func (s *Store) WriteSQLiteSnapshot(w io.Writer) error {
	dir, err := os.MkdirTemp("", "diarum-snapshot-")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(dir)

	target := filepath.Join(dir, "snapshot.db")
	if err := s.vacuumInto(target); err != nil {
		return err
	}
	return streamFile(target, w)
}

// WriteZIP 把「库快照 + 库外文件」打成 zip 流式写到 w，返回打包的库外文件个数。
//
// diarum 有两处真实的库外状态：本地媒体字节（storage/，默认 provider=local 时
// 图片只落盘不进库）与 chromem 向量持久化（vectors/）。只备份 .db 会丢附件，
// 只拷目录会丢账本，所以 zip 形态才是「一份产物能还原全部」。
// dataDir/backups/ 里的按用户导出 zip 是库内容的派生物，不进包：备份自己的备份
// 只会让产物无限膨胀。
//
// zip.Writer 本身就是流式的：逐个文件拷进去，整包不在内存里攒。
func (s *Store) WriteZIP(w io.Writer) (int, error) {
	dir, err := os.MkdirTemp("", "diarum-zip-")
	if err != nil {
		return 0, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(dir)

	snap := filepath.Join(dir, "snapshot.db")
	if err := s.vacuumInto(snap); err != nil {
		return 0, err
	}

	zw := zip.NewWriter(w)
	if err := zipOneFile(zw, ZipDBEntry, snap); err != nil {
		_ = zw.Close() // 尽力写完中央目录，让半截包在解包时还能报出前缀
		return 0, err
	}
	n := 0
	for _, item := range []struct{ prefix, dir string }{
		{ZipStorageDir, filepath.Join(s.DataDir, "storage")},
		{ZipVectorsDir, filepath.Join(s.DataDir, "vectors")},
	} {
		c, err := zipDir(zw, item.prefix, item.dir)
		n += c
		if err != nil {
			_ = zw.Close()
			return n, err
		}
	}
	if err := zw.Close(); err != nil {
		return n, fmt.Errorf("完成压缩包失败: %w", err)
	}
	return n, nil
}

// vacuumInto 用 VACUUM INTO 把当前库导出为磁盘上的一个自洽快照文件（目标须不存在）。
func (s *Store) vacuumInto(target string) error {
	if _, err := s.DB.Exec("VACUUM INTO ?", target); err != nil {
		return fmt.Errorf("导出数据库快照失败: %w", err)
	}
	return nil
}

// streamFile 把整个文件拷进 w。
func streamFile(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("读取快照失败: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("写出快照失败: %w", err)
	}
	return nil
}

// zipOneFile 把磁盘上的一个文件作为指定条目写进压缩包。
func zipOneFile(zw *zip.Writer, name, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", name, err)
	}
	defer f.Close()
	hw, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("创建压缩包条目 %s 失败: %w", name, err)
	}
	if _, err := io.Copy(hw, f); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", name, err)
	}
	return nil
}

// zipDir 把 dir 下的普通文件按相对路径写进 prefix 前缀之下，返回打包个数。
//
// 目录不存在返回 0 而不是报错——一个还没上传过图片、没建过向量库的部署是正常的。
// 非常规文件（符号链接、套接字之类）跳过：备份包里塞一条指向数据目录之外的链接，
// 回档时就成了越界写。
func zipDir(zw *zip.Writer, prefix, dir string) (int, error) {
	count := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		// 包内路径必须用正斜杠：zip 规范如此，filepath 的分隔符在别的平台上读不出来
		if err := zipOneFile(zw, prefix+filepath.ToSlash(rel), p); err != nil {
			return err
		}
		count++
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	return count, err
}
