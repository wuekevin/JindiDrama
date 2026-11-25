package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlatformVersionUsesTheNumericCore(t *testing.T) {
	for input, expected := range map[string]string{
		"v0.1.0": "0.1.0", "v2.3.4-rc.10+qa.1": "2.3.4", "v65535.0.65535": "65535.0.65535",
	} {
		if got, err := platformVersion(input); err != nil || got != expected {
			t.Errorf("platformVersion(%q) = %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"1.2.3", "v01.2.3", "v2.3.4-rc.01", "v1.65536.3"} {
		if _, err := platformVersion(input); err == nil {
			t.Errorf("must refuse %q", input)
		}
	}
}

func TestPlatformVersionFitsTheDesktopMarker(t *testing.T) {
	const prefix = "v1.2.3+"
	version := prefix + strings.Repeat("x", 128-len(prefix))
	if _, err := platformVersion(version); err != nil {
		t.Fatalf("maximum complete version rejected: %v", err)
	}
	if _, err := platformVersion(version + "x"); err == nil {
		t.Fatal("accepted version that cannot be verified from the startup marker")
	}
}

func TestStampWailsVersionPreservesOtherSettings(t *testing.T) {
	config := filepath.Join(t.TempDir(), "wails.json")
	if err := os.WriteFile(config, []byte(`{"outputfilename":"JindiDrama","info":{"productName":"金帝短剧","productVersion":"1.0.0"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := stampWailsVersion(config, "0.2.7"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	var options struct {
		Output string `json:"outputfilename"`
		Info   struct {
			Name    string `json:"productName"`
			Version string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(got, &options); err != nil || options.Output != "JindiDrama" ||
		options.Info.Name != "金帝短剧" || options.Info.Version != "0.2.7" {
		t.Fatalf("stamped config = %+v, %v", options, err)
	}
}
