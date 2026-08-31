package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/tools"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("GOPPI_NOTIFY", "off")
	os.Exit(m.Run())
}

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

func TestBlocksFromMessagesRedactsSecrets(t *testing.T) {
	const secret = "sk-abcdefghijklmnopqrst"
	blocks := blocksFromMessages([]provider.Message{
		{Role: provider.RoleUser, Content: "key " + secret},
		{Role: provider.RoleAssistant, Content: "use " + secret, Reasoning: "think " + secret, ToolCalls: []provider.ToolCall{{
			Name:  "bash",
			Input: []byte(`{"command":"echo ` + secret + `"}`),
		}}},
	})
	for _, bl := range blocks {
		if strings.Contains(bl.body, secret) || strings.Contains(bl.note, secret) {
			t.Fatalf("leaked %+v", bl)
		}
	}
}

func TestWelcomeHasBrand(t *testing.T) {
	st := newStyles()
	out := renderWelcome(st, 60, 12, "solar-pro3", "medium", "~/work")
	for _, want := range []string{"고삐", "solar-pro3"} {
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
	for _, want := range []string{"고삐", "enter", "tab"} {
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

func TestPermCtrlCDeniesAndCancelsTurn(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	canceled := false
	denied := make(chan tools.Verdict, 1)
	m.busy = true
	m.turnCancel = func() { canceled = true }
	m.overlay = overlayPerm
	m.perm = &permAskMsg{name: "bash", detail: "rm", reply: denied}
	m.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	select {
	case v := <-denied:
		if v != tools.Denied {
			t.Fatal("ctrl+c on perm should deny")
		}
	default:
		t.Fatal("perm reply not sent")
	}
	if !canceled {
		t.Fatal("ctrl+c on perm should cancel the turn")
	}
}

func TestPermAAllowsSession(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	reply := make(chan tools.Verdict, 1)
	m.overlay = overlayPerm
	m.perm = &permAskMsg{name: "bash", detail: "ls", reply: reply}
	m.handleKey(tea.KeyPressMsg{Code: 'a'})
	select {
	case v := <-reply:
		if v != tools.AllowedSession {
			t.Fatalf("got %v", v)
		}
	default:
		t.Fatal("perm reply not sent")
	}
	if m.overlay != overlayNone {
		t.Fatalf("overlay %d", m.overlay)
	}
}

func TestAskOverlayPicksOption(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	reply := make(chan askUserReply, 1)
	m.busy = true
	m.overlay = overlayAsk
	m.askq = &askUserMsg{question: "어느 쪽?", options: []string{"A", "B"}, reply: reply}
	m.handleKey(tea.KeyPressMsg{Code: '2'})
	select {
	case r := <-reply:
		if r.err != nil || r.text != "B" {
			t.Fatalf("%+v", r)
		}
	default:
		t.Fatal("ask reply not sent")
	}
	if m.overlay != overlayNone {
		t.Fatalf("overlay %d", m.overlay)
	}
}

func TestQueueWhileBusy(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.busy = true
	m.input.SetValue("다음에 이거")
	m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.queue) != 1 || m.queue[0] != "다음에 이거" {
		t.Fatalf("queue %+v", m.queue)
	}
	if last := m.blocks[len(m.blocks)-1]; last.kind != kindSystem || !strings.Contains(last.body, "대기") {
		t.Fatalf("want queue notice, got %+v", last)
	}
}

func TestQueueDrainsAfterDone(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.queue = []string{"이어서"}
	cmd := m.finishTurn(doneMsg{})
	if cmd == nil {
		t.Fatal("should start queued turn")
	}
	if !m.busy {
		t.Fatal("kick should mark busy")
	}
	if len(m.queue) != 0 {
		t.Fatalf("leftover %v", m.queue)
	}
	last := m.blocks[len(m.blocks)-1]
	if last.kind != kindUser || last.body != "이어서" {
		t.Fatalf("%+v", last)
	}
}

func TestCancelDropsQueue(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.queue = []string{"a", "b"}
	if cmd := m.finishTurn(doneMsg{err: context.Canceled}); cmd != nil {
		t.Fatal("cancel should not start next")
	}
	if len(m.queue) != 0 {
		t.Fatalf("queue %+v", m.queue)
	}
}

func TestQueueFull(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.busy = true
	for i := 0; i < maxQueuedPrompts; i++ {
		m.enqueue("x")
	}
	m.enqueue("overflow")
	if len(m.queue) != maxQueuedPrompts {
		t.Fatalf("got %d", len(m.queue))
	}
}

func TestLoadSessionByPrefix(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "old turn"},
		{Role: provider.RoleAssistant, Content: "reply"},
	})
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{Cfg: cfg, SessionID: session.NewID()}
	m := newModel(context.Background(), a)
	m.loadSession(id[:8])
	if m.agent.SessionID != id {
		t.Fatalf("id %s want %s", m.agent.SessionID, id)
	}
	if last := m.blocks[len(m.blocks)-1]; !strings.Contains(last.body, "이었습니다") {
		t.Fatalf("%q", last.body)
	}
}

