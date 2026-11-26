package sqlite

import (
	"path/filepath"
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
