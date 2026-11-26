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
	errUpstream     = errors.New("苹果 CMS 目录暂不可用")
	errInvalidPage  = errors.New("苹果 CMS 请求页码无效")
	errPageMismatch = errors.New("苹果 CMS 返回的页码与请求不一致")
	htmlTag         = regexp.MustCompile(`<[^>]*>`)
	metadataURL     = regexp.MustCompile(`(?i)https?://[^\s\p{Z}<>"，。；！？、）】》]+`)
	episodeLabel    = regexp.MustCompile(`^(?:([0-9]+)|第\s*([0-9]+)\s*集)$`)
)

type Provider struct {
	endpoint url.URL
	idPrefix string
	client   *http.Client
	media    func(string) (provider.MediaSource, bool)
}

type CatalogPage struct {
	Entries []provider.Drama `json:"entries"`
	Page    int              `json:"page"`
	HasMore bool             `json:"hasMore"`
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
	if page < 1 || page > maxPage {
		return CatalogPage{}, errInvalidPage
	}
	data, err := p.request(ctx, strings.TrimSpace(query), "", page)
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
	data, err := p.request(ctx, "", vodID, 0)
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

type record struct {
	ID         json.RawMessage `json:"vod_id"`
	Name       string          `json:"vod_name"`
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
}

func (p *Provider) request(ctx context.Context, keyword, id string, page int) (response, error) {
	endpoint := p.endpoint
	query := endpoint.Query()
	query.Set("ac", "detail")
	query.Set("at", "json")
	if id == "" {
		query.Set("pg", strconv.Itoa(page))
		if keyword != "" {
			query.Set("wd", keyword)
		}
	} else {
		query.Set("ids", id)
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
	var data response
	if err := json.Unmarshal(body, &data); err != nil || scalar(data.Code).decimal(1) != "1" ||
		data.List == nil || len(data.List) > maxRecords {
		return response{}, errUpstream
	}
	return data, nil
}

func plainText(value string) string {
	return strings.TrimSpace(html.UnescapeString(htmlTag.ReplaceAllString(value, "")))
}

func metadataText(value string) string {
	return strings.TrimSpace(metadataURL.ReplaceAllString(plainText(value), ""))
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

func (p *Provider) normalize(item record, id, title string) (provider.Drama, map[int][]provider.MediaSource) {
	total, _ := strconv.Atoi(scalar(item.Total).decimal(9))
	drama := provider.Drama{
		ID: p.idPrefix + id, Title: title, Tags: []string{},
		Summary: metadataText(item.Summary), EpisodeCount: min(total, maxEpisodes),
	}
	if category := metadataText(item.Category); category != "" {
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
