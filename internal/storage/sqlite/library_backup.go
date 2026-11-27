package sqlite

import (
	"errors"
	"os"
	"path/filepath"
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
	var added []string
	if len(backup.Favorites) != 0 {
		knownFavorites := make(map[string]struct{}, len(favorites))
		for _, id := range favorites {
			knownFavorites[id] = struct{}{}
		}
		for _, id := range backup.Favorites {
			if _, exists := knownFavorites[id]; exists {
				continue
			}
			knownFavorites[id] = struct{}{}
			added = append(added, id)
		}
	}
	if len(added) > 0 {
		if err := writeFavoriteOrder(tx, added, nil, slices.Concat(favorites, added)); err != nil {
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

// ReadLibraryBackup reads user library rows from a private copy. The caller
// supplies an unused name in a private root and removes the copy and its sidecars.
func ReadLibraryBackup(path string, root *os.Root, staged string) (*LibraryState, error) {
	if err := snapshotDatabase(path, root, staged); err != nil {
		return nil, err
	}
	original, err := root.Stat(".")
	if err != nil {
		return nil, err
	}
	current, err := os.Stat(root.Name())
	if err != nil || !os.SameFile(original, current) {
		return nil, errors.New("备份暂存目录已更换")
	}
	db, err := openDatabase(filepath.Join(root.Name(), staged), "ro")
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
		if strings.TrimSpace(entry.DramaID) == "" || entry.Percent < 0 || entry.Percent > 100 || !validHistoryNumbers(entry) {
			return nil, errors.New("备份包含无效的观看历史")
		}
	}
	return state, tx.Commit()
}
