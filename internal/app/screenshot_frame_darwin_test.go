//go:build darwin && cgo && !bindings

package app

import (
	"image"
	"os"
	"testing"
)

func TestNativeDemoScreenshotUsesPlaybackPosition(t *testing.T) {
	asset, err := New().demoExportAsset("d1", 7, "hd")
	if err != nil || asset != "demo/demo-2.mp4" {
		t.Fatalf("unexpected asset: %q, %v", asset, err)
	}
	assets := os.DirFS("../../frontend/public")
	first, err := nativeDemoScreenshot(assets, asset, 0)
	if err != nil {
		t.Fatal(err)
	}
	later, err := nativeDemoScreenshot(assets, asset, 5)
	if err != nil {
		t.Fatal(err)
	}
	want := image.Rect(0, 0, 360, 640)
	if first.Bounds() != want || later.Bounds() != want {
		t.Fatalf("capture sizes: %v, %v", first.Bounds(), later.Bounds())
	}
	for y := 0; y < want.Dy(); y += 8 {
		for x := 0; x < want.Dx(); x += 8 {
			if first.At(x, y) != later.At(x, y) {
				return
			}
		}
	}
	t.Fatal("distinct playback positions produced identical samples")
}
