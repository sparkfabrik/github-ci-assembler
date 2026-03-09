package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfiguration_AllowsRootEnvMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: [build]
env:
  GLOBAL_FLAG: "1"
  DEPLOY_ENV: staging
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := LoadConfiguration(path)
	if err != nil {
		t.Fatalf("LoadConfiguration() error: %v", err)
	}

	if cfg.Env == nil {
		t.Fatal("expected env to be loaded, got nil")
	}
	if cfg.Env["GLOBAL_FLAG"] != "1" {
		t.Fatalf("unexpected GLOBAL_FLAG value: %v", cfg.Env["GLOBAL_FLAG"])
	}
	if cfg.Env["DEPLOY_ENV"] != "staging" {
		t.Fatalf("unexpected DEPLOY_ENV value: %v", cfg.Env["DEPLOY_ENV"])
	}
}

func TestLoadConfiguration_RejectsInvalidRootEnvFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.yml")
	content := `version: "1"
stages: [build]
env: not-a-map
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := LoadConfiguration(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid \"env\" format") {
		t.Fatalf("unexpected error: %v", err)
	}
}
