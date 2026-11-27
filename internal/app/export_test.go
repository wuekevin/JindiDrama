package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
	"testing/fstest"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/licensed"
)

func TestStagedExportLeavesReplacementDirectoryAlone(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Windows retargeting is exercised with a directory link")
	}
	root := t.TempDir()
	directory := filepath.Join(root, "chosen")
	moved := filepath.Join(root, "moved")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "capture.png")
	var replacement string
	err := writePrivateStagedExport(target, func(out io.Writer) error {
		if _, err := io.WriteString(out, "captured data"); err != nil {
			return err
		}
		stagingPath, err := filepath.Rel(directory, out.(*os.File).Name())
		if err != nil {
			return err
		}
		if err := os.Rename(directory, moved); err != nil {
			return err
		}
		if err := os.Mkdir(directory, 0o700); err != nil {
			return err
		}
		replacement = filepath.Join(directory, stagingPath)
		if err := os.MkdirAll(filepath.Dir(replacement), 0o700); err != nil {
			return err
		}
		return os.WriteFile(replacement, []byte("unrelated content"), 0o600)
	})
	if !errors.Is(err, errUpdateDestinationMoved) {
		t.Fatalf("expected destination-change error, got %v", err)
	}
	if data, err := os.ReadFile(replacement); err != nil || string(data) != "unrelated content" {
		t.Fatalf("replacement contents changed: %q, %v", data, err)
	}
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unexpected output in replacement directory: %v", err)
	}
	if entries, err := os.ReadDir(moved); err != nil || len(entries) != 0 {
		t.Fatalf("temporary data remained in the moved directory: %v, %v", entries, err)
	}
}

