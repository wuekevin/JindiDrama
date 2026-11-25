//go:build unix

package licensed

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestOpenRegularFileValidatesOpenedType(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "regular"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("regular", filepath.Join(dir, "relative-link")); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(dir, "pipe")
	if err := unix.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	p := &DemoProvider{root: root}

	for _, name := range []string{"regular", "relative-link"} {
		file, info, err := p.openRegularFile(name)
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil || !info.Mode().IsRegular() || string(content) != "fixture" {
			t.Fatalf("%s: got %q, %v, %v", name, content, info.Mode(), err)
		}
	}
	for _, name := range []string{".", "pipe"} {
		result := make(chan error, 1)
		go func() {
			file, _, err := p.openRegularFile(name)
			if file != nil {
				_ = file.Close()
			}
			result <- err
		}()
		select {
		case err := <-result:
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("%s: expected file type rejection, got %v", name, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: opening a non-regular file blocked", name)
		}
	}
}
