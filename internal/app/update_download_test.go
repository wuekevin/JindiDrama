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
		saved, err := saveTestUpdate(ctx, path, download, commit)
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
	if saved, err := saveTestUpdate(final, path, verified, func(publish func() error) error {
		cancelFinal()
		return publish()
	}); saved || err != nil {
		t.Fatalf("cancelled at publication: %v %v", saved, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled update was published: %v", err)
	}
	if saved, err := saveTestUpdate(context.Background(), path, verified, commit); !saved || err != nil {
		t.Fatalf("successful download: %v %v", saved, err)
	}
	if saved, err := saveTestUpdate(context.Background(), path, verified, commit); saved || err == nil {
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

func TestSaveUpdateReportsStageCleanupFailure(t *testing.T) {
	verificationFailure := errors.New("invalid SHA-256")
	cases := []struct {
		name           string
		cancelDownload bool
		downloadErr    error
		wantSaved      bool
		wantMessage    string
	}{
		{"cancelled", true, context.Canceled, false, "暂存文件清理失败，请检查所选目录"},
		{"unverified then cancelled", true, verificationFailure, false, "暂存文件清理失败，请检查所选目录"},
		{"unverified", false, verificationFailure, false, "暂存文件清理失败，请检查所选目录"},
		{"verified", false, nil, true, "安装包已校验并保存，但暂存目录清理失败，请检查所选目录"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			path := filepath.Join(parent, "update.zip")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			const content = "fixture package"
			var unrelated, stagedPackage string
			download := func(_ context.Context, out io.Writer) error {
				stagedPackage = out.(*os.File).Name()
				unrelated = filepath.Join(filepath.Dir(stagedPackage), "unrelated")
				if err := os.WriteFile(unrelated, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(out, content); err != nil {
					t.Fatal(err)
				}
				if tc.cancelDownload {
					cancel()
				}
				return tc.downloadErr
			}
			saved, err := saveTestUpdate(ctx, path, download, func(publish func() error) error { return publish() })
			wantMessage := tc.wantMessage
			if errors.Is(tc.downloadErr, verificationFailure) {
				wantMessage = tc.downloadErr.Error() + "\n" + wantMessage
			}
			if saved != tc.wantSaved || err == nil || err.Error() != wantMessage {
				t.Errorf("saveUpdate = %v, %v; want %v, %q", saved, err, tc.wantSaved, wantMessage)
			}
			if errors.Is(tc.downloadErr, verificationFailure) && !errors.Is(err, verificationFailure) {
				t.Errorf("download failure lost: %v", err)
			}
			if payload, err := os.ReadFile(unrelated); err != nil || string(payload) != "keep" {
				t.Errorf("unrelated file changed: %q, %v", payload, err)
			}
			if _, err := os.Lstat(stagedPackage); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("staged package remains: %v", err)
			}
			if tc.wantSaved {
				if payload, err := os.ReadFile(path); err != nil || string(payload) != content {
					t.Errorf("published package: %q, %v", payload, err)
				}
			} else if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("unverified or cancelled package was published: %v", err)
			}
		})
	}
}

func TestUpdateDownloadResultKeepsVerifiedPathWithWarning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "verified.zip")
	if result, err := updateDownloadResult(path, errUpdateSavedCleanup); err != nil ||
		result.Path != path || result.Warning != errUpdateSavedCleanup.Error() {
		t.Fatalf("verified download status: %+v, %v", result, err)
	}
	if result, err := updateDownloadResult("", errUpdatePublishedIdentity); err != nil ||
		result.Path != "" || result.Warning != "" || result.Unverified != errUpdatePublishedIdentity.Error() {
		t.Fatalf("unverified persisted download status: %+v, %v", result, err)
	}
	if result, err := updateDownloadResult("", errUpdateCleanup); !errors.Is(err, errUpdateCleanup) ||
		result.Path != "" || result.Warning != "" {
		t.Fatalf("failed download status: %+v, %v", result, err)
	}
}

// These fixtures exercise saving, cancellation and cleanup independently of release checks.
func saveTestUpdate(ctx context.Context, path string, download func(context.Context, io.Writer) error, commit func(func() error) error) (bool, error) {
	return saveUpdate(ctx, path, download, func(context.Context, io.Reader) error { return nil }, commit)
}
