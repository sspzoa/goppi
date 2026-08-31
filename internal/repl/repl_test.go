package repl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/tools"
)

type errClient struct{}

func (errClient) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("nope")
}

type okClient struct{}

func (okClient) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "ok"},
	}, nil
}

func TestApplySessionRestoresModelAndEffort(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	a := &agent.Agent{Cfg: config.Default()}
	if err := ApplySession(a, session.File{
		ID:       "0123456789abcdef",
		Model:    "solar-mini",
		Effort:   "high",
		CacheKey: "goppi-abc",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	}); err != nil {
		t.Fatal(err)
	}
	if a.SessionID != "0123456789abcdef" {
		t.Fatalf("id %s", a.SessionID)
	}
	if a.Cfg.Model != "solar-mini" {
		t.Fatalf("model %s", a.Cfg.Model)
	}
	if a.Cfg.ReasoningEffort != "high" {
		t.Fatalf("effort %s", a.Cfg.ReasoningEffort)
	}
	if a.LastUsage.InputTokens != 0 {
		t.Fatalf("usage should be empty without file fields")
	}
	if a.Cfg.PromptCacheKey != "goppi-abc" {
		t.Fatalf("cache %s", a.Cfg.PromptCacheKey)
	}
	if len(a.Messages) != 1 {
		t.Fatalf("messages %d", len(a.Messages))
	}
}

func TestApplySessionRestoresExtraDirs(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	extra := t.TempDir()
	reg := tools.New(wd, nil, nil)
	a := agent.New(config.Default(), errClient{}, reg)
	if err := ApplySession(a, session.File{
		ID:        "0123456789abcdef",
		ExtraDirs: []string{extra},
		Messages:  []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(a.Cfg.ExtraDirs) != 1 || a.Cfg.ExtraDirs[0] != extra {
		t.Fatalf("extra dirs %v", a.Cfg.ExtraDirs)
	}
}

func TestApplySessionRestoresUsage(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	a := &agent.Agent{Cfg: config.Default()}
	if err := ApplySession(a, session.File{
		ID:         "0123456789abcdef",
		Messages:   []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		Usage:      provider.Usage{InputTokens: 12, OutputTokens: 3},
		TotalUsage: provider.Usage{InputTokens: 40, OutputTokens: 8},
	}); err != nil {
		t.Fatal(err)
	}
	if a.LastUsage.InputTokens != 12 || a.TotalUsage.InputTokens != 40 {
		t.Fatalf("last=%+v total=%+v", a.LastUsage, a.TotalUsage)
	}
}

func TestClearSlashAliasesNew(t *testing.T) {
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	a.SessionID = "0123456789abcdef"
	a.LastUsage = provider.Usage{InputTokens: 9}
	a.TotalUsage = provider.Usage{InputTokens: 40}
	quit, err := handleSlash(a, "/clear")
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
	if a.SessionID == "0123456789abcdef" {
		t.Fatal("expected a new session id")
	}
	if a.LastUsage != (provider.Usage{}) || a.TotalUsage != (provider.Usage{}) {
		t.Fatalf("last=%+v total=%+v", a.LastUsage, a.TotalUsage)
	}
}

func TestPermissionAskJSONDenies(t *testing.T) {
	cfg := config.Default()
	cfg.OutputFormat = "json"
	ask := permissionAsk(cfg)
	if ask == nil {
		t.Fatal("json should still have an ask that denies")
	}
	if ask("bash", "rm").OK() {
		t.Fatal("json must deny dangerous tools")
	}
}

func TestRunOnceSavesAfterError(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	if err := RunOnce(context.Background(), a, "keep me"); err == nil {
		t.Fatal("expected chat error")
	}
	last, err := session.LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if last.Title != "keep me" {
		t.Fatalf("title %q", last.Title)
	}
}

func TestRunOnceReportsPersistFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, okClient{}, tools.New(cfg.WorkDir, nil, nil))
	dir := filepath.Join(root, "sessions")
	if err := os.WriteFile(dir, []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := RunOnce(context.Background(), a, "hi")
	if err == nil || !strings.Contains(err.Error(), "session save") {
		t.Fatalf("got %v", err)
	}
}

type blockUntilCancel struct {
	started chan struct{}
}

func (c blockUntilCancel) Chat(ctx context.Context, _ provider.ChatRequest) (provider.ChatResponse, error) {
	if c.started != nil {
		select {
		case c.started <- struct{}{}:
		default:
		}
	}
	<-ctx.Done()
	return provider.ChatResponse{}, ctx.Err()
}

func TestLineLoopStopsWhenParentCanceled(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	t.Setenv("GOPPI_TUI", "0")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldIn := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldIn
		_ = r.Close()
		_ = w.Close()
	})
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	started := make(chan struct{}, 1)
	a := agent.New(cfg, blockUntilCancel{started: started}, tools.New(cfg.WorkDir, nil, nil))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- lineLoop(ctx, a) }()
	if _, err := w.Write([]byte("hi\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("turn did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("lineLoop should exit when the parent context is canceled")
	}
	last, err := session.LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if last.Title != "hi" {
		t.Fatalf("canceled turn should persist, title %q", last.Title)
	}
}

