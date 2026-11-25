package app

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"jindi-drama/internal/storage/sqlite"
)

// RestoreLibraryBackup requires a native selection and merges only missing
// library rows. The WebView cannot pass a path or import other tables.
func (a *App) RestoreLibraryBackup() (*sqlite.LibraryRestore, error) {
	a.awaitStartup()
	if a.store == nil {
		return nil, ErrNoStore
	}
	if a.ctx == nil {
		return nil, errors.New("无法打开系统文件选择器")
	}
	options := runtime.OpenDialogOptions{
		Title:   "选择金帝短剧数据库备份",
		Filters: []runtime.FileFilter{{DisplayName: "金帝短剧备份", Pattern: "*.db"}},
	}
	if dir, err := DataDir(); err == nil {
		backups := filepath.Join(dir, "backups")
		if info, err := os.Stat(backups); err == nil && info.IsDir() {
			options.DefaultDirectory = backups
		}
	}
	return a.restoreSelectedLibraryBackup(func() (string, error) { return runtime.OpenFileDialog(a.ctx, options) })
}

func (a *App) restoreSelectedLibraryBackup(pick func() (string, error)) (*sqlite.LibraryRestore, error) {
	path, err := pick()
	if err != nil {
		return nil, errors.New("无法打开系统文件选择器")
	}
	if path == "" {
		return nil, nil
	}
	state, err := sqlite.ReadLibraryBackup(path)
	if err != nil {
		return nil, errors.New("无法读取所选备份，请选择金帝短剧生成的数据库备份")
	}
	result, err := a.store.RestoreMissingLibrary(state)
	if err != nil {
		return nil, errors.New("无法保存恢复的收藏和历史")
	}
	return result, nil
}
