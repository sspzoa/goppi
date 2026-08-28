package main

import (
	"os"
	"path/filepath"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/instructions"
	"github.com/sspzoa/goppi/internal/ui"
)

func cmdInit() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	path := filepath.Join(cfg.WorkDir, "GOPPI.md")
	if _, err := os.Stat(path); err == nil {
		ui.Info("already exists: %s", path)
		return nil
	}
	if err := os.WriteFile(path, []byte(instructions.Template), 0o644); err != nil {
		return err
	}
	ui.Info("wrote %s", path)
	return nil
}
