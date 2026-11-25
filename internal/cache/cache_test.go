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
	if s, err := c.Stats(); err != nil || s.Bytes > 100 || s.Files != 2 {
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
	if s, err := c.Stats(); err != nil || s.Bytes > 150 || s.Files != 1 {
		t.Fatalf("%+v", s)
	}
	_ = c.Clear()
	if s, err := c.Stats(); err != nil || s.Files != 0 {
		t.Fatal("clear")
	}
}

func TestUnavailableCacheDoesNotReportZeroOrAcceptNewLimit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	c, err := New(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Stats(); err == nil {
		t.Fatal("expected missing cache directory to report an error")
	}
	if err := c.SetMax(50); err == nil {
		t.Fatal("expected a failed resize")
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stats, err := c.Stats()
	if err != nil || stats.Max != 100 {
		t.Fatalf("failed resize changed the limit: %+v, %v", stats, err)
	}
}

func TestManagedCacheRequiresOwnershipAndDoesNotFollowFileSymlinks(t *testing.T) {
	parent := t.TempDir()
	c, err := CreateManaged(parent, "test-owner", 1000)
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(parent, "personal.txt")
	if err := os.WriteFile(other, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(c.Path(), "entry")); err != nil {
		t.Skip("test environment cannot create file symlinks")
	}
	if _, err := c.Get("entry"); err == nil {
		t.Fatal("read through a cache symlink")
	}
	if err := c.Put("entry", []byte("data")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(other); err != nil || string(data) != "keep" {
		t.Fatalf("cache changed another file: %q, %v", data, err)
	}
	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenManaged(c.Path(), "test-owner", 1000); err != nil {
		t.Fatalf("clear removed the marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(c.Path(), ownerFile), []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.Clear(); err == nil {
		t.Fatal("clear accepted a different owner")
	}
}

func TestDefaultCacheRefusesAReplacedDirectorySymlink(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	c, err := New(dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	personal := filepath.Join(other, "person.txt")
	if err := os.WriteFile(personal, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, dir); err != nil {
		t.Skip("test environment cannot create directory symlinks")
	}
	if err := c.Clear(); err == nil {
		t.Fatal("clear accepted a linked directory")
	}
	if _, err := New(dir, 1000); err == nil {
		t.Fatal("reset accepted a linked directory")
	}
	if data, err := os.ReadFile(personal); err != nil || string(data) != "keep" {
		t.Fatalf("personal file was affected: %q, %v", data, err)
	}
}
