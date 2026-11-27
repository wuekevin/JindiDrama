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

func TestRouteNamesKeepSourcePositionsAndIgnoreInformationalErrors(t *testing.T) {
	p := fixtureProvider(t, "https://catalog.example.test/vod", http.DefaultClient)
	const routes = "1$https://media.example.test/main.mp4$$$1$http://media.example.test/blocked.mp4$$$1$https://media.example.test/main.mp4#1$https://media.example.test/backup.webm"
	for _, test := range []struct {
		name, metadata string
		want           []string
	}{
		{"parallel groups", `" 主线路 $$$不可播$$$备用线路_2 "`, []string{"主线路", "备用线路_2"}},
		{"filtered JSON", `"主线路（备用）,不可播,CDN A+B"`, []string{"主线路（备用）", "CDN A+B"}},
		{"combining accent", `"Cafe\u0301$$$不可播$$$备用线路"`, []string{"Cafe\u0301", "备用线路"}},
		{"mismatched", `"主线路$$$备用线路"`, []string{"", ""}},
		{"invalid names", `"https://example.test?token=secret$$$不可播$$$<b>备用</b>"`, []string{"", ""}},
		{"wrong type", `{"name":"主线路"}`, []string{"", ""}},
		{"missing", "", []string{"", ""}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, episodes := p.normalize(record{PlayURL: routes, PlayFrom: []byte(test.metadata)}, "1", "片源")
			sources := episodes[1]
			if len(sources) != 2 || sources[0].RouteName != test.want[0] || sources[1].RouteName != test.want[1] ||
				sources[0].Quality != "" || sources[1].Quality != "" {
				t.Fatalf("unexpected playback sources: %+v", sources)
			}
		})
	}
}

func TestRouteNamesRespectMediaGroupLimit(t *testing.T) {
	const primary = "主线路"
	raw := []byte(`"` + primary + strings.Repeat("$$$备用线路", maxRoutes+2) + `"`)
	names := playbackRouteNames(raw, maxRoutes+3)
	if len(names) != maxRoutes || names[0] != primary || names[maxRoutes-1] != "备用线路" {
		t.Fatalf("retained names: %q", names)
	}
	if names := playbackRouteNames(raw, maxRoutes+4); names != nil {
		t.Fatalf("mismatched names: %q", names)
	}
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
		item := fmt.Sprintf(`{"vod_id":%s,"vod_name":"<b>海岛来信</b> http://catalog.example.test/vod?access=private","type_id":"007","vod_actor":"<b>林舟</b>，雨岚、林舟; &lt;i&gt;Anne  Marie&lt;/i&gt;，https://example.test/a?key=private","vod_director":"顾远、顾远","type_name":"剧情 <a href='https://media.example.test/a?sig=private'>HTTPS&#58;//media.example.test/a?sig=private&amp;key=hidden</a>","vod_content":"<p>海岛 &amp; 信</p> https://catalog.example.test/vod?access=private","vod_total":"6","vod_play_url":%q}`, wideID, routes)
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
	combined, err := p.CatalogDetail(t.Context(), drama.ID)
	if err != nil || combined.Drama.ID != drama.ID || combined.GenreID != "7" || len(combined.Episodes) != 6 {
		t.Fatalf("combined detail = %+v, %v", combined, err)
	}
	if !slices.Equal(combined.Cast, []string{"林舟", "雨岚", "Anne Marie"}) ||
		!slices.Equal(combined.Directors, []string{"顾远"}) {
		t.Fatalf("unexpected credits: %+v, %+v", combined.Cast, combined.Directors)
	}
	episodes := combined.Episodes
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

func TestCreditNamesRejectsInvalidFieldsAndBoundsEntries(t *testing.T) {
	for _, raw := range []string{"", "null", "42", `["林舟"]`, `{"name":"林舟"}`} {
		if names := creditNames([]byte(raw)); len(names) != 0 {
			t.Errorf("unexpected names for %q: %v", raw, names)
		}
	}
	entries := []string{strings.Repeat("名", 81), "//example.test/credit", ""}
	for i := 1; i <= 14; i++ {
		entries = append(entries, fmt.Sprintf("主演%d", i))
	}
	data := []byte(fmt.Sprintf("%q", strings.Join(entries, "、")))
	if names := creditNames(data); len(names) != 12 || names[0] != "主演1" || names[11] != "主演12" {
		t.Fatalf("unexpected bounded names: %v", names)
	}
}

func TestRestrictedCatalogItemsAndEpisodeBound(t *testing.T) {
	for _, restriction := range []string{
		`"locked":true`, `"locked":"1"`, `"vod_points":1`, `"vod_points_play":"1"`, `"vod_pwd":"password"`, `"vod_pwd_play":"password"`,
		`"vod_trysee":2`, `"vod_trysee":" 2 "`, `"vod_trysee":"unknown"`, `"vod_trysee":-1`, `"vod_trysee":{}`,
	} {
		t.Run(restriction, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, `{"code":1,"list":[{"vod_id":"42","vod_name":"海岛","vod_total":1000000,"vod_play_url":"2$https://media.example.test/episode.mp4",%s}]}`, restriction)
			}))
			defer server.Close()
			p := fixtureProvider(t, server.URL, server.Client())
			id := p.idPrefix + "42"
			rows, err := p.Episodes(t.Context(), id)
			if err != nil || len(rows) != MaxEpisodes || rows[1].Available {
				t.Fatalf("limited episode result: %d, %v", len(rows), err)
			}
			if ticket, err := p.ResolvePlayback(t.Context(), id, 2); !errors.Is(err, provider.ErrEpisodeUnavailable) || ticket.Kind != "" {
				t.Fatalf("restricted ticket escaped: %+v, %v", ticket, err)
			}
		})
	}
}

