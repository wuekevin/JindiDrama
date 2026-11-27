package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jindi-drama/internal/provider"
)

func TestAppleCMSCoverBindingsAndRelay(t *testing.T) {
	const endpoint = "https://catalog.example.com/api/vod?catalog-key=private"
	const signature = "fixture-cover-signature"
	const label = "fixture-upstream-marker"
	picture := catalogCoverFixturePNG(t, 29, 41)
	picture = append(picture, []byte(label)...)
	var imageCalls atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/vod":
			if r.URL.Query().Get("ac") == "list" {
				_, _ = io.WriteString(w, `{"code":1,"class":[{"type_id":7,"type_name":"剧情"}]}`)
				return
			}
			_, _ = io.WriteString(w, `{"code":1,"page":1,"pagecount":1,"list":[{"vod_id":1,"vod_name":"测试短剧","type_id":7,
				"vod_total":1,"vod_pic":"covers/out.png?sig=`+signature+`"}]}`)
		case "/api/covers/out.png":
			imageCalls.Add(1)
			if r.Method != http.MethodGet || r.URL.RawQuery != "sig="+signature ||
				r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Referer") != "" ||
				r.Header.Get("Range") != "" || r.Header.Get("Accept") != "image/jpeg, image/png, image/webp" {
				t.Error("cover request did not use only the server-controlled URL and headers")
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Set-Cookie", "private="+label)
			w.Header().Set("Content-Disposition", "attachment; filename="+label)
			_, _ = w.Write(picture)
		default:
			t.Error("unexpected upstream request")
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	// Exercise actual TLS and the checked dial path without using an external network.
	client := newCatalogMediaClient()
	transport := client.Transport.(*http.Transport)
	roots := x509.NewCertPool()
	roots.AddCert(upstream.Certificate())
	transport.TLSClientConfig = &tls.Config{RootCAs: roots}
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialCheckedMediaAddress(ctx, network, address,
			func(_ context.Context, _, host string) ([]netip.Addr, error) {
				if host != "catalog.example.com" {
					t.Errorf("unexpected DNS hostname %q", host)
				}
				return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
			}, func(ctx context.Context, network, checked string) (net.Conn, error) {
				if checked != "8.8.8.8:443" {
					t.Errorf("unexpected checked IP %q", checked)
				}
				return dialer.DialContext(ctx, network, upstream.Listener.Addr().String())
			})
	}
	a := New()
	if err := configureAppleCMS(a, endpoint, client); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)

	home, err := a.Home()
	if err != nil || len(home) != 1 {
		t.Fatalf("home: %+v, %v", home, err)
	}
	first := home[0]
	local, err := url.Parse(first.Cover)
	if err != nil || local.Scheme != "http" || local.Hostname() != "127.0.0.1" || local.Port() == "" ||
		local.RawQuery != "" || local.User != nil {
		t.Fatal("home did not project the cover to the loopback listener")
	}
	token, ok := strings.CutPrefix(local.Path, catalogCoverPath)
	if !ok || !validCatalogCoverToken(token) || first.CoverURL != "" {
		t.Fatal("home did not project the cover into an opaque path")
	}
	assertCover := func(rows []provider.Drama, err error) {
		t.Helper()
		if err != nil || len(rows) != 1 || rows[0].Cover != first.Cover || rows[0].CoverURL != "" {
			t.Fatalf("binding did not reuse the same opaque cover: %v", err)
		}
		payload, err := json.Marshal(rows)
		if err != nil || strings.Contains(string(payload), "example.com") ||
			strings.Contains(string(payload), signature) || strings.Contains(string(payload), "catalog-key") {
			t.Fatalf("binding included an upstream address or signature: %v", err)
		}
	}
	assertCover(home, nil)
	assertCover(a.Search("测试"))
	assertCover(a.CatalogSearch("测试"))
	page, err := a.CatalogPage("测试", 1)
	assertCover(page.Entries, err)
	genre, err := a.CatalogGenrePage("7", 1)
	assertCover(genre.Entries, err)
	detail, err := a.Drama(first.ID)
	assertCover([]provider.Drama{detail}, err)
	detail, err = a.CatalogDrama(first.ID)
	assertCover([]provider.Drama{detail}, err)

	handler := a.catalogMedia
	playback, err := a.issueCatalogMedia(provider.PlaybackTicket{Sources: []provider.MediaSource{
		{Src: "https://catalog.example.com/example.webm", Type: "video/webm"},
	}}, time.Now().Add(time.Minute))
	if err != nil || !strings.HasPrefix(playback.Sources[0].Src, "http://"+local.Host+catalogMediaPath) {
		t.Fatalf("cover and playback did not reuse the same listener: %v", err)
	}
	a.ReleaseCatalogPlayback(playback.LeaseID)
	invalid := []struct{ method, path string }{
		{http.MethodGet, catalogCoverPath + strings.Repeat("0", 32)},
		{http.MethodHead, first.Cover},
		{http.MethodGet, first.Cover + "?some=parameter"},
		{http.MethodGet, first.Cover + "?"},
		{http.MethodGet, first.Cover + "/more"},
		{http.MethodGet, catalogCoverPath + fmt.Sprintf("%%%02x", token[0]) + token[1:]},
		{http.MethodGet, "/catalog-cover"},
	}
	for _, test := range invalid {
		res := serveCoverRequest(handler, test.method, test.path)
		if res.Code != http.StatusNotFound || strings.Contains(res.Body.String(), signature) {
			t.Errorf("%s %s returned %d", test.method, test.path, res.Code)
		}
	}
	for _, credentials := range []string{"Cookie", "Authorization"} {
		req := newCoverRequest(handler, http.MethodGet, first.Cover)
		req.Header.Set(credentials, "private=synthetic")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Errorf("%s was accepted: %d", credentials, res.Code)
		}
	}
	req := newCoverRequest(handler, http.MethodGet, first.Cover)
	req.Host = "localhost:" + local.Port()
	wrongHost := httptest.NewRecorder()
	handler.ServeHTTP(wrongHost, req)
	if wrongHost.Code != http.StatusNotFound {
		t.Fatalf("alternate host was accepted: %d", wrongHost.Code)
	}
	if imageCalls.Load() != 0 {
		t.Fatal("image requested before a valid application path was requested")
	}

	req = newCoverRequest(handler, http.MethodGet, first.Cover)
	req.Header.Set("Referer", "http://app.local/#/home")
	req.Header.Set("Range", "bytes=0-3")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	assertCatalogCoverJPEG(t, res, 29, 41)
	if imageCalls.Load() != 1 || bytes.Contains(res.Body.Bytes(), []byte(label)) ||
		res.Header().Get("Set-Cookie") != "" || res.Header().Get("Content-Disposition") != "" {
		t.Fatal("cover contained upstream bytes or headers, or requested the upstream more than once")
	}

	a.catalogCovers.mu.Lock()
	element := a.catalogCovers.byToken[token]
	expired := element.Value.(catalogCoverTicket)
	expired.expires = time.Now().Add(-time.Second)
	element.Value = expired
	a.catalogCovers.mu.Unlock()
	if res := serveCoverRequest(handler, http.MethodGet, first.Cover); res.Code != http.StatusNotFound || imageCalls.Load() != 1 {
		t.Fatalf("expired tickets reached the upstream or returned the wrong status: %d", res.Code)
	}
	fresh, err := a.Home()
	if err != nil || len(fresh) != 1 || fresh[0].Cover == "" || fresh[0].Cover == first.Cover {
		t.Fatalf("expired cover was not replaced: %v", err)
	}
	a.close()
	if res := serveCoverRequest(handler, http.MethodGet, fresh[0].Cover); res.Code != http.StatusNotFound || imageCalls.Load() != 1 {
		t.Fatalf("shutdown did not revoke cover tickets: %d", res.Code)
	}
	conn, err := net.DialTimeout("tcp4", local.Host, time.Second)
	if err == nil {
		_ = conn.Close()
		t.Fatal("cover port was still open after shutdown")
	}
}

