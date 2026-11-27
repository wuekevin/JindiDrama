//go:build !darwin && !windows

package app

import (
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// The desktop update selector supports Windows and macOS. Keep the storage
// path available on hosts that run the platform-independent Go tests.
func createOutputStage(parent, prefix string, kind outputStageKind) (*stagedOutput, error) {
	dest, err := os.OpenRoot(parent)
	if err != nil {
		return nil, err
	}
	dir := prefix + rand.Text()
	if err := dest.Mkdir(dir, 0o700); err != nil {
		_ = dest.Close()
		return nil, err
	}
	stage, err := dest.OpenRoot(dir)
	if err != nil {
		_ = dest.Remove(dir)
		_ = dest.Close()
		return nil, err
	}
	closeDirectories := func() error {
		original, statErr := stage.Stat(".")
		current, currentErr := dest.Lstat(dir)
		var removeErr error
		if statErr == nil && currentErr == nil && os.SameFile(original, current) {
			removeErr = dest.Remove(dir)
		} else if statErr == nil && currentErr == nil {
			removeErr = errors.New("staging directory was replaced")
		}
		return errors.Join(statErr, currentErr, removeErr, stage.Close(), dest.Close())
	}
	const name = "package"
	access := os.O_WRONLY
	if kind == privateBackupOutputStage {
		access = os.O_RDWR
	}
	file, err := stage.OpenFile(name, access|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		closeDirectories()
		return nil, err
	}
	discarded := false
	discard := func() error {
		if discarded {
			return nil
		}
		discarded = true
		return errors.Join(stage.Remove(name), file.Close())
	}
	matches := func() bool {
		opened, statErr := dest.Stat(".")
		current, currentErr := os.Stat(parent)
		return statErr == nil && currentErr == nil && os.SameFile(opened, current)
	}
	return &stagedOutput{
		file: file,
		publish: func(target string) error {
			return publishInUpdateDestination(matches, func() error {
				from, err := stage.Open(".")
				if err != nil {
					return err
				}
				defer from.Close()
				to, err := dest.Open(".")
				if err != nil {
					return err
				}
				defer to.Close()
				return unix.Linkat(int(from.Fd()), name, int(to.Fd()), filepath.Base(target), 0)
			})
		},
		cleanup: func() error {
			return errors.Join(discard(), closeDirectories())
		},
		copyDirect: func(target string, write func(io.Writer) error) error {
			return publishInUpdateDestination(matches, func() error {
				if err := discard(); err != nil {
					return err
				}
				output, err := dest.OpenFile(filepath.Base(target), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if err != nil {
					return err
				}
				return completeDirectExport(output, write)
			})
		},
	}, nil
}
