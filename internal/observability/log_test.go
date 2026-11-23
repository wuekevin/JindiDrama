package observability

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	in := "login failed phone=13800138000 Cookie: sessionid=abc123; token=xyz Authorization: Bearer abc.def https://a.example.com/play/v1?sign=SECRET&vid=1#frag"
	out := Redact(in)
	for _, leak := range []string{"13800138000", "abc123", "xyz", "abc.def", "SECRET", "vid=1", "frag"} {
		if strings.Contains(out, leak) {
			t.Fatalf("泄露 %q: %s", leak, out)
		}
	}
	if !strings.Contains(out, "https://a.example.com/play/v1?<redacted>") {
		t.Fatalf("应保留域名和路径: %s", out)
	}
}

func TestLogRotateAndTail(t *testing.T) {
	l, _ := New(t.TempDir(), 200)
	for i := 0; i < 20; i++ {
		_ = l.Log("error", "player", "token=SECRET boom 13800138000")
	}
	lines, err := l.Tail(5)
	if err != nil || len(lines) == 0 || len(lines) > 5 {
		t.Fatalf("%v %v", lines, err)
	}
	for _, ln := range lines {
		if strings.Contains(ln, "SECRET") || strings.Contains(ln, "13800138000") {
			t.Fatalf("日志含敏感信息: %s", ln)
		}
	}
}
