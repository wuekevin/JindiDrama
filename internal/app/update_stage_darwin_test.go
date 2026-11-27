//go:build cgo

package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

	"jindi-drama/internal/update"
)

func TestUpdatePublicationPinsStagingDirectory(t *testing.T) {
	parent := t.TempDir()
	stage, err := createUpdateStage(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.cleanup()
	if _, err := io.WriteString(stage.file, "verified"); err != nil {
		t.Fatal(err)
	}
	stagingDir := filepath.Dir(stage.file.Name())
	if err := os.Rename(stagingDir, stagingDir+"-moved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stagingDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage.file.Name(), []byte("substituted"), 0600); err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(parent, "update.zip")
	if err := stage.publish(final); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(final); err != nil || string(content) != "verified" {
		t.Fatalf("publication used the replacement path: %q, %v", content, err)
	}
	if size, err := unix.Getxattr(final, "com.apple.quarantine", nil); err != nil || size == 0 {
		t.Fatalf("download origin was not preserved: %d, %v", size, err)
	}
}

func TestSaveUpdateRejectsMovedDestination(t *testing.T) {
	for _, tc := range []struct {
		name    string
		replace bool
	}{
		{name: "moved"},
		{name: "replaced", replace: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			parent := filepath.Join(root, "selected")
			moved := filepath.Join(root, "moved")
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			saved, err := saveTestUpdate(context.Background(), filepath.Join(parent, "update.zip"), func(_ context.Context, w io.Writer) error {
				if _, err := io.WriteString(w, "verified"); err != nil {
					return err
				}
				if err := os.Rename(parent, moved); err != nil {
					return err
				}
				if tc.replace {
					return os.Mkdir(parent, 0o700)
				}
				return nil
			}, func(publish func() error) error { return publish() })
			if saved || !errors.Is(err, errUpdateDestinationMoved) {
				t.Fatalf("moved destination: saved=%v, err=%v", saved, err)
			}
			entries, err := os.ReadDir(moved)
			if err != nil || len(entries) != 0 {
				t.Fatalf("moved directory contains staged or published data: %v, %v", entries, err)
			}
			if tc.replace {
				entries, err := os.ReadDir(parent)
				if err != nil || len(entries) != 0 {
					t.Fatalf("replacement directory changed: %v, %v", entries, err)
				}
			}
		})
	}
}

func TestUpdateCleanupPreservesFileCreatedAfterPublication(t *testing.T) {
	parent := t.TempDir()
	stage, err := createUpdateStage(parent)
	if err != nil {
		t.Fatal(err)
	}
	cleaned := false
	defer func() {
		if !cleaned {
			_ = stage.cleanup()
		}
	}()
	path := filepath.Join(parent, "update.zip")
	originalName := stage.file.Name()
	if _, err := io.WriteString(stage.file, "verified"); err != nil {
		t.Fatal(err)
	}
	if err := stage.publish(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(originalName, []byte("unrelated"), 0600); err != nil {
		t.Fatal(err)
	}
	err = stage.cleanup()
	cleaned = true
	if !errors.Is(err, unix.ENOTEMPTY) {
		t.Errorf("cleanup of non-empty staging directory: %v", err)
	}
	if payload, err := os.ReadFile(originalName); err != nil || string(payload) != "unrelated" {
		t.Errorf("unrelated file changed: %q, %v", payload, err)
	}
	if payload, err := os.ReadFile(path); err != nil || string(payload) != "verified" {
		t.Errorf("published package changed: %q, %v", payload, err)
	}
}

func TestUpdateAPFSPublishesOpenFileWithQuarantine(t *testing.T) {
	parent := t.TempDir()
	var volume unix.Statfs_t
	if err := unix.Statfs(parent, &volume); err != nil {
		t.Fatal(err)
	}
	if unix.ByteSliceToString(volume.Fstypename[:]) != "apfs" {
		t.Skip("requires APFS")
	}

	path := filepath.Join(parent, "update.zip")
	const payload = "verified package fixture"
	wantDigest := sha256.Sum256([]byte(payload))
	var stagePath string
	saved, err := saveTestUpdate(context.Background(), path, func(_ context.Context, out io.Writer) error {
		stagePath = out.(*os.File).Name()
		if _, err := os.Lstat(stagePath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("download file has a directory entry: %v", err)
		}
		if err := os.WriteFile(stagePath, []byte("unrelated"), 0600); err != nil {
			return err
		}
		_, err := io.WriteString(out, payload)
		return err
	}, func(publish func() error) error { return publish() })
	if !saved || !errors.Is(err, errUpdateSavedCleanup) {
		t.Errorf("published update result: %v, %v", saved, err)
	}
	if data, err := os.ReadFile(path); err != nil || sha256.Sum256(data) != wantDigest {
		t.Errorf("saved update SHA-256: %x, %v", sha256.Sum256(data), err)
	}
	if data, err := os.ReadFile(stagePath); err != nil || string(data) != "unrelated" {
		t.Errorf("independent file changed: %q, %v", data, err)
	}
	if size, err := unix.Getxattr(path, "com.apple.quarantine", nil); err != nil || size == 0 {
		t.Errorf("download origin was not preserved: %d, %v", size, err)
	}
}

// Run with TMPDIR on HFS+ to exercise the named staging path on a real volume.
func TestUpdateNamedStagePreservesUnrelatedFile(t *testing.T) {
	parent := t.TempDir()
	var volume unix.Statfs_t
	if err := unix.Statfs(parent, &volume); err != nil {
		t.Fatal(err)
	}
	if unix.ByteSliceToString(volume.Fstypename[:]) != "hfs" {
		t.Skip("requires HFS+")
	}

	for _, scenario := range []string{"verified", "changed", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(parent, scenario+".zip")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			const payload = "verified package fixture"
			var stagePath string
			saved, err := saveTestUpdate(ctx, path, func(_ context.Context, out io.Writer) error {
				stagePath = out.(*os.File).Name()
				if _, err := io.WriteString(out, payload); err != nil {
					return err
				}
				if scenario == "verified" {
					return nil
				}
				if err := os.Rename(stagePath, stagePath+".original"); err != nil {
					return err
				}
				if scenario == "cancelled" {
					cancel()
				}
				return os.WriteFile(stagePath, []byte("unrelated"), 0600)
			}, func(publish func() error) error { return publish() })
			if scenario == "verified" {
				if !saved || err != nil {
					t.Fatalf("verified save: %v, %v", saved, err)
				}
				if data, err := os.ReadFile(path); err != nil || string(data) != payload {
					t.Errorf("saved update: %q, %v", data, err)
				}
				if size, err := unix.Getxattr(path, "com.apple.quarantine", nil); err != nil || size == 0 {
					t.Errorf("download origin was not preserved: %d, %v", size, err)
				}
				return
			}
			if saved || !errors.Is(err, errUpdateCleanup) {
				t.Errorf("save after staging entry changed: %v, %v", saved, err)
			}
			if scenario == "changed" && !errors.Is(err, errUpdateStageIdentity) {
				t.Errorf("missing staging identity failure: %v", err)
			}
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("update path exists: %v", err)
			}
			if data, err := os.ReadFile(stagePath); err != nil || string(data) != "unrelated" {
				t.Errorf("independent file changed: %q, %v", data, err)
			}
			if data, err := os.ReadFile(stagePath + ".original"); err != nil || string(data) != payload {
				t.Errorf("original staged file changed: %q, %v", data, err)
			}
		})
	}
}

