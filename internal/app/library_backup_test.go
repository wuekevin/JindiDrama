package app

import (
	"errors"
	"testing"
)

func TestRestoreBackupChooserCancelOrFailure(t *testing.T) {
	app := New()
	result, err := app.restoreSelectedLibraryBackup(func() (string, error) { return "", nil })
	if result != nil || err != nil {
		t.Fatalf("cancel: %v, %v", result, err)
	}
	result, err = app.restoreSelectedLibraryBackup(func() (string, error) { return "", errors.New("private fixture") })
	if result != nil || err == nil || err.Error() != "无法打开系统文件选择器" {
		t.Fatalf("chooser error: %v, %v", result, err)
	}
}
