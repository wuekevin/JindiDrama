package app

import (
	"context"
	"errors"
	"mime"
	"net/url"
	"path"
	"strings"
	"time"

	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/applecms"
)

var errCatalogPlaybackUnavailable = errors.New("授权播放源暂不可用")

// CatalogPlayback resolves only active-catalog media. Demo and configured local
// media continue to use their existing bindings and storage.
func (a *App) CatalogPlayback(id string, index int) (provider.PlaybackTicket, error) {
	ctx, cancel := context.WithTimeout(a.context(), 15*time.Second)
	defer cancel()
	capabilities := a.p.Capabilities(ctx)
	if capabilities["catalog"] != provider.Ready || capabilities["playback"] != provider.Ready {
		return provider.PlaybackTicket{}, provider.ErrNotAuthorized
	}
	if id == "" || index < 1 {
		return provider.PlaybackTicket{}, provider.ErrEpisodeUnavailable
	}

	if cms, ok := a.p.(*applecms.Provider); ok {
		// CMS resolves eligibility, episode ID and sources from one exact detail.
		ticket, err := cms.ResolvePlayback(ctx, id, index)
		if errors.Is(err, applecms.ErrRateLimited) {
			return provider.PlaybackTicket{}, applecms.ErrRateLimited
		}
		if errors.Is(err, provider.ErrEpisodeUnavailable) {
			return provider.PlaybackTicket{}, provider.ErrEpisodeUnavailable
		}
		return a.issueCatalogPlaybackTicket(ctx, ticket, ticket.EpisodeID, true, err)
	}

	drama, err := a.p.Drama(ctx, id)
	if err != nil {
		return provider.PlaybackTicket{}, errCatalogPlaybackUnavailable
	}
	if drama.ID != id || index > drama.EpisodeCount || index > drama.UpdatedTo {
		return provider.PlaybackTicket{}, provider.ErrEpisodeUnavailable
	}
	episodes, err := a.p.Episodes(ctx, id)
	if err != nil {
		return provider.PlaybackTicket{}, errCatalogPlaybackUnavailable
	}
	var episodeID string
	for _, ep := range episodes {
		if ep.Index != index || ep.DramaID != id {
			continue
		}
		if !ep.Available || ep.ID == "" || episodeID != "" {
			return provider.PlaybackTicket{}, provider.ErrEpisodeUnavailable
		}
		episodeID = ep.ID
	}
	if episodeID == "" {
		return provider.PlaybackTicket{}, provider.ErrEpisodeUnavailable
	}

	ticket, err := a.p.ResolvePlayback(ctx, id, index)
	return a.issueCatalogPlaybackTicket(ctx, ticket, episodeID, false, err)
}

func (a *App) issueCatalogPlaybackTicket(ctx context.Context, ticket provider.PlaybackTicket, episodeID string, allowHLS bool, err error) (provider.PlaybackTicket, error) {
	if err != nil || ctx.Err() != nil || ticket.Kind != "authorized" || episodeID == "" || ticket.EpisodeID != episodeID {
		return provider.PlaybackTicket{}, errCatalogPlaybackUnavailable
	}
	expiry, err := time.Parse(time.RFC3339, ticket.ExpiresAt)
	if err != nil || !expiry.After(time.Now()) {
		return provider.PlaybackTicket{}, errCatalogPlaybackUnavailable
	}
	if len(ticket.Restrictions) != 0 {
		return provider.PlaybackTicket{}, provider.ErrUnsupported
	}

	sources := make([]provider.MediaSource, 0, len(ticket.Sources))
	seen := make(map[string]bool)
	for _, source := range ticket.Sources {
		if candidate, ok := catalogMediaSource(source, false, allowHLS); ok && !seen[candidate.Src] {
			sources = append(sources, candidate)
			seen[candidate.Src] = true
		}
	}
	if len(sources) == 0 {
		if candidate, ok := catalogMediaSource(provider.MediaSource{Src: ticket.PlaybackURL}, true, allowHLS); ok {
			sources = append(sources, candidate)
		}
	}
	if len(sources) == 0 {
		return provider.PlaybackTicket{}, errCatalogPlaybackUnavailable
	}
	return a.issueCatalogMedia(provider.PlaybackTicket{
		Kind: ticket.Kind, EpisodeID: episodeID, Sources: sources, ExpiresAt: ticket.ExpiresAt,
	}, expiry)
}

func catalogMediaSource(source provider.MediaSource, inferType, allowHLS bool) (provider.MediaSource, bool) {
	u, ok := catalogHTTPSURL(source.Src)
	if !ok {
		return provider.MediaSource{}, false
	}
	mediaType, _, _ := mime.ParseMediaType(source.Type)
	extension := strings.ToLower(path.Ext(u.Path))
	if inferType {
		switch extension {
		case ".mp4":
			mediaType = "video/mp4"
		case ".webm":
			mediaType = "video/webm"
		case ".m3u8":
			mediaType = catalogHLSType
		}
	}
	switch mediaType {
	case "video/mp4", "video/webm":
	case catalogHLSType:
		if !allowHLS || extension != ".m3u8" || len(source.Src) > maxCatalogHLSURL {
			return provider.MediaSource{}, false
		}
	default:
		return provider.MediaSource{}, false
	}
	if source.Quality != "sd" && source.Quality != "hd" {
		source.Quality = ""
	}
	source.Type = mediaType
	return source, true
}

func catalogHTTPSURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.User != nil || u.Fragment != "" ||
		(u.Port() != "" && u.Port() != "443") ||
		strings.ContainsAny(raw, "\\\r\n\t ") || !catalogMediaHostname(u.Hostname()) {
		return nil, false
	}
	return u, true
}

func catalogMediaHostname(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return false
	}
	last := labels[len(labels)-1]
	switch last {
	case "", "localhost", "local", "internal", "lan", "home":
		return false
	}
	// Reject literal and browser-normalized numeric IP forms.
	if last[0] < 'a' || last[0] > 'z' {
		return false
	}
	for _, label := range labels {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, ch := range label {
			if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
				return false
			}
		}
	}
	return true
}
