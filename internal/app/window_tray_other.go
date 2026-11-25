//go:build !windows && (!darwin || !cgo)

package app

import "context"

const windowTrayAvailable = false

func hideWindowToTray(context.Context, func()) bool { return false }

func closeWindowTray() {}
