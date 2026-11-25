//go:build bindings

package main

import (
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"jindi-drama/internal/app"
	"jindi-drama/internal/observability"
)

func prepareDesktopMedia(*app.App, *assetserver.Options) func() {
	return func() {}
}

func beginDesktopSession() *observability.Session { return nil }
