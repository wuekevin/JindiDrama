package sqlite

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"slices"
	"time"
)

type LibraryChanges struct {
	Favorites            []string        `json:"favorites"`
	History              []HistoryEntry  `json:"history"`
	Removed              []string        `json:"removed"`
	AddedFavorites       []string        `json:"addedFavorites,omitempty"`
	RemovedFavorites     []string        `json:"removedFavorites,omitempty"`
	FavoriteOrder        []string        `json:"favoriteOrder,omitempty"`
	PreserveNewerHistory bool            `json:"preserveNewerHistory,omitempty"`
	Receipt              *LibraryReceipt `json:"receipt,omitempty"`
	LegacyJournal        string          `json:"legacyJournal,omitempty"`
	ReadBack             bool            `json:"readBack,omitempty"`
}

type LibraryState struct {
	Favorites []string       `json:"favorites"`
	History   []HistoryEntry `json:"history"`
}

type LibraryReceipt struct {
	Writer   string `json:"writer"`
	Revision int64  `json:"revision"`
}

// claimLibraryReceipt advances one writer's durable revision in the same
// transaction as the library mutations. Equal or older revisions are no-ops.
func claimLibraryReceipt(tx *sql.Tx, receipt *LibraryReceipt) (bool, error) {
	if receipt == nil {
		return true, nil
	}
	if len(receipt.Writer) != 32 || receipt.Revision < 1 {
		return false, errors.New("无效的收藏历史写入回执")
	}
	if decoded, err := hex.DecodeString(receipt.Writer); err != nil || hex.EncodeToString(decoded) != receipt.Writer {
		return false, errors.New("无效的收藏历史写入回执")
	}
	result, err := tx.Exec(`INSERT INTO library_receipts(writer, revision) VALUES (?, ?)
ON CONFLICT(writer) DO UPDATE SET revision=excluded.revision WHERE library_receipts.revision < excluded.revision`, receipt.Writer, receipt.Revision)
	if err != nil {
		return false, err
	}
	claimed, err := result.RowsAffected()
	return claimed != 0, err
}

// SaveLibraryChanges applies all changes atomically. A nil Favorites leaves the
// existing collection intact, while an empty non-nil slice clears it.
func (s *Store) SaveLibraryChanges(changes LibraryChanges) error {
	_, err := s.WriteLibraryChanges(changes)
	return err
}

// WriteLibraryChanges can return the state seen inside the same transaction,
// including when a previously committed batch is skipped.
func (s *Store) WriteLibraryChanges(changes LibraryChanges) (*LibraryState, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	receipt := changes.Receipt
	if changes.LegacyJournal != "" {
		digest := sha256.Sum256([]byte(changes.LegacyJournal))
		receipt = &LibraryReceipt{Writer: hex.EncodeToString(digest[:16]), Revision: 1}
	}
	claimed, err := claimLibraryReceipt(tx, receipt)
	if err != nil {
		return nil, err
	}
	if claimed {
		if err := applyLibraryChanges(tx, changes); err != nil {
			return nil, err
		}
	}
	var state *LibraryState
	if changes.ReadBack {
		favorites, err := readFavorites(tx)
		if err != nil {
			return nil, err
		}
		history, err := readHistory(tx, true)
		if err != nil {
			return nil, err
		}
		state = &LibraryState{Favorites: favorites, History: history}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return state, nil
}

func applyLibraryChanges(tx *sql.Tx, changes LibraryChanges) error {
	if changes.Favorites != nil {
		if err := setFavorites(tx, changes.Favorites); err != nil {
			return err
		}
	} else if len(changes.AddedFavorites) > 0 || len(changes.RemovedFavorites) > 0 {
		if err := applyFavoriteIntent(tx, changes.AddedFavorites, changes.RemovedFavorites, changes.FavoriteOrder); err != nil {
			return err
		}
	}
	if err := removeHistory(tx, changes.Removed); err != nil {
		return err
	}
	for _, entry := range changes.History {
		if err := writeHistory(tx, entry, changes.PreserveNewerHistory); err != nil {
			return err
		}
	}
	return nil
}

// applyFavoriteIntent reads existing favorites in the write transaction so recovery
// preserves favorites added after the WebView first loaded its snapshot.
func applyFavoriteIntent(tx *sql.Tx, added, removed, order []string) error {
	current, err := readFavorites(tx)
	if err != nil {
		return err
	}
	next := mergeFavoriteOrder(current, added, removed, order)
	if slices.Equal(current, next) {
		return nil
	}
	// Sparse recovery keeps the identity, creation time, and dirty flag of retained rows.
	for _, id := range removed {
		if _, err := tx.Exec(`DELETE FROM favorites WHERE account_scope=? AND drama_id=?`, scope, id); err != nil {
			return err
		}
	}
	now := time.Now().UnixMilli()
	for _, id := range added {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO favorites(account_scope, drama_id, created_at) VALUES(?,?,?)`, scope, id, now); err != nil {
			return err
		}
	}
	for index, id := range next {
		if _, err := tx.Exec(`UPDATE favorites SET sort_order=? WHERE account_scope=? AND drama_id=?`, len(next)-index, scope, id); err != nil {
			return err
		}
	}
	return nil
}

func mergeFavoriteOrder(current, added, removed, order []string) []string {
	touched := make(map[string]bool, len(added)+len(removed))
	for _, id := range removed {
		touched[id] = true
	}
	for _, id := range added {
		touched[id] = true
	}
	next := make([]string, 0, len(current)+len(added))
	for _, id := range current {
		if !touched[id] {
			next = append(next, id)
		}
	}
	order = slices.Concat(order, added)
	anchor := ""
	for _, id := range order {
		if slices.Contains(added, id) && !slices.Contains(next, id) {
			next = slices.Insert(next, slices.Index(next, anchor)+1, id)
		}
		if slices.Contains(next, id) {
			anchor = id
		}
	}
	return next
}
