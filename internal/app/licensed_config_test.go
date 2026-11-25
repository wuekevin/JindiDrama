package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"jindi-drama/internal/provider/licensed"
)

func TestSearchByDemoCastName(t *testing.T) {
	app := New()
	cases := []struct {
		query string
		want  []string
	}{
		{query: " 苏晚 ", want: []string{"d1", "d3"}},
		{query: "萧景渊", want: []string{"d1", "d2"}},
		{query: "不存在的主演"},
	}
	for _, tt := range cases {
		results, err := app.Search(tt.query)
		if err != nil {
			t.Fatalf("search %q: %v", tt.query, err)
		}
		var ids []string
		for _, drama := range results {
			ids = append(ids, drama.ID)
		}
		if !slices.Equal(ids, tt.want) {
			t.Errorf("search %q: got %v, want %v", tt.query, ids, tt.want)
		}
	}
}

func TestSearchIncludesActiveLicensedCatalogAndToleratesInvalidConfiguration(t *testing.T) {
	a := New()
	keywords := []string{"替嫁", ""}
	for _, query := range keywords {
		baseline, err := a.Search(query)
		if err != nil || len(baseline) == 0 {
			t.Fatalf("baseline search %q: %v, %v", query, baseline, err)
		}
	}
	ConfigureLicensedMedia(a, nil, errors.New("invalid catalog"))
	if results, err := a.Search("替嫁"); err != nil || len(results) == 0 {
		t.Fatalf("invalid catalog prevented demo search: %v, %v", results, err)
	}
	media, err := licensed.New(playableLicensedDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = media.Close() })
	ConfigureLicensedMedia(a, media, nil)
	if results, err := a.Search(" 本地素材 "); err != nil || len(results) != 1 || results[0].ID != "local-demo" {
		t.Fatalf("licensed title search: %v, %v", results, err)
	}
	if results, err := a.Search(""); err != nil || len(results) < 2 || results[0].ID != "local-demo" {
		t.Fatalf("combined search: %v, %v", results, err)
	}
	if results, err := a.Search("替嫁"); err != nil || len(results) == 0 || results[0].ID == "local-demo" {
		t.Fatalf("demo search after activation: %v, %v", results, err)
	}
}

func playableLicensedDirectory(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(`[{"id":"local-demo","title":"本地素材","license":"自有","episodes":[["demo.mp4"]]}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo.mp4"), []byte("catalog fixture media"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLicensedDirectoryPersistsAndInvalidChoiceDoesNotReplaceIt(t *testing.T) {
	database := filepath.Join(t.TempDir(), "settings.db")
	a := New()
	if err := a.openStore(database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	if initial, err := PrepareLicensedDirectory(a, ""); err != nil || initial != "" {
		t.Fatalf("initial directory: %q %v", initial, err)
	}
	directory := playableLicensedDirectory(t)
	choose := func(dir string) func() (string, error) { return func() (string, error) { return dir, nil } }
	config, err := a.chooseLicensedDirectory(choose(directory))
	if err != nil || config.SavedDirectory != directory || config.ActiveDirectory != "" || !config.RestartRequired {
		t.Fatalf("pending directory: %+v %v", config, err)
	}
	if _, err := a.chooseLicensedDirectory(choose(t.TempDir())); err == nil {
		t.Fatal("accepted invalid media directory")
	}
	cancelled, err := a.chooseLicensedDirectory(choose(""))
	if err != nil || cancelled != config {
		t.Fatalf("cancel replaced directory: %+v %v", cancelled, err)
	}
	if err := a.SaveSetting(licensedDirectoryKey, `"/unexpected"`); err == nil {
		t.Fatal("the generic settings binding changed the licensed directory")
	}
	a.close()

	started := New()
	if err := started.openStore(database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(started.close)
	selected, err := PrepareLicensedDirectory(started, "")
	if err != nil || selected != directory {
		t.Fatalf("reopened directory: %q %v", selected, err)
	}
	media, err := licensed.New(selected)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = media.Close() })
	ConfigureLicensedMedia(started, media, nil)
	entries, err := started.LicensedCatalog()
	if err != nil || len(entries) != 1 || entries[0].ID != "local-demo" {
		t.Fatalf("reopened catalog: %+v %v", entries, err)
	}
	applied := started.LicensedConfiguration()
	if applied.RestartRequired || applied.ActiveDirectory != directory {
		t.Fatalf("applied configuration: %+v", applied)
	}
	cleared, err := started.ClearLicensedDirectory()
	if err != nil || cleared.SavedDirectory != "" || cleared.ActiveDirectory != directory || !cleared.RestartRequired {
		t.Fatalf("clear pending: %+v %v", cleared, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "demo.mp4")); err != nil {
		t.Fatalf("clearing the setting modified media: %v", err)
	}
	if selected, err := PrepareLicensedDirectory(started, ""); err != nil || selected != "" {
		t.Fatalf("cleared configuration: %q %v", selected, err)
	}
}

func TestManagedLicensedDirectoryOverridesSavedPreference(t *testing.T) {
	a := New()
	if err := a.openStore(filepath.Join(t.TempDir(), "settings.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	saved := playableLicensedDirectory(t)
	if _, err := a.saveLicensedDirectory(saved); err != nil {
		t.Fatal(err)
	}
	managed := playableLicensedDirectory(t)
	active, err := PrepareLicensedDirectory(a, managed)
	if err != nil || active != managed {
		t.Fatalf("managed directory: %q %v", active, err)
	}
	status := a.LicensedConfiguration()
	if !status.Managed || status.CanConfigure || status.StartupDirectory != managed {
		t.Fatalf("managed configuration: %+v", status)
	}
	called := false
	if _, err := a.chooseLicensedDirectory(func() (string, error) { called = true; return saved, nil }); err == nil || called {
		t.Fatalf("managed directory picker should not open: %v", err)
	}
	if _, err := a.ClearLicensedDirectory(); err == nil {
		t.Fatal("cleared an operator-managed directory")
	}
	if active, err := PrepareLicensedDirectory(a, ""); err != nil || active != saved {
		t.Fatalf("saved preference lost after managed override: %q %v", active, err)
	}
}
