//go:build darwin && cgo

package app

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"jindi-drama/internal/storage/sqlite"
)

func TestBackupScratchDoesNotInheritAccessACL(t *testing.T) {
	parent := t.TempDir()
	permission := "everyone allow search,file_inherit,directory_inherit"
	if output, err := exec.Command("/bin/chmod", "+a", permission, parent).CombinedOutput(); err != nil {
		t.Fatalf("set fixture ACL: %v (%s)", err, output)
	}
	control := filepath.Join(parent, "inherited")
	if err := os.Mkdir(control, 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("/bin/ls", "-lde", control).CombinedOutput(); err != nil || !bytes.Contains(output, []byte("everyone")) {
		t.Fatalf("the fixture did not inherit an ACL: %v (%s)", err, output)
	}
	t.Setenv("TMPDIR", parent)
	if err := copyDatabaseSnapshot(io.Discard, func(path string) error {
		scratch := filepath.Dir(path)
		info, err := os.Stat(scratch)
		if err != nil || info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("snapshot directory is not private: %v, %v", info, err)
		}
		if output, err := exec.Command("/bin/ls", "-lde", scratch).CombinedOutput(); err != nil || bytes.Contains(output, []byte("everyone")) {
			t.Fatalf("snapshot directory retained an inherited ACL: %v (%s)", err, output)
		}
		return os.WriteFile(path, []byte("snapshot"), 0o600)
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 1 || entries[0].Name() != "inherited" {
		t.Fatalf("snapshot cleanup left data: %v, %v", entries, err)
	}
}

func TestBackupScratchCleanupLeavesReplacementFilesAlone(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	target := filepath.Join(t.TempDir(), "backup.db")
	var moved, marker string
	err := publishDatabaseBackup(target, func(path string) error {
		if err := os.WriteFile(path, []byte("snapshot"), 0o600); err != nil {
			return err
		}
		scratch := filepath.Dir(path)
		moved = scratch + "-moved"
		if err := os.Rename(scratch, moved); err != nil {
			return err
		}
		if err := os.Mkdir(scratch, 0o700); err != nil {
			return err
		}
		marker = filepath.Join(scratch, "unrelated")
		return os.WriteFile(marker, []byte("unrelated content"), 0o600)
	})
	if err == nil {
		t.Fatal("backup accepted a replaced scratch directory")
	}
	if _, err := os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("scratch failure created a backup: %v", err)
	}
	if contents, err := os.ReadFile(marker); err != nil || string(contents) != "unrelated content" {
		t.Fatalf("replacement contents changed: %q, %v", contents, err)
	}
	if files, err := os.ReadDir(moved); err != nil || len(files) > 0 {
		t.Fatalf("original scratch still contains data: %v, %v", files, err)
	}
}

func TestRestoreBackupScratchReplacementReceivesNoCopies(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	a := New()
	if err := a.openStore(":memory:"); err != nil {
		t.Fatal(err)
	}
	defer a.close()
	source := filepath.Join(t.TempDir(), "source.db")
	if err := a.store.Backup(source); err != nil {
		t.Fatal(err)
	}
	var moved, replacement string
	err := withPrivateDatabaseSnapshot(privateSnapshotStage, func(root *os.Root, file string) error {
		replacement = root.Name()
		moved = replacement + "-moved"
		if err := os.Rename(replacement, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(replacement, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(replacement, "unrelated"), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := sqlite.ReadLibraryBackup(source, root, file)
		return err
	})
	if err == nil {
		t.Fatal("restore read accepted a replaced scratch directory")
	}
	if files, err := os.ReadDir(moved); err != nil || len(files) != 0 {
		t.Fatalf("original scratch still contains data: %v, %v", files, err)
	}
	if files, err := os.ReadDir(replacement); err != nil || len(files) != 1 || files[0].Name() != "unrelated" {
		t.Fatalf("replacement received temporary data: %v, %v", files, err)
	}
}
