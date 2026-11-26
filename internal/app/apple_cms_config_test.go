package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jindi-drama/internal/provider"
)

func TestAppleCMSStartupConfiguration(t *testing.T) {
	a := New()
	if err := ConfigureAppleCMS(a, ""); err != nil || a.ProviderName() != "mock" {
		t.Fatalf("missing configuration: provider=%s, err=%v", a.ProviderName(), err)
	}
	if home, err := a.Home(); err != nil || len(home) == 0 || home[0].ID != "d1" {
		t.Fatalf("default demo catalog: %+v, %v", home, err)
	}

	for _, invalid := range []string{
		"http://public.example.test/api", "https://127.0.0.1/api", "https://[::1]/api",
		"https://foo.internal/api", "https://user:secret@public.example.test/api",
		"https://public.example.test:444/api", "https://public.example.test/api#private",
		"https://public.example.test/api?token=%zz", " ",
	} {
		if err := ConfigureAppleCMS(a, invalid); err != errAppleCMSConfig || a.ProviderName() != "mock" {
			t.Errorf("invalid config %q: provider=%s, err=%v", invalid, a.ProviderName(), err)
		}
	}
	if err := ConfigureAppleCMS(a, "https://public.example.test"); err != nil || a.ProviderName() != "apple-cms" {
		t.Fatalf("valid config: provider=%s, err=%v", a.ProviderName(), err)
	}
	for _, capability := range provider.AllCapabilities {
		expected := provider.Unsupported
		if capability == "catalog" || capability == "search" || capability == "playback" {
			expected = provider.Ready
		}
		if got := a.Capabilities()[capability]; got != expected {
			t.Errorf("%s = %s, want %s", capability, got, expected)
		}
	}
}

