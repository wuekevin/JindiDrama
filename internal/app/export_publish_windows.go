package app

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func publishExport(staged, target string) error {
	from, err := exportWindowsPath(staged)
	if err != nil {
		return err
	}
	to, err := exportWindowsPath(target)
	if err != nil {
		return err
	}
	// Zero flags disallow replacement and cross-volume copy.
	return windows.MoveFileEx(from, to, 0)
}

func exportWindowsPath(path string) (*uint16, error) {
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
