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
