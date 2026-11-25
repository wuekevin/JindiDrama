package sqlite

import (
	"errors"
	"os"
	"slices"
	"strings"
)

type LibraryRestore struct {
	State          LibraryState `json:"state"`
	FavoritesAdded int          `json:"favoritesAdded"`
	HistoryAdded   int          `json:"historyAdded"`
}

// RestoreMissingLibrary runs against the current transaction snapshot, so even
// changes committed after the backup was picked take precedence.
func (s *Store) RestoreMissingLibrary(backup *LibraryState) (*LibraryRestore, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	favorites, err := readFavorites(tx)
	if err != nil {
		return nil, err
	}
	history, err := readHistory(tx, true)
	if err != nil {
		return nil, err
	}
	added := []string{}
	for _, id := range backup.Favorites {
		if !slices.Contains(favorites, id) && !slices.Contains(added, id) {
			added = append(added, id)
		}
	}
	if len(added) > 0 {
		if err := applyFavoriteIntent(tx, added, nil, slices.Concat(favorites, added)); err != nil {
			return nil, err
		}
	}
	known := make(map[string]bool, len(history))
	for _, entry := range history {
		known[entry.DramaID] = true
	}
	count := 0
	for _, entry := range backup.History {
		if known[entry.DramaID] {
			continue
		}
		if err := writeHistory(tx, entry, false); err != nil {
			return nil, err
		}
		known[entry.DramaID] = true
		count++
	}
	history, err = readHistory(tx, true)
	if err != nil {
		return nil, err
	}
	favorites, err = readFavorites(tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &LibraryRestore{
		State:          LibraryState{Favorites: favorites, History: history},
		FavoritesAdded: len(added), HistoryAdded: count,
	}, nil
}

// ReadLibraryBackup reads user library rows from an application backup without
// migrating it or copying settings and other tables into the live database.
func ReadLibraryBackup(path string) (*LibraryState, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("备份必须是普通文件")
	}
	db, err := openDatabase(path, "ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA trusted_schema=OFF"); err != nil {
		return nil, err
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var tables int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM pragma_table_list
WHERE "schema"='main' AND type='table' AND name IN ('schema_migrations','favorites','history')`).Scan(&tables); err != nil {
		return nil, err
	}
	if tables != 3 {
		return nil, errors.New("备份缺少收藏或历史表")
	}
	version, err := supportedVersion(tx)
	if err != nil {
		return nil, err
	}
	if version < 1 {
		return nil, errors.New("备份没有有效的版本记录")
	}
	var check string
	if err := tx.QueryRow("PRAGMA quick_check(1)").Scan(&check); err != nil {
		return nil, err
	}
	if check != "ok" {
		return nil, errors.New("备份完整性检查失败")
	}

	favoritesQuery := `SELECT drama_id FROM favorites WHERE account_scope=? ORDER BY created_at DESC, id DESC`
	if version >= 5 {
		favoritesQuery = `SELECT drama_id FROM favorites WHERE account_scope=? ORDER BY COALESCE(sort_order, created_at) DESC, id DESC`
	}
	rows, err := tx.Query(favoritesQuery, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	state := &LibraryState{Favorites: []string{}}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("备份包含无效的收藏")
		}
		state.Favorites = append(state.Favorites, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	state.History, err = readHistory(tx, version >= historyEpisodeCountVersion)
	if err != nil {
		return nil, err
	}
	for _, entry := range state.History {
		if strings.TrimSpace(entry.DramaID) == "" || entry.Episode < 1 || entry.PositionMs < 0 ||
			entry.DurationMs < 0 || entry.Percent < 0 || entry.Percent > 100 || entry.WatchedAt < 0 || !validHistoryEpisodeCount(entry) {
			return nil, errors.New("备份包含无效的观看历史")
		}
	}
	return state, tx.Commit()
}
