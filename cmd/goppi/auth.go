package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/upstage"
)

func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	stdin := fs.Bool("stdin", false, "read API key from stdin")
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
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		key = strings.TrimSpace(string(data))
	}
	if key == "" && fs.NArg() > 0 {
		key = strings.TrimSpace(fs.Arg(0))
	}
	if key == "" {
		fmt.Fprintf(os.Stderr, "Upstage API 키를 입력하세요 (%s)\n> ", upstage.ConsoleURL)
		sc := bufio.NewScanner(os.Stdin)
		if !sc.Scan() {
			return sc.Err()
		}
		key = strings.TrimSpace(sc.Text())
	}
	if err := config.SaveAPIKey(key); err != nil {
		return err
	}
	ui.Info("키를 저장했습니다. (%s)", maskKey(key))
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

func maskKey(key string) string {
	if len(key) <= 8 {
		return "********"
	}
	return key[:4] + strings.Repeat("•", 8) + key[len(key)-4:]
}
