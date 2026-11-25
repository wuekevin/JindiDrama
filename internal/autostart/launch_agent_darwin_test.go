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

func TestDarwinLaunchAgent(t *testing.T) {
	home := t.TempDir()
	m := &Manager{GOOS: "darwin", Exe: "/Applications/Jindi & Drama.app/Contents/MacOS/JindiDrama", HomeDir: home}
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
	m := &Manager{GOOS: "darwin", Exe: exe, HomeDir: t.TempDir()}
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
			m := &Manager{GOOS: "darwin", Exe: exe, HomeDir: t.TempDir()}
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
	m := &Manager{GOOS: "darwin", Exe: "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama", HomeDir: t.TempDir()}
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
	m := &Manager{GOOS: "darwin", Exe: "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama", HomeDir: t.TempDir()}
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
	m := &Manager{GOOS: "darwin", Exe: "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama", HomeDir: t.TempDir()}
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
	m := &Manager{GOOS: "darwin", Exe: "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama", HomeDir: t.TempDir()}
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
	m := &Manager{GOOS: "darwin", Exe: "/Applications/JindiDrama.app/Contents/MacOS/JindiDrama", HomeDir: t.TempDir()}
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
	m := &Manager{GOOS: "darwin", Exe: exe, HomeDir: root}
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
