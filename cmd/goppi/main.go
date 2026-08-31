package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sspzoa/goppi/internal/complete"
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
	if cmd, rest, ok := findDispatch(args); ok {
		return dispatchCommand(cmd, rest)
	}
	if !isDispatched(args[0]) {
		return cmdRun(args)
	}
	return dispatchCommand(args[0], args[1:])
}

func dispatchCommand(cmd string, args []string) error {
	switch cmd {
	case "help", "-h", "--help":
		printHelp()
		return nil
	case "version", "--version", "-v":
		return cmdVersion()
	case "login":
		return cmdLogin(args)
	case "logout":
		return cmdLogout()
	case "models":
		return cmdModels()
	case "doctor":
		return cmdDoctor(args)
	case "init":
		return cmdInit(args)
	case "inspect":
		return cmdInspect(args)
	case "sessions":
		return cmdSessions(args)
	case "export":
		return cmdExport(args)
	case "completions":
		return cmdCompletions(args)
	case "complete":
		return cmdComplete(args)
	case "mcp":
		return cmdMCP(args)
	case "worktree":
		return cmdWorktree(args)
	case "acp":
		return cmdACP()
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func isDispatched(name string) bool {
	for _, d := range dispatchedCommands {
		if d == name {
			return true
		}
	}
	return false
}

func isLeadingGlobalFlag(flag string) bool {
	switch flag {
	case "-C", "--cwd":
		return true
	}
	return false
}

// findDispatch returns a CLI subcommand when only leading -C/--cwd precede it
// (e.g. goppi -C /tmp init). Any other flag means run mode (e.g. goppi -p hi init).
func findDispatch(args []string) (cmd string, rest []string, ok bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			if isLeadingGlobalFlag(a) {
				i++
				continue
			}
			return "", args, false
		}
		if isDispatched(a) {
			rest = append(append([]string{}, args[:i]...), args[i+1:]...)
			return a, rest, true
		}
		return "", args, false
	}
	return "", args, false
}

func printHelp() {
	var cmds strings.Builder
	for _, c := range complete.CLICommands() {
		fmt.Fprintf(&cmds, "  %-14s %s\n", c.Name, c.Summary)
	}
	fmt.Fprintf(os.Stderr, `고삐 — 한국형 에이전트 하네스

사용:
  goppi                     풀스크린 TUI
  goppi -p "할 일"           헤드리스 원샷
  goppi "할 일"              헤드리스 원샷

커맨드:
%s
플래그:
  -p, --prompt string     헤드리스 프롬프트
  -m, --model string      solar-pro4 | solar-pro3 | solar-pro2 | solar-mini
      --effort string     none | minimal | low | medium | high | xhigh | max
  -C, --cwd string        작업 디렉터리
  -c, --continue          마지막 세션 이어가기
  -r, --resume id         세션 id로 이어가기
      --mode              act | plan
      --provider          upstage | openai | compat
      --max-turns int     에이전트 턴 상한
      --output-format     plain | json
      --always-approve    권한 확인 생략 (별칭: --yolo)
      --sandbox           workspace | strict | off
      --worktree          메인 체크아웃을 건드리지 않는 git worktree

API 키: goppi login  또는  UPSTAGE_API_KEY
콘솔: %s
`, cmds.String(), upstage.ConsoleURL)
}

// dispatchedCommands is every name run() handles besides the default agent.
// Keep in sync with the switch in run; TestCLICommandsMatchDispatch checks both sides.
var dispatchedCommands = []string{
	"help", "version", "login", "logout", "models", "doctor", "init",
	"inspect", "sessions", "export", "completions", "complete", "mcp",
	"worktree", "acp",
}