func TestLineLoopReportsPersistFailureOnCancel(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("GOPPI_TUI", "0")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldIn := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldIn
		_ = r.Close()
		_ = w.Close()
	})
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	started := make(chan struct{}, 1)
	a := agent.New(cfg, blockUntilCancel{started: started}, tools.New(cfg.WorkDir, nil, nil))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- lineLoop(ctx, a) }()
	if _, err := w.Write([]byte("hi\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("turn did not start")
	}
	dir := filepath.Join(root, "sessions")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "session save") {
			t.Fatalf("got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("lineLoop should exit when the parent context is canceled")
	}
}

func TestRunLineTurnCancelDoesNotPoisonParent(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	defer stop()
	turn, cancelTurn := context.WithCancel(parent)
	cancelTurn()
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	if err := a.Run(turn, "hi"); err == nil {
		t.Fatal("expected canceled turn")
	}
	if parent.Err() != nil {
		t.Fatal("parent context must stay alive after a canceled turn")
	}
}

func TestNewPersistsCurrentSession(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	a.Messages = []provider.Message{{Role: provider.RoleUser, Content: "keep this"}}
	quit, err := handleSlash(a, "/new")
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
	last, err := session.LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if last.Title != "keep this" {
		t.Fatalf("title %q", last.Title)
	}
	if a.SessionID == last.ID {
		t.Fatal("/new should start a different session id")
	}
	if len(a.Messages) != 0 {
		t.Fatal("memory should be cleared")
	}
}

func TestQuitPersistsCurrentSession(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	a.Messages = []provider.Message{{Role: provider.RoleUser, Content: "save on quit"}}
	quit, err := handleSlash(a, "/quit")
	if !quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
	last, err := session.LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if last.Title != "save on quit" {
		t.Fatalf("title %q", last.Title)
	}
}

func TestQuitSlashReportsPersistFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, okClient{}, tools.New(cfg.WorkDir, nil, nil))
	a.Messages = []provider.Message{{Role: provider.RoleUser, Content: "quit fail"}}
	dir := filepath.Join(root, "sessions")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	quit, err := handleSlash(a, "/quit")
	if !quit || err == nil || !strings.Contains(err.Error(), "session save") {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
}

func TestLineLoopQuitReportsPersistFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	t.Setenv("GOPPI_TUI", "0")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldIn := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldIn
		_ = r.Close()
		_ = w.Close()
	})
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, okClient{}, tools.New(cfg.WorkDir, nil, nil))
	a.Messages = []provider.Message{{Role: provider.RoleUser, Content: "quit fail"}}
	dir := filepath.Join(root, "sessions")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- lineLoop(context.Background(), a) }()
	if _, err := w.Write([]byte("/quit\n")); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "session save") {
			t.Fatalf("got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("lineLoop should exit on /quit persist failure")
	}
}

func TestStatusDoesNotQuit(t *testing.T) {
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	quit, err := handleSlash(a, "/status")
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
}

func TestSessionsSlashLoadsPrefix(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	id, err := session.Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "resume me"}})
	if err != nil {
		t.Fatal(err)
	}
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	quit, err := handleSlash(a, "/sessions "+id[:8])
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
	if a.SessionID != id {
		t.Fatalf("id %s", a.SessionID)
	}
}

func TestDeleteSlashRemovesCurrent(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	id, err := session.Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "drop me"}})
	if err != nil {
		t.Fatal(err)
	}
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	a.SessionID = id
	a.Messages = []provider.Message{{Role: provider.RoleUser, Content: "drop me"}}
	quit, err := handleSlash(a, "/delete")
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
	if a.SessionID == id {
		t.Fatal("expected a new session id")
	}
	if _, err := session.Load(id); err == nil {
		t.Fatal("deleted session still on disk")
	}
}

func TestCopySlash(t *testing.T) {
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	quit, err := handleSlash(a, "/copy")
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
	a.Messages = []provider.Message{{Role: provider.RoleAssistant, Content: "ok"}}
	quit, err = handleSlash(a, "/copy")
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
}

func TestRetrySlashEmpty(t *testing.T) {
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	quit, err := handleSlash(a, "/retry")
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
}

func TestExportSlash(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	a.Messages = []provider.Message{{Role: provider.RoleUser, Content: "repl export"}}
	quit, err := handleSlash(a, "/export")
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
}

func TestDiffSlash(t *testing.T) {
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	quit, err := handleSlash(a, "/diff")
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
}

func TestJobsSlash(t *testing.T) {
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	a := agent.New(cfg, errClient{}, tools.New(cfg.WorkDir, nil, nil))
	quit, err := handleSlash(a, "/jobs")
	if quit || err != nil {
		t.Fatalf("quit=%v err=%v", quit, err)
	}
}

func TestPickPromptInterruptQuits(t *testing.T) {
	sig := make(chan os.Signal, 1)
	sig <- os.Interrupt
	_, err := pickPrompt(context.Background(), nil, sig)
	if !errors.Is(err, errPromptQuit) {
		t.Fatalf("got %v", err)
	}
}

func TestPickPromptLine(t *testing.T) {
	scanned := make(chan promptEvent, 1)
	scanned <- promptEvent{line: "hello"}
	line, err := pickPrompt(context.Background(), scanned, make(chan os.Signal))
	if err != nil || line != "hello" {
		t.Fatalf("%q %v", line, err)
	}
}

func TestPickPromptCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := pickPrompt(ctx, nil, make(chan os.Signal))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestPermissionAskAlwaysApprove(t *testing.T) {
	cfg := config.Default()
	cfg.AlwaysApprove = true
	if permissionAsk(cfg) != nil {
		t.Fatal("always_approve should skip ask")
	}
}
