// Package reference reads the catalog contracts exposed by the reference apps.
package reference

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"jindi-drama/internal/provider"
)

const (
	OpenPlayer  = "hongguo-open-player"
	DramaServer = "hongguo-drama-server"
	maxResponse = 2 << 20
	maxEpisodes = 1000
)

var (
	numericID      = regexp.MustCompile(`^[0-9]{1,30}$`)
	errConfig      = errors.New("参考片源配置无效")
	errUnavailable = errors.New("参考片源接口暂不可用")
	errResponse    = errors.New("参考片源返回了无效的目录数据")
)

type Provider struct {
	name     string
	base     *url.URL
	token    string
	client   *http.Client
	mu       sync.Mutex
	inflight map[string]*request
}

type request struct {
	done chan struct{}
	data []byte
	err  error
}

func New(name, address, token string) (*Provider, error) {
	if (name != OpenPlayer && name != DramaServer) || (name == OpenPlayer && token != "") ||
		(name == DramaServer && (len(token) < 32 || len(token) > 256 || strings.ContainsFunc(token, func(r rune) bool { return r <= ' ' || r >= 127 }))) {
		return nil, errConfig
	}
	base, err := url.Parse(address)
	if err != nil || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" ||
		strings.ContainsAny(address, "\\ \r\n\t#") || (base.Scheme != "https" && base.Scheme != "http") {
		return nil, errConfig
	}
	if base.Scheme == "http" {
		ip, err := netip.ParseAddr(base.Hostname())
		if err != nil || !ip.IsLoopback() {
			return nil, errConfig
		}
	}
	var jar http.CookieJar
	if name == OpenPlayer {
		if base.Scheme != "http" || base.Hostname() != "127.0.0.1" ||
			base.Port() == "" || (base.Path != "" && base.Path != "/") {
			return nil, errConfig
		}
		sessionCookies, err := cookiejar.New(nil)
		if err != nil {
			return nil, errConfig
		}
		jar = sessionCookies
	}
	return &Provider{name: name, base: base, token: token, inflight: make(map[string]*request), client: &http.Client{
		Jar:           jar,
		Transport:     &http.Transport{ForceAttemptHTTP2: true, IdleConnTimeout: 30 * time.Second},
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) Capabilities(context.Context) provider.Capabilities {
	caps := make(provider.Capabilities, len(provider.AllCapabilities))
	for _, name := range provider.AllCapabilities {
		caps[name] = provider.Unsupported
	}
	caps["catalog"] = provider.Ready
	caps["search"] = provider.Ready
	return caps
}

type series struct {
	ID           string    `json:"series_id"`
	Title        string    `json:"series_title"`
	Total        int       `json:"total"`
	EpisodeCount int       `json:"episode_count"`
	Intro        string    `json:"intro"`
	Description  string    `json:"description"`
	Tags         []string  `json:"tags"`
	Episodes     []episode `json:"episodes"`
}

type episode struct {
	ID       string `json:"vid"`
	Index    int    `json:"vid_index"`
	SeriesID string `json:"series_id"`
	Locked   bool   `json:"locked"`
}

type seriesList struct {
	Success *bool           `json:"success"`
	Results []series        `json:"results"`
	Error   json.RawMessage `json:"error"`
}

func (row series) drama() provider.Drama {
	summary := row.Intro
	if summary == "" {
		summary = row.Description
	}
	return provider.Drama{
		ID: row.ID, Title: strings.TrimSpace(row.Title), Summary: summary,
		Tags: append([]string{}, row.Tags...), EpisodeCount: max(row.Total, row.EpisodeCount),
	}
}

func (p *Provider) Home(ctx context.Context) ([]provider.Drama, error) {
	if p.name == DramaServer {
		return []provider.Drama{}, nil // This server exposes search and series, without a browse route.
	}
	return p.list(ctx, "catalog", url.Values{"page": {"1"}})
}

func (p *Provider) HomeFeed(ctx context.Context) (provider.HomeFeed, error) {
	rows, err := p.Home(ctx)
	if err != nil {
		return provider.HomeFeed{}, err
	}
	feed := provider.HomeFeed{TodayNew: []string{}, Rankings: provider.Rankings{Hot: []string{}, Rise: []string{}, New: []string{}}}
	for _, row := range rows {
		feed.Rankings.Hot = append(feed.Rankings.Hot, row.ID)
	}
	return feed, nil
}

func (p *Provider) Search(ctx context.Context, query string) ([]provider.Drama, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return p.Home(ctx)
	}
	key, maxLength := "keyword", 80
	if p.name == DramaServer {
		key, maxLength = "q", 200
	}
	if len(utf16.Encode([]rune(query))) > maxLength {
		return nil, errors.New("搜索词过长")
	}
	return p.list(ctx, "search", url.Values{key: {query}, "page": {"1"}})
}

func (p *Provider) list(ctx context.Context, route string, query url.Values) ([]provider.Drama, error) {
	var payload seriesList
	if err := p.get(ctx, query, &payload, route); err != nil {
		return nil, err
	}
	if payload.Results == nil || (payload.Success != nil && !*payload.Success) ||
		(p.name == DramaServer && payload.Success == nil) || (len(payload.Error) > 0 && string(payload.Error) != "null") {
		return nil, errResponse
	}
	rows := make([]provider.Drama, 0, len(payload.Results))
	seen := make(map[string]bool)
	for _, item := range payload.Results {
		row := item.drama()
		if numericID.MatchString(row.ID) && row.Title != "" && row.EpisodeCount >= 0 && row.EpisodeCount <= maxEpisodes && !seen[row.ID] {
			rows = append(rows, row)
			seen[row.ID] = true
		}
	}
	return rows, nil
}

func (p *Provider) detail(ctx context.Context, id string) (provider.Drama, []provider.Episode, error) {
	var empty provider.Drama
	if !numericID.MatchString(id) {
		return empty, nil, provider.ErrNotFound
	}
	var item series
	if p.name == DramaServer {
		var payload struct {
			Success bool   `json:"success"`
			Series  series `json:"series"`
		}
		if err := p.get(ctx, nil, &payload, "series", id); err != nil {
			return empty, nil, err
		}
		if !payload.Success {
			return empty, nil, errResponse
		}
		item = payload.Series
	} else if err := p.get(ctx, nil, &item, "series", id); err != nil {
		return empty, nil, err
	}
	row := item.drama()
	if row.ID != id || row.Title == "" || row.EpisodeCount < 0 || row.EpisodeCount > maxEpisodes || len(item.Episodes) > maxEpisodes || item.Episodes == nil {
		return empty, nil, errResponse
	}
	episodes := make([]provider.Episode, 0, len(item.Episodes))
	seenID, seenIndex := make(map[string]bool), make(map[int]bool)
	for _, ep := range item.Episodes {
		if ep.Index < 1 || ep.Index > maxEpisodes || !numericID.MatchString(ep.ID) || (ep.SeriesID != "" && ep.SeriesID != id) ||
			seenID[ep.ID] || seenIndex[ep.Index] {
			return empty, nil, errResponse
		}
		seenID[ep.ID], seenIndex[ep.Index] = true, true
		episodes = append(episodes, provider.Episode{ID: ep.ID, DramaID: id, Index: ep.Index, Available: !ep.Locked})
		row.EpisodeCount = max(row.EpisodeCount, ep.Index)
		row.UpdatedTo = max(row.UpdatedTo, ep.Index)
	}
	slices.SortFunc(episodes, func(a, b provider.Episode) int { return a.Index - b.Index })
	return row, episodes, nil
}

func (p *Provider) Drama(ctx context.Context, id string) (provider.Drama, error) {
	row, _, err := p.detail(ctx, id)
	return row, err
}

func (p *Provider) Episodes(ctx context.Context, id string) ([]provider.Episode, error) {
	_, episodes, err := p.detail(ctx, id)
	return episodes, err
}

func (*Provider) ResolvePlayback(context.Context, string, int) (provider.PlaybackTicket, error) {
	return provider.PlaybackTicket{}, provider.ErrUnsupported
}

func (p *Provider) get(ctx context.Context, query url.Values, dest any, parts ...string) error {
	endpoint := p.base.JoinPath(append([]string{"api"}, parts...)...)
	endpoint.RawQuery = query.Encode()
	key := endpoint.String()

	p.mu.Lock()
	current, exists := p.inflight[key]
	if !exists {
		current = &request{done: make(chan struct{})}
		p.inflight[key] = current
	}
	p.mu.Unlock()
	if exists {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-current.done:
		}
	} else {
		current.data, current.err = p.load(ctx, key)
		p.mu.Lock()
		delete(p.inflight, key)
		close(current.done)
		p.mu.Unlock()
	}
	if current.err != nil {
		return current.err
	}
	if json.Unmarshal(current.data, dest) != nil {
		return errResponse
	}
	return nil
}

func (p *Provider) load(ctx context.Context, endpoint string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	res, err := p.visit(ctx, endpoint)
	if err == nil && res.StatusCode == http.StatusUnauthorized && p.name == OpenPlayer {
		res.Body.Close()
		// Open Player issues its normal same-origin session cookie at the local homepage.
		home, homeErr := p.visit(ctx, p.base.JoinPath("/").String())
		if homeErr != nil {
			return nil, homeErr
		}
		home.Body.Close()
		if home.StatusCode != http.StatusOK {
			return nil, errUnavailable
		}
		res, err = p.visit(ctx, endpoint)
	}
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, provider.ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, errUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponse+1))
	if err != nil || len(data) > maxResponse {
		return nil, errResponse
	}
	return data, nil
}

func (p *Provider) visit(ctx context.Context, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errUnavailable
	}
	req.Header.Set("Accept", "application/json")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	res, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errUnavailable
	}
	return res, nil
}

var _ provider.VideoProvider = (*Provider)(nil)
