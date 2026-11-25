package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeBindings(t *testing.T) {
	a := New()
	if err := a.CacheClear(); err != ErrNoStore {
		t.Fatal("未初始化应报错")
	}
	if err := a.openRuntime(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := a.openStore(filepath.Join(t.TempDir(), "jindi.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	if err := a.CacheSetLimitMB(1); err == nil {
		t.Fatal("过小上限应拒绝")
	}
	if err := a.CacheSetLimitMB(512); err != nil {
		t.Fatal(err)
	}
	if s, _ := a.CacheStats(); s.Max != 512<<20 {
		t.Fatalf("%+v", s)
	}
	_ = a.LogEvent("error", "player", "load fail Cookie: sid=SECRET https://x.test/a?token=1")
	_ = a.LogEvent("weird", "player", strings.Repeat("a", 5000))
	lines, _ := a.DiagnosticsTail(10)
	if len(lines) != 2 || strings.Contains(lines[0], "SECRET") || strings.Contains(lines[0], "token=1") || !strings.Contains(lines[1], "[INFO]") {
		t.Fatalf("%v", lines)
	}
}

func TestRestoreCacheLimitFromDesktopSettings(t *testing.T) {
	a := New()
	root := t.TempDir()
	if err := a.openRuntime(root); err != nil {
		t.Fatal(err)
	}
	if err := a.openStore(filepath.Join(root, "jindi.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	if err := a.SaveSetting("app", `{"cacheLimitGB":0.5}`); err != nil {
		t.Fatal(err)
	}
	a.restoreCacheLimit()
	stats, err := a.CacheStats()
	if err != nil || stats.Max != 512<<20 || stats.Path != filepath.Join(root, "cache") {
		t.Fatalf("runtime cache: %+v, %v", stats, err)
	}
	if err := a.CacheSetLimitMB(1024); err != nil {
		t.Fatal(err)
	}
	if err := a.openRuntime(root); err != nil {
		t.Fatal(err)
	}
	a.restoreCacheLimit()
	stats, err = a.CacheStats()
	if err != nil || stats.Max != 1<<30 {
		t.Fatalf("persisted cache limit: %+v, %v", stats, err)
	}
}

func TestCacheLimitRollsBackWhenSettingsWriteFails(t *testing.T) {
	a := New()
	root := t.TempDir()
	if err := a.openRuntime(root); err != nil {
		t.Fatal(err)
	}
	if err := a.openStore(filepath.Join(root, "jindi.db")); err != nil {
		t.Fatal(err)
	}
	a.close()
	if err := a.CacheSetLimitMB(512); err == nil {
		t.Fatal("expected the database write to fail")
	}
	stats, err := a.CacheStats()
	if err != nil || stats.Max != defaultCacheMax {
		t.Fatalf("cache limit after failed database write: %+v, %v", stats, err)
	}
}

func TestCacheFailureKeepsDiagnosticsReadable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cache"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := New()
	if err := a.openRuntime(root); err == nil {
		t.Fatal("cache failure should be reported")
	}
	if err := a.LogEvent("warn", "session", "expected diagnostic"); err != nil {
		t.Fatal(err)
	}
	lines, err := a.DiagnosticsTail(10)
	if err != nil || len(lines) != 1 || !strings.Contains(lines[0], "expected diagnostic") {
		t.Fatalf("diagnostics unavailable after cache failure: %v, %v", lines, err)
	}
}
