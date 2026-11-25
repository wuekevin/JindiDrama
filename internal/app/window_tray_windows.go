package app

import (
	"context"
	goruntime "runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	windowTrayAvailable = true
	trayCallback        = 0x8001 // WM_APP + 1
	trayTimer           = 1
	wailsIconResource   = 3
	wmClose             = 0x0010
	wmTimer             = 0x0113
	wmContextMenu       = 0x007b
	wmLeftButtonUp      = 0x0202
	ninSelect           = 0x0400
	ninKeySelect        = 0x0401
)

var (
	trayUser32          = windows.NewLazySystemDLL("user32.dll")
	trayShell32         = windows.NewLazySystemDLL("shell32.dll")
	trayShell           = trayShell32.NewProc("Shell_NotifyIconW")
	trayIconRect        = trayShell32.NewProc("Shell_NotifyIconGetRect")
	trayCursorPosition  = trayUser32.NewProc("GetCursorPos")
	trayRegister        = trayUser32.NewProc("RegisterClassExW")
	trayUnregister      = trayUser32.NewProc("UnregisterClassW")
	trayCreate          = trayUser32.NewProc("CreateWindowExW")
	trayDestroy         = trayUser32.NewProc("DestroyWindow")
	trayDefault         = trayUser32.NewProc("DefWindowProcW")
	trayGetMsg          = trayUser32.NewProc("GetMessageW")
	trayDispatch        = trayUser32.NewProc("DispatchMessageW")
	trayPostMsg         = trayUser32.NewProc("PostMessageW")
	trayPostQuit        = trayUser32.NewProc("PostQuitMessage")
	trayLoadIcon        = trayUser32.NewProc("LoadIconW")
	trayFind            = trayUser32.NewProc("FindWindowExW")
	trayVisible         = trayUser32.NewProc("IsWindowVisible")
	trayShowWindow      = trayUser32.NewProc("ShowWindow")
	traySetTimer        = trayUser32.NewProc("SetTimer")
	trayKillTimer       = trayUser32.NewProc("KillTimer")
	trayRegisterMessage = trayUser32.NewProc("RegisterWindowMessageW")
	trayCreateMenu      = trayUser32.NewProc("CreatePopupMenu")
	trayAppendMenu      = trayUser32.NewProc("AppendMenuW")
	trayDestroyMenu     = trayUser32.NewProc("DestroyMenu")
	trayForeground      = trayUser32.NewProc("SetForegroundWindow")
	trayTrackMenu       = trayUser32.NewProc("TrackPopupMenu")
	trayEndMenu         = trayUser32.NewProc("EndMenu")

	trayProc      = windows.NewCallback(windowsTrayProc)
	trayRequestMu sync.Mutex
	trayMu        sync.Mutex
	trayClosing   bool
	activeTray    *notificationTray
	// Only the tray's locked OS thread reads and writes this pointer.
	dispatchTray *notificationTray
)

type trayWindowClass struct {
	size, style                        uint32
	proc                               uintptr
	classExtra, windowExtra            int32
	instance, icon, cursor, background uintptr
	menu, name                         *uint16
	smallIcon                          uintptr
}

type trayMessage struct {
	window         uintptr
	message        uint32
	wparam, lparam uintptr
	time           uint32
	x, y           int32
	private        uint32
}

type trayIconData struct {
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
}

type trayIconIdentifier struct {
	size   uint32
	window uintptr
	id     uint32
	guid   windows.GUID
}

type trayPoint struct{ x, y int32 }
type trayRect struct{ left, top, right, bottom int32 }

type notificationTray struct {
	ctx                          context.Context
	onQuit                       func()
	main, window, taskbarCreated uintptr
	icon                         trayIconData
	done                         chan struct{}
	stopRequested                atomic.Bool
}

