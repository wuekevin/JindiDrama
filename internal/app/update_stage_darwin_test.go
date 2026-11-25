//go:build cgo

package app

import (
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
