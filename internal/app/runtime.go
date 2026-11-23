package app

import (
	"errors"
	"path/filepath"

	"jindi-drama/internal/cache"
	"jindi-drama/internal/observability"
)

const defaultCacheMax = 2 << 30 // 2 GiB

// OpenRuntime 初始化缓存与日志目录（dir 为空时使用默认数据目录）。
func (a *App) OpenRuntime(dir string) error {
	if dir == "" {
		d, err := DataDir()
		if err != nil {
			return err
		}
		dir = d
	}
	c, err := cache.New(filepath.Join(dir, "cache"), defaultCacheMax)
	if err != nil {
		return err
	}
	l, err := observability.New(filepath.Join(dir, "logs"), 1<<20)
	if err != nil {
		return err
	}
	a.cache, a.log = c, l
	return nil
}

func (a *App) CacheStats() (cache.Stats, error) {
	if a.cache == nil {
		return cache.Stats{}, ErrNoStore
	}
	return a.cache.Stats(), nil
}
func (a *App) CacheClear() error {
	if a.cache == nil {
		return ErrNoStore
	}
	return a.cache.Clear()
}
func (a *App) CacheSetLimitMB(mb int) error {
	if a.cache == nil {
		return ErrNoStore
	}
	if mb < 64 || mb > 1<<20 {
		return errors.New("缓存上限需在 64MB 到 1TB 之间")
	}
	return a.cache.SetMax(int64(mb) << 20)
}

// LogEvent 接收前端上报的故障（统一脱敏、限长）。
func (a *App) LogEvent(level, component, msg string) error {
	if a.log == nil {
		return ErrNoStore
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
