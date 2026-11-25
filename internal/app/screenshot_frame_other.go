//go:build !darwin || !cgo || bindings

package app

import (
	"errors"
	"image"
	"io/fs"
)

func nativeDemoScreenshot(fs.FS, string, float64) (image.Image, error) {
	return nil, errors.New("当前环境无法捕捉内置视频画面")
}
