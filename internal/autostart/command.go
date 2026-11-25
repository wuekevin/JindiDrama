package autostart

import (
	"errors"
	"strings"
	"unicode/utf16"
)

const maxWindowsRunCommand = 260

func windowsRunCommand(exe string) (string, error) {
	command := quoteWindowsExecutable(exe)
	if !validWindowsRunLength(command) {
		return "", errors.New("应用路径过长，无法设置 Windows 开机启动")
	}
	return command, nil
}

func quoteWindowsExecutable(exe string) string {
	return `"` + exe + `"`
}

func validWindowsRunLength(command string) bool {
	return len(utf16.Encode([]rune(command))) <= maxWindowsRunCommand
}

func windowsRunExecutable(command string) string {
	if !validWindowsRunLength(command) {
		return ""
	}
	command = strings.TrimSpace(command)
	if len(command) >= 2 && command[0] == '"' && command[len(command)-1] == '"' {
		command = command[1 : len(command)-1]
		if !strings.ContainsRune(command, '"') {
			return command
		}
		return ""
	}
	if !strings.ContainsAny(command, " \t\"") {
		return command
	}
	return ""
}
