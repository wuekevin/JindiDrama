package autostart

import (
	"strings"
	"testing"
)

func TestWindowsRunCommand(t *testing.T) {
	exe := `C:\Program Files\JindiDrama\JindiDrama.exe`
	command, err := windowsRunCommand(exe)
	if err != nil || command != `"C:\Program Files\JindiDrama\JindiDrama.exe"` {
		t.Fatalf("command = %q, error = %v", command, err)
	}
	for _, tc := range []struct {
		command string
		path    string
	}{
		{strings.ToLower(command), strings.ToLower(exe)},
		{`   "C:\Program Files\JindiDrama\JindiDrama.exe"  `, exe},
		{exe, ""},
		{`C:\JindiDrama.exe`, `C:\JindiDrama.exe`},
		{command + " unexpected", ""},
	} {
		if got := windowsRunExecutable(tc.command); got != tc.path {
			t.Errorf("parse %q: got %q, want %q", tc.command, got, tc.path)
		}
	}
	valid := strings.Repeat("客", 256) + "🎬"
	if _, err := windowsRunCommand(valid); err != nil {
		t.Fatalf("rejected a 260-unit command: %v", err)
	}
	if _, err := windowsRunCommand(valid + "字"); err == nil {
		t.Fatal("accepted a command longer than the Run limit")
	}
	if windowsRunExecutable(quoteWindowsExecutable(valid+"字")) != "" {
		t.Fatal("read accepted a command longer than the Run limit")
	}
}
