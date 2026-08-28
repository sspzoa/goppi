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

func Banner(version, model, effort, workdir string) {
	fmt.Fprintf(Out(), "%s\n", Paint(Bold+Magenta, "고삐 goppi  "+version))
	fmt.Fprintf(Out(), "  %s  upstage / %s\n", Paint(Dim, "solar  "), model)
	fmt.Fprintf(Out(), "  %s  %s\n", Paint(Dim, "effort "), effortLabel(effort, model))
	fmt.Fprintf(Out(), "  %s  %s\n", Paint(Dim, "workdir"), workdir)
	fmt.Fprintln(Out())
}

func effortLabel(effort, model string) string {
	if model == "solar-mini" {
		return "n/a (solar-mini)"
	}
	if effort == "" {
		return "default (solar-pro4는 on)"
	}
	return effort
}

func Reasoning(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	fmt.Fprintf(Out(), "\n%s\n  %s\n", Paint(Dim, "reasoning"), Paint(Dim, text))
}

type Stream struct {
	phase string
}

func NewStream() *Stream { return &Stream{} }

func (s *Stream) Write(reasoning, content string) {
	if reasoning != "" {
		if s.phase == "" {
			fmt.Fprintf(Out(), "\n%s\n", Paint(Dim, "reasoning"))
			if colorEnabled() {
				fmt.Fprint(Out(), Dim)
			}
			s.phase = "reason"
		}
		fmt.Fprint(Out(), reasoning)
	}
	if content != "" {
		if s.phase == "reason" {
			if colorEnabled() {
				fmt.Fprint(Out(), Reset)
			}
			fmt.Fprint(Out(), "\n\n")
		} else if s.phase == "" {
			fmt.Fprint(Out(), "\n")
		}
		s.phase = "content"
		fmt.Fprint(Out(), content)
	}
}

func (s *Stream) Close() {
	if s.phase == "reason" && colorEnabled() {
		fmt.Fprint(Out(), Reset)
	}
	if s.phase != "" {
		fmt.Fprint(Out(), "\n\n")
	}
}

func Usage(in, out, reason int) {
	if in == 0 && out == 0 {
		return
	}
	msg := fmt.Sprintf("%d in · %d out", in, out)
	if reason > 0 {
		msg += fmt.Sprintf(" · %d reason", reason)
	}
	fmt.Fprintf(Out(), "%s\n", Paint(Dim, msg))
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
  /help                 이 도움말
  /model [name]         solar-pro4 | solar-pro3 | solar-pro2 | solar-mini
  /effort [level]       none | minimal | low | medium | high | xhigh | max
  /new                  세션 초기화
  /tools                등록된 툴
  /quit                 종료

문서(PDF, HWP, 이미지)는 document_parse / document_ocr 툴이 처리합니다.
`)
}
