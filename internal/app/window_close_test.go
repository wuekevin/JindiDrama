package app

import (
	"path/filepath"
	"testing"
)

func TestWindowClosePreferenceRestoresAndTracksSavedSettings(t *testing.T) {
	database := filepath.Join(t.TempDir(), "jindi.db")
	first := New()
	if err := first.openStore(database); err != nil {
		t.Fatal(err)
	}
	if err := first.SaveSetting("app", `{"closeAction":"隐藏到菜单栏"}`); err != nil {
		t.Fatal(err)
	}
	first.restoreWindowCloseAction()
	if got := first.WindowCloseAction(); got != "隐藏到菜单栏" {
		t.Fatalf("legacy tray preference: %q", got)
	}
	if err := first.SetWindowCloseAction("最小化到任务栏"); err != nil {
		t.Fatal(err)
	}
	if err := first.SaveSetting("app", `{"closeAction":"退出程序","theme":"light"}`); err != nil {
		t.Fatal(err)
	}
	first.close()

	reopened := New()
	if err := reopened.openStore(database); err != nil {
		t.Fatal(err)
	}
	reopened.restoreWindowCloseAction()
	if got := reopened.WindowCloseAction(); got != "最小化到任务栏" {
		t.Fatalf("stale settings snapshot replaced a committed preference: %q", got)
	}
	if err := reopened.SetWindowCloseAction("invalid"); err == nil {
		t.Fatal("invalid close preference was accepted")
	}
	reopened.close()
	if err := reopened.SetWindowCloseAction("退出程序"); err == nil {
		t.Fatal("closed store accepted a new preference")
	}
	if got := reopened.WindowCloseAction(); got != "最小化到任务栏" {
		t.Fatalf("failed save changed system close behavior: %q", got)
	}
}

func TestWindowCloseDefaultsToQuittingForMissingOrInvalidPreference(t *testing.T) {
	for _, raw := range []string{"", "{}", "{", `{"closeAction":1}`, `{"closeAction":"最小化到托盘"}`} {
		if got := settingsWindowCloseAction(raw); got != closeQuit {
			t.Errorf("settings %q gave action %v", raw, got)
		}
	}
}

func TestWindowClosePreferenceSupportsSessionWithoutSQLite(t *testing.T) {
	a := New()
	if value := a.WindowCloseAction(); value != "" {
		t.Fatalf("expected opportunity to restore browser preference, got %q", value)
	}
	if err := a.SetWindowCloseAction("最小化到任务栏"); err != nil {
		t.Fatal(err)
	}
	if value := a.WindowCloseAction(); value != "最小化到任务栏" {
		t.Fatalf("session preference: %q", value)
	}
}
