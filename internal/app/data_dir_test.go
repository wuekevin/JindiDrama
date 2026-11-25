package app

import (
	"path/filepath"
	"testing"
)

func TestDataDirUsesExplicitAbsoluteDirectory(t *testing.T) {
	profile := t.TempDir()
	t.Setenv("JINDIDRAMA_DATA_DIR", filepath.Join(profile, "profile"))
	got, err := DataDir()
	if err != nil || got != filepath.Join(profile, "profile") {
		t.Fatalf("DataDir = %q, %v", got, err)
	}
	t.Setenv("JINDIDRAMA_DATA_DIR", "relative-profile")
	if _, err := DataDir(); err == nil {
		t.Fatal("an invalid explicit profile must not fall back to the installed profile")
	}
}
