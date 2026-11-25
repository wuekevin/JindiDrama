package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	"golang.org/x/sys/windows"
	"jindi-drama/internal/app"
)

func prepareShareLaunch(links *shareLinks) (*options.SingleInstanceLock, func()) {
	directory, _ := app.DataDir()
	id := sha256.Sum256([]byte(strings.ToLower(directory)))
	return &options.SingleInstanceLock{
		UniqueId: fmt.Sprintf("jindidrama-%x", id[:12]),
		OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
			if len(data.Args) == 0 {
				links.revealWindow()
				return
			}
			links.openArgs(data.Args)
		},
	}, acquireShareLaunchGate(directory)
}

// Wails creates its process mutex before its message window. Serializing startup
// closes that gap. The kernel also releases the file lock if a process exits.
func acquireShareLaunchGate(directory string) func() {
	if directory == "" || os.MkdirAll(directory, 0o700) != nil {
		return func() {}
	}
	file, err := os.OpenFile(filepath.Join(directory, "share-launch.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}
	}
	deadline := time.Now().Add(8 * time.Second)
	for {
		err := windows.LockFileEx(windows.Handle(file.Fd()),
			windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
		if err == nil {
			var once sync.Once
			return func() { once.Do(func() { _ = file.Close() }) }
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) || time.Now().After(deadline) {
			_ = file.Close()
			return func() {}
		}
		time.Sleep(50 * time.Millisecond)
	}
}
