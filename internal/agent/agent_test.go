package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/tools"
)

type scriptClient struct {
	steps []provider.ChatResponse
	err   error
	n     int
	last  []provider.Message
}

type failThenScript struct {
	fail error
	once bool
	scriptClient
}

func (f *failThenScript) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if !f.once {
		f.once = true
		return provider.ChatResponse{}, f.fail
	}
	return f.scriptClient.Chat(ctx, req)
}

func (s *scriptClient) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return provider.ChatResponse{}, err
	}
	if s.n >= len(s.steps) {
		if s.err != nil {
			return provider.ChatResponse{}, s.err
		}
		return provider.ChatResponse{}, errors.New("unexpected extra chat")
	}
	s.last = append([]provider.Message(nil), req.Messages...)
	step := s.steps[s.n]
	s.n++
	if req.OnDelta != nil && step.Message.Content != "" {
		req.OnDelta(provider.Delta{Content: step.Message.Content})
	}
	return step, nil
}

func testAgent(t *testing.T, client provider.Client) *Agent {
	t.Helper()
	if os.Getenv("GOPPI_DATA_DIR") == "" {
		t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	}
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	cfg.MaxTurns = 8
	reg := tools.New(cfg.WorkDir, nil, tools.AlwaysAllow)
	return New(cfg, client, reg)
}

func TestRunAttachesMentionedFile(t *testing.T) {
	c := &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "ok"},
	}}}
	a := testAgent(t, c)
	if err := os.WriteFile(filepath.Join(a.Cfg.WorkDir, "note.md"), []byte("hello mention"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.Run(context.Background(), "see @note.md"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Messages[0].Content, "hello mention") {
		t.Fatalf("%q", a.Messages[0].Content)
	}
}

func TestRunPlainReply(t *testing.T) {
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "done"},
		Usage:   provider.Usage{InputTokens: 3, OutputTokens: 1},
	}}})
	if err := a.Run(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if a.LastUsage.InputTokens != 3 || a.TotalUsage.InputTokens != 3 || a.TotalUsage.OutputTokens != 1 {
		t.Fatalf("usage last=%+v total=%+v", a.LastUsage, a.TotalUsage)
	}
	if got := a.Messages[len(a.Messages)-1].Content; got != "done" {
		t.Fatalf("last = %q", got)
	}
}

func TestSessionSnapshotRedactsSecrets(t *testing.T) {
	a := testAgent(t, &scriptClient{})
	a.Messages = []provider.Message{{
		Role:    provider.RoleUser,
		Content: "here is sk-abcdefghijklmnopqrst",
	}}
	f := a.SessionSnapshot()
	if strings.Contains(f.Messages[0].Content, "sk-abcdefghijklmnopqrst") {
		t.Fatalf("%q", f.Messages[0].Content)
	}
	if !strings.Contains(a.Messages[0].Content, "sk-abcdefghijklmnopqrst") {
		t.Fatal("live transcript must stay")
	}
}

func TestChatRedactsSecretsBeforeProvider(t *testing.T) {
	c := &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "ok"},
	}}}
	a := testAgent(t, c)
	if err := a.Run(context.Background(), "use sk-abcdefghijklmnopqrst"); err != nil {
		t.Fatal(err)
	}
	if len(c.last) == 0 || !strings.Contains(c.last[0].Content, "[redacted]") || strings.Contains(c.last[0].Content, "sk-abcdefghijklmnopqrst") {
		t.Fatalf("%+v", c.last)
	}
	if !strings.Contains(a.Messages[0].Content, "sk-abcdefghijklmnopqrst") {
		t.Fatal("live user text must stay")
	}
}

type recordSink struct {
	delta, start string
}

func (r *recordSink) Delta(reasoning, content string) { r.delta += reasoning + content }
func (*recordSink) TurnEnd()                          {}
func (*recordSink) Usage(int, int, int)               {}
func (r *recordSink) ToolStart(name, detail string)   { r.start = name + " " + detail }
func (*recordSink) ToolDone(string, error)            {}
func (*recordSink) Compacted()                        {}

