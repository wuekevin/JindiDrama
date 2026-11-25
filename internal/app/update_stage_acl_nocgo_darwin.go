//go:build !cgo

package app

import "errors"

func clearUpdateStageACL(int) error {
	return errors.New("update staging requires macOS ACL support")
}
