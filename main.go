package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"jindi-drama/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := app.New()
	err := wails.Run(&options.App{
		Title:            "金帝短剧",
		Width:            1448,
		Height:           1086,
		MinWidth:         1280,
		MinHeight:        860,
		Frameless:        true,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 15, G: 8, B: 9, A: 1},
		OnStartup:        a.Startup,
		Bind:             []interface{}{a},
		Windows:          &windows.Options{WebviewIsTransparent: false, WindowIsTranslucent: false},
		Mac:              &mac.Options{TitleBar: mac.TitleBarHiddenInset(), WebviewIsTransparent: false},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
