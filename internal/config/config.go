package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sspzoa/goppi/internal/upstage"
)

const Version = "0.3.0"

var Efforts = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

type Config struct {
	APIKey          string `json:"api_key,omitempty"`
	BaseURL         string `json:"base_url,omitempty"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	MaxTurns        int    `json:"max_turns"`
	WorkDir         string `json:"workdir"`
	MaxTokens       int    `json:"max_tokens,omitempty"`
	PromptCacheKey  string `json:"prompt_cache_key,omitempty"`
}

func Default() Config {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return Config{
		BaseURL:   upstage.DefaultBaseURL,
		Model:     upstage.DefaultModel,
		MaxTurns:  30,
		WorkDir:   wd,
		MaxTokens: 32768,
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
	return upstage.ResolveAPIKey(c.APIKey)
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
	if v := os.Getenv("GOPPI_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("UPSTAGE_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("GOPPI_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("GOPPI_WORKDIR"); v != "" {
		cfg.WorkDir = v
	}
	if v := os.Getenv("GOPPI_EFFORT"); v != "" {
		cfg.ReasoningEffort = v
	}
	if v := os.Getenv("UPSTAGE_REASONING_EFFORT"); v != "" {
		cfg.ReasoningEffort = v
	}
}

func normalize(cfg *Config) error {
	if cfg.BaseURL == "" {
		cfg.BaseURL = upstage.DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 30
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 32768
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
		cfg.Model = upstage.DefaultModel
	}
	cfg.ReasoningEffort = strings.ToLower(strings.TrimSpace(cfg.ReasoningEffort))
	if cfg.ReasoningEffort != "" && !validEffort(cfg.ReasoningEffort) {
		return fmt.Errorf("unknown reasoning_effort %q (%s)", cfg.ReasoningEffort, strings.Join(Efforts, "|"))
	}
	if cfg.Model == "solar-mini" {
		cfg.ReasoningEffort = ""
	}
	return nil
}

func validEffort(s string) bool {
	for _, e := range Efforts {
		if e == s {
			return true
		}
	}
	return false
}

func UserDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "goppi")
	return dir, os.MkdirAll(dir, 0o755)
}
