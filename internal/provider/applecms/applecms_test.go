package applecms

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jindi-drama/internal/provider"
)

const wideID = "90071992547409931234"

func fixtureProvider(t *testing.T, endpoint string, client *http.Client) *Provider {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	return New(*u, client, func(raw string) (provider.MediaSource, bool) {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" {
			return provider.MediaSource{}, false
		}
		if strings.HasSuffix(u.Path, ".mp4") {
			return provider.MediaSource{Src: raw, Type: "video/mp4"}, true
		}
		if strings.HasSuffix(u.Path, ".webm") {
			return provider.MediaSource{Src: raw, Type: "video/webm"}, true
		}
		return provider.MediaSource{}, false
	})
}

func TestCatalogPreservesIDsAndPositionsAcrossRoutes(t *testing.T) {
	const routes = "第 3 集$https://media.example.test/three.mp4?key=synthetic#预告$https://media.example.test/extra.mp4#1$https://media.example.test/one.m3u8$$$3$https://media.example.test/three.webm#03$https://media.example.test/three.webm#第5集$https://media.example.test/five.webm#1001$https://media.example.test/overflow.mp4"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		q := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/vod" || r.Header.Get("Accept") != "application/json" ||
			q.Get("ac") != "detail" || q.Get("at") != "json" || q.Get("token") != "qa" || q.Has("t") {
			t.Errorf("unexpected CMS request: %s %s", r.Method, r.URL)
		}
		item := fmt.Sprintf(`{"vod_id":%s,"vod_name":"<b>海岛来信</b> http://catalog.example.test/vod?access=private","type_name":"剧情 <a href='https://media.example.test/a?sig=private'>HTTPS&#58;//media.example.test/a?sig=private&amp;key=hidden</a>","vod_content":"<p>海岛 &amp; 信</p> https://catalog.example.test/vod?access=private","vod_total":"6","vod_play_url":%q}`, wideID, routes)
		if q.Has("ids") {
			if q.Get("ids") != wideID || q.Has("pg") || q.Has("wd") {
				t.Errorf("unexpected detail query: %s", r.URL)
			}
			_, _ = fmt.Fprintf(w, `{"code":"1","list":[%s]}`, item)
			return
		}
		if q.Get("pg") != "1" || (q.Has("wd") && q.Get("wd") != "海岛 & 信") {
			t.Errorf("unexpected list query: %s", r.URL)
		}
		_, _ = fmt.Fprintf(w, `{"code":1,"page":"1","list":[%s,{"vod_id":"007","vod_name":"另一部"},{"vod_id":%q,"vod_name":"重项"},{"vod_id":1e4,"vod_name":"无效编号"}]}`, item, wideID)
	}))
	t.Cleanup(server.Close)
	endpoint := server.URL + "/vod?token=qa&t=old&ids=old&ac=old&pg=2&wd=old&at=old"
	p := fixtureProvider(t, endpoint, server.Client())
	home, err := p.Home(t.Context())
	if err != nil || len(home) != 2 {
		t.Fatalf("home = %+v, %v", home, err)
	}
	drama := home[0]
	if !strings.HasSuffix(drama.ID, ":"+wideID) || !strings.HasSuffix(home[1].ID, ":007") ||
		drama.Title != "海岛来信" || drama.Summary != "海岛 & 信" || len(drama.Tags) != 1 || drama.Tags[0] != "剧情" ||
		drama.EpisodeCount != 6 || drama.UpdatedTo != 5 || drama.AllowExport || drama.IsNewToday {
		t.Fatalf("unexpected home metadata: %+v", home)
	}
	if feed, err := p.HomeFeed(t.Context()); err != nil || feed.CatalogLabel != "片源目录" || len(feed.TodayNew) != 0 || len(feed.Rankings.Hot) != 0 || requests.Load() != 1 {
		t.Fatalf("unexpected CMS feed: %+v, %v", feed, err)
	}
	search, err := p.Search(t.Context(), " 海岛 & 信 ")
	if err != nil || len(search) != 2 || search[0].ID != drama.ID {
		t.Fatalf("search = %+v, %v", search, err)
	}
	detail, err := p.Drama(t.Context(), drama.ID)
	if err != nil || detail.Title != drama.Title || detail.Summary != drama.Summary || !slices.Equal(detail.Tags, drama.Tags) {
		t.Fatalf("detail metadata = %+v, %v", detail, err)
	}
	episodes, err := p.Episodes(t.Context(), drama.ID)
	if err != nil || len(episodes) != 6 {
		t.Fatalf("episodes = %+v, %v", episodes, err)
	}
	for index, ep := range episodes {
		if ep.Index != index+1 || ep.DramaID != drama.ID || ep.Available != (ep.Index == 3 || ep.Index == 5) {
			t.Fatalf("unexpected episode at index %d: %+v", index, ep)
		}
	}
	ticket, err := p.ResolvePlayback(t.Context(), drama.ID, 3)
	expiry, expiryErr := time.Parse(time.RFC3339, ticket.ExpiresAt)
	if err != nil || ticket.Kind != "authorized" || ticket.EpisodeID != episodes[2].ID || len(ticket.Sources) != 2 ||
		ticket.Sources[0].Type != "video/mp4" || ticket.Sources[1].Type != "video/webm" ||
		expiryErr != nil || !expiry.After(time.Now()) || time.Until(expiry) > ticketLifetime {
		t.Fatalf("unexpected ticket: %+v, %v", ticket, err)
	}
	if _, err := p.ResolvePlayback(t.Context(), drama.ID, 2); !errors.Is(err, provider.ErrEpisodeUnavailable) {
		t.Fatalf("hole resolved: %v", err)
	}
	changed := fixtureProvider(t, server.URL+"/vod?token=changed", server.Client())
	if _, err = changed.Drama(t.Context(), drama.ID); !errors.Is(err, provider.ErrNotFound) || requests.Load() != 6 {
		t.Fatalf("ID crossed source boundary: %v, %d requests", err, requests.Load())
	}
}

