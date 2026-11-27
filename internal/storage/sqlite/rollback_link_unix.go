//go:build unix

package sqlite

import (
	"os"

	"golang.org/x/sys/unix"
)

func linkRollbackFile(stage *os.Root, staged string, destination *os.Root, name string) error {
	from, err := stage.Open(".")
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := destination.Open(".")
	if err != nil {
		return err
	}
	defer to.Close()
	return unix.Linkat(int(from.Fd()), staged, int(to.Fd()), name, 0)
}
