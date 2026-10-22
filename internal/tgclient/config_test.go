package tgclient

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigFromEnvTestDC(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("TG_API_ID", "34914051")
	t.Setenv("TG_API_HASH", "8f0d5e02c829de90265925380f068cdf")
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
