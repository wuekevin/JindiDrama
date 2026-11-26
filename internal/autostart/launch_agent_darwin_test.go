package autostart

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func testLaunchAgentManager(home, exe string) *Manager {
	return &Manager{
		GOOS: "darwin", Exe: exe, HomeDir: home,
		launchAgentDisabled: func() (bool, error) { return false, nil },
	}
}

func TestParseLaunchAgentDisabled(t *testing.T) {
	for _, tc := range []struct {
		name, body        string
		disabled, invalid bool
	}{
		{name: "not listed", body: `"com.example.other" => disabled`},
		{name: "similar label only", body: `"com.jindidrama.app.beta" => disabled`},
		{name: "system disabled", body: `"com.jindidrama.app" => disabled`, disabled: true},
		{name: "system enabled", body: `"com.jindidrama.app" => enabled`},
		{name: "legacy disabled", body: `"com.jindidrama.app" => true`, disabled: true},
		{name: "legacy enabled", body: `"com.jindidrama.app" => false`},
		{name: "unknown state", body: `"com.jindidrama.app" => unspecified`, invalid: true},
		{name: "unknown format", body: `"com.jindidrama.app" = disabled`, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			disabled, err := parseLaunchAgentDisabled("\n\tdisabled services = {\n\t\t" + tc.body + "\n\t}\n")
			if disabled != tc.disabled || (err != nil) != tc.invalid {
				t.Fatalf("disabled=%v err=%v", disabled, err)
			}
		})
	}
	if _, err := parseLaunchAgentDisabled("unexpected launchctl response"); !errors.Is(err, errReadLaunchAgent) {
		t.Fatalf("unknown launchctl result: %v", err)
	}
}

func TestDarwinLaunchAgentHonorsSystemDisable(t *testing.T) {
	m := testLaunchAgentManager(t.TempDir(), "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama")
	dir := launchAgentsDirectory(m.HomeDir)
	path := filepath.Join(dir, plistName)
	checks := 0
	disabled := true
	var checkErr error
	m.launchAgentDisabled = func() (bool, error) {
		checks++
		return disabled, checkErr
	}
	if on, err := m.Enabled(); on || err != nil || checks != 0 {
		t.Fatalf("registration absent: enabled=%v checks=%d err=%v", on, checks, err)
	}
	if err := m.Set(true); !errors.Is(err, errLaunchAgentBlocked) {
		t.Fatalf("disabled registration accepted: %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("directory created when disabled: %v", err)
	}
	disabled = false
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	disabled = true
	if on, err := m.Enabled(); on || err != nil {
		t.Fatalf("system disabled registration: enabled=%v err=%v", on, err)
	}
	if err := m.Set(true); !errors.Is(err, errLaunchAgentBlocked) {
		t.Fatalf("disabled registration overridden: %v", err)
	}
	checkErr = errReadLaunchAgent
	if on, err := m.Enabled(); on || !errors.Is(err, errReadLaunchAgent) {
		t.Fatalf("unreadable system policy: enabled=%v err=%v", on, err)
	}
	if err := m.Set(true); !errors.Is(err, errReadLaunchAgent) {
		t.Fatalf("registration changed without system confirmation: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(original, got) {
		t.Fatalf("existing registry file changed: %v", err)
	}
	if err := m.Set(false); err != nil {
		t.Fatalf("cannot remove registration during a system query failure: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("registration was not removed: %v", err)
	}
}

func TestDarwinLaunchAgent(t *testing.T) {
	home := t.TempDir()
	m := testLaunchAgentManager(home, "/Applications/Jindi & Drama.app/Contents/MacOS/JindiDrama")
	if on, _ := m.Enabled(); on {
		t.Fatal("初始应为关闭")
	}
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(home, "Library", "LaunchAgents", plistName))
	if err != nil || !strings.Contains(string(b), "Jindi &amp; Drama") || !strings.Contains(string(b), "RunAtLoad") {
		t.Fatalf("plist 内容不对（需转义）: %v %s", err, b)
	}
	if on, _ := m.Enabled(); !on {
		t.Fatal("应为开启")
	}
	if err := m.Set(false); err != nil {
		t.Fatal(err)
	}
	if on, _ := m.Enabled(); on {
		t.Fatal("应为关闭")
	}
	if err := m.Set(false); err != nil {
		t.Fatalf("重复关闭不应报错: %v", err)
	}
}

func TestDarwinLaunchAgentRepairsMovedOrInvalidRegistration(t *testing.T) {
	const exe = "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama"
	m := testLaunchAgentManager(t.TempDir(), exe)
	dir := launchAgentsDirectory(m.HomeDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, plistName)
	for _, tc := range []struct {
		name  string
		plist string
	}{
		{"moved", launchAgentPlist("/Applications/Old/JindiDrama")},
		{"malformed", "not a property list"},
		{"disabled", strings.Replace(launchAgentPlist(exe), "</dict>", "<key>Disabled</key><true/></dict>", 1)},
		{"wrong key case", strings.Replace(launchAgentPlist(exe), "<key>Label", "<key>label", 1)},
		{"wrong program", strings.Replace(launchAgentPlist(exe), "</dict>", "<key>Program</key><string>/wrong/file</string></dict>", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.plist), 0o600); err != nil {
				t.Fatal(err)
			}
			if on, err := m.Enabled(); on || err != nil {
				t.Fatalf("invalid registration: enabled=%v, err=%v", on, err)
			}
			if err := m.Set(true); err != nil {
				t.Fatal(err)
			}
			if on, err := m.Enabled(); !on || err != nil {
				t.Fatalf("repaired registration: enabled=%v, err=%v", on, err)
			}
		})
	}
}

