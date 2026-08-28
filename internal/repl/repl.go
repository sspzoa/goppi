package repl

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/tools"
	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/upstage"
)

func ApplySession(a *agent.Agent, f session.File) {
	a.SessionID = f.ID
	a.Messages = f.Messages
	if f.CacheKey != "" {
		a.Cfg.PromptCacheKey = f.CacheKey
	}
	if f.Model != "" {
		a.Cfg.Model = f.Model
	}
}

func NewAgent(cfg config.Config) (*agent.Agent, error) {
	key := cfg.ResolveAPIKey()
	if key == "" {
		return nil, upstage.MissingKeyError()
	}
	api := upstage.New(key, cfg.BaseURL)
	if cfg.PromptCacheKey == "" {
		cfg.PromptCacheKey = newCacheKey()
	}
	a := agent.New(cfg, provider.NewSolar(api), tools.New(cfg.WorkDir, api, permissionAsk(cfg)))
	if cfg.OutputFormat == "json" {
		a.Quiet = true
	}
	return a, nil
}

func permissionAsk(cfg config.Config) tools.Ask {
	if cfg.AlwaysApprove {
		return nil
	}
	if cfg.OutputFormat == "json" {
		return func(string, string) bool { return false }
	}
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return func(string, string) bool { return false }
	}
	return func(name, detail string) bool {
		ui.Warn("%s  %s", name, detail)
		fmt.Fprint(os.Stderr, "  allow? [y/N] ")
		sc := bufio.NewScanner(os.Stdin)
		if !sc.Scan() {
			return false
		}
		s := strings.ToLower(strings.TrimSpace(sc.Text()))
		return s == "y" || s == "yes"
	}
}

func RunOnce(ctx context.Context, a *agent.Agent, prompt string) error {
	if err := a.Run(ctx, prompt); err != nil {
		return err
	}
	return saveSession(a)
}

func saveSession(a *agent.Agent) error {
	id, err := session.Persist(a.Cfg, a.SessionID, a.Messages)
	if err != nil {
		return err
	}
	a.SessionID = id
	return nil
}

func Loop(ctx context.Context, a *agent.Agent) error {
	ui.Banner(config.Version, a.Cfg.Model, a.Cfg.ReasoningEffort, a.Cfg.WorkDir)
	ui.Hint("메시지를 입력하세요.  /help 로 명령을 봅니다.")

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for {
		ui.UserPrompt()
		if !sc.Scan() {
			fmt.Fprintln(ui.Out())
			return sc.Err()
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			if quit, err := handleSlash(a, line); quit || err != nil {
				return err
			}
			continue
		}
		if err := a.Run(ctx, line); err != nil {
			ui.Error("%s", err)
			continue
		}
		if err := saveSession(a); err != nil {
			ui.Warn("세션 저장 실패: %s", err)
		}
	}
}

func handleSlash(a *agent.Agent, line string) (quit bool, err error) {
	fields := strings.Fields(line)
	cmd := fields[0]
	arg := strings.TrimSpace(strings.TrimPrefix(line, cmd))
	switch cmd {
	case "/help", "/?":
		ui.Help()
	case "/quit", "/exit", "/q":
		return true, nil
	case "/new":
		a.Reset()
		a.SessionID = session.NewID()
		a.Cfg.PromptCacheKey = newCacheKey()
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
	default:
		ui.Warn("모르는 명령입니다. /help")
	}
	return false, nil
}

func printModels(current string) {
	for _, m := range upstage.ChatModels {
		ui.ModelRow(m.ID == current, m.ID, m.Summary)
	}
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

func newCacheKey() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("goppi-%d", os.Getpid())
	}
	return "goppi-" + hex.EncodeToString(b[:])
}
