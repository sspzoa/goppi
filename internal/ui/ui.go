package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func Out() io.Writer { return os.Stdout }
func Err() io.Writer { return os.Stderr }

func Banner(version, model, effort, workdir string) {
	title := "고삐"
	tag := "한국형"
	meta := "v" + version
	rows := [][2]string{
		{"model", model},
		{"effort", effortLabel(effort, model)},
		{"workdir", shortPath(workdir)},
	}

	inner := visibleLen(title) + 2 + visibleLen(tag) + 1 + visibleLen(meta)
	for _, row := range rows {
		inner = max(inner, 9+visibleLen(row[1]))
	}
	if inner < 44 {
		inner = 44
	}
	if inner > 72 {
		inner = 72
	}

	top := "╭" + strings.Repeat("─", inner+2) + "╮"
	bot := "╰" + strings.Repeat("─", inner+2) + "╯"
	mid := "├" + strings.Repeat("─", inner+2) + "┤"

	fmt.Fprintln(Out(), Paint(Violet(), top))
	left := title + "  " + tag
	gap := inner - visibleLen(left) - visibleLen(meta)
	if gap < 1 {
		gap = 1
	}
	fmt.Fprintf(Out(), "%s %s%s%s %s\n",
		Paint(Violet(), "│"),
		brand(title)+"  "+soft(tag),
		pad(gap),
		mute(meta),
		Paint(Violet(), "│"),
	)
	fmt.Fprintln(Out(), Paint(Violet(), mid))
	for _, row := range rows {
		label := mute(fmt.Sprintf("%-7s", row[0]))
		value := row[1]
		if utf8.RuneCountInString(value) > inner-9 {
			value = trimRunes(value, inner-10) + "…"
		}
		fmt.Fprintf(Out(), "%s %s  %s%s %s\n",
			Paint(Violet(), "│"),
			label,
			value,
			pad(inner-9-visibleLen(value)),
			Paint(Violet(), "│"),
		)
	}
	fmt.Fprintln(Out(), Paint(Violet(), bot))
	fmt.Fprintln(Out())
}

func effortLabel(effort, model string) string {
	if model == "solar-mini" {
		return "n/a · solar-mini"
	}
	if effort == "" {
		return "medium"
	}
	return effort
}

func Hint(format string, args ...any) {
	fmt.Fprintf(Out(), "  %s\n\n", mute(fmt.Sprintf(format, args...)))
}

func Reasoning(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	fmt.Fprintf(Out(), "\n  %s\n  %s\n", soft("생각"), mute(text))
}

type Stream struct {
	phase string
}

func NewStream() *Stream { return &Stream{} }

func (s *Stream) Write(reasoning, content string) {
	if reasoning != "" {
		if s.phase == "" {
			fmt.Fprintf(Out(), "\n  %s\n  ", soft("생각"))
			if colorEnabled() {
				fmt.Fprint(Out(), Mute())
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
	parts := []string{fmt.Sprintf("%d in", in), fmt.Sprintf("%d out", out)}
	if reason > 0 {
		parts = append(parts, fmt.Sprintf("%d reason", reason))
	}
	fmt.Fprintf(Out(), "  %s %s\n", soft("·"), mute(strings.Join(parts, "  ·  ")))
}

func UserPrompt() {
	fmt.Fprint(Out(), brand("› "))
}

func Assistant(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	fmt.Fprintf(Out(), "\n%s\n\n", text)
}

func ToolCall(name, detail string) {
	fmt.Fprintf(Out(), "  %s %s\n", brand("▸"), Paint(Bold, name))
	if strings.TrimSpace(detail) != "" {
		for _, line := range strings.Split(strings.TrimRight(detail, "\n"), "\n") {
			fmt.Fprintf(Out(), "    %s\n", mute(line))
		}
	}
}

func ToolOK(summary string) {
	if strings.TrimSpace(summary) == "" {
		return
	}
	fmt.Fprintf(Out(), "    %s %s\n", Paint(OK(), "✓"), mute(summary))
}

func ToolFail(err error) {
	fmt.Fprintf(Out(), "    %s %s\n", Paint(ErrC(), "✗"), err)
}

func Warn(format string, args ...any) {
	fmt.Fprintf(Err(), "  %s %s\n", Paint(WarnC(), "!"), fmt.Sprintf(format, args...))
}

func Error(format string, args ...any) {
	fmt.Fprintf(Err(), "  %s %s\n", Paint(ErrC(), "error"), fmt.Sprintf(format, args...))
}

func Info(format string, args ...any) {
	fmt.Fprintf(Out(), "  %s\n", mute(fmt.Sprintf(format, args...)))
}

func ModelRow(current bool, id, summary string) {
	mark := mute("○")
	name := id
	if current {
		mark = brand("●")
		name = brand(id)
	}
	fmt.Fprintf(Out(), "  %s %s  %s\n", mark, name, mute(summary))
}

func Help() {
	fmt.Fprintln(Out(), mute("  명령"))
	rows := [][2]string{
		{"/help", "이 도움말 · tab 완성"},
		{"/model [name]", "solar-pro4 · pro3 · pro2 · mini"},
		{"/effort [level]", "none · low · medium · high · max"},
		{"/new", "세션 초기화"},
		{"/tools", "등록된 툴"},
		{"/sessions", "최근 세션"},
		{"/status", "현재 설정"},
		{"/quit", "종료"},
	}
	for _, row := range rows {
		fmt.Fprintf(Out(), "  %s  %s\n", brand(fmt.Sprintf("%-16s", row[0])), mute(row[1]))
	}
	fmt.Fprintf(Out(), "\n  %s\n", mute("문서(PDF, HWP, 이미지)는 document_parse / document_ocr"))
}

func ShortPath(p string) string { return shortPath(p) }

func shortPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return filepath.Clean(p)
}

func visibleLen(s string) int {
	return utf8.RuneCountInString(s)
}

func pad(n int) string {
	if n < 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

func trimRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
