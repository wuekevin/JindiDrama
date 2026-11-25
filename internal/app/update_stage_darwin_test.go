//go:build cgo

package app

import (
	"context"
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

func TestUpdateReportsDestinationChangedDuringPublication(t *testing.T) {
	matches := true
	err := publishInUpdateDestination(func() bool { return matches }, func() error {
		matches = false
		return nil
	})
	if !errors.Is(err, errUpdateDestinationMovedAfterPublish) {
		t.Fatalf("published to original directory after selected path changed: %v", err)
	}
}
