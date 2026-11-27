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

	"jindi-drama/internal/fileprivacy"
)

func TestUpdatePublicationPreservesWindowsDownloadZone(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "update.exe")
	saved, err := saveTestUpdate(context.Background(), final, func(_ context.Context, file io.Writer) error {
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
	saved, err := saveTestUpdate(t.Context(), filepath.Join(selected, "update.exe"), func(_ context.Context, file io.Writer) error {
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
	err := writePrivateStagedExport(filepath.Join(selected, "capture.png"), func(file *os.File) error {
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
	saved, err := saveTestUpdate(t.Context(), final, func(_ context.Context, file io.Writer) error {
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

func TestBackupPublicationKeepsProtectedWindowsACL(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "backups")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;OICI;GR;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := fixture.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	readACL := func(path string) *windows.SECURITY_DESCRIPTOR {
		t.Helper()
		info, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	const everyone = ";;;WD)"
	control := filepath.Join(dir, "ordinary-file")
	if err := os.WriteFile(control, []byte("control"), 0o600); err != nil {
		t.Fatal(err)
	}
	if acl := readACL(control).String(); !strings.Contains(acl, everyone) {
		t.Fatalf("control file did not inherit Everyone access: %s", acl)
	}

	expected, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;SY)(A;;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	controlPath, err := fileprivacy.WindowsPath(control)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(controlPath, windows.READ_CONTROL, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	checkErr := verifyPrivateBackupACL(handle, expected)
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(checkErr, errPrivateBackupACL) {
		t.Fatalf("verification accepted nonprivate file: %v", checkErr)
	}

	app := New()
	if err := app.openStore(filepath.Join(root, "jindi.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.close)
	backup, err := app.backupTo(root)
	if err != nil {
		t.Fatal(err)
	}
	info := readACL(backup)
	flags, _, err := info.Control()
	_, actualACEs, actualFound := strings.Cut(info.String(), "(")
	_, expectedACEs, expectedFound := strings.Cut(expected.String(), "(")
	if err != nil || flags&windows.SE_DACL_PROTECTED == 0 || !actualFound || !expectedFound || actualACEs != expectedACEs {
		t.Fatalf("backup ACL is not private: %s, %v", info.String(), err)
	}
	if files, err := os.ReadDir(dir); err != nil || len(files) != 2 {
		t.Fatalf("expected only the control and final backup: %v, %v", files, err)
	}
}
