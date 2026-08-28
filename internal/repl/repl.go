package repl

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/tools"
	"github.com/sspzoa/goppi/internal/ui"
)

func NewClient(cfg config.Config) (provider.Client, error) {
	key := cfg.ResolveAPIKey()
	if key == "" {
		return nil, fmt.Errorf("API 키가 없습니다. ANTHROPIC_API_KEY, OPENAI_API_KEY, 또는 GOPPI_API_KEY 를 설정하세요")
	}
	switch cfg.Provider {
	case "anthropic":
		return provider.NewAnthropic(key, cfg.BaseURL), nil
	case "openai":
		return provider.NewOpenAI(key, cfg.BaseURL), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}

func NewAgent(cfg config.Config) (*agent.Agent, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return agent.New(cfg, client, tools.New(cfg.WorkDir)), nil
}

func RunOnce(ctx context.Context, a *agent.Agent, prompt string) error {
	if err := a.Run(ctx, prompt); err != nil {
		return err
	}
	return session.Save(a.Cfg, a.Messages)
}

func Loop(ctx context.Context, a *agent.Agent) error {
	ui.Banner(config.Version, a.Cfg.Provider, a.Cfg.Model, a.Cfg.WorkDir)
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
		ui.Info("세션을 초기화했습니다.")
	case "/tools":
		ui.Info("%s", strings.Join(a.Tools.Names(), ", "))
	case "/model":
		if arg == "" {
			ui.Info("%s", a.Cfg.Model)
			return false, nil
		}
		a.Cfg.Model = arg
		ui.Info("model → %s", a.Cfg.Model)
	case "/provider":
		if arg == "" {
			ui.Info("%s", a.Cfg.Provider)
			return false, nil
		}
		a.Cfg.Provider = strings.ToLower(arg)
		client, err := NewClient(a.Cfg)
		if err != nil {
			ui.Error("%s", err)
			return false, nil
		}
		a.Client = client
		ui.Info("provider → %s", a.Cfg.Provider)
	default:
		ui.Warn("모르는 명령입니다. /help")
	}
	return false, nil
}
