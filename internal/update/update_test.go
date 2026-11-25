package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.2.0", "0.1.0", 1}, {"0.1.0", "v0.1.0", 0}, {"v0.1.0", "v0.1.1", -1},
		{"v1.0.0", "v1.0.0-rc1", 1}, {"v1.0.0-rc2", "v1.0.0-rc1", 1}, {"v0.10.0", "v0.9.9", 1},
		{"v1.0.0-rc.10", "v1.0.0-rc.2", 1}, {"v1.0.0-1", "v1.0.0-beta", -1},
		{"v1.0.0+macos", "v1.0.0+windows", 0},
	}
	for _, c := range cases {
		got, err := Compare(c.a, c.b)
		if err != nil || got != c.want {
			t.Errorf("Compare(%s,%s)=%d,%v want %d", c.a, c.b, got, err, c.want)
		}
	}
	for _, malformed := range []string{"abc", "v1.2", "v1.2.3-", "1.2.3-rc.01", "01.2.3", "1.2.3+foo..bar", " v1.2.3"} {
		if _, err := Compare(malformed, "0.1.0"); err == nil {
			t.Errorf("应拒绝非法版本号 %q", malformed)
		}
	}
}

func srv(code int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
}

func TestCheck(t *testing.T) {
	s := srv(200, `{"draft":false,"prerelease":false,"tag_name":"v0.2.0","html_url":"https://github.com/wuekevin/JindiDrama/releases/tag/v0.2.0","body":"notes"}`)
	defer s.Close()
	r, err := Check(context.Background(), nil, s.URL, "v0.1.0")
	if err != nil || !r.HasUpdate || r.Latest != "v0.2.0" {
		t.Fatalf("got %+v %v", r, err)
	}
	r, _ = Check(context.Background(), nil, s.URL, "v0.2.0")
	if r.HasUpdate {
		t.Error("同版本不应提示更新")
	}
}

func TestCheckNoReleaseAndErrors(t *testing.T) {
	s404 := srv(404, `{}`)
	defer s404.Close()
	r, err := Check(context.Background(), nil, s404.URL, "v0.1.0")
	if err != nil || !r.NoRelease {
		t.Fatalf("404 应视为暂无发布：%+v %v", r, err)
	}
	s500 := srv(500, ``)
	defer s500.Close()
	if _, err := Check(context.Background(), nil, s500.URL, "v0.1.0"); err == nil {
		t.Error("500 应报错")
	}
	sBad := srv(200, `not json`)
	defer sBad.Close()
	if _, err := Check(context.Background(), nil, sBad.URL, "v0.1.0"); err == nil {
		t.Error("坏 JSON 应报错")
	}
	if _, err := Check(context.Background(), nil, "http://127.0.0.1:1", "v0.1.0"); err == nil {
		t.Error("网络不可达应报错")
	}
}

func TestReleasePageTag(t *testing.T) {
	for _, link := range []string{
		releaseURLPrefix + "v1.2.3", releaseURLPrefix + "v1.2.3-rc.2", releaseURLPrefix + "1.2.3", releaseURLPrefix + "v1.2.3%2Bbuild",
	} {
		if _, ok := releasePageTag(link); !ok {
			t.Errorf("valid release page rejected: %s", link)
		}
	}
	for _, link := range []string{
		releaseURLPrefix + "v1", releaseURLPrefix + "v1.2.3?foo=bar",
		releaseURLPrefix + "v1.2.3#assets", releaseURLPrefix + "%2E%2E", releaseURLPrefix + "v1.2.3%2Fother",
		"https://github.com/wuekevin/JindiDrama/issues/1", "https://github.com/other/JindiDrama/releases/tag/v1.2.3",
		"https://github.com.evil.example/wuekevin/JindiDrama/releases/tag/v1.2.3",
		"https://github.com@evil.example/wuekevin/JindiDrama/releases/tag/v1.2.3", "file:///etc/passwd",
	} {
		if _, ok := releasePageTag(link); ok {
			t.Errorf("invalid release page allowed: %s", link)
		}
	}
}

func TestCheckRejectsNonStableAndInvalidReleaseData(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		noRelease bool
	}{
		{"prerelease flag", `{"tag_name":"v2.0.0","prerelease":true,"draft":false}`, true},
		{"draft flag", `{"tag_name":"v2.0.0","draft":true,"prerelease":false}`, true},
		{"prerelease tag", `{"draft":false,"prerelease":false,"tag_name":"v2.0.0-rc.10"}`, true},
		{"incorrect version", `{"draft":false,"prerelease":false,"tag_name":"v02.0.0"}`, false},
		{"mismatched tag", `{"draft":false,"prerelease":false,"tag_name":"v2.0.0","html_url":"https://github.com/wuekevin/JindiDrama/releases/tag/v3.0.0"}`, false},
		{"external url", `{"draft":false,"prerelease":false,"tag_name":"v2.0.0","html_url":"https://evil.example/releases/tag/v2.0.0"}`, false},
		{"missing flag", `{"tag_name":"v2.0.0","draft":false,"html_url":"https://github.com/wuekevin/JindiDrama/releases/tag/v2.0.0"}`, false},
		{"null flag", `{"tag_name":"v2.0.0","draft":null,"prerelease":false,"html_url":"https://github.com/wuekevin/JindiDrama/releases/tag/v2.0.0"}`, false},
		{"extra json", `{"tag_name":"v2.0.0"} {}`, false},
		{"oversized body", strings.Repeat(" ", maxResponse+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := srv(200, tc.body)
			defer server.Close()
			result, err := Check(t.Context(), nil, server.URL, "v1.2.3")
			if (err == nil) != tc.noRelease || result.NoRelease != tc.noRelease ||
				result.HasUpdate || result.Latest != "" || result.URL != "" {
				t.Fatalf("result = %+v, err = %v", result, err)
			}
		})
	}
}

