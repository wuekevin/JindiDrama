package app

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/applecms"
)

var errAppleCMSConfig = errors.New("苹果 CMS 配置无效，继续使用演示目录")

// ConfigureAppleCMS is called before the desktop host registers the bindings.
func ConfigureAppleCMS(a *App, raw string) error {
	if raw == "" {
		return nil
	}
	client := newCatalogMediaClient()
	client.Timeout = 12 * time.Second
	return configureAppleCMS(a, raw, client)
}

func configureAppleCMS(a *App, raw string, client *http.Client) error {
	raw = strings.TrimSpace(raw)
	u, ok := catalogHTTPSURL(raw)
	if !ok || len(raw) > 4096 || strings.Contains(raw, "#") {
		return errAppleCMSConfig
	}
	if _, err := url.ParseQuery(u.RawQuery); err != nil {
		return errAppleCMSConfig
	}
	u.Host = strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if u.Path == "" || u.Path == "/" {
		u.Path = "/api.php/provide/vod/"
		u.RawPath = ""
	}
	a.p = applecms.New(*u, client, func(raw string) (provider.MediaSource, bool) {
		return catalogMediaSource(provider.MediaSource{Src: raw}, true, true)
	})
	return nil
}

// CatalogPage addresses only the configured Apple CMS provider.
func (a *App) CatalogPage(query string, page int) (applecms.CatalogPage, error) {
	cms, ok := a.p.(*applecms.Provider)
	if !ok {
		return applecms.CatalogPage{}, provider.ErrNotAuthorized
	}
	return cms.CatalogPage(a.context(), query, page)
}

func (a *App) CatalogGenres() ([]applecms.CatalogGenre, error) {
	ctx := a.context()
	cms, ok := a.p.(*applecms.Provider)
	if !ok || cms.Capabilities(ctx)["catalog"] != provider.Ready {
		return nil, provider.ErrNotAuthorized
	}
	return cms.CatalogGenres(ctx)
}

func (a *App) CatalogGenrePage(genre string, page int) (applecms.CatalogPage, error) {
	ctx := a.context()
	cms, ok := a.p.(*applecms.Provider)
	if !ok || cms.Capabilities(ctx)["catalog"] != provider.Ready {
		return applecms.CatalogPage{}, provider.ErrNotAuthorized
	}
	return cms.CatalogGenrePage(ctx, genre, page)
}
