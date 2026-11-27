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

func TestRedactQuotedCredentialAssignments(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`password="private alpha; beta" status=failed`, `password=<redacted> status=failed`},
		{`password="private Cookie: alpha" status=failed`, `password=<redacted> status=failed`},
		{`token='private, https://credentials.example.test/secret?key=private' status=failed`, `token=<redacted> status=failed`},
		{`X-TT-TOKEN : "private\"second" status=failed`, `X-TT-TOKEN :<redacted> status=failed`},
		{`sid='private\'second' refresh_token="private\\suffix" status=failed`, `sid=<redacted> refresh_token=<redacted> status=failed`},
		{"secret=\"private truncated\r\nplayer ready", "secret=<redacted>\nplayer ready"},
		{"secret='private truncated\\\nplayer ready", "secret=<redacted>\nplayer ready"},
	} {
		if got := Redact(tc.input); got != tc.want {
			t.Errorf("Redact(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

func TestDiagnosticsNeverExportsQuotedCredentials(t *testing.T) {
	log, err := New(t.TempDir(), 2000)
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Log("warn", "player", `password="synthetic-new-private log" status=failed`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log.Path()+".1", []byte("token='synthetic-old-private token' stage=loading\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(log.Path())
	if err != nil || strings.Contains(string(saved), "synthetic-new-private") {
		t.Fatalf("raw log file should omit the synthetic credential: %q %v", saved, err)
	}
	lines, err := log.Tail(5)
	if err != nil || len(lines) != 2 || lines[0] != "token=<redacted> stage=loading" ||
		!strings.HasSuffix(lines[1], "password=<redacted> status=failed") {
		t.Fatalf("diagnostic output: %v %v", lines, err)
	}
}
