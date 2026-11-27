//go:build !unix && !windows

package sqlite

import (
	"errors"
	"os"
)

func linkRollbackFile(*os.Root, string, *os.Root, string) error {
	return errors.ErrUnsupported
}
