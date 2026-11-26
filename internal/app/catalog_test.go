package app

import (
	"context"
	"errors"
	"testing"

	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/licensed"
)

type catalogTestProvider struct {
	provider.VideoProvider
	capabilities provider.Capabilities
	calls        int
}

func (p *catalogTestProvider) Capabilities(context.Context) provider.Capabilities {
	return p.capabilities
}
func (p *catalogTestProvider) Drama(_ context.Context, id string) (provider.Drama, error) {
	p.calls++
	return provider.Drama{ID: id, Title: "主目录剧目"}, nil
}

func (p *catalogTestProvider) Episodes(_ context.Context, id string) ([]provider.Episode, error) {
	p.calls++
	return []provider.Episode{{DramaID: id, Index: 1}}, nil
}

func (p *catalogTestProvider) Search(context.Context, string) ([]provider.Drama, error) {
	p.calls++
	return []provider.Drama{{ID: "local-foreign"}}, nil
}

func TestCatalogBindingsPreserveProviderIdentityAndCapabilities(t *testing.T) {
	p := &catalogTestProvider{capabilities: provider.Capabilities{"catalog": provider.Ready}}
	a := NewWith(p)
	ConfigureLicensedMedia(a, &licensed.DemoProvider{}, nil)
	id := "local-foreign"
	if _, err := a.Drama(id); !errors.Is(err, provider.ErrNotFound) {
		t.Fatalf("legacy local path: %v", err)
	}
	if d, err := a.CatalogDrama(id); err != nil || d.ID != id || d.Title != "主目录剧目" {
		t.Fatalf("primary drama: %+v, %v", d, err)
	}
	if eps, err := a.CatalogEpisodes(id); err != nil || len(eps) != 1 || eps[0].DramaID != id {
		t.Fatalf("primary episodes: %+v, %v", eps, err)
	}
	if _, err := a.CatalogSearch("剧目"); !errors.Is(err, provider.ErrNotAuthorized) || p.calls != 2 {
		t.Fatalf("denied search: calls=%d, err=%v", p.calls, err)
	}
	p.capabilities = provider.Capabilities{"search": provider.Ready}
	if matches, err := a.CatalogSearch("剧目"); err != nil || len(matches) != 1 || matches[0].ID != id {
		t.Fatalf("primary search: %+v, %v", matches, err)
	}
	if _, err := a.CatalogDrama(id); !errors.Is(err, provider.ErrNotAuthorized) || p.calls != 3 {
		t.Fatalf("denied detail: calls=%d, err=%v", p.calls, err)
	}
}
