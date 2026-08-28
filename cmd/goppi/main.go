package main

import (
	"fmt"
	"os"

	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/upstage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		ui.Error("%s", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return cmdRun(nil)
	}
	switch args[0] {
	case "help", "-h", "--help":
		printHelp()
		return nil
	case "version", "--version", "-v":
		return cmdVersion()
	case "login":
		return cmdLogin(args[1:])
	case "logout":
		return cmdLogout()
	case "models":
		return cmdModels()
	case "doctor":
		return cmdDoctor()
	case "init":
		return cmdInit()
	case "inspect":
		return cmdInspect(args[1:])
	case "sessions":
		return cmdSessions(args[1:])
	case "export":
		return cmdExport(args[1:])
	default:
		return cmdRun(args)
	}
}

func printHelp() {
	fmt.Fprintf(os.Stderr, `goppi — Upstage Solar coding agent

Usage:
  goppi                     Interactive REPL
  goppi -p "task"           Headless one-shot
  goppi "task"              Headless one-shot

Commands:
  login          Save an Upstage API key
  logout         Remove the saved key
  models         List Solar chat models
  doctor         Check local setup
  init           Write GOPPI.md project instructions
  inspect        Show resolved config
  sessions       List and manage sessions
  export [id]    Export a session as Markdown
  version        Print version

Flags:
  -p, --prompt string     Headless prompt
  -m, --model string      solar-pro4 | solar-pro3 | solar-pro2 | solar-mini
      --effort string     none | minimal | low | medium | high | xhigh | max
  -C, --cwd string        Working directory
  -c, --continue          Resume last session
  -r, --resume id         Resume a session by id
      --max-turns int     Max agent turns
      --output-format     plain | json
      --always-approve    Skip permission prompts (alias: --yolo)

API key: goppi login  or  UPSTAGE_API_KEY
Console: %s
`, upstage.ConsoleURL)
}
