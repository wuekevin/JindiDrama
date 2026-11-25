// Package autostart 管理“开机启动”：Windows 写 HKCU\...\Run，macOS 写 LaunchAgent。
// 只写当前用户范围，不需要管理员权限；关闭时删除对应项。
package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const plistName = "com.jindidrama.app.plist"

type Manager struct {
	GOOS    string
	Exe     string // 可执行文件绝对路径
	HomeDir string
}

// New 使用当前进程的可执行文件与用户目录。
func New() (*Manager, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Manager{GOOS: runtime.GOOS, Exe: exe, HomeDir: home}, nil
}

var ErrUnsupported = errors.New("当前系统暂不支持开机启动设置")

func (m *Manager) plistPath() string {
	return filepath.Join(m.HomeDir, "Library", "LaunchAgents", plistName)
}

func (m *Manager) plist() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.jindidrama.app</string>
<key>ProgramArguments</key><array><string>%s</string></array>
<key>RunAtLoad</key><true/>
</dict></plist>
`, xmlEscape(m.Exe))
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// Enabled 返回当前是否已启用。
func (m *Manager) Enabled() (bool, error) {
	switch m.GOOS {
	case "windows":
		return windowsAutostartEnabled(m.Exe)
	case "darwin":
		_, err := os.Stat(m.plistPath())
		return err == nil, nil
	}
	return false, ErrUnsupported
}

// Set 启用或关闭开机启动。
func (m *Manager) Set(on bool) error {
	switch m.GOOS {
	case "windows":
		return setWindowsAutostart(m.Exe, on)
	case "darwin":
		p := m.plistPath()
		if on {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			return os.WriteFile(p, []byte(m.plist()), 0o644)
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return ErrUnsupported
}
