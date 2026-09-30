package store

import (
	"database/sql"
	"time"

	"github.com/songtianlun/diarum/internal/logger"
)

// DiaryVersion 是日记在一次编辑会话开始前的快照。
// 列表接口只返回摘要（Content 为空，Preview/ContentLength 供展示），
// 通过 GetDiaryVersion 获取完整内容。
type DiaryVersion struct {
	ID            string   `json:"id"`
	DiaryID       string   `json:"diary_id"`
	Date          string   `json:"date"`
	EditSessionID string   `json:"-"`
	Content       string   `json:"content"`
	Preview       string   `json:"preview"`
	ContentLength int      `json:"content_length"`
	Mood          int      `json:"mood"`
	MoodStates    []string `json:"mood_states"`
	Scenarios     []string `json:"scenarios"`
	Weather       string   `json:"weather"`
	City          string   `json:"city"`
	TempMin       float64  `json:"temp_min"`
	TempMax       float64  `json:"temp_max"`
	Tags          []string `json:"tags"`
	Created       string   `json:"created"`
}

// recordDiaryVersion 在编辑会话首次修改正文前把旧内容快照为一个版本。
// 依赖 (diary_id, edit_session_id) 唯一索引实现"同一会话只记一次"（INSERT OR IGNORE）。
func (s *Store) recordDiaryVersion(d *Diary, editSessionID string) error {
	if editSessionID == "" || d == nil {
		return nil
	}
	id, err := GenerateID()
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(
		`INSERT OR IGNORE INTO diary_versions(id, owner, diary_id, date, edit_session_id, content, mood, mood_states, scenarios, weather, city, temp_min, temp_max, tags, created)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, d.Owner, d.ID, DateOnly(d.Date), editSessionID, d.Content, d.Mood,
		encodeJSON(d.MoodStates), encodeJSON(d.Scenarios), d.Weather, d.City,
		d.TempMin, d.TempMax, encodeJSON(d.Tags), nowString(),
	)
	return err
}

// ListDiaryVersions 返回日记的版本摘要（按时间倒序），并先清理过期版本。
func (s *Store) ListDiaryVersions(owner, diaryID string, retentionDays int) ([]DiaryVersion, error) {
	_ = s.PurgeExpiredDiaryVersions(owner, retentionDays)
	rows, err := s.DB.Query(
		`SELECT id, diary_id, date, edit_session_id, substr(content, 1, 200), length(content), created
		 FROM diary_versions WHERE owner = ? AND diary_id = ?
		 ORDER BY created DESC, id DESC`,
		owner, diaryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := make([]DiaryVersion, 0)
	for rows.Next() {
		v := DiaryVersion{}
		if err := rows.Scan(&v.ID, &v.DiaryID, &v.Date, &v.EditSessionID, &v.Preview, &v.ContentLength, &v.Created); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// GetDiaryVersion 返回单个版本的完整内容，仅限 owner 可见。
func (s *Store) GetDiaryVersion(owner, versionID string) (*DiaryVersion, error) {
	v := &DiaryVersion{}
	var moodStatesRaw, scenariosRaw, tagsRaw string
	err := s.DB.QueryRow(
		`SELECT id, diary_id, date, edit_session_id, content, length(content), mood, mood_states, scenarios, weather, city, temp_min, temp_max, tags, created
		 FROM diary_versions WHERE id = ? AND owner = ?`,
		versionID, owner,
	).Scan(&v.ID, &v.DiaryID, &v.Date, &v.EditSessionID, &v.Content, &v.ContentLength, &v.Mood,
		&moodStatesRaw, &scenariosRaw, &v.Weather, &v.City, &v.TempMin, &v.TempMax, &tagsRaw, &v.Created)
	if err != nil {
		return nil, err
	}
	v.MoodStates = decodeStringSlice(moodStatesRaw)
	v.Scenarios = decodeStringSlice(scenariosRaw)
	v.Tags = decodeStringSlice(tagsRaw)
	return v, nil
}

// RestoreDiaryVersion 把日记回写为指定版本的内容。
// 恢复前会把当前内容另存为一个版本（restore- 前缀会话），保证恢复操作本身可撤销。
func (s *Store) RestoreDiaryVersion(owner, versionID string) (*Diary, error) {
	version, err := s.GetDiaryVersion(owner, versionID)
	if err != nil {
		return nil, err
	}
	diary, err := s.GetDiaryByID(version.DiaryID)
	if err != nil {
		return nil, err
	}
	if diary.Owner != owner {
		return nil, sql.ErrNoRows
	}
	contentChanged := diary.Content != version.Content
	if !contentChanged &&
		diary.Mood == version.Mood &&
		diary.Weather == version.Weather &&
		diary.City == version.City &&
		diary.TempMin == version.TempMin &&
		diary.TempMax == version.TempMax {
		// 内容与元数据一致：no-op，不产生新版本
		return diary, nil
	}
	if contentChanged {
		// 恢复前快照当前内容，使"恢复"也可撤销；restore- 会话每次唯一，不与编辑会话去重冲突
		if sessionID, err := GenerateID(); err == nil {
			if err := s.recordDiaryVersion(diary, "restore-"+sessionID); err != nil {
				logger.Warn("[Store] failed to snapshot diary before restore: %v", err)
			}
		}
	}
	now := nowString()
	contentUpdated := diary.ContentUpdated
	if contentChanged {
		contentUpdated = now
	}
	_, err = s.DB.Exec(
		`UPDATE diaries SET content = ?, content_updated = ?, mood = ?, mood_states = ?, scenarios = ?, weather = ?, city = ?, temp_min = ?, temp_max = ?, tags = ?, updated = ? WHERE id = ? AND owner = ?`,
		version.Content, contentUpdated, version.Mood, encodeJSON(version.MoodStates), encodeJSON(version.Scenarios),
		version.Weather, version.City, version.TempMin, version.TempMax, encodeJSON(version.Tags), now,
		version.DiaryID, owner,
	)
	if err != nil {
		return nil, err
	}
	return s.GetDiaryByID(version.DiaryID)
}

// PurgeExpiredDiaryVersions 删除超过保留期的版本。retentionDays <= 0 视为不清理。
func (s *Store) PurgeExpiredDiaryVersions(owner string, retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format("2006-01-02 15:04:05.000Z")
	_, err := s.DB.Exec(`DELETE FROM diary_versions WHERE owner = ? AND created < ?`, owner, cutoff)
	return err
}