func TestCatalogCoverRejectsInvalidMetadataAndImages(t *testing.T) {
	const signature = "fixture-cover-signature"
	picture := catalogCoverFixturePNG(t, 2, 2)
	jpegPicture := new(bytes.Buffer)
	if err := jpeg.Encode(jpegPicture, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	// A 2x2 lossless WebP generated from four colored pixels.
	webpPicture, err := base64.StdEncoding.DecodeString("UklGRiwAAABXRUJQVlA4TB8AAAAvAUAAAB8gEEjeHzqN+RcQFPwf3fxHZA/gBgwR/Q8BAA==")
	if err != nil {
		t.Fatal(err)
	}
	inconsistentWebP := slices.Clone(webpPicture)
	binary.LittleEndian.PutUint32(inconsistentWebP[4:8], uint32(len(inconsistentWebP)-8+2))
	covers := []struct {
		name, mime string
		body       []byte
		status     int
	}{
		{"png", "image/png", picture, http.StatusOK},
		{"jpeg", "image/jpeg", jpegPicture.Bytes(), http.StatusOK},
		{"webp", "image/webp", webpPicture, http.StatusOK},
		{"inconsistent-webp-container-size", "image/webp", inconsistentWebP, http.StatusBadGateway},
		{"html", "text/html", []byte("<html>" + signature), http.StatusBadGateway},
		{"html-as-png", "image/png", []byte("<html>" + signature), http.StatusBadGateway},
		{"svg", "image/svg+xml", []byte("<svg></svg>"), http.StatusBadGateway},
		{"wrong-mime", "image/jpeg", picture, http.StatusBadGateway},
		{"bad-header", "image/png", picture[:12], http.StatusBadGateway},
		{"bad-image-data", "image/png", picture[:40], http.StatusBadGateway},
		{"oversized", "image/png", append(slices.Clone(picture), make([]byte, maxCatalogCoverBytes)...), http.StatusBadGateway},
		{"wide", "image/png", catalogCoverFixturePNG(t, maxCatalogCoverSide+1, 1), http.StatusBadGateway},
		{"pixel-limit", "image/png", catalogCoverFixturePNG(t, 2001, 2000), http.StatusBadGateway},
		{"redirect", "", nil, http.StatusBadGateway},
		{"upstream-error", "", []byte(signature), http.StatusBadGateway},
	}

	client := newCatalogMediaClient()
	var imageCalls int
	client.Transport = catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		imageCalls++
		if req.URL.Host != "catalog.example.com" || req.URL.RawQuery != "sig="+signature {
			t.Error("image fetched an unexpected target")
		}
		name := strings.TrimPrefix(req.URL.Path, "/")
		for _, cover := range covers {
			if cover.name != name {
				continue
			}
			res := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {cover.mime}},
				Body: io.NopCloser(bytes.NewReader(cover.body)), ContentLength: -1}
			if name == "redirect" {
				res.StatusCode = http.StatusFound
				res.Header.Set("Location", "https://different.example.com/?sig="+signature)
			}
			if name == "upstream-error" {
				res.StatusCode = http.StatusInternalServerError
			}
			return res, nil
		}
		t.Error("rejected image request reached the upstream")
		return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil
	})
	a := New()
	if err := configureAppleCMS(a, "https://catalog.example.com/api/vod", client); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	for _, cover := range covers {
		t.Run(cover.name, func(t *testing.T) {
			drama := provider.Drama{CoverURL: "https://catalog.example.com/" + cover.name + "?sig=" + signature}
			a.projectCatalogCover(&drama)
			if drama.Cover == "" {
				t.Fatal("fixture did not receive a cover ticket")
			}
			res := serveCoverRequest(a.catalogMedia, http.MethodGet, drama.Cover)
			if res.Code != cover.status {
				t.Fatalf("got %d, want %d", res.Code, cover.status)
			}
			if cover.status == http.StatusOK {
				assertCatalogCoverJPEG(t, res, 2, 2)
			} else if res.Body.String() != "Bad Gateway\n" || res.Header().Get("Location") != "" {
				t.Fatal("upstream response data escaped an error response")
			}
		})
	}
	for _, address := range []string{
		"file:///tmp/cover.png", "http://catalog.example.com/a", "https://catalog.example.com:8080/a",
		"https://private:secret@catalog.example.com/a", "https://127.0.0.1/a", "https://cdn.local/a",
		"https://[::1]/a", "https://catalog.example.com/a#hidden", "https://catalog.example.com/a b",
	} {
		drama := provider.Drama{CoverURL: address}
		a.projectCatalogCover(&drama)
		if drama.Cover != "" || drama.CoverURL != "" {
			t.Error("invalid address was retained or issued a ticket")
		}
	}
	if imageCalls != len(covers) {
		t.Fatalf("got %d upstream requests, want %d", imageCalls, len(covers))
	}
	payload, err := json.Marshal(provider.Drama{CoverURL: "https://catalog.example.com/?sig=" + signature})
	if err != nil || bytes.Contains(payload, []byte(signature)) || bytes.Contains(payload, []byte("cover")) {
		t.Fatalf("unprojected provider drama serializes upstream cover metadata: %v", err)
	}
}

