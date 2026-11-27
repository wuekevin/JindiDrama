package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
)

func TestSelectUpdateAsset(t *testing.T) {
	mac := releaseAsset{17, "JindiDrama-macos-universal.zip", "uploaded", 3, fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("zip")))}
	win := mac
	win.ID, win.Name = 31, "JindiDrama-amd64-installer.exe"
	entries := []releaseAsset{mac, win}
	for _, tc := range []struct {
		system, arch string
		id           int64
	}{{"darwin", "arm64", 17}, {"darwin", "amd64", 17}, {"windows", "amd64", 31}} {
		if asset := selectAsset(entries, tc.system, tc.arch); asset == nil || asset.id != tc.id {
			t.Fatalf("%s/%s: %+v", tc.system, tc.arch, asset)
		}
	}
	missingDigest := mac
	missingDigest.Digest = ""
	if selectAsset(entries, "linux", "amd64") != nil ||
		selectAsset([]releaseAsset{missingDigest}, "darwin", "arm64") != nil ||
		selectAsset([]releaseAsset{mac, mac}, "darwin", "arm64") != nil {
		t.Fatal("accepted unsupported, unsigned by digest, or ambiguous download")
	}
}

func TestUpdateDownloadSizeAndDigest(t *testing.T) {
	for _, tc := range []struct{ body, failure string }{
		{"zip", ""}, {"bad", "SHA-256"}, {"zi", "大小"}, {"zipp", "大小"},
	} {
		t.Run(tc.body, func(t *testing.T) {
			server := srv(http.StatusOK, tc.body)
			defer server.Close()
			asset := Asset{size: 3, digest: sha256.Sum256([]byte("zip"))}
			var staged bytes.Buffer
			err := asset.download(context.Background(), &staged, server.Client(), server.URL)
			if tc.failure == "" {
				if err != nil || staged.String() != tc.body {
					t.Fatalf("download: %q, %v", staged.String(), err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.failure) {
				t.Fatalf("want size/hash failure, got %v", err)
			}
		})
	}
}

func TestUpdateDownloadHTTPFailure(t *testing.T) {
	for _, tc := range []struct {
		name          string
		status        int
		header, value string
		want          string
	}{
		{"rate limited", http.StatusTooManyRequests, "", "", "下载请求过于频繁"},
		{"primary limit", http.StatusForbidden, "X-RateLimit-Remaining", "0", "下载请求过于频繁"},
		{"secondary limit", http.StatusForbidden, "Retry-After", "60", "下载请求过于频繁"},
		{"forbidden", http.StatusForbidden, "X-RateLimit-Remaining", "10", "确认访问权限"},
		{"unauthorized", http.StatusUnauthorized, "", "", "确认访问权限"},
		{"unavailable", http.StatusServiceUnavailable, "", "", "下载服务暂时不可用"},
		{"missing", http.StatusNotFound, "", "", "重新检查更新"},
		{"removed", http.StatusGone, "", "", "重新检查更新"},
		{"redirect refused", http.StatusFound, "", "", "前往发布页下载"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.header != "" {
					w.Header().Set(tc.header, tc.value)
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, "private response")
			}))
			defer server.Close()
			var staged bytes.Buffer
			asset := Asset{size: 3}
			err := asset.download(context.Background(), &staged, server.Client(), server.URL)
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "private response") {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
			if staged.Len() != 0 {
				t.Fatalf("wrote failed response to staging file: %q", staged.String())
			}
		})
	}
}

type downloadTestWriter func([]byte) (int, error)

func (w downloadTestWriter) Write(p []byte) (int, error) { return w(p) }

func TestUpdateDownloadWriteFailure(t *testing.T) {
	server := srv(http.StatusOK, "zip")
	defer server.Close()
	asset := Asset{size: 3}
	for _, tc := range []struct {
		name   string
		err    error
		cancel bool
	}{
		{"write error", errors.New("write /private/staging: disk full"), false},
		{"short write", nil, false},
		{"write error with concurrent cancellation", errors.New("write /private/staging: disk full"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writer := downloadTestWriter(func([]byte) (int, error) {
				if tc.cancel {
					cancel()
				}
				return 1, tc.err
			})
			err := asset.download(ctx, writer, server.Client(), server.URL)
			if err == nil || err.Error() != "无法写入更新文件，请检查保存位置和可用空间" {
				t.Fatalf("want safe write failure, got %v", err)
			}
		})
	}
}

type downloadTestTransport func(*http.Request) (*http.Response, error)

func (f downloadTestTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestUpdateDownloadNetworkFailure(t *testing.T) {
	asset := Asset{size: 3}
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"timeout", context.DeadlineExceeded, "下载更新超时"},
		{"connection", errors.New("private connection failure"), "无法连接到 GitHub"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: downloadTestTransport(func(*http.Request) (*http.Response, error) {
				return nil, tc.err
			})}
			err := asset.download(context.Background(), io.Discard, client, "https://test.invalid/asset")
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "private") {
				t.Fatalf("want safe %q error, got %v", tc.want, err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := asset.download(ctx, io.Discard, client, "https://test.invalid/asset"); !errors.Is(err, context.Canceled) {
				t.Fatalf("want cancellation, got %v", err)
			}

			concurrent, cancelConcurrent := context.WithCancel(context.Background())
			defer cancelConcurrent()
			client.Transport = downloadTestTransport(func(*http.Request) (*http.Response, error) {
				cancelConcurrent()
				return nil, tc.err
			})
			if err := asset.download(concurrent, io.Discard, client, "https://test.invalid/asset"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q with cancellation, got %v", tc.want, err)
			}
		})
	}
}

func TestUpdateDownloadBodyTimeout(t *testing.T) {
	client := &http.Client{Transport: downloadTestTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: -1,
			Body:          io.NopCloser(iotest.ErrReader(context.DeadlineExceeded)),
		}, nil
	})}
	asset := Asset{size: 3}
	err := asset.download(context.Background(), io.Discard, client, "https://test.invalid/asset")
	if err == nil || !strings.Contains(err.Error(), "下载更新超时") {
		t.Fatalf("want timeout in response body, got %v", err)
	}
}

func TestUpdateRedirectPolicy(t *testing.T) {
	redirect := assetClient().CheckRedirect
	for _, tc := range []struct {
		link    string
		hops    int
		allowed bool
	}{
		{"https://release-assets.githubusercontent.com/path?key=1", 1, true},
		{"https://objects.githubusercontent.com/path", 2, true},
		{"https://release-assets.githubusercontent.com/path", 4, false},
		{"https://release-assets.githubusercontent.com.evil.test/path", 1, false},
		{"https://release-assets.githubusercontent.com@evil.test/path", 1, false},
		{"http://objects.githubusercontent.com/path", 1, false},
	} {
		u, err := url.Parse(tc.link)
		if err != nil {
			t.Fatal(err)
		}
		if got := redirect(&http.Request{URL: u}, make([]*http.Request, tc.hops)) == nil; got != tc.allowed {
			t.Errorf("%v: allowed=%v", tc, got)
		}
	}
}
