// Package mock 是离线 MockProvider：不访问网络，数据仅用于 UI 与流程验证。
package mock

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"jindi-drama/internal/provider"
)

//go:embed catalog.json
var catalog []byte

//go:embed rankings.json
var rankings []byte

//go:embed cast.json
var cast []byte

type castMember struct {
	Name string `json:"name"`
}

type Provider struct {
	dramas []provider.Drama
	cast   map[string][]castMember
}

func New() *Provider {
	p := &Provider{}
	if err := json.Unmarshal(catalog, &p.dramas); err != nil {
		panic(fmt.Errorf("mock: invalid bundled catalog: %w", err))
	}
	if err := json.Unmarshal(cast, &p.cast); err != nil {
		panic(fmt.Errorf("mock: invalid bundled cast: %w", err))
	}
	return p
}

func cloneDrama(d provider.Drama) provider.Drama {
	d.Tags = slices.Clone(d.Tags)
	return d
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

func (p *Provider) Home(ctx context.Context) ([]provider.Drama, error) {
	return p.Search(ctx, "")
}

func (p *Provider) HomeFeed(ctx context.Context) (provider.HomeFeed, error) {
	if err := ctx.Err(); err != nil {
		return provider.HomeFeed{}, err
	}
	feed := provider.HomeFeed{TodayNew: make([]string, 0)}
	if err := json.Unmarshal(rankings, &feed.Rankings); err != nil {
		return provider.HomeFeed{}, fmt.Errorf("mock: invalid bundled rankings: %w", err)
	}
	for _, drama := range p.dramas {
		if drama.IsNewToday {
			feed.TodayNew = append(feed.TodayNew, drama.ID)
		}
	}
	return feed, nil
}

func (p *Provider) Search(_ context.Context, q string) ([]provider.Drama, error) {
	q = strings.TrimSpace(q)
	out := make([]provider.Drama, 0)
	for _, d := range p.dramas {
		if strings.Contains(d.Title, q) || strings.Contains(d.Summary, q) ||
			slices.ContainsFunc(d.Tags, func(tag string) bool { return strings.Contains(tag, q) }) ||
			slices.ContainsFunc(p.cast[d.ID], func(member castMember) bool { return strings.Contains(member.Name, q) }) {
			out = append(out, cloneDrama(d))
		}
	}
	return out, nil
}

func (p *Provider) Drama(_ context.Context, id string) (provider.Drama, error) {
	for _, d := range p.dramas {
		if d.ID == id {
			return cloneDrama(d), nil
		}
	}
	return provider.Drama{}, provider.ErrNotFound
}

func episodeID(dramaID string, index int) string {
	return fmt.Sprintf("%s:%d", dramaID, index)
}

func (p *Provider) Episodes(ctx context.Context, id string) ([]provider.Episode, error) {
	drama, err := p.Drama(ctx, id)
	if err != nil {
		return nil, err
	}
	episodes := make([]provider.Episode, drama.EpisodeCount)
	for idx := range episodes {
		index := idx + 1
		episodes[idx] = provider.Episode{ID: episodeID(id, index), DramaID: id, Index: index, Available: index <= drama.UpdatedTo}
	}
	return episodes, nil
}

func (p *Provider) ResolvePlayback(ctx context.Context, id string, ep int) (provider.PlaybackTicket, error) {
	drama, err := p.Drama(ctx, id)
	if err != nil {
		return provider.PlaybackTicket{}, err
	}
	if ep < 1 || ep > drama.EpisodeCount {
		return provider.PlaybackTicket{}, provider.ErrNotFound
	}
	if ep > drama.UpdatedTo {
		return provider.PlaybackTicket{}, provider.ErrEpisodeUnavailable
	}
	clip := ep%3 + 1
	return provider.PlaybackTicket{
		Kind:      "demo",
		EpisodeID: episodeID(id, ep),
		Sources: []provider.MediaSource{
			{Src: fmt.Sprintf("./demo/demo-%d.webm", clip), Type: "video/webm", Quality: "hd"},
			{Src: fmt.Sprintf("./demo/demo-%d.mp4", clip), Type: "video/mp4", Quality: "hd"},
			{Src: fmt.Sprintf("./demo/demo-%d-sd.webm", clip), Type: "video/webm", Quality: "sd"},
			{Src: fmt.Sprintf("./demo/demo-%d-sd.mp4", clip), Type: "video/mp4", Quality: "sd"},
		},
	}, nil
}
