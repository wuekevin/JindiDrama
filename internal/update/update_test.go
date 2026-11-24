package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.2.0", "0.1.0", 1}, {"0.1.0", "v0.1.0", 0}, {"v0.1.0", "v0.1.1", -1},
		{"v1.0.0", "v1.0.0-rc1", 1}, {"v1.0.0-rc2", "v1.0.0-rc1", 1}, {"v0.10.0", "v0.9.9", 1},
	}
	for _, c := range cases {
		got, err := Compare(c.a, c.b)
		if err != nil || got != c.want {
			t.Errorf("Compare(%s,%s)=%d,%v want %d", c.a, c.b, got, err, c.want)
		}
	}
	if _, err := Compare("abc", "0.1.0"); err == nil {
		t.Error("应拒绝非法版本号")
	}
}

func srv(code int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
}

func TestCheck(t *testing.T) {
	s := srv(200, `{"tag_name":"v0.2.0","html_url":"https://github.com/wuekevin/JindiDrama/releases/tag/v0.2.0","body":"notes"}`)
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

func TestSafeReleaseURL(t *testing.T) {
	if !SafeReleaseURL("https://github.com/wuekevin/JindiDrama/releases/tag/v1") || SafeReleaseURL("https://evil.example/x") || SafeReleaseURL("file:///etc/passwd") {
		t.Error("URL 白名单错误")
	}
}
