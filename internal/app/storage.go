package app

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"time"

	"jindi-drama/internal/storage/sqlite"
)

// 允许前端持久化的设置键（白名单，防止任意写入）。
var settingKeys = map[string]bool{"app": true, "player": true, "auth_demo": true}

// DataDir returns the explicit desktop data directory or the OS default.
func DataDir() (string, error) {
	if configured := os.Getenv("JINDIDRAMA_DATA_DIR"); configured != "" {
		if !filepath.IsAbs(configured) {
			return "", errors.New("JINDIDRAMA_DATA_DIR 必须为绝对路径")
		}
		return filepath.Clean(configured), nil
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "JindiDrama"), nil
}

// openStore 打开（并迁移）本地数据库。path 为空时使用默认目录。
func (a *App) openStore(path string) error {
	if path == "" {
		d, err := DataDir()
		if err != nil {
			return err
		}
		path = filepath.Join(d, "jindi.db")
	}
	s, err := sqlite.Open(path)
	if err != nil {
		return err
	}
	a.store = s
	return nil
}

func (a *App) close() {
	if a.store != nil {
		a.store.Close()
	}
}

var ErrNoStore = errors.New("本地存储未就绪")

func (a *App) StorageReady() bool {
	a.awaitStartup()
	return a.store != nil
}

func (a *App) Favorites() ([]string, error) {
	if a.store == nil {
		return nil, ErrNoStore
	}
	return a.store.Favorites()
}
func (a *App) SaveFavorites(ids []string) error {
	if a.store == nil {
		return ErrNoStore
	}
	return a.store.SetFavorites(ids)
}
func (a *App) SaveLibraryChanges(changes sqlite.LibraryChanges) (*sqlite.LibraryState, error) {
	if a.store == nil {
		return nil, ErrNoStore
	}
	return a.store.WriteLibraryChanges(changes)
}

func (a *App) History() ([]sqlite.HistoryEntry, error) {
	if a.store == nil {
		return nil, ErrNoStore
	}
	return a.store.History()
}
func (a *App) RecordHistory(h sqlite.HistoryEntry) error {
	if a.store == nil {
		return ErrNoStore
	}
	return a.store.RecordHistory(h)
}
func (a *App) RemoveHistory(ids []string) error {
	if a.store == nil {
		return ErrNoStore
	}
	return a.store.RemoveHistory(ids)
}
func (a *App) ClearHistory() error {
	if a.store == nil {
		return ErrNoStore
	}
	return a.store.ClearHistory()
}
func (a *App) SearchHistory() ([]string, error) {
	if a.store == nil {
		return nil, ErrNoStore
	}
	return a.store.SearchHistory()
}
func (a *App) SaveSearchHistory(q []string) error {
	if a.store == nil {
		return ErrNoStore
	}
	return a.store.SetSearchHistory(q)
}
func (a *App) GetSetting(key string) (string, error) {
	if a.store == nil {
		return "", ErrNoStore
	}
	if !settingKeys[key] {
		return "", errors.New("不允许的设置键")
	}
	v, _, err := a.store.GetSetting(key)
	return v, err
}
func (a *App) SaveSetting(key, valueJSON string) error {
	if a.store == nil {
		return ErrNoStore
	}
	if !settingKeys[key] {
		return errors.New("不允许的设置键")
	}
	if len(valueJSON) > 64*1024 {
		return errors.New("设置内容过大")
	}
	return a.store.SetSetting(key, valueJSON)
}

// BackupNow 备份到数据目录下 backups/，返回备份文件路径。
func (a *App) BackupNow() (string, error) {
	if a.store == nil {
		return "", ErrNoStore
	}
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return a.backupTo(d)
}

func (a *App) backupTo(dir string) (string, error) {
	name := "jindi-" + time.Now().Format("20060102-150405") + "-" + rand.Text() + ".db"
	dest := filepath.Join(dir, "backups", name)
	if err := publishDatabaseBackup(dest, a.store.Backup); err != nil {
		return "", err
	}
	return dest, nil
}

func publishDatabaseBackup(dest string, snapshot func(string) error) (result error) {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(dir, ".jindi-backup-")
	if err != nil {
		return err
	}
	defer func() {
		// A cleanup error must not relabel an already published backup as failed.
		if err := os.RemoveAll(staging); result != nil && err != nil {
			result = errors.Join(result, err)
		}
	}()
	staged := filepath.Join(staging, "snapshot")
	if err := snapshot(staged); err != nil {
		return err
	}
	file, err := os.OpenFile(staged, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err := errors.Join(file.Chmod(0o600), file.Sync(), file.Close()); err != nil {
		return err
	}
	return publishExport(staged, dest)
}
