//go:build unix

package sqlite

import (
	"os"
	"syscall"
)

// A regular pathname may be replaced with a FIFO before open.
const snapshotReadFlags = os.O_RDONLY | syscall.O_NONBLOCK
