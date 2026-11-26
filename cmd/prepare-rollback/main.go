// prepare-rollback creates a database copy that an older desktop build can read.
package main

import (
	"os"

	"jindi-drama/internal/rollbackcli"
)

func main() {
	os.Exit(rollbackcli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