func TestSlashRetry(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{
		Cfg: config.Default(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "try again"},
			{Role: provider.RoleAssistant, Content: "nope"},
		},
	})
	cmd := m.runSlash("/retry")
	if cmd == nil {
		t.Fatal("retry should start a turn")
	}
	if len(m.agent.Messages) != 0 {
		t.Fatalf("should rewind before run: %+v", m.agent.Messages)
	}
	if last := m.blocks[len(m.blocks)-1]; last.body != "다시 보냅니다." {
		t.Fatalf("%q", last.body)
	}
}

func TestNewModelSeedsSessionTotals(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{
		Cfg:        config.Default(),
		TotalUsage: provider.Usage{InputTokens: 40, OutputTokens: 8, ReasoningTokens: 2},
	})
	if m.inTok != 40 || m.outTok != 8 || m.reasonTok != 2 {
		t.Fatalf("in=%d out=%d r=%d", m.inTok, m.outTok, m.reasonTok)
	}
}

func TestSlashStatusShowsUsage(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{
		Cfg:       config.Default(),
		LastUsage: provider.Usage{InputTokens: 12, OutputTokens: 3, ReasoningTokens: 1},
	})
	m.inTok, m.outTok = 40, 8
	m.runSlash("/status")
	last := m.blocks[len(m.blocks)-1]
	if !strings.Contains(last.body, "last 12→3 r1") || !strings.Contains(last.body, "Σ 40→8") {
		t.Fatalf("%q", last.body)
	}
}

func TestSlashCopy(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{
		Cfg: config.Default(),
		Messages: []provider.Message{
			{Role: provider.RoleAssistant, Content: "copy me"},
		},
	})
	m.runSlash("/copy")
	last := m.blocks[len(m.blocks)-1]
	if !strings.Contains(last.body, "클립보드") {
		t.Fatalf("%q", last.body)
	}
	m = newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.runSlash("/copy")
	last = m.blocks[len(m.blocks)-1]
	if !strings.Contains(last.body, "복사할 답") {
		t.Fatalf("%q", last.body)
	}
}

func TestSlashExport(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	m := newModel(context.Background(), &agent.Agent{
		Cfg: config.Default(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "tui export"},
		},
	})
	m.runSlash("/export")
	last := m.blocks[len(m.blocks)-1]
	if !strings.Contains(last.body, "내보냄") {
		t.Fatalf("%q", last.body)
	}
}

func TestSlashDiff(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.runSlash("/diff")
	last := m.blocks[len(m.blocks)-1]
	if !strings.Contains(last.body, "no edits") {
		t.Fatalf("%q", last.body)
	}
}

func TestSlashDeleteCurrent(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	id, err := session.Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "drop tui"}})
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{
		Cfg:       cfg,
		SessionID: id,
		Messages:  []provider.Message{{Role: provider.RoleUser, Content: "drop tui"}},
		Tools:     tools.New(cfg.WorkDir, nil, nil),
	}
	m := newModel(context.Background(), a)
	m.runSlash("/delete")
	if a.SessionID == id {
		t.Fatal("expected a new session id")
	}
	if _, err := session.Load(id); err == nil {
		t.Fatal("deleted session still on disk")
	}
	last := m.blocks[len(m.blocks)-1]
	if !strings.Contains(last.body, "지웠습니다") {
		t.Fatalf("%q", last.body)
	}
}

