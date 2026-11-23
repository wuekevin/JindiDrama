// Package observability 提供故障诊断日志：自动脱敏（Cookie/Token/手机号/完整 URL 查询串），按大小滚动。
package observability

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	reKV     = regexp.MustCompile(`(?i)\b(cookie|set-cookie|authorization|token|access_token|refresh_token|sessionid|sid|passwd|password|secret|sign|x-tt-token)\s*[=:]\s*[^\s;,&"']+`)
	rePhone  = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	reURL    = regexp.MustCompile(`https?://[^\s"']+`)
	reBearer = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]+`)
)

// Redact 去除敏感信息；URL 仅保留 scheme+host+path，丢弃查询串与片段。
func Redact(s string) string {
	s = reURL.ReplaceAllStringFunc(s, func(u string) string {
		if i := strings.IndexAny(u, "?#"); i >= 0 {
			u = u[:i] + "?<redacted>"
		}
		return u
	})
	s = reBearer.ReplaceAllString(s, "Bearer <redacted>")
	s = reKV.ReplaceAllStringFunc(s, func(m string) string {
		i := strings.IndexAny(m, "=:")
		return m[:i+1] + "<redacted>"
	})
	return rePhone.ReplaceAllString(s, "<phone>")
}

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

func (l *Logger) Log(level, component, msg string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if fi, err := os.Stat(l.path); err == nil && fi.Size() >= l.maxBytes {
		_ = os.Rename(l.path, l.path+".1") // 仅保留一份历史
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s [%s] %s: %s\n", time.Now().Format(time.RFC3339), strings.ToUpper(level), component, Redact(msg))
	return err
}

// Tail 返回最近 n 行（用于诊断导出）。
func (l *Logger) Tail(n int) ([]string, error) {
	b, err := os.ReadFile(l.path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}
