package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds final, already-parsed configuration values used by the program.
type Config struct {
	DocsDir           string
	Port              string
	CacheTTL          time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	LogFile           string
}

// yamlConfig mirrors the YAML structure with string durations.
type yamlConfig struct {
	DocsDir           string `yaml:"docs_dir"`
	Port              string `yaml:"port"`
	CacheTTL          string `yaml:"cache_ttl"`
	ReadTimeout       string `yaml:"read_timeout"`
	WriteTimeout      string `yaml:"write_timeout"`
	IdleTimeout       string `yaml:"idle_timeout"`
	ReadHeaderTimeout string `yaml:"read_header_timeout"`
	LogFile           string `yaml:"log_file"`
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		DocsDir:           "./docs",
		Port:              "8080",
		CacheTTL:          5 * time.Minute,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		LogFile:           "access.log",
	}
}

// LoadConfig reads config from the given path if it exists, applying it on top of defaults.
// If the file does not exist, defaults are returned without error.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config file %q: %w", path, err)
	}

	var yc yamlConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return cfg, fmt.Errorf("parse config file %q: %w", path, err)
	}

	// Simple string fields.
	if yc.DocsDir != "" {
		cfg.DocsDir = yc.DocsDir
	}
	if yc.Port != "" {
		cfg.Port = yc.Port
	}
	if yc.LogFile != "" {
		cfg.LogFile = yc.LogFile
	}

	// Durations.
	var perr error
	if yc.CacheTTL != "" {
		cfg.CacheTTL, perr = parseDurationField("cache_ttl", yc.CacheTTL)
		if perr != nil {
			return cfg, perr
		}
	}
	if yc.ReadTimeout != "" {
		cfg.ReadTimeout, perr = parseDurationField("read_timeout", yc.ReadTimeout)
		if perr != nil {
			return cfg, perr
		}
	}
	if yc.WriteTimeout != "" {
		cfg.WriteTimeout, perr = parseDurationField("write_timeout", yc.WriteTimeout)
		if perr != nil {
			return cfg, perr
		}
	}
	if yc.IdleTimeout != "" {
		cfg.IdleTimeout, perr = parseDurationField("idle_timeout", yc.IdleTimeout)
		if perr != nil {
			return cfg, perr
		}
	}
	if yc.ReadHeaderTimeout != "" {
		cfg.ReadHeaderTimeout, perr = parseDurationField("read_header_timeout", yc.ReadHeaderTimeout)
		if perr != nil {
			return cfg, perr
		}
	}

	return cfg, nil
}

func parseDurationField(name, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %q: %w", name, value, err)
	}
	return d, nil
}
