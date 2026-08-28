package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/repl"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/ui"
)

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("goppi", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { printHelp() }
	prompt := fs.String("p", "", "headless prompt")
	fs.StringVar(prompt, "prompt", "", "headless prompt")
	model := fs.String("m", "", "model")
	fs.StringVar(model, "model", "", "model")
	effort := fs.String("effort", "", "reasoning effort")
	workdir := fs.String("C", "", "working directory")
	fs.StringVar(workdir, "cwd", "", "working directory")
	maxTurns := fs.Int("max-turns", 0, "max agent turns")
	cont := fs.Bool("c", false, "resume last session")
	fs.BoolVar(cont, "continue", false, "resume last session")
	resume := fs.String("r", "", "resume session id")
	fs.StringVar(resume, "resume", "", "resume session id")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if *model != "" {
		cfg.Model = *model
	}
	if *effort != "" {
		cfg.ReasoningEffort = *effort
	}
	if *workdir != "" {
		cfg.WorkDir = *workdir
	}
	if *maxTurns > 0 {
		cfg.MaxTurns = *maxTurns
	}
	if err := cfg.Normalize(); err != nil {
		return err
	}

	a, err := repl.NewAgent(cfg)
	if err != nil {
		return err
	}
	switch {
	case *resume != "":
		f, err := session.Load(*resume)
		if err != nil {
			return fmt.Errorf("session %s: %w", *resume, err)
		}
		repl.ApplySession(a, f)
		ui.Info("세션 %s 을 이었습니다. (%d messages)", f.ID, len(f.Messages))
	case *cont:
		last, err := session.LoadLast()
		if err != nil {
			return fmt.Errorf("last session: %w", err)
		}
		repl.ApplySession(a, last)
		ui.Info("이전 세션을 이었습니다. (%s, %d messages)", last.ID, len(last.Messages))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	text := strings.TrimSpace(*prompt)
	if text == "" {
		text = strings.TrimSpace(strings.Join(fs.Args(), " "))
	}
	if text != "" {
		return repl.RunOnce(ctx, a, text)
	}
	return repl.Loop(ctx, a)
}
