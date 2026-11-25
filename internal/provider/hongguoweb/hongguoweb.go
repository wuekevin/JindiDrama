// Package hongguoweb 是 HongguoWebProvider（网页公开数据适配）的占位。
// 在取得对方书面许可、并确认网页数据的使用条款前保持禁用：
// 所有能力 not_authorized，所有方法返回 ErrNotAuthorized，且不发起任何网络请求。
// 启用条件见 docs/UPSTREAM-AUTHORIZATION.md。
package hongguoweb

import (
	"context"

	"jindi-drama/internal/provider"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (*Provider) Name() string { return "hongguo-web" }

func (*Provider) Capabilities(context.Context) provider.Capabilities {
	c := provider.Capabilities{}
	for _, k := range provider.AllCapabilities {
		c[k] = provider.NotAuthorized
	}
	return c
}
func (*Provider) HomeFeed(context.Context) (provider.HomeFeed, error) {
	return provider.HomeFeed{}, provider.ErrNotAuthorized
}
func (*Provider) Home(context.Context) ([]provider.Drama, error) {
	return nil, provider.ErrNotAuthorized
}
func (*Provider) Search(context.Context, string) ([]provider.Drama, error) {
	return nil, provider.ErrNotAuthorized
}
func (*Provider) Drama(context.Context, string) (provider.Drama, error) {
	return provider.Drama{}, provider.ErrNotAuthorized
}
func (*Provider) Episodes(context.Context, string) ([]provider.Episode, error) {
	return nil, provider.ErrNotAuthorized
}
func (*Provider) ResolvePlayback(context.Context, string, int) (provider.PlaybackTicket, error) {
	return provider.PlaybackTicket{}, provider.ErrNotAuthorized
}
