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

func main() {
	if err := run(os.Args[1:]); err != nil {
		ui.Error("%s", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("goppi", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `goppi — 고삐, 로컬 에이전트 하네스

사용:
  goppi                     REPL
  goppi "할 일"              한 번 실행
  goppi --continue           마지막 세션 이어서

`)
		fs.PrintDefaults()
	}
	showVersion := fs.Bool("version", false, "print version")
	providerName := fs.String("provider", "", "anthropic | openai")
	model := fs.String("model", "", "model id")
	workdir := fs.String("C", "", "working directory")
	maxTurns := fs.Int("max-turns", 0, "max agent turns")
	cont := fs.Bool("continue", false, "resume last session")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if *showVersion {
		fmt.Println(config.Version)
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if *providerName != "" {
		cfg.Provider = *providerName
	}
	if *model != "" {
		cfg.Model = *model
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
	if *cont {
		last, err := session.LoadLast()
		if err != nil {
			return fmt.Errorf("last session: %w", err)
		}
		a.Messages = last.Messages
		ui.Info("이전 세션을 이었습니다. (%d messages)", len(last.Messages))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	rest := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if rest != "" {
		return repl.RunOnce(ctx, a, rest)
	}
	return repl.Loop(ctx, a)
}
