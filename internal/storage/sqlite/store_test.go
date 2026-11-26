package sqlite

import (
	"path/filepath"
	"slices"
	"testing"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "jd.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestWALAndMigrations(t *testing.T) {
	s := open(t)
	var mode string
	if err := s.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode=%q err=%v", mode, err)
	}
	if v, _ := s.Version(); v != len(migrations) {
		t.Fatalf("version=%d", v)
	}
	if err := s.Migrate(1); err != nil { // 回滚到 v1
		t.Fatal(err)
	}
	if v, _ := s.Version(); v != 1 {
		t.Fatalf("rollback version=%d", v)
	}
	if err := s.Migrate(0); err != nil { // 全部回滚
		t.Fatal(err)
	}
	if err := s.Migrate(len(migrations)); err != nil { // 再升级
		t.Fatal(err)
	}
	if err := s.Migrate(99); err == nil {
		t.Fatal("未知版本必须报错")
	}
}

func TestFavoritesOrderAndReplace(t *testing.T) {
	s := open(t)
	if err := s.SetFavorites([]string{"d3", "d1", "d2", "d1"}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Favorites()
	if len(got) != 3 || got[0] != "d3" || got[1] != "d1" || got[2] != "d2" {
		t.Fatalf("order: %v", got)
	}
	_ = s.SetFavorites(nil)
	if got, _ := s.Favorites(); len(got) != 0 {
		t.Fatalf("clear: %v", got)
	}
}

func TestHistoryUpsertExactProgress(t *testing.T) {
	s := open(t)
	_ = s.RecordHistory(HistoryEntry{DramaID: "d1", Episode: 32, PositionMs: 157000, DurationMs: 392000, Percent: 40, WatchedAt: 1000})
	_ = s.RecordHistory(HistoryEntry{DramaID: "d1", Episode: 33, PositionMs: 1000, DurationMs: 390000, Percent: 150, WatchedAt: 2000})
	_ = s.RecordHistory(HistoryEntry{DramaID: "d2", Episode: 1, Percent: -5, WatchedAt: 3000})
	h, _ := s.History()
	if len(h) != 2 || h[0].DramaID != "d2" || h[0].Percent != 0 || h[1].Percent != 100 || h[1].Episode != 33 || h[1].PositionMs != 1000 {
		t.Fatalf("%+v", h)
	}
	if err := s.RecordHistory(HistoryEntry{}); err == nil {
		t.Fatal("空记录必须报错")
	}
	_ = s.RemoveHistory([]string{"d2"})
	if h, _ := s.History(); len(h) != 1 {
		t.Fatalf("remove: %+v", h)
	}
	_ = s.ClearHistory()
	if h, _ := s.History(); len(h) != 0 {
		t.Fatal("clear")
	}
}

func TestSettingsAndSearchHistory(t *testing.T) {
	s := open(t)
	if _, ok, _ := s.GetSetting("x"); ok {
		t.Fatal("不应存在")
	}
	_ = s.SetSetting("x", `{"a":1}`)
	_ = s.SetSetting("x", `{"a":2}`)
	if v, ok, _ := s.GetSetting("x"); !ok || v != `{"a":2}` {
		t.Fatalf("%q", v)
	}
	_ = s.SetSearchHistory([]string{"替嫁", "古装"})
	if q, _ := s.SearchHistory(); len(q) != 2 || q[0] != "替嫁" {
		t.Fatalf("%v", q)
	}
}

func TestSearchHistoryRevisionRejectsStaleAndRollsBackOnFailure(t *testing.T) {
	s := open(t)
	if err := s.SetSearchHistory([]string{"原记录"}); err != nil {
		t.Fatal(err)
	}
	if accepted, err := s.SetSearchHistoryRevision(SearchHistoryState{Revision: 3}); !accepted || err != nil {
		t.Fatalf("clear: accepted=%v err=%v", accepted, err)
	}
	if accepted, err := s.SetSearchHistoryRevision(SearchHistoryState{Terms: []string{"旧记录"}, Revision: 2}); accepted || err != nil {
		t.Fatalf("stale replay: accepted=%v err=%v", accepted, err)
	}
	if state, err := s.SearchHistoryState(); err != nil || state.Revision != 3 || state.Terms == nil || len(state.Terms) != 0 {
		t.Fatalf("after stale replay: state=%+v err=%v", state, err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_search BEFORE INSERT ON search_history
WHEN NEW.query = '受保护' BEGIN SELECT RAISE(ABORT, 'rejected search'); END`); err != nil {
		t.Fatal(err)
	}
	if accepted, err := s.SetSearchHistoryRevision(SearchHistoryState{Terms: []string{"受保护"}, Revision: 4}); accepted || err == nil {
		t.Fatalf("failed write: accepted=%v err=%v", accepted, err)
	}
	if state, err := s.SearchHistoryState(); err != nil || state.Revision != 3 || len(state.Terms) != 0 {
		t.Fatalf("after rollback: state=%+v err=%v", state, err)
	}
	terms := []string{"最新", "其次"}
	if accepted, err := s.SetSearchHistoryRevision(SearchHistoryState{Terms: terms, Revision: 4}); !accepted || err != nil {
		t.Fatalf("retry: accepted=%v err=%v", accepted, err)
	}
	if state, err := s.SearchHistoryState(); err != nil || state.Revision != 4 || !slices.Equal(state.Terms, terms) {
		t.Fatalf("after retry: state=%+v err=%v", state, err)
	}
}

func TestBackupRestoreAndPersistence(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.db")
	s, _ := Open(p)
	_ = s.SetFavorites([]string{"d9"})
	bk := filepath.Join(dir, "bk", "b.db")
	if err := s.Backup(bk); err != nil {
		t.Fatal(err)
	}
	if err := s.Backup(bk); err == nil {
		t.Fatal("备份不得覆盖已有文件")
	}
	s.Close()
	r, err := Open(bk) // 备份可独立打开
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if f, _ := r.Favorites(); len(f) != 1 || f[0] != "d9" {
		t.Fatalf("restore: %v", f)
	}
	s2, _ := Open(p) // 重启后数据仍在
	defer s2.Close()
	if f, _ := s2.Favorites(); len(f) != 1 {
		t.Fatal("重启后数据丢失")
	}
}
