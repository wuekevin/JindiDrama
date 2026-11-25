package sqlite

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
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
			snapshot, err := ReadLibraryBackup(file)
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
		if result, err := ReadLibraryBackup(path); err == nil {
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
		snapshot, err := ReadLibraryBackup(fixture.path)
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
	if _, err := ReadLibraryBackup(invalid); err == nil {
		t.Fatal("accepted a count that cannot be represented safely in the desktop bridge")
	}
}
