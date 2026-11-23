// Package app 是 Wails 绑定层：前端通过它访问 Provider，永远不直接访问上游。
package app

import (
	"context"

	"jindi-drama/internal/cache"
	"jindi-drama/internal/observability"
	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/mock"
	"jindi-drama/internal/storage/sqlite"
)

type App struct {
	ctx   context.Context
	p     provider.VideoProvider
	store *sqlite.Store
	cache *cache.Cache
	log   *observability.Logger
}

func New() *App { return &App{p: mock.New()} }

// NewWith 允许注入其他 Provider（测试或授权后切换）。
func NewWith(p provider.VideoProvider) *App { return &App{p: p} }

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.OpenRuntime(""); err != nil {
		println("缓存/日志初始化失败:", err.Error())
	}
	if err := a.OpenStore(""); err != nil {
		println("本地存储打开失败（将回退到浏览器存储）:", err.Error())
	}
}

func (a *App) Shutdown(context.Context) { a.Close() }

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

func (a *App) Search(q string) ([]provider.Drama, error) { return a.p.Search(a.context(), q) }
