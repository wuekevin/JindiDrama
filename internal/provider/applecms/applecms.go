// Package applecms adapts the Apple CMS V10 JSON catalog to VideoProvider.
package applecms

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"jindi-drama/internal/provider"
)

const (
	maxResponseBytes = 8 << 20
	maxRecords       = 500
	maxPage          = 100000
	maxEpisodes      = 1000
	maxRoutes        = 32
	maxMediaSources  = 4
	ticketLifetime   = 15 * time.Minute
)

var (
	errUpstream      = errors.New("苹果 CMS 目录暂不可用")
	errInvalidPage   = errors.New("苹果 CMS 请求页码无效")
	errPageMismatch  = errors.New("苹果 CMS 返回的页码与请求不一致")
	errInvalidGenre  = errors.New("苹果 CMS 分类无效")
	errGenreMismatch = errors.New("苹果 CMS 返回的短剧与请求分类不一致")
	htmlTag          = regexp.MustCompile(`<[^>]*>`)
	metadataURL      = regexp.MustCompile(`(?i)https?://[^\s\p{Z}<>"，。；！？、）】》]+`)
	genreURL         = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*:)?//[^\s\p{Z}<>"，。；！？、）】》]+`)
	episodeLabel     = regexp.MustCompile(`^(?:([0-9]+)|第\s*([0-9]+)\s*集)$`)
)

type Provider struct {
	endpoint url.URL
	idPrefix string
	client   *http.Client
	media    func(string) (provider.MediaSource, bool)
	genreMu  sync.Mutex
	genreIDs map[string]bool
}

type CatalogPage struct {
	Entries []provider.Drama `json:"entries"`
	Page    int              `json:"page"`
	HasMore bool             `json:"hasMore"`
}

type CatalogGenre struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var _ provider.VideoProvider = (*Provider)(nil)

// New receives the validated URL, direct HTTP client and media validator
// supplied by the app's catalog transport.
func New(endpoint url.URL, client *http.Client, media func(string) (provider.MediaSource, bool)) *Provider {
	query := endpoint.Query()
	for _, key := range []string{"ac", "at", "pg", "t", "wd", "ids"} {
		query.Del(key)
	}
	endpoint.RawQuery = query.Encode()
	endpoint.ForceQuery = false
	fingerprint := sha256.Sum256([]byte(endpoint.String()))
	return &Provider{
		endpoint: endpoint,
		idPrefix: fmt.Sprintf("apple-cms:%x:", fingerprint[:16]),
		client:   client,
		media:    media,
	}
}

func (*Provider) Name() string { return "apple-cms" }

func (*Provider) Capabilities(context.Context) provider.Capabilities {
	capabilities := make(provider.Capabilities, len(provider.AllCapabilities))
	for _, name := range provider.AllCapabilities {
		capabilities[name] = provider.Unsupported
	}
	capabilities["catalog"] = provider.Ready
	capabilities["search"] = provider.Ready
	capabilities["playback"] = provider.Ready
	return capabilities
}

func (p *Provider) Home(ctx context.Context) ([]provider.Drama, error) {
	return p.Search(ctx, "")
}

func (*Provider) HomeFeed(context.Context) (provider.HomeFeed, error) {
	return provider.HomeFeed{
		CatalogLabel: "片源目录",
		TodayNew:     []string{},
		Rankings:     provider.Rankings{Hot: []string{}, Rise: []string{}, New: []string{}},
	}, nil
}

func (p *Provider) Search(ctx context.Context, query string) ([]provider.Drama, error) {
	result, err := p.CatalogPage(ctx, query, 1)
	return result.Entries, err
}

func (p *Provider) CatalogPage(ctx context.Context, query string, page int) (CatalogPage, error) {
	return p.catalogPage(ctx, strings.TrimSpace(query), "", page)
}

func (p *Provider) CatalogGenres(ctx context.Context) ([]CatalogGenre, error) {
	p.genreMu.Lock()
	defer p.genreMu.Unlock()
	return p.refreshCatalogGenres(ctx)
}

