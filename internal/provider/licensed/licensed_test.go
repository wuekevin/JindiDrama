package licensed

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jindi-drama/internal/provider"
)

const manifest = `[{"id":"local-short","title":"南风","summary":"原创片段","license":"作者自有","tags":["风景"],"episodes":[["clips/first.mp4","clips/first.webm"],["clips/second.mp4"]]}]`

func catalogFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, value := range map[string]string{
		"catalog.json": manifest, "clips/first.mp4": "0123456789",
		"clips/first.webm": "webm", "clips/second.mp4": "second", "clips/unlisted.mp4": "unlisted",
	} {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCatalogAndMediaMiddleware(t *testing.T) {
	dir := catalogFixture(t)
	p, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	ctx := context.Background()
	for _, key := range []string{"catalog", "search", "playback"} {
		if p.Capabilities(ctx)[key] != provider.Ready {
			t.Fatalf("capability %s not ready", key)
		}
	}
	dramas, err := p.Search(ctx, " 风景 ")
	if err != nil || len(dramas) != 1 || dramas[0].License != "作者自有" {
		t.Fatalf("search: %v, %v", dramas, err)
	}
	id := dramas[0].ID
	episodes, err := p.Episodes(ctx, id)
	if err != nil || len(episodes) != 2 || !episodes[1].Available {
		t.Fatalf("episodes: %v, %v", episodes, err)
	}
	ticket, err := p.ResolvePlayback(ctx, id, 1)
	if err != nil || ticket.Kind != "licensed" || len(ticket.Sources) != 2 {
		t.Fatalf("ticket: %v, %v", ticket, err)
	}
	if _, err := p.ResolvePlayback(ctx, id, 3); !errors.Is(err, provider.ErrEpisodeUnavailable) {
		t.Fatalf("out of range: %v", err)
	}
	target := ticket.Sources[0].Src
	if strings.Contains(target, "clips/") {
		t.Fatalf("ticket includes configured path: %s", target)
	}

	handler := p.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	for _, tc := range []struct {
		method, url, rangeHeader, body string
		status                         int
	}{
		{"GET", target, "", "0123456789", 200},
		{"GET", target, "bytes=2-4", "234", 206},
		{"HEAD", target, "", "", 200},
		{"POST", target, "", "", 405},
		{"GET", MediaPrefix + "clips/unlisted.mp4", "", "404 page not found\n", 404},
		{"GET", MediaPrefix + "../catalog.json", "", "404 page not found\n", 404},
		{"GET", "/index.html", "", "", 202},
	} {
		r := httptest.NewRequest(tc.method, tc.url, nil)
		if tc.rangeHeader != "" {
			r.Header.Set("Range", tc.rangeHeader)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		response := w.Result()
		payload, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil || response.StatusCode != tc.status || string(payload) != tc.body {
			t.Errorf("%s %s %s: %d, %q, %v", tc.method, tc.url, tc.rangeHeader, response.StatusCode, payload, err)
		}
		if tc.status == 206 && (response.Header.Get("Content-Range") != "bytes 2-4/10" ||
			response.Header.Get("Content-Type") != "video/mp4" || response.Header.Get("Cache-Control") != "no-store") {
			t.Errorf("range response headers: %v", response.Header)
		}
	}
}

func TestCatalogRejectsUnlicensedAndEscapingFiles(t *testing.T) {
	for _, tc := range []struct {
		name, original, replacement string
	}{
		{"missing license", "作者自有", " "},
		{"outside path", "clips/first.mp4", "../first.mp4"},
		{"absolute path", "clips/first.mp4", "/tmp/first.mp4"},
		{"drive path", "clips/first.mp4", "C:/first.mp4"},
		{"missing file", "clips/first.mp4", "clips/missing.mp4"},
		{"duplicate format", "clips/first.webm", "clips/second.mp4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := catalogFixture(t)
			changed := strings.Replace(manifest, tc.original, tc.replacement, 1)
			if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(changed), 0o600); err != nil {
				t.Fatal(err)
			}
			if p, err := New(dir); err == nil {
				_ = p.Close()
				t.Fatal("accepted invalid manifest")
			}
		})
	}
}

func TestMediaCannotBeReplacedWithOutsideSymlink(t *testing.T) {
	dir := catalogFixture(t)
	p, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	outside := filepath.Join(t.TempDir(), "outside.mp4")
	if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "clips", "first.mp4")
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := New(dir); err == nil {
		t.Fatal("startup accepted outside symlink")
	}
	ticket, err := p.ResolvePlayback(context.Background(), "local-short", 1)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	p.Middleware(http.NotFoundHandler()).ServeHTTP(w, httptest.NewRequest("GET", ticket.Sources[0].Src, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("served outside path: %d", w.Code)
	}
}

func TestLoopbackServesOnlyTokenizedMediaPaths(t *testing.T) {
	p, err := New(catalogFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	stop, err := p.StartLoopback()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop() })

	ticket, err := p.ResolvePlayback(context.Background(), "local-short", 1)
	if err != nil {
		t.Fatal(err)
	}
	mediaURL := ticket.Sources[0].Src
	parsed, err := url.Parse(mediaURL)
	if err != nil || parsed.Hostname() != "127.0.0.1" {
		t.Fatalf("media URL is not loopback: %q", mediaURL)
	}
	parsed.Path = MediaPrefix + "local-short/1/clip.mp4"
	withoutToken := parsed.String()
	client := http.Client{Timeout: 5 * time.Second}
	for _, tc := range []struct {
		url, host string
		status    int
	}{
		{mediaURL, "", 206},
		{withoutToken, "", 404},
		{mediaURL, "other.example", 404},
	} {
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = tc.host
		req.Header.Set("Range", "bytes=3-5")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil || response.StatusCode != tc.status || (tc.status == 206 && string(body) != "345") {
			t.Errorf("response: %d, %q, %v", response.StatusCode, body, err)
		}
	}
}

func TestCatalogRejectsUnreadableMedia(t *testing.T) {
	directory := catalogFixture(t)
	video := filepath.Join(directory, "clips", "first.mp4")
	if err := os.Chmod(video, 0); err != nil {
		t.Skipf("cannot change file mode: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(video, 0o600) })
	if file, err := os.Open(video); err == nil {
		_ = file.Close()
		t.Skip("this platform or account can read mode 000 media")
	}
	if p, err := New(directory); err == nil {
		_ = p.Close()
		t.Fatal("accepted unreadable media")
	}
}
