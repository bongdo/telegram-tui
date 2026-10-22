package chafa

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func testImage(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gradient.png")
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 180, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRender(t *testing.T) {
	if !Available() {
		t.Skip("chafa is not installed")
	}
	lines, err := Render(testImage(t), 20, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 || len(lines) > 8 {
		t.Fatalf("lines = %d, want 1..8", len(lines))
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got > 20 {
			t.Fatalf("line %d width = %d, want <= 20", i, got)
		}
	}
}
