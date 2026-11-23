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

func TestLicensedDemoNotReadyUntilImplemented(t *testing.T) {
	for _, dir := range []string{"", "/tmp/media"} {
		for k, v := range licensed.New(dir).Capabilities(context.Background()) {
			if v == provider.Ready {
				t.Fatalf("%s 不应在 P1 前为 ready", k)
			}
		}
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
