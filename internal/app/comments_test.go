package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalCommentsPersistPerEpisode(t *testing.T) {
	a := New()
	db := filepath.Join(t.TempDir(), "jindi.db")
	if err := a.openStore(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	const snapshot = `[{"id":"me-1","text":"好看"},{"id":"me-2","text":"同意","replies":[{"id":"r1","text":"旧回复"}]}]`
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

func TestLocalCommentsRejectInvalidSnapshotsWithoutChangingStorage(t *testing.T) {
	a := New()
	if err := a.openStore(filepath.Join(t.TempDir(), "jindi.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	const valid = `[{"id":"c1","text":"原评论","replies":[{"id":"r1","text":"原回复","mine":true}]}]`
	tests := map[string]string{
		"empty snapshot":             "",
		"null list":                  `null`,
		"missing ID":                 `[{"ID":"c1","text":"comment"}]`,
		"duplicate ID property":      `[{"id":"c1","id":"c2","text":"one"}]`,
		"duplicate replies property": `[{"id":"c1","text":"comment","replies":null,"replies":[]}]`,
		"duplicate reply property":   `[{"id":"c1","text":"comment","replies":[{"id":"r1","text":"one","text":"two"}]}]`,
		"duplicate ID":               `[{"id":"c1","text":"first"},{"id":"c1","text":"second"}]`,
		"null replies":               `[{"id":"c1","text":"comment","replies":null}]`,
		"replies object":             `[{"id":"c1","text":"comment","replies":{}}]`,
		"null reply":                 `[{"id":"c1","text":"comment","replies":[null]}]`,
		"empty reply":                `[{"id":"c1","text":"comment","replies":[{"id":"r1","text":""}]}]`,
		"duplicate reply":            `[{"id":"c1","text":"comment","replies":[{"id":"r1","text":"one"},{"id":"r1","text":"two"}]}]`,
		"short count":                `[{"id":"c1","text":"comment","replies":[{"id":"r1","text":"reply"}],"replyTotal":0}]`,
		"fractional count":           `[{"id":"c1","text":"comment","replyTotal":1.2}]`,
		"null field":                 `[{"id":"c1","text":"comment","user":null}]`,
		"invalid count":              `[{"id":"c1","text":"comment","replyTotal":"1"}]`,
		"invalid owner":              `[{"id":"c1","text":"comment","replies":[{"id":"r1","text":"reply","mine":"false"}]}]`,
	}
	for name, invalid := range tests {
		t.Run(name, func(t *testing.T) {
			if err := a.SaveLocalComments("d1", 32, valid); err != nil {
				t.Fatal(err)
			}
			if err := a.SaveLocalComments("d1", 32, invalid); !errors.Is(err, errInvalidLocalComments) {
				t.Fatalf("invalid write: %v", err)
			}
			if got, err := a.LocalComments("d1", 32); got != valid || err != nil {
				t.Fatalf("invalid write changed the existing comment: %q, %v", got, err)
			}

			// Simulate an older client that already saved the corrupt snapshot.
			if err := a.store.SaveLocalComments("d1", 32, invalid); err != nil {
				t.Fatal(err)
			}
			if got, err := a.LocalComments("d1", 32); got != "" || !errors.Is(err, errInvalidLocalComments) {
				t.Fatalf("invalid read: %q, %v", got, err)
			}
			if got, found, err := a.store.LocalComments("d1", 32); !found || got != invalid || err != nil {
				t.Fatalf("invalid read changed the stored bytes: %q, %v", got, err)
			}
		})
	}
}

func TestLocalCommentsRejectOversizedStoredSnapshot(t *testing.T) {
	a := New()
	if err := a.openStore(filepath.Join(t.TempDir(), "jindi.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	const prefix = `[{"id":"c1","text":"`
	const suffix = `"}]`
	atLimit := prefix + strings.Repeat("x", maxLocalCommentSnapshot-len(prefix)-len(suffix)) + suffix
	if err := a.SaveLocalComments("d1", 32, atLimit); err != nil {
		t.Fatal(err)
	}
	if got, err := a.LocalComments("d1", 32); len(got) != maxLocalCommentSnapshot || err != nil {
		t.Fatalf("read at size limit: length %d, %v", len(got), err)
	}
	overLimit := atLimit + " "
	if err := a.SaveLocalComments("d1", 32, overLimit); err == nil {
		t.Fatal("accepted oversized write")
	}
	if err := a.store.SaveLocalComments("d1", 32, overLimit); err != nil {
		t.Fatal(err)
	}
	if got, err := a.LocalComments("d1", 32); got != "" || err == nil {
		t.Fatalf("accepted oversized stored snapshot: length %d, %v", len(got), err)
	}
}
