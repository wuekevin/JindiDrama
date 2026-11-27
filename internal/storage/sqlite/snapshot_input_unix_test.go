//go:build unix

package sqlite

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestVerifyDesktopDatabaseRejectsPipeWithoutWriter(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "fixture.db")
	if err := unix.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", t.TempDir())
	done := make(chan error, 1)
	go func() { done <- VerifyDesktopDatabase(pipe) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("copy accepted the pipe")
		}
	case <-time.After(time.Second):
		// Release a blocking open in the original implementation.
		if fd, err := unix.Open(pipe, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0); err == nil {
			defer unix.Close(fd)
			<-done
		}
		t.Fatal("copy waited for a FIFO writer")
	}
	if entries, err := os.ReadDir(os.TempDir()); err != nil || len(entries) != 0 {
		t.Fatalf("invalid input wrote to staging: %v, %v", entries, err)
	}
}

func TestVerifyDesktopDatabaseUsesWALBehindSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "real.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SetFavorites([]string{"wal-row"}); err != nil {
		t.Fatal(err)
	}
	wal, err := os.ReadFile(path + "-wal")
	if err != nil || len(wal) <= 32 {
		t.Fatalf("fixture has no committed WAL: %v", err)
	}
	alias := filepath.Join(t.TempDir(), "alias.db")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDesktopDatabase(alias); err != nil {
		t.Fatalf("verify database via symlink: %v", err)
	}
}

func TestOpenSnapshotInputRejectsReplacementPipe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.db")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, filepath.Join(dir, "original.db")); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		input, _, err := openSnapshotInput(path)
		if input != nil {
			input.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("open accepted the replacement pipe")
		}
	case <-time.After(time.Second):
		if fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0); err == nil {
			defer unix.Close(fd)
			<-done
		}
		t.Fatal("opening the replacement waited for a FIFO writer")
	}
}

func TestCheckSnapshotSourcesRejectsRetargetedSelection(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.db")
	replacement := filepath.Join(dir, "replacement.db")
	link := filepath.Join(dir, "selected.db")
	for _, file := range []string{original, replacement} {
		if err := os.WriteFile(file, []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(original, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	input, info, err := openSnapshotInput(resolved)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(replacement, link); err != nil {
		t.Fatal(err)
	}
	if err := checkSnapshotSources(link, resolved, map[string]os.FileInfo{"": info}); !errors.Is(err, errSnapshotChanged) {
		t.Fatalf("retargeted backup: %v", err)
	}
}
