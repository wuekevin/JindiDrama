package autostart

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestWindowsRegistryCommands(t *testing.T) {
	var calls [][]string
	state := false
	m := &Manager{GOOS: "windows", Exe: `C:\Program Files\JindiDrama\JindiDrama.exe`, Run: func(n string, a ...string) ([]byte, error) {
		calls = append(calls, append([]string{n}, a...))
		switch a[0] {
		case "add":
			state = true
		case "delete":
			state = false
		case "query":
			if !state {
				return nil, errors.New("not found")
			}
			return []byte("    JindiDrama    REG_SZ    x"), nil
		}
		return nil, nil
	}}
	if err := m.Set(true); err != nil {
		t.Fatal(err)
	}
	add := strings.Join(calls[0], " ")
	if !strings.Contains(add, `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`) || !strings.Contains(add, `"C:\Program Files\JindiDrama\JindiDrama.exe"`) {
		t.Fatalf("reg add 参数不对: %s", add)
	}
	if on, _ := m.Enabled(); !on {
		t.Fatal("应为开启")
	}
	_ = m.Set(false)
	if on, _ := m.Enabled(); on {
		t.Fatal("应为关闭")
	}
}

func TestUnsupportedOS(t *testing.T) {
	m := &Manager{GOOS: "linux"}
	if err := m.Set(true); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}
