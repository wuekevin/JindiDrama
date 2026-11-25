//go:build darwin && cgo && !bindings

package app

/*
#cgo LDFLAGS: -framework AVFoundation -framework CoreGraphics -framework CoreMedia -framework Foundation -framework ImageIO
#include <stdlib.h>
void *jindiDemoFrame(const char *path, double seconds, size_t *length);
*/
import "C"

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io"
	"io/fs"
	"os"
	"unsafe"
)

func nativeDemoScreenshot(assets fs.FS, asset string, seconds float64) (image.Image, error) {
	source, err := assets.Open(asset)
	if err != nil {
		return nil, errors.New("无法读取内置演示视频")
	}
	defer source.Close()

	file, err := os.CreateTemp("", "jindidrama-frame-*.mp4")
	if err != nil {
		return nil, errors.New("无法准备临时视频")
	}
	defer os.Remove(file.Name())
	_, copyErr := io.Copy(file, source)
	if errors.Join(copyErr, file.Close()) != nil {
		return nil, errors.New("无法准备临时视频")
	}

	path := C.CString(file.Name())
	defer C.free(unsafe.Pointer(path))
	var length C.size_t
	data := C.jindiDemoFrame(path, C.double(seconds), &length)
	if data == nil {
		return nil, errors.New("无法从当前播放位置捕捉画面")
	}
	defer C.free(data)
	return png.Decode(bytes.NewReader(C.GoBytes(data, C.int(length))))
}
