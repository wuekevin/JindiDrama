//go:build !windows

package main

import "github.com/wailsapp/wails/v2/pkg/options"

func prepareShareLaunch(*shareLinks) (*options.SingleInstanceLock, func()) {
	return nil, func() {}
}
