// Package autostart 管理“开机启动”：Windows 写 HKCU\...\Run，macOS 写 LaunchAgent。
// 只写当前用户范围，不需要管理员权限；关闭时删除对应项。
package autostart

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	appName   = "JindiDrama"
	runKey    = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	plistName = "com.jindidrama.app.plist"
)

// Runner 抽象外部命令，便于测试。
type Runner func(name string, args ...string) ([]byte, error)

func execRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

type Manager struct {
	GOOS    string
	Exe     string // 可执行文件绝对路径
	HomeDir string
	Run     Runner
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
	return &Manager{GOOS: runtime.GOOS, Exe: exe, HomeDir: home, Run: execRunner}, nil
}

var ErrUnsupported = errors.New("当前系统暂不支持开机启动设置")

func (m *Manager) plistPath() string {
	return filepath.Join(m.HomeDir, "Library", "LaunchAgents", plistName)
}

// quoteWin 给含空格路径加引号（注册表 Run 项要求）。
func quoteWin(p string) string { return `"` + strings.ReplaceAll(p, `"`, "") + `"` }

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
		out, err := m.Run("reg", "query", runKey, "/v", appName)
		if err != nil {
			return false, nil // 未找到该值
		}
		return strings.Contains(string(out), appName), nil
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
		if on {
			if out, err := m.Run("reg", "add", runKey, "/v", appName, "/t", "REG_SZ", "/d", quoteWin(m.Exe), "/f"); err != nil {
				return fmt.Errorf("写入开机启动失败: %v %s", err, strings.TrimSpace(string(out)))
			}
			return nil
		}
		// 不存在时 reg delete 会报错，视为已关闭
		_, _ = m.Run("reg", "delete", runKey, "/v", appName, "/f")
		return nil
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
