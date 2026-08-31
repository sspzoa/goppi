package repl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/lsp"
	"github.com/sspzoa/goppi/internal/mcp"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/skills"
	"github.com/sspzoa/goppi/internal/tools"
	"github.com/sspzoa/goppi/internal/tui"
	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/upstage"
	"github.com/sspzoa/goppi/internal/worktree"
)

func ApplySession(a *agent.Agent, f session.File) error {
	return a.LoadFile(f)
}

func NewAgent(cfg config.Config) (*agent.Agent, error) {
	key := cfg.ResolveAPIKey()
	if key == "" {
		return nil, upstage.MissingKeyError()
	}
	api := upstage.New(key, cfg.BaseURL)
	api.UserAgent = "goppi/" + config.Version
	if cfg.PromptCacheKey == "" {
		cfg.PromptCacheKey = session.NewCacheKey()
	}
	var doc *upstage.Client
	if cfg.DocumentTools() {
		doc = api
	}
	reg := tools.New(cfg.WorkDir, doc, permissionAsk(cfg))
	reg.SetMode(cfg.Mode)
	reg.SetSandbox(cfg.Sandbox)
	reg.SetSkills(skills.Load(cfg.WorkDir))
	reg.SetHooks(cfg.Hooks)
	reg.SetAskUser(userAsk(cfg))
	reg.SetExtraDirs(cfg.ExtraDirs)
	sessions, mcpErrs := mcp.StartAll(context.Background(), cfg.MCPServers, cfg.WorkDir, config.Version, func(cmd *exec.Cmd) error {
		return tools.ApplySandbox(cmd, cfg.WorkDir, cfg.Sandbox, cfg.ExtraDirs...)
	})
	reg.AttachMCP(sessions)
	hub, lspErrs := lsp.StartAll(context.Background(), cfg.LSPServers, cfg.WorkDir, config.Version, func(cmd *exec.Cmd) error {
		return tools.ApplySandbox(cmd, cfg.WorkDir, cfg.Sandbox, cfg.ExtraDirs...)
	})
	reg.AttachLSP(hub)
	if cfg.OutputFormat != "json" {
		for _, e := range mcpErrs {
			ui.Warn("%s", e)
		}
		for _, e := range lspErrs {
			ui.Warn("%s", e)
		}
	}
	client := provider.NewSolar(api)
	reg.EnableDelegate(func(ctx context.Context, prompt string) (string, error) {
		return runDelegate(ctx, cfg, client, prompt)
	})
	a := agent.New(cfg, client, reg)
	if cfg.OutputFormat == "json" {
		a.Quiet = true
	} else {
		a.Sink = ui.NewPrinter()
	}
	if err := a.Begin(); err != nil && cfg.OutputFormat != "json" {
		ui.Warn("hook session_start: %s", err)
	}
	return a, nil
}

func runDelegate(ctx context.Context, cfg config.Config, client provider.Client, prompt string) (string, error) {
	subCfg := cfg
	subCfg.Mode = "plan"
	subCfg.MaxTurns = 8
	subCfg.AlwaysApprove = false
	subCfg.MCPServers = nil
	subReg := tools.New(cfg.WorkDir, nil, nil)
	subReg.SetMode("plan")
	subReg.SetSkills(skills.Load(cfg.WorkDir))
	subReg.SetHooks(cfg.Hooks)
	sub := agent.New(subCfg, client, subReg)
	sub.Quiet = true
	err := sub.Run(ctx, prompt)
	text := strings.TrimSpace(sub.LastAssistant())
	if text == "" {
		if err != nil {
			return "", err
		}
		return "subagent produced no reply", nil
	}
	if err != nil {
		return text + "\n(stopped: " + err.Error() + ")", nil
	}
	return text, nil
}

