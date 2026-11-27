// Package observability 提供故障诊断日志：自动脱敏（Cookie/Token/手机号/完整 URL 查询串），按大小滚动。
package observability

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

const credentialKeys = `(?:token|access_token|refresh_token|sessionid|sid|passwd|password|secret|sign|x-tt-token)`

var (
	reHeader     = regexp.MustCompile(`(?i)\b((?:cookie|set-cookie|authorization|proxy-authorization)\s*[=:][ \t]*)[^\r\n]*(?:\r?\n[ \t]+[^\r\n]*)*`)
	reJSONSecret = regexp.MustCompile(`(?i)("(?:cookie|set-cookie|authorization|proxy-authorization|` + credentialKeys + `)"[ \t]*:[ \t]*)"(?:\\.|[^"\\])*"`)
	// A quoted value may contain delimiters and whole URLs. Treat an open
	// quote on a truncated log line as sensitive through the end of that line.
	reQuotedKV = regexp.MustCompile(`(?im)\b(` + credentialKeys + `[ \t]*[=:][ \t]*)` +
		`(?:"(?:\\[^\r\n]|[^"\\\r\n])*(?:"|\\?\r?$)|'(?:\\[^\r\n]|[^'\\\r\n])*(?:'|\\?\r?$))`)
	reKV     = regexp.MustCompile(`(?i)\b(` + credentialKeys + `)\s*[=:]\s*[^\s;,&"']+`)
	rePhone  = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	reURL    = regexp.MustCompile(`(?i)https?://[^\s"]+`)
	reBearer = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/\-]+=*`)
)

// Redact 去除敏感信息；URL 仅保留 scheme+host+path，丢弃查询串与片段。
func Redact(s string) string {
	s = reJSONSecret.ReplaceAllString(s, `${1}"<redacted>"`)
	s = reQuotedKV.ReplaceAllString(s, "${1}<redacted>")
	s = reHeader.ReplaceAllString(s, "${1}<redacted>")
	s = reURL.ReplaceAllStringFunc(s, func(u string) string {
		parsed, err := url.Parse(u)
		if err != nil || parsed.Host == "" {
			return "<redacted-url>"
		}
		parsed.User = nil
		hasQueryOrFragment := parsed.ForceQuery || parsed.RawQuery != "" || parsed.Fragment != ""
		parsed.RawQuery, parsed.Fragment, parsed.RawFragment = "", "", ""
		parsed.ForceQuery = false
		if hasQueryOrFragment {
			return parsed.String() + "?<redacted>"
		}
		return parsed.String()
	})
	s = reBearer.ReplaceAllString(s, "Bearer <redacted>")
	s = reKV.ReplaceAllStringFunc(s, func(m string) string {
		i := strings.IndexAny(m, "=:")
		return m[:i+1] + "<redacted>"
	})
	return rePhone.ReplaceAllString(s, "<phone>")
}

func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, s)
}

const DefaultLogBytes = 1 << 20

type Logger struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
}

func New(dir string, maxBytes int64) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Logger{path: filepath.Join(dir, "jindi.log"), maxBytes: maxBytes}, nil
}

func (l *Logger) Path() string { return l.path }

// A separate lock file is never rotated, so separate processes agree on the
// same lock while reading or updating both log generations.
func (l *Logger) lock() (*os.File, error) {
	file, err := os.OpenFile(l.path+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	for range 20 {
		locked, err := tryFileLock(file)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		if locked {
			return file, nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	_ = file.Close()
	return nil, errors.New("诊断日志正在使用")
}

func (l *Logger) Log(level, component, msg string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	lock, err := l.lock()
	if err != nil {
		return err
	}
	defer lock.Close()
	if fi, err := os.Stat(l.path); err == nil && fi.Size() >= l.maxBytes {
		_ = os.Rename(l.path, l.path+".1") // 仅保留一份历史
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s [%s] %s: %s\n", time.Now().Format(time.RFC3339), oneLine(Redact(strings.ToUpper(level))), oneLine(Redact(component)), oneLine(Redact(msg)))
	return err
}

// Tail 返回最近 n 行（用于诊断导出）。
func (l *Logger) Tail(n int) ([]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	lock, err := l.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	current, currentErr := os.ReadFile(l.path)
	rotated, rotatedErr := os.ReadFile(l.path + ".1")
	for _, err := range []error{currentErr, rotatedErr} {
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if currentErr != nil && rotatedErr != nil {
		return nil, currentErr
	}
	// Older log files may contain raw HTTP fields written by previous versions.
	content := strings.TrimRight(string(rotated), "\n")
	if len(current) > 0 {
		if content != "" {
			content += "\n"
		}
		content += string(current)
	}
	lines := strings.Split(strings.TrimRight(Redact(content), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i := range lines {
		lines[i] = oneLine(lines[i])
	}
	return lines, nil
}
