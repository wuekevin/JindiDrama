// Package sqlite 是本地存储：收藏、观看历史、搜索历史、设置。WAL 模式，带版本化迁移、
// 可回滚与备份。纯 Go 驱动（modernc），无需 cgo。
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	{3, `CREATE TABLE local_danmaku(id INTEGER PRIMARY KEY, drama_id TEXT NOT NULL, episode INTEGER NOT NULL, offset_ms INTEGER NOT NULL, text TEXT NOT NULL);
CREATE INDEX idx_local_danmaku_episode ON local_danmaku(drama_id, episode, id);`,
		`DROP TABLE local_danmaku;`},
	{4, `CREATE TABLE local_comments(drama_id TEXT NOT NULL, episode INTEGER NOT NULL, snapshot TEXT NOT NULL,
PRIMARY KEY(drama_id, episode));`, `DROP TABLE local_comments;`},
	{5, `ALTER TABLE favorites ADD COLUMN sort_order INTEGER;`, `
WITH ordered AS MATERIALIZED (
  SELECT id, MAX(created_at) OVER (PARTITION BY account_scope)
    - ROW_NUMBER() OVER (PARTITION BY account_scope ORDER BY COALESCE(sort_order, created_at) DESC, id DESC) + 1 AS legacy_order
  FROM favorites
  WHERE account_scope IN (SELECT account_scope FROM favorites WHERE sort_order IS NOT NULL)
)
UPDATE favorites SET created_at = (SELECT legacy_order FROM ordered WHERE ordered.id = favorites.id)
WHERE id IN (SELECT id FROM ordered);
ALTER TABLE favorites DROP COLUMN sort_order;`},
	{6, `CREATE TABLE library_receipts(writer TEXT PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision > 0));`, `DROP TABLE library_receipts;`},
	{historyEpisodeCountVersion, `ALTER TABLE history ADD COLUMN episode_count_at_watch INTEGER NOT NULL DEFAULT 0
CHECK (episode_count_at_watch = 0 OR episode_count_at_watch BETWEEN episode AND 9007199254740991);`, `ALTER TABLE history DROP COLUMN episode_count_at_watch;`},
}

const (
	scope                      = "local"
	historyEpisodeCountVersion = 7
)

type Store struct {
	db   *sql.DB
	Path string
}

type HistoryEntry struct {
	DramaID             string `json:"dramaId"`
	Episode             int    `json:"episode"`
	PositionMs          int64  `json:"positionMs"`
	DurationMs          int64  `json:"durationMs"`
	Percent             int    `json:"percent"`
	WatchedAt           int64  `json:"watchedAt"`
	EpisodeCountAtWatch int    `json:"episodeCountAtWatch,omitempty"`
}

func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := preflightVersion(path); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
	}
	db, err := openDatabase(path, "rwc")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, Path: path}
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON"} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	if err := s.Migrate(len(migrations)); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func openDatabase(path, mode string) (*sql.DB, error) {
	dsn := path
	if path != ":memory:" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err == nil {
			absolute = resolved
		} else if mode != "rwc" || !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		uri := url.URL{Scheme: "file", RawQuery: "mode=" + mode}
		if mode != "ro" {
			// Our transactions write. Reserve the lock before reading the schema
			// or library state so concurrent writers do not promote stale snapshots.
			uri.RawQuery += "&_txlock=immediate"
		}
		if strings.HasPrefix(absolute, `\\?\`) {
			// Keep the Win32 extended-length namespace intact through URI decoding.
			uri.Opaque = url.PathEscape(absolute)
		} else {
			diskPath := filepath.ToSlash(absolute)
			if !strings.HasPrefix(diskPath, "/") {
				diskPath = "/" + diskPath
			}
			uri.Path = diskPath
		}
		dsn = uri.String()
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func preflightVersion(path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	db, err := openDatabase(path, "ro")
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = supportedVersion(db)
	return err
}

func (s *Store) Close() error { return s.db.Close() }

type schemaReader interface {
	QueryRow(string, ...any) *sql.Row
}

func (s *Store) Version() (int, error) { return schemaVersion(s.db) }

func schemaVersion(db schemaReader) (int, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='schema_migrations')`).Scan(&exists)
	if err != nil || !exists {
		return 0, err
	}
	var count, first, last int
	err = db.QueryRow(`SELECT COUNT(*), COALESCE(MIN(version), 0), COALESCE(MAX(version), 0) FROM schema_migrations`).
		Scan(&count, &first, &last)
	if err != nil {
		return 0, err
	}
	if count != 0 && (first != 1 || count != last) {
		return 0, errors.New("数据库迁移历史不连续")
	}
	return last, nil
}

