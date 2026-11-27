//go:build !cgo

package fileprivacy

import "errors"

func ClearDirectoryACL(int) error {
	return errors.New("private staging requires macOS ACL support")
}
