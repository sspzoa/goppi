package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/upstage"
)

func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	stdin := fs.Bool("stdin", false, "read API key from stdin")
	offline := fs.Bool("offline", false, "save without calling the API")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	key := strings.TrimSpace(os.Getenv("UPSTAGE_API_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("GOPPI_API_KEY"))
	}
	if *stdin {
		data, err := io.ReadAll(io.LimitReader(os.Stdin, 4096))
		if err != nil {
			return err
		}
		key = strings.TrimSpace(string(data))
		if key == "" {
			return fmt.Errorf("empty API key")
		}
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("do not pass the API key as an argument (ps and shell history keep it); use goppi login or goppi login --stdin")
	}
	if key == "" {
		fmt.Fprintf(os.Stderr, "API 키를 입력하세요 (%s)\n> ", upstage.ConsoleURL)
		sc := bufio.NewScanner(os.Stdin)
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return err
			}
			return fmt.Errorf("empty API key")
		}
		key = strings.TrimSpace(sc.Text())
		if key == "" {
			return fmt.Errorf("empty API key")
		}
	}
	if !*offline {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		api := upstage.New(key, cfg.BaseURL)
		api.UserAgent = "goppi/" + config.Version
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := api.ProbeKey(ctx, cfg.Model); err != nil {
			return fmt.Errorf("키를 저장하지 않았습니다: %w", err)
		}
	}
	if err := config.SaveAPIKey(key); err != nil {
		return err
	}
	if *offline {
		ui.Info("키를 저장했습니다. (확인 생략)")
	} else {
		ui.Info("키를 확인·저장했습니다.")
	}
	return nil
}

func cmdLogout() error {
	if err := config.ClearAPIKey(); err != nil {
		return err
	}
	ui.Info("저장된 키를 삭제했습니다.")
	return nil
}

func cmdVersion() error {
	fmt.Printf("goppi %s\n", config.Version)
	return nil
}
