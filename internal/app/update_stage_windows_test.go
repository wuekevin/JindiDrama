package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
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

func TestSaveUpdateRejectsRetargetedWindowsDestination(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "original")
	replacement := filepath.Join(root, "replacement")
	selected := filepath.Join(root, "selected")
	for _, dir := range []string{original, replacement} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(original, selected); err != nil {
		if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
			t.Skip("creating a directory symlink requires Windows Developer Mode or elevation")
		}
		t.Fatal(err)
	}
	saved, err := saveUpdate(t.Context(), filepath.Join(selected, "update.exe"), func(_ context.Context, file io.Writer) error {
		if _, err := io.WriteString(file, "verified"); err != nil {
			return err
		}
		if err := os.Remove(selected); err != nil {
			return err
		}
		return os.Symlink(replacement, selected)
	}, func(publish func() error) error { return publish() })
	if saved || !errors.Is(err, errUpdateDestinationMoved) {
		t.Fatalf("retargeted destination: saved=%v, err=%v", saved, err)
	}
	for _, dir := range []string{original, replacement} {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("%s retained staged or published files: %v, %v", dir, entries, err)
		}
	}
}

func TestStagedExportRejectsRetargetedWindowsDestination(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "original")
	replacement := filepath.Join(root, "replacement")
	selected := filepath.Join(root, "selected")
	for _, dir := range []string{original, replacement} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(original, selected); err != nil {
		if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
			t.Skip("creating a directory symlink requires Windows Developer Mode or elevation")
		}
		t.Fatal(err)
	}
	err := writeStagedExport(filepath.Join(selected, "capture.png"), func(file io.Writer) error {
		if _, err := io.WriteString(file, "capture"); err != nil {
			return err
		}
		if err := os.Remove(selected); err != nil {
			return err
		}
		return os.Symlink(replacement, selected)
	})
	if !errors.Is(err, errUpdateDestinationMoved) {
		t.Fatalf("retargeted export: %v", err)
	}
	for _, dir := range []string{original, replacement} {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("%s retained staged or published files: %v, %v", dir, entries, err)
		}
	}
}

func TestSaveUpdateRejectsWindowsDestinationCreatedDuringDownload(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "update.exe")
	saved, err := saveUpdate(t.Context(), final, func(_ context.Context, file io.Writer) error {
		if _, err := io.WriteString(file, "verified"); err != nil {
			return err
		}
		return os.WriteFile(final, []byte("existing"), 0o600)
	}, func(publish func() error) error { return publish() })
	if saved || err == nil || err.Error() != "目标文件已存在，请使用其他文件名" {
		t.Fatalf("occupied destination: saved=%v, err=%v", saved, err)
	}
	if bytes, err := os.ReadFile(final); err != nil || string(bytes) != "existing" {
		t.Fatalf("existing file changed: %q, %v", bytes, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Fatalf("staged files remain: %v, %v", entries, err)
	}
}