func TestSlashJobs(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.runSlash("/jobs")
	last := m.blocks[len(m.blocks)-1]
	if !strings.Contains(last.body, "no jobs") {
		t.Fatalf("%q", last.body)
	}
}

func TestSlashDeniedWhileBusy(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.busy = true
	m.runSlash("/new")
	last := m.blocks[len(m.blocks)-1]
	if !strings.Contains(last.body, "턴이 끝난") {
		t.Fatalf("%q", last.body)
	}
	m.runSlash("/jobs")
	last = m.blocks[len(m.blocks)-1]
	if !strings.Contains(last.body, "no jobs") {
		t.Fatalf("jobs should run while busy: %q", last.body)
	}
	m.runSlash("/export")
	last = m.blocks[len(m.blocks)-1]
	if strings.Contains(last.body, "턴이 끝난") {
		t.Fatalf("export should run while busy: %q", last.body)
	}
}

func TestFinishWaitsForInFlightTurn(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	a := &agent.Agent{Cfg: config.Default()}
	m := newModel(context.Background(), a)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	m.turnDone = done
	m.busy = true
	m.turnCancel = func() { close(release) }
	go func() {
		close(started)
		<-release
		a.Messages = append(a.Messages, provider.Message{Role: provider.RoleUser, Content: "late keep"})
		close(done)
	}()
	<-started
	if err := m.finish(); err != nil {
		t.Fatal(err)
	}
	last, err := session.LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if last.Title != "late keep" {
		t.Fatalf("finish raced persist, title %q", last.Title)
	}
}

func TestFinishPersistsCurrent(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	a := &agent.Agent{
		Cfg: config.Default(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "term keep"},
		},
	}
	m := newModel(context.Background(), a)
	if err := m.finish(); err != nil {
		t.Fatal(err)
	}
	last, err := session.LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if last.Title != "term keep" {
		t.Fatalf("title %q", last.Title)
	}
}

func TestQuitPersistsCurrent(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	a := &agent.Agent{
		Cfg: config.Default(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "quit keep"},
		},
	}
	m := newModel(context.Background(), a)
	_ = m.quit()
	last, err := session.LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if last.Title != "quit keep" {
		t.Fatalf("title %q", last.Title)
	}
}

func TestQuitReportsPersistFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	a := &agent.Agent{
		Cfg: config.Default(),
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "quit fail"},
		},
	}
	m := newModel(context.Background(), a)
	dir := filepath.Join(root, "sessions")
	if err := os.WriteFile(dir, []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = m.quit()
	if len(m.blocks) == 0 || m.blocks[len(m.blocks)-1].kind != kindError {
		t.Fatalf("blocks %#v", m.blocks)
	}
	if !strings.Contains(m.blocks[len(m.blocks)-1].body, "session save") {
		t.Fatalf("body %q", m.blocks[len(m.blocks)-1].body)
	}
	if err := m.finish(); err == nil || !strings.Contains(err.Error(), "session save") {
		t.Fatalf("finish %v", err)
	}
}

func TestResetSessionPersistsFirst(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	a := &agent.Agent{
		Cfg: cfg,
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "tui keep"},
		},
	}
	m := newModel(context.Background(), a)
	m.resetSession()
	last, err := session.LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if last.Title != "tui keep" {
		t.Fatalf("title %q", last.Title)
	}
	if a.SessionID == last.ID {
		t.Fatal("reset should use a new id")
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

func TestPermPanelShowsWritePreview(t *testing.T) {
	m := newModel(context.Background(), &agent.Agent{Cfg: config.Default()})
	m.width, m.height = 80, 24
	m.overlay = overlayPerm
	m.perm = &permAskMsg{name: "write_file", detail: "a.go\nnew file\n+ package a"}
	out := m.View().Content
	if !strings.Contains(out, "write_file") || !strings.Contains(out, "package a") {
		t.Fatalf("preview missing:\n%s", out)
	}
}