func TestDarwinLaunchAgentPreservesValidRegistration(t *testing.T) {
	const exe = "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama"
	for _, tc := range []struct {
		name    string
		content string
	}{
		{"binary with arguments and date", strings.Replace(launchAgentPlist(exe), "</array>",
			"<string>--example</string></array><key>Created</key><date>2026-11-26T00:00:00Z</date>", 1)},
		{"implicit launch", strings.Replace(launchAgentPlist(exe), "<key>RunAtLoad</key><true/>", "<key>KeepAlive</key><true/>", 1)},
		{"launch with exit rule", strings.Replace(launchAgentPlist(exe), "<key>RunAtLoad</key><true/>",
			"<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testLaunchAgentManager(t.TempDir(), exe)
			dir := launchAgentsDirectory(m.HomeDir)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, plistName)
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command("/usr/bin/plutil", "-lint", path).CombinedOutput(); err != nil {
				t.Fatalf("system rejected registration: %v, %s", err, output)
			}
			if output, err := exec.Command("/usr/bin/plutil", "-convert", "binary1", path).CombinedOutput(); err != nil {
				t.Fatalf("make binary plist fixture: %v, %s", err, output)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if on, err := m.Enabled(); !on || err != nil {
				t.Fatalf("valid registration: %v, %v", on, err)
			}
			if err := m.Set(true); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("valid binary registration rewritten: %v", err)
			}
		})
	}
}

func TestDarwinLaunchAgentRepairsUnsafePermissions(t *testing.T) {
	m := testLaunchAgentManager(t.TempDir(), "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama")
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(launchAgentsDirectory(m.HomeDir), plistName)
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	if on, err := m.Enabled(); on || err != nil {
		t.Fatalf("unsafe registration: %v, %v", on, err)
	}
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions were not secured: %v, %v", info, err)
	}
	if on, err := m.Enabled(); !on || err != nil {
		t.Fatalf("repaired registration: %v, %v", on, err)
	}
}

func TestDarwinLaunchAgentDoesNotFollowFinalLink(t *testing.T) {
	m := testLaunchAgentManager(t.TempDir(), "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama")
	dir := launchAgentsDirectory(m.HomeDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(t.TempDir(), "leave-alone")
	data := []byte(launchAgentPlist(m.Exe))
	if err := os.WriteFile(sentinel, data, 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, plistName)
	if err := os.Symlink(sentinel, path); err != nil {
		t.Fatal(err)
	}
	if on, err := m.Enabled(); on || err != nil {
		t.Fatalf("symbolic link was treated as active: %v, %v", on, err)
	}
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("registration did not replace the link: %v, %v", info, err)
	}
	if after, err := os.ReadFile(sentinel); err != nil || !bytes.Equal(data, after) {
		t.Fatalf("link target changed: %v", err)
	}
}

func TestDarwinLaunchAgentRejectsNonregularEntries(t *testing.T) {
	m := testLaunchAgentManager(t.TempDir(), "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama")
	dir := launchAgentsDirectory(m.HomeDir)
	path := filepath.Join(dir, plistName)
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := m.Set(true); err == nil {
		t.Fatal("enabled by replacing an existing directory")
	}
	if err := m.Set(false); err == nil {
		t.Fatal("disabled by removing an existing directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != plistName || !entries[0].IsDir() {
		t.Fatalf("directory or staged cleanup changed: %v, %v", entries, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		on, err := m.Enabled()
		if on {
			err = errors.New("named pipe was treated as enabled")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("launch registration read blocked on a named pipe")
	}
}

func TestDarwinLaunchAgentReportsUnreadableRegistration(t *testing.T) {
	m := testLaunchAgentManager(t.TempDir(), "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama")
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(launchAgentsDirectory(m.HomeDir), plistName)
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	if file, err := os.Open(path); err == nil {
		_ = file.Close()
		t.Skip("current account can read a mode 000 file")
	}
	if on, err := m.Enabled(); on || !errors.Is(err, errReadLaunchAgent) {
		t.Fatalf("unreadable registration: %v, %v", on, err)
	}
	if err := m.Set(true); !errors.Is(err, errReadLaunchAgent) {
		t.Fatalf("unreadable existing registration should be preserved: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0 {
		t.Fatalf("unreadable registration changed: %v, %v", info, err)
	}
}

func TestDarwinLaunchAgentDoesNotOverwriteOversizedRegistration(t *testing.T) {
	m := testLaunchAgentManager(t.TempDir(), "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama")
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(launchAgentsDirectory(m.HomeDir), plistName)
	data := []byte(strings.Replace(launchAgentPlist(m.Exe), "<dict>", "<dict><!--"+
		strings.Repeat("a", maxLaunchAgentBytes)+"-->", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := m.Set(true); !errors.Is(err, errReadLaunchAgent) {
		t.Fatalf("oversized registration should be preserved: %v", err)
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(data, after) {
		t.Fatalf("oversized registration changed: %v", err)
	}
}

func TestDarwinLaunchAgentPreservesAlternatePathToSameExecutable(t *testing.T) {
	root := t.TempDir()
	exe, alias := filepath.Join(root, "JindiDrama"), filepath.Join(root, "JindiAlias")
	if err := os.WriteFile(exe, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(exe, alias); err != nil {
		t.Fatal(err)
	}
	m := testLaunchAgentManager(root, exe)
	dir := launchAgentsDirectory(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, plistName)
	data := []byte(launchAgentPlist(alias))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if on, err := m.Enabled(); !on || err != nil {
		t.Fatalf("equivalent target: %v, %v", on, err)
	}
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(data, after) {
		t.Fatalf("valid registration was rewritten: %v", err)
	}
}
