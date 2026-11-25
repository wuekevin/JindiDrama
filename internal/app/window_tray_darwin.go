package app

/*
#cgo LDFLAGS: -framework AppKit
#include <stdbool.h>
#include <stdint.h>
bool JindiHideWindowToStatusItem(uintptr_t handle);
*/
import "C"

import (
	"context"
	"runtime/cgo"
)

const windowTrayAvailable = true

func hideWindowToTray(_ context.Context, onQuit func()) bool {
	handle := cgo.NewHandle(onQuit)
	if !bool(C.JindiHideWindowToStatusItem(C.uintptr_t(handle))) {
		handle.Delete()
		return false
	}
	return true
}

//export JindiStatusItemAction
func JindiStatusItemAction(value C.uintptr_t, quit C.bool) {
	handle := cgo.Handle(value)
	if bool(quit) {
		go handle.Value().(func())()
		return
	}
	handle.Delete()
}

func closeWindowTray() {}
