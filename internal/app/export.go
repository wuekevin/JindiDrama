package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var errDemoDirectCopy = errors.New("当前保存位置仅支持直接写入，演示视频保存中断")

func (a *App) ExportDemoEpisode(dramaID string, episode int, quality string) (bool, error) {
	if a.ctx == nil || a.assets == nil {
		return false, errors.New("桌面演示媒体暂不可导出")
	}
	if !a.beginMediaExport() {
		return false, errors.New("已有保存操作进行中，或窗口正在关闭")
	}
	defer a.endMediaExport()
	asset, err := a.demoExportAsset(dramaID, episode, quality)
	if err != nil {
		return false, err
	}
	filename := fmt.Sprintf("JindiDrama-demo-%s-%d", dramaID, episode)
	if quality == "sd" {
		filename += "-sd"
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "保存演示视频",
		DefaultFilename:      filename + ".mp4",
		Filters:              []runtime.FileFilter{{DisplayName: "MP4 视频", Pattern: "*.mp4"}},
		CanCreateDirectories: true,
	})
	if err != nil || path == "" {
		return false, err
	}
	if err := exportAsset(a.assets, asset, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, errors.New("目标文件已存在，请使用其他文件名")
		}
		if errors.Is(err, errDemoDirectCopy) {
			return false, errDemoDirectCopy
		}
		return false, errors.New("演示视频保存失败")
	}
	return true, nil
}

func (a *App) demoExportAsset(dramaID string, episode int, quality string) (string, error) {
	if quality != "sd" && quality != "hd" {
		return "", errors.New("无效的下载清晰度")
	}
	ticket, err := a.ResolvePlayback(dramaID, episode)
	if err != nil {
		return "", err
	}
	if ticket.Kind == "demo" {
		for _, source := range ticket.Sources {
			path := strings.TrimPrefix(source.Src, "./")
			matchesQuality := source.Quality == quality || (quality == "hd" && source.Quality == "")
			if matchesQuality && source.Type == "video/mp4" && strings.HasPrefix(path, "demo/") && fs.ValidPath(path) {
				return path, nil
			}
		}
	}
	return "", errors.New("本集没有可导出的内置演示视频")
}

func (a *App) ExportLicensedEpisode(dramaID string, episode int) (bool, error) {
	if a.ctx == nil {
		return false, errors.New("桌面本地片库暂不可导出")
	}
	if !a.beginMediaExport() {
		return false, errors.New("已有保存操作进行中，或窗口正在关闭")
	}
	defer a.endMediaExport()
	return a.exportLicensedEpisode(dramaID, episode, func(options runtime.SaveDialogOptions) (string, error) {
		return runtime.SaveFileDialog(a.ctx, options)
	})
}

func (a *App) beginMediaExport() bool {
	a.mediaExportMu.Lock()
	defer a.mediaExportMu.Unlock()
	if a.windowClosing || a.mediaExport != nil {
		return false
	}
	a.mediaExport = make(chan struct{})
	return true
}

func (a *App) endMediaExport() {
	a.mediaExportMu.Lock()
	defer a.mediaExportMu.Unlock()
	close(a.mediaExport)
	a.mediaExport = nil
}

func (a *App) exportLicensedEpisode(dramaID string, episode int, pick func(runtime.SaveDialogOptions) (string, error)) (bool, error) {
	if a.licensed == nil {
		return false, errors.New("桌面本地片库暂不可导出")
	}
	source, ext, err := a.licensed.OpenExport(dramaID, episode)
	if err != nil {
		return false, err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || info.Size() == 0 {
		return false, errors.New("找不到可读取的媒体文件")
	}
	path, err := pick(runtime.SaveDialogOptions{
		Title:                "保存本地片库视频",
		DefaultFilename:      fmt.Sprintf("JindiDrama-%s-%d%s", dramaID, episode, ext),
		Filters:              []runtime.FileFilter{{DisplayName: strings.ToUpper(ext[1:]) + " 视频", Pattern: "*" + ext}},
		CanCreateDirectories: true,
	})
	if err != nil {
		return false, errors.New("无法打开系统保存对话框")
	}
	if path == "" {
		return false, nil
	}
	if !strings.HasSuffix(strings.ToLower(path), ext) {
		return false, fmt.Errorf("请选择以 %s 结尾的文件名", ext)
	}
	if err := exportLicensedFile(source, info, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, errors.New("目标文件已存在，请使用其他文件名")
		}
		return false, errors.New("本地片库视频保存失败，请确认源文件仍可读取并检查保存位置")
	}
	return true, nil
}

func exportLicensedFile(source *os.File, expected os.FileInfo, target string) error {
	return writeStagedExport(target, func(staged io.Writer) error {
		if _, err := io.CopyN(staged, source, expected.Size()); err != nil {
			return err
		}
		actual, err := source.Stat()
		if err != nil {
			return err
		}
		if actual.Size() != expected.Size() || !actual.ModTime().Equal(expected.ModTime()) {
			return errors.New("源文件在导出期间发生变化")
		}
		return nil
	})
}

func writeStagedExport(target string, write func(io.Writer) error) error {
	return stageExport(target, write, publishExport)
}

func stageExport(target string, write func(io.Writer) error, publish func(string, string) error) (result error) {
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		if err == nil {
			return fs.ErrExist
		}
		return err
	}
	staged, err := os.CreateTemp(filepath.Dir(target), ".jindidrama-export-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(staged.Name()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}()
	if err := errors.Join(write(staged), staged.Close()); err != nil {
		return err
	}
	return publish(staged.Name(), target)
}

func exportAsset(assets fs.FS, path, target string) error {
	return exportDemoAsset(assets, path, target, publishExport)
}

func exportDemoAsset(assets fs.FS, path, target string, publish func(string, string) error) error {
	copyAsset := func(output io.Writer) error {
		source, err := assets.Open(path)
		if err != nil {
			return err
		}
		defer source.Close()
		_, err = io.Copy(output, source)
		return err
	}

	allowCopyFallback := false
	err := stageExport(target, copyAsset, func(staged, target string) error {
		err := publish(staged, target)
		if errors.Is(err, errors.ErrUnsupported) && !errors.Is(err, fs.ErrExist) {
			allowCopyFallback = true
			return nil
		}
		return err
	})
	if err != nil || !allowCopyFallback {
		return err
	}

	// The temporary copy is already gone so the destination only needs space for one file.
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := copyAsset(file); err != nil {
		// Acting on the open file keeps any replacement at the same path intact.
		return errors.Join(errDemoDirectCopy, err, file.Truncate(0), file.Close())
	}
	if err := file.Close(); err != nil {
		return errors.Join(errDemoDirectCopy, err)
	}
	return nil
}