func TestStagedExportReportsCleanupFailureAfterPublishing(t *testing.T) {
	target := filepath.Join(t.TempDir(), "capture.png")
	var otherFile string
	err := writePrivateStagedExport(target, func(out io.Writer) error {
		otherFile = filepath.Join(filepath.Dir(out.(*os.File).Name()), "unrelated")
		if err := os.WriteFile(otherFile, []byte("unrelated content"), 0o600); err != nil {
			return err
		}
		_, err := io.WriteString(out, "captured data")
		return err
	})
	if !errors.Is(err, errExportCleanup) {
		t.Fatalf("expected cleanup failure after publication, got %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "captured data" {
		t.Fatalf("published data lost: %q, %v", data, err)
	}
	if data, err := os.ReadFile(otherFile); err != nil || string(data) != "unrelated content" {
		t.Fatalf("unrelated file changed: %q, %v", data, err)
	}
}

func TestDemoDirectExportRemainsInSelectedDirectory(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Windows can reject renaming a directory containing an open file")
	}
	root := t.TempDir()
	selected := filepath.Join(root, "selected")
	moved := filepath.Join(root, "moved")
	if err := os.Mkdir(selected, 0o700); err != nil {
		t.Fatal(err)
	}
	name := "episode.mp4"
	target := filepath.Join(selected, name)
	asset := "demo/episode.mp4"
	payload := []byte("embedded demo")
	fallback, changed := false, false
	assets := observedDemoAssets{FS: fstest.MapFS{asset: {Data: payload}}}
	assets.beforeRead = func() error {
		if !fallback || changed {
			return nil
		}
		changed = true
		if err := os.Rename(selected, moved); err != nil {
			return err
		}
		if err := os.Mkdir(selected, 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, []byte("existing data"), 0o600)
	}
	err := exportDemoAsset(assets, asset, target, func(*stagedOutput, string) error {
		fallback = true
		return errors.ErrUnsupported
	})
	if !errors.Is(err, errUpdateDestinationMovedAfterPublish) {
		t.Fatalf("expected moved-after-saving error, got %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "existing data" {
		t.Fatalf("replacement file changed: %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(moved, name)); err != nil || string(data) != string(payload) {
		t.Fatalf("original directory lost the saved video: %q, %v", data, err)
	}
}

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

type observedDemoAssets struct {
	fs.FS
	beforeRead func() error
}

func (assets observedDemoAssets) Open(name string) (fs.File, error) {
	file, err := assets.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return observedDemoAsset{File: file, beforeRead: assets.beforeRead}, nil
}

type observedDemoAsset struct {
	fs.File
	beforeRead func() error
}

func (asset observedDemoAsset) Read(buffer []byte) (int, error) {
	if err := asset.beforeRead(); err != nil {
		return 0, err
	}
	return asset.File.Read(buffer[:min(len(buffer), 4)])
}

func TestDemoExportPublishesOnlyAfterCopy(t *testing.T) {
	const assetName = "demo/episode.mp4"
	failedRead := errors.New("source interrupted")
	for _, scenario := range []struct {
		name          string
		duringCopy    func(string) error
		expectedError error
		expectedData  string
	}{
		{name: "complete", duringCopy: func(string) error { return nil }, expectedData: "demo video"},
		{name: "read failure", duringCopy: func(string) error { return failedRead }, expectedError: failedRead},
		{name: "name taken during copy", duringCopy: func(path string) error {
			return os.WriteFile(path, []byte("other program"), 0o600)
		}, expectedError: fs.ErrExist, expectedData: "other program"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			directory := t.TempDir()
			target := filepath.Join(directory, "episode.mp4")
			calls := 0
			assets := observedDemoAssets{FS: fstest.MapFS{assetName: {Data: []byte("demo video")}}}
			assets.beforeRead = func() error {
				calls++
				if calls != 2 {
					return nil
				}
				if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("target became visible before copying ended: %v", err)
				}
				return scenario.duringCopy(target)
			}
			if err := exportAsset(assets, assetName, target); !errors.Is(err, scenario.expectedError) {
				t.Fatalf("export result: %v, want %v", err, scenario.expectedError)
			}
			if calls < 2 {
				t.Fatalf("expected more than one read, got %d", calls)
			}
			data, err := os.ReadFile(target)
			if scenario.expectedData == "" {
				if !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("failed copy left target: %q, %v", data, err)
				}
			} else if err != nil || string(data) != scenario.expectedData {
				t.Fatalf("target content: %q, %v", data, err)
			}
			if paths, err := filepath.Glob(filepath.Join(directory, ".jindidrama-export-*")); err != nil || len(paths) != 0 {
				t.Fatalf("export left temporary files: %v, %v", paths, err)
			}
		})
	}
}