func TestUpdatePreservesTargetCreatedBeforePublication(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "update.zip")
	const existing = "user's existing file"
	saved, err := saveTestUpdate(context.Background(), path, func(_ context.Context, out io.Writer) error {
		if _, err := io.WriteString(out, "verified"); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(existing), 0600)
	}, func(publish func() error) error { return publish() })
	if saved || err == nil || err.Error() != "目标文件已存在，请使用其他文件名" {
		t.Errorf("save with competing target: %v, %v", saved, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != existing {
		t.Errorf("existing file changed: %q, %v", data, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 1 {
		t.Errorf("staging not removed: %v, %v", entries, err)
	}
}

func TestUpdateNamedStageChecksSavedBytes(t *testing.T) {
	root := t.TempDir()
	var volume unix.Statfs_t
	if err := unix.Statfs(root, &volume); err != nil {
		t.Fatal(err)
	}
	if unix.ByteSliceToString(volume.Fstypename[:]) != "hfs" {
		t.Skip("requires HFS+")
	}

	const payload = "verified"
	digest := sha256.Sum256([]byte(payload))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v2.0.0","prerelease":false,"draft":false,
			"html_url":"https://github.com/wuekevin/JindiDrama/releases/tag/v2.0.0",
			"assets":[{"id":1,"name":"JindiDrama-macos-universal.zip","state":"uploaded","size":%d,"digest":"sha256:%x"}]}`, len(payload), digest)
	}))
	defer server.Close()
	release, err := update.Check(t.Context(), server.Client(), server.URL, "v1.0.0")
	if err != nil || release.Download == nil {
		t.Fatalf("test asset: %v", err)
	}

	for _, tc := range []struct {
		scenario string
		saved    bool
		err      error
		output   string
		checks   int
	}{
		{"verified", true, nil, payload, 2},
		{"changed_before_publish", false, nil, "", 1},
		{"changed_during_commit", false, errUpdatePublishedIdentity, "modified", 2},
		{"changed_during_readback", false, errUpdatePublishedIdentity, "modified", 2},
		{"mapped_during_readback", false, errUpdatePublishedIdentity, "modified", 2},
		{"unmapped_during_readback", false, errUpdatePublishedIdentity, "modified", 2},
		{"changed_after_publish", false, errUpdatePublishedIdentity, "modified", 2},
		{"replaced_during_readback", false, errUpdatePublishedIdentity, "unrelated", 2},
		{"destination_moved", false, errUpdateDestinationMovedAfterPublish, payload, 2},
		{"cancelled_before_publish", false, nil, "", 1},
		{"cancelled_after_publish", true, nil, payload, 2},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			parent := filepath.Join(root, tc.scenario)
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(parent, "update.zip")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			checks := 0
			var stagePath string
			saved, err := saveUpdate(ctx, path, func(_ context.Context, out io.Writer) error {
				if _, err := io.WriteString(out, payload); err != nil {
					return err
				}
				stagePath = out.(*os.File).Name()
				if tc.scenario == "changed_before_publish" {
					return os.WriteFile(stagePath, []byte("modified"), 0600)
				}
				return nil
			}, func(ctx context.Context, input io.Reader) error {
				checks++
				if tc.scenario == "cancelled_before_publish" {
					cancel()
				}
				if err := release.Download.VerifySaved(ctx, input); err != nil {
					return err
				}
				if tc.scenario == "changed_during_readback" && checks == 2 {
					f, err := os.OpenFile(path, os.O_WRONLY, 0)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := f.WriteAt([]byte("modified"), 0); err != nil {
						f.Close()
						t.Fatal(err)
					}
					if err := f.Close(); err != nil {
						t.Fatal(err)
					}
				}
				if (tc.scenario == "mapped_during_readback" || tc.scenario == "unmapped_during_readback") && checks == 2 {
					f, err := os.OpenFile(path, os.O_RDWR, 0)
					if err != nil {
						t.Fatal(err)
					}
					defer f.Close()
					mapping, err := unix.Mmap(int(f.Fd()), 0, len(payload), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
					if err != nil {
						t.Fatal(err)
					}
					copy(mapping, "modified")
					if tc.scenario == "mapped_during_readback" {
						t.Cleanup(func() { _ = unix.Munmap(mapping) })
					} else if err := unix.Munmap(mapping); err != nil {
						t.Fatal(err)
					}
				}
				if tc.scenario == "replaced_during_readback" && checks == 2 {
					if err := os.Rename(path, path+".verified"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("unrelated"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				return nil
			}, func(publish func() error) error {
				if tc.scenario == "changed_during_commit" {
					if err := os.WriteFile(stagePath, []byte("modified"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := publish(); err != nil {
					return err
				}
				switch tc.scenario {
				case "changed_after_publish":
					if err := os.WriteFile(path, []byte("modified"), 0600); err != nil {
						t.Fatal(err)
					}
				case "cancelled_after_publish":
					cancel()
				case "destination_moved":
					if err := os.Rename(parent, parent+"-original"); err != nil {
						t.Fatal(err)
					}
					return errUpdateDestinationMovedAfterPublish
				}
				return nil
			})

			if saved != tc.saved || checks != tc.checks {
				t.Errorf("save = %v, %v with %d readbacks", saved, err, checks)
			}
			if tc.scenario == "changed_before_publish" {
				if err == nil || err.Error() != "更新文件 SHA-256 校验失败" {
					t.Errorf("want staged SHA-256 failure: %v", err)
				}
			} else if !errors.Is(err, tc.err) {
				t.Errorf("save error = %v, want %v", err, tc.err)
			}
			if tc.scenario == "destination_moved" {
				path = filepath.Join(parent+"-original", "update.zip")
			}
			data, readErr := os.ReadFile(path)
			if tc.output == "" {
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Errorf("unsaved update at destination: %q, %v", data, readErr)
				}
			} else if readErr != nil || string(data) != tc.output {
				t.Errorf("destination = %q, %v; want %q", data, readErr, tc.output)
			}
		})
	}
}

func TestDarwinUpdateReadbackDetectsConcurrentWrite(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "stage")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := io.WriteString(f, "verified"); err != nil {
		t.Fatal(err)
	}
	err = verifyDarwinUpdateReadback(int(f.Fd()), func() error {
		_, err := f.WriteAt([]byte("modified"), 0)
		return err
	})
	if !errors.Is(err, errUpdateStageIdentity) {
		t.Fatalf("expected a write notification during readback, got %v", err)
	}
}
