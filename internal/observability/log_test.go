package observability

import (
	"os"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	in := "login failed phone=13800138000 Cookie: sessionid=abc123; token=xyz; other=SECONDCOOKIE\n  folded=THIRDCOOKIE\nAuthorization: Bearer abc.def\nHTTPS://urluser:urlpass'passwordtail@a.example.com/play/v1?sign=SECRET&vid=1#frag"
	out := Redact(in)
	for _, leak := range []string{"13800138000", "abc123", "xyz", "abc.def", "SECRET", "vid=1", "frag", "SECONDCOOKIE", "THIRDCOOKIE", "urluser", "urlpass", "passwordtail"} {
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

func TestLogSanitizesMetadataAndKeepsOneLine(t *testing.T) {
	l, err := New(t.TempDir(), 2000)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Log("warn", "player\nauthorization: Basic PRIVATE", "sensitive metadata"); err != nil {
		t.Fatal(err)
	}
	if err := l.Log("warn", "player", "line one\r\nline two\u2028line three"); err != nil {
		t.Fatal(err)
	}
	lines, err := l.Tail(5)
	if err != nil || len(lines) != 2 || strings.Contains(lines[0], "PRIVATE") || !strings.Contains(lines[1], "line one  line two line three") {
		t.Fatalf("diagnostic output: %v, %v", lines, err)
	}
}

func TestDiagnosticTailRedactsLegacyFiles(t *testing.T) {
	l, err := New(t.TempDir(), 2000)
	if err != nil {
		t.Fatal(err)
	}
	const old = "cache hit\nCookie: abc=PRIVATE; second=ALSO_PRIVATE\n  folded=FOLDED_PRIVATE\nplayer ready\n"
	if err := os.WriteFile(l.Path(), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := l.Tail(5)
	if err != nil || len(lines) != 3 || strings.Contains(strings.Join(lines, "\n"), "PRIVATE") {
		t.Fatalf("legacy diagnostics: %v, %v", lines, err)
	}
}

func TestRedactStructuredCredentialStrings(t *testing.T) {
	in := `{"Authorization":"Basic secret-base64","token":"secret-escaped\"value","ok":true} bearer secret+/suffix==`
	out := Redact(in)
	if strings.Contains(out, "secret") || strings.Contains(out, "suffix") || !strings.Contains(out, `"ok":true`) {
		t.Fatalf("structured diagnostics: %s", out)
	}
}