func TestCheckNeverOffersDowngradeAndPreservesUTF8(t *testing.T) {
	notes := strings.Repeat("更新🎬", maxNotes)
	body, err := json.Marshal(map[string]any{
		"draft": false, "prerelease": false,
		"tag_name": "v1.2.3+build", "html_url": releaseURLPrefix + "v1.2.3%2Bbuild", "body": notes,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := srv(200, string(body))
	defer server.Close()
	for _, installed := range []string{"v1.2.3+older", "v2.0.0"} {
		result, err := Check(t.Context(), nil, server.URL, installed)
		if err != nil || result.HasUpdate || result.URL != "" || result.Notes != "" {
			t.Fatalf("unexpected downgrade or equal-version link: %+v, %v", result, err)
		}
	}
	result, err := Check(t.Context(), nil, server.URL, "v1.2.3-rc.10")
	if err != nil || !result.HasUpdate || result.URL != releaseURLPrefix+"v1.2.3+build" ||
		!utf8.ValidString(result.Notes) || len([]rune(result.Notes)) != maxNotes {
		t.Fatalf("expected a stable release page and bounded UTF-8 notes, got version %q, %v", result.Latest, err)
	}
}

func TestCheckDoesNotFollowRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("update checker followed an HTTP redirect")
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirect.Close()
	if _, err := Check(t.Context(), nil, redirect.URL, "1.2.3"); err == nil {
		t.Fatal("redirect must be reported as a failed check")
	}
}

func TestCheckPreviewSelectsHighestAcrossPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("per_page") != "20" {
			t.Errorf("missing page size: %v", req.URL)
		}
		switch req.URL.Query().Get("page") {
		case "1":
			w.Header().Set("Link", `<https://example.invalid/releases?page=2>; rel="next"`)
			_, _ = w.Write([]byte(`[
				{"tag_name":"v9.0.0","draft":true,"prerelease":false},
				{"tag_name":"v8.0.0","draft":false,"prerelease":true},
				{"tag_name":"v7.0.0-rc.1","draft":false,"prerelease":true,"html_url":"https://example.invalid/v7.0.0-rc.1"},
				{"tag_name":"v2.0.0-rc.2","draft":false,"prerelease":true,"html_url":"` + releaseURLPrefix + `v2.0.0-rc.2"},
				{"tag_name":"v1.5.0","draft":false,"prerelease":false,"html_url":"` + releaseURLPrefix + `v1.5.0"}
			]`))
		case "2":
			_, _ = w.Write([]byte(`[
				{"tag_name":"v2.0.0-rc.10","draft":false,"prerelease":true,"html_url":"` + releaseURLPrefix + `v2.0.0-rc.10","body":"preview"}
			]`))
		default:
			t.Errorf("unexpected page: %v", req.URL)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	result, err := CheckPreview(t.Context(), nil, server.URL, "v1.5.0")
	if err != nil || !result.HasUpdate || !result.Preview || result.Latest != "v2.0.0-rc.10" ||
		result.URL != releaseURLPrefix+"v2.0.0-rc.10" || result.Notes != "preview" {
		t.Fatalf("preview = %+v, %v", result, err)
	}
	result, err = CheckPreview(t.Context(), nil, server.URL, "v2.0.0-rc.11")
	if err != nil || result.HasUpdate || result.URL != "" {
		t.Fatalf("preview should not downgrade: %+v, %v", result, err)
	}
}

func TestCheckPreviewOffersStablePromotion(t *testing.T) {
	server := srv(200, `[
		{"tag_name":"v2.0.0-rc.10","draft":false,"prerelease":true,"html_url":"`+releaseURLPrefix+`v2.0.0-rc.10"},
		{"tag_name":"v2.0.0","draft":false,"prerelease":false,"html_url":"`+releaseURLPrefix+`v2.0.0"}
	]`)
	defer server.Close()
	result, err := CheckPreview(t.Context(), nil, server.URL, "v2.0.0-rc.10")
	if err != nil || !result.HasUpdate || result.Preview || result.Latest != "v2.0.0" {
		t.Fatalf("preview should offer the stable promotion: %+v, %v", result, err)
	}
}

func TestCheckPreviewRejectsUnsafeOrIncompleteResults(t *testing.T) {
	for _, tc := range []struct {
		name, body, link string
		wantEmpty        bool
	}{
		{"empty", "[]", "", true},
		{"null", "null", "", false},
		{"missing flag", `[{"tag_name":"v2.0.0","draft":false}]`, "", false},
		{"unsafe winner", `[{"tag_name":"v2.0.0-rc.2","draft":false,"prerelease":true,"html_url":"https://example.invalid/v2.0.0-rc.2"}]`, "", false},
		{"pagination limit", "[]", `<https://example.invalid>; rel="next"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Link", tc.link)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			result, err := CheckPreview(t.Context(), nil, server.URL, "v1.0.0")
			if (err == nil) != tc.wantEmpty || result.NoRelease != tc.wantEmpty || result.URL != "" || result.HasUpdate {
				t.Fatalf("result = %+v, err = %v", result, err)
			}
		})
	}
}
