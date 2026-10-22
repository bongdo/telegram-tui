package tgclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	AppID       int
	AppHash     string
	SessionPath string
	CacheDir    string
	DBPath      string
	DC          int
	TestDC      bool
}

// ConfigFromEnv returns enabled=false when Telegram credentials are absent, so
// the TUI can keep running in demo mode.
func ConfigFromEnv() (Config, bool, error) {
	if os.Getenv("TG_TUI_DEMO") == "1" {
		return Config{}, false, nil
	}
	idRaw := firstEnv("TG_API_ID", "APP_ID")
	hash := firstEnv("TG_API_HASH", "APP_HASH")
	if idRaw == "" && hash == "" {
		return Config{}, false, nil
	}
	if idRaw == "" || hash == "" {
		return Config{}, false, fmt.Errorf("set both TG_API_ID and TG_API_HASH")
	}
	id, err := strconv.Atoi(idRaw)
	if err != nil || id <= 0 {
		return Config{}, false, fmt.Errorf("invalid TG_API_ID %q", idRaw)
	}

	testDC := os.Getenv("TG_TEST_DC") == "1"
	dc := 2
	if raw := os.Getenv("TG_DC"); raw != "" {
		dc, err = strconv.Atoi(raw)
		if err != nil || dc < 1 || dc > 5 {
			return Config{}, false, fmt.Errorf("invalid TG_DC %q", raw)
		}
	}
	suffix := ""
	if testDC {
		suffix = "-test"
	}

	sessionPath := os.Getenv("TG_SESSION")
	if sessionPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return Config{}, false, err
		}
		dir = filepath.Join(dir, "tg-matrix-tui")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Config{}, false, err
		}
		sessionPath = filepath.Join(dir, "session"+suffix+".json")
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		return Config{}, false, err
	}
	cacheDir := os.Getenv("TG_CACHE")
	if cacheDir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			base = os.TempDir()
		}
		cacheDir = filepath.Join(base, "tg-matrix-tui", "media"+suffix)
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return Config{}, false, err
	}
	dbPath := os.Getenv("TG_DB")
	if dbPath == "" {
		dbPath = filepath.Join(filepath.Dir(sessionPath), "cache"+suffix+".db")
	}
	return Config{
		AppID:       id,
		AppHash:     hash,
		SessionPath: sessionPath,
		CacheDir:    cacheDir,
		DBPath:      dbPath,
		DC:          dc,
		TestDC:      testDC,
	}, true, nil
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}