func TestSinkRedactsSecrets(t *testing.T) {
	const secret = "sk-abcdefghijklmnopqrst"
	sink := &recordSink{}
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{
		{Message: provider.Message{
			Role:    provider.RoleAssistant,
			Content: "use " + secret,
			ToolCalls: []provider.ToolCall{{
				ID:    "1",
				Name:  "bash",
				Input: json.RawMessage(`{"command":"echo ` + secret + `"}`),
			}},
		}},
		{Message: provider.Message{Role: provider.RoleAssistant, Content: "done"}},
	}})
	a.Sink = sink
	a.Tools = tools.New(t.TempDir(), nil, tools.AlwaysAllow)
	if err := a.Run(context.Background(), "run"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sink.delta, secret) || strings.Contains(sink.start, secret) {
		t.Fatalf("sink leaked delta=%q start=%q", sink.delta, sink.start)
	}
}

func TestLastAssistantRedactsSecrets(t *testing.T) {
	a := testAgent(t, &scriptClient{})
	a.Messages = []provider.Message{{
		Role:    provider.RoleAssistant,
		Content: "key sk-abcdefghijklmnopqrst",
	}}
	got := a.LastAssistant()
	if strings.Contains(got, "sk-abcdefghijklmnopqrst") || !strings.Contains(got, "[redacted]") {
		t.Fatalf("%q", got)
	}
}

func TestCompactRedactsSecrets(t *testing.T) {
	c := &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "요약"},
	}}}
	a := testAgent(t, c)
	a.Messages = []provider.Message{
		{Role: provider.RoleUser, Content: "use sk-abcdefghijklmnopqrst"},
		{Role: provider.RoleAssistant, Content: "ok"},
		{Role: provider.RoleUser, Content: "next"},
	}
	if err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(c.last) == 0 || strings.Contains(c.last[0].Content, "sk-abcdefghijklmnopqrst") {
		t.Fatalf("%+v", c.last)
	}
}

func TestRunToolThenReply(t *testing.T) {
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{
		{Message: provider.Message{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{{
				ID:    "1",
				Name:  "glob",
				Input: json.RawMessage(`{"pattern":"*.go"}`),
			}},
		}},
		{Message: provider.Message{Role: provider.RoleAssistant, Content: "no go files"}},
	}})
	if err := a.Run(context.Background(), "find go"); err != nil {
		t.Fatal(err)
	}
	var roles []provider.Role
	for _, m := range a.Messages {
		roles = append(roles, m.Role)
	}
	if len(roles) < 4 || roles[2] != provider.RoleTool {
		t.Fatalf("roles %v", roles)
	}
}

func TestRunParallelReads(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{
		{Message: provider.Message{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{
				{ID: "1", Name: "read_file", Input: json.RawMessage(`{"path":"a.txt"}`)},
				{ID: "2", Name: "read_file", Input: json.RawMessage(`{"path":"b.txt"}`)},
			},
		}},
		{Message: provider.Message{Role: provider.RoleAssistant, Content: "둘 다 읽음"}},
	}})
	a.Cfg.WorkDir = dir
	a.Tools = tools.New(dir, nil, tools.AlwaysAllow)
	if err := a.Run(context.Background(), "읽어"); err != nil {
		t.Fatal(err)
	}
	var toolsOut []string
	for _, m := range a.Messages {
		if m.Role == provider.RoleTool {
			toolsOut = append(toolsOut, m.ToolCallID+" "+m.Content)
		}
	}
	if len(toolsOut) != 2 {
		t.Fatalf("tools %v", toolsOut)
	}
	if !strings.Contains(toolsOut[0], "alpha") || !strings.Contains(toolsOut[1], "beta") {
		t.Fatalf("order/content %v", toolsOut)
	}
}

func TestRunMixedBatchStaysSerial(t *testing.T) {
	dir := t.TempDir()
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{
		{Message: provider.Message{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{
				{ID: "1", Name: "read_file", Input: json.RawMessage(`{"path":"missing.txt"}`)},
				{ID: "2", Name: "write_file", Input: json.RawMessage(`{"path":"out.txt","contents":"x"}`)},
			},
		}},
		{Message: provider.Message{Role: provider.RoleAssistant, Content: "ok"}},
	}})
	a.Cfg.WorkDir = dir
	a.Tools = tools.New(dir, nil, tools.AlwaysAllow)
	if err := a.Run(context.Background(), "써"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestRunToolErrorBecomesMessage(t *testing.T) {
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{
		{Message: provider.Message{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{{
				ID:    "1",
				Name:  "read_file",
				Input: json.RawMessage(`{"path":"../secret"}`),
			}},
		}},
		{Message: provider.Message{Role: provider.RoleAssistant, Content: "blocked"}},
	}})
	if err := a.Run(context.Background(), "read secret"); err != nil {
		t.Fatal(err)
	}
	var tool provider.Message
	for _, m := range a.Messages {
		if m.Role == provider.RoleTool {
			tool = m
		}
	}
	if !strings.Contains(tool.Content, "error:") {
		t.Fatalf("tool msg %q", tool.Content)
	}
}

