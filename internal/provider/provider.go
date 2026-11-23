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
	ErrNotAuthorized = errors.New("provider: 未取得官方授权，功能已禁用")
	ErrUnsupported   = errors.New("provider: 不支持")
	ErrNotFound      = errors.New("provider: 未找到")
)

var AllCapabilities = []string{"catalog", "search", "playback", "account_login", "comments_read", "comments_write", "danmaku_read", "danmaku_write", "favorites_sync", "history_sync"}

type Capabilities map[string]CapabilityState

type Drama struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Tags         []string `json:"tags"`
	Summary      string   `json:"summary"`
	EpisodeCount int      `json:"episodeCount"`
}

type PlaybackTicket struct {
	Kind         string   `json:"kind"` // demo | authorized
	EpisodeID    string   `json:"episodeId"`
	PlaybackURL  string   `json:"playbackUrl,omitempty"`
	Restrictions []string `json:"restrictions,omitempty"`
}

type VideoProvider interface {
	Name() string
	Capabilities(ctx context.Context) Capabilities
	Home(ctx context.Context) ([]Drama, error)
	Search(ctx context.Context, query string) ([]Drama, error)
	Drama(ctx context.Context, id string) (Drama, error)
	ResolvePlayback(ctx context.Context, dramaID string, episode int) (PlaybackTicket, error)
}
