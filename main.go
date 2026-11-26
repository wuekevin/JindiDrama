package main

import (
	"context"
	"embed"
	"io/fs"
	"os"
	goruntime "runtime"
	"sync/atomic"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"jindi-drama/internal/app"
	"jindi-drama/internal/observability"
	"jindi-drama/internal/provider/reference"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
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
	if name := os.Getenv("JINDIDRAMA_REFERENCE_PROVIDER"); name != "" {
		catalog, err := reference.New(name, os.Getenv("JINDIDRAMA_REFERENCE_URL"), os.Getenv("JINDIDRAMA_REFERENCE_TOKEN"))
		if err != nil {
			println(err.Error())
			return
		}
		app.ConfigureCatalogProvider(a, catalog)
	}
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
		},
		OnShutdown:         shutdown,
		OnBeforeClose:      app.PrepareWindowClose(a),
		Menu:               app.DesktopWindowMenu(a),
		SingleInstanceLock: instance,
		Bind:               []interface{}{a},
		Windows:            &windows.Options{WebviewIsTransparent: false, WindowIsTranslucent: false, WindowClassName: app.WindowsMainWindowClass},
		Mac: &mac.Options{
			OnUrlOpen:   links.open,
			TitleBar:    mac.TitleBarHiddenInset(),
			Preferences: &mac.Preferences{EnableAutoplayWithoutUserAction: mac.Enabled},
		},
	})
	if err != nil {
		println("Error:", err.Error())
		return
	}
	clean = true
}
