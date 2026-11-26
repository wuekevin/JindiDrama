package app

import (
	"context"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestWebViewBindingSurface(t *testing.T) {
	// Wails registers exported methods of the bound App value.
	want := strings.Fields(`
AppVersion AppleCMSConfiguration AutostartEnabled BackupNow CacheClear CacheSetLimitMB CacheStats
CancelUpdateDownload Capabilities CatalogDetail CatalogDrama CatalogEpisodes CatalogGenrePage CatalogGenres CatalogPage CatalogPlayback CatalogResumeIDs CatalogSearch
CheckUpdate ChooseCacheDirectory ChooseLicensedDirectory ClearAppleCMSURL ClearHistory ClearLicensedDirectory CloseWindow CompareAndSaveLocalComments DiagnosticsTail DownloadUpdate DownloadUpdateFile Drama Episodes ExportDemoEpisode ExportDemoScreenshot ExportLicensedEpisode
Favorites GetSetting HideWindowToTray History Home HomeFeed LicensedCatalog LicensedConfiguration LocalComments LocalCommentsState LocalDanmaku LocalDataProfile LocalEpisodeLiked LogEvent MarkFrontendReady
OpenReleasePage ProviderName RecordHistory ReleaseCatalogPlayback RemoveHistory ResetCacheDirectory ResolvePlayback RestoreLibraryBackup SaveAppleCMSURL SaveFavorites
SaveLibraryChanges SaveLocalComments SaveSearchHistory SaveSearchHistoryRevision SaveSetting Search SearchHistory SearchHistoryState
SendLocalDanmaku SendLocalDanmakuOnce SetAutostart SetLocalEpisodeLiked SetUpdateChannel SetWindowCloseAction StorageReady TestAppleCMSConnection UpdateChannel WindowCloseAction WindowTrayAvailable
`)
	appType := reflect.TypeOf((*App)(nil))
	got := make([]string, appType.NumMethod())
	for i := range got {
		got[i] = appType.Method(i).Name
	}
	if !slices.Equal(got, want) {
		t.Fatalf("exported App methods changed\nwant: %v\n got: %v", want, got)
	}
}

func TestLifecycleClosesStore(t *testing.T) {
	a := New()
	if err := a.openStore(filepath.Join(t.TempDir(), "jindi.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.close)
	_, shutdown := Lifecycle(a)
	shutdown(context.Background())
	if _, err := a.History(); err == nil {
		t.Fatal("expected database to be closed after shutdown")
	}
}
