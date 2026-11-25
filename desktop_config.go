//go:build !bindings

package main

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"jindi-drama/internal/app"
	"jindi-drama/internal/observability"
	"jindi-drama/internal/provider/licensed"
)

func prepareDesktopMedia(a *app.App, options *assetserver.Options) func() {
	directory, err := app.PrepareLicensedDirectory(a, os.Getenv("JINDIDRAMA_LICENSED_MEDIA_DIR"))
	if err != nil || directory == "" {
		app.ConfigureLicensedMedia(a, nil, err)
		return func() {}
	}
	media, err := licensed.New(directory)
	if err != nil {
		app.ConfigureLicensedMedia(a, nil, err)
		return func() {}
	}
	if runtime.GOOS == "windows" {
		stop, err := media.StartLoopback()
		if err != nil {
			_ = media.Close()
			app.ConfigureLicensedMedia(a, nil, err)
			return func() {}
		}
		app.ConfigureLicensedMedia(a, media, nil)
		return func() {
			_ = stop()
			_ = media.Close()
		}
	}
	options.Middleware = media.Middleware
	app.ConfigureLicensedMedia(a, media, nil)
	return func() { _ = media.Close() }
}

func beginDesktopSession() *observability.Session {
	dir, err := app.DataDir()
	if err != nil {
		return nil
	}
	logDir := filepath.Join(dir, "logs")
	log, err := observability.New(logDir, observability.DefaultLogBytes)
	if err != nil {
		return nil
	}
	session, err := observability.StartSession(filepath.Join(logDir, "sessions"), app.Version, log)
	if err != nil {
		_ = log.Log("warn", "session", "无法创建桌面运行标记")
	}
	return session
}
