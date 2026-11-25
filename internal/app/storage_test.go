package app

import (
	"path/filepath"
	"testing"

	"jindi-drama/internal/storage/sqlite"
)

func TestAppStorageBindings(t *testing.T) {
	a := New()
	if _, err := a.Favorites(); err != ErrNoStore {
		t.Fatalf("未打开存储时应报 ErrNoStore: %v", err)
	}
	if err := a.openStore(filepath.Join(t.TempDir(), "x.db")); err != nil {
		t.Fatal(err)
	}
	defer a.close()
	_ = a.SaveFavorites([]string{"d1", "d2"})
	if f, _ := a.Favorites(); len(f) != 2 {
		t.Fatal(f)
	}
	_ = a.RecordHistory(sqlite.HistoryEntry{DramaID: "d1", Episode: 3, PositionMs: 5000, DurationMs: 10000, Percent: 50})
	if h, _ := a.History(); len(h) != 1 || h[0].PositionMs != 5000 {
		t.Fatal(h)
	}
	if err := a.SaveSetting("evil", "{}"); err == nil {
		t.Fatal("非白名单键必须拒绝")
	}
	if err := a.SaveSetting("app", `{"theme":"light"}`); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.GetSetting("app"); v != `{"theme":"light"}` {
		t.Fatal(v)
	}
}

func TestBackupsHaveSeparatePathsAndPreserveData(t *testing.T) {
	a := New()
	root := t.TempDir()
	if err := a.openStore(filepath.Join(root, "jindi.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	if err := a.SaveFavorites([]string{"d1"}); err != nil {
		t.Fatal(err)
	}
	first, err := a.backupTo(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.backupTo(root)
	if err != nil || first == second {
		t.Fatalf("backup paths: %q, %q, %v", first, second, err)
	}
	copy, err := sqlite.Open(second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { copy.Close() })
	favorites, err := copy.Favorites()
	if err != nil || len(favorites) != 1 || favorites[0] != "d1" {
		t.Fatalf("backup contents: %v, %v", favorites, err)
	}
}
