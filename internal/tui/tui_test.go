package tui

import (
	"context"
	"strings"
	"testing"

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

func TestRenderEmptyHasBrand(t *testing.T) {
	st := newStyles()
	out := renderBlocks(st, nil, 60)
	for _, want := range []string{"goppi", "UPSTAGE SOLAR"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q\n%s", want, out)
		}
	}
}

func TestRenderUserBlock(t *testing.T) {
	st := newStyles()
	out := renderBlock(st, block{kind: kindUser, body: "hello"}, 40)
	if !strings.Contains(out, "you") || !strings.Contains(out, "hello") {
		t.Fatalf("bad user block:\n%s", out)
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
	for _, want := range []string{"goppi", "UPSTAGE SOLAR", "enter", "tab"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q\n%s", want, out)
		}
	}
}
