package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func appleCMSSettingsFixture(t *testing.T) *http.Client {
	t.Helper()
	client := newAppleCMSClient()
	client.Transport = catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		if req.Method != http.MethodGet || req.URL.Scheme != "https" ||
			query.Get("ac") != "detail" || query.Get("pg") != "1" || query.Get("at") != "json" {
			t.Errorf("unexpected catalog request: %s %s", req.Method, req.URL)
		}
		status, body := http.StatusOK, `{"code":1,"page":1,"list":[{"vod_id":42,"vod_name":"授权夹具"}]}`
		switch req.URL.Hostname() {
		case "catalog.example.test":
			if req.URL.Path != "/api.php/provide/vod/" {
				t.Errorf("unexpected API path: %s", req.URL.Path)
			}
		case "managed.example.test":
			if query.Get("access") != "managed-private" || req.URL.Path != "/operator/api" {
				t.Errorf("managed request lost its endpoint configuration: %s", req.URL)
			}
		case "empty.example.test":
			body = `{"code":1,"list":[]}`
		case "bad-page.example.test":
			body = `{"code":1,"page":2,"list":[]}`
		case "bad-response.example.test":
			body = `{"code":1,"list":"response-private"}`
		case "redirect.example.test":
			return &http.Response{
				StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://blocked.example.test"}},
				Body: io.NopCloser(strings.NewReader("response-private")),
			}, nil
		case "unavailable.example.test":
			return nil, errors.New("transport-private")
		default:
			t.Errorf("unexpected request to %s", req.URL)
			status, body = http.StatusBadGateway, ""
		}
		return &http.Response{
			StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(body)),
		}, nil
	})
	return client
}

func TestAppleCMSSettingsPersistUntilStartup(t *testing.T) {
	t.Setenv("JINDIDRAMA_DATA_DIR", t.TempDir())
	client := appleCMSSettingsFixture(t)
	start := func() *App {
		t.Helper()
		a := New()
		if err := prepareAppleCMS(a, "", client); err != nil {
			t.Fatal(err)
		}
		return a
	}
	a := start()
	if config := a.AppleCMSConfiguration(); config != (AppleCMSConfiguration{CanConfigure: true}) || a.ProviderName() != "mock" {
		t.Fatalf("initial config: %+v, provider=%s", config, a.ProviderName())
	}
	if _, err := checkAppleCMSConnection(context.Background(), "https://empty.example.test/fixture", client); err != nil {
		t.Fatalf("valid empty catalog: %v", err)
	}
	if config := a.AppleCMSConfiguration(); config.SavedURL != "" {
		t.Fatalf("connection test changed config: %+v", config)
	}
	const saved = "https://catalog.example.test/api.php/provide/vod/"
	config, err := a.saveAppleCMSURL("  https://Catalog.Example.Test.:443  ", client)
	if err != nil || config != (AppleCMSConfiguration{CanConfigure: true, SavedURL: saved, RestartRequired: true}) || a.ProviderName() != "mock" {
		t.Fatalf("save: %+v, provider=%s, err=%v", config, a.ProviderName(), err)
	}
	if value, err := a.GetSetting(appleCMSURLKey); value != "" || err == nil {
		t.Fatalf("generic bridge read the source setting: %q, %v", value, err)
	}
	if err := a.SaveSetting(appleCMSURLKey, `"https://empty.example.test"`); err == nil {
		t.Fatal("generic bridge overwrote the source setting")
	}
	a.close()

	a = start()
	config = a.AppleCMSConfiguration()
	if config != (AppleCMSConfiguration{Active: true, CanConfigure: true, SavedURL: saved}) || a.ProviderName() != "apple-cms" {
		t.Fatalf("reopened: %+v, provider=%s", config, a.ProviderName())
	}
	page, err := a.CatalogPage("", 1)
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Title != "授权夹具" {
		t.Fatalf("active directory: %+v, err=%v", page, err)
	}
	config, err = a.ClearAppleCMSURL()
	if err != nil || config != (AppleCMSConfiguration{Active: true, CanConfigure: true, RestartRequired: true}) || a.ProviderName() != "apple-cms" {
		t.Fatalf("clear: %+v, provider=%s, err=%v", config, a.ProviderName(), err)
	}
	a.close()

	a = start()
	t.Cleanup(a.close)
	if config := a.AppleCMSConfiguration(); config != (AppleCMSConfiguration{CanConfigure: true}) || a.ProviderName() != "mock" {
		t.Fatalf("reopened after clear: %+v, provider=%s", config, a.ProviderName())
	}
}

