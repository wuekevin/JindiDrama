// Package provider 定义 Provider 接口与能力门禁。界面只通过 Provider 获取数据，
// 不得直接访问任何非公开端点。
package provider

import (
	"context"
	"errors"
)

type CapabilityState string

const (
	Ready         CapabilityState = "ready"
	Mock          CapabilityState = "mock"
	NotAuthorized CapabilityState = "not_authorized"
	Unavailable   CapabilityState = "unavailable"
	Unsupported   CapabilityState = "unsupported"
)

var (
	ErrNotAuthorized      = errors.New("provider: 未取得官方授权，功能已禁用")
	ErrUnsupported        = errors.New("provider: 不支持")
	ErrEpisodeUnavailable = errors.New("provider: 剧集尚未更新")
	ErrNotFound           = errors.New("provider: 未找到")
)

var AllCapabilities = []string{"catalog", "search", "playback", "account_login", "comments_read", "comments_write", "danmaku_read", "danmaku_write", "favorites_sync", "history_sync"}

type Capabilities map[string]CapabilityState

type HomeFeed struct {
	TodayNew []string `json:"todayNew"`
	Rankings Rankings `json:"rankings"`
}

type Rankings struct {
	Hot  []string `json:"hot"`
	Rise []string `json:"rise"`
	New  []string `json:"new"`
}

type Drama struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	License      string   `json:"license,omitempty"`
	AllowExport  bool     `json:"allowExport,omitempty"`
	Tags         []string `json:"tags"`
	Summary      string   `json:"summary"`
	UpdatedTo    int      `json:"updatedTo"`
	IsNewToday   bool     `json:"isNewToday,omitempty"`
	EpisodeCount int      `json:"episodeCount"`
}

type Episode struct {
	ID        string `json:"id"`
	DramaID   string `json:"dramaId"`
	Index     int    `json:"index"`
	Available bool   `json:"available"`
}

type MediaSource struct {
	Src     string `json:"src"`
	Type    string `json:"type"`
	Quality string `json:"quality,omitempty"`
}

type PlaybackTicket struct {
	Kind         string        `json:"kind"` // demo | licensed | authorized
	EpisodeID    string        `json:"episodeId"`
	Sources      []MediaSource `json:"sources,omitempty"`
	PlaybackURL  string        `json:"playbackUrl,omitempty"`
	ExpiresAt    string        `json:"expiresAt,omitempty"`
	LeaseID      string        `json:"leaseId,omitempty"`
	Restrictions []string      `json:"restrictions,omitempty"`
}

type VideoProvider interface {
	Name() string
	Capabilities(ctx context.Context) Capabilities
	Home(ctx context.Context) ([]Drama, error)
	HomeFeed(ctx context.Context) (HomeFeed, error)
	Search(ctx context.Context, query string) ([]Drama, error)
	Drama(ctx context.Context, id string) (Drama, error)
	Episodes(ctx context.Context, dramaID string) ([]Episode, error)
	ResolvePlayback(ctx context.Context, dramaID string, episode int) (PlaybackTicket, error)
}
