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

func TestLocalDanmakuRetryAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jindi.db")
	first := New()
	if err := first.openStore(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.close)
	const id = "84d46f108fd048128c2c6dd0731e13d0"
	const secondID = "eeedf8dcb6c1446e9b11cc8f55d5a1fa"
	sent, err := first.SendLocalDanmakuOnce("d1", 32, 1205, "  重试验证  ", id)
	if err != nil {
		t.Fatal(err)
	}
	other := New()
	if err := other.openStore(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(other.close)
	again, err := other.SendLocalDanmakuOnce("d1", 32, 1205, "重试验证", id)
	if err != nil || again != sent {
		t.Fatalf("same request returned a different row: %+v, %v", again, err)
	}

	for _, mismatch := range []struct {
		episode int
		offset  int64
		text    string
	}{
		{31, 1205, "重试验证"},
		{32, 1206, "重试验证"},
		{32, 1205, "不同弹幕"},
	} {
		if _, err := other.SendLocalDanmakuOnce("d1", mismatch.episode, mismatch.offset, mismatch.text, id); err == nil {
			t.Fatal("a request ID was reused for a different payload")
		}
	}

	if _, err := other.SendLocalDanmakuOnce("d1", 32, 1205, "重试验证", "invalid"); err == nil {
		t.Fatal("invalid request ID accepted")
	}
	distinct, err := other.SendLocalDanmakuOnce("d1", 32, 1205, "重试验证", secondID)
	if err != nil || distinct.ID == sent.ID {
		t.Fatalf("distinct request did not create a new row: %+v, %v", distinct, err)
	}
	if rows, err := other.LocalDanmaku("d1", 32); err != nil || len(rows) != 2 || rows[0] != sent || rows[1] != distinct {
		t.Fatalf("unexpected persisted rows: %+v, %v", rows, err)
	}
}
