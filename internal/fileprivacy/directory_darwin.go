package fileprivacy

import (
	"errors"
	"os"
	"syscall"
)

// ProtectDirectory removes inherited ACLs before data is created under root.
func ProtectDirectory(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	info, err := dir.Stat()
	if err != nil {
		return err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uint32(os.Geteuid()) || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("staging directory is not private")
	}
	return ClearDirectoryACL(int(dir.Fd()))
}
