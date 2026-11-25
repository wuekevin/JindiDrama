//go:build !darwin && !windows

package app

import (
	"os"
	"path/filepath"
)

// The desktop update selector supports Windows and macOS. Keep the storage
// path available on hosts that run the platform-independent Go tests.
func createUpdateStage(parent string) (*stagedUpdate, error) {
	dir, err := os.MkdirTemp(parent, ".jindidrama-update-")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "package")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		_ = os.Remove(dir)
		return nil, err
	}
	return &stagedUpdate{
		file:    file,
		publish: func(target string) error { return os.Link(path, target) },
		cleanup: func() { _ = file.Close(); _ = os.Remove(path); _ = os.Remove(dir) },
	}, nil
}
