package autostart

import (
	"errors"
	"testing"
)

func TestUnsupportedOS(t *testing.T) {
	m := &Manager{GOOS: "linux"}
	if err := m.Set(true); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}