func TestRestrictedCatalogItemsAndEpisodeBound(t *testing.T) {
	for _, restriction := range []string{`"locked":true`, `"locked":"1"`, `"vod_points":1`, `"vod_points_play":"1"`, `"vod_pwd":"password"`, `"vod_pwd_play":"password"`} {
		t.Run(restriction, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, `{"code":1,"list":[{"vod_id":"42","vod_name":"海岛","vod_total":1000000,"vod_play_url":"2$https://media.example.test/episode.mp4",%s}]}`, restriction)
			}))
			defer server.Close()
			p := fixtureProvider(t, server.URL, server.Client())
			id := p.idPrefix + "42"
			rows, err := p.Episodes(t.Context(), id)
			if err != nil || len(rows) != maxEpisodes || rows[1].Available {
				t.Fatalf("limited episode result: %d, %v", len(rows), err)
			}
			if ticket, err := p.ResolvePlayback(t.Context(), id, 2); !errors.Is(err, provider.ErrEpisodeUnavailable) || ticket.Kind != "" {
				t.Fatalf("restricted ticket escaped: %+v, %v", ticket, err)
			}
		})
	}
}

func TestUpstreamFailuresHaveFixedErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		err    error
	}{
		{"non-success", http.StatusServiceUnavailable, "secret upstream error", errUpstream},
		{"invalid JSON", http.StatusOK, "<html>secret upstream error</html>", errUpstream},
		{"trailing document", http.StatusOK, `{"code":1,"list":[]}{"secret":1}`, errUpstream},
		{"error body", http.StatusOK, `{"code":0,"msg":"secret upstream error","list":[]}`, errUpstream},
		{"missing list", http.StatusOK, `{"code":1,"list":null}`, errUpstream},
		{"other title", http.StatusOK, `{"code":1,"list":[{"vod_id":43,"vod_name":"其他剧"}]}`, provider.ErrNotFound},
		{"multiple titles", http.StatusOK, `{"code":1,"list":[{"vod_id":42,"vod_name":"同一剧"},{"vod_id":43,"vod_name":"其他剧"}]}`, provider.ErrNotFound},
		{"oversize streamed body", http.StatusOK, strings.Repeat(" ", maxResponseBytes+1), errUpstream},
		{"too many records", http.StatusOK, `{"code":1,"list":[` + strings.Repeat("{},", maxRecords) + "{}]}", errUpstream},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.(http.Flusher).Flush()
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			p := fixtureProvider(t, server.URL+"?token=hidden", server.Client())
			if drama, err := p.Drama(t.Context(), p.idPrefix+"42"); err != tt.err || drama.ID != "" {
				t.Fatalf("unexpected failure result: %+v, %v", drama, err)
			}
		})
	}
}