func TestAppleCMSAppCatalogAndLoopbackPlayback(t *testing.T) {
	const upstream = "catalog.example.test"
	const mediaHost = "media.example.test"
	const mediaResponse = "synthetic WebM bytes"
	const hlsResponse = "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\nsegment.ts?token=only-go\n#EXT-X-ENDLIST\n"
	const cmsID = "90071992547409931234"
	client := newCatalogMediaClient()
	var detailCalls, mediaCalls atomic.Int32
	client.Transport = catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" {
			t.Errorf("invalid scheme: %s", req.URL.Scheme)
		}
		switch req.URL.Host {
		case upstream:
			q := req.URL.Query()
			if q.Has("ids") {
				detailCalls.Add(1)
			}
			if req.Method != http.MethodGet || req.URL.Path != "/api.php/provide/vod/" || q.Get("ac") != "detail" ||
				q.Get("at") != "json" || q.Get("access") != "synthetic-catalog-token" ||
				(q.Get("ids") == "" && q.Get("pg") != "1") ||
				(q.Get("ids") != "" && q.Get("ids") != cmsID && q.Get("ids") != "42") ||
				(q.Get("wd") != "" && q.Get("wd") != "海岛") {
				t.Errorf("unexpected CMS request: %s", req.URL)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{
				"code":1,"list":[{"vod_id":90071992547409931234,"vod_name":"CMS 海岛来信","vod_total":"4",
				"vod_pic":"https://media.example.test/thumbnail.png?signature=secret",
				"vod_play_url":"1$http://media.example.test/one.mp4#第2集$https://127.0.0.1/two.mp4#3$https://media.example.test/three.webm?signature=synthetic-media-token#4$https://media.example.test/four.m3u8"}]
			}`))}, nil
		case mediaHost:
			mediaCalls.Add(1)
			if req.URL.Path == "/four.m3u8" {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {catalogHLSType}},
					ContentLength: int64(len(hlsResponse)), Body: io.NopCloser(strings.NewReader(hlsResponse))}, nil
			}
			if req.URL.Path != "/three.webm" || req.URL.Query().Get("signature") != "synthetic-media-token" {
				t.Errorf("unexpected media request: %s", req.URL)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"video/webm"}},
				ContentLength: int64(len(mediaResponse)), Body: io.NopCloser(strings.NewReader(mediaResponse))}, nil
		default:
			return nil, errors.New("unexpected fixture request")
		}
	})
	a := New()
	if err := configureAppleCMS(a, "https://"+upstream+"?access=synthetic-catalog-token", client); err != nil {
		t.Fatal(err)
	}
	relay, err := startCatalogMediaRelay(client)
	if err != nil {
		t.Fatal(err)
	}
	a.catalogMedia = relay
	t.Cleanup(a.closeCatalogMedia)

	home, err := a.Home()
	if err != nil || len(home) != 1 || home[0].Title != "CMS 海岛来信" {
		t.Fatalf("home: %+v, %v", home, err)
	}
	id := home[0].ID
	search, err := a.CatalogSearch("海岛")
	if err != nil || len(search) != 1 || search[0].ID != id {
		t.Fatalf("search: %+v, %v", search, err)
	}
	drama, err := a.CatalogDrama(id)
	if err != nil || drama.Title != home[0].Title {
		t.Fatalf("detail: %+v, %v", drama, err)
	}
	episodes, err := a.CatalogEpisodes(id)
	if err != nil || len(episodes) != 4 || episodes[0].Available || episodes[1].Available || !episodes[2].Available || !episodes[3].Available {
		t.Fatalf("episodes: %+v, %v", episodes, err)
	}
	for _, attempt := range []struct {
		id    string
		index int
		err   error
		gets  int32
	}{
		{id, 1, provider.ErrEpisodeUnavailable, 1},
		{id, 2, provider.ErrEpisodeUnavailable, 1},
		{id, 5, provider.ErrEpisodeUnavailable, 1},
		{strings.TrimSuffix(id, cmsID) + "42", 3, errCatalogPlaybackUnavailable, 1},
		{"other-source:42", 3, errCatalogPlaybackUnavailable, 0},
		{id, 0, provider.ErrEpisodeUnavailable, 0},
	} {
		before := detailCalls.Load()
		ticket, err := a.CatalogPlayback(attempt.id, attempt.index)
		if err != attempt.err || ticket.Kind != "" || detailCalls.Load()-before != attempt.gets {
			t.Fatalf("invalid playback %s/%d: %+v, %v, %d detail GETs", attempt.id, attempt.index, ticket, err, detailCalls.Load()-before)
		}
	}
	if _, err := a.ResolvePlayback(id, 3); !errors.Is(err, provider.ErrNotAuthorized) {
		t.Fatalf("legacy playback accepted CMS ID: %v", err)
	}
	before := detailCalls.Load()
	ticket, err := a.CatalogPlayback(id, 3)
	if calls := detailCalls.Load() - before; calls != 1 {
		t.Fatalf("valid playback made %d detail GETs, want 1", calls)
	}
	if err != nil || ticket.Kind != "authorized" || ticket.LeaseID == "" || len(ticket.Sources) != 1 ||
		ticket.EpisodeID != episodes[2].ID || ticket.Sources[0].Type != "video/webm" {
		t.Fatalf("ticket: %+v, %v", ticket, err)
	}
	lease, err := url.Parse(ticket.Sources[0].Src)
	expires, expiryErr := time.Parse(time.RFC3339, ticket.ExpiresAt)
	if err != nil || lease.Scheme != "http" || lease.Hostname() != "127.0.0.1" || lease.RawQuery != "" ||
		!strings.HasPrefix(lease.Path, catalogMediaPath+ticket.LeaseID+"/") ||
		expiryErr != nil || !expires.After(time.Now()) || time.Until(expires) > 15*time.Minute {
		t.Fatalf("invalid loopback lease: %+v", ticket)
	}
	payload, err := json.Marshal([]any{home, search, drama, episodes, ticket})
	if err != nil || strings.Contains(string(payload), upstream) || strings.Contains(string(payload), mediaHost) ||
		strings.Contains(string(payload), "signature") || strings.Contains(string(payload), "synthetic-") {
		t.Fatalf("binding exposed upstream fields: %s, %v", payload, err)
	}
	if mediaCalls.Load() != 0 {
		t.Fatal("media was requested before the browser accessed the lease")
	}
	browser := &http.Client{Timeout: 5 * time.Second}
	response, err := browser.Get(lease.String())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "video/webm" ||
		string(body) != mediaResponse || mediaCalls.Load() != 1 {
		t.Fatalf("relayed response: %d, %q, %v", response.StatusCode, body, err)
	}
	ticket, err = a.CatalogPlayback(id, 4)
	if err != nil || ticket.EpisodeID != episodes[3].ID || len(ticket.Sources) != 1 ||
		ticket.Sources[0].Type != catalogHLSType ||
		!strings.HasSuffix(ticket.Sources[0].Src, catalogMediaPath+ticket.LeaseID+"/0.m3u8") {
		t.Fatalf("HLS ticket: %+v, %v", ticket, err)
	}
	response, err = browser.Get(ticket.Sources[0].Src)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err = io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != catalogHLSType ||
		!strings.Contains(string(body), catalogMediaPath+ticket.LeaseID+"/hls/0") ||
		strings.Contains(string(body), "segment.ts") || strings.Contains(string(body), "only-go") || mediaCalls.Load() != 2 {
		t.Fatalf("HLS response: %d, %q, %v", response.StatusCode, body, err)
	}

}

func TestAppleCMSClientDoesNotFollowRedirects(t *testing.T) {
	client := newCatalogMediaClient()
	var calls int
	client.Transport = catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://different.example.test/?token=secret"}},
			Body: http.NoBody}, nil
	})
	a := New()
	if err := configureAppleCMS(a, "https://public.example.test/?access=secret", client); err != nil {
		t.Fatal(err)
	}
	if rows, err := a.Home(); calls != 1 || len(rows) != 0 || err == nil || err.Error() != "苹果 CMS 目录暂不可用" {
		t.Fatalf("redirect handling: %+v, calls=%d, err=%v", rows, calls, err)
	}
}

func TestAppleCMSCatalogPageBinding(t *testing.T) {
	a := New()
	if _, err := a.CatalogPage("海岛", 2); !errors.Is(err, provider.ErrNotAuthorized) {
		t.Fatalf("expected non-CMS binding refusal, got %v", err)
	}

	client := newCatalogMediaClient()
	var calls []string
	client.Transport = catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		calls = append(calls, q.Get("pg")+":"+q.Get("wd"))
		if req.URL.Host != "catalog.example.test" || req.URL.Path != "/vod" || q.Get("token") != "fixture" ||
			q.Get("ac") != "detail" || q.Get("at") != "json" || q.Has("ids") {
			t.Errorf("unexpected CMS request: %s", req.URL)
		}
		body := `{"code":1,"page":"2","pagecount":"3","list":[{"vod_id":42,"vod_name":"岸边灯塔"}]}`
		if q.Has("wd") {
			body = `{"code":1,"page":2,"pagecount":2,"list":[{"vod_id":43,"vod_name":"海岛归航"}]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	if err := configureAppleCMS(a, "https://catalog.example.test/vod?token=fixture&pg=7&wd=old", client); err != nil {
		t.Fatal(err)
	}

	directory, err := a.CatalogPage("", 2)
	if err != nil || directory.Page != 2 || !directory.HasMore || len(directory.Entries) != 1 || directory.Entries[0].Title != "岸边灯塔" {
		t.Fatalf("directory page = %+v, %v", directory, err)
	}
	search, err := a.CatalogPage(" 海岛 ", 2)
	if err != nil || search.Page != 2 || search.HasMore || len(search.Entries) != 1 || search.Entries[0].Title != "海岛归航" {
		t.Fatalf("search page = %+v, %v", search, err)
	}
	payload, err := json.Marshal(search)
	if err != nil || strings.Contains(string(payload), "fixture") || strings.Contains(string(payload), "pagecount") {
		t.Fatalf("unexpected page payload: %s, %v", payload, err)
	}
	if _, err := a.CatalogPage("", 100001); err == nil {
		t.Fatal("expected page bound")
	}
	if got := strings.Join(calls, ","); got != "2:,2:海岛" {
		t.Fatalf("upstream queries = %s", got)
	}
}