func TestRunStopsAtMaxTurns(t *testing.T) {
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	cfg.MaxTurns = 1
	loop := &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{{
				ID:    "1",
				Name:  "glob",
				Input: json.RawMessage(`{"pattern":"*"}`),
			}},
		},
	}}}
	a := New(cfg, loop, tools.New(cfg.WorkDir, nil, nil))
	err := a.Run(context.Background(), "loop")
	if err == nil || !strings.Contains(err.Error(), "stopped after") {
		t.Fatalf("got %v", err)
	}
}

func TestCloseOpenToolCalls(t *testing.T) {
	msgs := []provider.Message{{
		Role: provider.RoleAssistant,
		ToolCalls: []provider.ToolCall{
			{ID: "1", Name: "glob", Input: json.RawMessage(`{"pattern":"*"}`)},
			{ID: "2", Name: "glob", Input: json.RawMessage(`{"pattern":"*"}`)},
		},
	}, {
		Role:       provider.RoleTool,
		ToolCallID: "1",
		Content:    "ok",
	}}
	got := CloseOpenToolCalls(msgs, "canceled")
	if len(got) != 3 || got[2].ToolCallID != "2" || !strings.Contains(got[2].Content, "canceled") {
		t.Fatalf("%+v", got)
	}
}

func TestRunCancelClosesRemainingTools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{
				{ID: "1", Name: "glob", Input: json.RawMessage(`{"pattern":"*"}`)},
				{ID: "2", Name: "glob", Input: json.RawMessage(`{"pattern":"*"}`)},
			},
		},
	}}})
	a.Sink = cancelAfterTool{cancel: cancel}
	err := a.Run(ctx, "x")
	if err == nil {
		t.Fatal("expected cancel")
	}
	var toolIDs []string
	for _, m := range a.Messages {
		if m.Role == provider.RoleTool {
			toolIDs = append(toolIDs, m.ToolCallID)
		}
	}
	if len(toolIDs) != 2 {
		t.Fatalf("incomplete tool round %v", toolIDs)
	}
}

type cancelAfterTool struct{ cancel func() }

func (cancelAfterTool) Delta(string, string)     {}
func (cancelAfterTool) TurnEnd()                 {}
func (cancelAfterTool) Usage(int, int, int)      {}
func (cancelAfterTool) ToolStart(string, string) {}
func (c cancelAfterTool) ToolDone(string, error) { c.cancel() }
func (cancelAfterTool) Compacted()               {}

func TestPlanModeDeniesWrite(t *testing.T) {
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{
		{Message: provider.Message{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{{
				ID: "1", Name: "write_file", Input: json.RawMessage(`{"path":"x","contents":"y"}`),
			}},
		}},
		{Message: provider.Message{Role: provider.RoleAssistant, Content: "ok plan"}},
	}})
	a.Cfg.Mode = "plan"
	if err := a.Run(context.Background(), "write"); err != nil {
		t.Fatal(err)
	}
	var tool provider.Message
	for _, m := range a.Messages {
		if m.Role == provider.RoleTool {
			tool = m
		}
	}
	if !strings.Contains(tool.Content, "plan mode") {
		t.Fatalf("got %q", tool.Content)
	}
}

func TestCompact(t *testing.T) {
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "요약: 파일 a를 고침"},
	}}})
	a.Messages = []provider.Message{
		{Role: provider.RoleUser, Content: "고쳐"},
		{Role: provider.RoleAssistant, Content: "고쳤다"},
		{Role: provider.RoleUser, Content: "테스트도"},
	}
	if err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(a.Messages) != 2 || !strings.Contains(a.Messages[0].Content, "요약") {
		t.Fatalf("%+v", a.Messages)
	}
}

func TestRunCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "no"},
	}}})
	if err := a.Run(ctx, "hi"); err == nil {
		t.Fatal("expected canceled context")
	}
}

func TestUserMentionAttachesImage(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shot.png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	var sawImages int
	client := captureImages{n: &sawImages, next: &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "a button"},
	}}}}
	a := testAgent(t, &client)
	a.Cfg.WorkDir = dir
	if err := a.Run(context.Background(), "이게 뭐야 @shot.png"); err != nil {
		t.Fatal(err)
	}
	if sawImages != 1 {
		t.Fatalf("images %d", sawImages)
	}
}

type captureImages struct {
	n    *int
	next *scriptClient
}

func (c *captureImages) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	for _, m := range req.Messages {
		*c.n += len(m.Images)
	}
	return c.next.Chat(ctx, req)
}

func TestDelegateRunsPlanSubagent(t *testing.T) {
	sub := &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "found cmd/goppi"},
	}}}
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	cfg.MaxTurns = 8
	reg := tools.New(cfg.WorkDir, nil, nil)
	reg.EnableDelegate(func(ctx context.Context, prompt string) (string, error) {
		a := New(cfg, sub, tools.New(cfg.WorkDir, nil, nil))
		a.Cfg.Mode = "plan"
		a.Quiet = true
		if err := a.Run(ctx, prompt); err != nil {
			return "", err
		}
		return a.LastAssistant(), nil
	})
	parent := New(cfg, &scriptClient{steps: []provider.ChatResponse{
		{Message: provider.Message{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{{
				ID:    "1",
				Name:  "delegate",
				Input: json.RawMessage(`{"prompt":"find mains"}`),
			}},
		}},
		{Message: provider.Message{Role: provider.RoleAssistant, Content: "ok"}},
	}}, reg)
	if err := parent.Run(context.Background(), "explore"); err != nil {
		t.Fatal(err)
	}
	var tool provider.Message
	for _, m := range parent.Messages {
		if m.Role == provider.RoleTool {
			tool = m
		}
	}
	if !strings.Contains(tool.Content, "found cmd/goppi") {
		t.Fatalf("%q", tool.Content)
	}
	if sub.n != 1 {
		t.Fatalf("sub chats %d", sub.n)
	}
}

func TestRunChatError(t *testing.T) {
	a := testAgent(t, &scriptClient{err: errors.New("boom")})
	if err := a.Run(context.Background(), "x"); err == nil || err.Error() != "boom" {
		t.Fatalf("got %v", err)
	}
}

func TestRunAutoCompactOnOverflow(t *testing.T) {
	c := &failThenScript{
		fail: errors.New("upstage 400: context_length_exceeded"),
		scriptClient: scriptClient{steps: []provider.ChatResponse{
			{Message: provider.Message{Role: provider.RoleAssistant, Content: "요약: foo를 고침"}},
			{Message: provider.Message{Role: provider.RoleAssistant, Content: "이어서 한다"}},
		}},
	}
	a := testAgent(t, c)
	a.Messages = []provider.Message{
		{Role: provider.RoleUser, Content: "first"},
		{Role: provider.RoleAssistant, Content: "ok"},
	}
	if err := a.Run(context.Background(), "계속"); err != nil {
		t.Fatal(err)
	}
	if a.LastAssistant() != "이어서 한다" {
		t.Fatalf("got %q", a.LastAssistant())
	}
	if !strings.Contains(a.Messages[0].Content, "요약") {
		t.Fatalf("expected compact summary: %+v", a.Messages)
	}
}

func TestRunAutoCompactOnUsage(t *testing.T) {
	c := &scriptClient{steps: []provider.ChatResponse{
		{
			Message: provider.Message{
				Role: provider.RoleAssistant,
				ToolCalls: []provider.ToolCall{{
					ID: "1", Name: "glob", Input: json.RawMessage(`{"pattern":"*"}`),
				}},
			},
			Usage: provider.Usage{InputTokens: 200000},
		},
		{Message: provider.Message{Role: provider.RoleAssistant, Content: "요약: glob 함"}},
		{Message: provider.Message{Role: provider.RoleAssistant, Content: "끝"}},
	}}
	a := testAgent(t, c)
	if err := a.Run(context.Background(), "찾아"); err != nil {
		t.Fatal(err)
	}
	if a.LastAssistant() != "끝" {
		t.Fatalf("got %q", a.LastAssistant())
	}
	if !strings.Contains(a.Messages[0].Content, "요약") {
		t.Fatalf("expected compact: %+v", a.Messages)
	}
}

