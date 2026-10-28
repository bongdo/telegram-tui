package tgclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppDir(t *testing.T) {
	base := t.TempDir()
	want := filepath.Join(base, "telegram-tui")
	if got := appDir(base, "telegram-tui"); got != want {
		t.Fatalf("new dir = %q, want %q", got, want)
	}

	legacy := filepath.Join(base, "tg-matrix-tui")
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := appDir(base, "telegram-tui"); got != legacy {
		t.Fatalf("legacy dir = %q, want %q", got, legacy)
	}

	if err := os.Mkdir(want, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := appDir(base, "telegram-tui"); got != want {
		t.Fatalf("new dir wins = %q, want %q", got, want)
	}
}
