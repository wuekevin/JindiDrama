package registry

import (
	"context"
	"errors"
	"testing"

	"jindi-drama/internal/provider"
)

func TestAllRegisteredProviders(t *testing.T) {
	for _, n := range Names() {
		p, err := New(n, Options{})
		if err != nil || p == nil {
			t.Fatalf("%s: %v", n, err)
		}
		if len(p.Capabilities(context.Background())) != len(provider.AllCapabilities) {
			t.Errorf("%s: 能力项数量不符", n)
		}
	}
}

func TestHongguoProvidersStayDisabled(t *testing.T) {
	ctx := context.Background()
	for _, n := range []string{HongguoWeb, HongguoApp} {
		p, _ := New(n, Options{})
		for k, st := range p.Capabilities(ctx) {
			if st != provider.NotAuthorized {
				t.Errorf("%s.%s=%s，未授权前必须是 not_authorized", n, k, st)
			}
		}
		if _, err := p.Home(ctx); !errors.Is(err, provider.ErrNotAuthorized) {
			t.Errorf("%s.Home 应返回 ErrNotAuthorized: %v", n, err)
		}
		if _, err := p.ResolvePlayback(ctx, "d1", 1); !errors.Is(err, provider.ErrNotAuthorized) {
			t.Errorf("%s.ResolvePlayback 应返回 ErrNotAuthorized: %v", n, err)
		}
	}
}

func TestUnknownFallsBackToMock(t *testing.T) {
	if _, err := New("nope", Options{}); err == nil {
		t.Error("未知名称应报错")
	}
	if NewOrDefault("nope", Options{}).Name() != "mock" && NewOrDefault("nope", Options{}).Name() == "" {
		t.Error("回退失败")
	}
	if p := NewOrDefault("nope", Options{}); p.Capabilities(context.Background())["catalog"] != provider.Mock {
		t.Error("回退应为 Mock")
	}
}