func TestCMSAllowsPlaybackWithoutATrialLimit(t *testing.T) {
	for _, value := range []string{`null`, `""`, `0`, `" 0 "`} {
		t.Run(value, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w, `{"code":1,"list":[{"vod_id":"42","vod_name":"海岛","vod_trysee":%s,"vod_play_url":"1$https://media.example.test/episode.mp4"}]}`, value)
			}))
			defer server.Close()
			p := fixtureProvider(t, server.URL, server.Client())
			ticket, err := p.ResolvePlayback(t.Context(), p.idPrefix+"42", 1)
			if err != nil || len(ticket.Sources) != 1 {
				t.Fatalf("no trial restriction expected: %+v, %v", ticket, err)
			}
		})
	}
}

func TestCatalogShowsOnlyPublishedTitles(t *testing.T) {
	const current = `{"vod_id":42,"vod_name":"海岛来信","type_id":7,"vod_status":1}`
	const legacy = `{"vod_id":43,"vod_name":"旧版目录","type_id":7}`
	const stringStatus = `{"vod_id":44,"vod_name":"旧版状态","type_id":7,"vod_status":"1"}`
	const unavailable = `{"vod_id":41,"vod_name":"不可见剧目","type_id":8,"vod_status":0,"vod_play_url":"1$https://media.example.test/private.mp4"}`
	const invalid = `{"vod_id":40,"vod_name":"无效状态","type_id":7,"vod_status":null}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch {
		case query.Get("ac") == "list":
			_, _ = io.WriteString(w, `{"code":1,"class":[{"type_id":7,"type_name":"剧情"}]}`)
		case query.Get("ids") == "41":
			_, _ = fmt.Fprintf(w, `{"code":1,"list":[%s]}`, unavailable)
		case query.Get("pg") == "2":
			_, _ = fmt.Fprintf(w, `{"code":1,"page":2,"pagecount":3,"list":[%s,%s]}`, unavailable, invalid)
		default:
			_, _ = fmt.Fprintf(w, `{"code":1,"page":1,"pagecount":3,"list":[%s,%s,%s,%s,%s]}`,
				unavailable, current, legacy, invalid, stringStatus)
		}
	}))
	defer server.Close()

	p := fixtureProvider(t, server.URL, server.Client())
	want := []string{"海岛来信", "旧版目录", "旧版状态"}
	check := func(rows []provider.Drama, err error) {
		t.Helper()
		titles := make([]string, 0, len(rows))
		for _, row := range rows {
			titles = append(titles, row.Title)
		}
		if err != nil || !slices.Equal(titles, want) {
			t.Errorf("want published titles %v, got %v, %v", want, titles, err)
		}
	}
	check(p.Home(t.Context()))
	check(p.Search(t.Context(), "海岛"))
	page, err := p.CatalogGenrePage(t.Context(), "7", 1)
	check(page.Entries, err)
	page, err = p.CatalogGenrePage(t.Context(), "7", 2)
	if err != nil || page.Page != 2 || len(page.Entries) != 0 || !page.HasMore {
		t.Errorf("filtered page should still allow the next page: %+v, %v", page, err)
	}
	id := p.idPrefix + "41"
	if detail, err := p.CatalogDetail(t.Context(), id); !errors.Is(err, provider.ErrNotFound) || detail.Drama.ID != "" {
		t.Errorf("unpublished detail escaped: %+v, %v", detail, err)
	}
	if ticket, err := p.ResolvePlayback(t.Context(), id, 1); !errors.Is(err, provider.ErrNotFound) || len(ticket.Sources) != 0 {
		t.Errorf("unpublished playback escaped: %+v, %v", ticket, err)
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
		{"null detail record", http.StatusOK, `{"code":1,"list":[null]}`, errUpstream},
		{"valid detail mixed with invalid", http.StatusOK, `{"code":1,"list":[{"vod_id":42,"vod_name":"同一剧","vod_play_url":"1$https://media.example.test/one.mp4"},{"vod_id":42,"vod_name":[],"vod_status":0}]}`, errUpstream},
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
			if tt.name == "valid detail mixed with invalid" {
				if ticket, err := p.ResolvePlayback(t.Context(), p.idPrefix+"42", 1); err != errUpstream || len(ticket.Sources) != 0 {
					t.Fatalf("unexpected playback result: %+v, %v", ticket, err)
				}
			}
		})
	}
}

func TestCatalogPageMetadata(t *testing.T) {
	const valid = `{"vod_id":42,"vod_name":"海岛"}`
	tests := []struct {
		name     string
		page     int
		metadata string
		list     string
		hasMore  bool
		wantErr  error
	}{
		{"numbers", 1, `"page":1,"pagecount":2,`, valid, true, nil},
		{"digit strings", 2, `"page":"002","pagecount":"3",`, valid, true, nil},
		{"last page", 2, `"page":"2","pagecount":2,`, valid, false, nil},
		{"legacy response", 1, "", valid, false, nil},
		{"legacy first page with advertised count", 1, `"pagecount":2,`, valid, false, nil},
		{"legacy first page with null echo", 1, `"page":null,"pagecount":2,`, valid, false, nil},
		{"later page without echo", 2, `"pagecount":2,`, valid, false, errPageMismatch},
		{"later page with null echo", 2, `"page":null,"pagecount":2,`, valid, false, errPageMismatch},
		{"no page count on a later page", 2, `"page":2,`, valid, false, nil},
		{"unusable page count", 1, `"page":1,"pagecount":"2 pages",`, valid, false, nil},
		{"empty page", 2, `"page":2,"pagecount":5,`, "", false, nil},
		{"invalid records", 2, `"page":2,"pagecount":5,`, `{"vod_id":"a","vod_name":"无效"}`, false, nil},
		{"valid and malformed rows", 2, `"page":2,"pagecount":5,`, `null,{"vod_id":41,"vod_name":[]},` + valid + `,7`, true, nil},
		{"all rows malformed", 2, `"page":2,"pagecount":5,`, `null,{"vod_id":41,"vod_name":[]},7`, false, errUpstream},
		{"record cap counts malformed rows", 2, `"page":2,"pagecount":5,`, strings.Repeat("null,", maxRecords) + valid, false, errUpstream},
		{"bounded final page", maxPage, `"page":100000,"pagecount":100001,`, valid, false, nil},
		{"different page", 2, `"page":1,"pagecount":2,`, valid, false, errPageMismatch},
		{"non-digit page", 2, `"page":"2.0","pagecount":2,`, valid, false, errPageMismatch},
		{"non-integer page", 2, `"page":2e0,"pagecount":2,`, valid, false, errPageMismatch},
		{"invalid page value", 2, `"page":{},"pagecount":2,`, valid, false, errPageMismatch},
		{"invalid request", 0, "", "", false, errInvalidPage},
		{"negative request", -1, "", "", false, errInvalidPage},
		{"above maximum request", maxPage + 1, "", "", false, errInvalidPage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				q := r.URL.Query()
				if q.Get("pg") != fmt.Sprint(tt.page) || q.Get("wd") != "海岛 & 信" || q.Has("ids") {
					t.Errorf("incorrect pagination query: %v", q)
				}
				_, _ = fmt.Fprintf(w, `{"code":1,%s"list":[%s]}`, tt.metadata, tt.list)
			}))
			defer server.Close()
			p := fixtureProvider(t, server.URL, server.Client())
			got, err := p.CatalogPage(t.Context(), " 海岛 & 信 ", tt.page)
			if err != tt.wantErr {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == errInvalidPage {
				if calls.Load() != 0 {
					t.Fatal("invalid page reached upstream")
				}
				return
			}
			if calls.Load() != 1 {
				t.Fatalf("unexpected number of upstream requests: %d", calls.Load())
			}
			if tt.wantErr != nil {
				if len(got.Entries) != 0 {
					t.Fatalf("mismatched records escaped: %+v", got)
				}
				return
			}
			wantEntries := 0
			if tt.list == valid || tt.name == "valid and malformed rows" {
				wantEntries = 1
			}
			if got.Page != tt.page || got.HasMore != tt.hasMore || got.Entries == nil || len(got.Entries) != wantEntries {
				t.Fatalf("incorrect page: %+v", got)
			}
			if wantEntries == 1 && got.Entries[0].Title != "海岛" {
				t.Fatalf("incorrect remaining title: %+v", got.Entries[0])
			}
		})
	}
}

func TestCatalogGenres(t *testing.T) {
	const maxID = "123456789012345678901234567890"
	const classes = `"class":[
		{"type_id":` + maxID + `,"type_pid":7,"type_name":"<b>悬疑</b> https://catalog.example.test/?secret"},
		{"type_id":"007","type_name":"&lt;b&gt;悬疑&lt;/b&gt; //catalog.example.test/?secret"},
		{"type_id":7,"type_name":"重复"},
		{"type_id":"` + maxID + `","type_name":"重复"},
		{"type_id":0,"type_name":"零"},
		{"type_id":"000","type_name":"零字符串"},
		{"type_id":1.5,"type_name":"小数"},
		{"type_id":1e10,"type_name":"指数"},
		{"type_id":-1,"type_name":"负数"},
		{"type_id":" 8","type_name":"空格"},
		{"type_id":"` + maxID + `1","type_name":"超长"},
		{"type_id":{},"type_name":"对象"},
		{"type_id":8,"type_name":true},
		{"type_id":8,"type_name":"https://catalog.example.test/?secret"},
		null, "非对象", {}
	]`
	wantGenres := []CatalogGenre{{ID: maxID, Name: "悬疑"}, {ID: "7", Name: "悬疑"}}
	bounded := make([]string, maxRecords+1)
	wantBounded := make([]CatalogGenre, maxRecords)
	for i := range bounded {
		bounded[i] = fmt.Sprintf(`{"type_id":%d,"type_name":"正常分类"}`, i+1)
		if i < maxRecords {
			wantBounded[i] = CatalogGenre{ID: fmt.Sprint(i + 1), Name: "正常分类"}
		}
	}
	tests := []struct {
		name string
		body string
		want []CatalogGenre
		err  error
	}{
		{"standard list", `{"code":1,` + classes + `,"list":[{"vod_id":42,"vod_name":"第一页","type_id":9}]}`, wantGenres, nil},
		{"class only", `{"code":"1",` + classes + `}`, wantGenres, nil},
		{"missing class", `{"code":1,"list":[{"type_name":"非分类目录"}]}`, []CatalogGenre{}, nil},
		{"null class", `{"code":1,"class":null}`, []CatalogGenre{}, nil},
		{"bounded", `{"code":1,"class":[` + strings.Join(bounded, ",") + `]}`, wantBounded, nil},
		{"malformed class", `{"code":1,"class":{"url":"https://catalog.example.test/?secret"}}`, nil, errUpstream},
		{"upstream error", `{"code":0,"msg":"private",` + classes + `}`, nil, errUpstream},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				const want = "ac=list&at=json&pg=1&token=fixture"
				if r.URL.RawQuery != want {
					t.Errorf("catalog genre query = %s, want %s", r.URL.RawQuery, want)
				}
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			p := fixtureProvider(t, server.URL+"?token=fixture&t=8&pg=4&wd=old&ids=41", server.Client())
			genres, err := p.CatalogGenres(t.Context())
			if err != tt.err || !slices.Equal(genres, tt.want) || (genres == nil) != (tt.want == nil) {
				t.Fatalf("genres = %+v, %v, want %+v, %v", genres, err, tt.want, tt.err)
			}
			if calls.Load() != 1 {
				t.Fatalf("catalog genre requests = %d, want 1", calls.Load())
			}
		})
	}
}

func TestCatalogGenrePageMatchesEveryRecord(t *testing.T) {
	const row = `{"vod_id":42,"vod_name":"分类剧目","type_id":` + wideID + `,"type_name":"&lt;b&gt;剧情 &amp; 奇幻&lt;/b&gt; //catalog.example.test/?secret"}`
	tests := []struct {
		name  string
		genre string
		page  int
		body  string
		count int
		more  bool
		err   error
	}{
		{"first page", wideID, 1, `{"code":1,"page":1,"pagecount":2,"list":[` + row + `]}`, 1, true, nil},
		{"ignores malformed member", wideID, 1, `{"code":1,"page":1,"pagecount":2,"list":[{"vod_id":43,"vod_name":[],"type_id":` + wideID + `},` + row + `]}`, 1, true, nil},
		{"next page", "00" + wideID, 2, `{"code":1,"page":"2","pagecount":2,"list":[{"vod_id":43,"vod_name":"下一部","type_id":"00` + wideID + `"}]}`, 1, false, nil},
		{"empty page", wideID, 2, `{"code":1,"page":2,"pagecount":3,"list":[]}`, 0, false, nil},
		{"mixed page", wideID, 1, `{"code":1,"list":[` + row + `,{"vod_id":43,"vod_name":"同类标签","type_name":"分类剧目","type_id":3}]}`, 0, false, errGenreMismatch},
		{"unverifiable type with a valid row", wideID, 1, `{"code":1,"list":[` + row + `,{"vod_id":43,"vod_name":"无效分类","type_id":{}}]}`, 0, false, errGenreMismatch},
		{"missing type", wideID, 1, `{"code":1,"list":[` + row + `,{"vod_id":43,"vod_name":"缺少类型"}]}`, 0, false, errGenreMismatch},
		{"invalid type", wideID, 1, `{"code":1,"list":[{"vod_id":42,"vod_name":"类型错误","type_id":9e19}]}`, 0, false, errGenreMismatch},
		{"ignores class", wideID, 2, `{"code":1,"page":2,"class":"unused","list":[` + row + `]}`, 1, false, nil},
		{"wrong page", wideID, 2, `{"code":1,"page":1,"list":[` + row + `]}`, 0, false, errPageMismatch},
		{"blank genre", "", 1, "", 0, false, errInvalidGenre},
		{"zero genre", "000", 1, "", 0, false, errInvalidGenre},
		{"invalid genre", "1&t=2", 1, "", 0, false, errInvalidGenre},
		{"overlong genre", strings.Repeat("1", 31), 1, "", 0, false, errInvalidGenre},
		{"invalid page", wideID, 0, "", 0, false, errInvalidPage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var classCalls, pageCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.RawQuery == "ac=list&at=json&pg=1" {
					classCalls.Add(1)
					_, _ = io.WriteString(w, `{"code":1,"class":[{"type_id":`+wideID+`,"type_name":"剧情"}]}`)
					return
				}
				pageCalls.Add(1)
				want := fmt.Sprintf("ac=detail&at=json&pg=%d&t=%s", tt.page, wideID)
				if r.URL.RawQuery != want {
					t.Errorf("catalog genre page query = %s, want %s", r.URL.RawQuery, want)
				}
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			p := fixtureProvider(t, server.URL, server.Client())
			page, err := p.CatalogGenrePage(t.Context(), tt.genre, tt.page)
			if err != tt.err || len(page.Entries) != tt.count || page.HasMore != tt.more ||
				(err == nil && (page.Page != tt.page || page.Entries == nil)) {
				t.Fatalf("genre page = %+v, %v, want %d records, more=%v, err=%v", page, err, tt.count, tt.more, tt.err)
			}
			if tt.name == "first page" && !slices.Equal(page.Entries[0].Tags, []string{"剧情 & 奇幻"}) {
				t.Fatalf("unexpected filtered page tags: %v", page.Entries[0].Tags)
			}
			wantCalls := int32(1)
			if tt.body == "" {
				wantCalls = 0
			}
			if classCalls.Load() != wantCalls || pageCalls.Load() != wantCalls {
				t.Fatalf("class calls = %d, page calls = %d, want %d each", classCalls.Load(), pageCalls.Load(), wantCalls)
			}
		})
	}
}

func TestCatalogGenrePageUsesLatestClassSnapshot(t *testing.T) {
	var version, classCalls, pageCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("ac") == "list" {
			classCalls.Add(1)
			_, _ = fmt.Fprintf(w, `{"code":1,"class":[{"type_id":%d,"type_name":"剧情"}]}`, version.Load())
			return
		}
		pageCalls.Add(1)
		if q.Get("t") != fmt.Sprint(version.Load()) {
			t.Errorf("unknown type reached upstream: %v", q)
		}
		_, _ = fmt.Fprintf(w, `{"code":1,"page":%s,"list":[]}`, q.Get("pg"))
	}))
	defer server.Close()
	version.Store(1)
	p := fixtureProvider(t, server.URL, server.Client())
	for _, id := range []string{"9", "2"} {
		if _, err := p.CatalogGenrePage(t.Context(), id, 1); err != errInvalidGenre {
			t.Fatalf("unlisted genre %s: %v", id, err)
		}
	}
	for page := 1; page <= 2; page++ {
		if _, err := p.CatalogGenrePage(t.Context(), "1", page); err != nil {
			t.Fatal(err)
		}
	}
	if classCalls.Load() != 1 || pageCalls.Load() != 2 {
		t.Fatalf("initial requests: class %d, filtered %d", classCalls.Load(), pageCalls.Load())
	}
	version.Store(2)
	genres, err := p.CatalogGenres(t.Context())
	if err != nil || !slices.Equal(genres, []CatalogGenre{{ID: "2", Name: "剧情"}}) {
		t.Fatalf("refresh = %v, %v", genres, err)
	}
	if _, err := p.CatalogGenrePage(t.Context(), "1", 2); err != errInvalidGenre {
		t.Fatalf("deleted genre: %v", err)
	}
	if _, err := p.CatalogGenrePage(t.Context(), "2", 1); err != nil {
		t.Fatal(err)
	}
	if classCalls.Load() != 2 || pageCalls.Load() != 3 {
		t.Fatalf("requests after refresh: class %d, filtered %d", classCalls.Load(), pageCalls.Load())
	}
}

func TestCatalogCoverMetadataIsOptionalAndResolvedAgainstEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"code":1,"list":[
			{"vod_id":1,"vod_name":"相对路径","vod_pic":"../covers/a.png?sig=cover"},
			{"vod_id":2,"vod_name":"外部地址","vod_pic":"https://cdn.example.test/b.webp?sig=cover"},
			{"vod_id":3,"vod_name":"站点路径","vod_pic":"/covers/c.jpeg"},
			{"vod_id":4,"vod_name":"缺少封面"},
			{"vod_id":5,"vod_name":"无效封面","vod_pic":{"malformed":true}},
			{"vod_id":6,"vod_name":"无效路径","vod_pic":"cover%zz"},
			{"vod_id":7,"vod_name":"查询封面","vod_pic":"?sig=cover"},
			{"vod_id":8,"vod_name":"空查询","vod_pic":"?"},
			{"vod_id":9,"vod_name":"无效片段","vod_pic":"#cover"},
			{"vod_id":10,"vod_name":"空封面","vod_pic":""}
		]}`)
	}))
	defer server.Close()
	p := fixtureProvider(t, server.URL+"/api/v1/vod?sig=catalog", server.Client())
	dramas, err := p.Home(t.Context())
	expected := []string{
		server.URL + "/api/covers/a.png?sig=cover",
		"https://cdn.example.test/b.webp?sig=cover",
		server.URL + "/covers/c.jpeg",
		"", "", "",
		server.URL + "/api/v1/vod?sig=cover",
		"", "", "",
	}
	if err != nil || len(dramas) != len(expected) {
		t.Fatalf("optional picture changes catalog results: got %d records, err=%v", len(dramas), err)
	}
	for i, drama := range dramas {
		if drama.CoverURL != expected[i] || drama.Cover != "" {
			t.Errorf("unexpected cover metadata for record %d", i+1)
		}
	}
}

