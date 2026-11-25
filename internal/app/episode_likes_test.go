package app

import (
	"path/filepath"
	"testing"
)

func TestLocalEpisodeLikePersistsAndIsIsolated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jindi.db")
	a := New()
	if err := a.openStore(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	if err := a.SetLocalEpisodeLiked("d1", 32, true); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		id      string
		episode int
	}{{"d1", 33}, {"d1", 0}, {"missing", 1}} {
		if err := a.SetLocalEpisodeLiked(test.id, test.episode, true); err == nil {
			t.Fatalf("accepted an invalid episode: %+v", test)
		}
		if _, err := a.LocalEpisodeLiked(test.id, test.episode); err == nil {
			t.Fatalf("read an invalid episode: %+v", test)
		}
	}
	a.close()

	b := New()
	if err := b.openStore(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.close)
	if liked, err := b.LocalEpisodeLiked("d1", 32); err != nil || !liked {
		t.Fatalf("restored like: %v, %v", liked, err)
	}
	if liked, err := b.LocalEpisodeLiked("d1", 31); err != nil || liked {
		t.Fatalf("unliked adjacent episode: %v, %v", liked, err)
	}
	if err := b.SetLocalEpisodeLiked("d1", 32, false); err != nil {
		t.Fatal(err)
	}
	if liked, err := b.LocalEpisodeLiked("d1", 32); err != nil || liked {
		t.Fatalf("like should be removed: %v, %v", liked, err)
	}
}
