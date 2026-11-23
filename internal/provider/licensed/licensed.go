// Package licensed 是 LicensedDemoProvider 占位：仅在配置了开发者自有版权/公开许可的媒体目录后启用（P1）。
package licensed

import (
	"context"

	"jindi-drama/internal/provider"
)

type DemoProvider struct{ MediaDir string }

func New(mediaDir string) *DemoProvider { return &DemoProvider{MediaDir: mediaDir} }

func (*DemoProvider) Name() string { return "licensed-demo" }

func (p *DemoProvider) Capabilities(context.Context) provider.Capabilities {
	c := provider.Capabilities{}
	for _, k := range provider.AllCapabilities {
		c[k] = provider.Unavailable
	}
	if p.MediaDir != "" {
		c["playback"] = provider.Unavailable // P1 实现媒体解析后才可置为 ready
	}
	return c
}
func (*DemoProvider) Home(context.Context) ([]provider.Drama, error) {
	return nil, provider.ErrUnsupported
}
func (*DemoProvider) Search(context.Context, string) ([]provider.Drama, error) {
	return nil, provider.ErrUnsupported
}
func (*DemoProvider) Drama(context.Context, string) (provider.Drama, error) {
	return provider.Drama{}, provider.ErrUnsupported
}
func (*DemoProvider) ResolvePlayback(context.Context, string, int) (provider.PlaybackTicket, error) {
	return provider.PlaybackTicket{}, provider.ErrUnsupported
}