func TestCatalogRequestsCoolDownAcrossOperations(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, "private upstream message")
			return
		}
		_, _ = io.WriteString(w, `{"code":1,"list":[]}`)
	}))
	defer server.Close()

	p := fixtureProvider(t, server.URL, server.Client())
	ctx := t.Context()
	if _, err := p.Search(ctx, "海岛"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("429 should report a fixed rate limit error: %v", err)
	}
	if _, err := p.Home(ctx); !errors.Is(err, ErrRateLimited) {
		t.Fatal("home bypassed the rate limit")
	}
	if _, err := p.CatalogGenres(ctx); !errors.Is(err, ErrRateLimited) {
		t.Fatal("genres bypassed the rate limit")
	}
	if _, err := p.CatalogDetail(ctx, p.idPrefix+"42"); !errors.Is(err, ErrRateLimited) {
		t.Fatal("detail bypassed the rate limit")
	}
	if _, err := p.ResolvePlayback(ctx, p.idPrefix+"42", 1); !errors.Is(err, ErrRateLimited) {
		t.Fatal("ticket lookup bypassed the rate limit")
	}
	// An in-flight category refresh holds this lock while waiting on HTTP.
	p.genreMu.Lock()
	categoryErrors := make(chan error, 2)
	go func() {
		_, err := p.CatalogGenres(ctx)
		categoryErrors <- err
	}()
	go func() {
		_, err := p.CatalogGenrePage(ctx, "7", 1)
		categoryErrors <- err
	}()
	for range 2 {
		select {
		case err := <-categoryErrors:
			if !errors.Is(err, ErrRateLimited) {
				t.Errorf("category request while throttled: %v", err)
			}
		case <-time.After(time.Second):
			p.genreMu.Unlock()
			t.Fatal("throttled category request waited for the in-flight refresh")
		}
	}
	p.genreMu.Unlock()
	if got := calls.Load(); got != 1 {
		t.Fatalf("blocked operations reached the upstream: %d requests", got)
	}
	other := fixtureProvider(t, server.URL, server.Client())
	if _, err := other.Home(ctx); err != nil || calls.Load() != 2 {
		t.Fatalf("another provider inherited the cooldown: %v", err)
	}
	p.retryMu.Lock()
	firstDeadline := p.retryAt
	p.retryMu.Unlock()
	p.extendCooldown(firstDeadline.Add(-time.Second))
	p.retryMu.Lock()
	if !p.retryAt.Equal(firstDeadline) {
		t.Errorf("a shorter cooldown replaced the existing deadline: %v", p.retryAt)
	}
	p.retryAt = time.Now().Add(-time.Second)
	p.retryMu.Unlock()
	if _, err := p.Home(ctx); err != nil || calls.Load() != 3 {
		t.Fatalf("expired cooldown did not allow a new request: %v", err)
	}
}