func TestRunNoAutoCompactWhenOff(t *testing.T) {
	a := testAgent(t, &scriptClient{err: errors.New("context_length_exceeded")})
	a.Cfg.AutoCompact = false
	a.Messages = []provider.Message{
		{Role: provider.RoleUser, Content: "first"},
		{Role: provider.RoleAssistant, Content: "ok"},
	}
	err := a.Run(context.Background(), "계속")
	if err == nil || !strings.Contains(err.Error(), "context_length") {
		t.Fatalf("got %v", err)
	}
}

func TestNeedsCompact(t *testing.T) {
	a := testAgent(t, &scriptClient{})
	if a.needsCompact() {
		t.Fatal("empty")
	}
	a.Messages = []provider.Message{
		{Role: provider.RoleUser, Content: "a"},
		{Role: provider.RoleAssistant, Content: "b"},
		{Role: provider.RoleUser, Content: "c"},
	}
	a.LastUsage = provider.Usage{InputTokens: 200000}
	if !a.needsCompact() {
		t.Fatal("expected compact")
	}
	a.Cfg.AutoCompact = false
	if a.needsCompact() {
		t.Fatal("disabled")
	}
}

func TestCheckpointAfterTool(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{{
				ID: "1", Name: "write_file", Input: json.RawMessage(`{"path":"x.txt","contents":"saved"}`),
			}},
		},
	}}})
	a.SessionID = session.NewID()
	a.Sink = cancelAfterTool{cancel: cancel}
	_ = a.Run(ctx, "써 둬")
	got, err := session.Load(a.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	var sawTool bool
	for _, m := range got.Messages {
		if m.Role == provider.RoleTool && m.ToolCallID == "1" {
			sawTool = true
		}
	}
	if !sawTool {
		t.Fatalf("disk missing tool result: %+v", got.Messages)
	}
}

func TestResetEndsSessionAndClearsUsage(t *testing.T) {
	a := testAgent(t, &scriptClient{})
	a.Cfg.Hooks.SessionEnd = []config.Hook{{Command: "echo end >> log"}}
	a.Cfg.Hooks.SessionStart = []config.Hook{{Command: "echo start >> log"}}
	a.SessionID = "0123456789abcdef"
	a.LastUsage = provider.Usage{InputTokens: 9, OutputTokens: 2, ReasoningTokens: 1}
	a.TotalUsage = provider.Usage{InputTokens: 40, OutputTokens: 8}
	if err := a.Begin(); err != nil {
		t.Fatal(err)
	}
	a.Reset()
	if a.LastUsage != (provider.Usage{}) || a.TotalUsage != (provider.Usage{}) {
		t.Fatalf("usage last=%+v total=%+v", a.LastUsage, a.TotalUsage)
	}
	a.EnsureSession()
	if a.SessionID == "" || a.SessionID == "0123456789abcdef" {
		t.Fatalf("id %s", a.SessionID)
	}
	got, err := os.ReadFile(filepath.Join(a.Cfg.WorkDir, "log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "start\nend\nstart\n" {
		t.Fatalf("hooks %q", got)
	}
}

func TestLoadFileEndsPreviousSession(t *testing.T) {
	a := testAgent(t, &scriptClient{})
	a.Cfg.Hooks.SessionEnd = []config.Hook{{Command: "touch ended"}}
	a.SessionID = "aaaaaaaaaaaaaaaa"
	a.LastUsage = provider.Usage{InputTokens: 4}
	if err := a.Begin(); err != nil {
		t.Fatal(err)
	}
	if err := a.LoadFile(session.File{
		ID:         "bbbbbbbbbbbbbbbb",
		Messages:   []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		Usage:      provider.Usage{InputTokens: 7, OutputTokens: 2},
		TotalUsage: provider.Usage{InputTokens: 21, OutputTokens: 6},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.Cfg.WorkDir, "ended")); err != nil {
		t.Fatal(err)
	}
	if a.LastUsage.InputTokens != 7 || a.TotalUsage.InputTokens != 21 {
		t.Fatalf("usage last=%+v total=%+v", a.LastUsage, a.TotalUsage)
	}
	if a.SessionID != "bbbbbbbbbbbbbbbb" {
		t.Fatalf("id %s", a.SessionID)
	}
}

func TestCloseEndsSession(t *testing.T) {
	a := testAgent(t, &scriptClient{})
	a.Cfg.Hooks.SessionEnd = []config.Hook{{Command: "touch closed"}}
	if err := a.Begin(); err != nil {
		t.Fatal(err)
	}
	a.Close()
	if _, err := os.Stat(filepath.Join(a.Cfg.WorkDir, "closed")); err != nil {
		t.Fatal(err)
	}
	a.Close()
	a.Close()
}

func TestResetKillsBackgroundJobs(t *testing.T) {
	a := testAgent(t, &scriptClient{})
	t.Cleanup(a.Close)
	in, _ := json.Marshal(map[string]any{"command": "sleep 30", "background": true})
	if _, err := a.Tools.Run(context.Background(), "bash", in); err != nil {
		t.Fatal(err)
	}
	a.Reset()
	run, total := a.Tools.JobCounts()
	if run != 0 || total != 0 {
		t.Fatalf("running=%d total=%d", run, total)
	}
}

func TestRetryResendsLastUser(t *testing.T) {
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "again"},
	}}})
	a.Messages = []provider.Message{
		{Role: provider.RoleUser, Content: "do it"},
		{Role: provider.RoleAssistant, Content: "partial"},
	}
	if err := a.Retry(context.Background()); err != nil {
		t.Fatal(err)
	}
	var users int
	for _, m := range a.Messages {
		if m.Role == provider.RoleUser && m.Content == "do it" {
			users++
		}
	}
	if users != 1 {
		t.Fatalf("users %d: %+v", users, a.Messages)
	}
	last := a.Messages[len(a.Messages)-1]
	if last.Role != provider.RoleAssistant || last.Content != "again" {
		t.Fatalf("%+v", last)
	}
}

