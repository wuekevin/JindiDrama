//go:build !cgo

package fileprivacy

import "errors"

func ClearDirectoryACL(int) error {
	return errors.New("private staging requires macOS ACL support")
}

func WriteGeneration(int) (uint32, error) {
	return 0, errors.ErrUnsupported
}
