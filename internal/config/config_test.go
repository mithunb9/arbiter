package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	f.Close()
	return f.Name()
}

const validYAML = `
server:
  port: 9099
tiers:
  - name: fast
    adapters: [a1]
adapters:
  - name: a1
    type: ollama
    base_url: http://localhost:11434
    model: llama3
`

func TestLoad_Valid(t *testing.T) {
	cfg, err := Load(writeTemp(t, validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 9099 {
		t.Errorf("port: got %d, want 9099", cfg.Server.Port)
	}
	if len(cfg.Tiers) != 1 || cfg.Tiers[0].Name != "fast" {
		t.Errorf("tiers: got %+v", cfg.Tiers)
	}
	if len(cfg.Adapters) != 1 || cfg.Adapters[0].Type != "ollama" {
		t.Errorf("adapters: got %+v", cfg.Adapters)
	}
}

func TestLoad_EnvExpansion(t *testing.T) {
	t.Setenv("TEST_API_KEY", "secret-key")
	yaml := `
server:
  port: 9099
tiers:
  - name: fast
    adapters: [a1]
adapters:
  - name: a1
    type: anthropic
    api_key: $TEST_API_KEY
    model: claude-haiku-4-5
`
	cfg, err := Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Adapters[0].APIKey != "secret-key" {
		t.Errorf("api_key: got %q, want \"secret-key\"", cfg.Adapters[0].APIKey)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	_, err := Load(writeTemp(t, ":::invalid yaml:::"))
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoad_MissingPort(t *testing.T) {
	yaml := `
server: {}
tiers:
  - name: fast
    adapters: [a1]
adapters:
  - name: a1
    type: ollama
    model: llama3
`
	_, err := Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected validation error for missing port, got nil")
	}
}

func TestLoad_InvalidAdapterType(t *testing.T) {
	yaml := `
server:
  port: 9099
tiers:
  - name: fast
    adapters: [a1]
adapters:
  - name: a1
    type: unknown
    model: llama3
`
	_, err := Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected validation error for invalid adapter type, got nil")
	}
}

func TestLoad_EmptyTiers(t *testing.T) {
	yaml := `
server:
  port: 9099
tiers: []
adapters:
  - name: a1
    type: ollama
    model: llama3
`
	_, err := Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected validation error for empty tiers, got nil")
	}
}
