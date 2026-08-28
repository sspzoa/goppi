package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sspzoa/goppi/internal/upstage"
)

const Version = "0.6.1"

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
	AlwaysApprove   bool   `json:"always_approve,omitempty"`
	OutputFormat    string `json:"-"`
}

func Default() Config {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return Config{
		BaseURL:         upstage.DefaultBaseURL,
		Model:           upstage.DefaultModel,
		ReasoningEffort: "medium",
		MaxTurns:        30,
		WorkDir:         wd,
		MaxTokens:       32768,
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
	if k := upstage.ResolveAPIKey(c.APIKey); k != "" {
		return k
	}
	return LoadStoredAPIKey()
}

func (c Config) KeySource() string {
	if strings.TrimSpace(c.APIKey) != "" {
		return "config.json"
	}
	if os.Getenv("UPSTAGE_API_KEY") != "" {
		return "UPSTAGE_API_KEY"
	}
	if os.Getenv("GOPPI_API_KEY") != "" {
		return "GOPPI_API_KEY"
	}
	if LoadStoredAPIKey() != "" {
		return "goppi login"
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
	if os.Getenv("GOPPI_ALWAYS_APPROVE") == "1" {
		cfg.AlwaysApprove = true
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
	} else if cfg.ReasoningEffort == "" {
		// solar-pro4 docs say omit == on, but with tools+stream the API
		// often skips reasoning unless effort is sent explicitly.
		cfg.ReasoningEffort = "medium"
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

func UserConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "goppi")
	return dir, os.MkdirAll(dir, 0o700)
}

func UserDataDir() (string, error) {
	if v := os.Getenv("GOPPI_DATA_DIR"); v != "" {
		return v, os.MkdirAll(v, 0o755)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "goppi")
	return dir, os.MkdirAll(dir, 0o755)
}

func credentialsPath() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

func LoadStoredAPIKey() string {
	path, err := credentialsPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var file struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return ""
	}
	return strings.TrimSpace(file.APIKey)
}

func SaveAPIKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("empty API key")
	}
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		APIKey string `json:"api_key"`
	}{APIKey: key}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func ClearAPIKey() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