type UnsupportedSchemaError struct {
	Found, Supported int
}

func (e *UnsupportedSchemaError) Error() string {
	return fmt.Sprintf("数据库版本 v%d 高于此应用支持的 v%d", e.Found, e.Supported)
}

func supportedVersion(db schemaReader) (int, error) {
	version, err := schemaVersion(db)
	if err == nil && version > len(migrations) {
		return 0, &UnsupportedSchemaError{Found: version, Supported: len(migrations)}
	}
	return version, err
}

// Migrate changes the full schema and history in a single SQLite transaction.
func (s *Store) Migrate(target int) error {
	if target < 0 || target > len(migrations) {
		return fmt.Errorf("未知版本 %d", target)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := supportedVersion(tx)
	if err != nil || current == target {
		return err
	}
	if current == 0 {
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
			return err
		}
	}
	for current < target {
		m := migrations[current]
		if _, err := tx.Exec(m.up); err != nil {
			return fmt.Errorf("迁移 v%d 失败: %w", m.version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?,?)`, m.version, time.Now().Unix()); err != nil {
			return err
		}
		current++
	}
	for current > target {
		m := migrations[current-1]
		if _, err := tx.Exec(m.down); err != nil {
			return fmt.Errorf("回滚 v%d 失败: %w", m.version, err)
		}
		if _, err := tx.Exec(`DELETE FROM schema_migrations WHERE version=?`, m.version); err != nil {
			return err
		}
		current--
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
	return readFavorites(s.db)
}

func readFavorites(db interface {
	Query(string, ...any) (*sql.Rows, error)
}) ([]string, error) {
	rows, err := db.Query(`SELECT drama_id FROM favorites WHERE account_scope=? ORDER BY COALESCE(sort_order, created_at) DESC, id DESC`, scope)
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
	if err := setFavorites(tx, ids); err != nil {
		return err
	}
	return tx.Commit()
}

func setFavorites(tx *sql.Tx, ids []string) error {
	if _, err := tx.Exec(`DELETE FROM favorites WHERE account_scope=?`, scope); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	for i, id := range ids {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO favorites(account_scope, drama_id, created_at) VALUES(?,?,?)`, scope, id, now-int64(i)); err != nil {
			return err
		}
	}
	return nil
}

// ---- 历史 ----
func (s *Store) History() ([]HistoryEntry, error) {
	return readHistory(s.db, true)
}

