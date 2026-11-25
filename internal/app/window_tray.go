package app

import "context"

const WindowsMainWindowClass = "JindiDramaMainWindow"

func (a *App) WindowTrayAvailable() bool {
	return windowTrayAvailable
}

func (a *App) HideWindowToTray() bool {
	a.awaitStartup()
	return a.ctx != nil && a.hideToTray(a.ctx)
}

func (a *App) hideToTray(ctx context.Context) bool {
	return hideWindowToTray(ctx, func() { a.quitApplication(ctx) })
}
