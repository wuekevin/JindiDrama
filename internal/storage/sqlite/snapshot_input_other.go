//go:build !unix

package sqlite

import "os"

const snapshotReadFlags = os.O_RDONLY
