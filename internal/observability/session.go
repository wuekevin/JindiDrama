package observability

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	sessionStarting = 'S'
	sessionReady    = 'R'
	sessionClosed   = 'C'
	maxSessionBytes = 512
	maxVersionBytes = 128
	staleEmptyAge   = 24 * time.Hour
)

var sessionField = regexp.MustCompile(`^[A-Za-z0-9.+_-]+$`)

type sessionInfo struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Started int64  `json:"started"`
}

type Session struct {
	mu    sync.Mutex
	file  *os.File
	stage byte
}

// StartSession records a session before the desktop runtime starts. A process
// holds a kernel lock on its marker while it is alive.
func StartSession(dir, version string, log *Logger) (*Session, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if !sessionField.MatchString(version) {
		version = "unknown"
	} else if len(version) > maxVersionBytes {
		version = version[:maxVersionBytes-3] + "..."
	}
	info := sessionInfo{Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH, Started: time.Now().Unix()}
	metadata, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(dir, "run-*.session")
	if err != nil {
		return nil, err
	}
	locked, err := tryFileLock(f)
	if err == nil && !locked {
		err = errors.New("无法锁定桌面运行标记")
	}
	if err == nil {
		_, err = f.Write(append([]byte{sessionStarting}, metadata...))
	}
	if err == nil {
		err = f.Sync()
	}
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}
	// Previous-session recovery is itself covered by this process's marker.
	if log != nil {
		entries, _ := os.ReadDir(dir)
		current := filepath.Base(f.Name())
		for _, entry := range entries {
			name := entry.Name()
			if name != current && entry.Type().IsRegular() && strings.HasPrefix(name, "run-") && strings.HasSuffix(name, ".session") {
				reportSession(filepath.Join(dir, name), log)
			}
		}
	}
	return &Session{file: f, stage: sessionStarting}, nil
}

// Ready marks that the bundled application mounted its initial route.
func (s *Session) Ready() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil && s.stage == sessionStarting && writeSessionStage(s.file, sessionReady) == nil {
		s.stage = sessionReady
	}
}

// End releases the marker. An unexpected end is available to the next launch.
func (s *Session) End(clean bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return
	}
	f := s.file
	s.file = nil
	if clean {
		_ = writeSessionStage(f, sessionClosed)
	}
	_ = f.Close()
	if clean {
		_ = os.Remove(f.Name())
	}
}

func writeSessionStage(f *os.File, stage byte) error {
	if _, err := f.WriteAt([]byte{stage}, 0); err != nil {
		return err
	}
	return f.Sync()
}

func reportSession(path string, log *Logger) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return
	}
	remove := false
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	stat, err := file.Stat()
	if err != nil || (stat.Size() == 0 && time.Since(stat.ModTime()) < staleEmptyAge) {
		// A newly created marker stays empty until its owner holds the lock.
		return
	}
	if locked, err := tryFileLock(file); err != nil || !locked {
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxSessionBytes+1))
	if err != nil {
		return
	}
	if len(data) == 0 || data[0] == sessionClosed {
		remove = true
		return
	}
	var info sessionInfo
	if len(data) > maxSessionBytes || json.Unmarshal(data[1:], &info) != nil ||
		len(info.Version) > maxVersionBytes || !sessionField.MatchString(info.Version) || !sessionField.MatchString(info.OS) ||
		!sessionField.MatchString(info.Arch) || info.Started <= 0 {
		remove = true
		return
	}
	var stage string
	switch data[0] {
	case sessionStarting:
		stage = "窗口就绪前"
	case sessionReady:
		stage = "窗口就绪后"
	default:
		remove = true
		return
	}
	message := fmt.Sprintf("检测到上次运行未正常结束（%s），版本 %s，平台 %s/%s，启动时间 %s",
		stage, info.Version, info.OS, info.Arch, time.Unix(info.Started, 0).UTC().Format(time.RFC3339))
	if log.Log("warn", "session", message) == nil {
		_ = writeSessionStage(file, sessionClosed)
		remove = true
	}
}
