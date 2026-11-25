package app

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"jindi-drama/internal/cache"
)

const cacheDirectoryKey = "cache_directory"

type cacheDirectory struct {
	Path  string `json:"path"`
	Owner string `json:"owner"`
}

func (a *App) ChooseCacheDirectory() (*cache.Stats, error) {
	a.awaitStartup()
	if a.ctx == nil || a.store == nil {
		return nil, ErrNoStore
	}
	parent, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择缓存所在目录", CanCreateDirectories: true})
	if err != nil {
		return nil, errors.New("无法打开系统目录选择器")
	}
	if parent == "" {
		return nil, nil
	}
	stats, err := a.activateCacheDirectory(parent)
	return &stats, err
}

func (a *App) activateCacheDirectory(parent string) (cache.Stats, error) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if a.store == nil {
		return cache.Stats{}, ErrNoStore
	}
	parent, err := filepath.EvalSymlinks(parent)
	if err != nil || !filepath.IsAbs(parent) {
		return cache.Stats{}, errors.New("无法访问所选目录")
	}
	if a.cache != nil && a.cache.Path() != a.cacheDefault && filepath.Dir(a.cache.Path()) == parent {
		if stats, err := a.cache.Stats(); err == nil {
			return stats, nil
		}
	}
	owner := rand.Text()
	next, err := cache.CreateManaged(parent, owner, a.savedCacheLimit())
	if err != nil {
		return cache.Stats{}, errors.New("无法在所选位置创建缓存目录")
	}
	stats, err := a.saveCacheDirectory(next, cacheDirectory{Path: next.Path(), Owner: owner})
	if err != nil {
		next.DiscardUnused()
	}
	return stats, err
}

func (a *App) ResetCacheDirectory() (cache.Stats, error) {
	a.awaitStartup()
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if a.store == nil || a.cacheDefault == "" {
		return cache.Stats{}, ErrNoStore
	}
	next, err := cache.New(a.cacheDefault, a.savedCacheLimit())
	if err != nil {
		return cache.Stats{}, errors.New("无法使用默认缓存目录")
	}
	stats, err := a.saveCacheDirectory(next, cacheDirectory{})
	if err != nil || stats.Bytes <= stats.Max {
		return stats, err
	}
	if err := next.SetMax(stats.Max); err != nil {
		_ = a.LogEvent("warn", "cache", "默认目录恢复后暂时无法缩减至缓存上限")
	}
	if updated, err := next.Stats(); err == nil {
		stats = updated
	}
	return stats, nil
}

func (a *App) saveCacheDirectory(next *cache.Cache, config cacheDirectory) (cache.Stats, error) {
	stats, err := next.Stats()
	if err != nil {
		return cache.Stats{}, errors.New("无法读取新缓存目录")
	}
	raw, _ := json.Marshal(config)
	if err := a.store.SetSetting(cacheDirectoryKey, string(raw)); err != nil {
		return cache.Stats{}, errors.New("无法保存缓存目录设置")
	}
	a.cache, a.cacheFallback = next, false
	return stats, nil
}

func (a *App) restoreCacheDirectory() {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if a.store == nil {
		return
	}
	value, exists, err := a.store.GetSetting(cacheDirectoryKey)
	if err != nil || !exists {
		return
	}
	var config cacheDirectory
	if json.Unmarshal([]byte(value), &config) == nil {
		if config == (cacheDirectory{}) {
			return
		}
		if next, err := cache.OpenManaged(config.Path, config.Owner, a.savedCacheLimit()); err == nil {
			a.cache = next
			return
		}
	}
	a.cacheFallback = true
	_ = a.LogEvent("warn", "cache", "已保存的缓存目录不可用")
}
