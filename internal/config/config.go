package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const Version = "0.1.0"

type Config struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	MaxTurns  int    `json:"max_turns"`
	WorkDir   string `json:"workdir"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

func Default() Config {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return Config{
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-5",
		MaxTurns:  30,
		WorkDir:   wd,
		MaxTokens: 8192,
	}
}

func Load() (Config, error) {
	cfg := Default()
	for _, path := range searchPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return cfg, fmt.Errorf("config %s: %w", path, err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("config %s: %w", path, err)
		}
	}
	applyEnv(&cfg)
	if err := cfg.Normalize(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) Normalize() error {
	return normalize(c)
}

func (c Config) ResolveAPIKey() string {
	if c.APIKey != "" {
		return c.APIKey
	}
	if v := os.Getenv("GOPPI_API_KEY"); v != "" {
		return v
	}
	switch strings.ToLower(c.Provider) {
	case "anthropic":
		return os.Getenv("ANTHROPIC_API_KEY")
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	}
	return ""
}

func searchPaths() []string {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "goppi", "config.json"))
	}
	if wd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(wd, ".goppi.json"))
	}
	return paths
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("GOPPI_PROVIDER"); v != "" {
		cfg.Provider = v
	}
	if v := os.Getenv("GOPPI_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("GOPPI_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("GOPPI_WORKDIR"); v != "" {
		cfg.WorkDir = v
	}
}

func normalize(cfg *Config) error {
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch cfg.Provider {
	case "anthropic", "openai":
	default:
		return fmt.Errorf("unknown provider %q (anthropic|openai)", cfg.Provider)
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 30
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 8192
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = "."
	}
	abs, err := filepath.Abs(cfg.WorkDir)
	if err != nil {
		return fmt.Errorf("workdir: %w", err)
	}
	cfg.WorkDir = abs
	if cfg.Model == "" {
		switch cfg.Provider {
		case "anthropic":
			cfg.Model = "claude-sonnet-4-5"
		case "openai":
			cfg.Model = "gpt-4.1"
		}
	}
	if cfg.BaseURL == "" && cfg.Provider == "openai" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.BaseURL == "" && cfg.Provider == "anthropic" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	return nil
}

func UserDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "goppi")
	return dir, os.MkdirAll(dir, 0o755)
}