func TestAppleCMSSettingsRejectFailuresWithoutReplacingSavedURL(t *testing.T) {
	t.Setenv("JINDIDRAMA_DATA_DIR", t.TempDir())
	client := appleCMSSettingsFixture(t)
	a := New()
	if err := prepareAppleCMS(a, "", client); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	before, err := a.saveAppleCMSURL("https://catalog.example.test", client)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		url  string
		want error
	}{
		{"http://catalog.example.test", errAppleCMSUserURL},
		{"https://127.0.0.1", errAppleCMSUserURL},
		{"https://catalog.internal", errAppleCMSUserURL},
		{"https://catalog.example.test?", errAppleCMSUserURL},
		{"https://catalog.example.test?private=query", errAppleCMSUserURL},
		{"https://user:password@catalog.example.test", errAppleCMSUserURL},
		{"https://catalog.example.test/#private", errAppleCMSUserURL},
		{"https://catalog.example.test/" + strings.Repeat("中", 500), errAppleCMSUserURL},
		{"https://bad-response.example.test", errAppleCMSConnection},
		{"https://bad-page.example.test", errAppleCMSConnection},
		{"https://redirect.example.test", errAppleCMSConnection},
		{"https://unavailable.example.test", errAppleCMSConnection},
	} {
		t.Run(tc.url, func(t *testing.T) {
			if _, err := checkAppleCMSConnection(context.Background(), tc.url, client); err != tc.want {
				t.Errorf("test error = %v, want %v", err, tc.want)
			}
			if _, err := a.saveAppleCMSURL(tc.url, client); err != tc.want {
				t.Errorf("save error = %v, want %v", err, tc.want)
			}
			if got := a.AppleCMSConfiguration(); got != before {
				t.Errorf("failed test or save changed config: %+v", got)
			}
		})
	}
	if client.Timeout != 12*time.Second ||
		client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("fixture client lost the production timeout or redirect policy")
	}
	if err := a.store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveAppleCMSURL("https://empty.example.test", client); err != errAppleCMSWrite {
		t.Fatalf("write error = %v", err)
	}
	reopened := New()
	if err := prepareAppleCMS(reopened, "", client); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.close)
	if got := reopened.AppleCMSConfiguration(); got.SavedURL != before.SavedURL || !got.Active {
		t.Fatalf("failed database write replaced persisted endpoint: %+v", got)
	}
}

func TestAppleCMSSettingsManagedPriorityAndEndpointPrivacy(t *testing.T) {
	t.Setenv("JINDIDRAMA_DATA_DIR", t.TempDir())
	client := appleCMSSettingsFixture(t)
	a := New()
	if err := prepareAppleCMS(a, "", client); err != nil {
		t.Fatal(err)
	}
	before, err := a.saveAppleCMSURL("https://catalog.example.test", client)
	if err != nil {
		t.Fatal(err)
	}
	a.close()

	for _, tc := range []struct {
		destination string
		want        error
	}{
		{"https://managed.example.test/operator/api?access=managed-private", nil},
		{"http://managed.example.test/operator/api?access=managed-private", errAppleCMSConfig},
	} {
		managed := New()
		if err := managed.openStore(""); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(managed.close)
		if err := prepareAppleCMS(managed, tc.destination, client); err != tc.want {
			t.Fatalf("managed startup error = %v, want %v", err, tc.want)
		}
		config := managed.AppleCMSConfiguration()
		wire, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		if !config.Managed || config.CanConfigure || config.Active != (tc.want == nil) || config.RestartRequired ||
			strings.Contains(string(wire), "managed.example.test") ||
			strings.Contains(string(wire), "managed-private") || config.SavedURL != "" {
			t.Fatalf("managed config exposed endpoint or editing: %s", wire)
		}
		if tc.want == nil {
			if _, err := managed.CatalogPage("", 1); err != nil {
				t.Fatal(err)
			}
		} else if config.ReadError != tc.want.Error() || managed.ProviderName() != "mock" {
			t.Fatalf("invalid managed startup: %+v, provider=%s", config, managed.ProviderName())
		}
		if err := managed.TestAppleCMSConnection("https://empty.example.test"); err != errAppleCMSManaged {
			t.Fatalf("managed test: %v", err)
		}
		if _, err := managed.SaveAppleCMSURL("https://empty.example.test"); err != errAppleCMSManaged {
			t.Fatalf("managed save: %v", err)
		}
		if _, err := managed.ClearAppleCMSURL(); err != errAppleCMSManaged {
			t.Fatalf("managed clear: %v", err)
		}
		raw, found, err := managed.store.GetSetting(appleCMSURLKey)
		if err != nil || !found || raw != `"`+before.SavedURL+`"` {
			t.Fatalf("managed startup or binding changed local value: %q, %v", raw, err)
		}
	}
}

