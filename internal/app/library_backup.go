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
	// Closing a raw descriptor for the live database can drop this process's
	// SQLite file locks on Unix. Refuse the live file and any aliases to it.
	if a.store.Path != ":memory:" {
		live, liveErr := os.Stat(a.store.Path)
		selected, selectedErr := os.Stat(path)
		if liveErr == nil && selectedErr == nil && os.SameFile(live, selected) {
			return nil, errors.New("请选择备份文件，当前数据库正在使用")
		}
	}
	var state *sqlite.LibraryState
	err = withPrivateDatabaseSnapshot(func(root *os.Root, filename string) error {
		var err error
		state, err = sqlite.ReadLibraryBackup(path, root, filename)
		return err
	})
	if err != nil {
		return nil, errors.New("无法读取所选备份，请选择金帝短剧生成的数据库备份")
	}
	result, err := a.store.RestoreMissingLibrary(state)
	if err != nil {
		return nil, errors.New("无法保存恢复的收藏和历史")
	}
	return result, nil
}
