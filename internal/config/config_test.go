package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	content := `
server:
  host: "127.0.0.1"
  port: 9000

upstreams:
  - name: "test-model"
    url: "http://localhost:8000"
    api_key: "sk-test"

default_upstream: "test-model"
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.Port != 9000 {
		t.Errorf("Expected port 9000, got %d", cfg.Server.Port)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Expected host 127.0.0.1, got %s", cfg.Server.Host)
	}

	if len(cfg.Upstreams) != 1 {
		t.Errorf("Expected 1 upstream, got %d", len(cfg.Upstreams))
	}

	if cfg.Default != "test-model" {
		t.Errorf("Expected default test-model, got %s", cfg.Default)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	content := `
upstreams:
  - name: "test"
    url: "http://localhost:8080"
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
}
