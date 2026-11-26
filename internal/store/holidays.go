package store

import (
	"database/sql"
	"errors"
)

// HolidayYearRecord 保存某个地区某一年的法定节假日缓存。
type HolidayYearRecord struct {
	Region       string
	Year         int
	Payload      string
	ETag         string
	Source       string
	NotPublished bool
	FetchedAt    string
	Updated      string
}

// HolidayOverrideRecord 保存管理员（实例所有者）手工添加的节假日/调休覆盖。
type HolidayOverrideRecord struct {
	Region   string
	Date     string
	Year     int
	Name     string
	IsOffDay bool
	Updated  string
}

// migrateHolidaySchema 创建节假日缓存与手工覆盖表（幂等）。
func migrateHolidaySchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS holiday_years (
			region TEXT NOT NULL,
			year INTEGER NOT NULL,
			payload TEXT DEFAULT '' NOT NULL,
			etag TEXT DEFAULT '' NOT NULL,
			source TEXT DEFAULT '' NOT NULL,
			not_published BOOLEAN DEFAULT FALSE NOT NULL,
			fetched_at TEXT DEFAULT '' NOT NULL,
			updated TEXT DEFAULT '' NOT NULL,
			PRIMARY KEY (region, year)
		)`,
		`CREATE TABLE IF NOT EXISTS holiday_overrides (
			region TEXT NOT NULL,
			date TEXT NOT NULL,
			year INTEGER NOT NULL,
			name TEXT NOT NULL,
			is_off_day BOOLEAN DEFAULT TRUE NOT NULL,
			updated TEXT DEFAULT '' NOT NULL,
			PRIMARY KEY (region, date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_holiday_overrides_region_year ON holiday_overrides(region, year)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// GetHolidayYear 读取缓存的年度节假日数据；不存在时返回 ErrNotFound。
func (s *Store) GetHolidayYear(region string, year int) (*HolidayYearRecord, error) {
	var rec HolidayYearRecord
	err := s.DB.QueryRow(
		`SELECT region, year, payload, etag, source, not_published, fetched_at, updated
		 FROM holiday_years
		 WHERE region = ? AND year = ?`,
		region, year,
	).Scan(
		&rec.Region,
		&rec.Year,
		&rec.Payload,
		&rec.ETag,
		&rec.Source,
		&rec.NotPublished,
		&rec.FetchedAt,
		&rec.Updated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// UpsertHolidayYear 写入或更新年度节假日缓存。
func (s *Store) UpsertHolidayYear(rec *HolidayYearRecord) error {
	updated := rec.Updated
	if updated == "" {
		updated = nowString()
	}
	_, err := s.DB.Exec(
		`INSERT INTO holiday_years(region, year, payload, etag, source, not_published, fetched_at, updated)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(region, year) DO UPDATE SET
			payload = excluded.payload,
			etag = excluded.etag,
			source = excluded.source,
			not_published = excluded.not_published,
			fetched_at = excluded.fetched_at,
			updated = excluded.updated`,
		rec.Region,
		rec.Year,
		rec.Payload,
		rec.ETag,
		rec.Source,
		rec.NotPublished,
		rec.FetchedAt,
		updated,
	)
	return err
}

// ListHolidayYears 返回某地区所有缓存的年度数据，按年份升序。
func (s *Store) ListHolidayYears(region string) ([]HolidayYearRecord, error) {
	rows, err := s.DB.Query(
		`SELECT region, year, payload, etag, source, not_published, fetched_at, updated
		 FROM holiday_years
		 WHERE region = ?
		 ORDER BY year ASC`,
		region,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []HolidayYearRecord
	for rows.Next() {
		var rec HolidayYearRecord
		if err := rows.Scan(
			&rec.Region,
			&rec.Year,
			&rec.Payload,
			&rec.ETag,
			&rec.Source,
			&rec.NotPublished,
			&rec.FetchedAt,
			&rec.Updated,
		); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// UpsertHolidayOverride 新增或更新手工节假日/调休覆盖。
func (s *Store) UpsertHolidayOverride(rec *HolidayOverrideRecord) error {
	updated := rec.Updated
	if updated == "" {
		updated = nowString()
	}
	_, err := s.DB.Exec(
		`INSERT INTO holiday_overrides(region, date, year, name, is_off_day, updated)
		 VALUES(?, ?, ?, ?, ?, ?)
		 ON CONFLICT(region, date) DO UPDATE SET
			year = excluded.year,
			name = excluded.name,
			is_off_day = excluded.is_off_day,
			updated = excluded.updated`,
		rec.Region,
		rec.Date,
		rec.Year,
		rec.Name,
		rec.IsOffDay,
		updated,
	)
	return err
}

// DeleteHolidayOverride 按日期删除手工覆盖。
func (s *Store) DeleteHolidayOverride(region, date string) error {
	res, err := s.DB.Exec(
		`DELETE FROM holiday_overrides WHERE region = ? AND date = ?`,
		region, date,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListHolidayOverrides 返回某地区的手工覆盖。
// year > 0 时只返回该年份的记录。
func (s *Store) ListHolidayOverrides(region string, year int) ([]HolidayOverrideRecord, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if year > 0 {
		rows, err = s.DB.Query(
			`SELECT region, date, year, name, is_off_day, updated
			 FROM holiday_overrides
			 WHERE region = ? AND year = ?
			 ORDER BY date ASC`,
			region, year,
		)
	} else {
		rows, err = s.DB.Query(
			`SELECT region, date, year, name, is_off_day, updated
			 FROM holiday_overrides
			 WHERE region = ?
			 ORDER BY date ASC`,
			region,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []HolidayOverrideRecord
	for rows.Next() {
		var rec HolidayOverrideRecord
		if err := rows.Scan(
			&rec.Region,
			&rec.Date,
			&rec.Year,
			&rec.Name,
			&rec.IsOffDay,
			&rec.Updated,
		); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
