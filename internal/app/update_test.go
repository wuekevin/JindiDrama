package app

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"jindi-drama/internal/storage/sqlite"
)

const validReleasePage = "https://github.com/wuekevin/JindiDrama/releases/tag/v2.0.0"
const stableReleaseFixture = `{"draft":false,"prerelease":false,"tag_name":"v2.0.0","html_url":"` + validReleasePage + `"}`

func expectReleasePageError(t *testing.T, a *App, link, want string) {
	t.Helper()
	if err := a.OpenReleasePage(link); err == nil || err.Error() != want {
		t.Fatalf("OpenReleasePage(%q) = %v, want %q", link, err, want)
	}
}

func TestOpenReleasePageRequiresLatestVerifiedCheck(t *testing.T) {
	a := New()
	expectReleasePageError(t, a, validReleasePage, "不允许打开该链接")
	stable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(stableReleaseFixture))
	}))
	defer stable.Close()
	if result, err := a.checkReleaseUpdate(t.Context(), stable.URL, "", "v1.0.0"); err != nil || !result.HasUpdate {
		t.Fatalf("fixture check = %+v, %v", result, err)
	}
	// The URL is authorized. This unit test has no Wails window to open it.
	expectReleasePageError(t, a, validReleasePage, "窗口尚未就绪")
	expectReleasePageError(t, a, validReleasePage+"-other", "不允许打开该链接")

	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer failed.Close()
	if _, err := a.checkReleaseUpdate(t.Context(), failed.URL, "", "v1.0.0"); err == nil {
		t.Fatal("expected failed check")
	}
	expectReleasePageError(t, a, validReleasePage, "不允许打开该链接")
}

func TestNewerUpdateCheckInvalidatesPendingResponse(t *testing.T) {
	a := New()
	started := make(chan struct{})
	complete := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-complete
		_, _ = w.Write([]byte(stableReleaseFixture))
	}))
	defer slow.Close()
	latest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer latest.Close()

	older := make(chan error, 1)
	go func() {
		_, err := a.checkReleaseUpdate(t.Context(), slow.URL, "", "v1.0.0")
		older <- err
	}()
	<-started
	result, err := a.checkReleaseUpdate(t.Context(), latest.URL, "", "v1.0.0")
	close(complete)
	if err != nil || !result.NoRelease {
		t.Fatalf("newer check = %+v, %v", result, err)
	}
	if err := <-older; err == nil {
		t.Fatal("stale check must not publish its result")
	}
	expectReleasePageError(t, a, validReleasePage, "不允许打开该链接")
}

func TestUpdateChannelPersistsAndRevokesPreviewLink(t *testing.T) {
	const previewPage = "https://github.com/wuekevin/JindiDrama/releases/tag/v2.0.0-rc.10"
	path := filepath.Join(t.TempDir(), "settings.db")
	open := func() *App {
		a := New()
		var err error
		a.store, err = sqlite.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = a.store.Close() })
		return a
	}

	a := open()
	if channel, err := a.UpdateChannel(); channel != "stable" || err != nil {
		t.Fatalf("new database channel = %q, %v", channel, err)
	}
	if err := a.SetUpdateChannel("invalid"); err == nil {
		t.Fatal("invalid channel must be rejected")
	}
	if err := a.SaveSetting(updateChannelKey, `"preview"`); err == nil {
		t.Fatal("generic settings bridge must not set the update channel")
	}
	if err := a.SetUpdateChannel("preview"); err != nil {
		t.Fatal(err)
	}
	if err := a.store.Close(); err != nil {
		t.Fatal(err)
	}

	a = open()
	if channel, err := a.UpdateChannel(); channel != "preview" || err != nil {
		t.Fatalf("reopened database channel = %q, %v", channel, err)
	}
	preview := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"draft":false,"prerelease":true,"tag_name":"v2.0.0-rc.10","html_url":"` + previewPage + `"}]`))
	}))
	defer preview.Close()
	result, err := a.checkReleaseUpdate(t.Context(), "", preview.URL, "v1.0.0")
	if err != nil || !result.Preview || result.URL != previewPage {
		t.Fatalf("preview check = %+v, %v", result, err)
	}
	expectReleasePageError(t, a, previewPage, "窗口尚未就绪")
	started := make(chan struct{})
	complete := make(chan struct{})
	waiting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-complete
		_, _ = w.Write([]byte(`[{"draft":false,"prerelease":true,"tag_name":"v2.0.0-rc.10","html_url":"` + previewPage + `"}]`))
	}))
	defer waiting.Close()
	older := make(chan error, 1)
	go func() {
		_, err := a.checkReleaseUpdate(t.Context(), "", waiting.URL, "v1.0.0")
		older <- err
	}()
	<-started
	err = a.SetUpdateChannel("stable")
	close(complete)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-older; err == nil {
		t.Fatal("preview check must be discarded after switching to stable")
	}
	expectReleasePageError(t, a, previewPage, "不允许打开该链接")
	if channel, err := a.UpdateChannel(); channel != "stable" || err != nil {
		t.Fatalf("switched database channel = %q, %v", channel, err)
	}
}
