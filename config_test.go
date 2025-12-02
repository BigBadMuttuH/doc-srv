package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_NoFileReturnsDefaults(t *testing.T) {
	// Use a path that definitely does not exist inside a temp dir.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "no-such-config.yaml")

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error for missing file: %v", err)
	}

	def := DefaultConfig()
	if cfg.DocsDir != def.DocsDir || cfg.Port != def.Port || cfg.LogFile != def.LogFile {
		t.Fatalf("expected defaults, got cfg=%+v", cfg)
	}
}

func TestLoadConfig_ValidYamlOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`
 docs_dir: "./custom-docs"
 port: "9090"
 cache_ttl: "10s"
 read_timeout: "11s"
 write_timeout: "12s"
 idle_timeout: "13s"
 read_header_timeout: "14s"
 log_file: "./log/custom-access.log"
 `)

	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.DocsDir != "./custom-docs" {
		t.Errorf("DocsDir: expected ./custom-docs, got %q", cfg.DocsDir)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port: expected 9090, got %q", cfg.Port)
	}
	if cfg.LogFile != "./log/custom-access.log" {
		t.Errorf("LogFile: expected ./log/custom-access.log, got %q", cfg.LogFile)
	}

	if cfg.CacheTTL != 10*time.Second {
		t.Errorf("CacheTTL: expected 10s, got %s", cfg.CacheTTL)
	}
	if cfg.ReadTimeout != 11*time.Second {
		t.Errorf("ReadTimeout: expected 11s, got %s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 12*time.Second {
		t.Errorf("WriteTimeout: expected 12s, got %s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 13*time.Second {
		t.Errorf("IdleTimeout: expected 13s, got %s", cfg.IdleTimeout)
	}
	if cfg.ReadHeaderTimeout != 14*time.Second {
		t.Errorf("ReadHeaderTimeout: expected 14s, got %s", cfg.ReadHeaderTimeout)
	}
}

func TestLoadConfig_InvalidDurationReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// invalid duration for cache_ttl
	content := []byte(`
 cache_ttl: "not-a-duration"
 `)

	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Fatalf("expected error for invalid duration, got nil")
	}
}
