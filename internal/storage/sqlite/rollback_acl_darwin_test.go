//go:build darwin && cgo

package sqlite

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"jindi-drama/internal/fileprivacy"
)

func TestRollbackDoesNotInheritDestinationACL(t *testing.T) {
	source := filepath.Join(t.TempDir(), "current.db")
	store, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	grant := "everyone allow read,readattr,readextattr,readsecurity,file_inherit,directory_inherit"
	if output, err := exec.Command("/bin/chmod", "+a", grant, parent).CombinedOutput(); err != nil {
		t.Fatalf("set inherited ACL fixture: %v (%s)", err, output)
	}
	probe := filepath.Join(parent, "probe")
	if err := os.WriteFile(probe, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	access, err := exec.Command("/bin/ls", "-le", probe).CombinedOutput()
	if err != nil || !bytes.Contains(access, []byte("everyone")) {
		t.Fatalf("fixture must inherit everyone ACL: %v (%s)", err, access)
	}
	stage := filepath.Join(parent, "stage-probe")
	if err := os.Mkdir(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(stage)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := fileprivacy.ProtectDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("rollback.db-journal", []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	access, err = exec.Command("/bin/ls", "-lde", stage, filepath.Join(stage, "rollback.db-journal")).CombinedOutput()
	if err != nil || bytes.Contains(access, []byte("everyone")) {
		t.Fatalf("stage or journal retained ACL: %v (%s)", err, access)
	}
	target := filepath.Join(parent, "rollback.db")
	if err := PrepareRollbackCopy(source, target, 2); err != nil {
		t.Fatal(err)
	}
	access, err = exec.Command("/bin/ls", "-le", target).CombinedOutput()
	if err != nil || bytes.Contains(access, []byte("everyone")) {
		t.Fatalf("rollback inherited an ACL: %v (%s)", err, access)
	}
	if err := VerifyDesktopRollback(target); err != nil {
		t.Fatal(err)
	}
}

func TestCleanRollbackStageLeavesReplacementAlone(t *testing.T) {
	parent := t.TempDir()
	stage := filepath.Join(parent, "stage")
	if err := os.Mkdir(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(stage)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	if err := root.WriteFile("rollback.db", []byte("snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := stage + "-original"
	if err := os.Rename(stage, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stage, "rollback.db")
	if err := os.WriteFile(path, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	cleanRollbackStage(root, "rollback.db")
	if entries, err := os.ReadDir(original); err != nil || len(entries) != 0 {
		t.Fatalf("original snapshot was not cleared: %v %v", entries, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "unrelated" {
		t.Fatalf("replacement changed: %q %v", data, err)
	}
}

func TestUnclaimedRollbackStageLeavesExistingFiles(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	if err := root.WriteFile("rollback.db", []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkEmptyRollbackStage(root); err == nil {
		t.Fatal("accepted an occupied rollback directory")
	}
	cleanRollbackStage(root, "")
	if data, err := os.ReadFile(filepath.Join(dir, "rollback.db")); err != nil || string(data) != "keep" {
		t.Fatalf("preserved contents: %q %v", data, err)
	}
}
