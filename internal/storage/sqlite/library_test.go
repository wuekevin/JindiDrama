package sqlite

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestLibraryChangesAreAtomicAndPersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SaveLibraryChanges(LibraryChanges{
		Favorites: []string{"d1"},
		History:   []HistoryEntry{{DramaID: "d1", Episode: 2}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveLibraryChanges(LibraryChanges{
		Favorites: []string{"d2"},
		Removed:   []string{"d1"},
		History:   []HistoryEntry{{DramaID: "d3", Episode: 1}, {DramaID: "d2", Episode: 0}},
	}); err == nil {
		t.Fatal("expected an invalid history row to abort the transaction")
	}
	favorites, err := store.Favorites()
	if err != nil || len(favorites) != 1 || favorites[0] != "d1" {
		t.Fatalf("favorites after rollback: %v, %v", favorites, err)
	}
	history, err := store.History()
	if err != nil || len(history) != 1 || history[0].DramaID != "d1" {
		t.Fatalf("history after rollback: %v, %v", history, err)
	}
	if err := store.SaveLibraryChanges(LibraryChanges{Removed: []string{"d1"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	favorites, err = reopened.Favorites()
	if err != nil || len(favorites) != 1 || favorites[0] != "d1" {
		t.Fatalf("favorites after reopen: %v, %v", favorites, err)
	}
	history, err = reopened.History()
	if err != nil || len(history) != 0 {
		t.Fatalf("history after reopen: %v, %v", history, err)
	}
}

func TestRecoveryAppliesOnlyTouchedRowsAndKeepsNewerPlayback(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SetFavorites([]string{"unseen", "keep", "remove"}); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []HistoryEntry{
		{DramaID: "keep", Episode: 4, WatchedAt: 400, PositionMs: 6400},
		{DramaID: "remove", Episode: 2, WatchedAt: 200},
		{DramaID: "unseen", Episode: 5, WatchedAt: 500},
	} {
		if err := store.RecordHistory(entry); err != nil {
			t.Fatal(err)
		}
	}
	changes := LibraryChanges{
		AddedFavorites: []string{"offline"}, RemovedFavorites: []string{"remove"},
		PreserveNewerHistory: true, Removed: []string{"remove"},
		History: []HistoryEntry{{DramaID: "keep", Episode: 1, WatchedAt: 100}, {DramaID: "offline", Episode: 3, WatchedAt: 300}},
	}
	// Replaying after a crash immediately after commit must be safe.
	for i := 0; i < 2; i++ {
		if err := store.SaveLibraryChanges(changes); err != nil {
			t.Fatal(err)
		}
	}
	favorites, err := store.Favorites()
	if err != nil || !slices.Equal(favorites, []string{"offline", "unseen", "keep"}) {
		t.Fatalf("favorites: %v, %v", favorites, err)
	}
	history, err := store.History()
	if err != nil || len(history) != 3 || history[0].DramaID != "unseen" ||
		history[1].DramaID != "keep" || history[1].Episode != 4 || history[1].PositionMs != 6400 || history[2].DramaID != "offline" {
		t.Fatalf("history: %v, %v", history, err)
	}
}

func TestRecoveryRestoresUndoOrderAndPreservesFavoriteMetadata(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "undo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SetFavorites([]string{"A", "B", "C"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("UPDATE favorites SET dirty=1 WHERE drama_id='A'"); err != nil {
		t.Fatal(err)
	}
	readA := func() [3]int64 {
		var meta [3]int64
		if err := store.db.QueryRow("SELECT id, created_at, dirty FROM favorites WHERE drama_id='A'").Scan(&meta[0], &meta[1], &meta[2]); err != nil {
			t.Fatal(err)
		}
		return meta
	}
	before := readA()
	undo := LibraryChanges{AddedFavorites: []string{"B"}, FavoriteOrder: []string{"A", "B", "C"}}
	if err := store.SaveLibraryChanges(undo); err != nil {
		t.Fatal(err)
	}
	first, err := store.Favorites()
	if err != nil || !slices.Equal(first, []string{"A", "B", "C"}) {
		t.Fatalf("undo order: %v, %v", first, err)
	}
	if _, err := store.db.Exec("DELETE FROM favorites WHERE drama_id='B'"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveLibraryChanges(undo); err != nil {
		t.Fatal(err)
	}
	second, err := store.Favorites()
	if err != nil || !slices.Equal(second, []string{"A", "B", "C"}) || before != readA() {
		t.Fatalf("recovered order or untouched metadata changed: %v, %v", second, err)
	}
}

func TestLibraryReceiptSurvivesReopenAndSkipsOlderMutations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	first := &LibraryReceipt{Writer: "aabbccddeeff00112233445566778899", Revision: 1}
	latest := &LibraryReceipt{Writer: first.Writer, Revision: 2}
	if err := store.SaveLibraryChanges(LibraryChanges{Removed: []string{"d1"}, Receipt: latest}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.RecordHistory(HistoryEntry{DramaID: "d1", Episode: 9, PositionMs: 2300}); err != nil {
		t.Fatal(err)
	}
	if err := reopened.SetFavorites([]string{"d1", "d9"}); err != nil {
		t.Fatal(err)
	}
	for _, receipt := range []*LibraryReceipt{first, latest} {
		err := reopened.SaveLibraryChanges(LibraryChanges{
			Removed: []string{"d1"}, RemovedFavorites: []string{"d1"}, Receipt: receipt,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	history, err := reopened.History()
	if err != nil || len(history) != 1 || history[0].Episode != 9 || history[0].PositionMs != 2300 {
		t.Fatalf("new history overwritten: %v, %v", history, err)
	}
	favorites, err := reopened.Favorites()
	if err != nil || !slices.Equal(favorites, []string{"d1", "d9"}) {
		t.Fatalf("new favorites overwritten: %v, %v", favorites, err)
	}
}

func TestLibraryReceiptRollsBackWithMutations(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "rolled-back-receipt.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	receipt := &LibraryReceipt{Writer: "aabbccddeeff00112233445566778899", Revision: 1}
	err = store.SaveLibraryChanges(LibraryChanges{
		Favorites: []string{"d1"}, History: []HistoryEntry{{DramaID: "d1", Episode: 0}}, Receipt: receipt,
	})
	if err == nil {
		t.Fatal("invalid episode must roll back")
	}
	// The same receipt can be retried after the whole transaction rolled back.
	err = store.SaveLibraryChanges(LibraryChanges{
		Favorites: []string{"d2"}, History: []HistoryEntry{{DramaID: "d2", Episode: 8}}, Receipt: receipt,
	})
	if err != nil {
		t.Fatal(err)
	}
	history, err := store.History()
	if err != nil || len(history) != 1 || history[0].DramaID != "d2" {
		t.Fatalf("retry was not applied: %v, %v", history, err)
	}
}

func TestLegacyLibraryJournalKeepsIdentityWhenCacheCannotBeUpgraded(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "legacy-receipt.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	changes := LibraryChanges{
		LegacyJournal: `{"snapshot":{"history":[],"favorites":[]}}`,
		Removed:       []string{"d1"},
		ReadBack:      true,
	}
	if _, err := store.WriteLibraryChanges(changes); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordHistory(HistoryEntry{DramaID: "d1", Episode: 3, PositionMs: 1400}); err != nil {
		t.Fatal(err)
	}
	// An old cache can be converted again with a different transient writer ID.
	changes.Receipt = &LibraryReceipt{Writer: "aabbccddeeff00112233445566778899", Revision: 8}
	state, err := store.WriteLibraryChanges(changes)
	if err != nil || state == nil || len(state.History) != 1 || state.History[0].PositionMs != 1400 {
		t.Fatalf("duplicate legacy journal did not return current state: %+v, %v", state, err)
	}
}
