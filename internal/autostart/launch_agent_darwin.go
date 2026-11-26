package autostart

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const launchAgentLabel = "com.jindidrama.app"
const plistName = launchAgentLabel + ".plist"
const maxLaunchAgentBytes = 64 << 10
const maxLaunchAgentXMLBytes = 1 << 20

var (
	errReadLaunchAgent    = errors.New("无法读取 macOS 开机启动设置")
	errLaunchAgentBlocked = errors.New("macOS 已禁用此启动项，请在系统设置的登录项中检查后台运行权限")
)

func launchAgentsDirectory(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents")
}

func launchAgentPlist(exe string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string></array>
<key>RunAtLoad</key><true/>
</dict></plist>
`, launchAgentLabel, xmlEscape(exe))
}

func xmlEscape(s string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(s))
	return escaped.String()
}

func darwinAutostartEnabled(m *Manager) (bool, error) {
	dir, err := unix.Open(launchAgentsDirectory(m.HomeDir), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if err != nil {
		return false, errReadLaunchAgent
	}
	defer unix.Close(dir)
	registered, err := readLaunchAgent(dir, m.Exe)
	if err != nil || !registered {
		return false, err
	}
	disabled, err := m.macOSLaunchAgentDisabled()
	return err == nil && !disabled, err
}

func (m *Manager) macOSLaunchAgentDisabled() (bool, error) {
	if m.launchAgentDisabled != nil {
		return m.launchAgentDisabled()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/launchctl", "print-disabled", "gui/"+strconv.Itoa(os.Geteuid()))
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.Output()
	if err != nil {
		return false, errReadLaunchAgent
	}
	return parseLaunchAgentDisabled(string(output))
}

func parseLaunchAgentDisabled(output string) (bool, error) {
	const header = "disabled services = {"
	output = strings.TrimSpace(output)
	if !strings.HasPrefix(output, header) || !strings.HasSuffix(output, "}") {
		return false, errReadLaunchAgent
	}
	for line := range strings.Lines(output[len(header) : len(output)-1]) {
		label, status, ok := strings.Cut(strings.TrimSpace(line), "=>")
		if !ok {
			if label == "" {
				continue
			}
			return false, errReadLaunchAgent
		}
		if strings.TrimSpace(label) != strconv.Quote(launchAgentLabel) {
			continue
		}
		switch strings.TrimSpace(status) {
		case "disabled", "true":
			return true, nil
		case "enabled", "false":
			return false, nil
		default:
			return false, errReadLaunchAgent
		}
	}
	return false, nil
}

func readLaunchAgent(dir int, exe string) (bool, error) {
	fd, err := unix.Openat(dir, plistName, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ELOOP) {
		return false, nil
	}
	if err != nil {
		return false, errReadLaunchAgent
	}
	file := os.NewFile(uintptr(fd), plistName)
	defer file.Close()
	var info unix.Stat_t
	if err := unix.Fstat(fd, &info); err != nil {
		return false, errReadLaunchAgent
	}
	if info.Mode&unix.S_IFMT != unix.S_IFREG ||
		info.Uid != uint32(os.Geteuid()) || info.Mode&(unix.S_IWGRP|unix.S_IWOTH) != 0 {
		return false, nil
	}
	if info.Size > maxLaunchAgentBytes {
		return false, errReadLaunchAgent
	}
	data, err := io.ReadAll(io.LimitReader(file, maxLaunchAgentBytes+1))
	if err != nil || len(data) > maxLaunchAgentBytes {
		return false, errReadLaunchAgent
	}

	normalized, err := normalizeLaunchAgent(data)
	if err != nil {
		return false, err
	}
	target := launchAgentExecutable(normalized)
	if target == exe && target != "" {
		return true, nil
	}
	if !filepath.IsAbs(target) {
		return false, nil
	}
	registered, err := os.Stat(target)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errReadLaunchAgent
	}
	if !registered.Mode().IsRegular() {
		return false, nil
	}
	current, err := os.Stat(exe)
	if err != nil {
		return false, errReadLaunchAgent
	}
	return os.SameFile(registered, current), nil
}

func normalizeLaunchAgent(data []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/plutil", "-convert", "xml1", "-o", "-", "-")
	cmd.Stdin = bytes.NewReader(data)
	stdout, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return nil, errReadLaunchAgent
	}
	output, readErr := io.ReadAll(io.LimitReader(stdout, maxLaunchAgentXMLBytes+1))
	if len(output) > maxLaunchAgentXMLBytes {
		cancel()
		_ = cmd.Wait()
		return nil, errReadLaunchAgent
	}
	waitErr := cmd.Wait()
	if readErr != nil || ctx.Err() != nil {
		return nil, errReadLaunchAgent
	}
	if waitErr != nil {
		var invalidPlist *exec.ExitError
		if errors.As(waitErr, &invalidPlist) && invalidPlist.ExitCode() == 1 {
			return nil, nil
		}
		return nil, errReadLaunchAgent
	}
	return output, nil
}

type plistElement struct {
	XMLName xml.Name
	Text    string         `xml:",chardata"`
	Items   []plistElement `xml:",any"`
}

func launchAgentExecutable(data []byte) string {
	var plist plistElement
	if xml.Unmarshal(data, &plist) != nil || plist.XMLName.Local != "plist" ||
		len(plist.Items) != 1 || plist.Items[0].XMLName.Local != "dict" {
		return ""
	}
	items := plist.Items[0].Items
	if len(items)%2 != 0 {
		return ""
	}
	values := make(map[string]plistElement)
	for index := 0; index < len(items); index += 2 {
		if items[index].XMLName.Local != "key" {
			return ""
		}
		values[items[index].Text] = items[index+1]
	}
	label := values["Label"]
	disabled := values["Disabled"].XMLName.Local
	keepAlive := values["KeepAlive"]
	runAtLoad := values["RunAtLoad"].XMLName.Local == "true" ||
		keepAlive.XMLName.Local == "true" || (keepAlive.XMLName.Local == "dict" && len(keepAlive.Items) != 0)
	if label.XMLName.Local != "string" || label.Text != launchAgentLabel ||
		(disabled != "" && disabled != "false") || !runAtLoad {
		return ""
	}
	args, hasArgs := values["ProgramArguments"]
	if hasArgs {
		if args.XMLName.Local != "array" {
			return ""
		}
		for _, value := range args.Items {
			if value.XMLName.Local != "string" {
				return ""
			}
		}
	}
	if program, exists := values["Program"]; exists {
		if program.XMLName.Local == "string" {
			return program.Text
		}
		return ""
	}
	if len(args.Items) != 0 {
		return args.Items[0].Text
	}
	return ""
}

func setDarwinAutostart(m *Manager, on bool) error {
	directory := launchAgentsDirectory(m.HomeDir)
	if on {
		disabled, err := m.macOSLaunchAgentDisabled()
		if err != nil {
			return err
		}
		if disabled {
			return errLaunchAgentBlocked
		}
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return errors.New("无法创建 macOS 开机启动目录")
		}
	}
	dir, err := unix.Open(directory, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if !on && errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return errors.New("无法打开 macOS 开机启动目录")
	}
	defer unix.Close(dir)

	if !on {
		if err := unix.Unlinkat(dir, plistName, 0); err != nil && !errors.Is(err, unix.ENOENT) {
			return errors.New("无法关闭 macOS 开机启动")
		}
		return nil
	}
	enabled, err := readLaunchAgent(dir, m.Exe)
	if err != nil || enabled {
		return err
	}
	return writeLaunchAgent(dir, m.Exe)
}

func writeLaunchAgent(dir int, exe string) error {
	errWrite := errors.New("无法保存 macOS 开机启动设置")
	staged := ".jindidrama-launch-agent-" + rand.Text()
	fd, err := unix.Openat(dir, staged, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return errWrite
	}
	defer unix.Unlinkat(dir, staged, 0)
	file := os.NewFile(uintptr(fd), staged)
	_, writeErr := io.WriteString(file, launchAgentPlist(exe))
	if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
		return errWrite
	}
	if err := unix.Renameat(dir, staged, dir, plistName); err != nil {
		return errWrite
	}
	return nil
}
