//go:build cgo

package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
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
			saved, err := saveUpdate(context.Background(), filepath.Join(parent, "update.zip"), func(_ context.Context, w io.Writer) error {
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
	saved, err := saveUpdate(context.Background(), path, func(_ context.Context, out io.Writer) error {
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
			saved, err := saveUpdate(ctx, path, func(_ context.Context, out io.Writer) error {
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
	saved, err := saveUpdate(context.Background(), path, func(_ context.Context, out io.Writer) error {
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
