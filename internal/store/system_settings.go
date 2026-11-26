package store

import (
	"database/sql"
	"errors"
)

// migrateSystemSettingsSchema 创建实例级键值设置表。
func migrateSystemSettingsSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS system_settings (
		key TEXT PRIMARY KEY NOT NULL,
		value TEXT NOT NULL,
		updated TEXT NOT NULL
	)`)
	return err
}

// GetSystemSetting 读取实例级设置；ok 为 false 表示该键从未设置过。
func (s *Store) GetSystemSetting(key string) (value string, ok bool, err error) {
	err = s.DB.QueryRow(`SELECT value FROM system_settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// SetSystemSetting 写入实例级设置。
func (s *Store) SetSystemSetting(key, value string) error {
	_, err := s.DB.Exec(
		`INSERT INTO system_settings(key, value, updated) VALUES(?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated = excluded.updated`,
		key, value, nowString(),
	)
	return err
}