func TestRewindLastUserEmpty(t *testing.T) {
	a := testAgent(t, &scriptClient{})
	if _, err := a.RewindLastUser(); err == nil {
		t.Fatal("empty should fail")
	}
}

func TestExportMarkdownCurrent(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	a := testAgent(t, &scriptClient{})
	a.Messages = []provider.Message{{Role: provider.RoleUser, Content: "keep this"}}
	path, err := a.ExportMarkdown("")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "keep this") {
		t.Fatalf("%s", data)
	}
	if a.SessionID == "" {
		t.Fatal("export should assign a session id")
	}
}

func TestExportMarkdownEmpty(t *testing.T) {
	a := testAgent(t, &scriptClient{})
	if _, err := a.ExportMarkdown(""); err == nil {
		t.Fatal("empty session should fail")
	}
}

func TestRunReportsPersistFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "ok"},
	}}})
	a.EnsureSession()
	if err := os.RemoveAll(filepath.Join(root, "sessions")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sessions"), []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := a.Run(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "session save") {
		t.Fatalf("got %v", err)
	}
	if a.LastAssistant() != "ok" {
		t.Fatalf("reply %q", a.LastAssistant())
	}
}

func TestRunCancelReportsPersistFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", root)
	ctx, cancel := context.WithCancel(context.Background())
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{{
				ID: "1", Name: "glob", Input: json.RawMessage(`{"pattern":"*"}`),
			}},
		}},
	}})
	a.EnsureSession()
	a.Sink = cancelAfterTool{cancel: func() {
		_ = os.RemoveAll(filepath.Join(root, "sessions"))
		_ = os.WriteFile(filepath.Join(root, "sessions"), []byte("not a dir"), 0o600)
		cancel()
	}}
	err := a.Run(ctx, "x")
	if err == nil || !strings.Contains(err.Error(), "session save") {
		t.Fatalf("got %v", err)
	}
}

func TestNoSessionIDSkipsPersist(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	a := testAgent(t, &scriptClient{steps: []provider.ChatResponse{{
		Message: provider.Message{Role: provider.RoleAssistant, Content: "ok"},
	}}})
	if err := a.Run(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	items, err := session.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("tests without SessionID should not write: %+v", items)
	}
}
