package app

import (
	"path/filepath"
	"testing"
)

func TestLocalDanmakuPersistsPerEpisode(t *testing.T) {
	a := New()
	db := filepath.Join(t.TempDir(), "jindi.db")
	if err := a.openStore(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)

	item, err := a.SendLocalDanmaku("d1", 32, 1205, "  好看！  ")
	if err != nil || item.ID < 1 || item.Text != "好看！" || item.OffsetMS != 1205 {
		t.Fatalf("send: %+v, %v", item, err)
	}
	if _, err := a.SendLocalDanmaku("d1", 33, 0, "尚未更新"); err == nil {
		t.Fatal("accepted a danmaku for an unavailable episode")
	}
	if _, err := a.SendLocalDanmaku("d1", 32, -1, "时间错误"); err == nil {
		t.Fatal("accepted a negative offset")
	}
	a.close()
	if err := a.openStore(db); err != nil {
		t.Fatal(err)
	}
	got, err := a.LocalDanmaku("d1", 32)
	if err != nil || len(got) != 1 || got[0] != item {
		t.Fatalf("after reopen: %v, %v", got, err)
	}
	got, err = a.LocalDanmaku("d1", 31)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("different episode: %v, %v", got, err)
	}
}
