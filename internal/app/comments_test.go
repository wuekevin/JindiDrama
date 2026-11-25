package app

import (
	"path/filepath"
	"testing"
)

func TestLocalCommentsPersistPerEpisode(t *testing.T) {
	a := New()
	db := filepath.Join(t.TempDir(), "jindi.db")
	if err := a.openStore(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	const snapshot = `[{"id":"me-1","text":"好看","replies":[]}]`
	if err := a.SaveLocalComments("d1", 32, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveLocalComments("d1", 32, "null"); err == nil {
		t.Fatal("accepted null snapshot")
	}
	if err := a.SaveLocalComments("d1", 33, snapshot); err == nil {
		t.Fatal("accepted unavailable episode")
	}
	a.close()
	if err := a.openStore(db); err != nil {
		t.Fatal(err)
	}
	if got, err := a.LocalComments("d1", 32); err != nil || got != snapshot {
		t.Fatalf("after reopen: %q, %v", got, err)
	}
	if got, err := a.LocalComments("d1", 31); err != nil || got != "" {
		t.Fatalf("other episode: %q, %v", got, err)
	}
}
