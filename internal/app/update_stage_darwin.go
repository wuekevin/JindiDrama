package app

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

func createUpdateStage(parent string) (*stagedUpdate, error) {
	destFD, err := unix.Open(parent, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	dirName := ".jindidrama-update-" + rand.Text()
	if err := unix.Mkdirat(destFD, dirName, 0700); err != nil {
		_ = unix.Close(destFD)
		return nil, err
	}
	dirFD, err := unix.Openat(destFD, dirName, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		_ = unix.Close(destFD)
		return nil, err
	}
	closeDirectory := func() {
		// Unlink only the original, empty directory entry. A renamed stage
		// may belong to a shared directory managed by another user.
		var original, current unix.Stat_t
		if unix.Fstat(dirFD, &original) == nil &&
			unix.Fstatat(destFD, dirName, &current, unix.AT_SYMLINK_NOFOLLOW) == nil &&
			original.Dev == current.Dev && original.Ino == current.Ino {
			_ = unix.Unlinkat(destFD, dirName, unix.AT_REMOVEDIR)
		}
		_ = unix.Close(dirFD)
		_ = unix.Close(destFD)
	}
	var info unix.Stat_t
	if err := unix.Fstat(dirFD, &info); err != nil || info.Uid != uint32(os.Geteuid()) || info.Mode&0077 != 0 {
		closeDirectory()
		return nil, errors.New("update staging directory is not private")
	}
	if err := clearUpdateStageACL(dirFD); err != nil {
		closeDirectory()
		return nil, err
	}
	const name = "package"
	fd, err := unix.Openat(dirFD, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		closeDirectory()
		return nil, err
	}
	file := os.NewFile(uintptr(fd), filepath.Join(parent, dirName, name))
	cleanup := func() {
		_ = file.Close()
		_ = unix.Unlinkat(dirFD, name, 0)
		closeDirectory()
	}
	quarantine := fmt.Appendf(nil, "0081;%x;JindiDrama;", time.Now().Unix())
	if err := unix.Fsetxattr(fd, "com.apple.quarantine", quarantine, 0); err != nil {
		cleanup()
		return nil, errUpdateOrigin
	}
	destinationMatches := func() bool {
		var original, selected unix.Stat_t
		return unix.Fstat(destFD, &original) == nil && unix.Stat(parent, &selected) == nil &&
			original.Dev == selected.Dev && original.Ino == selected.Ino
	}
	return &stagedUpdate{
		file: file,
		publish: func(target string) error {
			return publishInUpdateDestination(destinationMatches, func() error {
				destName := filepath.Base(target)
				err := unix.RenameatxNp(dirFD, name, destFD, destName, unix.RENAME_EXCL)
				if errors.Is(err, unix.ENOTSUP) {
					return unix.Linkat(dirFD, name, destFD, destName, 0)
				}
				return err
			})
		},
		cleanup: cleanup,
	}, nil
}

func publishInUpdateDestination(matches func() bool, publish func() error) error {
	if !matches() {
		return errUpdateDestinationMoved
	}
	if err := publish(); err != nil {
		return err
	}
	// The descriptor pins publication to the original directory. If its
	// path changed during the rename, tell the caller where the file went.
	if !matches() {
		return errUpdateDestinationMovedAfterPublish
	}
	return nil
}
