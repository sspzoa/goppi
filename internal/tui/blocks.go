package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/tools"
)

type kind int

const (
	kindUser kind = iota
	kindAssistant
	kindReason
	kindTool
	kindSystem
	kindError
)

type block struct {
	kind  kind
	title string // tool name
	body  string // text, or tool detail
	note  string // tool result summary / error
	state string // tool: running | ok | fail
	live  bool
}

func blocksFromMessages(msgs []provider.Message) []block {
	var out []block
	for _, msg := range msgs {
		switch msg.Role {
		case provider.RoleUser:
			if strings.TrimSpace(msg.Content) != "" {
				out = append(out, block{kind: kindUser, body: tools.RedactSecrets(msg.Content)})
			}
		case provider.RoleAssistant:
			if strings.TrimSpace(msg.Reasoning) != "" {
				out = append(out, block{kind: kindReason, body: tools.RedactSecrets(msg.Reasoning)})
			}
			if strings.TrimSpace(msg.Content) != "" {
				out = append(out, block{kind: kindAssistant, body: tools.RedactSecrets(msg.Content)})
			}
			for _, tc := range msg.ToolCalls {
				out = append(out, block{
					kind:  kindTool,
					title: tc.Name,
					body:  tools.RedactSecrets(tools.Detail(tc.Name, tc.Input)),
					state: "ok",
				})
			}
		}
	}
	return out
}

// compact kinds stack with a single newline instead of a blank line.
func compact(k kind) bool { return k == kindTool || k == kindSystem }

func renderBlocks(st styles, blocks []block, width int, spin string, showReason bool) string {
	if width < 20 {
		width = 20
	}
	var b strings.Builder
	for i, bl := range blocks {
		if i > 0 {
			if compact(blocks[i-1].kind) && compact(bl.kind) {
				b.WriteString("\n")
			} else {
				b.WriteString("\n\n")
			}
		}
		b.WriteString(renderBlock(st, bl, width, spin, showReason))
	}
	return b.String()
}

func renderWelcome(st styles, width, height int, model, effort, workdir string) string {
	if height < 8 {
		height = 8
	}
	lines := []string{
		st.brand.Render("고삐"),
		"",
		st.mute.Render(model + " · " + effort),
		st.mute.Render(workdir),
		"",
		st.text.Render("메시지를 입력하고 enter"),
		st.mute.Render("/help 명령  ·  ? 단축키"),
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, strings.Join(lines, "\n"))
}

func renderBlock(st styles, bl block, width int, spin string, showReason bool) string {
	switch bl.kind {
	case kindUser:
		return gutter(st.brand.Render("❯"), bl.body, st.text, width)
	case kindAssistant:
		return gutter(st.tag.Render("●"), bl.body, st.text, width)
	case kindReason:
		return renderReason(st, bl, width, showReason)
	case kindTool:
		return renderTool(st, bl, width, spin)
	case kindSystem:
		return gutter(st.mute.Render("·"), bl.body, st.mute, width)
	case kindError:
		return gutter(st.err.Render("✗"), bl.body, st.err, width)
	default:
		return gutter(" ", bl.body, st.text, width)
	}
}

func renderReason(st styles, bl block, width int, expanded bool) string {
	body := strings.TrimSpace(bl.body)
	if bl.live {
		head := st.mute.Render("✻ ") + st.reason.Render("생각 중…")
		tail := lastLines(body, 3)
		if tail == "" {
			return head
		}
		return head + "\n" + indentWrap(st.reason, tail, width)
	}
	if !expanded {
		n := utf8.RuneCountInString(body)
		return st.mute.Render(fmt.Sprintf("✻ 생각 · %d자 · ctrl+o 펼치기", n))
	}
	return gutter(st.mute.Render("✻"), body, st.reason, width)
}

func renderTool(st styles, bl block, width int, spin string) string {
	var mark string
	switch bl.state {
	case "running":
		mark = spin
		if mark == "" {
			mark = st.spin.Render("⠿")
		}
	case "fail":
		mark = st.err.Render("✗")
	default:
		mark = st.ok.Render("✓")
	}
	head := mark + " " + st.brand.Render(bl.title)
	if d := firstLine(bl.body); d != "" {
		room := width - lipgloss.Width(head) - 3
		if room > 8 {
			head += "  " + st.mute.Render(fit(d, room))
		}
	}
	out := head
	if note := strings.TrimSpace(bl.note); note != "" {
		out += "\n" + indentWrap(st.mute, note, width)
	}
	return out
}

// gutter renders a 1-char mark and hanging-indents the wrapped body under it.
func gutter(mark, body string, style lipgloss.Style, width int) string {
	w := max(width-2, 8)
	wrapped := style.Width(w).Render(strings.TrimRight(body, "\n"))
	lines := strings.Split(wrapped, "\n")
	var b strings.Builder
	for i, ln := range lines {
		if i == 0 {
			b.WriteString(mark + " " + ln)
		} else {
			b.WriteString("\n  " + ln)
		}
	}
	return b.String()
}

func indentWrap(style lipgloss.Style, text string, width int) string {
	w := max(width-2, 8)
	wrapped := style.Width(w).Render(strings.TrimRight(text, "\n"))
	lines := strings.Split(wrapped, "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func previewLines(s string, n int) []string {
	s = strings.TrimRight(s, "\n")
	if strings.TrimSpace(s) == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if n > 0 && len(lines) > n {
		return append(lines[:n], "…")
	}
	return lines
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
