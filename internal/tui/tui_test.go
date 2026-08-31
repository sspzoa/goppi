package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
)

func TestBlocksFromMessages(t *testing.T) {
	blocks := blocksFromMessages([]provider.Message{
		{Role: provider.RoleUser, Content: "안녕"},
		{Role: provider.RoleAssistant, Content: "반가워", Reasoning: "생각"},
	})
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks", len(blocks))
	}
	if blocks[0].kind != kindUser || blocks[1].kind != kindReason || blocks[2].kind != kindAssistant {
		t.Fatalf("unexpected kinds: %+v", blocks)
	}
}

func TestWelcomeHasBrand(t *testing.T) {
	st := newStyles()
	out := renderWelcome(st, 60, 12, "solar-pro3", "medium", "~/work")
	for _, want := range []string{"고삐", "한국형 하네스", "solar-pro3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q\n%s", want, out)
		}
	}
}

func TestRenderUserBlockGutter(t *testing.T) {
	st := newStyles()
	out := renderBlock(st, block{kind: kindUser, body: "hello"}, 40, "", false)
	if !strings.Contains(out, "❯") || !strings.Contains(out, "hello") {
		t.Fatalf("bad user block:\n%s", out)
	}
}

func TestRenderReasonCollapsed(t *testing.T) {
	st := newStyles()
	out := renderBlock(st, block{kind: kindReason, body: "아주 긴 생각의 내용입니다"}, 60, "", false)
	if !strings.Contains(out, "생각") || !strings.Contains(out, "ctrl+o") {
		t.Fatalf("collapsed reason missing summary:\n%s", out)
	}
	if strings.Contains(out, "아주 긴 생각의 내용입니다") {
		t.Fatalf("collapsed reason leaked body:\n%s", out)
	}
	expanded := renderBlock(st, block{kind: kindReason, body: "아주 긴 생각의 내용입니다"}, 60, "", true)
	if !strings.Contains(expanded, "아주 긴 생각의 내용입니다") {
		t.Fatalf("expanded reason missing body:\n%s", expanded)
	}
}

func TestRenderToolLine(t *testing.T) {
	st := newStyles()
	out := renderBlock(st, block{kind: kindTool, title: "bash", body: "go test ./...", note: "ok", state: "ok"}, 60, "", false)
	if !strings.Contains(out, "✓") || !strings.Contains(out, "bash") || !strings.Contains(out, "go test") {
		t.Fatalf("bad tool block:\n%s", out)
	}
	if strings.Contains(out, "╭") {
		t.Fatalf("tool block still boxed:\n%s", out)
	}
}

func TestSuggestRendersModel(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.width, m.height = 80, 24
	m.input.SetValue("/mo")
	m.syncSuggest()
	out := m.View().Content
	if !strings.Contains(out, "/model") {
		t.Fatalf("missing /model suggestion\n%s", out)
	}
}

func TestViewHasChrome(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.width, m.height = 80, 24
	out := m.View().Content
	for _, want := range []string{"고삐", "한국형", "enter", "tab"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "UPSTAGE SOLAR") {
		t.Fatalf("view still has exclusive brand\n%s", out)
	}
}

func TestModelPickViaSuggest(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.width, m.height = 80, 24
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}

	m.input.SetValue("/model")
	m.syncSuggest()
	m.handleKey(enter)
	if got := m.input.Value(); got != "/model " {
		t.Fatalf("expected arg pick mode, input = %q", got)
	}
	if len(m.suggest) == 0 || m.suggest[m.suggestIdx].Name != m.agent.Cfg.Model {
		t.Fatalf("current model not preselected: idx=%d suggest=%+v", m.suggestIdx, m.suggest)
	}

	m.handleKey(enter)
	if got := m.input.Value(); got != "" {
		t.Fatalf("pick should submit in one enter, input = %q", got)
	}
	last := m.blocks[len(m.blocks)-1]
	if !strings.Contains(last.body, "model →") {
		t.Fatalf("model not applied: %+v", last)
	}
}

func TestCtrlCOpensQuit(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.width, m.height = 80, 24
	m.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if m.overlay != overlayQuit {
		t.Fatalf("ctrl+c should open quit, overlay=%d", m.overlay)
	}
}

func TestCtrlCMatchesWhenTextIsC(t *testing.T) {
	// Kitty-style events set Text, so String() is "c" not "ctrl+c".
	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl, Text: "c"}
	if msg.String() == "ctrl+c" {
		t.Fatal("precondition: String() should not be ctrl+c when Text is set")
	}
	if !isCtrlC(msg) {
		t.Fatalf("isCtrlC missed Text=c event; keystroke=%q string=%q", msg.Keystroke(), msg.String())
	}
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.handleKey(msg)
	if m.overlay != overlayQuit {
		t.Fatalf("printable ctrl+c should still quit-confirm, overlay=%d", m.overlay)
	}
}

func TestKeepCtrlCRewritesInterrupt(t *testing.T) {
	msg := keepCtrlC(nil, tea.InterruptMsg{})
	got, ok := msg.(tea.KeyPressMsg)
	if !ok || !isCtrlC(got) {
		t.Fatalf("InterruptMsg should become ctrl+c, got %#v", msg)
	}
}

func TestCtrlCCancelsTurn(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	canceled := false
	m.busy = true
	m.turnCancel = func() { canceled = true }
	m.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !canceled {
		t.Fatal("ctrl+c should cancel the running turn")
	}
	if m.overlay != overlayNone {
		t.Fatalf("busy ctrl+c should not open quit, overlay=%d", m.overlay)
	}
}

func TestSecondCtrlCQuits(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.overlay = overlayQuit
	_, cmd := m.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("second ctrl+c should quit")
	}
}

func TestPermPanelReplacesInput(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.width, m.height = 80, 24
	m.overlay = overlayPerm
	m.perm = &permAskMsg{name: "bash", detail: "rm -rf /tmp/x"}
	out := m.View().Content
	if !strings.Contains(out, "허용할까요") || !strings.Contains(out, "bash") {
		t.Fatalf("perm panel missing:\n%s", out)
	}
}