func userAsk(cfg config.Config) tools.AskUser {
	if cfg.OutputFormat == "json" || !isTTY(os.Stdin) {
		return nil
	}
	return func(question string, options []string) (string, error) {
		ui.Warn("%s", question)
		if len(options) > 0 {
			for i, o := range options {
				fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, o)
			}
		}
		fmt.Fprint(os.Stderr, "  ? ")
		sc := bufio.NewScanner(os.Stdin)
		if !sc.Scan() {
			return "", fmt.Errorf("user skipped")
		}
		s := strings.TrimSpace(sc.Text())
		if s == "" {
			return "", fmt.Errorf("user skipped")
		}
		if len(options) > 0 {
			var n int
			if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n >= 1 && n <= len(options) {
				return options[n-1], nil
			}
		}
		return s, nil
	}
}

func permissionAsk(cfg config.Config) tools.Ask {
	if cfg.AlwaysApprove {
		return nil
	}
	if cfg.OutputFormat == "json" {
		return tools.AlwaysDeny
	}
	if !isTTY(os.Stdin) {
		return tools.AlwaysDeny
	}
	return func(name, detail string) tools.Verdict {
		ui.Warn("%s", name)
		for _, line := range strings.Split(strings.TrimRight(detail, "\n"), "\n") {
			if strings.TrimSpace(line) != "" {
				ui.Info("%s", line)
			}
		}
		fmt.Fprint(os.Stderr, "  allow? [y once / a session / N] ")
		sc := bufio.NewScanner(os.Stdin)
		if !sc.Scan() {
			return tools.Denied
		}
		s := strings.ToLower(strings.TrimSpace(sc.Text()))
		switch s {
		case "a", "always", "session":
			return tools.AllowedSession
		case "y", "yes":
			return tools.Allowed
		default:
			return tools.Denied
		}
	}
}

func RunOnce(ctx context.Context, a *agent.Agent, prompt string) error {
	a.EnsureSession()
	runErr := a.Run(ctx, prompt)
	if err := saveSession(a); err != nil && runErr == nil {
		return err
	}
	return runErr
}

func saveSession(a *agent.Agent) error {
	id, err := session.PersistFull(a.Cfg, a.SessionSnapshot())
	if err != nil {
		return fmt.Errorf("session save: %w", err)
	}
	a.SessionID = id
	return nil
}

func Loop(ctx context.Context, a *agent.Agent) error {
	if useTUI() {
		a.Sink = nil
		return tui.Run(ctx, a)
	}
	ui.CloseStdinOnDone(ctx)
	return lineLoop(ctx, a)
}

