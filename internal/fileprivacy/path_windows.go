package fileprivacy

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func WindowsPath(path string) (*uint16, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if len(path) >= 248 && !strings.HasPrefix(path, `\\?\`) && !strings.HasPrefix(path, `\\.\`) {
		if strings.HasPrefix(path, `\\`) {
			path = `\\?\UNC\` + path[2:]
		} else {
			path = `\\?\` + path
		}
	}
	return windows.UTF16PtrFromString(path)
}
