// Package cache 管理本地媒体/下载缓存：有总容量上限，超限时按最久未访问淘汰（LRU）。
package cache

import (
	"crypto/rand"
	"errors"
	"io/fs"
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
	owner    string
}

const ownerFile = ".jindidrama-cache"

// CreateManaged creates a private cache under a directory selected by the user.
func CreateManaged(parent, owner string, maxBytes int64) (*Cache, error) {
	if owner == "" || maxBytes <= 0 {
		return nil, errors.New("缓存目录配置无效")
	}
	dir, err := os.MkdirTemp(parent, "JindiDrama-cache-")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, ownerFile), []byte(owner), 0o600); err != nil {
		_ = os.Remove(dir)
		return nil, err
	}
	return &Cache{dir: dir, maxBytes: maxBytes, owner: owner}, nil
}

func OpenManaged(dir, owner string, maxBytes int64) (*Cache, error) {
	c := &Cache{dir: dir, maxBytes: maxBytes, owner: owner}
	if owner == "" || maxBytes <= 0 || !filepath.IsAbs(dir) || !strings.HasPrefix(filepath.Base(dir), "JindiDrama-cache-") {
		return nil, errors.New("缓存目录配置无效")
	}
	root, err := c.open()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if _, _, err := list(root); err != nil {
		return nil, err
	}
	probe := ".probe-" + rand.Text()
	file, err := root.OpenFile(probe, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(file.Close(), root.Remove(probe)); err != nil {
		return nil, err
	}
	return c, nil
}

// DiscardUnused removes a fresh directory after saving its configuration fails.
func (c *Cache) DiscardUnused() {
	if c.owner == "" {
		return
	}
	root, err := c.open()
	if err != nil {
		return
	}
	entries, err := fs.ReadDir(root.FS(), ".")
	if err == nil && len(entries) == 1 && entries[0].Name() == ownerFile {
		info, statErr := root.Stat(".")
		removeErr := root.Remove(ownerFile)
		_ = root.Close()
		current, currentErr := os.Lstat(c.dir)
		if statErr == nil && removeErr == nil && currentErr == nil && os.SameFile(info, current) {
			_ = os.Remove(c.dir)
		}
		return
	}
	_ = root.Close()
}

func (c *Cache) open() (*os.Root, error) {
	info, err := os.Lstat(c.dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("缓存位置必须是真实目录")
	}
	root, err := os.OpenRoot(c.dir)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		_ = root.Close()
		return nil, errors.New("缓存目录已更换")
	}
	if c.owner == "" {
		return root, nil
	}
	if info, err := root.Lstat(ownerFile); err == nil && info.Mode().IsRegular() && info.Size() == int64(len(c.owner)) {
		if marker, err := root.ReadFile(ownerFile); err == nil && string(marker) == c.owner {
			return root, nil
		}
	}
	_ = root.Close()
	return nil, errors.New("无法确认缓存目录归属")
}

func (c *Cache) Path() string { return c.dir }

type Stats struct {
	Bytes    int64  `json:"bytes"`
	Files    int    `json:"files"`
	Max      int64  `json:"max"`
	Path     string `json:"path"`
	Custom   bool   `json:"custom,omitempty"`
	Fallback bool   `json:"fallback,omitempty"`
}

func New(dir string, maxBytes int64) (*Cache, error) {
	if maxBytes <= 0 {
		return nil, errors.New("缓存上限必须大于 0")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	c := &Cache{dir: dir, maxBytes: maxBytes}
	root, err := c.open()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return c, nil
}

// safe 拒绝路径穿越：key 只能是单层文件名。
func safe(key string) bool {
	return filepath.IsLocal(key) && !strings.ContainsAny(key, `\/:`) && !strings.HasPrefix(key, ".")
}

func (c *Cache) Put(key string, data []byte) error {
	if !safe(key) {
		return errors.New("非法缓存键")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if int64(len(data)) > c.maxBytes {
		return errors.New("单个文件超过缓存上限")
	}
	root, err := c.open()
	if err != nil {
		return err
	}
	defer root.Close()
	staged := ".put-" + rand.Text()
	file, err := root.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(staged)
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	if err := root.Rename(staged, key); err != nil {
		return err
	}
	return c.evictLocked(root, key)
}

func (c *Cache) Get(key string) ([]byte, error) {
	if !safe(key) {
		return nil, errors.New("非法缓存键")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	root, err := c.open()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(key)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("非法缓存文件")
	}
	b, err := root.ReadFile(key)
	if err == nil {
		now := time.Now()
		_ = root.Chtimes(key, now, now) // 记录访问时间
	}
	return b, err
}

type entry struct {
	path string
	size int64
	at   time.Time
}

func list(root *os.Root) ([]entry, int64, error) {
	ents, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, 0, err
	}
	var out []entry
	var total int64
	for _, e := range ents {
		if !safe(e.Name()) || !e.Type().IsRegular() {
			continue
		}
		fi, err := e.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		if !fi.Mode().IsRegular() {
			continue
		}
		out = append(out, entry{e.Name(), fi.Size(), fi.ModTime()})
		total += fi.Size()
	}
	return out, total, nil
}

func (c *Cache) evictLocked(root *os.Root, keep string) error {
	es, total, err := list(root)
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
		if root.Remove(e.path) == nil {
			total -= e.size
		}
	}
	if total > c.maxBytes {
		return errors.New("无法将缓存缩减到指定上限")
	}
	return nil
}

func (c *Cache) SetMax(maxBytes int64) error {
	if maxBytes <= 0 {
		return errors.New("缓存上限必须大于 0")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	root, err := c.open()
	if err != nil {
		return err
	}
	defer root.Close()
	previous := c.maxBytes
	c.maxBytes = maxBytes
	if err := c.evictLocked(root, ""); err != nil {
		c.maxBytes = previous
		return err
	}
	return nil
}

func (c *Cache) Stats() (Stats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	root, err := c.open()
	if err != nil {
		return Stats{}, err
	}
	defer root.Close()
	es, total, err := list(root)
	if err != nil {
		return Stats{}, err
	}
	return Stats{Bytes: total, Files: len(es), Max: c.maxBytes, Path: c.dir, Custom: c.owner != ""}, nil
}

func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	root, err := c.open()
	if err != nil {
		return err
	}
	defer root.Close()
	es, _, err := list(root)
	if err != nil {
		return err
	}
	for _, e := range es {
		if err := root.Remove(e.path); err != nil {
			return err
		}
	}
	return nil
}
