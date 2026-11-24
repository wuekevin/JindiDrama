package app

import (
	"context"
	"errors"
	"time"

	"github.com/gen2brain/beeep"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"jindi-drama/internal/update"
)

// Version 由构建时 -ldflags "-X jindi-drama/internal/app.Version=vX.Y.Z" 注入。
var Version = "v0.1.0"

func (a *App) AppVersion() string { return Version }

// CheckUpdate 查询 GitHub Releases；有新版本时发出系统通知。不下载、不安装。
func (a *App) CheckUpdate() (update.Result, error) {
	ctx, cancel := context.WithTimeout(a.context(), 10*time.Second)
	defer cancel()
	res, err := update.Check(ctx, nil, update.DefaultAPI, Version)
	if err == nil && res.HasUpdate {
		a.Notify("金帝短剧有新版本", "发现 "+res.Latest+"，可在设置 → 关于与更新中查看")
	}
	return res, err
}

// Notify 发送系统通知（失败只记日志，不影响主流程）。
func (a *App) Notify(title, body string) {
	if err := beeep.Notify(title, body, ""); err != nil && a.log != nil {
		_ = a.log.Log("warn", "notify", "系统通知发送失败")
	}
}

// OpenReleasePage 仅允许打开本项目的 GitHub 页面。
func (a *App) OpenReleasePage(url string) error {
	if !update.SafeReleaseURL(url) {
		return errors.New("不允许打开该链接")
	}
	if a.ctx == nil {
		return errors.New("窗口尚未就绪")
	}
	wr.BrowserOpenURL(a.ctx, url)
	return nil
}
