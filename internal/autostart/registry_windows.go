package autostart

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	appName = "JindiDrama"
	runKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
)

func windowsAutostartEnabled(exe string) (bool, error) {
	return readWindowsRunValue(registry.CURRENT_USER, runKey, exe)
}

func setWindowsAutostart(exe string, on bool) error {
	return writeWindowsRunValue(registry.CURRENT_USER, runKey, exe, on)
}

func readWindowsRunValue(root registry.Key, path, exe string) (bool, error) {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	readError := errors.New("无法读取 Windows 开机启动设置")
	if err != nil {
		return false, readError
	}
	defer key.Close()
	command, kind, err := key.GetStringValue(appName)
	if errors.Is(err, registry.ErrNotExist) || errors.Is(err, registry.ErrUnexpectedType) {
		return false, nil
	}
	if err != nil {
		return false, readError
	}
	if !validWindowsRunLength(command) {
		return false, nil
	}
	if kind == registry.EXPAND_SZ {
		command, err = registry.ExpandString(command)
		if err != nil {
			return false, readError
		}
	}
	return matchesWindowsRunExecutable(windowsRunExecutable(command), exe), nil
}

func matchesWindowsRunExecutable(registered, exe string) bool {
	if registered == "" {
		return false
	}
	if strings.EqualFold(registered, exe) {
		return true
	}
	if !filepath.IsAbs(registered) || !strings.EqualFold(filepath.VolumeName(registered), filepath.VolumeName(exe)) {
		return false
	}
	// Resolve aliases only on the volume where the current executable is running.
	resolved, err := filepath.EvalSymlinks(registered)
	return err == nil && strings.EqualFold(resolved, exe)
}

func writeWindowsRunValue(root registry.Key, path, exe string, on bool) error {
	if on {
		enabled, err := readWindowsRunValue(root, path, exe)
		if err != nil || enabled {
			return err
		}
		command, err := windowsRunCommand(exe)
		if err != nil {
			return err
		}
		key, _, err := registry.CreateKey(root, path, registry.SET_VALUE)
		if err != nil {
			return errors.New("无法写入 Windows 开机启动设置")
		}
		defer key.Close()
		if err := key.SetStringValue(appName, command); err != nil {
			return errors.New("无法写入 Windows 开机启动设置")
		}
		return nil
	}

	key, err := registry.OpenKey(root, path, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	removeError := errors.New("无法关闭 Windows 开机启动")
	if err != nil {
		return removeError
	}
	defer key.Close()
	if err := key.DeleteValue(appName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return removeError
	}
	return nil
}
