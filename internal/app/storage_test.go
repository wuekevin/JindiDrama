package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"jindi-drama/internal/storage/sqlite"
)

func TestAppStorageBindings(t *testing.T) {
	a := New()
	if _, err := a.Favorites(); err != ErrNoStore {
		t.Fatalf("未打开存储时应报 ErrNoStore: %v", err)
	}
	if err := a.openStore(filepath.Join(t.TempDir(), "x.db")); err != nil {
		t.Fatal(err)
	}
	defer a.close()
	_ = a.SaveFavorites([]string{"d1", "d2"})
	if f, _ := a.Favorites(); len(f) != 2 {
		t.Fatal(f)
	}
	_ = a.RecordHistory(sqlite.HistoryEntry{DramaID: "d1", Episode: 3, PositionMs: 5000, DurationMs: 10000, Percent: 50})
	if h, _ := a.History(); len(h) != 1 || h[0].PositionMs != 5000 {
		t.Fatal(h)
	}
	if err := a.SaveSetting("evil", "{}"); err == nil {
		t.Fatal("非白名单键必须拒绝")
	}
	if err := a.SaveSetting("app", `{"theme":"light"}`); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.GetSetting("app"); v != `{"theme":"light"}` {
		t.Fatal(v)
	}
}

func TestBackupsHaveSeparatePathsAndPreserveData(t *testing.T) {
	a := New()
	root := t.TempDir()
	if err := a.openStore(filepath.Join(root, "jindi.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	if err := a.SaveFavorites([]string{"d1"}); err != nil {
		t.Fatal(err)
	}
	first, err := a.backupTo(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.backupTo(root)
	if err != nil || first == second {
		t.Fatalf("backup paths: %q, %q, %v", first, second, err)
	}
	entries, err := os.ReadDir(filepath.Dir(second))
	if err != nil || len(entries) != 2 {
		t.Fatalf("backup directory should contain only the two finalized files: %v, %v", entries, err)
	}
	copy, err := sqlite.Open(second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { copy.Close() })
	favorites, err := copy.Favorites()
	if err != nil || len(favorites) != 1 || favorites[0] != "d1" {
		t.Fatalf("backup contents: %v, %v", favorites, err)
	}
}

func TestBackupSnapshotFailureDoesNotPublishPartialFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "backups")
	dest := filepath.Join(dir, "backup.db")
	failure := errors.New("snapshot interrupted")
	err := publishDatabaseBackup(dest, func(staged string) error {
		if err := os.WriteFile(staged, []byte("partial"), 0o600); err != nil {
			return err
		}
		if _, err := os.Lstat(dest); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("destination appeared before snapshot completed: %v", err)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("backup returned %v, want snapshot failure", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed backup left files: %v, %v", entries, err)
	}
}

func TestBackupSnapshotCannotReplaceConcurrentDestination(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "backup.db")
	err := publishDatabaseBackup(dest, func(staged string) error {
		if err := os.WriteFile(staged, []byte("completed snapshot"), 0o600); err != nil {
			return err
		}
		return os.WriteFile(dest, []byte("existing file"), 0o600)
	})
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("backup returned %v, want existing target error", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "existing file" {
		t.Fatalf("existing file was modified: %q, %v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "backup.db" {
		t.Fatalf("failed publication left staging files: %v, %v", entries, err)
	}
}
