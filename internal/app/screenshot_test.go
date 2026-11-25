package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestDemoScreenshotValidationAndSave(t *testing.T) {
	a := New()
	frame := image.NewRGBA(image.Rect(0, 0, 3, 4))
	pixel := color.RGBA{R: 21, G: 86, B: 149, A: 255}
	frame.SetRGBA(1, 2, pixel)
	encode := func(frame image.Image) string {
		var data bytes.Buffer
		if err := png.Encode(&data, frame); err != nil {
			t.Fatal(err)
		}
		return base64.StdEncoding.EncodeToString(data.Bytes())
	}
	valid := encode(frame)
	pickerMustNotOpen := func(runtime.SaveDialogOptions) (string, error) {
		t.Fatal("invalid screenshot opened the save dialog")
		return "", nil
	}
	for _, payload := range []string{"", "broken base64", encode(image.NewRGBA(image.Rect(0, 0, 1921, 1)))} {
		if saved, err := a.exportDemoScreenshot("d1", 1, "hd", 0, payload, pickerMustNotOpen); saved || err == nil {
			t.Fatalf("invalid capture: %v, %v", saved, err)
		}
	}
	if saved, err := a.exportDemoScreenshot("d1", 33, "hd", 0, valid, pickerMustNotOpen); saved || err == nil {
		t.Fatalf("unavailable episode: %v, %v", saved, err)
	}
	if saved, err := a.exportDemoScreenshot("d1", 1, "hd", 0, valid, func(runtime.SaveDialogOptions) (string, error) {
		return "", nil
	}); saved || err != nil {
		t.Fatalf("cancel: %v, %v", saved, err)
	}
	target := filepath.Join(t.TempDir(), "clip.png")
	pick := func(options runtime.SaveDialogOptions) (string, error) {
		if options.DefaultFilename != "JindiDrama-d1-1.png" {
			t.Fatalf("default filename: %q", options.DefaultFilename)
		}
		return target, nil
	}
	if saved, err := a.exportDemoScreenshot("d1", 1, "hd", 0, valid, pick); !saved || err != nil {
		t.Fatalf("saving screenshot: %v, %v", saved, err)
	}
	if saved, err := a.exportDemoScreenshot("d1", 1, "hd", 0, valid, pick); saved || err == nil {
		t.Fatalf("overwriting screenshot: %v, %v", saved, err)
	}
	output, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := png.Decode(bytes.NewReader(output))
	if err != nil || actual.Bounds() != frame.Bounds() || color.RGBAModel.Convert(actual.At(1, 2)) != pixel {
		t.Fatalf("decoded output did not preserve frame dimensions and pixels: %v", err)
	}
}
