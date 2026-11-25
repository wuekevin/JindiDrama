package app

import (
	"context"
	"encoding/json"
	"errors"
	goruntime "runtime"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type windowCloseAction uint32

const (
	closeUnset windowCloseAction = iota
	closeQuit
	closeMinimise
	closeTray
)

func parseWindowCloseAction(value string) windowCloseAction {
	switch value {
	case "最小化到任务栏":
		return closeMinimise
	case "隐藏到菜单栏":
		return closeTray
	default:
		return closeQuit
	}
}

func settingsWindowCloseAction(raw string) windowCloseAction {
	var settings struct {
		CloseAction string `json:"closeAction"`
	}
	if json.Unmarshal([]byte(raw), &settings) != nil {
		return closeQuit
	}
	return parseWindowCloseAction(settings.CloseAction)
}

const windowCloseSettingKey = "window_close_action"

func (a *App) restoreWindowCloseAction() {
	if a.store == nil {
		return
	}
	raw, exists, err := a.store.GetSetting(windowCloseSettingKey)
	if err != nil {
		return
	}
	if exists {
		var value string
		if json.Unmarshal([]byte(raw), &value) == nil {
			a.closePreference.Store(uint32(parseWindowCloseAction(value)))
		}
		return
	}
	if raw, _, err := a.store.GetSetting("app"); err == nil {
		a.closePreference.Store(uint32(settingsWindowCloseAction(raw)))
	}
}

func (a *App) WindowCloseAction() string {
	a.awaitStartup()
	switch windowCloseAction(a.closePreference.Load()) {
	case closeUnset:
		if a.store == nil {
			return ""
		}
		return "退出程序"
	case closeMinimise:
		return "最小化到任务栏"
	case closeTray:
		return "隐藏到菜单栏"
	default:
		return "退出程序"
	}
}

func (a *App) SetWindowCloseAction(value string) error {
	a.awaitStartup()
	action := parseWindowCloseAction(value)
	if action == closeQuit && value != "退出程序" {
		return errors.New("无效的关闭窗口设置")
	}
	a.closeSettingsMu.Lock()
	defer a.closeSettingsMu.Unlock()
	if a.store != nil {
		raw, _ := json.Marshal(value)
		if err := a.store.SetSetting(windowCloseSettingKey, string(raw)); err != nil {
			return err
		}
	}
	a.closePreference.Store(uint32(action))
	return nil
}

// CloseWindow applies the same persisted preference as a system close request.
func (a *App) CloseWindow() bool {
	a.awaitStartup()
	if a.ctx == nil {
		return false
	}
	return a.applyWindowClose(a.ctx, windowCloseAction(a.closePreference.Load()))
}

func (a *App) applyWindowClose(ctx context.Context, action windowCloseAction) bool {
	if a.explicitQuit.Load() {
		return true
	}
	switch action {
	case closeTray:
		if a.hideToTray(ctx) {
			return true
		}
		runtime.WindowMinimise(ctx)
		return !windowTrayAvailable
	case closeMinimise:
		runtime.WindowMinimise(ctx)
	default:
		a.quitApplication(ctx)
	}
	return true
}

func (a *App) quitApplication(ctx context.Context) {
	a.explicitQuit.Store(true)
	runtime.Quit(ctx)
}

// DesktopWindowMenu keeps the macOS quit role separate from the close-window command.
func DesktopWindowMenu(a *App) *menu.Menu {
	if goruntime.GOOS != "darwin" {
		return nil
	}
	result := menu.NewMenuFromItems(menu.AppMenu(), menu.EditMenu())
	result.AddSubmenu("窗口").AddText("关闭窗口", keys.CmdOrCtrl("w"), func(_ *menu.CallbackData) {
		go a.CloseWindow()
	})
	return result
}

// PrepareWindowClose gives the WebView a bounded opportunity to finish local writes.
func PrepareWindowClose(a *App) func(context.Context) bool {
	const (
		idle uint32 = iota
		waiting
		ready
	)
	var state atomic.Uint32
	var handlingNativeClose atomic.Bool
	return func(ctx context.Context) bool {
		if goruntime.GOOS == "windows" && !a.explicitQuit.Load() && state.Load() == idle {
			if handlingNativeClose.CompareAndSwap(false, true) {
				// Wails invokes the native close hook on the Windows UI thread.
				go func() {
					defer handlingNativeClose.Store(false)
					a.awaitStartup()
					a.applyWindowClose(ctx, windowCloseAction(a.closePreference.Load()))
				}()
			}
			return true
		}
		if !state.CompareAndSwap(idle, waiting) {
			return state.Load() != ready
		}
		a.explicitQuit.Store(true)
		finish := func() {
			if state.CompareAndSwap(waiting, ready) {
				runtime.Quit(ctx)
			}
		}
		flush := func() {
			runtime.EventsOnce(ctx, "jindi-window-close-ready", func(_ ...any) { finish() })
			time.AfterFunc(2*time.Second, finish)
			runtime.EventsEmit(ctx, "jindi-before-window-close")
		}
		if exporting := a.prepareExportShutdown(); exporting != nil {
			a.CancelUpdateDownload()
			go func() {
				<-exporting
				flush()
			}()
		} else {
			flush()
		}
		return true
	}
}

func (a *App) prepareExportShutdown() <-chan struct{} {
	a.mediaExportMu.Lock()
	defer a.mediaExportMu.Unlock()
	a.windowClosing = true
	return a.mediaExport
}
