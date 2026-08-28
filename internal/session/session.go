package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
)

type File struct {
	UpdatedAt time.Time          `json:"updated_at"`
	WorkDir   string             `json:"workdir"`
	Model     string             `json:"model"`
	Effort    string             `json:"reasoning_effort,omitempty"`
	CacheKey  string             `json:"prompt_cache_key,omitempty"`
	Messages  []provider.Message `json:"messages"`
}

func lastPath() (string, error) {
	dir, err := config.UserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "last.json"), nil
}

func Save(cfg config.Config, messages []provider.Message) error {
	path, err := lastPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(File{
		UpdatedAt: time.Now(),
		WorkDir:   cfg.WorkDir,
		Model:     cfg.Model,
		Effort:    cfg.ReasoningEffort,
		CacheKey:  cfg.PromptCacheKey,
		Messages:  messages,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func LoadLast() (File, error) {
	var f File
	path, err := lastPath()
	if err != nil {
		return f, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, err
	}
	return f, nil
}
