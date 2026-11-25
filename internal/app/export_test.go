package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/licensed"
)

func TestDemoExportUsesPlaybackAssetAndPreservesExistingFile(t *testing.T) {
	a := New()
	asset, err := a.demoExportAsset("d1", 1, "hd")
	if err != nil || asset != "demo/demo-2.mp4" {
		t.Fatalf("asset: %q, %v", asset, err)
	}
	if _, err := a.demoExportAsset("d1", 33, "sd"); !errors.Is(err, provider.ErrEpisodeUnavailable) {
		t.Fatalf("unavailable episode: %v", err)
	}
	if sd, err := a.demoExportAsset("d1", 1, "sd"); err != nil || sd != "demo/demo-2-sd.mp4" {
		t.Fatalf("sd asset: %q, %v", sd, err)
	}
	if _, err := a.demoExportAsset("d1", 1, "4k"); err == nil {
		t.Fatal("unsupported quality accepted")
	}
	assets := fstest.MapFS{asset: {Data: []byte("demo payload")}}
	target := filepath.Join(t.TempDir(), "clip.mp4")
	if err := exportAsset(assets, asset, target); err != nil {
		t.Fatal(err)
	}
	assets[asset].Data = []byte("must not overwrite")
	if err := exportAsset(assets, asset, target); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("existing file: %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "demo payload" {
		t.Fatalf("exported file: %q, %v", data, err)
	}
}

func TestLicensedEpisodeExport(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"catalog.json": `[
			{"id":"local-play","title":"播放","license":"仅播放","episodes":[["episode.mp4"]]},
			{"id":"local-copy","title":"复制","license":"允许复制","allowExport":true,"episodes":[["episode.webm","episode.mp4"],["episode.webm"]]}
		]`,
		"episode.mp4":  "mp4 bytes",
		"episode.webm": "webm bytes",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := licensed.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	a := New()
	ConfigureLicensedMedia(a, p, nil)
	unexpectedPicker := func(runtime.SaveDialogOptions) (string, error) {
		t.Fatal("dialog opened for an unavailable export")
		return "", nil
	}
	for _, id := range []string{"local-play", "local-missing", "../episode", "d1"} {
		if saved, err := a.exportLicensedEpisode(id, 1, unexpectedPicker); saved || err == nil {
			t.Fatalf("unexpected result for %q: %v, %v", id, saved, err)
		}
	}
	if _, err := a.exportLicensedEpisode("local-copy", 3, unexpectedPicker); !errors.Is(err, provider.ErrEpisodeUnavailable) {
		t.Fatalf("unavailable episode: %v", err)
	}
	if saved, err := a.exportLicensedEpisode("local-copy", 1, func(runtime.SaveDialogOptions) (string, error) {
		return "", nil
	}); saved || err != nil {
		t.Fatalf("cancel: %v, %v", saved, err)
	}
	output := t.TempDir()
	for _, item := range []struct {
		episode int
		name    string
		data    string
	}{
		{1, "JindiDrama-local-copy-1.mp4", "mp4 bytes"},
		{2, "JindiDrama-local-copy-2.webm", "webm bytes"},
	} {
		target := filepath.Join(output, item.name)
		pick := func(options runtime.SaveDialogOptions) (string, error) {
			if options.DefaultFilename != item.name {
				t.Fatalf("default filename: %q", options.DefaultFilename)
			}
			return target, nil
		}
		if saved, err := a.exportLicensedEpisode("local-copy", item.episode, pick); !saved || err != nil {
			t.Fatalf("save episode %d: %v, %v", item.episode, saved, err)
		}
		if saved, err := a.exportLicensedEpisode("local-copy", item.episode, pick); saved || err == nil {
			t.Fatalf("overwrote existing file: %v, %v", saved, err)
		}
		if data, err := os.ReadFile(target); err != nil || string(data) != item.data {
			t.Fatalf("file content: %q, %v", data, err)
		}
	}
	failedOutput := t.TempDir()
	if saved, err := a.exportLicensedEpisode("local-copy", 1, func(runtime.SaveDialogOptions) (string, error) {
		if err := os.Truncate(filepath.Join(dir, "episode.mp4"), 3); err != nil {
			t.Fatal(err)
		}
		return filepath.Join(failedOutput, "truncated.mp4"), nil
	}); saved || err == nil {
		t.Fatalf("truncated source appeared to save: %v, %v", saved, err)
	}
	if entries, err := os.ReadDir(failedOutput); err != nil || len(entries) != 0 {
		t.Fatalf("failed export left files behind: %v, %v", entries, err)
	}
}

func TestPublishExportRejectsExistingDestination(t *testing.T) {
	dir := t.TempDir()
	staged, target := filepath.Join(dir, "staged"), filepath.Join(dir, "target")
	if err := os.WriteFile(staged, []byte("new bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("existing bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := publishExport(staged, target); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("existing target: %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "existing bytes" {
		t.Fatalf("target content: %q, %v", data, err)
	}
}

func TestWindowCloseWaitsForMediaExport(t *testing.T) {
	a := New()
	if !a.beginMediaExport() || a.beginMediaExport() {
		t.Fatal("expected one active export")
	}
	done := a.prepareExportShutdown()
	if done == nil || a.beginMediaExport() {
		t.Fatal("closing should wait for the active export and refuse new work")
	}
	select {
	case <-done:
		t.Fatal("shutdown released before the export finished")
	default:
	}
	a.endMediaExport()
	select {
	case <-done:
	default:
		t.Fatal("export completion did not release shutdown")
	}
	if a.beginMediaExport() {
		t.Fatal("window should still be closing")
	}
}

func TestDemoExportRejectsNewWorkDuringShutdown(t *testing.T) {
	a := NewWithAssets(fstest.MapFS{})
	a.ctx = context.Background()
	a.prepareExportShutdown()
	saved, err := a.ExportDemoEpisode("d1", 1, "hd")
	if saved || err == nil || err.Error() != "已有保存操作进行中，或窗口正在关闭" {
		t.Fatalf("demo export during shutdown: saved=%v, err=%v", saved, err)
	}
}
