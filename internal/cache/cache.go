// Package cache 管理本地媒体/下载缓存：有总容量上限，超限时按最久未访问淘汰（LRU）。
package cache

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Cache struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
}

type Stats struct {
	Bytes int64 `json:"bytes"`
	Files int   `json:"files"`
	Max   int64 `json:"max"`
}

func New(dir string, maxBytes int64) (*Cache, error) {
	if maxBytes <= 0 {
		return nil, errors.New("缓存上限必须大于 0")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Cache{dir: dir, maxBytes: maxBytes}, nil
}

// safe 拒绝路径穿越：key 只能是单层文件名。
func (c *Cache) safe(key string) (string, error) {
	if key == "" || key != filepath.Base(key) || strings.HasPrefix(key, ".") {
		return "", errors.New("非法缓存键")
	}
	return filepath.Join(c.dir, key), nil
}

func (c *Cache) Put(key string, data []byte) error {
	p, err := c.safe(key)
	if err != nil {
		return err
	}
	if int64(len(data)) > c.maxBytes {
		return errors.New("单个文件超过缓存上限")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return err
	}
	return c.evictLocked(p)
}

func (c *Cache) Get(key string) ([]byte, error) {
	p, err := c.safe(key)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := os.ReadFile(p)
	if err == nil {
		now := time.Now()
		_ = os.Chtimes(p, now, now) // 记录访问时间
	}
	return b, err
}

type entry struct {
	path string
	size int64
	at   time.Time
}

func (c *Cache) list() ([]entry, int64, error) {
	ents, err := os.ReadDir(c.dir)
	if err != nil {
		return nil, 0, err
	}
	var out []entry
	var total int64
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, entry{filepath.Join(c.dir, e.Name()), fi.Size(), fi.ModTime()})
		total += fi.Size()
	}
	return out, total, nil
}

func (c *Cache) evictLocked(keep string) error {
	es, total, err := c.list()
	if err != nil {
		return err
	}
	if total <= c.maxBytes {
		return nil
	}
	sort.Slice(es, func(i, j int) bool { return es[i].at.Before(es[j].at) })
	for _, e := range es {
		if total <= c.maxBytes {
			break
		}
		if e.path == keep {
			continue
		}
		if os.Remove(e.path) == nil {
			total -= e.size
		}
	}
	return nil
}

func (c *Cache) SetMax(maxBytes int64) error {
	if maxBytes <= 0 {
		return errors.New("缓存上限必须大于 0")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxBytes = maxBytes
	return c.evictLocked("")
}

func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	es, total, _ := c.list()
	return Stats{Bytes: total, Files: len(es), Max: c.maxBytes}
}

func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	es, _, err := c.list()
	if err != nil {
		return err
	}
	for _, e := range es {
		if err := os.Remove(e.path); err != nil {
			return err
		}
	}
	return nil
}
