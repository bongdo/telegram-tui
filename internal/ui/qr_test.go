package ui

import (
	"testing"

	"rsc.io/qr"
)

func qrModule(t *testing.T, r rune, bottom bool) bool {
	t.Helper()
	switch r {
	case ' ':
		return false
	case '█':
		return true
	case '▀':
		return !bottom
	case '▄':
		return bottom
	default:
		t.Fatalf("unexpected QR rune %q", r)
		return false
	}
}

func TestQRModulesMatchEncoding(t *testing.T) {
	const url = "tg://login?token=AQIDBAUGBwgJCgsMDQ4PEA"
	lines, ok := qrLines(url)
	if !ok {
		t.Fatal("qrLines failed")
	}
	code, err := qr.Encode(url, qr.L)
	if err != nil {
		t.Fatal(err)
	}
	size := code.Size + qrQuiet*2
	if len(lines) != (size+1)/2 {
		t.Fatalf("lines = %d, want %d for size %d", len(lines), (size+1)/2, size)
	}
	for i, line := range lines {
		if got := len([]rune(line)); got != size {
			t.Fatalf("line %d width = %d, want %d", i, got, size)
		}
		for x, r := range []rune(line) {
			for _, bottom := range []bool{false, true} {
				y := i * 2
				if bottom {
					y++
				}
				mx, my := x-qrQuiet, y-qrQuiet
				want := mx >= 0 && my >= 0 && mx < code.Size && my < code.Size && code.Black(mx, my)
				if got := qrModule(t, r, bottom); got != want {
					t.Fatalf("module %d,%d = %v, want %v", x, y, got, want)
				}
			}
		}
	}
}