func TestRetryWait(t *testing.T) {
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name  string
		value string
		wait  time.Duration
	}{
		{"seconds", " 120 ", 2 * time.Minute},
		{"date", now.Add(42 * time.Second).Format(http.TimeFormat), 42 * time.Second},
		{"past date", now.Add(-time.Second).Format(http.TimeFormat), 0},
		{"zero", "0", 0},
		{"invalid", "-1", defaultRetryWait},
		{"absent", "", defaultRetryWait},
		{"large", "18446744073709551615", ((1<<63 - 1) / time.Second) * time.Second},
		{"overflowing digits", "18446744073709551616", ((1<<63 - 1) / time.Second) * time.Second},
		{"overflowing invalid", "18446744073709551616x", defaultRetryWait},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryWait(tt.value, now); got != tt.wait {
				t.Fatalf("retryWait(%q) = %s, want %s", tt.value, got, tt.wait)
			}
		})
	}
}

func TestCatalogCooldownExpiresNaturally(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(w, `{"code":1,"list":[]}`)
	}))
	defer server.Close()

	p := fixtureProvider(t, server.URL, server.Client())
	if _, err := p.Home(t.Context()); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("initial cooldown: %v", err)
	}
	if _, err := p.Home(t.Context()); !errors.Is(err, ErrRateLimited) || calls.Load() != 1 {
		t.Fatalf("request during cooldown: %v, calls: %d", err, calls.Load())
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := p.Home(t.Context()); err != nil || calls.Load() != 2 {
		t.Fatalf("request after Retry-After: %v, calls: %d", err, calls.Load())
	}
}
