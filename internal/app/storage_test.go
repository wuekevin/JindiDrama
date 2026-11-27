package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

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

func TestBackupSnapshotRejectsRetargetedDestination(t *testing.T) {
	root := t.TempDir()
	selected, original, replacement := filepath.Join(root, "selected"), filepath.Join(root, "original"), filepath.Join(root, "replacement")
	for _, directory := range []string{original, replacement} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(original, selected); err != nil {
		const privilegeNotHeld = syscall.Errno(1314)
		if runtime.GOOS == "windows" && errors.Is(err, privilegeNotHeld) {
			t.Skipf("directory fixture requires Windows symbolic link support: %v", err)
		}
		t.Fatal(err)
	}
	marker := filepath.Join(replacement, "other-file")
	if err := os.WriteFile(marker, []byte("unrelated file"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := publishDatabaseBackup(filepath.Join(selected, "backup.db"), func(snapshot string) error {
		if err := os.WriteFile(snapshot, []byte("legitimate snapshot"), 0o600); err != nil {
			return err
		}
		if err := os.Remove(selected); err != nil {
			return err
		}
		if err := os.Symlink(replacement, selected); err != nil {
			return err
		}
		// Older versions placed the SQLite snapshot under the selected directory.
		if relative, err := filepath.Rel(selected, snapshot); err == nil && filepath.IsLocal(relative) {
			if err := os.MkdirAll(filepath.Dir(snapshot), 0o700); err != nil {
				return err
			}
			return os.WriteFile(snapshot, []byte("replacement snapshot"), 0o600)
		}
		return nil
	})
	if !errors.Is(err, errUpdateDestinationMoved) {
		t.Fatalf("retargeted backup: expected destination error, got %v", err)
	}
	entries, err := os.ReadDir(original)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary data remained in original directory: %v, %v", entries, err)
	}
	entries, err = os.ReadDir(replacement)
	if err != nil || len(entries) != 1 || entries[0].Name() != "other-file" {
		t.Fatalf("unexpected files in replacement directory: %v, %v", entries, err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "unrelated file" {
		t.Fatalf("replacement file changed: %q, %v", data, err)
	}
}

func TestStorageReadyWaitsForStartup(t *testing.T) {
	ready := make(chan struct{})
	a := &App{startupDone: ready}
	defer a.close()
	started := make(chan struct{})
	result := make(chan bool, 1)
	go func() {
		close(started)
		result <- a.StorageReady()
	}()
	<-started
	select {
	case <-result:
		t.Fatal("storage readiness returned before startup published its result")
	case <-time.After(10 * time.Millisecond):
	}
	if err := a.openStore(filepath.Join(t.TempDir(), "ready.db")); err != nil {
		close(ready)
		t.Fatal(err)
	}
	close(ready)
	select {
	case ok := <-result:
		if !ok {
			t.Fatal("initialized SQLite store should be ready")
		}
	case <-time.After(time.Second):
		t.Fatal("storage readiness did not finish after startup")
	}
}
