package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdatePublicationPreservesWindowsDownloadZone(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "update.exe")
	saved, err := saveUpdate(context.Background(), final, func(_ context.Context, file io.Writer) error {
		_, err := io.WriteString(file, "verified")
		return err
	}, func(publish func() error) error { return publish() })
	if !saved || err != nil {
		t.Fatalf("publication: saved=%v, %v", saved, err)
	}
	mark, err := os.ReadFile(final + ":Zone.Identifier")
	if err != nil || !strings.Contains(string(mark), "ZoneId=3") {
		t.Fatalf("download origin was not preserved: %q, %v", mark, err)
	}
}
