package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
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
	format := fs.String("output-format", "plain", "plain | json")
	always := fs.Bool("always-approve", false, "do not ask before write/bash")
	fs.BoolVar(always, "yolo", false, "alias for --always-approve")
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
	cfg.AlwaysApprove = cfg.AlwaysApprove || *always
	cfg.OutputFormat = strings.ToLower(*format)
	if cfg.OutputFormat != "plain" && cfg.OutputFormat != "json" {
		return fmt.Errorf("output-format must be plain or json")
	}
	if err := cfg.Normalize(); err != nil {
		return err
	}

	a, err := repl.NewAgent(cfg)
	if err != nil {
		return err
	}
	var resumeNote string
	switch {
	case *resume != "":
		f, err := session.Load(*resume)
		if err != nil {
			return fmt.Errorf("session %s: %w", *resume, err)
		}
		repl.ApplySession(a, f)
		resumeNote = fmt.Sprintf("세션 %s 을 이었습니다. (%d messages)", f.ID, len(f.Messages))
	case *cont:
		last, err := session.LoadLast()
		if err != nil {
			return fmt.Errorf("last session: %w", err)
		}
		repl.ApplySession(a, last)
		resumeNote = fmt.Sprintf("이전 세션을 이었습니다. (%s, %d messages)", last.ID, len(last.Messages))
	}

	text := strings.TrimSpace(*prompt)
	if text == "" {
		text = strings.TrimSpace(strings.Join(fs.Args(), " "))
	}
	if text != "" {
		if resumeNote != "" {
			ui.Info("%s", resumeNote)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if err := repl.RunOnce(ctx, a, text); err != nil {
			return err
		}
		if cfg.OutputFormat == "json" {
			return writeJSONResult(a)
		}
		return nil
	}
	return repl.Loop(context.Background(), a)
}

func writeJSONResult(a *agent.Agent) error {
	text, reasoning := "", ""
	for i := len(a.Messages) - 1; i >= 0; i-- {
		if a.Messages[i].Role == provider.RoleAssistant {
			text = a.Messages[i].Content
			reasoning = a.Messages[i].Reasoning
			break
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"text":       text,
		"reasoning":  reasoning,
		"usage":      a.LastUsage,
		"session_id": a.SessionID,
	})
}
