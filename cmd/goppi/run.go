package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/repl"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/tools"
	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/worktree"
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
	mode := fs.String("mode", "", "act | plan")
	prov := fs.String("provider", "", "upstage | openai | compat")
	always := fs.Bool("always-approve", false, "do not ask before write/bash/MCP")
	fs.BoolVar(always, "yolo", false, "alias for --always-approve")
	sandbox := fs.String("sandbox", "", "workspace | strict | off")
	isolate := fs.Bool("worktree", false, "run in a git worktree so the main checkout stays clean")
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
	if *mode != "" {
		cfg.Mode = *mode
	}
	if *prov != "" {
		cfg.Provider = *prov
	}
	cfg.AlwaysApprove = cfg.AlwaysApprove || *always
	if *sandbox != "" {
		cfg.Sandbox = *sandbox
	}
	cfg.Worktree = cfg.Worktree || *isolate
	headlessText := strings.TrimSpace(*prompt)
	if headlessText == "" {
		headlessText = strings.TrimSpace(strings.Join(fs.Args(), " "))
	}
	cfg.OutputFormat = strings.ToLower(*format)
	if cfg.OutputFormat != "plain" && cfg.OutputFormat != "json" {
		err := fmt.Errorf("output-format must be plain or json")
		if headlessText != "" {
			_ = writeJSONResult(&agent.Agent{Cfg: cfg}, err)
		}
		return err
	}
	jsonHeadless := cfg.OutputFormat == "json" && headlessText != ""
	emitJSONErr := func(err error, a *agent.Agent, sessID string) error {
		if err == nil || !jsonHeadless {
			return err
		}
		ag := a
		if ag == nil {
			ag = &agent.Agent{Cfg: cfg}
		}
		if ag.SessionID == "" && sessID != "" {
			ag.SessionID = sessID
		}
		if werr := writeJSONResult(ag, err); werr != nil {
			return werr
		}
		return err
	}
	if err := cfg.Normalize(); err != nil {
		return emitJSONErr(err, nil, "")
	}

	var loaded *session.File
	var resumeNote string
	switch {
	case *resume != "":
		f, err := session.Resolve(*resume)
		if err != nil {
			return emitJSONErr(fmt.Errorf("session %s: %w", *resume, err), nil, "")
		}
		loaded = &f
		resumeNote = fmt.Sprintf("세션 %s 을 이었습니다. (%d messages)", f.ID, len(f.Messages))
	case *cont:
		last, err := session.LoadLast()
		if err != nil {
			return emitJSONErr(fmt.Errorf("last session: %w", err), nil, "")
		}
		loaded = &last
		resumeNote = fmt.Sprintf("이전 세션을 이었습니다. (%s, %d messages)", last.ID, len(last.Messages))
	}

	isolateID := ""
	if cfg.Worktree {
		isolateID = session.NewID()
		if loaded != nil {
			isolateID = loaded.ID
		}
		wt, err := worktree.Ensure(cfg.WorkDir, isolateID)
		if err != nil {
			return emitJSONErr(err, nil, isolateID)
		}
		cfg.WorkDir = wt.Path
		if cfg.OutputFormat != "json" {
			ui.Info("git worktree %s  (%s)", wt.Branch, ui.ShortPath(wt.Path))
		}
	}

	a, err := repl.NewAgent(cfg)
	if err != nil {
		return emitJSONErr(err, nil, "")
	}
	defer a.Close()
	if loaded != nil {
		if err := repl.ApplySession(a, *loaded); err != nil {
			return emitJSONErr(err, a, loaded.ID)
		}
	} else if isolateID != "" {
		a.SessionID = isolateID
	}

	text := headlessText
	ctx, stop := ui.NotifyStop(context.Background())
	if text != "" {
		ctx, stop = ui.NotifyStopHeadless(context.Background())
	}
	defer stop()
	if text != "" {
		if resumeNote != "" {
			ui.Info("%s", resumeNote)
		}
		runErr := repl.RunOnce(ctx, a, text)
		if cfg.OutputFormat == "json" {
			if err := writeJSONResult(a, runErr); err != nil && runErr == nil {
				return err
			}
			return runErr
		}
		return runErr
	}
	return repl.Loop(ctx, a)
}

func writeJSONResult(a *agent.Agent, runErr error) error {
	text, reasoning := "", ""
	for i := len(a.Messages) - 1; i >= 0; i-- {
		if a.Messages[i].Role == provider.RoleAssistant {
			text = tools.RedactSecrets(a.Messages[i].Content)
			reasoning = tools.RedactSecrets(a.Messages[i].Reasoning)
			break
		}
	}
	out := map[string]any{
		"text":       text,
		"reasoning":  reasoning,
		"usage":      a.LastUsage,
		"session_id": a.SessionID,
		"mode":       a.Cfg.Mode,
		"workdir":    a.Cfg.WorkDir,
		"worktree":   a.Cfg.Worktree,
	}
	if runErr != nil {
		out["error"] = tools.RedactSecrets(runErr.Error())
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