func useTUI() bool {
	if os.Getenv("GOPPI_TUI") == "0" {
		return false
	}
	return isTTY(os.Stdin) && isTTY(os.Stdout)
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func lineLoop(ctx context.Context, a *agent.Agent) error {
	ui.Banner(config.Version, a.Cfg.Model, a.Cfg.ReasoningEffort, a.Cfg.WorkDir)
	if a.Cfg.AlwaysApprove {
		ui.Warn("always_approve 켜짐 — write/bash/MCP를 묻지 않습니다")
	}
	ui.Hint("메시지를 입력하세요.  /help 로 명령을 봅니다.")

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for {
		ui.UserPrompt()
		line, err := readPrompt(ctx, sc)
		if err != nil {
			fmt.Fprintln(ui.Out())
			if len(a.Messages) > 0 {
				if err := saveSession(a); err != nil {
					return err
				}
			}
			if errors.Is(err, errPromptQuit) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			if quit, err := handleSlash(a, line); quit || err != nil {
				return err
			}
			continue
		}
		if err := runLineTurn(ctx, a, line); err != nil {
			if errors.Is(err, context.Canceled) && ctx.Err() == nil {
				ui.Info("취소했습니다.")
			} else if ctx.Err() != nil {
				if serr := saveSession(a); serr != nil {
					return serr
				}
				if err != nil && !errors.Is(err, context.Canceled) {
					return err
				}
				return nil
			} else {
				ui.Error("%s", err)
				ui.NotifyDone()
			}
		} else {
			ui.NotifyDone()
		}
		if err := saveSession(a); err != nil {
			if ctx.Err() != nil {
				return err
			}
			ui.Warn("세션 저장 실패: %s", err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

var errPromptQuit = errors.New("quit")

type promptEvent struct {
	line string
	err  error
}

func pickPrompt(ctx context.Context, scanned <-chan promptEvent, sig <-chan os.Signal) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-sig:
		return "", errPromptQuit
	case ev := <-scanned:
		return ev.line, ev.err
	}
}

func readPrompt(ctx context.Context, sc *bufio.Scanner) (string, error) {
	scanned := make(chan promptEvent, 1)
	go func() {
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				scanned <- promptEvent{err: err}
				return
			}
			scanned <- promptEvent{err: errPromptQuit}
			return
		}
		scanned <- promptEvent{line: sc.Text()}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	return pickPrompt(ctx, scanned, sig)
}

// runLineTurn cancels only this turn on SIGINT. The parent context
// stays alive so the next prompt still works.
func runLineTurn(ctx context.Context, a *agent.Agent, line string) error {
	turnCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	a.EnsureSession()
	return a.Run(turnCtx, line)
}

func handleSlash(a *agent.Agent, line string) (quit bool, err error) {
	fields := strings.Fields(line)
	cmd := fields[0]
	arg := strings.TrimSpace(strings.TrimPrefix(line, cmd))
	switch cmd {
	case "/help", "/?":
		ui.Help()
	case "/quit", "/exit", "/q":
		if len(a.Messages) > 0 {
			if err := saveSession(a); err != nil {
				return true, err
			}
		}
		return true, nil
	case "/new", "/clear":
		if len(a.Messages) > 0 {
			if err := saveSession(a); err != nil {
				ui.Error("세션 저장 실패: %s", err)
				return false, nil
			}
		}
		a.Reset()
		a.Cfg.PromptCacheKey = session.NewCacheKey()
		a.EnsureSession()
		ui.Info("세션을 초기화했습니다. (%s)", a.SessionID)
	case "/tools":
		ui.Info("%s", strings.Join(a.Tools.Names(), ", "))
	case "/model":
		if arg == "" {
			printModels(a.Cfg.Model)
			return false, nil
		}
		a.Cfg.Model = arg
		if a.Cfg.Model == "solar-mini" {
			a.Cfg.ReasoningEffort = ""
		}
		if err := a.Cfg.Normalize(); err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		ui.Info("model → %s", a.Cfg.Model)
	case "/effort":
		if arg == "" {
			ui.Info("%s", uiEffort(a.Cfg))
			return false, nil
		}
		a.Cfg.ReasoningEffort = arg
		if err := a.Cfg.Normalize(); err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		ui.Info("effort → %s", uiEffort(a.Cfg))
	case "/plan":
		setMode(a, "plan")
	case "/act":
		setMode(a, "act")
	case "/undo":
		if a.Tools == nil {
			ui.Error("nothing to undo")
			return false, nil
		}
		msg, err := a.Tools.UndoLast()
		if err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		ui.Info("%s", msg)
	case "/compact":
		if err := a.Compact(context.Background()); err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		ui.Info("세션을 압축했습니다. (%d messages)", len(a.Messages))
	case "/skills":
		names := skills.Names(skills.Load(a.Cfg.WorkDir))
		if len(names) == 0 {
			ui.Info("skill 없음. .goppi/skills/<name>/SKILL.md")
		} else {
			ui.Info("%s", strings.Join(names, ", "))
		}
	case "/mcp":
		showMCP(a)
	case "/diff":
		if a.Tools == nil {
			ui.Info("(no edits)")
		} else {
			ui.Info("%s", a.Tools.SessionDiff())
		}
	case "/jobs":
		if a.Tools == nil {
			ui.Info("(no jobs)")
		} else {
			ui.Info("%s", a.Tools.JobSummary())
		}
	case "/export":
		path, err := a.ExportMarkdown(arg)
		if err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		ui.Info("내보냄 %s", path)
	case "/copy":
		if err := ui.CopyClipboard(a.LastAssistant()); err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		ui.Info("클립보드에 복사했습니다.")
	case "/retry":
		text, err := a.RewindLastUser()
		if err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		ui.Info("다시 보냅니다.")
		if err := runLineTurn(context.Background(), a, text); err != nil {
			if errors.Is(err, context.Canceled) {
				ui.Info("취소했습니다.")
			} else {
				ui.Error("%s", err)
				ui.NotifyDone()
			}
		} else {
			ui.NotifyDone()
		}
		if err := saveSession(a); err != nil {
			ui.Warn("세션 저장 실패: %s", err)
		}
	case "/sessions":
		if arg == "" {
			showSessionList()
			return false, nil
		}
		if len(a.Messages) > 0 {
			if err := saveSession(a); err != nil {
				ui.Error("세션 저장 실패: %s", err)
				return false, nil
			}
		}
		f, err := session.Resolve(arg)
		if err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		if err := ApplySession(a, f); err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		ui.Info("세션을 이었습니다. (%s, %d messages)", a.SessionID, len(a.Messages))
	case "/delete":
		id := arg
		if id == "" {
			id = a.SessionID
		} else {
			f, err := session.Resolve(id)
			if err != nil {
				ui.Error("%s", err)
				return false, nil
			}
			id = f.ID
		}
		if id == "" {
			ui.Info("지울 세션이 없습니다.")
			return false, nil
		}
		current := id == a.SessionID
		if current {
			a.End("delete")
			a.ReleaseSession()
		} else {
			_ = tools.FireSessionEnd(context.Background(), a.Cfg, id, "delete")
		}
		if err := session.Delete(id); err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		if err := worktree.Remove(id); err != nil {
			ui.Warn("worktree: %s", err)
		}
		if current {
			a.Discard()
			a.Cfg.PromptCacheKey = session.NewCacheKey()
			a.EnsureSession()
			ui.Info("세션을 지웠습니다. (%s)", a.SessionID)
		} else {
			ui.Info("세션을 지웠습니다. (%s)", id)
		}
	case "/status":
		effort := uiEffort(a.Cfg)
		run, total := 0, 0
		if a.Tools != nil {
			run, total = a.Tools.JobCounts()
		}
		u := a.LastUsage
		ui.Info("%s  ·  %s  ·  %s  ·  sandbox %s  ·  worktree %v  ·  compact %s  ·  jobs %d/%d  ·  last %d→%d r%d  ·  session %s\n%s",
			a.Cfg.Mode, a.Cfg.Model, effort, a.Cfg.Sandbox, a.Cfg.Worktree, compactLabel(a.Cfg.AutoCompact), run, total, u.InputTokens, u.OutputTokens, u.ReasoningTokens, a.SessionID, a.Cfg.WorkDir)
	default:
		ui.Warn("모르는 명령입니다. /help")
	}
	return false, nil
}

func setMode(a *agent.Agent, mode string) {
	a.Cfg.Mode = mode
	if a.Tools != nil {
		a.Tools.SetMode(mode)
	}
	ui.Info("mode → %s", mode)
}

func showMCP(a *agent.Agent) {
	configured := a.Cfg.MCPNames()
	var live []string
	if a.Tools != nil {
		live = a.Tools.MCPNames()
	}
	if len(configured) == 0 && len(live) == 0 {
		ui.Info("mcp 없음. ~/.config/goppi/config.json 의 mcp_servers")
		return
	}
	if len(configured) > 0 {
		ui.Info("servers  %s", strings.Join(configured, ", "))
	}
	if len(live) > 0 {
		ui.Info("tools    %s", strings.Join(live, ", "))
	}
}

func showSessionList() {
	items, err := session.List()
	if err != nil {
		ui.Error("%s", err)
		return
	}
	if len(items) == 0 {
		ui.Info("세션이 없습니다.")
		return
	}
	limit := min(len(items), 8)
	for i := 0; i < limit; i++ {
		f := items[i]
		ui.Info("%s  %s  %s", f.ID[:min(8, len(f.ID))], f.UpdatedAt.Local().Format("01-02 15:04"), session.SafeTitle(f.Title))
	}
}

func printModels(current string) {
	for _, m := range upstage.ChatModels {
		ui.ModelRow(m.ID == current, m.ID, m.Summary)
	}
}

func compactLabel(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func uiEffort(cfg config.Config) string {
	if cfg.Model == "solar-mini" {
		return "n/a"
	}
	if cfg.ReasoningEffort == "" {
		return "default"
	}
	return cfg.ReasoningEffort
}