func TestCatalogCoverCapacity(t *testing.T) {
	s := newCatalogCoverService(newCatalogMediaClient())
	defer s.close()
	const first = "https://catalog.example.com/first.png"
	oldest := s.issue(first)
	for i := range catalogCoverCapacity {
		address := "https://catalog.example.com/" + strconv.Itoa(i) + ".png"
		if token := s.issue(address); token == "" || s.lookup(token).address != address {
			t.Fatal("valid cover could not be issued")
		}
	}
	if s.lookup(oldest).address != "" || s.issue(first) == oldest {
		t.Fatal("oldest ticket was not evicted")
	}
	if s.issued.Len() != catalogCoverCapacity || len(s.byAddress) != catalogCoverCapacity || len(s.byToken) != catalogCoverCapacity {
		t.Fatal("ticket indices exceeded the capacity")
	}
}

func TestCatalogCoverCachesOnlyFreshPublicImages(t *testing.T) {
	picture := catalogCoverFixturePNG(t, 2, 2)
	for _, tc := range []struct {
		name, policy, header, value, query string
		wantCalls                          int
	}{
		{name: "fresh public", policy: "public, max-age=60, immutable", wantCalls: 1},
		{name: "unmarked", wantCalls: 2},
		{name: "private", policy: "public, max-age=60, private", wantCalls: 2},
		{name: "no-store", policy: "public, max-age=60, no-store", wantCalls: 2},
		{name: "no-cache", policy: "public, max-age=60, no-cache", wantCalls: 2},
		{name: "ambiguous freshness", policy: "public, max-age=60, max-age=0", wantCalls: 2},
		{name: "stale Age", policy: "public, max-age=60", header: "Age", value: "60", wantCalls: 2},
		{name: "stale Date", policy: "public, max-age=60", header: "Date", value: time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat), wantCalls: 2},
		{name: "varies", policy: "public, max-age=60", header: "Vary", value: "Accept", wantCalls: 2},
		{name: "cookie", policy: "public, max-age=60", header: "Set-Cookie", value: "session=example", wantCalls: 2},
		{name: "signed URL", policy: "public, max-age=60", query: "?signed=example", wantCalls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			reloaded := catalogCoverFixturePNG(t, 3, 2)
			client := newCatalogMediaClient()
			client.Transport = catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				header := http.Header{"Content-Type": {"image/png"}}
				if tc.policy != "" {
					header.Set("Cache-Control", tc.policy)
				}
				if tc.header != "" {
					header.Set(tc.header, tc.value)
				}
				body := picture
				if tc.wantCalls == 1 && calls > 1 {
					body = reloaded
				}
				return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(bytes.NewReader(body))}, nil
			})
			s := newCatalogCoverService(client)
			t.Cleanup(s.close)
			token := s.issue("https://catalog.example.com/cover.png" + tc.query)
			handler := &catalogMediaRelay{host: "localhost:8080", covers: s}
			path := catalogCoverPath + token
			first := serveCoverRequest(handler, http.MethodGet, path)
			second := serveCoverRequest(handler, http.MethodGet, path)
			assertCatalogCoverJPEG(t, first, 2, 2)
			assertCatalogCoverJPEG(t, second, 2, 2)
			if calls != tc.wantCalls || !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
				t.Fatalf("upstream requests: got %d, want %d, or JPEG changed", calls, tc.wantCalls)
			}
			if tc.wantCalls == 1 {
				request := newCoverRequest(handler, http.MethodGet, path)
				request.Header.Set("Cache-Control", "no-cache")
				reply := httptest.NewRecorder()
				handler.ServeHTTP(reply, request)
				assertCatalogCoverJPEG(t, reply, 3, 2)
				assertCatalogCoverJPEG(t, serveCoverRequest(handler, http.MethodGet, path), 3, 2)
				if calls != 2 {
					t.Fatal("reload did not fetch and replace the cached image")
				}
				request = newCoverRequest(handler, http.MethodGet, path)
				request.Header.Set("Cache-Control", "no-store")
				handler.ServeHTTP(httptest.NewRecorder(), request)
				assertCatalogCoverJPEG(t, serveCoverRequest(handler, http.MethodGet, path), 3, 2)
				if calls != 4 {
					t.Fatal("no-store did not discard the cached image")
				}
			}
		})
	}
}

