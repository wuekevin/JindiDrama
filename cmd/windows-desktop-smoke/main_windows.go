// windows-desktop-smoke probes isolated packaging test applications through Win32.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"

	"jindi-drama/internal/app"
)

const (
	trayClass           = "JindiDramaNotificationTray"
	trayNotify          = 0x8001
	ninSelectByKeyboard = 0x0401
	trayIconID          = 1
	guiPopupMenuMode    = 0x10
)

var (
	user32      = windows.NewLazySystemDLL("user32.dll")
	findWindow  = user32.NewProc("FindWindowExW")
	isVisible   = user32.NewProc("IsWindowVisible")
	isIconic    = user32.NewProc("IsIconic")
	postMessage = user32.NewProc("PostMessageW")
	guiThread   = user32.NewProc("GetGUIThreadInfo")
)

type windowState struct {
	Main      uintptr
	Tray      uintptr
	Visible   bool
	Minimized bool
	TrayMenu  bool
}

type guiThreadInfo struct {
	size, flags                                        uint32
	active, focus, capture, menuOwner, moveSize, caret uintptr
	caretRect                                          [4]int32
}

func runNative(args []string) error {
	if len(args) == 1 && args[0] == "notification-owner" {
		window := windowByClass("Shell_TrayWnd", 0)
		var pid uint32
		if window != 0 {
			_, _ = windows.GetWindowThreadProcessId(windows.HWND(window), &pid)
		}
		return json.NewEncoder(os.Stdout).Encode(pid)
	}
	if len(args) == 1 && args[0] == "notification-available" {
		ok, err := probeNotificationArea()
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(ok)
	}
	if len(args) != 2 {
		return errors.New("usage: windows-desktop-smoke seed <fixture database> <Minimize|Tray>, notification-available, notification-owner, inspect <PID>, or activate-tray <PID>")
	}
	pid, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil || pid == 0 {
		return errors.New("invalid application process id")
	}
	state := inspect(uint32(pid))
	switch args[0] {
	case "inspect":
		return json.NewEncoder(os.Stdout).Encode(state)
	case "activate-tray":
		if state.Main == 0 || state.Visible || state.Tray == 0 {
			return errors.New("expected an owned tray window and a hidden main window")
		}
		if result, _, err := postMessage.Call(state.Tray, trayNotify, 0, trayIconID<<16|ninSelectByKeyboard); result == 0 {
			return fmt.Errorf("send notification area keyboard callback: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func inspect(pid uint32) windowState {
	state := windowState{
		Main: windowByClass(app.WindowsMainWindowClass, pid),
		Tray: windowByClass(trayClass, pid),
	}
	if state.Main != 0 {
		visible, _, _ := isVisible.Call(state.Main)
		minimized, _, _ := isIconic.Call(state.Main)
		state.Visible = visible != 0
		state.Minimized = minimized != 0
	}
	state.TrayMenu = trayMenuOpen(state.Tray)
	return state
}

func trayMenuOpen(tray uintptr) bool {
	if tray == 0 {
		return false
	}
	thread, err := windows.GetWindowThreadProcessId(windows.HWND(tray), nil)
	if err != nil || thread == 0 {
		return false
	}
	var info guiThreadInfo
	info.size = uint32(unsafe.Sizeof(info))
	read, _, _ := guiThread.Call(uintptr(thread), uintptr(unsafe.Pointer(&info)))
	return read != 0 && info.flags&guiPopupMenuMode != 0 && info.menuOwner == tray
}

func windowByClass(class string, pid uint32) uintptr {
	name := windows.StringToUTF16Ptr(class)
	var previous uintptr
	for {
		window, _, _ := findWindow.Call(0, previous, uintptr(unsafe.Pointer(name)), 0)
		if window == 0 || pid == 0 {
			return window
		}
		var owner uint32
		_, _ = windows.GetWindowThreadProcessId(windows.HWND(window), &owner)
		if owner == pid {
			return window
		}
		previous = window
	}
}

// probeNotificationArea tries the shell operation with a disposable window on
// this process's own thread. The presence of Explorer alone proves too little.
func probeNotificationArea() (bool, error) {
	if windows.GetCurrentProcessToken().IsElevated() || windowByClass("Shell_TrayWnd", 0) == 0 {
		return false, nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	class := windows.StringToUTF16Ptr("STATIC")
	window, _, err := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), 0, 0x80000000, 0, 0, 0, 0, 0, 0, 0, 0)
	if window == 0 {
		return false, fmt.Errorf("create notification probe window: %w", err)
	}
	defer user32.NewProc("DestroyWindow").Call(window)

	icon, _, err := user32.NewProc("LoadIconW").Call(0, 32512) // IDI_APPLICATION
	if icon == 0 {
		return false, fmt.Errorf("load notification probe icon: %w", err)
	}
	var data = struct {
		size                uint32
		window              uintptr
		id, flags, callback uint32
		icon                uintptr
		tip                 [128]uint16
		state, stateMask    uint32
		info                [256]uint16
		version             uint32
		title               [64]uint16
		infoFlags           uint32
		guid                windows.GUID
		balloonIcon         uintptr
	}{
		window: window, id: 1, flags: 2, icon: icon, version: 4, // NIF_ICON, NOTIFYICON_VERSION_4
	}
	data.size = uint32(unsafe.Sizeof(data))
	notify := windows.NewLazySystemDLL("shell32.dll").NewProc("Shell_NotifyIconW")
	added, _, _ := notify.Call(0, uintptr(unsafe.Pointer(&data))) // NIM_ADD
	if added == 0 {
		return false, nil
	}
	versioned, _, _ := notify.Call(4, uintptr(unsafe.Pointer(&data))) // NIM_SETVERSION
	deleted, _, _ := notify.Call(2, uintptr(unsafe.Pointer(&data)))   // NIM_DELETE
	if deleted == 0 {
		return false, errors.New("unable to remove the notification probe icon")
	}
	return versioned != 0, nil
}
