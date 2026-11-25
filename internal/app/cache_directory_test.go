package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheDirectoryPersistsAndPreservesOtherLocations(t *testing.T) {
	data, parent := t.TempDir(), t.TempDir()
	database := filepath.Join(data, "jindi.db")
	start := func() *App {
		t.Helper()
		a := New()
		if err := a.openStore(database); err != nil {
			t.Fatal(err)
		}
		if err := a.openRuntime(data); err != nil {
			t.Fatal(err)
		}
		a.restoreCacheDirectory()
		a.restoreCacheLimit()
		t.Cleanup(a.close)
		return a
	}
	a := start()
	if err := a.CacheSetLimitMB(640); err != nil {
		t.Fatal(err)
	}
	if err := a.cache.Put("old", []byte("default data")); err != nil {
		t.Fatal(err)
	}
	parentFile := filepath.Join(parent, "personal.txt")
	if err := os.WriteFile(parentFile, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	chosen, err := a.activateCacheDirectory(parent)
	resolved, pathErr := filepath.EvalSymlinks(parent)
	if err != nil || pathErr != nil || !chosen.Custom || filepath.Dir(chosen.Path) != resolved || chosen.Max != 640<<20 {
		t.Fatalf("chosen cache: %+v, %v, %v", chosen, err, pathErr)
	}
	if err := a.cache.Put("entry", []byte("cache bytes")); err != nil {
		t.Fatal(err)
	}
	a.close()
	b := start()
	if got, err := b.CacheStats(); err != nil || got.Path != chosen.Path || got.Files != 1 || got.Fallback {
		t.Fatalf("restored cache: %+v, %v", got, err)
	}
	if err := b.CacheClear(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{parentFile, filepath.Join(data, "cache", "old")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("other location changed: %v", err)
		}
	}
	if err := os.Rename(chosen.Path, chosen.Path+"-moved"); err != nil {
		t.Fatal(err)
	}
	b.close()
	c := start()
	if got, err := c.CacheStats(); err != nil || !got.Fallback || got.Path != filepath.Join(data, "cache") || got.Files != 1 {
		t.Fatalf("fallback cache: %+v, %v", got, err)
	}
	if got, err := c.ResetCacheDirectory(); err != nil || got.Fallback || got.Custom || got.Max != 640<<20 {
		t.Fatalf("reset cache: %+v, %v", got, err)
	}
	c.close()
	if got, err := start().CacheStats(); err != nil || got.Fallback || got.Custom {
		t.Fatalf("persisted reset: %+v, %v", got, err)
	}
}

func TestCacheDirectoryWriteFailureLeavesPreviousCache(t *testing.T) {
	a := New()
	data, parent := t.TempDir(), t.TempDir()
	if err := a.openRuntime(data); err != nil {
		t.Fatal(err)
	}
	if err := a.openStore(filepath.Join(data, "jindi.db")); err != nil {
		t.Fatal(err)
	}
	a.close()
	if _, err := a.activateCacheDirectory(parent); err == nil {
		t.Fatal("closed settings database accepted a cache directory")
	}
	if got, err := a.CacheStats(); err != nil || got.Path != filepath.Join(data, "cache") {
		t.Fatalf("active cache was changed: %+v, %v", got, err)
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Fatalf("abandoned new directory: %+v, %v", entries, err)
	}
}

func TestCacheDirectoryResetAppliesCurrentLimit(t *testing.T) {
	a := New()
	data := t.TempDir()
	if err := a.openRuntime(data); err != nil {
		t.Fatal(err)
	}
	if err := a.openStore(filepath.Join(data, "jindi.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	old := filepath.Join(data, "cache", "default-data")
	if err := a.cache.Put("default-data", []byte("cached")); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(old, 65<<20); err != nil {
		t.Fatal(err)
	}
	if _, err := a.activateCacheDirectory(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := a.CacheSetLimitMB(64); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("inactive directory was affected: %v", err)
	}
	if stats, err := a.ResetCacheDirectory(); err != nil || stats.Bytes > stats.Max || stats.Files != 0 {
		t.Fatalf("restored cache outside limit: %+v, %v", stats, err)
	}
}

func TestCacheStatsWaitsForSavedLocationAtStartup(t *testing.T) {
	data := t.TempDir()
	t.Setenv("JINDIDRAMA_DATA_DIR", data)
	first := New()
	if err := first.openRuntime(data); err != nil {
		t.Fatal(err)
	}
	if err := first.openStore(filepath.Join(data, "jindi.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.close)
	chosen, err := first.activateCacheDirectory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first.close()

	next := New()
	ctx := context.Background()
	startup, shutdown := Lifecycle(next)
	defer shutdown(ctx)
	started, result := make(chan struct{}), make(chan string, 1)
	go func() {
		close(started)
		stats, _ := next.CacheStats()
		result <- stats.Path
	}()
	<-started
	select {
	case path := <-result:
		t.Fatalf("cache binding returned before startup: %q", path)
	case <-time.After(10 * time.Millisecond):
	}
	startup(ctx)
	if path := <-result; path != chosen.Path {
		t.Fatalf("first binding returned %q, want %q", path, chosen.Path)
	}
}
