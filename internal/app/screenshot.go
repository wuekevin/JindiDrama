package app

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"math"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ExportDemoScreenshot(dramaID string, episode int, quality string, seconds float64, data string) (bool, error) {
	if a.ctx == nil || a.assets == nil {
		return false, errors.New("桌面截图暂不可保存")
	}
	if !a.beginMediaExport() {
		return false, errors.New("已有保存操作进行中，或窗口正在关闭")
	}
	defer a.endMediaExport()
	return a.exportDemoScreenshot(dramaID, episode, quality, seconds, data, func(options runtime.SaveDialogOptions) (string, error) {
		return runtime.SaveFileDialog(a.ctx, options)
	})
}

func (a *App) exportDemoScreenshot(dramaID string, episode int, quality string, seconds float64, data string, pick func(runtime.SaveDialogOptions) (string, error)) (bool, error) {
	asset, err := a.demoExportAsset(dramaID, episode, quality)
	if err != nil {
		return false, err
	}
	var frame image.Image
	if data == "" && a.assets != nil && !math.IsNaN(seconds) && seconds >= 0 && seconds <= 60 {
		frame, err = nativeDemoScreenshot(a.assets, asset, seconds)
	} else {
		frame, err = decodeScreenshot(data)
	}
	if err != nil {
		return false, err
	}
	target, err := pick(runtime.SaveDialogOptions{
		Title:                "保存视频截图",
		DefaultFilename:      fmt.Sprintf("JindiDrama-%s-%d.png", dramaID, episode),
		Filters:              []runtime.FileFilter{{DisplayName: "PNG 图片", Pattern: "*.png"}},
		CanCreateDirectories: true,
	})
	if err != nil {
		return false, errors.New("无法打开系统保存对话框")
	}
	if target == "" {
		return false, nil
	}
	if !strings.HasSuffix(strings.ToLower(target), ".png") {
		return false, errors.New("请选择以 .png 结尾的文件名")
	}
	if err := writeStagedExport(target, func(output io.Writer) error {
		return png.Encode(output, frame)
	}); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, errors.New("目标文件已存在，请使用其他文件名")
		}
		if destinationErr := exportDestinationError(err); destinationErr != nil {
			return false, destinationErr
		}
		return false, errors.New("截图保存失败")
	}
	return true, nil
}

func decodeScreenshot(data string) (image.Image, error) {
	const maxBase64Length = 8 << 20
	const maxDimension = 1920
	const maxPixels = 1920 * 1080
	invalid := errors.New("无效的视频截图")
	if data == "" || len(data) > maxBase64Length {
		return nil, invalid
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(data)
	if err != nil {
		return nil, invalid
	}
	config, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width <= 0 || config.Height <= 0 ||
		config.Width > maxDimension || config.Height > maxDimension || config.Width*config.Height > maxPixels {
		return nil, invalid
	}
	frame, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, invalid
	}
	return frame, nil
}
