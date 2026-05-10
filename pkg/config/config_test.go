package config

import (
	"os"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Server.Port != 8081 {
		t.Errorf("expected port 8081, got %d", cfg.Server.Port)
	}
	if cfg.Database.Path != "./data/aetherstream.db" {
		t.Errorf("expected db path, got %s", cfg.Database.Path)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if cfg.Server.Port != 8081 {
		t.Errorf("expected fallback port 8081, got %d", cfg.Server.Port)
	}
}

func TestJWTSecretFromEnv(t *testing.T) {
	os.Setenv("AETHERSTREAM_SECRET", "test-secret-32-chars-long!!")
	defer os.Unsetenv("AETHERSTREAM_SECRET")
	// TODO: implement env override in config.Load
}
