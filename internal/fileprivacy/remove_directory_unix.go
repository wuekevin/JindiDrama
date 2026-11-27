//go:build unix

package fileprivacy

import "syscall"

// RemoveEmptyDirectory removes an empty directory without deleting a regular file.
func RemoveEmptyDirectory(path string) error {
	return syscall.Rmdir(path)
}
