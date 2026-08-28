package tui

import (
	"fmt"
	"strings"

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
	title string
	body  string
	state string
	live  bool
}

func blocksFromMessages(msgs []provider.Message) []block {
	var out []block
	for _, msg := range msgs {
		switch msg.Role {
		case provider.RoleUser:
			if strings.TrimSpace(msg.Content) != "" {
				out = append(out, block{kind: kindUser, body: msg.Content})
			}
		case provider.RoleAssistant:
			if strings.TrimSpace(msg.Reasoning) != "" {
				out = append(out, block{kind: kindReason, body: msg.Reasoning})
			}
			if strings.TrimSpace(msg.Content) != "" {
				out = append(out, block{kind: kindAssistant, body: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				out = append(out, block{
					kind:  kindTool,
					title: tc.Name,
					body:  tools.Detail(tc.Name, tc.Input),
					state: "ok",
				})
			}
		}
	}
	return out
}

func renderBlocks(st styles, blocks []block, width int) string {
	if width < 20 {
		width = 20
	}
	if len(blocks) == 0 {
		return renderEmpty(st, width)
	}
	var b strings.Builder
	for i, bl := range blocks {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(renderBlock(st, bl, width))
	}
	return b.String()
}

func renderEmpty(st styles, width int) string {
	lines := []string{
		st.brand.Render("고삐"),
		st.tag.Render("에이전트 하네스"),
		"",
		st.mute.Render("메시지를 입력하고 enter"),
		st.mute.Render("/help  ·  /model  ·  ? 단축키"),
	}
	inner := strings.Join(lines, "\n")
	return lipgloss.Place(width, 10, lipgloss.Center, lipgloss.Center, inner)
}

func renderBlock(st styles, bl block, width int) string {
	bodyWidth := width
	if bodyWidth < 8 {
		bodyWidth = 8
	}
	switch bl.kind {
	case kindUser:
		return st.user.Render("나") + "\n" + wrapBody(st.text, bl.body, bodyWidth)
	case kindAssistant:
		return st.assistant.Render("고삐") + "\n" + wrapBody(st.text, bl.body, bodyWidth)
	case kindReason:
		label := "생각"
		if bl.live {
			label += " …"
		}
		return st.mute.Render(label) + "\n" + wrapBody(st.reason, bl.body, bodyWidth)
	case kindTool:
		return renderTool(st, bl, bodyWidth)
	case kindSystem:
		return st.mute.Render("· " + bl.body)
	case kindError:
		return st.err.Render("error") + "  " + st.mute.Render(bl.body)
	default:
		return wrapBody(st.text, bl.body, bodyWidth)
	}
}

func renderTool(st styles, bl block, width int) string {
	mark := st.mute.Render("·")
	state := bl.state
	switch state {
	case "running":
		mark = st.tag.Render("▸")
	case "ok":
		mark = st.ok.Render("✓")
	case "fail":
		mark = st.err.Render("✗")
		state = "fail"
	default:
		state = "ok"
	}
	head := fmt.Sprintf("%s %s  %s", mark, st.brand.Render(bl.title), st.mute.Render(state))
	inner := head
	if strings.TrimSpace(bl.body) != "" {
		inner += "\n" + wrapBody(st.mute, bl.body, max(width-4, 8))
	}
	return st.card.Width(width).Render(inner)
}

func wrapBody(style lipgloss.Style, text string, width int) string {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return ""
	}
	return style.Width(width).Render(text)
}
