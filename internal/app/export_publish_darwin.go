package app

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func publishExport(staged, target string) error {
	err := unix.RenameatxNp(unix.AT_FDCWD, staged, unix.AT_FDCWD, target, unix.RENAME_EXCL)
	if errors.Is(err, unix.ENOTSUP) {
		if linkErr := os.Link(staged, target); linkErr != nil {
			return errors.Join(err, linkErr)
		}
		return nil
	}
	return err
}
