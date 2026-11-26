package rollbackcli

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReportsUsageAndFailures(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "missing source.db")
	output := filepath.Join(directory, "older copy.db")
	validFlags := []string{"-source", source, "-output", output, "-target", "2"}
	for _, tc := range []struct {
		name     string
		args     []string
		exitCode int
		message  string
	}{
		{"help", []string{"-h"}, 0, "-target"},
		{"missing flags", []string{"-source", source}, 2, "-target"},
		{"extra argument", append(validFlags, "unexpected"), 2, "-target"},
		{"invalid target", []string{"-target", "v2"}, 2, "invalid value"},
		{"missing source", validFlags, 1, "missing source.db"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			got := Run(tc.args, io.Discard, &stderr)
			if got != tc.exitCode || !strings.Contains(stderr.String(), tc.message) {
				t.Fatalf("exit code %d, stderr %q", got, stderr.String())
			}
		})
	}
}
