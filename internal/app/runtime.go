package app

import (
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"strconv"

	"jindi-drama/internal/cache"
	"jindi-drama/internal/observability"
)

const defaultCacheMax = 2 << 30 // 2 GiB

// openRuntime 初始化缓存与日志目录（dir 为空时使用默认数据目录）。
func (a *App) openRuntime(dir string) error {
	if dir == "" {
		d, err := DataDir()
		if err != nil {
			return err
		}
		dir = d
	}
	log, logErr := observability.New(filepath.Join(dir, "logs"), observability.DefaultLogBytes)
	if logErr == nil {
		a.log = log
	}
	a.cacheDefault = filepath.Join(dir, "cache")
	c, cacheErr := cache.New(a.cacheDefault, defaultCacheMax)
	if cacheErr == nil {
		a.cache = c
	}
	return errors.Join(logErr, cacheErr)
}

func (a *App) awaitStartup() {
	if a.startupDone != nil {
		<-a.startupDone
	}
}

func (a *App) CacheStats() (cache.Stats, error) {
	a.awaitStartup()
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if a.cache == nil {
		return cache.Stats{}, ErrNoStore
	}
	stats, err := a.cache.Stats()
	stats.Fallback = a.cacheFallback
	return stats, err
}
func (a *App) CacheClear() error {
	a.awaitStartup()
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if a.cache == nil {
		return ErrNoStore
	}
	return a.cache.Clear()
}

const cacheLimitKey = "cache_limit_mb"
const minCacheMB, maxCacheMB = 64, 1 << 20

func (a *App) CacheSetLimitMB(mb int) error {
	a.awaitStartup()
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if a.cache == nil || a.store == nil {
		return ErrNoStore
	}
	if mb < minCacheMB || mb > maxCacheMB {
		return errors.New("缓存上限需在 64MB 到 1TB 之间")
	}
	current, err := a.cache.Stats()
	if err != nil {
		return err
	}
	if err := a.cache.SetMax(int64(mb) << 20); err != nil {
		return err
	}
	if err := a.store.SetSetting(cacheLimitKey, strconv.Itoa(mb)); err != nil {
		return errors.Join(err, a.cache.SetMax(current.Max))
	}
	return nil
}

func (a *App) restoreCacheLimit() {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if a.store == nil || a.cache == nil {
		return
	}
	if err := a.cache.SetMax(a.savedCacheLimit()); err != nil {
		_ = a.LogEvent("warn", "cache", "恢复上次缓存上限失败")
	}
}

func (a *App) savedCacheLimit() int64 {
	if a.store == nil {
		return defaultCacheMax
	}
	raw, _, err := a.store.GetSetting(cacheLimitKey)
	if err != nil {
		return defaultCacheMax
	}
	mb, _ := strconv.Atoi(raw)
	if mb < minCacheMB || mb > maxCacheMB {
		// Earlier builds stored the preference in the general settings snapshot.
		raw, _, err = a.store.GetSetting("app")
		if err != nil {
			return defaultCacheMax
		}
		var settings struct {
			CacheLimitGB float64 `json:"cacheLimitGB"`
		}
		if json.Unmarshal([]byte(raw), &settings) != nil || settings.CacheLimitGB < 0.0625 || settings.CacheLimitGB > 1024 {
			return defaultCacheMax
		}
		mb = int(math.Round(settings.CacheLimitGB * 1024))
	}
	return int64(mb) << 20
}

// LogEvent 接收前端上报的故障（统一脱敏、限长）。
func (a *App) LogEvent(level, component, msg string) error {
	if a.log == nil {
		return ErrNoStore
	}
	if len(component) > 80 {
		component = component[:80]
	}
	if len(msg) > 2000 {
		msg = msg[:2000]
	}
	switch level {
	case "info", "warn", "error":
	default:
		level = "info"
	}
	return a.log.Log(level, component, msg)
}

// DiagnosticsTail 返回最近日志行，供“复制诊断信息”。
func (a *App) DiagnosticsTail(n int) ([]string, error) {
	if a.log == nil {
		return nil, ErrNoStore
	}
	if n <= 0 || n > 500 {
		n = 100
	}
	return a.log.Tail(n)
}