// refreshCatalogGenres replaces the genre snapshot with genreMu held.
func (p *Provider) refreshCatalogGenres(ctx context.Context) ([]CatalogGenre, error) {
	data, err := p.request(ctx, url.Values{"ac": {"list"}, "pg": {"1"}})
	if err != nil {
		return nil, err
	}
	var classes []json.RawMessage
	if len(data.Class) > 0 && json.Unmarshal(data.Class, &classes) != nil {
		return nil, errUpstream
	}
	genres := make([]CatalogGenre, 0, min(len(classes), maxRecords))
	seen := make(map[string]bool)
	for _, raw := range classes {
		var item struct {
			ID   json.RawMessage `json:"type_id"`
			Name string          `json:"type_name"`
		}
		if json.Unmarshal(raw, &item) != nil {
			continue
		}
		id := genreID(scalar(item.ID).text())
		name := genreText(item.Name)
		if id == "" || name == "" || seen[id] {
			continue
		}
		seen[id] = true
		genres = append(genres, CatalogGenre{ID: id, Name: name})
		if len(genres) == maxRecords {
			break
		}
	}
	p.genreIDs = seen
	return genres, nil
}

func (p *Provider) requireCatalogGenre(ctx context.Context, id string) error {
	p.genreMu.Lock()
	defer p.genreMu.Unlock()
	if p.genreIDs == nil {
		if _, err := p.refreshCatalogGenres(ctx); err != nil {
			return err
		}
	}
	if !p.genreIDs[id] {
		return errInvalidGenre
	}
	return nil
}

func (p *Provider) CatalogGenrePage(ctx context.Context, genre string, page int) (CatalogPage, error) {
	id := genreID(genre)
	if id == "" {
		return CatalogPage{}, errInvalidGenre
	}
	return p.catalogPage(ctx, "", id, page)
}

func (p *Provider) catalogPage(ctx context.Context, keyword, genre string, page int) (CatalogPage, error) {
	if page < 1 || page > maxPage {
		return CatalogPage{}, errInvalidPage
	}
	query := url.Values{"ac": {"detail"}, "pg": {strconv.Itoa(page)}}
	if keyword != "" {
		query.Set("wd", keyword)
	}
	if genre != "" {
		if err := p.requireCatalogGenre(ctx, genre); err != nil {
			return CatalogPage{}, err
		}
		query.Set("t", genre)
	}
	data, err := p.request(ctx, query)
	if err != nil {
		return CatalogPage{}, err
	}
	echoedPage := len(data.Page) > 0 && string(data.Page) != "null"
	if echoedPage {
		if actual, ok := scalar(data.Page).pageNumber(); !ok || actual != uint64(page) {
			return CatalogPage{}, errPageMismatch
		}
	} else if page > 1 {
		return CatalogPage{}, errPageMismatch
	}
	results := make([]provider.Drama, 0, len(data.List))
	seen := make(map[string]bool)
	for _, item := range data.List {
		if genre != "" && genreID(scalar(item.TypeID).text()) != genre {
			return CatalogPage{}, errGenreMismatch
		}
		id := scalar(item.ID).decimal(30)
		title := metadataText(item.Name)
		if id == "" || title == "" || seen[id] {
			continue
		}
		seen[id] = true
		drama, _ := p.normalize(item, id, title)
		results = append(results, drama)
	}
	pageCount, ok := scalar(data.PageCount).pageNumber()
	return CatalogPage{
		Entries: results,
		Page:    page,
		HasMore: echoedPage && len(results) > 0 && page < maxPage && ok && uint64(page) < pageCount,
	}, nil
}

func (p *Provider) Drama(ctx context.Context, id string) (provider.Drama, error) {
	drama, _, err := p.detail(ctx, id)
	return drama, err
}

func (p *Provider) Episodes(ctx context.Context, id string) ([]provider.Episode, error) {
	drama, sources, err := p.detail(ctx, id)
	if err != nil {
		return nil, err
	}
	episodes := make([]provider.Episode, 0, drama.EpisodeCount)
	for index := 1; index <= drama.EpisodeCount; index++ {
		episodes = append(episodes, provider.Episode{
			ID: episodeID(id, index), DramaID: id, Index: index, Available: len(sources[index]) > 0,
		})
	}
	return episodes, nil
}

