package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
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
