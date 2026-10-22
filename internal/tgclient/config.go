package tgclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// defaultAppID/defaultAppHash are the my.telegram.org pair registered for this
// application, so a fresh install can log in without any setup.
const (
	defaultAppID   = 34914051
	defaultAppHash = "8f0d5e02c829de90265925380f068cdf"
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

// savedCredentials is the my.telegram.org application pair, kept next to the
// session so the app can start without exported environment variables. It is
// not a login credential by itself: the gotd session is.
type savedCredentials struct {
	AppID   int    `json:"app_id"`
	AppHash string `json:"app_hash"`
}

// ConfigFilePath returns the location of the saved app credentials.
func ConfigFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tg-matrix-tui", "config.json"), nil
}

// ParseCredentials validates what the login screen asks for.
func ParseCredentials(idRaw, hashRaw string) (int, string, error) {
	id, err := strconv.Atoi(strings.TrimSpace(idRaw))
	if err != nil || id <= 0 {
		return 0, "", errors.New("API ID must be a positive number")
	}
	hash := strings.ToLower(strings.TrimSpace(hashRaw))
	if len(hash) != 32 {
		return 0, "", errors.New("API hash must be 32 hex characters")
	}
	if strings.Trim(hash, "0123456789abcdef") != "" {
		return 0, "", errors.New("API hash must be 32 hex characters")
	}
	return id, hash, nil
}

// SaveCredentials writes the pair with 0600 and an atomic rename.
func SaveCredentials(id int, hash string) error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(savedCredentials{AppID: id, AppHash: hash}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadCredentials() (int, string, bool) {
	path, err := ConfigFilePath()
	if err != nil {
		return 0, "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", false
	}
	var saved savedCredentials
	if err := json.Unmarshal(data, &saved); err != nil || saved.AppID <= 0 || saved.AppHash == "" {
		return 0, "", false
	}
	return saved.AppID, saved.AppHash, true
}

// ConfigFromEnv prefers TG_API_ID/TG_API_HASH, then the saved config.json, then
// the app pair built into the binary. enabled=false only in demo mode.
func ConfigFromEnv() (Config, bool, error) {
	if os.Getenv("TG_TUI_DEMO") == "1" {
		return Config{}, false, nil
	}
	idRaw := firstEnv("TG_API_ID", "APP_ID")
	hash := firstEnv("TG_API_HASH", "APP_HASH")
	if idRaw == "" && hash == "" {
		id, savedHash, ok := loadCredentials()
		if !ok {
			id, savedHash = defaultAppID, defaultAppHash
		}
		idRaw, hash = strconv.Itoa(id), savedHash
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
