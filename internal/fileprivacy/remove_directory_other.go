//go:build !unix && !windows

package fileprivacy

import (
	"errors"
	"os"
)

// RemoveEmptyDirectory removes an empty directory without deleting a regular file.
func RemoveEmptyDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("staging directory changed")
	}
	return os.Remove(path)
}