func (p *Provider) ResolvePlayback(ctx context.Context, id string, index int) (provider.PlaybackTicket, error) {
	if index < 1 || index > maxEpisodes {
		return provider.PlaybackTicket{}, provider.ErrEpisodeUnavailable
	}
	_, sources, err := p.detail(ctx, id)
	if err != nil {
		return provider.PlaybackTicket{}, err
	}
	if len(sources[index]) == 0 {
		return provider.PlaybackTicket{}, provider.ErrEpisodeUnavailable
	}
	return provider.PlaybackTicket{
		Kind: "authorized", EpisodeID: episodeID(id, index), Sources: sources[index],
		ExpiresAt: time.Now().Add(ticketLifetime).UTC().Format(time.RFC3339),
	}, nil
}

func episodeID(id string, index int) string { return id + ":" + strconv.Itoa(index) }

func (p *Provider) detail(ctx context.Context, id string) (provider.Drama, map[int][]provider.MediaSource, error) {
	vodID, ok := strings.CutPrefix(id, p.idPrefix)
	if !ok || !decimalDigits(vodID, 30) {
		return provider.Drama{}, nil, provider.ErrNotFound
	}
	data, err := p.request(ctx, url.Values{"ac": {"detail"}, "ids": {vodID}})
	if err != nil {
		return provider.Drama{}, nil, err
	}
	items := data.List
	if len(items) != 1 || scalar(items[0].ID).decimal(30) != vodID {
		return provider.Drama{}, nil, provider.ErrNotFound
	}
	title := metadataText(items[0].Name)
	if title == "" {
		return provider.Drama{}, nil, errUpstream
	}
	drama, sources := p.normalize(items[0], vodID, title)
	return drama, sources, nil
}

// scalar retains JSON number spelling instead of decoding IDs through float64.
type scalar json.RawMessage

func (s scalar) text() string {
	if len(s) == 0 || string(s) == "null" {
		return ""
	}
	if s[0] != '"' {
		return string(s)
	}
	var value string
	_ = json.Unmarshal(s, &value)
	return value
}

func (s scalar) decimal(limit int) string {
	value := s.text()
	if decimalDigits(value, limit) {
		return value
	}
	return ""
}

func (s scalar) pageNumber() (uint64, bool) {
	number, err := strconv.ParseUint(s.text(), 10, 64)
	return number, err == nil
}

func decimalDigits(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.IndexFunc(value, func(r rune) bool {
		return r < '0' || r > '9'
	}) < 0
}

func genreID(value string) string {
	if decimalDigits(value, 30) {
		return strings.TrimLeft(value, "0")
	}
	return ""
}

type record struct {
	ID         json.RawMessage `json:"vod_id"`
	TypeID     json.RawMessage `json:"type_id"`
	Name       string          `json:"vod_name"`
	Picture    json.RawMessage `json:"vod_pic"`
	Category   string          `json:"type_name"`
	Summary    string          `json:"vod_content"`
	Total      json.RawMessage `json:"vod_total"`
	PlayURL    string          `json:"vod_play_url"`
	Locked     json.RawMessage `json:"locked"`
	PlayPoints json.RawMessage `json:"vod_points_play"`
	Points     json.RawMessage `json:"vod_points"`
	PlayPass   json.RawMessage `json:"vod_pwd_play"`
	Pass       json.RawMessage `json:"vod_pwd"`
}

type response struct {
	Code      json.RawMessage `json:"code"`
	Page      json.RawMessage `json:"page"`
	PageCount json.RawMessage `json:"pagecount"`
	List      []record        `json:"list"`
	Class     json.RawMessage `json:"class"`
}

