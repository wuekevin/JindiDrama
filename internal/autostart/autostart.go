// Package autostart 管理“开机启动”：Windows 写 HKCU\...\Run，macOS 写 LaunchAgent。
// 只写当前用户范围，不需要管理员权限；关闭时删除对应项。
package autostart

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

type Manager struct {
	GOOS                string
	Exe                 string // 可执行文件绝对路径
	HomeDir             string
	launchAgentDisabled func() (bool, error)
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

// Enabled 返回当前是否已启用。
func (m *Manager) Enabled() (bool, error) {
	switch m.GOOS {
	case "windows":
		return windowsAutostartEnabled(m.Exe)
	case "darwin":
		return darwinAutostartEnabled(m)
	}
	return false, ErrUnsupported
}

// Set 启用或关闭开机启动。
func (m *Manager) Set(on bool) error {
	switch m.GOOS {
	case "windows":
		return setWindowsAutostart(m.Exe, on)
	case "darwin":
		return setDarwinAutostart(m, on)
	}
	return ErrUnsupported
}
