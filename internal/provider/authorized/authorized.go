// Package authorized 保留 AuthorizedHongguoProvider 的位置。
// 在取得官方授权、接口文档与测试账号前（见 docs/UPSTREAM-AUTHORIZATION.md）保持禁用：
// 所有能力 not_authorized，所有方法返回 ErrNotAuthorized，且不发起任何网络请求。
package authorized

import (
	"context"

	"jindi-drama/internal/provider"
)

type HongguoProvider struct{}

func New() *HongguoProvider { return &HongguoProvider{} }

func (*HongguoProvider) Name() string { return "authorized-hongguo" }

func (*HongguoProvider) Capabilities(context.Context) provider.Capabilities {
	c := provider.Capabilities{}
	for _, k := range provider.AllCapabilities {
		c[k] = provider.NotAuthorized
	}
	return c
}
func (*HongguoProvider) HomeFeed(context.Context) (provider.HomeFeed, error) {
	return provider.HomeFeed{}, provider.ErrNotAuthorized
}
func (*HongguoProvider) Home(context.Context) ([]provider.Drama, error) {
	return nil, provider.ErrNotAuthorized
}
func (*HongguoProvider) Search(context.Context, string) ([]provider.Drama, error) {
	return nil, provider.ErrNotAuthorized
}
func (*HongguoProvider) Drama(context.Context, string) (provider.Drama, error) {
	return provider.Drama{}, provider.ErrNotAuthorized
}
func (*HongguoProvider) Episodes(context.Context, string) ([]provider.Episode, error) {
	return nil, provider.ErrNotAuthorized
}
func (*HongguoProvider) ResolvePlayback(context.Context, string, int) (provider.PlaybackTicket, error) {
	return provider.PlaybackTicket{}, provider.ErrNotAuthorized
}
