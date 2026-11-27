package app

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// DownloadUpdate saves the package bound to the last verified release check.
func (a *App) DownloadUpdate(releaseURL string) (bool, error) {
	path, err := a.DownloadUpdateFile(releaseURL)
	return path != "", err
}

type UpdateDownloadResult struct {
	Path    string `json:"path"`
	Warning string `json:"warning,omitempty"`
}

// DownloadUpdateWithStatus preserves the verified path when only stage cleanup fails.
func (a *App) DownloadUpdateWithStatus(releaseURL string) (UpdateDownloadResult, error) {
	return updateDownloadResult(a.DownloadUpdateFile(releaseURL))
}

func updateDownloadResult(path string, err error) (UpdateDownloadResult, error) {
	if path != "" && errors.Is(err, errUpdateSavedCleanup) {
		return UpdateDownloadResult{Path: path, Warning: err.Error()}, nil
	}
	return UpdateDownloadResult{Path: path}, err
}

// DownloadUpdateFile returns the published path after validation, or an empty path on cancellation.
// A published path can accompany a staging cleanup error.
func (a *App) DownloadUpdateFile(releaseURL string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("窗口尚未就绪")
	}
	a.updateMu.Lock()
	asset := a.releaseAsset
	if asset == nil || releaseURL != a.releaseURL {
		a.updateMu.Unlock()
		return "", errors.New("安装包已变更，请重新检查更新")
	}
	if !a.beginMediaExport() {
		a.updateMu.Unlock()
		return "", errors.New("已有保存操作进行中，或窗口正在关闭")
	}
	ctx, cancel := context.WithCancel(a.context())
	a.cancelUpdate = cancel
	a.updateMu.Unlock()
	defer func() {
		cancel()
		a.updateMu.Lock()
		a.cancelUpdate = nil
		a.endMediaExport()
		a.updateMu.Unlock()
	}()

	ext := filepath.Ext(asset.Name)
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "保存新版安装包",
		DefaultFilename:      asset.Name,
		Filters:              []runtime.FileFilter{{DisplayName: "金帝短剧安装包", Pattern: "*" + ext}},
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", errors.New("无法打开系统保存对话框")
	}
	if path == "" || ctx.Err() != nil {
		return "", nil
	}
	if !strings.EqualFold(filepath.Ext(path), ext) {
		return "", errors.New("请选择以 " + ext + " 结尾的文件名")
	}
	a.updateMu.Lock()
	current := asset == a.releaseAsset
	a.updateMu.Unlock()
	if !current {
		return "", errors.New("安装包已变更，请重新检查更新")
	}
	saved, err := saveUpdate(ctx, path, asset.Download, func(publish func() error) error {
		a.updateMu.Lock()
		defer a.updateMu.Unlock()
		return publish()
	})
	if !saved {
		return "", err
	}
	return path, err
}

func (a *App) CancelUpdateDownload() {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	if a.cancelUpdate != nil {
		a.cancelUpdate()
	}
}

type stagedOutput struct {
	file       *os.File
	publish    func(string) error
	cleanup    func() error
	copyDirect func(string, func(io.Writer) error) error
}

type outputStageKind uint8

const (
	mediaOutputStage outputStageKind = iota
	privateSnapshotStage
	privateBackupOutputStage
	updateOutputStage
)

func createUpdateStage(parent string) (*stagedOutput, error) {
	return createOutputStage(parent, ".jindidrama-update-", updateOutputStage)
}

var (
	errUpdateCleanup                      = errors.New("暂存文件清理失败，请检查所选目录")
	errUpdateMovedCleanup                 = errors.New("暂存目录清理失败，请在原目录检查")
	errUpdateSavedCleanup                 = errors.New("安装包已校验并保存，但暂存目录清理失败，请检查所选目录")
	errUpdateOrigin                       = errors.New("当前保存位置无法标记文件来源，请选择其他位置或前往发布页下载")
	errUpdateDestinationMoved             = errors.New("安装包保存目录已移动或替换，请重新选择保存位置")
	errUpdateDestinationMovedAfterPublish = errors.New("安装包已保存到原目录，但该目录已移动或替换，请在原目录查找安装包")
)

func outputStageSetupError(kind outputStageKind, cause, cleanupErr error) error {
	if kind == updateOutputStage && cleanupErr != nil {
		return errors.Join(cause, errUpdateCleanup)
	}
	return cause
}

func saveUpdate(ctx context.Context, path string, download func(context.Context, io.Writer) error, commit func(func() error) error) (saved bool, result error) {
	const saveError = "无法保存安装包，请检查保存位置和可用空间"
	if _, err := os.Lstat(path); err == nil {
		return false, errors.New("目标文件已存在，请使用其他文件名")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, errors.New(saveError)
	}
	stage, err := createUpdateStage(filepath.Dir(path))
	if errors.Is(err, errUpdateOrigin) {
		return false, err
	}
	if err != nil {
		if errors.Is(err, errUpdateCleanup) {
			return false, errors.Join(errors.New(saveError), errUpdateCleanup)
		}
		return false, errors.New(saveError)
	}
	defer func() {
		if err := stage.cleanup(); err == nil {
			return
		}
		if saved {
			result = errUpdateSavedCleanup
			return
		}
		if errors.Is(result, errUpdateDestinationMovedAfterPublish) {
			result = errors.Join(result, errUpdateMovedCleanup)
			return
		}
		result = errors.Join(result, errUpdateCleanup)
	}()
	if err := download(ctx, stage.file); err != nil {
		if contextErr := ctx.Err(); contextErr != nil && errors.Is(err, contextErr) {
			return false, nil
		}
		return false, err
	}
	if err := stage.file.Sync(); err != nil {
		return false, errors.New(saveError)
	}
	err = commit(func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return stage.publish(path)
	})
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, nil
	}
	if errors.Is(err, fs.ErrExist) {
		return false, errors.New("目标文件已存在，请使用其他文件名")
	}
	if errors.Is(err, errUpdateDestinationMoved) || errors.Is(err, errUpdateDestinationMovedAfterPublish) {
		return false, err
	}
	if err != nil {
		return false, errors.New(saveError)
	}
	return true, nil
}

func publishInUpdateDestination(matches func() bool, publish func() error) error {
	if !matches() {
		return errUpdateDestinationMoved
	}
	if err := publish(); err != nil {
		return err
	}
	// The descriptor pins publication to the original directory. If its
	// path changed during the rename, tell the caller where the file went.
	if !matches() {
		return errUpdateDestinationMovedAfterPublish
	}
	return nil
}
