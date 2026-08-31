package main

import (
	"context"
	"fmt"

	"github.com/sspzoa/goppi/internal/acp"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/repl"
	"github.com/sspzoa/goppi/internal/ui"
)

func cmdACP() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.OutputFormat = "json"
	if err := cfg.Normalize(); err != nil {
		return err
	}
	ctx, stop := ui.NotifyStop(context.Background())
	defer stop()
	srv := &acp.Server{Cfg: cfg, New: repl.NewAgent, Ctx: ctx}
	if err := srv.Serve(); err != nil {
		return fmt.Errorf("acp: %w", err)
	}
	return nil
}
