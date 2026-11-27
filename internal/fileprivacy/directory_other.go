//go:build !darwin && !windows

package fileprivacy

import (
	"errors"
	"os"
)

// ProtectDirectory checks the permissions on an empty staging root.
func ProtectDirectory(root *os.Root) error {
	info, err := root.Stat(".")
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("staging directory is not private")
	}
	return nil
}
