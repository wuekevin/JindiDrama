// Package mock 是离线 MockProvider：不访问网络，数据仅用于 UI 与流程验证。
package mock

import (
	"context"
	"strings"

	"jindi-drama/internal/provider"
)

type Provider struct{ dramas []provider.Drama }

func New() *Provider {
	return &Provider{dramas: []provider.Drama{
		{ID: "d1", Title: "替嫁成了千亿总裁的心尖宠", Tags: []string{"都市", "甜宠", "替嫁"}, Summary: "演示数据", EpisodeCount: 80},
		{ID: "d3", Title: "雨夜逢君", Tags: []string{"都市", "救赎"}, Summary: "演示数据", EpisodeCount: 72},
		{ID: "d10", Title: "将门凰女", Tags: []string{"古装", "权谋"}, Summary: "演示数据", EpisodeCount: 90},
	}}
}

func (p *Provider) Name() string { return "mock" }

func (p *Provider) Capabilities(context.Context) provider.Capabilities {
	return provider.Capabilities{
		"catalog": provider.Mock, "search": provider.Mock, "playback": provider.Mock,
		"account_login": provider.NotAuthorized, "comments_read": provider.Mock, "comments_write": provider.Mock,
		"danmaku_read": provider.Mock, "danmaku_write": provider.Mock,
		"favorites_sync": provider.NotAuthorized, "history_sync": provider.NotAuthorized,
	}
}

func (p *Provider) Home(context.Context) ([]provider.Drama, error) { return p.dramas, nil }

func (p *Provider) Search(_ context.Context, q string) ([]provider.Drama, error) {
	q = strings.TrimSpace(q)
	var out []provider.Drama
	for _, d := range p.dramas {
		if q == "" || strings.Contains(d.Title, q) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (p *Provider) Drama(_ context.Context, id string) (provider.Drama, error) {
	for _, d := range p.dramas {
		if d.ID == id {
			return d, nil
		}
	}
	return provider.Drama{}, provider.ErrNotFound
}

func (p *Provider) ResolvePlayback(_ context.Context, id string, ep int) (provider.PlaybackTicket, error) {
	if _, err := p.Drama(context.Background(), id); err != nil {
		return provider.PlaybackTicket{}, err
	}
	return provider.PlaybackTicket{Kind: "demo", EpisodeID: id, Restrictions: []string{"模拟播放：无真实媒体"}}, nil
}
