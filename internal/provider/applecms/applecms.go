// Package applecms adapts the Apple CMS V10 JSON catalog to VideoProvider.
package applecms

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/json/jsontext"
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
	"unicode"
	"unicode/utf8"

	"jindi-drama/internal/provider"
)

const (
	maxResponseBytes = 8 << 20
	maxRecords       = 500
	maxPage          = 100000
	// MaxEpisodes bounds episode indices from an Apple CMS catalog.
	MaxEpisodes      = 1000
	maxRoutes        = 32
	maxMediaSources  = 4
	ticketLifetime   = 15 * time.Minute
	defaultRetryWait = 5 * time.Second
)

var (
	// ErrRateLimited tells callers the active CMS is cooling down.
	ErrRateLimited   = errors.New("苹果 CMS 请求过于频繁，请稍后重试")
	errUpstream      = errors.New("苹果 CMS 目录暂不可用")
	errInvalidPage   = errors.New("苹果 CMS 请求页码无效")
	errPageMismatch  = errors.New("苹果 CMS 返回的页码与请求不一致")
	errInvalidGenre  = errors.New("苹果 CMS 分类无效")
	errGenreMismatch = errors.New("苹果 CMS 返回的短剧与请求分类不一致")
	htmlTag          = regexp.MustCompile(`<[^>]*>`)
	metadataURL      = regexp.MustCompile(`(?i)https?://[^\s\p{Z}<>"，。；！？、）】》]+`)
	genreURL         = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*:)?//[^\s\p{Z}<>"，。；！？、）】》]+`)
	routeName        = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}\p{M} _+()（）·-]{0,31}$`)
	episodeLabel     = regexp.MustCompile(`^(?:([0-9]+)|第\s*([0-9]+)\s*集)$`)
)

type Provider struct {
	endpoint     url.URL
	idPrefix     string
	client       *http.Client
	media        func(string) (provider.MediaSource, bool)
	genreMu      sync.Mutex
	genres       *catalogGenreSnapshot
	genreRefresh *catalogGenreRefresh
	retryMu      sync.Mutex
	retryAt      time.Time
}

type CatalogPage struct {
	Entries []provider.Drama `json:"entries"`
	Page    int              `json:"page"`
	HasMore bool             `json:"hasMore"`
}

type CatalogDetail struct {
	Drama     provider.Drama     `json:"drama"`
	Episodes  []provider.Episode `json:"episodes"`
	GenreID   string             `json:"genreId,omitempty"`
	Cast      []string           `json:"cast,omitempty"`
	Directors []string           `json:"directors,omitempty"`
}

type CatalogGenre struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type catalogGenreSnapshot struct {
	entries []CatalogGenre
	ids     map[string]bool
}

type catalogGenreRefresh struct {
	done     chan struct{}
	snapshot *catalogGenreSnapshot
	err      error
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
	snapshot, err := p.catalogGenreList(ctx, true)
	if err != nil {
		return nil, err
	}
	return slices.Clone(snapshot.entries), nil
}

func (p *Provider) catalogGenreList(ctx context.Context, refresh bool) (*catalogGenreSnapshot, error) {
	needRefresh := refresh
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if p.coolingDown() {
			return nil, ErrRateLimited
		}
		p.genreMu.Lock()
		saved := p.genres
		pending := p.genreRefresh
		if pending != nil {
			p.genreMu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-pending.done:
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			if p.coolingDown() {
				return nil, ErrRateLimited
			}
			if errors.Is(pending.err, context.Canceled) || errors.Is(pending.err, context.DeadlineExceeded) {
				needRefresh = true
				continue // Another caller can refresh after the initiating caller leaves.
			}
		} else {
			if !needRefresh && saved != nil {
				p.genreMu.Unlock()
				return saved, nil
			}
			pending = &catalogGenreRefresh{done: make(chan struct{})}
			p.genreRefresh = pending
			p.genreMu.Unlock()
			p.completeGenreRefresh(ctx, pending)
		}
		if pending.err != nil && !refresh && saved != nil &&
			!errors.Is(pending.err, ErrRateLimited) && ctx.Err() == nil {
			return saved, nil
		}
		return pending.snapshot, pending.err
	}
}

func (p *Provider) completeGenreRefresh(ctx context.Context, pending *catalogGenreRefresh) {
	snapshot, err := p.fetchCatalogGenres(ctx)
	p.genreMu.Lock()
	defer p.genreMu.Unlock()
	if canceled := ctx.Err(); canceled != nil && !errors.Is(err, ErrRateLimited) {
		snapshot, err = nil, canceled
	}
	if err == nil {
		p.genres = snapshot
	}
	pending.snapshot, pending.err = snapshot, err
	p.genreRefresh = nil
	close(pending.done)
}

func (p *Provider) fetchCatalogGenres(ctx context.Context) (*catalogGenreSnapshot, error) {
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
		if unmarshalUniqueObject(raw, &item) != nil {
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
	return &catalogGenreSnapshot{entries: genres, ids: seen}, nil
}

func (p *Provider) requireCatalogGenre(ctx context.Context, id string) error {
	snapshot, err := p.catalogGenreList(ctx, false)
	if err != nil {
		return err
	}
	if !snapshot.ids[id] {
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
	skippedUnpublished := false
	for _, item := range data.List {
		if item.publicationStatus() != "1" {
			skippedUnpublished = true
			continue
		}
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
		HasMore: echoedPage && (len(results) > 0 || skippedUnpublished) && page < maxPage && ok && uint64(page) < pageCount,
	}, nil
}

func (p *Provider) Drama(ctx context.Context, id string) (provider.Drama, error) {
	drama, _, _, err := p.detail(ctx, id)
	return drama, err
}

func (p *Provider) Episodes(ctx context.Context, id string) ([]provider.Episode, error) {
	result, err := p.CatalogDetail(ctx, id)
	return result.Episodes, err
}

// CatalogDetail projects metadata and episode availability from the same record.
func (p *Provider) CatalogDetail(ctx context.Context, id string) (CatalogDetail, error) {
	drama, sources, raw, err := p.detail(ctx, id)
	if err != nil {
		return CatalogDetail{}, err
	}
	episodes := make([]provider.Episode, 0, drama.EpisodeCount)
	for index := 1; index <= drama.EpisodeCount; index++ {
		episodes = append(episodes, provider.Episode{
			ID: episodeID(id, index), DramaID: id, Index: index, Available: len(sources[index]) > 0,
		})
	}
	return CatalogDetail{
		Drama: drama, Episodes: episodes, GenreID: genreID(scalar(raw.TypeID).text()),
		Cast: creditNames(raw.Actor), Directors: creditNames(raw.Director),
	}, nil
}

func (p *Provider) ResolvePlayback(ctx context.Context, id string, index int) (provider.PlaybackTicket, error) {
	if index < 1 || index > MaxEpisodes {
		return provider.PlaybackTicket{}, provider.ErrEpisodeUnavailable
	}
	_, sources, _, err := p.detail(ctx, id)
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

func (p *Provider) dramaID(id string) (string, bool) {
	vodID, ok := strings.CutPrefix(id, p.idPrefix)
	return vodID, ok && decimalDigits(vodID, 30)
}

// OwnsDramaID matches an ID to the configured endpoint without contacting it.
func (p *Provider) OwnsDramaID(id string) bool {
	_, ok := p.dramaID(id)
	return ok
}

func (p *Provider) detail(ctx context.Context, id string) (provider.Drama, map[int][]provider.MediaSource, record, error) {
	vodID, ok := p.dramaID(id)
	if !ok {
		return provider.Drama{}, nil, record{}, provider.ErrNotFound
	}
	data, err := p.request(ctx, url.Values{"ac": {"detail"}, "ids": {vodID}})
	if err != nil {
		return provider.Drama{}, nil, record{}, err
	}
	items := data.List
	if len(items) != 1 || scalar(items[0].ID).decimal(30) != vodID {
		return provider.Drama{}, nil, record{}, provider.ErrNotFound
	}
	item := items[0]
	switch item.publicationStatus() {
	case "0":
		return provider.Drama{}, nil, record{}, provider.ErrNotFound
	case "1":
	default:
		return provider.Drama{}, nil, record{}, errUpstream
	}
	title := metadataText(item.Name)
	if title == "" {
		return provider.Drama{}, nil, record{}, errUpstream
	}
	drama, sources := p.normalize(item, vodID, title)
	return drama, sources, item, nil
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
	ID           json.RawMessage `json:"vod_id"`
	TypeID       json.RawMessage `json:"type_id"`
	Status       json.RawMessage `json:"vod_status"`
	Name         string          `json:"vod_name"`
	Picture      json.RawMessage `json:"vod_pic"`
	Category     string          `json:"type_name"`
	Summary      string          `json:"vod_content"`
	Actor        json.RawMessage `json:"vod_actor"`
	Director     json.RawMessage `json:"vod_director"`
	Total        json.RawMessage `json:"vod_total"`
	PlayURL      string          `json:"vod_play_url"`
	PlayFrom     json.RawMessage `json:"vod_play_from"`
	Locked       json.RawMessage `json:"locked"`
	TrialMinutes json.RawMessage `json:"vod_trysee"`
	PlayPoints   json.RawMessage `json:"vod_points_play"`
	Points       json.RawMessage `json:"vod_points"`
	PlayPass     json.RawMessage `json:"vod_pwd_play"`
	Pass         json.RawMessage `json:"vod_pwd"`
}

type response struct {
	Code      json.RawMessage `json:"code"`
	Page      json.RawMessage `json:"page"`
	PageCount json.RawMessage `json:"pagecount"`
	List      []record        `json:"list"`
	Class     json.RawMessage `json:"class"`
}

func retryWait(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil || (errors.Is(err, strconv.ErrRange) && decimalDigits(value, len(value))) {
		const maxSeconds = uint64((1<<63 - 1) / time.Second)
		return time.Duration(min(seconds, maxSeconds)) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		return max(0, date.Sub(now))
	}
	return defaultRetryWait
}

func (p *Provider) coolingDown() bool {
	return p.CooldownUntil().After(time.Now())
}

// CooldownUntil is the end of the current catalog Retry-After window.
func (p *Provider) CooldownUntil() time.Time {
	p.retryMu.Lock()
	defer p.retryMu.Unlock()
	return p.retryAt
}

func (p *Provider) extendCooldown(until time.Time) {
	p.retryMu.Lock()
	defer p.retryMu.Unlock()
	if until.After(p.retryAt) {
		p.retryAt = until
	}
}

func (p *Provider) request(ctx context.Context, params url.Values) (response, error) {
	if p.coolingDown() {
		return response{}, ErrRateLimited
	}

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
	if res.StatusCode == http.StatusTooManyRequests {
		now := time.Now()
		p.extendCooldown(now.Add(retryWait(res.Header.Get("Retry-After"), now)))
		return response{}, ErrRateLimited
	}
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
	if err := unmarshalUniqueObject(body, &wire); err != nil || scalar(wire.Code).decimal(1) != "1" {
		return response{}, errUpstream
	}
	if params.Get("ac") == "list" {
		return wire.response, nil
	}

	var rows []json.RawMessage
	if json.Unmarshal(wire.List, &rows) != nil || rows == nil || len(rows) > maxRecords {
		return response{}, errUpstream
	}
	wire.response.List = make([]record, 0, len(rows))
	detail := params.Has("ids")
	for _, row := range rows {
		var item record
		if unmarshalUniqueObject(row, &item) == nil {
			wire.response.List = append(wire.response.List, item)
		} else if detail {
			return response{}, errUpstream
		}
	}
	if len(rows) > 0 && len(wire.response.List) == 0 {
		return response{}, errUpstream
	}
	return wire.response, nil
}

// Check only this object's fields so each catalog row can be validated and
// discarded independently of the envelope and its siblings.
func unmarshalUniqueObject(data []byte, target any) error {
	decoder := jsontext.NewDecoder(bytes.NewReader(data), jsontext.AllowDuplicateNames(true), jsontext.AllowInvalidUTF8(true))
	open, err := decoder.ReadToken()
	if err != nil || open.Kind() != jsontext.KindBeginObject {
		return errUpstream
	}
	seen := make(map[string]bool)
	for decoder.PeekKind() != jsontext.KindEndObject {
		key, err := decoder.ReadToken()
		if err != nil || key.Kind() != jsontext.KindString {
			return errUpstream
		}
		// encoding/json matches struct fields using Unicode simple case folding.
		name := strings.Map(func(r rune) rune {
			for {
				next := unicode.SimpleFold(r)
				if next <= r {
					return next
				}
				r = next
			}
		}, key.String())
		if seen[name] || decoder.SkipValue() != nil {
			return errUpstream
		}
		seen[name] = true
	}
	return json.Unmarshal(data, target)
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

func creditNames(raw json.RawMessage) []string {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return nil
	}
	const maxNames = 12
	const maxNameRunes = 80
	var names []string
	for field := range strings.FieldsFuncSeq(genreText(text), func(r rune) bool {
		return strings.ContainsRune(",，、;；\r\n", r)
	}) {
		name := strings.Join(strings.Fields(field), " ")
		if name == "" || utf8.RuneCountInString(name) > maxNameRunes || slices.Contains(names, name) {
			continue
		}
		names = append(names, name)
		if len(names) == maxNames {
			break
		}
	}
	return names
}

func (item record) publicationStatus() string {
	// Some compatible endpoints omit this field for their published catalog.
	if len(item.Status) == 0 {
		return "1"
	}
	return strings.TrimSpace(scalar(item.Status).text())
}

func (item record) restricted() bool {
	switch strings.ToLower(strings.TrimSpace(scalar(item.Locked).text())) {
	case "", "false", "0":
	default:
		return true
	}
	// This provider can serve full media only, so any trial limit blocks playback.
	if trial := strings.TrimSpace(scalar(item.TrialMinutes).text()); trial != "" {
		if minutes, err := strconv.ParseUint(trial, 10, 64); err != nil || minutes != 0 {
			return true
		}
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

func playbackRouteNames(raw json.RawMessage, count int) []string {
	var names string
	if json.Unmarshal(raw, &names) != nil || names == "" {
		return nil
	}
	delimiter := "$$$"
	if !strings.Contains(names, delimiter) {
		delimiter = ","
	}
	if strings.Count(names, delimiter)+1 != count {
		return nil
	}
	labels := strings.SplitN(names, delimiter, maxRoutes+1)
	labels = labels[:min(len(labels), maxRoutes)]
	for index, name := range labels {
		label := strings.TrimSpace(name)
		labels[index] = ""
		if routeName.MatchString(label) {
			labels[index] = label
		}
	}
	return labels
}

func (p *Provider) normalize(item record, id, title string) (provider.Drama, map[int][]provider.MediaSource) {
	total, _ := strconv.Atoi(scalar(item.Total).decimal(9))
	drama := provider.Drama{
		ID: p.idPrefix + id, Title: title, Tags: []string{},
		Summary: metadataText(item.Summary), EpisodeCount: min(total, MaxEpisodes),
		CoverURL: p.coverURL(item.Picture),
	}
	if category := genreText(item.Category); category != "" {
		drama.Tags = append(drama.Tags, category)
	}

	sources := make(map[int][]provider.MediaSource)
	restricted := item.restricted()
	routes := strings.SplitN(item.PlayURL, "$$$", maxRoutes+1)
	labels := playbackRouteNames(item.PlayFrom, strings.Count(item.PlayURL, "$$$")+1)
	for routeIndex, route := range routes[:min(len(routes), maxRoutes)] {
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
			if err != nil || index < 1 || index > MaxEpisodes {
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
			if slices.ContainsFunc(sources[index], func(earlier provider.MediaSource) bool { return earlier.Src == source.Src }) {
				continue
			}
			if labels != nil {
				source.RouteName = labels[routeIndex]
			}
			sources[index] = append(sources[index], source)
			drama.UpdatedTo = max(drama.UpdatedTo, index)
		}
	}
	return drama, sources
}
