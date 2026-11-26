package sqlite

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenCurrentCommentsWhileAnotherConnectionHasWriteLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jindi.db")
	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	const snapshot = `[{"id":"one","text":"already committed"}]`
	if err := a.SaveLocalComments("d1", 32, snapshot); err != nil {
		t.Fatal(err)
	}
	lock, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()

	b, err := Open(path)
	if err != nil {
		t.Fatalf("open current database while locked: %v", err)
	}
	defer b.Close()
	current, err := b.LocalCommentsState("d1", 32)
	if err != nil || current.Snapshot != snapshot || current.Revision == "" {
		t.Fatalf("read current comments while locked: %+v, %v", current, err)
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	const updated = `[{"id":"one","text":"recovered"}]`
	result, err := b.CompareAndSaveLocalComments("d1", 32, current.Revision, updated, strings.Repeat("a", 32))
	if err != nil || len(result) != 64 {
		t.Fatalf("save after unlocking: %q, %v", result, err)
	}
}

func TestCommentProfileRemainsBoundToOpenedDatabase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require Windows privileges")
	}
	dir := t.TempDir()
	a := filepath.Join(dir, "a.db")
	b := filepath.Join(dir, "b.db")
	for _, target := range []string{a, b} {
		store, err := Open(target)
		if err != nil {
			t.Fatal(err)
		}
		store.Close()
	}
	active := filepath.Join(dir, "active.db")
	if err := os.Symlink(a, active); err != nil {
		t.Fatal(err)
	}
	first, err := Open(active)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	original := first.CommentProfile()
	if err := os.Remove(active); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(b, active); err != nil {
		t.Fatal(err)
	}
	second, err := Open(active)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if original == "" || original != first.CommentProfile() || original == second.CommentProfile() {
		t.Fatal("a running connection changed its comment profile")
	}
}
