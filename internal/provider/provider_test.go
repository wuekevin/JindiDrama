package provider_test

import (
	"context"
	"errors"
	"testing"

	"jindi-drama/internal/app"
	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/authorized"
	"jindi-drama/internal/provider/licensed"
	"jindi-drama/internal/provider/mock"
)

func TestMockProvider(t *testing.T) {
	p := mock.New()
	ctx := context.Background()
	caps := p.Capabilities(ctx)
	if caps["account_login"] != provider.NotAuthorized || caps["history_sync"] != provider.NotAuthorized {
		t.Fatalf("官方账号/云同步必须保持 not_authorized: %v", caps)
	}
	if got, _ := p.Search(ctx, "雨夜"); len(got) != 1 || got[0].ID != "d3" {
		t.Fatalf("search: %v", got)
	}
	if got, err := p.Search(ctx, "  废柴师妹  "); err != nil || len(got) != 1 || got[0].ID != "d5" {
		t.Fatalf("summary search: %v, %v", got, err)
	}
	if got, err := p.Search(ctx, "校园"); err != nil || len(got) != 1 || got[0].ID != "d4" {
		t.Fatalf("tag search: %v, %v", got, err)
	}
	if _, err := p.Drama(ctx, "nope"); !errors.Is(err, provider.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	tk, err := p.ResolvePlayback(ctx, "d1", 1)
	if err != nil || tk.Kind != "demo" || tk.PlaybackURL != "" {
		t.Fatalf("mock 播放不得返回真实 URL: %+v %v", tk, err)
	}
}

func TestAuthorizedProviderStaysDisabled(t *testing.T) {
	p := authorized.New()
	ctx := context.Background()
	for k, v := range p.Capabilities(ctx) {
		if v != provider.NotAuthorized {
			t.Fatalf("%s 应为 not_authorized，实际 %s", k, v)
		}
	}
	if len(p.Capabilities(ctx)) != len(provider.AllCapabilities) {
		t.Fatal("能力表不完整")
	}
	if _, err := p.Home(ctx); !errors.Is(err, provider.ErrNotAuthorized) {
		t.Fatal("Home 必须返回 ErrNotAuthorized")
	}
	if _, err := p.ResolvePlayback(ctx, "x", 1); !errors.Is(err, provider.ErrNotAuthorized) {
		t.Fatal("ResolvePlayback 必须返回 ErrNotAuthorized")
	}
}

func TestLicensedDemoWithoutConfiguration(t *testing.T) {
	p, err := licensed.New("")
	if err != nil {
		t.Fatal(err)
	}
	if p.Capabilities(context.Background())["playback"] != provider.Unavailable {
		t.Fatal("未配置时不能播放")
	}
	if _, err := licensed.New(t.TempDir()); err == nil {
		t.Fatal("缺失清单时应报告错误")
	}
}

func TestAppDefaultsToMock(t *testing.T) {
	a := app.New()
	if a.ProviderName() != "mock" {
		t.Fatal("默认必须是 mock")
	}
	if h, err := a.Home(); err != nil || len(h) == 0 {
		t.Fatalf("home: %v %v", h, err)
	}
	b := app.NewWith(authorized.New())
	if _, err := b.Search("x"); !errors.Is(err, provider.ErrNotAuthorized) {
		t.Fatal("授权 Provider 未授权时必须报错")
	}
}

func TestDesktopDemoEpisodesAndPlayback(t *testing.T) {
	a := app.New()
	for _, tc := range []struct {
		id        string
		total     int
		available int
	}{
		{"d1", 80, 32},
		{"d16", 48, 40},
		{"d17", 12, 3},
	} {
		drama, err := a.Drama(tc.id)
		if err != nil || drama.ID != tc.id || drama.EpisodeCount != tc.total {
			t.Fatalf("Drama(%s) = %+v, %v", tc.id, drama, err)
		}
		episodes, err := a.Episodes(tc.id)
		if err != nil || len(episodes) != tc.total {
			t.Fatalf("Episodes(%s): len=%d, err=%v", tc.id, len(episodes), err)
		}
		for index, episode := range episodes {
			if episode.DramaID != tc.id || episode.Index != index+1 || episode.Available != (index < tc.available) {
				t.Fatalf("episode %d: %+v", index+1, episode)
			}
		}
		ticket, err := a.ResolvePlayback(tc.id, 1)
		if err != nil || ticket.EpisodeID != episodes[0].ID || len(ticket.Sources) != 4 ||
			ticket.Sources[0].Src != "./demo/demo-2.webm" || ticket.Sources[1].Src != "./demo/demo-2.mp4" ||
			ticket.Sources[0].Quality != "hd" || ticket.Sources[1].Quality != "hd" ||
			ticket.Sources[2].Src != "./demo/demo-2-sd.webm" || ticket.Sources[3].Src != "./demo/demo-2-sd.mp4" ||
			ticket.Sources[2].Quality != "sd" || ticket.Sources[3].Quality != "sd" {
			t.Fatalf("ResolvePlayback(%s, 1) = %+v, %v", tc.id, ticket, err)
		}
		for _, index := range []int{0, tc.total + 1} {
			if _, err := a.ResolvePlayback(tc.id, index); !errors.Is(err, provider.ErrNotFound) {
				t.Fatalf("episode %d: expected ErrNotFound, got %v", index, err)
			}
		}
		if _, err := a.ResolvePlayback(tc.id, tc.available+1); !errors.Is(err, provider.ErrEpisodeUnavailable) {
			t.Fatalf("unreleased episode: %v", err)
		}
	}
	if _, err := a.Episodes("missing"); !errors.Is(err, provider.ErrNotFound) {
		t.Fatalf("missing drama: %v", err)
	}
	if _, err := app.NewWith(authorized.New()).Episodes("d1"); !errors.Is(err, provider.ErrNotAuthorized) {
		t.Fatalf("unauthorized episodes: %v", err)
	}
}

func TestDesktopHomeFeed(t *testing.T) {
	a := app.New()
	feed, err := a.HomeFeed()
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.TodayNew) != 6 || feed.TodayNew[0] != "d2" {
		t.Fatalf("unexpected new releases: %v", feed.TodayNew)
	}
	for _, ranking := range [][]string{feed.Rankings.Hot, feed.Rankings.Rise, feed.Rankings.New} {
		if len(ranking) != 5 {
			t.Fatalf("unexpected ranking length: %v", ranking)
		}
		for _, id := range ranking {
			if _, err := a.Drama(id); err != nil {
				t.Fatalf("ranking entry %s: %v", id, err)
			}
		}
	}
	if _, err := app.NewWith(authorized.New()).HomeFeed(); !errors.Is(err, provider.ErrNotAuthorized) {
		t.Fatalf("unauthorized home feed: %v", err)
	}
}