func TestAppleCMSSettingsReadErrorsAndStorageFallback(t *testing.T) {
	t.Setenv("JINDIDRAMA_DATA_DIR", t.TempDir())
	client := appleCMSSettingsFixture(t)
	a := New()
	if err := a.openStore(""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	if err := a.store.SetSetting(appleCMSURLKey, `"https://catalog.example.test/?token=stored-private"`); err != nil {
		t.Fatal(err)
	}
	if err := prepareAppleCMS(a, "", client); err != errAppleCMSStoredURL {
		t.Fatalf("invalid stored value: %v", err)
	}
	if config := a.AppleCMSConfiguration(); config != (AppleCMSConfiguration{CanConfigure: true, ReadError: errAppleCMSStoredURL.Error()}) || a.ProviderName() != "mock" {
		t.Fatalf("invalid config: %+v, provider=%s", config, a.ProviderName())
	}
	if config, err := a.ClearAppleCMSURL(); err != nil || config != (AppleCMSConfiguration{CanConfigure: true}) {
		t.Fatalf("failed to clear an unreadable saved value: %+v, %v", config, err)
	}
	t.Setenv("JINDIDRAMA_DATA_DIR", ".")
	noStore := New()
	if err := prepareAppleCMS(noStore, "", client); err != errAppleCMSRead {
		t.Fatalf("unavailable storage: %v", err)
	}
	if config := noStore.AppleCMSConfiguration(); config != (AppleCMSConfiguration{ReadError: errAppleCMSRead.Error()}) || noStore.ProviderName() != "mock" {
		t.Fatalf("storage fallback: %+v, provider=%s", config, noStore.ProviderName())
	}
	if err := prepareAppleCMS(noStore, "https://managed.example.test/operator/api?access=managed-private", client); err != nil ||
		noStore.AppleCMSConfiguration() != (AppleCMSConfiguration{Active: true, Managed: true}) {
		t.Fatalf("managed storage fallback: %+v, err=%v", noStore.AppleCMSConfiguration(), err)
	}
}

func TestAppleCMSSettingsRecoveredStoreReportsMissedStartup(t *testing.T) {
	for _, failure := range []string{"open", "read"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("JINDIDRAMA_DATA_DIR", dir)
			client := appleCMSSettingsFixture(t)
			a := New()
			if err := a.openStore(""); err != nil {
				t.Fatal(err)
			}
			saved, err := a.saveAppleCMSURL("https://catalog.example.test", client)
			if err != nil {
				t.Fatal(err)
			}
			a.close()

			a = New()
			t.Cleanup(a.close)
			if failure == "open" {
				t.Setenv("JINDIDRAMA_DATA_DIR", ".")
			} else {
				if err := a.openStore(""); err != nil {
					t.Fatal(err)
				}
				if err := a.store.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := prepareAppleCMS(a, "", client); err != errAppleCMSRead {
				t.Fatalf("startup error = %v", err)
			}
			t.Setenv("JINDIDRAMA_DATA_DIR", dir)
			if err := a.openStore(""); err != nil {
				t.Fatal(err)
			}
			want := AppleCMSConfiguration{CanConfigure: true, SavedURL: saved.SavedURL, RestartRequired: true, ReadError: errAppleCMSRead.Error()}
			if got := a.AppleCMSConfiguration(); got != want || a.ProviderName() != "mock" {
				t.Fatalf("recovered profile: %+v, provider=%s", got, a.ProviderName())
			}

			var updated AppleCMSConfiguration
			if failure == "open" {
				updated, err = a.saveAppleCMSURL("https://empty.example.test", client)
				want = AppleCMSConfiguration{CanConfigure: true, SavedURL: "https://empty.example.test/api.php/provide/vod/", RestartRequired: true}
			} else {
				updated, err = a.ClearAppleCMSURL()
				want = AppleCMSConfiguration{CanConfigure: true}
			}
			if err != nil || updated != want || a.AppleCMSConfiguration() != want || a.ProviderName() != "mock" {
				t.Fatalf("edited after recovery: %+v, provider=%s, err=%v", updated, a.ProviderName(), err)
			}
			restarted := New()
			t.Cleanup(restarted.close)
			if err := prepareAppleCMS(restarted, "", client); err != nil {
				t.Fatal(err)
			}
			if got := restarted.AppleCMSConfiguration(); got.SavedURL != want.SavedURL || got.Active != (want.SavedURL != "") ||
				got.RestartRequired || got.ReadError != "" {
				t.Fatalf("restarted after edit: %+v", got)
			}
		})
	}
}

func TestAppleCMSResumeIDsFilterWrongSourcesBeforeLimit(t *testing.T) {
	a := New()
	t.Cleanup(a.close)
	client := appleCMSSettingsFixture(t)
	if err := configureAppleCMS(a, "https://catalog.example.test", client); err != nil {
		t.Fatal(err)
	}
	page, err := a.CatalogPage("", 1)
	if err != nil || len(page.Entries) != 1 {
		t.Fatalf("catalog fixture: %+v, %v", page, err)
	}
	id := page.Entries[0].ID
	prefix := id[:strings.LastIndexByte(id, ':')+1]
	var ids, want []string
	for i := 1; i <= 15; i++ {
		ids = append(ids, "apple-cms:ffffffffffffffffffffffffffffffff:"+strconv.Itoa(i))
	}
	ids = append(ids, prefix+"invalid")
	for i := 20; i <= 33; i++ {
		current := prefix + strconv.Itoa(i)
		ids = append(ids, current)
		if len(want) < 12 {
			want = append(want, current)
		}
	}
	if got := a.CatalogResumeIDs(ids); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("candidate bookmark IDs = %v, want %v", got, want)
	}
	if got := New().CatalogResumeIDs(ids); len(got) != 0 {
		t.Fatalf("non-CMS provider returned bookmarks: %v", got)
	}
}
