package sqlite

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyDesktopDatabaseReadsExistingProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "应用数据 #1.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.SetFavorites([]string{"d1"}); err != nil {
		t.Fatal(err)
	}
	wal, err := os.ReadFile(path + "-wal")
	if err != nil || len(wal) <= 32 {
		t.Fatalf("fixture has no committed WAL frames: %v", err)
	}
	if err := VerifyDesktopDatabase(path); err != nil {
		t.Fatalf("read a database with committed WAL content: %v", err)
	}
	if after, err := os.ReadFile(path + "-wal"); err != nil || !bytes.Equal(wal, after) {
		t.Fatalf("original WAL was changed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dirBefore, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyDesktopDatabase(path); err != nil {
		t.Fatalf("read a closed profile: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("profile was changed: %v", err)
	}
	dirAfter, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(dirAfter) != len(dirBefore) {
		t.Fatalf("profile directory changed: %v, %v", dirAfter, err)
	}
	for i, entry := range dirAfter {
		if entry.Name() != dirBefore[i].Name() {
			t.Fatalf("unexpected profile entry after verification: %s", entry.Name())
		}
	}
}

func TestVerifyDesktopDatabaseRejectsProfileDefects(t *testing.T) {
	for _, test := range []struct {
		name, statement, want string
	}{
		{"missing table", "DROP TABLE local_comments", "local_comments"},
		{"missing column", "ALTER TABLE favorites DROP COLUMN sort_order", "sort_order"},
		{"missing index", "DROP INDEX idx_history_watched", "index idx_history_watched"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "jindi.db")
			store, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			if _, err := store.db.Exec(test.statement); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			if err := VerifyDesktopDatabase(path); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestVerifyDesktopDatabaseRejectsAnOlderPhysicalSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jindi.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(len(migrations) - 1); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDesktopDatabase(path); err == nil || !strings.Contains(err.Error(), "schema version") {
		t.Fatalf("expected outdated schema, got %v", err)
	}
}

func TestVerifyDesktopDatabaseRejectsMissingOrCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jindi.db")
	if err := VerifyDesktopDatabase(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing path: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("verification created a file: %v", err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Break the first b-tree page while preserving the 100-byte database
	// header. The old 16-byte check would still accept this file.
	data[100] = 0
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDesktopDatabase(path); err == nil {
		t.Fatal("verification accepted a corrupt database with a valid header")
	}
}
