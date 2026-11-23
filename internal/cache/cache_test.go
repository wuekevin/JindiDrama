package cache

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLRUEvictionKeepsUnderLimit(t *testing.T) {
	c, _ := New(t.TempDir(), 100)
	blob := bytes.Repeat([]byte("x"), 40)
	_ = c.Put("a", blob)
	time.Sleep(15 * time.Millisecond)
	_ = c.Put("b", blob)
	time.Sleep(15 * time.Millisecond)
	if _, err := c.Get("a"); err != nil { // 访问 a，使 b 成为最久未访问
		t.Fatal(err)
	}
	time.Sleep(15 * time.Millisecond)
	_ = c.Put("c", blob) // 120 > 100 → 淘汰 b
	if _, err := c.Get("b"); !os.IsNotExist(err) {
		t.Fatalf("b 应被淘汰: %v", err)
	}
	if _, err := c.Get("a"); err != nil {
		t.Fatal("a 不应被淘汰")
	}
	if s := c.Stats(); s.Bytes > 100 || s.Files != 2 {
		t.Fatalf("%+v", s)
	}
}

func TestRejectsTraversalAndOversize(t *testing.T) {
	d := t.TempDir()
	c, _ := New(filepath.Join(d, "c"), 10)
	for _, k := range []string{"../x", "a/b", "", ".hidden", "/etc/passwd"} {
		if err := c.Put(k, []byte("1")); err == nil {
			t.Fatalf("应拒绝 %q", k)
		}
	}
	if err := c.Put("big", bytes.Repeat([]byte("x"), 11)); err == nil {
		t.Fatal("超限单文件应拒绝")
	}
	if _, err := New(d, 0); err == nil {
		t.Fatal("上限 0 应拒绝")
	}
}

func TestSetMaxShrinksAndClear(t *testing.T) {
	c, _ := New(t.TempDir(), 1000)
	for _, k := range []string{"a", "b", "c"} {
		_ = c.Put(k, bytes.Repeat([]byte("x"), 100))
		time.Sleep(10 * time.Millisecond)
	}
	_ = c.SetMax(150)
	if s := c.Stats(); s.Bytes > 150 || s.Files != 1 {
		t.Fatalf("%+v", s)
	}
	_ = c.Clear()
	if s := c.Stats(); s.Files != 0 {
		t.Fatal("clear")
	}
}