func TestCatalogCoverCacheBudgetAndExpiry(t *testing.T) {
	s := newCatalogCoverService(newCatalogMediaClient())
	t.Cleanup(s.close)
	firstToken := s.issue("https://catalog.example.com/first.png")
	secondToken := s.issue("https://catalog.example.com/second.png")
	s.retain(firstToken, s.beginFetch(firstToken), make([]byte, 1, maxCachedCoverBytes), time.Now().Add(time.Minute))
	s.retain(secondToken, s.beginFetch(secondToken), []byte{1}, time.Now().Add(time.Minute))
	if len(s.lookup(firstToken).data) == 0 || len(s.lookup(secondToken).data) != 0 || s.cachedBytes != maxCachedCoverBytes {
		t.Fatal("expected successful storage up to the limit, then a cache miss")
	}
	first := s.byToken[firstToken].Value.(catalogCoverTicket)
	first.freshTo = time.Now().Add(-time.Second)
	s.byToken[firstToken].Value = first
	s.retain(secondToken, s.beginFetch(secondToken), []byte{1}, time.Now().Add(time.Minute))
	if cap(s.lookup(secondToken).data) != 1 || len(s.lookup(firstToken).data) != 0 || s.cachedBytes != 1 {
		t.Fatal("admission did not reclaim stale images")
	}
	older := s.beginFetch(secondToken)
	newer := s.beginFetch(secondToken)
	s.retain(secondToken, newer, []byte{2}, time.Now().Add(time.Minute))
	s.retain(secondToken, older, []byte{1}, time.Now().Add(time.Minute))
	if cached := s.lookup(secondToken).data; !bytes.Equal(cached, []byte{2}) {
		t.Fatal("an older request replaced cached data from the latest request")
	}
	s.close()
	if s.cachedBytes != 0 || len(s.lookup(secondToken).data) != 0 {
		t.Fatal("closing the service retained images")
	}
}

