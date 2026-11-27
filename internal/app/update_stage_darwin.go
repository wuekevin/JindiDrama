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

	"jindi-drama/internal/fileprivacy"
)

func createOutputStage(parent, prefix string, kind outputStageKind) (*stagedOutput, error) {
	destFD, err := unix.Open(parent, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	var filesystem string
	if kind == updateOutputStage {
		var volume unix.Statfs_t
		if err := unix.Fstatfs(destFD, &volume); err != nil {
			_ = unix.Close(destFD)
			return nil, err
		}
		filesystem = unix.ByteSliceToString(volume.Fstypename[:])
	}
	cloneUpdate := filesystem == "apfs"
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
		// Without an open handle, the entry may have been replaced.
		return nil, outputStageSetupError(kind, err, err)
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
		return nil, outputStageSetupError(kind, err, closeDirectory())
	}
	if info.Uid != uint32(os.Geteuid()) || info.Mode&0077 != 0 {
		if kind == mediaOutputStage {
			// A volume without private directory permissions can still create a
			// media file exclusively inside the opened destination.
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
		return nil, outputStageSetupError(kind, errors.New("update staging directory is not private"), closeDirectory())
	}
	if err := fileprivacy.ClearDirectoryACL(dirFD); err != nil {
		return nil, outputStageSetupError(kind, err, closeDirectory())
	}
	const name = "package"
	flags := unix.O_WRONLY
	if kind == privateBackupOutputStage || kind == updateOutputStage {
		flags = unix.O_RDWR
	}
	fd, err := unix.Openat(dirFD, name, flags|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, outputStageSetupError(kind, err, closeDirectory())
	}
	file := os.NewFile(uintptr(fd), filepath.Join(parent, dirName, name))
	discarded := false
	stageEntryRemoved := false
	discard := func() error {
		if discarded {
			return nil
		}
		discarded = true
		if stageEntryRemoved {
			return file.Close()
		}
		if kind == updateOutputStage {
			if err := checkUpdateFileIdentity(fd, dirFD, name); err != nil {
				return errors.Join(err, file.Close())
			}
		}
		removeErr := unix.Unlinkat(dirFD, name, 0)
		if errors.Is(removeErr, unix.ENOENT) {
			removeErr = nil // Successful publication already moved the staging file.
		}
		return errors.Join(removeErr, file.Close())
	}
	cleanup := func() error {
		return errors.Join(discard(), closeDirectory())
	}
	if cloneUpdate {
		// Keep the download accessible only through this descriptor.
		if err := checkUpdateFileIdentity(fd, dirFD, name); err != nil {
			return nil, outputStageSetupError(kind, err, cleanup())
		}
		if err := unix.Unlinkat(dirFD, name, 0); err != nil {
			return nil, outputStageSetupError(kind, err, cleanup())
		}
		stageEntryRemoved = true
		var opened unix.Stat_t
		if unix.Fstat(fd, &opened) != nil || opened.Nlink != 0 {
			return nil, outputStageSetupError(kind, errUpdateStageIdentity, cleanup())
		}
	}
	if kind == updateOutputStage {
		quarantine := fmt.Appendf(nil, "0081;%x;JindiDrama;", time.Now().Unix())
		if err := unix.Fsetxattr(fd, "com.apple.quarantine", quarantine, 0); err != nil {
			return nil, outputStageSetupError(kind, errUpdateOrigin, cleanup())
		}
	}
	var checkPublishedUpdate func(string, func() error) error
	if kind == updateOutputStage && !cloneUpdate {
		checkPublishedUpdate = func(target string, verify func() error) error {
			destName := filepath.Base(target)
			var selectedMatches bool
			var generation uint32
			err := verifyDarwinUpdateReadback(fd, func() error {
				var before, after unix.Stat_t
				if unix.Fstat(fd, &before) != nil || checkUpdateFileIdentity(fd, destFD, destName) != nil {
					return errUpdateStageIdentity
				}
				if filesystem == "hfs" {
					value, err := fileprivacy.WriteGeneration(fd)
					if err != nil || value == 0 {
						return errUpdateStageIdentity
					}
					generation = value
				}
				if err := verify(); err != nil {
					return err
				}
				if unix.Fstat(fd, &after) != nil || before.Size != after.Size ||
					before.Mtim != after.Mtim || before.Ctim != after.Ctim ||
					checkUpdateFileIdentity(fd, destFD, destName) != nil {
					return errUpdateStageIdentity
				}
				selectedMatches = destinationMatches()
				return nil
			})
			if err != nil {
				return err
			}
			// Check HFS+ mapped writes after the final vnode event poll.
			if filesystem == "hfs" {
				value, err := fileprivacy.WriteGeneration(fd)
				if err != nil || value != generation {
					return errUpdateStageIdentity
				}
			}
			if !selectedMatches {
				return errUpdateDestinationMovedAfterPublish
			}
			return nil
		}
	}
	return &stagedOutput{
		file: file,
		publish: func(target string) error {
			return publishInUpdateDestination(destinationMatches, func() error {
				destName := filepath.Base(target)
				if cloneUpdate {
					return unix.Fclonefileat(fd, destFD, destName, unix.CLONE_NOOWNERCOPY)
				}
				if kind == updateOutputStage {
					if err := checkUpdateFileIdentity(fd, dirFD, name); err != nil {
						return err
					}
				}
				err := unix.RenameatxNp(dirFD, name, destFD, destName, unix.RENAME_EXCL)
				if err == nil {
					stageEntryRemoved = true
				} else if errors.Is(err, unix.ENOTSUP) {
					linkErr := unix.Linkat(dirFD, name, destFD, destName, 0)
					if linkErr != nil && kind == mediaOutputStage {
						return errors.Join(err, linkErr)
					}
					err = linkErr
				}
				if err == nil && kind == updateOutputStage {
					if err := checkUpdateFileIdentity(fd, destFD, destName); err != nil {
						return errUpdatePublishedIdentity
					}
				}
				return err
			})
		},
		cleanup:              cleanup,
		checkPublishedUpdate: checkPublishedUpdate,
		copyDirect: func(target string, write func(io.Writer) error) error {
			if err := discard(); err != nil {
				return err
			}
			return copyDirect(target, write)
		},
	}, nil
}

func checkUpdateFileIdentity(fd, dirFD int, name string) error {
	var opened, entry unix.Stat_t
	if unix.Fstat(fd, &opened) != nil ||
		unix.Fstatat(dirFD, name, &entry, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		opened.Dev != entry.Dev || opened.Ino != entry.Ino || entry.Mode&unix.S_IFMT != unix.S_IFREG {
		return errUpdateStageIdentity
	}
	return nil
}

// HFS+ timestamps can have one-second precision, so also watch for writes
// while the open file is being checked.
func verifyDarwinUpdateReadback(fd int, verify func() error) error {
	queue, err := unix.Kqueue()
	if err != nil {
		return errUpdateStageIdentity
	}
	defer unix.Close(queue)
	unix.CloseOnExec(queue)
	var event unix.Kevent_t
	unix.SetKevent(&event, fd, unix.EVFILT_VNODE, unix.EV_ADD|unix.EV_CLEAR)
	event.Fflags = unix.NOTE_WRITE | unix.NOTE_EXTEND | unix.NOTE_DELETE | unix.NOTE_RENAME | unix.NOTE_REVOKE
	timeout := unix.Timespec{}
	if _, err := unix.Kevent(queue, []unix.Kevent_t{event}, nil, &timeout); err != nil {
		return errUpdateStageIdentity
	}
	if err := verify(); err != nil {
		return err
	}
	if n, err := unix.Kevent(queue, nil, []unix.Kevent_t{event}, &timeout); err != nil || n != 0 {
		return errUpdateStageIdentity
	}
	return nil
}
