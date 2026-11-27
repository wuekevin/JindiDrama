//go:build darwin && cgo

package app

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"jindi-drama/internal/storage/sqlite"
)

func TestRestoreScratchDoesNotInheritAccessACL(t *testing.T) {
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
	if err := withPrivateDatabaseSnapshot(privateSnapshotStage, func(root *os.Root, _ string) error {
		scratch := root.Name()
		info, err := os.Stat(scratch)
		if err != nil || info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("snapshot directory is not private: %v, %v", info, err)
		}
		if output, err := exec.Command("/bin/ls", "-lde", scratch).CombinedOutput(); err != nil || bytes.Contains(output, []byte("everyone")) {
			t.Fatalf("snapshot directory retained an inherited ACL: %v (%s)", err, output)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 1 || entries[0].Name() != "inherited" {
		t.Fatalf("snapshot cleanup left data: %v, %v", entries, err)
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
