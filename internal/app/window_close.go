package app

import (
	"context"
	"encoding/json"
	"errors"
	goruntime "runtime"
	"strconv"
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

const (
	windowCloseReadyEvent    = "jindi-window-close-ready"
	windowCloseDeadlineEvent = "jindi-window-close-deadline"
	windowCloseCanceledEvent = "jindi-window-close-canceled"
)

type pendingWindowWrites struct {
	library bool
	danmaku bool
}

func parseWindowCloseResponse(id string, data []any) (pendingWindowWrites, bool) {
	if (len(data) != 2 && len(data) != 3) || data[0] != id {
		return pendingWindowWrites{}, false
	}
	library, ok := data[1].(bool)
	if !ok {
		return pendingWindowWrites{}, false
	}
	if len(data) == 2 {
		return pendingWindowWrites{library: library}, true
	}
	danmaku, ok := data[2].(bool)
	if !ok {
		return pendingWindowWrites{}, false
	}
	return pendingWindowWrites{library: library, danmaku: danmaku}, true
}

// PrepareWindowClose gives the WebView a bounded opportunity to finish local writes.
func PrepareWindowClose(a *App) func(context.Context) bool {
	const (
		waiting uint32 = iota
		confirming
		ready
	)
	type closeAttempt struct {
		state atomic.Uint32
	}
	var active atomic.Pointer[closeAttempt]
	var sequence atomic.Uint64
	var handlingNativeClose atomic.Bool
	return func(ctx context.Context) bool {
		if current := active.Load(); current != nil {
			return current.state.Load() != ready
		}
		if goruntime.GOOS == "windows" && !a.explicitQuit.Load() {
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

		current := new(closeAttempt)
		if !active.CompareAndSwap(nil, current) {
			return true
		}
		id := strconv.FormatUint(sequence.Add(1), 10)
		a.explicitQuit.Store(true)
		finish := func(from uint32) {
			if current.state.CompareAndSwap(from, ready) {
				if from == waiting {
					runtime.EventsOff(ctx, windowCloseReadyEvent)
				}
				runtime.Quit(ctx)
			}
		}
		flush := func() {
			runtime.EventsOn(ctx, windowCloseReadyEvent, func(data ...any) {
				pending, ok := parseWindowCloseResponse(id, data)
				if !ok {
					return
				}
				if !pending.library && !pending.danmaku {
					finish(waiting)
					return
				}
				if !current.state.CompareAndSwap(waiting, confirming) {
					return
				}

				runtime.EventsOff(ctx, windowCloseReadyEvent)
				if confirmUnsavedQuit(ctx, pending) {
					finish(confirming)
					return
				}
				a.mediaExportMu.Lock()
				a.windowClosing = false
				a.mediaExportMu.Unlock()
				a.explicitQuit.Store(false)
				active.CompareAndSwap(current, nil)
				runtime.EventsEmit(ctx, windowCloseCanceledEvent, id)
			})
			time.AfterFunc(2*time.Second, func() {
				if current.state.Load() != waiting {
					return
				}
				// Give a responsive WebView one final check before a forced close.
				time.AfterFunc(250*time.Millisecond, func() { finish(waiting) })
				runtime.EventsEmit(ctx, windowCloseDeadlineEvent, id)
			})
			runtime.EventsEmit(ctx, "jindi-before-window-close", id)
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

func confirmUnsavedQuit(ctx context.Context, pending pendingWindowWrites) bool {
	// Wails maps Windows question dialogs to the system's localized Yes/No.
	quit, keepEditing := "Yes", "No"
	if goruntime.GOOS == "darwin" {
		quit, keepEditing = "仍然退出", "继续编辑"
	}
	title := "收藏或历史尚未保存"
	message := "收藏或观看历史的改动尚未保存，退出后可能丢失。仍然退出吗？"
	if pending.danmaku {
		title = "正在保存弹幕"
		message = "弹幕还未收到保存回执，退出后可能丢失。仍然退出吗？"
		if pending.library {
			title = "仍有本地内容尚未确认保存"
			message = "收藏或观看历史的改动及弹幕尚未确认保存，退出后可能丢失。仍然退出吗？"
		}
	}
	selected, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         title,
		Message:       message,
		Buttons:       []string{quit, keepEditing},
		DefaultButton: keepEditing,
		CancelButton:  keepEditing,
	})
	return err == nil && selected == quit
}

func (a *App) prepareExportShutdown() <-chan struct{} {
	a.mediaExportMu.Lock()
	defer a.mediaExportMu.Unlock()
	a.windowClosing = true
	return a.mediaExport
}
