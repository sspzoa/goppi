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

func NewAgent(cfg config.Config) (*agent.Agent, error) {
	key := cfg.ResolveAPIKey()
	if key == "" {
		return nil, upstage.MissingKeyError()
	}
	api := upstage.New(key, cfg.BaseURL)
	if cfg.PromptCacheKey == "" {
		cfg.PromptCacheKey = newCacheKey()
	}
	return agent.New(cfg, provider.NewSolar(api), tools.New(cfg.WorkDir, api)), nil
}

func RunOnce(ctx context.Context, a *agent.Agent, prompt string) error {
	if err := a.Run(ctx, prompt); err != nil {
		return err
	}
	return session.Save(a.Cfg, a.Messages)
}

func Loop(ctx context.Context, a *agent.Agent) error {
	ui.Banner(config.Version, a.Cfg.Model, a.Cfg.ReasoningEffort, a.Cfg.WorkDir)
	ui.Info("메시지를 입력하세요. /help 로 명령을 봅니다.")
	fmt.Fprintln(ui.Out())

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
		if err := session.Save(a.Cfg, a.Messages); err != nil {
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
		a.Cfg.PromptCacheKey = newCacheKey()
		ui.Info("세션을 초기화했습니다.")
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
		mark := " "
		if m.ID == current {
			mark = "*"
		}
		ui.Info("%s %s  %s", mark, m.ID, m.Summary)
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
