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
	case "completions":
		return cmdCompletions(args[1:])
	case "complete":
		return cmdComplete(args[1:])
	default:
		return cmdRun(args)
	}
}

func printHelp() {
	fmt.Fprintf(os.Stderr, `고삐 — 한국형 에이전트 하네스

사용:
  goppi                     풀스크린 TUI
  goppi -p "할 일"           헤드리스 원샷
  goppi "할 일"              헤드리스 원샷

커맨드:
  login          API 키 저장
  logout         저장된 키 삭제
  models         채팅 모델 목록
  doctor         로컬 설정 확인
  init           GOPPI.md 프로젝트 지시 작성
  inspect        해석된 설정 보기
  sessions       세션 목록·관리
  export [id]    세션을 Markdown으로 내보내기
  completions    셸 자동완성 스크립트
  version        버전 출력

플래그:
  -p, --prompt string     헤드리스 프롬프트
  -m, --model string      solar-pro4 | solar-pro3 | solar-pro2 | solar-mini
      --effort string     none | minimal | low | medium | high | xhigh | max
  -C, --cwd string        작업 디렉터리
  -c, --continue          마지막 세션 이어가기
  -r, --resume id         세션 id로 이어가기
      --max-turns int     에이전트 턴 상한
      --output-format     plain | json
      --always-approve    권한 확인 생략 (별칭: --yolo)

API 키: goppi login  또는  UPSTAGE_API_KEY
콘솔: %s
`, upstage.ConsoleURL)
}