func hideWindowToTray(ctx context.Context, onQuit func()) bool {
	// An unelevated Explorer cannot send tray callbacks to an elevated app.
	if windows.GetCurrentProcessToken().IsElevated() {
		return false
	}
	trayRequestMu.Lock()
	defer trayRequestMu.Unlock()

	trayMu.Lock()
	previous, closing := activeTray, trayClosing
	trayMu.Unlock()
	if closing {
		return false
	}
	if previous != nil {
		select {
		case <-previous.done:
		default:
			// A just-restored window can be closed before its icon is removed.
			visible, _, _ := trayVisible.Call(previous.main)
			if visible == 0 && !previous.stopRequested.Load() {
				return true
			}
			if !previous.stop() {
				return false
			}
		}
	}
	main := findWindowsMainWindow()
	if main == 0 {
		return false
	}
	tray := &notificationTray{ctx: ctx, onQuit: onQuit, main: main, done: make(chan struct{})}
	trayMu.Lock()
	if trayClosing {
		trayMu.Unlock()
		return false
	}
	activeTray = tray
	trayMu.Unlock()

	ready := make(chan bool, 1)
	go tray.run(ready)
	if !<-ready {
		<-tray.done
		return false
	}
	return true
}

func closeWindowTray() {
	trayMu.Lock()
	trayClosing = true
	tray := activeTray
	trayMu.Unlock()
	if tray != nil {
		tray.stop()
	}
}

func (t *notificationTray) stop() bool {
	t.stopRequested.Store(true)
	select {
	case <-t.done:
		return true
	case <-time.After(time.Second):
		return false
	}
}

func findWindowsMainWindow() uintptr {
	className := windows.StringToUTF16Ptr(WindowsMainWindowClass)
	pid := windows.GetCurrentProcessId()
	var previous uintptr
	for {
		window, _, _ := trayFind.Call(0, previous, uintptr(unsafe.Pointer(className)), 0)
		if window == 0 {
			return 0
		}
		var owner uint32
		_, _ = windows.GetWindowThreadProcessId(windows.HWND(window), &owner)
		if owner == pid {
			return window
		}
		previous = window
	}
}

func (t *notificationTray) run(ready chan<- bool) {
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()
	defer close(t.done)
	started := false
	defer func() {
		if !started {
			ready <- false
		}
	}()

	var instance windows.Handle
	if windows.GetModuleHandleEx(windows.GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT, nil, &instance) != nil {
		return
	}
	name := windows.StringToUTF16Ptr("JindiDramaNotificationTray")
	class := trayWindowClass{proc: trayProc, instance: uintptr(instance), name: name}
	class.size = uint32(unsafe.Sizeof(class))
	atom, _, _ := trayRegister.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return
	}
	defer trayUnregister.Call(uintptr(unsafe.Pointer(name)), uintptr(instance))

	// A hidden top-level window receives Explorer restart broadcasts.
	dispatchTray = t
	defer func() { dispatchTray = nil }()
	t.window, _, _ = trayCreate.Call(0, uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 0, 0, 0, 0, uintptr(instance), 0)
	if t.window == 0 {
		return
	}
	defer trayDestroy.Call(t.window)
	messageName := windows.StringToUTF16Ptr("TaskbarCreated")
	t.taskbarCreated, _, _ = trayRegisterMessage.Call(uintptr(unsafe.Pointer(messageName)))
	if t.taskbarCreated == 0 {
		return
	}

	icon, _, _ := trayLoadIcon.Call(uintptr(instance), wailsIconResource)
	if icon == 0 {
		icon, _, _ = trayLoadIcon.Call(0, 32512) // IDI_APPLICATION
	}
	t.icon = trayIconData{window: t.window, id: 1, flags: 0x87, callback: trayCallback, icon: icon, version: 4}
	t.icon.size = uint32(unsafe.Sizeof(t.icon))
	copy(t.icon.tip[:], windows.StringToUTF16("金帝短剧"))
	if icon == 0 || !t.addIcon() {
		return
	}
	defer t.notify(2) // NIM_DELETE
	timer, _, _ := traySetTimer.Call(t.window, trayTimer, 150, 0)
	if timer == 0 {
		return
	}
	defer trayKillTimer.Call(t.window, timer)
	if t.stopRequested.Load() {
		return
	}
	// Wails dispatches Go bindings off the WebView thread. Complete the
	// hide before servicing tray events so a restore cannot overtake it.
	trayShowWindow.Call(t.main, 0) // SW_HIDE
	if visible, _, _ := trayVisible.Call(t.main); visible != 0 {
		return
	}
	started = true
	ready <- true
	var message trayMessage
	for {
		result, _, _ := trayGetMsg.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) < 0 {
			t.restore()
		}
		if int32(result) <= 0 {
			return
		}
		trayDispatch.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func (t *notificationTray) notify(action uintptr) bool {
	ok, _, _ := trayShell.Call(action, uintptr(unsafe.Pointer(&t.icon)))
	return ok != 0
}

