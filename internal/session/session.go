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
	Provider  string             `json:"provider"`
	Model     string             `json:"model"`
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
		Provider:  cfg.Provider,
		Model:     cfg.Model,
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