func TestDemoExportFallbackCopiesWithoutReplacingDestination(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "episode.mp4")
	const asset = "demo/episode.mp4"
	for _, collision := range []bool{false, true} {
		t.Run(fmt.Sprintf("collision=%v", collision), func(t *testing.T) {
			if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatal(err)
			}
			expected := "embedded demo"
			assets := fstest.MapFS{asset: {Data: []byte(expected)}}
			var staged string
			publish := func(stage *stagedOutput, to string) error {
				staged = stage.file.Name()
				if collision {
					expected = "other program"
					if err := os.WriteFile(to, []byte(expected), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return errors.ErrUnsupported
			}
			observed := observedDemoAssets{FS: assets, beforeRead: func() error {
				if staged != "" {
					if _, err := os.Lstat(staged); !errors.Is(err, fs.ErrNotExist) {
						t.Fatalf("fallback began before staged file was removed: %v", err)
					}
				}
				return nil
			}}
			err := exportDemoAsset(observed, asset, target, publish)
			if (!collision && err != nil) || (collision && !errors.Is(err, fs.ErrExist)) {
				t.Fatalf("fallback result: %v", err)
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != expected {
				t.Fatalf("target content: %q, %v", data, err)
			}
		})
	}
}

func TestDemoExportFallbackFailureClearsOnlyOpenFile(t *testing.T) {
	const asset = "demo/episode.mp4"
	target := filepath.Join(t.TempDir(), "episode.mp4")
	assets := observedDemoAssets{FS: fstest.MapFS{asset: {Data: []byte("embedded demo")}}}
	fallbackReads := 0
	useFallback := false
	assets.beforeRead = func() error {
		if useFallback {
			fallbackReads++
			if fallbackReads == 2 {
				return errors.New("source interrupted")
			}
		}
		return nil
	}
	err := exportDemoAsset(assets, asset, target, func(*stagedOutput, string) error {
		useFallback = true
		return errors.ErrUnsupported
	})
	if !errors.Is(err, errMediaDirectCopy) {
		t.Fatalf("fallback failure: %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || len(data) != 0 {
		t.Fatalf("failed fallback output: %q, %v", data, err)
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

func TestLicensedExportFallsBackToAvailableFormat(t *testing.T) {
	for _, change := range []string{"removed", "empty"} {
		t.Run(change, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range map[string]string{
				"catalog.json": `[{"id":"local-alternate","title":"替代片源","license":"允许复制","allowExport":true,"episodes":[["episode.webm","episode.mp4"]]}]`,
				"episode.mp4":  "mp4 fixture", "episode.webm": "webm fixture",
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			media, err := licensed.New(dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = media.Close() })
			a := New()
			ConfigureLicensedMedia(a, media, nil)
			mp4 := filepath.Join(dir, "episode.mp4")
			if change == "removed" {
				err = os.Remove(mp4)
			} else {
				err = os.Truncate(mp4, 0)
			}
			if err != nil {
				t.Fatal(err)
			}

			const filename = "JindiDrama-local-alternate-1.webm"
			target := filepath.Join(t.TempDir(), filename)
			saved, err := a.exportLicensedEpisode("local-alternate", 1, func(options runtime.SaveDialogOptions) (string, error) {
				if options.DefaultFilename != filename {
					t.Fatalf("suggested filename: %q", options.DefaultFilename)
				}
				return target, nil
			})
			if err != nil || !saved {
				t.Fatalf("save alternate format: saved=%t err=%v", saved, err)
			}
			body, err := os.ReadFile(target)
			if err != nil || string(body) != "webm fixture" {
				t.Fatalf("exported data: %q, %v", body, err)
			}

			if err := os.Remove(filepath.Join(dir, "episode.webm")); err != nil {
				t.Fatal(err)
			}
			saved, err = a.exportLicensedEpisode("local-alternate", 1, func(runtime.SaveDialogOptions) (string, error) {
				t.Fatal("save dialog opened with no readable sources")
				return "", nil
			})
			if err == nil || saved {
				t.Fatalf("all formats unavailable: saved=%t err=%v", saved, err)
			}
		})
	}
}

func TestLicensedExportFallbackReplaysAndRechecksSource(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("source-changed=%v", changed), func(t *testing.T) {
			dir := t.TempDir()
			sourcePath := filepath.Join(dir, "source.mp4")
			target := filepath.Join(dir, "destination.mp4")
			const original = "licensed source"
			if err := os.WriteFile(sourcePath, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			source, err := os.Open(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			info, err := source.Stat()
			if err != nil {
				t.Fatal(err)
			}
			passes := 0
			err = stageMediaExport(target, func(output io.Writer) error {
				passes++
				return copyLicensedExport(source, info, output)
			}, func(*stagedOutput, string) error {
				if changed {
					if err := os.WriteFile(sourcePath, []byte(original+" extra"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return errors.ErrUnsupported
			})
			if passes != 2 {
				t.Fatalf("copy passes: %d", passes)
			}
			data, readErr := os.ReadFile(target)
			if changed {
				if !errors.Is(err, errMediaDirectCopy) || readErr != nil || len(data) != 0 {
					t.Fatalf("changed source: %q, %v, %v", data, readErr, err)
				}
			} else if err != nil || readErr != nil || string(data) != original {
				t.Fatalf("direct copy: %q, %v, %v", data, readErr, err)
			}
		})
	}
}
