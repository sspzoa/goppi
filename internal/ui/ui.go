package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
)

func Out() io.Writer { return os.Stdout }
func Err() io.Writer { return os.Stderr }

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func Paint(code, s string) string {
	if !colorEnabled() {
		return s
	}
	return code + s + Reset
}

func Banner(version, provider, model, workdir string) {
	fmt.Fprintf(Out(), "%s\n", Paint(Bold+Magenta, "고삐 goppi  "+version))
	fmt.Fprintf(Out(), "  %s  %s\n", Paint(Dim, "provider"), provider)
	fmt.Fprintf(Out(), "  %s  %s\n", Paint(Dim, "model   "), model)
	fmt.Fprintf(Out(), "  %s  %s\n", Paint(Dim, "workdir "), workdir)
	fmt.Fprintln(Out())
}

func UserPrompt() {
	fmt.Fprint(Out(), Paint(Bold+Cyan, "› "))
}

func Assistant(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	fmt.Fprintf(Out(), "\n%s\n\n", text)
}

func ToolCall(name, detail string) {
	fmt.Fprintf(Out(), "%s %s\n", Paint(Yellow, "⚙"), Paint(Bold, name))
	if strings.TrimSpace(detail) != "" {
		for _, line := range strings.Split(strings.TrimRight(detail, "\n"), "\n") {
			fmt.Fprintf(Out(), "  %s\n", Paint(Dim, line))
		}
	}
}

func ToolOK(summary string) {
	if strings.TrimSpace(summary) == "" {
		return
	}
	fmt.Fprintf(Out(), "  %s %s\n", Paint(Green, "✓"), Paint(Dim, summary))
}

func ToolFail(err error) {
	fmt.Fprintf(Out(), "  %s %s\n", Paint(Red, "✗"), err)
}

func Warn(format string, args ...any) {
	fmt.Fprintf(Err(), "%s %s\n", Paint(Yellow, "!"), fmt.Sprintf(format, args...))
}

func Error(format string, args ...any) {
	fmt.Fprintf(Err(), "%s %s\n", Paint(Red, "error"), fmt.Sprintf(format, args...))
}

func Info(format string, args ...any) {
	fmt.Fprintf(Out(), "%s\n", Paint(Dim, fmt.Sprintf(format, args...)))
}

func Help() {
	fmt.Fprint(Out(), `명령
  /help              이 도움말
  /model [name]      모델 보기 / 바꾸기
  /provider [name]   anthropic | openai
  /new               세션 초기화
  /tools             등록된 툴
  /quit              종료

그냥 메시지를 입력하면 에이전트가 툴을 쓰며 일합니다.
`)
}
