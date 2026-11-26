package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/licensed"
)

type catalogPlaybackFixture struct {
	provider.VideoProvider
	caps                       provider.Capabilities
	episodes                   []provider.Episode
	ticket                     provider.PlaybackTicket
	resolveErr                 error
	episodeCalls, resolveCalls int
}

func (p *catalogPlaybackFixture) Capabilities(context.Context) provider.Capabilities { return p.caps }
func (*catalogPlaybackFixture) Name() string                                         { return "authorized-fixture" }
func (*catalogPlaybackFixture) Drama(_ context.Context, id string) (provider.Drama, error) {
	return provider.Drama{ID: id, EpisodeCount: 2, UpdatedTo: 1}, nil
}
func (p *catalogPlaybackFixture) Episodes(context.Context, string) ([]provider.Episode, error) {
	p.episodeCalls++
	return p.episodes, nil
}
func (p *catalogPlaybackFixture) ResolvePlayback(context.Context, string, int) (provider.PlaybackTicket, error) {
	p.resolveCalls++
	return p.ticket, p.resolveErr
}
func readyCatalogPlayback() *catalogPlaybackFixture {
	return &catalogPlaybackFixture{
		caps:     provider.Capabilities{"catalog": provider.Ready, "playback": provider.Ready},
		episodes: []provider.Episode{{ID: "feed-1", DramaID: "local-overlap", Index: 1, Available: true}},
		ticket: provider.PlaybackTicket{Kind: "authorized", EpisodeID: "feed-1", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339), Sources: []provider.MediaSource{
			{Src: "https://media.example.test/one.mp4?authorization=test-opaque", Type: "video/mp4", Quality: "sd"},
		}},
	}
}

func TestCatalogPlaybackUsesActiveProviderAndReturnsOnlyAllowedSources(t *testing.T) {
	p := readyCatalogPlayback()
	a := NewWith(p)
	t.Cleanup(a.closeCatalogMedia)
	ConfigureLicensedMedia(a, &licensed.DemoProvider{}, nil)
	p.ticket.Sources = append([]provider.MediaSource{
		{Src: "https://127.0.0.1/one.mp4", Type: "video/mp4"},
		{Src: "https://media.example.test/live.m3u8", Type: "application/vnd.apple.mpegurl"},
	}, p.ticket.Sources...)
	got, err := a.CatalogPlayback("local-overlap", 1)
	if err != nil || got.Kind != "authorized" || got.EpisodeID != "feed-1" || len(got.Sources) != 1 ||
		got.Sources[0].Type != "video/mp4" || got.Sources[0].Quality != "sd" ||
		got.LeaseID == "" || !strings.HasPrefix(got.Sources[0].Src, "http://127.0.0.1:") || got.ExpiresAt != p.ticket.ExpiresAt {
		t.Fatalf("resolved authorized source: %+v err=%v", got, err)
	}
	renderer, err := json.Marshal(got)
	if err != nil || strings.Contains(string(renderer), "test-opaque") || strings.Contains(string(renderer), "example.test") {
		t.Fatalf("upstream must not appear in renderer ticket: %s, err=%v", renderer, err)
	}
	p.ticket.Sources = nil
	p.ticket.PlaybackURL = "https://media.example.test/one.webm?authorization=test-opaque"
	got, err = a.CatalogPlayback("local-overlap", 1)
	if err != nil || len(got.Sources) != 1 || got.Sources[0].Type != "video/webm" || got.PlaybackURL != "" {
		t.Fatalf("normalized direct URL: %+v err=%v", got, err)
	}
}

func TestCatalogPlaybackChecksCapabilitiesAndEpisodeBeforeResolving(t *testing.T) {
	p := readyCatalogPlayback()
	a := NewWith(p)
	p.caps["playback"] = provider.NotAuthorized
	if _, err := a.CatalogPlayback("local-overlap", 1); !errors.Is(err, provider.ErrNotAuthorized) || p.episodeCalls != 0 {
		t.Fatalf("denied capability: calls=%d err=%v", p.episodeCalls, err)
	}
	p.caps["playback"] = provider.Ready
	if _, err := a.ResolvePlayback("feed", 1); !errors.Is(err, provider.ErrNotAuthorized) || p.resolveCalls != 0 {
		t.Fatalf("legacy binding must not serve active catalog tickets: calls=%d err=%v", p.resolveCalls, err)
	}
	if _, err := a.CatalogPlayback("local-overlap", 2); !errors.Is(err, provider.ErrEpisodeUnavailable) || p.episodeCalls != 0 {
		t.Fatalf("denied unreleased episode: calls=%d err=%v", p.episodeCalls, err)
	}
	p.episodes[0].DramaID = "unrelated"
	if _, err := a.CatalogPlayback("local-overlap", 1); !errors.Is(err, provider.ErrEpisodeUnavailable) || p.resolveCalls != 0 {
		t.Fatalf("denied episode: calls=%d err=%v", p.resolveCalls, err)
	}
}

func TestCatalogPlaybackRejectsInvalidTicketsWithoutExposingProviderErrors(t *testing.T) {
	cases := map[string]func(*catalogPlaybackFixture){
		"other-source":         func(p *catalogPlaybackFixture) { p.ticket.Kind = "demo" },
		"other-episode":        func(p *catalogPlaybackFixture) { p.ticket.EpisodeID = "feed-2" },
		"expired":              func(p *catalogPlaybackFixture) { p.ticket.ExpiresAt = "2001-01-01T00:00:00Z" },
		"missing-expiry":       func(p *catalogPlaybackFixture) { p.ticket.ExpiresAt = "" },
		"unparseable-expiry":   func(p *catalogPlaybackFixture) { p.ticket.ExpiresAt = "tomorrow" },
		"unhandled-protection": func(p *catalogPlaybackFixture) { p.ticket.Restrictions = []string{"drm"} },
		"no-download":          func(p *catalogPlaybackFixture) { p.ticket.Restrictions = []string{"no-download"} },
		"unsafe-http":          func(p *catalogPlaybackFixture) { p.ticket.Sources[0].Src = "http://media.example.test/a.mp4" },
		"local-host":           func(p *catalogPlaybackFixture) { p.ticket.Sources[0].Src = "https://media.local/a.mp4" },
		"numeric-host":         func(p *catalogPlaybackFixture) { p.ticket.Sources[0].Src = "https://127.0.0.0x1/a.mp4" },
		"inline-password": func(p *catalogPlaybackFixture) {
			p.ticket.Sources[0].Src = "https://user:test-opaque@media.example.test/a.mp4"
		},
		"upstream-error": func(p *catalogPlaybackFixture) {
			p.resolveErr = errors.New("https://media.example.test/?token=test-opaque")
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := readyCatalogPlayback()
			mutate(p)
			got, err := NewWith(p).CatalogPlayback("local-overlap", 1)
			if (!errors.Is(err, errCatalogPlaybackUnavailable) && !errors.Is(err, provider.ErrUnsupported)) ||
				got.Kind != "" || len(got.Sources) != 0 {
				t.Fatalf("invalid ticket escaped or disclosed an upstream error: %+v err=%v", got, err)
			}
		})
	}
}
