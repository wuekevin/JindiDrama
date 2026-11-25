package autostart

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestWindowsRunRegistryFixture(t *testing.T) {
	root := registry.CURRENT_USER
	path := `Software\JindiDramaAutostartTest-` + rand.Text()
	t.Cleanup(func() { _ = registry.DeleteKey(root, path) })
	const exe = `C:\Program Files\JindiDrama\JindiDrama.exe`
	assertEnabled := func(executable string, expected bool) {
		t.Helper()
		got, err := readWindowsRunValue(root, path, executable)
		if err != nil || got != expected {
			t.Fatalf("enabled = %v, error = %v, want %v", got, err, expected)
		}
	}
	assertEnabled(exe, false)
	if err := writeWindowsRunValue(root, path, exe, false); err != nil {
		t.Fatal(err)
	}
	if err := writeWindowsRunValue(root, path, exe, true); err != nil {
		t.Fatal(err)
	}
	assertEnabled(exe, true)
	assertEnabled(`D:\Moved\JindiDrama.exe`, false)

	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	t.Setenv("JINDI_AUTOSTART_TEST_DIR", `C:\Program Files\JindiDrama`)
	if err := key.SetExpandStringValue(appName, `"%JINDI_AUTOSTART_TEST_DIR%\JindiDrama.exe"`); err != nil {
		t.Fatal(err)
	}
	assertEnabled(exe, true)
	if err := writeWindowsRunValue(root, path, exe, true); err != nil {
		t.Fatal(err)
	}
	const expanded = `"%JINDI_AUTOSTART_TEST_DIR%\JindiDrama.exe"`
	if command, kind, err := key.GetStringValue(appName); err != nil || kind != registry.EXPAND_SZ || command != expanded {
		t.Fatalf("valid registration changed: %q, %v, %v", command, kind, err)
	}
	if err := key.SetDWordValue(appName, 1); err != nil {
		t.Fatal(err)
	}
	assertEnabled(exe, false)
	for range 2 {
		if err := writeWindowsRunValue(root, path, exe, false); err != nil {
			t.Fatal(err)
		}
		assertEnabled(exe, false)
	}
}

func TestWindowsRunRegistryAccessDeniedIsReported(t *testing.T) {
	root := registry.CURRENT_USER
	path := `Software\JindiDramaAutostartTest-` + rand.Text()
	const old = `"C:\Old\JindiDrama.exe"`
	const exe = `C:\JindiDrama.exe`
	key, _, err := registry.CreateKey(root, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = key.Close()
		if err := registry.DeleteKey(root, path); err != nil {
			t.Errorf("remove isolated key: %v", err)
		}
	})
	if err := key.SetStringValue(appName, old); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	deny := func(mask string, access uint32) {
		t.Helper()
		descriptor, err := windows.SecurityDescriptorFromString("D:P(D;;" + mask + ";;;" + sid + ")(A;;KA;;;" + sid + ")")
		if err != nil {
			t.Fatal(err)
		}
		acl, _, err := descriptor.DACL()
		if err != nil {
			t.Fatal(err)
		}
		if err := windows.SetSecurityInfo(windows.Handle(key), windows.SE_REGISTRY_KEY,
			windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
			t.Fatal(err)
		}
		probe, err := registry.OpenKey(root, path, access)
		if err == nil {
			_ = probe.Close()
		}
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Fatalf("expected access denied for isolated key, got %v", err)
		}
	}
	deny("0x2", registry.SET_VALUE)
	for _, on := range []bool{true, false} {
		if err := writeWindowsRunValue(root, path, exe, on); err == nil {
			t.Fatalf("registry write failure was hidden for enabled=%v", on)
		}
	}
	if command, _, err := key.GetStringValue(appName); err != nil || command != old {
		t.Fatalf("denied operation changed the fixture: %q, %v", command, err)
	}
	deny("0x3", registry.QUERY_VALUE)
	if on, err := readWindowsRunValue(root, path, exe); on || err == nil {
		t.Fatalf("registry read failure: %v, %v", on, err)
	}
}

func TestWindowsRunMatchesResolvedExecutablePath(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "JindiDrama.exe")
	if err := os.WriteFile(exe, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		t.Fatal(err)
	}
	aliased := filepath.Dir(exe) + `\.\` + filepath.Base(exe)
	if !matchesWindowsRunExecutable(aliased, resolved) {
		t.Fatalf("equivalent registered path did not match: %q, %q", aliased, resolved)
	}
}