func (p *Provider) request(ctx context.Context, params url.Values) (response, error) {
	endpoint := p.endpoint
	query := endpoint.Query()
	query.Set("at", "json")
	for key, values := range params {
		query[key] = values
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return response{}, errUpstream
	}
	req.Header.Set("Accept", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		return response{}, errUpstream
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.ContentLength > maxResponseBytes {
		return response{}, errUpstream
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return response{}, errUpstream
	}
	var wire struct {
		response
		List json.RawMessage `json:"list"`
	}
	if err := json.Unmarshal(body, &wire); err != nil || scalar(wire.Code).decimal(1) != "1" {
		return response{}, errUpstream
	}
	if params.Get("ac") != "list" {
		if json.Unmarshal(wire.List, &wire.response.List) != nil ||
			wire.response.List == nil || len(wire.response.List) > maxRecords {
			return response{}, errUpstream
		}
	}
	return wire.response, nil
}

func plainText(value string) string {
	return strings.TrimSpace(html.UnescapeString(htmlTag.ReplaceAllString(value, "")))
}

func metadataText(value string) string {
	return strings.TrimSpace(metadataURL.ReplaceAllString(plainText(value), ""))
}

func genreText(value string) string {
	// Strip tags decoded from HTML entities and URLs split by markup.
	return strings.TrimSpace(genreURL.ReplaceAllString(htmlTag.ReplaceAllString(metadataText(value), ""), ""))
}

func (item record) restricted() bool {
	switch strings.ToLower(strings.TrimSpace(scalar(item.Locked).text())) {
	case "", "false", "0":
	default:
		return true
	}
	for _, value := range []json.RawMessage{item.PlayPoints, item.Points} {
		text := strings.TrimSpace(scalar(value).text())
		if text != "" {
			points, err := strconv.ParseFloat(text, 64)
			if err != nil || !(points <= 0) {
				return true
			}
		}
	}
	return strings.TrimSpace(scalar(item.PlayPass).text()) != "" || strings.TrimSpace(scalar(item.Pass).text()) != ""
}

func (p *Provider) coverURL(picture json.RawMessage) string {
	var value string
	if json.Unmarshal(picture, &value) != nil {
		return ""
	}
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 4096 || strings.ContainsAny(value, "#\\\r\n\t ") {
		return ""
	}
	ref, err := url.Parse(value)
	if err != nil || (!ref.IsAbs() && ref.Host == "" && ref.Path == "" && ref.RawQuery == "") {
		return ""
	}
	return p.endpoint.ResolveReference(ref).String()
}

func (p *Provider) normalize(item record, id, title string) (provider.Drama, map[int][]provider.MediaSource) {
	total, _ := strconv.Atoi(scalar(item.Total).decimal(9))
	drama := provider.Drama{
		ID: p.idPrefix + id, Title: title, Tags: []string{},
		Summary: metadataText(item.Summary), EpisodeCount: min(total, maxEpisodes),
		CoverURL: p.coverURL(item.Picture),
	}
	if category := genreText(item.Category); category != "" {
		drama.Tags = append(drama.Tags, category)
	}

	sources := make(map[int][]provider.MediaSource)
	restricted := item.restricted()
	routes := strings.SplitN(item.PlayURL, "$$$", maxRoutes+1)
	for _, route := range routes[:min(len(routes), maxRoutes)] {
		for entry := range strings.SplitSeq(route, "#") {
			label, rawURL, ok := strings.Cut(entry, "$")
			if !ok {
				continue
			}
			match := episodeLabel.FindStringSubmatch(plainText(label))
			if match == nil {
				continue
			}
			index, err := strconv.Atoi(match[1] + match[2])
			if err != nil || index < 1 || index > maxEpisodes {
				continue
			}
			drama.EpisodeCount = max(drama.EpisodeCount, index)
			if restricted || len(sources[index]) >= maxMediaSources {
				continue
			}
			source, allowed := p.media(strings.TrimSpace(rawURL))
			if !allowed {
				continue
			}
			if !slices.Contains(sources[index], source) {
				sources[index] = append(sources[index], source)
				drama.UpdatedTo = max(drama.UpdatedTo, index)
			}
		}
	}
	return drama, sources
}
