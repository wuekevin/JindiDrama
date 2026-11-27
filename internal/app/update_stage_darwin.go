package app

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

func createOutputStage(parent, prefix string, kind outputStageKind) (*stagedOutput, error) {
	destFD, err := unix.Open(parent, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	destinationMatches := func() bool {
		var original, selected unix.Stat_t
		return unix.Fstat(destFD, &original) == nil && unix.Stat(parent, &selected) == nil &&
			original.Dev == selected.Dev && original.Ino == selected.Ino
	}
	copyDirect := func(target string, write func(io.Writer) error) error {
		return publishInUpdateDestination(destinationMatches, func() error {
			fd, err := unix.Openat(destFD, filepath.Base(target), unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
			if err != nil {
				return err
			}
			return completeDirectExport(os.NewFile(uintptr(fd), target), write)
		})
	}
	dirName := prefix + rand.Text()
	if err := unix.Mkdirat(destFD, dirName, 0700); err != nil {
		_ = unix.Close(destFD)
		return nil, err
	}
	dirFD, err := unix.Openat(destFD, dirName, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		_ = unix.Close(destFD)
		return nil, err
	}
	closeStageDirectory := func() error {
		// Unlink only the original, empty directory entry. A renamed stage
		// may belong to a shared directory managed by another user.
		var original, current unix.Stat_t
		err := unix.Fstat(dirFD, &original)
		if err == nil {
			err = unix.Fstatat(destFD, dirName, &current, unix.AT_SYMLINK_NOFOLLOW)
		}
		if err == nil {
			if original.Dev == current.Dev && original.Ino == current.Ino {
				err = unix.Unlinkat(destFD, dirName, unix.AT_REMOVEDIR)
			} else {
				err = errors.New("staging directory was replaced")
			}
		}
		return errors.Join(err, unix.Close(dirFD))
	}
	closeDirectory := func() error {
		return errors.Join(closeStageDirectory(), unix.Close(destFD))
	}
	var info unix.Stat_t
	if err := unix.Fstat(dirFD, &info); err != nil {
		closeDirectory()
		return nil, err
	}
	if info.Uid != uint32(os.Geteuid()) || info.Mode&0077 != 0 {
		if kind == mediaOutputStage {
			// A volume without private directory permissions can still create a
			// demo's final file exclusively inside the opened destination.
			if err := closeStageDirectory(); err != nil {
				_ = unix.Close(destFD)
				return nil, err
			}
			return &stagedOutput{
				publish:    func(string) error { return errors.ErrUnsupported },
				cleanup:    func() error { return unix.Close(destFD) },
				copyDirect: copyDirect,
			}, nil
		}
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
	discarded := false
	discard := func() error {
		if discarded {
			return nil
		}
		discarded = true
		removeErr := unix.Unlinkat(dirFD, name, 0)
		if errors.Is(removeErr, unix.ENOENT) {
			removeErr = nil // Successful publication already moved the staging file.
		}
		return errors.Join(removeErr, file.Close())
	}
	cleanup := func() error {
		return errors.Join(discard(), closeDirectory())
	}
	if kind == updateOutputStage {
		quarantine := fmt.Appendf(nil, "0081;%x;JindiDrama;", time.Now().Unix())
		if err := unix.Fsetxattr(fd, "com.apple.quarantine", quarantine, 0); err != nil {
			cleanup()
			return nil, errUpdateOrigin
		}
	}
	return &stagedOutput{
		file: file,
		publish: func(target string) error {
			return publishInUpdateDestination(destinationMatches, func() error {
				destName := filepath.Base(target)
				err := unix.RenameatxNp(dirFD, name, destFD, destName, unix.RENAME_EXCL)
				if errors.Is(err, unix.ENOTSUP) {
					linkErr := unix.Linkat(dirFD, name, destFD, destName, 0)
					if linkErr != nil && kind == mediaOutputStage {
						return errors.Join(err, linkErr)
					}
					return linkErr
				}
				return err
			})
		},
		cleanup: cleanup,
		copyDirect: func(target string, write func(io.Writer) error) error {
			if err := discard(); err != nil {
				return err
			}
			return copyDirect(target, write)
		},
	}, nil
}
