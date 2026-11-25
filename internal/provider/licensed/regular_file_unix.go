//go:build unix

package licensed

import (
	"os"
	"syscall"
)

// Opening a pipe for reading must not block before we can reject its file type.
const regularReadFlags = os.O_RDONLY | syscall.O_NONBLOCK
