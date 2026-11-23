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
	if err := a.OpenStore(filepath.Join(t.TempDir(), "x.db")); err != nil {
		t.Fatal(err)
	}
	defer a.Close()
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
