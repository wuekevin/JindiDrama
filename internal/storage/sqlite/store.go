// Package sqlite 是本地存储：收藏、观看历史、搜索历史、设置。WAL 模式，带版本化迁移、
// 可回滚与备份。纯 Go 驱动（modernc），无需 cgo。
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type migration struct {
	version  int
	up, down string
}

var migrations = []migration{
	{1, `CREATE TABLE favorites(id INTEGER PRIMARY KEY AUTOINCREMENT, account_scope TEXT NOT NULL DEFAULT 'local', drama_id TEXT NOT NULL, created_at INTEGER NOT NULL, dirty INTEGER NOT NULL DEFAULT 0, UNIQUE(account_scope, drama_id));
CREATE TABLE history(id INTEGER PRIMARY KEY AUTOINCREMENT, account_scope TEXT NOT NULL DEFAULT 'local', drama_id TEXT NOT NULL, episode INTEGER NOT NULL, position_ms INTEGER NOT NULL DEFAULT 0, duration_ms INTEGER NOT NULL DEFAULT 0, percent INTEGER NOT NULL DEFAULT 0, watched_at INTEGER NOT NULL, dirty INTEGER NOT NULL DEFAULT 0, UNIQUE(account_scope, drama_id));
CREATE TABLE search_history(id INTEGER PRIMARY KEY AUTOINCREMENT, query TEXT NOT NULL UNIQUE, searched_at INTEGER NOT NULL);
CREATE TABLE settings(key TEXT PRIMARY KEY, value_json TEXT NOT NULL, updated_at INTEGER NOT NULL);`,
		`DROP TABLE settings; DROP TABLE search_history; DROP TABLE history; DROP TABLE favorites;`},
	{2, `CREATE INDEX idx_history_watched ON history(account_scope, watched_at DESC); CREATE INDEX idx_fav_created ON favorites(account_scope, created_at DESC);`,
		`DROP INDEX idx_history_watched; DROP INDEX idx_fav_created;`},
}

const scope = "local"

type Store struct {
	db   *sql.DB
	Path string
}

type HistoryEntry struct {
	DramaID    string `json:"dramaId"`
	Episode    int    `json:"episode"`
	PositionMs int64  `json:"positionMs"`
	DurationMs int64  `json:"durationMs"`
	Percent    int    `json:"percent"`
	WatchedAt  int64  `json:"watchedAt"`
}

func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // 单连接避免 :memory: 与 WAL 写冲突
	s := &Store{db: db, Path: path}
	for _, p := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000"} {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	if err := s.Migrate(len(migrations)); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Version() (int, error) {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return 0, err
	}
	var v sql.NullInt64
	err := s.db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v)
	return int(v.Int64), err
}

// Migrate 升级或回滚到 target 版本；每步在事务中执行。
func (s *Store) Migrate(target int) error {
	cur, err := s.Version()
	if err != nil {
		return err
	}
	if target < 0 || target > len(migrations) {
		return fmt.Errorf("未知版本 %d", target)
	}
	for cur < target {
		m := migrations[cur]
		if err := s.apply(m.up, func(tx *sql.Tx) error {
			_, e := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?,?)`, m.version, time.Now().Unix())
			return e
		}); err != nil {
			return fmt.Errorf("迁移 v%d 失败: %w", m.version, err)
		}
		cur++
	}
	for cur > target {
		m := migrations[cur-1]
		if err := s.apply(m.down, func(tx *sql.Tx) error {
			_, e := tx.Exec(`DELETE FROM schema_migrations WHERE version=?`, m.version)
			return e
		}); err != nil {
			return fmt.Errorf("回滚 v%d 失败: %w", m.version, err)
		}
		cur--
	}
	return nil
}

func (s *Store) apply(script string, record func(*sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(script); err != nil {
		return err
	}
	if err := record(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Backup 使用 VACUUM INTO 生成一致性备份，目标必须不存在（不覆盖）。
func (s *Store) Backup(dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return errors.New("备份目标已存在，拒绝覆盖")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	_, err := s.db.Exec(`VACUUM INTO ?`, dest)
	return err
}

// ---- 收藏 ----
func (s *Store) Favorites() ([]string, error) {
	rows, err := s.db.Query(`SELECT drama_id FROM favorites WHERE account_scope=? ORDER BY created_at DESC, id DESC`, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) SetFavorites(ids []string) error { // 全量替换（保持前端顺序），事务内完成
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM favorites WHERE account_scope=?`, scope); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	for i, id := range ids {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO favorites(account_scope, drama_id, created_at) VALUES(?,?,?)`, scope, id, now-int64(i)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---- 历史 ----
func (s *Store) History() ([]HistoryEntry, error) {
	rows, err := s.db.Query(`SELECT drama_id, episode, position_ms, duration_ms, percent, watched_at FROM history WHERE account_scope=? ORDER BY watched_at DESC`, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HistoryEntry{}
	for rows.Next() {
		var h HistoryEntry
		if err := rows.Scan(&h.DramaID, &h.Episode, &h.PositionMs, &h.DurationMs, &h.Percent, &h.WatchedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) RecordHistory(h HistoryEntry) error {
	if h.DramaID == "" || h.Episode < 1 {
		return errors.New("无效的历史记录")
	}
	if h.Percent < 0 {
		h.Percent = 0
	}
	if h.Percent > 100 {
		h.Percent = 100
	}
	if h.WatchedAt == 0 {
		h.WatchedAt = time.Now().UnixMilli()
	}
	_, err := s.db.Exec(`INSERT INTO history(account_scope, drama_id, episode, position_ms, duration_ms, percent, watched_at) VALUES(?,?,?,?,?,?,?)
ON CONFLICT(account_scope, drama_id) DO UPDATE SET episode=excluded.episode, position_ms=excluded.position_ms, duration_ms=excluded.duration_ms, percent=excluded.percent, watched_at=excluded.watched_at`,
		scope, h.DramaID, h.Episode, h.PositionMs, h.DurationMs, h.Percent, h.WatchedAt)
	return err
}

func (s *Store) RemoveHistory(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM history WHERE account_scope=? AND drama_id=?`, scope, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ClearHistory() error {
	_, err := s.db.Exec(`DELETE FROM history WHERE account_scope=?`, scope)
	return err
}

// ---- 搜索历史 ----
func (s *Store) SearchHistory() ([]string, error) {
	rows, err := s.db.Query(`SELECT query FROM search_history ORDER BY searched_at DESC LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var q string
		if err := rows.Scan(&q); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *Store) SetSearchHistory(qs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM search_history`); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	for i, q := range qs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO search_history(query, searched_at) VALUES(?,?)`, q, now-int64(i)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---- 设置（JSON 文本，key 白名单由上层控制）----
func (s *Store) GetSetting(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value_json FROM settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s *Store) SetSetting(key, valueJSON string) error {
	_, err := s.db.Exec(`INSERT INTO settings(key, value_json, updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json, updated_at=excluded.updated_at`, key, valueJSON, time.Now().UnixMilli())
	return err
}

var _ = context.Background
