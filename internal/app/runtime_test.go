package app

import (
	"strings"
	"testing"
)

func TestRuntimeBindings(t *testing.T) {
	a := New()
	if err := a.CacheClear(); err != ErrNoStore {
		t.Fatal("未初始化应报错")
	}
	if err := a.OpenRuntime(t.TempDir()); err != nil {
		t.Fatal(err)
	}
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
