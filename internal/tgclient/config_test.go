package tgclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigFromEnvTestDC(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("TG_API_ID", "123456")
	t.Setenv("TG_API_HASH", "0123456789abcdef0123456789abcdef")
	t.Setenv("TG_TEST_DC", "1")
	t.Setenv("TG_DC", "")
	t.Setenv("TG_SESSION", "")
	t.Setenv("TG_DB", "")
	t.Setenv("TG_CACHE", "")

	cfg, enabled, err := ConfigFromEnv()
	if err != nil || !enabled {
		t.Fatalf("enabled=%v err=%v", enabled, err)
	}
	if !cfg.TestDC || cfg.DC != 2 {
		t.Fatalf("got test=%v dc=%d", cfg.TestDC, cfg.DC)
	}
	if !strings.HasSuffix(cfg.SessionPath, "session-test.json") {
		t.Fatalf("session path %q", cfg.SessionPath)
	}
	if !strings.HasSuffix(cfg.DBPath, "cache-test.db") {
		t.Fatalf("db path %q", cfg.DBPath)
	}
	if !strings.HasSuffix(cfg.CacheDir, "media-test") {
		t.Fatalf("cache dir %q", cfg.CacheDir)
	}
}

func TestSavedCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("TG_TUI_DEMO", "")
	t.Setenv("TG_API_ID", "")
	t.Setenv("TG_API_HASH", "")
	t.Setenv("APP_ID", "")
	t.Setenv("APP_HASH", "")
	t.Setenv("TG_SESSION", "")
	t.Setenv("TG_DB", "")
	t.Setenv("TG_CACHE", "")

	if _, enabled, err := ConfigFromEnv(); enabled || err != nil {
		t.Fatalf("expected unconfigured start, enabled=%v err=%v", enabled, err)
	}

	id, hash, err := ParseCredentials(" 123456 ", "0123456789ABCDEF0123456789ABCDEF")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := SaveCredentials(id, hash); err != nil {
		t.Fatalf("save: %v", err)
	}
	path, err := ConfigFilePath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("saved file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config permissions = %o, want 600", perm)
	}

	cfg, enabled, err := ConfigFromEnv()
	if err != nil || !enabled {
		t.Fatalf("saved config not used: enabled=%v err=%v", enabled, err)
	}
	if cfg.AppID != 123456 || cfg.AppHash != hash {
		t.Fatalf("saved config = %d/%s", cfg.AppID, cfg.AppHash)
	}

	// Environment still wins over the file.
	t.Setenv("TG_API_ID", "777")
	t.Setenv("TG_API_HASH", "abcdefabcdefabcdefabcdefabcdefab")
	cfg, enabled, err = ConfigFromEnv()
	if err != nil || !enabled || cfg.AppID != 777 {
		t.Fatalf("env override: enabled=%v id=%d err=%v", enabled, cfg.AppID, err)
	}
}

func TestParseCredentialsRejectsBadInput(t *testing.T) {
	cases := [][2]string{
		{"", "0123456789abcdef0123456789abcdef"},
		{"abc", "0123456789abcdef0123456789abcdef"},
		{"0", "0123456789abcdef0123456789abcdef"},
		{"123456", ""},
		{"123456", "0123"},
		{"123456", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
	}
	for _, c := range cases {
		if _, _, err := ParseCredentials(c[0], c[1]); err == nil {
			t.Fatalf("ParseCredentials(%q, %q) accepted", c[0], c[1])
		}
	}
}

func TestConfigFromEnvDC(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("TG_API_ID", "1")
	t.Setenv("TG_API_HASH", "hash")
	t.Setenv("TG_TEST_DC", "")
	t.Setenv("TG_DC", "4")
	t.Setenv("TG_SESSION", "")
	t.Setenv("TG_DB", "")
	t.Setenv("TG_CACHE", "")

	cfg, enabled, err := ConfigFromEnv()
	if err != nil || !enabled {
		t.Fatalf("enabled=%v err=%v", enabled, err)
	}
	if cfg.TestDC || cfg.DC != 4 {
		t.Fatalf("got test=%v dc=%d", cfg.TestDC, cfg.DC)
	}
	if !strings.HasSuffix(cfg.SessionPath, "session.json") {
		t.Fatalf("session path %q", cfg.SessionPath)
	}

	t.Setenv("TG_DC", "9")
	if _, _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected invalid TG_DC error")
	}
}