func readHistory(db interface {
	Query(string, ...any) (*sql.Rows, error)
}, includeEpisodeCount bool) ([]HistoryEntry, error) {
	episodeCount := "0"
	if includeEpisodeCount {
		episodeCount = "episode_count_at_watch"
	}
	rows, err := db.Query(`SELECT drama_id, episode, position_ms, duration_ms, percent, watched_at, `+episodeCount+`
FROM history WHERE account_scope=? ORDER BY watched_at DESC`, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HistoryEntry{}
	for rows.Next() {
		var h HistoryEntry
		if err := rows.Scan(&h.DramaID, &h.Episode, &h.PositionMs, &h.DurationMs, &h.Percent, &h.WatchedAt, &h.EpisodeCountAtWatch); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

type historyWriter interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func (s *Store) RecordHistory(h HistoryEntry) error {
	return recordHistory(s.db, h)
}

func recordHistory(db historyWriter, h HistoryEntry) error {
	return writeHistory(db, h, false)
}

func writeHistory(db historyWriter, h HistoryEntry, preserveNewer bool) error {
	if h.DramaID == "" || h.Episode < 1 || !validHistoryEpisodeCount(h) {
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
	_, err := db.Exec(`INSERT INTO history(account_scope, drama_id, episode, position_ms, duration_ms, percent, watched_at, episode_count_at_watch) VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(account_scope, drama_id) DO UPDATE SET episode=excluded.episode, position_ms=excluded.position_ms, duration_ms=excluded.duration_ms, percent=excluded.percent, watched_at=excluded.watched_at,
episode_count_at_watch=excluded.episode_count_at_watch
WHERE NOT ? OR excluded.watched_at >= history.watched_at`,
		scope, h.DramaID, h.Episode, h.PositionMs, h.DurationMs, h.Percent, h.WatchedAt, h.EpisodeCountAtWatch, preserveNewer)
	return err
}

func validHistoryEpisodeCount(h HistoryEntry) bool {
	return h.EpisodeCountAtWatch == 0 || (h.EpisodeCountAtWatch >= h.Episode && int64(h.EpisodeCountAtWatch) <= 1<<53-1)
}

func (s *Store) RemoveHistory(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := removeHistory(tx, ids); err != nil {
		return err
	}
	return tx.Commit()
}

func removeHistory(tx *sql.Tx, ids []string) error {
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM history WHERE account_scope=? AND drama_id=?`, scope, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ClearHistory() error {
	_, err := s.db.Exec(`DELETE FROM history WHERE account_scope=?`, scope)
	return err
}

// ---- 搜索历史 ----
const searchHistoryRevisionKey = "search_history_revision"

type SearchHistoryState struct {
	Terms    []string `json:"terms"`
	Revision int64    `json:"revision"`
}

func (s *Store) SearchHistory() ([]string, error) {
	state, err := s.SearchHistoryState()
	return state.Terms, err
}

func (s *Store) SearchHistoryState() (SearchHistoryState, error) {
	state := SearchHistoryState{Terms: []string{}}
	// One SQLite statement reads terms and their revision from the same snapshot,
	// including the case where the last accepted change cleared every term.
	rows, err := s.db.Query(`SELECT current.revision, recent.query
FROM (SELECT COALESCE((SELECT CAST(value_json AS INTEGER) FROM settings WHERE key=?), 0) AS revision) current
LEFT JOIN (SELECT query, searched_at FROM search_history ORDER BY searched_at DESC LIMIT 10) recent ON TRUE
ORDER BY recent.searched_at DESC`, searchHistoryRevisionKey)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	for rows.Next() {
		var q sql.NullString
		if err := rows.Scan(&state.Revision, &q); err != nil {
			return state, err
		}
		if q.Valid {
			state.Terms = append(state.Terms, q.String)
		}
	}
	return state, rows.Err()
}

func (s *Store) SetSearchHistory(qs []string) error {
	_, err := s.setSearchHistory(qs, 0)
	return err
}

func (s *Store) SetSearchHistoryRevision(state SearchHistoryState) (bool, error) {
	if state.Revision < 1 || state.Revision >= 1<<53 {
		return false, errors.New("无效的搜索历史版本")
	}
	return s.setSearchHistory(state.Terms, state.Revision)
}

func (s *Store) setSearchHistory(qs []string, revision int64) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	receipt, err := tx.Exec(`INSERT INTO settings(key, value_json, updated_at) VALUES(?, MAX(1, ?), ?)
ON CONFLICT(key) DO UPDATE SET value_json=CASE WHEN ?=0 THEN CAST(settings.value_json AS INTEGER)+1 ELSE excluded.value_json END,
updated_at=excluded.updated_at WHERE ?=0 OR CAST(settings.value_json AS INTEGER) < CAST(excluded.value_json AS INTEGER)`,
		searchHistoryRevisionKey, revision, now, revision, revision)
	if err != nil {
		return false, err
	}
	claimed, err := receipt.RowsAffected()
	if err != nil || claimed == 0 {
		return false, err
	}
	if _, err := tx.Exec(`DELETE FROM search_history`); err != nil {
		return false, err
	}
	for i, q := range qs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO search_history(query, searched_at) VALUES(?,?)`, q, now-int64(i)); err != nil {
			return false, err
		}
	}
	err = tx.Commit()
	return err == nil, err
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
