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

func TestBuiltInCredentials(t *testing.T) {
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

	cfg, enabled, err := ConfigFromEnv()
	if err != nil || !enabled {
		t.Fatalf("built-in pair not used: enabled=%v err=%v", enabled, err)
	}
	if cfg.AppID != defaultAppID || cfg.AppHash != defaultAppHash {
		t.Fatalf("built-in pair = %d/%s", cfg.AppID, cfg.AppHash)
	}

	t.Setenv("TG_API_ID", "777")
	t.Setenv("TG_API_HASH", "abcdefabcdefabcdefabcdefabcdefab")
	cfg, enabled, err = ConfigFromEnv()
	if err != nil || !enabled || cfg.AppID != 777 {
		t.Fatalf("env override: enabled=%v id=%d err=%v", enabled, cfg.AppID, err)
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
