//go:build !unix

package licensed

import "os"

const regularReadFlags = os.O_RDONLY
