// Package app 是 Wails 绑定层：前端通过它访问 Provider，永远不直接访问上游。
package app

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"

	"jindi-drama/internal/cache"
	"jindi-drama/internal/observability"
	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/licensed"
	"jindi-drama/internal/provider/registry"
	"jindi-drama/internal/storage/sqlite"
	"jindi-drama/internal/update"
)

type App struct {
	ctx               context.Context
	assets            fs.FS
	frontendReady     func()
	p                 provider.VideoProvider
	licensed          *licensed.DemoProvider
	licensedErr       error
	licensedDirectory string
	licensedManaged   bool
	licensedConfigMu  sync.Mutex
	mediaExportMu     sync.Mutex
	mediaExport       chan struct{}
	windowClosing     bool
	closeSettingsMu   sync.Mutex
	closePreference   atomic.Uint32
	explicitQuit      atomic.Bool
	store             *sqlite.Store
	cache             *cache.Cache
	cacheMu           sync.Mutex
	startupDone       <-chan struct{}
	cacheDefault      string
	cacheFallback     bool
	updateMu          sync.Mutex
	updateRevision    uint64
	releaseURL        string
	releaseAsset      *update.Asset
	cancelUpdate      context.CancelFunc
	log               *observability.Logger
}

func New() *App { return &App{p: registry.NewOrDefault(registry.Default, registry.Options{})} }

func NewWithAssets(assets fs.FS) *App {
	a := New()
	a.assets = assets
	return a
}

// SetFrontendReadyHandler wires the bundled frontend's startup acknowledgement.
func SetFrontendReadyHandler(a *App, handler func()) {
	a.frontendReady = handler
}

func (a *App) MarkFrontendReady() {
	if a.frontendReady != nil {
		a.frontendReady()
	}
}

// ConfigureLicensedMedia is called by the desktop host before bindings are registered.
func ConfigureLicensedMedia(a *App, p *licensed.DemoProvider, err error) {
	a.licensed, a.licensedErr = p, err
}

func (a *App) LicensedCatalog() ([]provider.Drama, error) {
	if a.licensedErr != nil {
		return nil, a.licensedErr
	}
	if a.licensed == nil {
		return []provider.Drama{}, nil
	}
	return a.licensed.Home(a.context())
}

func (a *App) providerForDrama(id string) provider.VideoProvider {
	if a.licensed != nil && strings.HasPrefix(id, "local-") {
		return a.licensed
	}
	return a.p
}

// NewWith 允许注入其他 Provider（测试或授权后切换）。
func NewWith(p provider.VideoProvider) *App { return &App{p: p} }

// Lifecycle keeps Wails callbacks outside the methods exposed to the WebView.
func Lifecycle(a *App) (startup, shutdown func(context.Context)) {
	ready := make(chan struct{})
	a.startupDone = ready
	return func(ctx context.Context) {
			defer close(ready)
			a.startup(ctx)
		}, func(context.Context) {
			closeWindowTray()
			a.close()
		}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.openRuntime(""); err != nil {
		println("缓存/日志初始化失败:", err.Error())
	}
	if a.store == nil {
		if err := a.openStore(""); err != nil {
			println("本地存储打开失败（将回退到浏览器存储）:", err.Error())
			var schemaErr *sqlite.UnsupportedSchemaError
			if errors.As(err, &schemaErr) && a.log != nil {
				_ = a.log.Log("warn", "storage", schemaErr.Error())
			}
		}
	}
	a.restoreWindowCloseAction()
	a.restoreCacheDirectory()
	a.restoreCacheLimit()
}

func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// ProviderName 返回当前 Provider，前端据此显示“演示模式”。
func (a *App) ProviderName() string { return a.p.Name() }

func (a *App) Capabilities() provider.Capabilities { return a.p.Capabilities(a.context()) }

func (a *App) Home() ([]provider.Drama, error) { return a.p.Home(a.context()) }

func (a *App) HomeFeed() (provider.HomeFeed, error) { return a.p.HomeFeed(a.context()) }

func (a *App) Search(q string) ([]provider.Drama, error) {
	ctx := a.context()
	results, err := a.p.Search(ctx, q)
	if err != nil || a.licensed == nil {
		return results, err
	}
	local, err := a.licensed.Search(ctx, q)
	if err != nil {
		return nil, err
	}
	return append(local, results...), nil
}

func (a *App) Drama(id string) (provider.Drama, error) {
	return a.providerForDrama(id).Drama(a.context(), id)
}

func (a *App) Episodes(dramaID string) ([]provider.Episode, error) {
	return a.providerForDrama(dramaID).Episodes(a.context(), dramaID)
}

func (a *App) ResolvePlayback(dramaID string, episode int) (provider.PlaybackTicket, error) {
	return a.providerForDrama(dramaID).ResolvePlayback(a.context(), dramaID, episode)
}
