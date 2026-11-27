package sqlite

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

func TestReadLibraryBackupAndRestoreMissing(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source.db")
	source, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	saved := HistoryEntry{DramaID: "d2", Episode: 9, PositionMs: 7200, DurationMs: 24000, Percent: 30, WatchedAt: 2000}
	if err := source.SetFavorites([]string{"d2", "d3"}); err != nil {
		t.Fatal(err)
	}
	if err := source.RecordHistory(saved); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, "backup.db")
	if err := source.Backup(backup); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "legacy.db")
	if err := PrepareRollbackCopy(backup, legacy, 2); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{backup, legacy} {
		t.Run(filepath.Base(file), func(t *testing.T) {
			before, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := readTestLibraryBackup(t, file)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(snapshot.Favorites, []string{"d2", "d3"}) || !slices.Equal(snapshot.History, []HistoryEntry{saved}) {
				t.Fatalf("unexpected snapshot: %+v", snapshot)
			}
			destination, err := Open(":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer destination.Close()
			if err := destination.SetFavorites([]string{"d1", "d3"}); err != nil {
				t.Fatal(err)
			}
			if _, err := destination.db.Exec("UPDATE favorites SET id=99, created_at=123, dirty=0 WHERE drama_id='d3'"); err != nil {
				t.Fatal(err)
			}
			existing := HistoryEntry{DramaID: "d2", Episode: 5, WatchedAt: 1000}
			if err := destination.RecordHistory(existing); err != nil {
				t.Fatal(err)
			}
			incoming := HistoryEntry{DramaID: "d3", Episode: 6, WatchedAt: 3000}
			snapshot.History = append(snapshot.History, incoming)
			result, err := destination.RestoreMissingLibrary(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if result.FavoritesAdded != 1 || result.HistoryAdded != 1 ||
				!slices.Equal(result.State.Favorites, []string{"d1", "d3", "d2"}) ||
				!slices.Equal(result.State.History, []HistoryEntry{incoming, existing}) {
				t.Fatalf("unexpected merge: %+v", result)
			}
			var id, createdAt, dirty int
			if err := destination.db.QueryRow("SELECT id, created_at, dirty FROM favorites WHERE drama_id='d3'").Scan(&id, &createdAt, &dirty); err != nil {
				t.Fatal(err)
			}
			if id != 99 || createdAt != 123 || dirty != 0 {
				t.Fatalf("existing favorite changed: id=%d createdAt=%d dirty=%d", id, createdAt, dirty)
			}
			result, err = destination.RestoreMissingLibrary(snapshot)
			if err != nil || result.FavoritesAdded != 0 || result.HistoryAdded != 0 {
				t.Fatalf("repeated merge: %+v, %v", result, err)
			}
			after, err := os.ReadFile(file)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("original backup changed: %v", err)
			}
		})
	}
}

