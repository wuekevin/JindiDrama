//go:build !darwin && !windows

package app

import "os"

func publishExport(staged, target string) error {
	return os.Link(staged, target)
}
