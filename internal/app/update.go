package app

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gen2brain/beeep"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"jindi-drama/internal/autostart"
	"jindi-drama/internal/update"
)

// Version 由构建时 -ldflags "-X jindi-drama/internal/app.Version=vX.Y.Z" 注入。
var Version = "v0.1.0"

const updateChannelKey = "update_channel"

func (a *App) AppVersion() string { return Version }

// CheckUpdate 查询 GitHub Releases；有新版本时发出系统通知。不下载、不安装。
func (a *App) CheckUpdate() (update.Result, error) {
	ctx, cancel := context.WithTimeout(a.context(), 10*time.Second)
	defer cancel()
	res, err := a.checkReleaseUpdate(ctx, update.DefaultAPI, update.DefaultPreviewAPI, Version)
	if err == nil && res.HasUpdate {
		a.notify("金帝短剧有新版本", "发现 "+res.Latest+"，可在设置 → 关于与更新中查看")
	}
	return res, err
}

// UpdateChannel defaults to stable unless SQLite contains explicit opt-in.
func (a *App) UpdateChannel() (string, error) {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	return a.readUpdateChannel()
}

func (a *App) readUpdateChannel() (string, error) {
	if a.store == nil {
		return update.Stable, nil
	}
	value, _, err := a.store.GetSetting(updateChannelKey)
	if err != nil {
		return "", err
	}
	if value == strconv.Quote(update.Preview) {
		return update.Preview, nil
	}
	return update.Stable, nil
}

func (a *App) SetUpdateChannel(channel string) error {
	if channel != update.Stable && channel != update.Preview {
		return errors.New("无效的更新渠道")
	}
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	if a.store == nil {
		return ErrNoStore
	}
	if err := a.store.SetSetting(updateChannelKey, strconv.Quote(channel)); err != nil {
		return err
	}
	a.updateRevision++
	a.releaseURL = ""
	a.releaseAsset = nil
	if a.cancelUpdate != nil {
		a.cancelUpdate()
	}
	return nil
}

func (a *App) checkReleaseUpdate(ctx context.Context, stableAPI, previewAPI, installed string) (update.Result, error) {
	a.updateMu.Lock()
	a.updateRevision++
	revision := a.updateRevision
	a.releaseURL = ""
	a.releaseAsset = nil
	if a.cancelUpdate != nil {
		a.cancelUpdate()
	}
	channel, err := a.readUpdateChannel()
	a.updateMu.Unlock()
	if err != nil {
		return update.Result{}, err
	}

	check, api := update.Check, stableAPI
	if channel == update.Preview {
		check, api = update.CheckPreview, previewAPI
	}
	result, err := check(ctx, nil, api, installed)

	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	if revision != a.updateRevision {
		return update.Result{}, errors.New("已有较新的版本检查请求")
	}
	if err == nil && result.HasUpdate {
		a.releaseURL = result.URL
		a.releaseAsset = result.Download
	}
	return result, err
}

// notify 发送系统通知（失败只记日志，不影响主流程）。
func (a *App) notify(title, body string) {
	if err := beeep.Notify(title, body, ""); err != nil && a.log != nil {
		_ = a.log.Log("warn", "notify", "系统通知发送失败")
	}
}

// OpenReleasePage 打开最近一次成功检查确认的版本页面。
func (a *App) OpenReleasePage(url string) error {
	a.updateMu.Lock()
	authorized := url != "" && url == a.releaseURL
	a.updateMu.Unlock()
	if !authorized {
		return errors.New("不允许打开该链接")
	}
	if a.ctx == nil {
		return errors.New("窗口尚未就绪")
	}
	wr.BrowserOpenURL(a.ctx, url)
	return nil
}

// AutostartEnabled / SetAutostart：开机启动（Windows 注册表 Run、macOS LaunchAgent）。
func (a *App) AutostartEnabled() (bool, error) {
	m, err := autostart.New()
	if err != nil {
		return false, err
	}
	return m.Enabled()
}

func (a *App) SetAutostart(on bool) error {
	m, err := autostart.New()
	if err != nil {
		return err
	}
	return m.Set(on)
}