func (t *notificationTray) addIcon() bool {
	if !t.notify(0) { // NIM_ADD
		return false
	}
	if !t.notify(4) { // NIM_SETVERSION
		t.notify(2)
		return false
	}
	return true
}

func windowsTrayProc(window uintptr, message uint32, wparam, lparam uintptr) uintptr {
	t := dispatchTray
	if t != nil && window == t.window {
		switch {
		case message == wmClose:
			if !t.stopRequested.Load() {
				t.restore()
			}
			trayEndMenu.Call()
			trayPostQuit.Call(0)
			return 0
		case uintptr(message) == t.taskbarCreated:
			if !t.addIcon() {
				t.restore()
			}
			return 0
		case message == wmTimer && wparam == trayTimer:
			if visible, _, _ := trayVisible.Call(t.main); visible != 0 || t.stopRequested.Load() {
				trayEndMenu.Call()
				trayPostQuit.Call(0)
			}
			return 0
		case message == trayCallback && uint16(lparam>>16) == uint16(t.icon.id):
			switch uint16(lparam) {
			case wmContextMenu:
				t.openMenu()
			case wmLeftButtonUp, ninSelect, ninKeySelect:
				t.restore()
			}
			return 0
		}
	}
	result, _, _ := trayDefault.Call(window, uintptr(message), wparam, lparam)
	return result
}

func (t *notificationTray) restore() {
	go runtime.WindowShow(t.ctx)
}

func (t *notificationTray) openMenu() {
	menu, _, _ := trayCreateMenu.Call()
	if menu == 0 {
		return
	}
	defer trayDestroyMenu.Call(menu)
	restore := windows.StringToUTF16Ptr("显示窗口")
	quit := windows.StringToUTF16Ptr("退出金帝短剧")
	first, _, _ := trayAppendMenu.Call(menu, 0, 1, uintptr(unsafe.Pointer(restore)))
	second, _, _ := trayAppendMenu.Call(menu, 0, 2, uintptr(unsafe.Pointer(quit)))
	if first == 0 || second == 0 {
		return
	}
	position, ok := t.menuPosition()
	if !ok {
		return
	}
	trayForeground.Call(t.window)
	command, _, _ := trayTrackMenu.Call(menu, 0x182, uintptr(position.x), uintptr(position.y), 0, t.window, 0) // TPM_RETURNCMD | TPM_NONOTIFY | TPM_RIGHTBUTTON
	trayPostMsg.Call(t.window, 0, 0, 0)
	switch command {
	case 1:
		t.restore()
	case 2:
		go t.onQuit()
	default:
		t.notify(3) // NIM_SETFOCUS
	}
}

func (t *notificationTray) menuPosition() (trayPoint, bool) {
	identifier := trayIconIdentifier{window: t.window, id: t.icon.id}
	identifier.size = uint32(unsafe.Sizeof(identifier))
	var rect trayRect
	result, _, _ := trayIconRect.Call(uintptr(unsafe.Pointer(&identifier)), uintptr(unsafe.Pointer(&rect)))
	if int32(result) >= 0 {
		return trayPoint{rect.left, rect.bottom}, true
	}
	var position trayPoint
	ok, _, _ := trayCursorPosition.Call(uintptr(unsafe.Pointer(&position)))
	return position, ok != 0
}
