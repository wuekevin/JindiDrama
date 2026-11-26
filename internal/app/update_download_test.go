package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveUpdateOnlyPublishesVerifiedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update.zip")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := errors.New("invalid SHA-256")
	commit := func(publish func() error) error { return publish() }
	for _, download := range []func(context.Context, io.Writer) error{
		func(_ context.Context, w io.Writer) error { _, _ = io.WriteString(w, "invalid"); return failure },
		func(_ context.Context, w io.Writer) error {
			_, _ = io.WriteString(w, "cancelled")
			cancel()
			return nil
		},
	} {
		saved, err := saveUpdate(ctx, path, download, commit)
		if saved || (ctx.Err() == nil && !errors.Is(err, failure)) || (ctx.Err() != nil && err != nil) {
			t.Fatalf("failed or cancelled download: saved=%v err=%v", saved, err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("staging or output survived: %v %v", entries, err)
		}
	}
	verified := func(_ context.Context, w io.Writer) error { _, err := io.WriteString(w, "verified"); return err }
	final, cancelFinal := context.WithCancel(context.Background())
	defer cancelFinal()
	if saved, err := saveUpdate(final, path, verified, func(publish func() error) error {
		cancelFinal()
		return publish()
	}); saved || err != nil {
		t.Fatalf("cancelled at publication: %v %v", saved, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled update was published: %v", err)
	}
	if saved, err := saveUpdate(context.Background(), path, verified, commit); !saved || err != nil {
		t.Fatalf("successful download: %v %v", saved, err)
	}
	if saved, err := saveUpdate(context.Background(), path, verified, commit); saved || err == nil {
		t.Fatalf("overwriting existing update was allowed: %v %v", saved, err)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "verified" {
		t.Fatalf("existing file was modified: %q %v", b, err)
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