func TestRestoreMissingLibraryPreservesMixedFavoriteOrderAndRollsBack(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`INSERT INTO favorites(id, drama_id, created_at, sort_order, dirty) VALUES
(11, 'ranked-first', 20, 120, 1), (12, 'legacy', 100, NULL, 0), (13, 'ranked-last', 200, 5, 0)`); err != nil {
		t.Fatal(err)
	}
	existing := HistoryEntry{DramaID: "legacy", Episode: 2, WatchedAt: 30}
	if err := store.RecordHistory(existing); err != nil {
		t.Fatal(err)
	}
	incoming := HistoryEntry{DramaID: "new-first", Episode: 3, WatchedAt: 20}
	backup := &LibraryState{
		Favorites: []string{"ranked-last", "new-first", "legacy", "new-second", "new-first", "ranked-first"},
		History:   []HistoryEntry{{DramaID: "legacy", Episode: 4, WatchedAt: 40}, incoming},
	}
	if _, err := store.db.Exec(`CREATE TRIGGER reject_restore BEFORE INSERT ON history
BEGIN SELECT RAISE(ABORT, 'fixture write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if result, err := store.RestoreMissingLibrary(backup); err == nil || result != nil {
		t.Fatalf("failed merge produced a result: %+v, %v", result, err)
	}
	originalOrder := []string{"ranked-first", "legacy", "ranked-last"}
	if favorites, err := store.Favorites(); err != nil || !slices.Equal(favorites, originalOrder) {
		t.Fatalf("failed merge changed favorites: %v, %v", favorites, err)
	}
	var originalRank int
	if err := store.db.QueryRow("SELECT sort_order FROM favorites WHERE id=11").Scan(&originalRank); err != nil || originalRank != 120 {
		t.Fatalf("failed merge changed the original rank: %d, %v", originalRank, err)
	}
	if history, err := store.History(); err != nil || !slices.Equal(history, []HistoryEntry{existing}) {
		t.Fatalf("failed merge changed history: %+v, %v", history, err)
	}
	if _, err := store.db.Exec("DROP TRIGGER reject_restore"); err != nil {
		t.Fatal(err)
	}
	wantFavorites := append(originalOrder, "new-first", "new-second")
	wantHistory := []HistoryEntry{existing, incoming}
	for _, want := range []struct{ favorites, history int }{{2, 1}, {0, 0}} {
		result, err := store.RestoreMissingLibrary(backup)
		if err != nil || result == nil || result.FavoritesAdded != want.favorites || result.HistoryAdded != want.history ||
			!slices.Equal(result.State.Favorites, wantFavorites) || !slices.Equal(result.State.History, wantHistory) {
			t.Fatalf("unexpected merge: %+v, %v", result, err)
		}
	}
	historyOnly := HistoryEntry{DramaID: "history-only", Episode: 1, WatchedAt: 10}
	result, err := store.RestoreMissingLibrary(&LibraryState{History: []HistoryEntry{historyOnly}})
	if err != nil || result == nil || result.FavoritesAdded != 0 || result.HistoryAdded != 1 ||
		!slices.Equal(result.State.Favorites, wantFavorites) || !slices.Equal(result.State.History, append(wantHistory, historyOnly)) {
		t.Fatalf("unexpected history-only merge: %+v, %v", result, err)
	}
	var id, createdAt, dirty int
	if err := store.db.QueryRow("SELECT id, created_at, dirty FROM favorites WHERE drama_id='ranked-first'").Scan(&id, &createdAt, &dirty); err != nil ||
		id != 11 || createdAt != 20 || dirty != 1 {
		t.Fatalf("existing favorite changed: id=%d createdAt=%d dirty=%d, %v", id, createdAt, dirty, err)
	}
}

func TestReadLibraryBackupRejectsInvalidFiles(t *testing.T) {
	root := t.TempDir()
	plain := filepath.Join(root, "plain.db")
	if err := os.WriteFile(plain, []byte("invalid backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	future := filepath.Join(root, "future.db")
	db, err := Open(future)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec("INSERT INTO schema_migrations(version, applied_at) VALUES(?, 0)", len(migrations)+1); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, plain, future} {
		if result, err := readTestLibraryBackup(t, path); err == nil {
			t.Errorf("accepted invalid backup: %+v", result)
		}
	}
}

func TestLicensedHistoryContextSurvivesBackupAndLegacyMigration(t *testing.T) {
	root := t.TempDir()
	store, err := Open(filepath.Join(root, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	entry := HistoryEntry{
		DramaID: "local-series", Episode: 2, PositionMs: 24000, DurationMs: 24000,
		Percent: 100, WatchedAt: 2000, EpisodeCountAtWatch: 3,
	}
	state, err := store.WriteLibraryChanges(LibraryChanges{
		History: []HistoryEntry{entry}, ReadBack: true,
		Receipt: &LibraryReceipt{Writer: "abcdef0123456789abcdef0123456789", Revision: 1},
	})
	if err != nil || state == nil || !slices.Equal(state.History, []HistoryEntry{entry}) {
		t.Fatalf("saved context: %+v, %v", state, err)
	}
	backup := filepath.Join(root, "backup.db")
	if err := store.Backup(backup); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "legacy-v6.db")
	if err := PrepareRollbackCopy(backup, legacy, 6); err != nil {
		t.Fatal(err)
	}

	check := func(history []HistoryEntry, count int) {
		t.Helper()
		want := entry
		want.EpisodeCountAtWatch = count
		if !slices.Equal(history, []HistoryEntry{want}) {
			t.Fatalf("history = %+v, want %+v", history, want)
		}
	}
	for _, fixture := range []struct {
		path  string
		count int
	}{{backup, 3}, {legacy, 0}} {
		snapshot, err := readTestLibraryBackup(t, fixture.path)
		if err != nil {
			t.Fatal(err)
		}
		check(snapshot.History, fixture.count)
		restored, err := Open(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		result, err := restored.RestoreMissingLibrary(snapshot)
		restored.Close()
		if err != nil || result.HistoryAdded != 1 {
			t.Fatalf("restore: %+v, %v", result, err)
		}
		check(result.State.History, fixture.count)
	}
	migrated, err := Open(legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	history, err := migrated.History()
	if err != nil {
		t.Fatal(err)
	}
	check(history, 0)
	entry.EpisodeCountAtWatch = 1
	if err := migrated.RecordHistory(entry); err == nil {
		t.Fatal("accepted a recorded total smaller than the watched episode")
	}
	if _, err := store.db.Exec("PRAGMA ignore_check_constraints=ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("UPDATE history SET episode_count_at_watch=9007199254740992"); err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(root, "invalid-v7.db")
	if err := store.Backup(invalid); err != nil {
		t.Fatal(err)
	}
	if _, err := readTestLibraryBackup(t, invalid); err == nil {
		t.Fatal("accepted a count that cannot be represented safely in the desktop bridge")
	}
}

func readTestLibraryBackup(t *testing.T, source string) (*LibraryState, error) {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	return ReadLibraryBackup(source, root, "staged.db")
}

func TestHistoryBridgeRejectsUnrepresentableValues(t *testing.T) {
	unsafeInteger := int64(9007199254740993)
	const lossyPosition = int64(9007199254740962)
	tests := []struct {
		column string
		value  int64
		change func(*HistoryEntry)
	}{
		{"episode", unsafeInteger, func(h *HistoryEntry) { h.Episode = int(unsafeInteger) }},
		{"position_ms", lossyPosition, func(h *HistoryEntry) { h.PositionMs = lossyPosition }},
		{"duration_ms", unsafeInteger, func(h *HistoryEntry) { h.DurationMs = unsafeInteger }},
		{"watched_at", 8640000000000001, func(h *HistoryEntry) { h.WatchedAt = 8640000000000001 }},
	}
	valid := HistoryEntry{DramaID: "current", Episode: 2, PositionMs: 1123, DurationMs: 3234, Percent: 35, WatchedAt: 2000}
	for _, test := range tests {
		t.Run(test.column, func(t *testing.T) {
			if test.column == "episode" && strconv.IntSize < 64 {
				t.Skip("episode overflows native int")
			}
			store, err := Open(":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			// Seed through the normal API, then reproduce a backup made before
			// this validation existed by editing its stored value directly.
			if err := store.RecordHistory(valid); err != nil {
				t.Fatal(err)
			}
			bad := valid
			test.change(&bad)
			if err := store.RecordHistory(bad); err == nil {
				t.Error("accepted an unrepresentable value through the direct writer")
			}
			if err := store.SaveLibraryChanges(LibraryChanges{
				Favorites: []string{"should-roll-back"}, History: []HistoryEntry{bad},
				Receipt: &LibraryReceipt{Writer: "abcdef0123456789abcdef0123456789", Revision: 1},
			}); err == nil {
				t.Error("accepted an unrepresentable value through the batch writer")
			}
			if favorites, err := store.Favorites(); err != nil || len(favorites) != 0 {
				t.Errorf("failed batch changed favorites: %v %v", favorites, err)
			}
			var receipts int
			if err := store.db.QueryRow("SELECT COUNT(*) FROM library_receipts").Scan(&receipts); err != nil || receipts != 0 {
				t.Errorf("failed batch left a receipt: %d %v", receipts, err)
			}
			if history, err := store.History(); err != nil || !slices.Equal(history, []HistoryEntry{valid}) {
				t.Errorf("failed write changed history: %v %v", history, err)
			}
			if _, err := store.db.Exec("UPDATE history SET "+test.column+"=?", test.value); err != nil {
				t.Fatal(err)
			}
			backup := filepath.Join(t.TempDir(), "legacy.db")
			if err := store.Backup(backup); err != nil {
				t.Fatal(err)
			}
			if result, err := readTestLibraryBackup(t, backup); result != nil || err == nil {
				t.Errorf("accepted an unrepresentable value from backup: %+v %v", result, err)
			}
		})
	}
}

func TestHistoryBridgePreservesRepresentableValues(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	entry := HistoryEntry{
		DramaID: "boundary", Episode: 7, EpisodeCountAtWatch: 7,
		PositionMs: 9007199254740963, DurationMs: 9007199254740990,
		Percent: 99, WatchedAt: 8640000000000000,
	}
	if err := store.RecordHistory(entry); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "history.db")
	if err := store.Backup(backup); err != nil {
		t.Fatal(err)
	}
	result, err := readTestLibraryBackup(t, backup)
	if err != nil || result == nil || !slices.Equal(result.History, []HistoryEntry{entry}) {
		t.Fatalf("representable entry changed: %+v %v", result, err)
	}
}