func newCoverRequest(handler *catalogMediaRelay, method, path string) *http.Request {
	req := httptest.NewRequest(method, strings.TrimPrefix(path, "http://"+handler.host), nil)
	req.Host = handler.host
	return req
}

func serveCoverRequest(handler *catalogMediaRelay, method, path string) *httptest.ResponseRecorder {
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, newCoverRequest(handler, method, path))
	return res
}

func catalogCoverFixturePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var out bytes.Buffer
	source := image.NewPaletted(image.Rect(0, 0, width, height), color.Palette{color.RGBA{41, 67, 175, 255}})
	if err := png.Encode(&out, source); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func assertCatalogCoverJPEG(t *testing.T, res *httptest.ResponseRecorder, width, height int) {
	t.Helper()
	config, err := jpeg.DecodeConfig(bytes.NewReader(res.Body.Bytes()))
	if err != nil || config.Width != width || config.Height != height ||
		res.Code != http.StatusOK || res.Header().Get("Content-Type") != "image/jpeg" ||
		res.Header().Get("X-Content-Type-Options") != "nosniff" || res.Header().Get("Cache-Control") != "no-store" ||
		res.Header().Get("Access-Control-Allow-Origin") != "*" ||
		res.Header().Get("Content-Length") != strconv.Itoa(res.Body.Len()) || res.Body.Len() > maxCatalogCoverBytes {
		t.Fatalf("invalid sanitized cover: status %d, dimensions %dx%d, bytes %d, decode error %v",
			res.Code, config.Width, config.Height, res.Body.Len(), err)
	}
}

func TestCatalogCoverQueuedRequestCancelsBeforeUpstream(t *testing.T) {
	client := newCatalogMediaClient()
	var requests atomic.Int32
	picture := catalogCoverFixturePNG(t, 3, 5)
	client.Transport = catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		if req.URL.Path != "/next.png" {
			t.Error("canceled cover reached the upstream")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}},
			Body: io.NopCloser(bytes.NewReader(picture))}, nil
	})
	a := New()
	if err := configureAppleCMS(a, "https://catalog.example.com/vod", client); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	waiting := provider.Drama{CoverURL: "https://catalog.example.com/queued.png"}
	next := provider.Drama{CoverURL: "https://catalog.example.com/next.png"}
	a.projectCatalogCover(&waiting)
	a.projectCatalogCover(&next)
	for range catalogCoverConcurrent {
		a.catalogCovers.active <- struct{}{}
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	browser := &http.Client{Timeout: 5 * time.Second}
	defer browser.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, waiting.Cover, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		res, err := browser.Do(req)
		if res != nil {
			res.Body.Close()
		}
		done <- err
	}()
	waitForCoverRequests := func(want int) {
		t.Helper()
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for len(a.catalogCovers.outstanding) != want {
			select {
			case <-ticker.C:
			case <-timer.C:
				t.Fatalf("cover queue count = %d, want %d", len(a.catalogCovers.outstanding), want)
			}
		}
	}
	waitForCoverRequests(1)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel returned %v", err)
	}
	waitForCoverRequests(0)
	<-a.catalogCovers.active

	res, err := browser.Get(next.Cover)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || requests.Load() != 1 {
		t.Fatalf("next cover status %d, upstream calls %d", res.StatusCode, requests.Load())
	}
}
