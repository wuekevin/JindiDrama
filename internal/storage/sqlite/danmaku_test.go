package sqlite

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestDanmakuReceiptSurvivesConcurrentRetryAndPreservesLegacyRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(8); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"legacy one", "legacy two"} {
		if _, err := db.AddLocalDanmaku("d1", 32, 1234, text); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(9); err != nil {
		t.Fatal(err)
	}
	other, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	type result struct {
		row LocalDanmaku
		err error
	}
	ready := make(chan struct{})
	results := make(chan result, 2)
	for _, store := range []*Store{db, other} {
		go func() {
			<-ready
			row, err := store.AddLocalDanmakuOnce("d1", 32, 9000, "new", "84d46f108fd048128c2c6dd0731e13d0")
			results <- result{row, err}
		}()
	}
	close(ready)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.row != second.row {
		t.Fatalf("concurrent retries did not return the same row: %+v, %+v", first, second)
	}
	before, err := db.LocalDanmaku("d1", 32)
	if err != nil || len(before) != 3 {
		t.Fatalf("unexpected rows after upgrade and retry: %+v, %v", before, err)
	}
	rollbackPath := filepath.Join(filepath.Dir(path), "v8.db")
	if err := PrepareRollbackCopy(path, rollbackPath, 8); err != nil {
		t.Fatal(err)
	}
	rollback, err := openDatabase(rollbackPath, "ro")
	if err != nil {
		t.Fatal(err)
	}
	legacy := &Store{db: rollback, Path: rollbackPath}
	defer legacy.Close()
	after, err := legacy.LocalDanmaku("d1", 32)
	if err != nil || !slices.Equal(before, after) {
		t.Fatalf("rollback did not preserve danmaku rows: %+v, %v", after, err)
	}
}

func TestDanmakuProfileDistinguishesReplacementAtSamePath(t *testing.T) {
	folder := t.TempDir()
	dir := filepath.Join(folder, "Profile")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// Exercise the Windows spelling difference between a new file in an
		// existing directory and the same file once EvalSymlinks can resolve it.
		alias := filepath.Join(folder, "profile")
		if _, err := os.Stat(alias); err == nil {
			dir = alias
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "app.db")
	openProfile := func(database string) (string, string) {
		t.Helper()
		db, err := Open(database)
		if err != nil {
			t.Fatal(err)
		}
		profile, profileErr := db.DataProfile()
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if profileErr != nil || len(profile) != 64 {
			t.Fatalf("profile unavailable: %v", profileErr)
		}
		return profile, db.canonicalPath
	}
	original, originalPath := openProfile(path)
	if got, openedPath := openProfile(path); got != original {
		t.Fatalf("profile changed on reopen (opened path %q, then %q)", originalPath, openedPath)
	}
	saved := filepath.Join(folder, "original.db")
	if err := os.Rename(path, saved); err != nil {
		t.Fatal(err)
	}
	if moved, _ := openProfile(saved); moved == original {
		t.Fatal("the same database at a different path shared the profile")
	}
	if replacement, _ := openProfile(path); replacement == original {
		t.Fatal("an unrelated database at the same path shared the profile")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, path); err != nil {
		t.Fatal(err)
	}
	if restored, _ := openProfile(path); restored != original {
		t.Fatal("restoring the original database at the original path changed the profile")
	}
}
