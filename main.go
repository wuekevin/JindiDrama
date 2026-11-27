package main

import (
	"context"
	"embed"
	"io/fs"
	"os"
	"os/signal"
	goruntime "runtime"
	"sync/atomic"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"jindi-drama/internal/app"
	"jindi-drama/internal/observability"
	"jindi-drama/internal/rollbackcli"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if args := os.Args[1:]; len(args) > 0 && args[0] == rollbackcli.DesktopFlag {
		os.Exit(rollbackcli.Run(args[1:], os.Stdout, os.Stderr))
	}
	// Wails installs its own signal handler only after the desktop host initializes.
	startupSignals, stopStartupSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopStartupSignals()
	var session atomic.Pointer[observability.Session]
	var documentReady atomic.Bool
	if goruntime.GOOS != "windows" {
		session.Store(beginDesktopSession())
	}
	defer func() { session.Load().End(false) }()
	links := newShareLinks()
	links.openArgs(os.Args[1:])
	instance, releaseLaunch := prepareShareLaunch(links)
	defer releaseLaunch()
	webAssets, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		println("读取应用资源失败:", err.Error())
		return
	}
	a := app.NewWithAssets(webAssets)
	app.SetFrontendReadyHandler(a, func() {
		documentReady.Store(true)
		session.Load().Ready()
		links.frontendReady()
	})
	assetOptions := &assetserver.Options{Assets: assets}
	stopMedia := prepareDesktopMedia(a, assetOptions)
	clean := false
	defer func() {
		stopMedia()
		if clean {
			session.Load().End(true)
		}
	}()
	startup, shutdown := app.Lifecycle(a)
	beforeClose, quit := app.PrepareWindowClose(a)
	const (
		starting = iota
		quitAfterStartup
		started
	)
	var startupState atomic.Uint32
	var desktopContext context.Context
	err = wails.Run(&options.App{
		Title:            "金帝短剧",
		Width:            1448,
		Height:           1086,
		MinWidth:         1280,
		MinHeight:        860,
		Frameless:        true,
		AssetServer:      assetOptions,
		BackgroundColour: &options.RGBA{R: 15, G: 8, B: 9, A: 1},
		OnStartup: func(ctx context.Context) {
			releaseLaunch()
			// Wails exits forwarded Windows instances before OnStartup.
			if goruntime.GOOS == "windows" {
				active := beginDesktopSession()
				session.Store(active)
				if documentReady.Load() {
					active.Ready()
				}
			}
			links.start(func(route string) {
				if goruntime.GOOS == "darwin" && runtime.WindowIsMinimised(ctx) {
					runtime.WindowUnminimise(ctx)
				}
				runtime.WindowShow(ctx)
				if route != "" {
					runtime.EventsEmit(ctx, "jindi-open-share-link", route)
				}
			})
			startup(ctx)
			desktopContext = ctx
			quitRequested := startupState.Swap(started) == quitAfterStartup || startupSignals.Err() != nil
			stopStartupSignals()
			if quitRequested {
				quit(ctx)
			}
		},
		OnShutdown: shutdown,
		OnBeforeClose: func(context.Context) bool {
			// Wails can receive a quit signal before constructing the native window.
			// Defer that request until startup can safely replay the normal close hook.
			if startupState.CompareAndSwap(starting, quitAfterStartup) || startupState.Load() == quitAfterStartup {
				return true
			}
			// Startup publishes the complete Wails context before marking the state started.
			return beforeClose(desktopContext)
		},
		Menu:               app.DesktopWindowMenu(a),
		SingleInstanceLock: instance,
		Bind:               []interface{}{a},
		Windows:            &windows.Options{WebviewIsTransparent: false, WindowIsTranslucent: false, WindowClassName: app.WindowsMainWindowClass},
		Mac: &mac.Options{
			OnUrlOpen: links.open,
			TitleBar:  mac.TitleBarHiddenInset(),
			Preferences: &mac.Preferences{
				EnableAutoplayWithoutUserAction: mac.Enabled,
				FullscreenEnabled:               mac.Enabled,
			},
		},
	})
	if err != nil {
		println("Error:", err.Error())
		return
	}
	clean = true
}
