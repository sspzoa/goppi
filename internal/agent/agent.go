package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/instructions"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/skills"
	"github.com/sspzoa/goppi/internal/tools"
)

type Agent struct {
	Cfg        config.Config
	Client     provider.Client
	Tools      *tools.Registry
	Messages   []provider.Message
	SessionID  string
	Quiet      bool
	Sink       Sink
	LastUsage  provider.Usage
	TotalUsage provider.Usage
	Incoming   []provider.Image
	hookLive   bool
	persistErr error
	sessLock   *session.Lock
}

func New(cfg config.Config, client provider.Client, registry *tools.Registry) *Agent {
	return &Agent{Cfg: cfg, Client: client, Tools: registry}
}

func (a *Agent) Close() {
	if a == nil {
		return
	}
	a.fireEnd("close")
	a.ReleaseSession()
	if a.Tools != nil {
		a.Tools.Close()
	}
}

func (a *Agent) ReleaseSession() {
	if a == nil || a.sessLock == nil {
		return
	}
	a.sessLock.Release()
	a.sessLock = nil
}

func (a *Agent) holdSession(id string) error {
	if a == nil || id == "" {
		return nil
	}
	if a.sessLock != nil && a.sessLock.ID == id {
		return nil
	}
	lk, err := session.Hold(id)
	if err != nil {
		return err
	}
	a.ReleaseSession()
	a.sessLock = lk
	return nil
}

func (a *Agent) Begin() error {
	if a == nil || a.hookLive {
		return nil
	}
	err := tools.FireSessionStart(context.Background(), a.Cfg, a.hookSessionID())
	a.hookLive = true
	return err
}

func (a *Agent) End(reason string) {
	a.fireEnd(reason)
}

func (a *Agent) hookSessionID() string {
	if a != nil && a.SessionID != "" {
		return a.SessionID
	}
	if a != nil {
		return a.Cfg.PromptCacheKey
	}
	return ""
}

func (a *Agent) fireEnd(reason string) {
	if a == nil || !a.hookLive {
		return
	}
	_ = tools.FireSessionEnd(context.Background(), a.Cfg, a.hookSessionID(), reason)
	a.hookLive = false
}

func (a *Agent) RewindLastUser() (string, error) {
	if a == nil {
		return "", fmt.Errorf("다시 보낼 메시지가 없습니다")
	}
	for i := len(a.Messages) - 1; i >= 0; i-- {
		if a.Messages[i].Role != provider.RoleUser {
			continue
		}
		text := a.Messages[i].Content
		var extra []provider.Image
		for _, img := range a.Messages[i].Images {
			if img.Path == "" || !strings.Contains(text, img.Path) {
				extra = append(extra, img)
			}
		}
		a.Incoming = extra
		a.Messages = a.Messages[:i]
		return text, nil
	}
	return "", fmt.Errorf("다시 보낼 메시지가 없습니다")
}

func (a *Agent) Retry(ctx context.Context) error {
	text, err := a.RewindLastUser()
	if err != nil {
		return err
	}
	return a.Run(ctx, text)
}

func (a *Agent) LastAssistant() string {
	if a == nil {
		return ""
	}
	for i := len(a.Messages) - 1; i >= 0; i-- {
		if a.Messages[i].Role == provider.RoleAssistant && strings.TrimSpace(a.Messages[i].Content) != "" {
			return tools.RedactSecrets(a.Messages[i].Content)
		}
	}
	return ""
}

func (a *Agent) LoadFile(f session.File) error {
	if a == nil {
		return nil
	}
	if err := a.holdSession(f.ID); err != nil {
		return err
	}
	if a.hookLive && a.SessionID != f.ID {
		a.fireEnd("load")
	}
	a.SessionID = f.ID
	a.Messages = CloseOpenToolCalls(f.Messages, "interrupted")
	a.Incoming = nil
	if f.CacheKey != "" {
		a.Cfg.PromptCacheKey = f.CacheKey
	}
	if f.Model != "" {
		a.Cfg.Model = f.Model
	}
	if f.Effort != "" {
		a.Cfg.ReasoningEffort = f.Effort
	}
	if f.Mode != "" {
		a.Cfg.Mode = f.Mode
	}
	if len(f.ExtraDirs) > 0 {
		a.Cfg.ExtraDirs = append([]string(nil), f.ExtraDirs...)
		if a.Tools != nil {
			a.Tools.SetExtraDirs(a.Cfg.ExtraDirs)
		}
	}
	if a.Tools != nil {
		a.Tools.ResetRuntime()
		a.Tools.SetMode(a.Cfg.Mode)
		a.Tools.Todos().Replace(f.Todos)
	}
	a.LastUsage = f.Usage
	a.TotalUsage = f.TotalUsage
	_ = a.Begin()
	return nil
}

func (a *Agent) SessionSnapshot() session.File {
	if a == nil {
		return session.File{}
	}
	f := session.File{
		ID:         a.SessionID,
		Messages:   tools.RedactMessages(a.Messages),
		Mode:       a.Cfg.Mode,
		Usage:      a.LastUsage,
		TotalUsage: a.TotalUsage,
	}
	if a.Tools != nil {
		f.Todos = a.Tools.Todos().Items()
	}
	return f
}

func (a *Agent) addUsage(u provider.Usage) {
	a.LastUsage = u
	a.TotalUsage.InputTokens += u.InputTokens
	a.TotalUsage.OutputTokens += u.OutputTokens
	a.TotalUsage.ReasoningTokens += u.ReasoningTokens
}

func (a *Agent) Reset() {
	a.recycle("reset")
}

func (a *Agent) Discard() {
	a.recycle("delete")
}

func (a *Agent) recycle(reason string) {
	if a == nil {
		return
	}
	a.fireEnd(reason)
	a.ReleaseSession()
	a.Messages = nil
	a.SessionID = ""
	a.Incoming = nil
	a.LastUsage = provider.Usage{}
	a.TotalUsage = provider.Usage{}
	a.persistErr = nil
	if a.Tools != nil {
		a.Tools.ResetRuntime()
		a.Tools.Todos().Replace(nil)
	}
}

func (a *Agent) sink() Sink {
	if a.Sink != nil {
		return a.Sink
	}
	return nopSink{}
}

func (a *Agent) EnsureSession() {
	if a == nil {
		return
	}
	if a.SessionID == "" {
		a.SessionID = session.NewID()
	}
	if err := a.holdSession(a.SessionID); err != nil {
		return
	}
	_ = a.Begin()
}

func (a *Agent) ExportMarkdown(arg string) (string, error) {
	if a == nil {
		return "", fmt.Errorf("내보낼 세션이 없습니다")
	}
	arg = strings.TrimSpace(arg)
	if arg != "" {
		f, err := session.Resolve(arg)
		if err != nil {
			return "", err
		}
		return session.WriteMarkdown(f)
	}
	if len(a.Messages) == 0 {
		if a.SessionID == "" {
			return "", fmt.Errorf("내보낼 세션이 없습니다")
		}
		f, err := session.Load(a.SessionID)
		if err != nil {
			return "", err
		}
		return session.WriteMarkdown(f)
	}
	a.EnsureSession()
	f := a.SessionSnapshot()
	id, err := session.PersistFull(a.Cfg, f)
	if err != nil {
		return "", err
	}
	a.SessionID = id
	if loaded, err := session.Load(id); err == nil {
		f = loaded
	} else {
		f.ID = id
	}
	return session.WriteMarkdown(f)
}

func (a *Agent) Save() error {
	if a == nil || a.SessionID == "" || len(a.Messages) == 0 {
		return nil
	}
	id, err := session.PersistFull(a.Cfg, a.SessionSnapshot())
	if err != nil {
		a.persistErr = err
		return fmt.Errorf("session save: %w", err)
	}
	a.persistErr = nil
	a.SessionID = id
	return nil
}

func (a *Agent) checkpoint() {
	_ = a.Save()
}

func (a *Agent) finish(err error) error {
	if a != nil && a.persistErr != nil {
		return fmt.Errorf("session save: %w", a.persistErr)
	}
	return err
}

func (a *Agent) Run(ctx context.Context, user string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.Messages = CloseOpenToolCalls(a.Messages, "interrupted")
	user, imgs := tools.ExpandMentions(a.Cfg.WorkDir, user, a.Cfg.ExtraDirs...)
	if len(a.Incoming) > 0 {
		imgs = append(imgs, a.Incoming...)
		a.Incoming = nil
	}
	if len(imgs) > 3 {
		imgs = imgs[:3]
	}
	a.Messages = append(a.Messages, provider.Message{
		Role:    provider.RoleUser,
		Content: user,
		Images:  imgs,
	})
	a.checkpoint()

	auto := 0
	for turn := 0; turn < a.Cfg.MaxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			a.Messages = CloseOpenToolCalls(a.Messages, err.Error())
			a.checkpoint()
			return a.finish(err)
		}
		if a.needsCompact() && auto < 2 {
			if err := a.autoCompact(ctx); err == nil {
				auto++
			}
		}
		resp, err := a.chat(ctx)
		if err != nil && provider.ContextOverflow(err) && a.Cfg.AutoCompact && auto < 2 {
			if cerr := a.autoCompact(ctx); cerr == nil {
				auto++
				resp, err = a.chat(ctx)
			}
		}
		a.sink().TurnEnd()
		if err != nil {
			a.checkpoint()
			return a.finish(err)
		}
		a.Messages = append(a.Messages, resp.Message)
		a.addUsage(resp.Usage)
		a.sink().Usage(resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.ReasoningTokens)
		a.checkpoint()
		if len(resp.Message.ToolCalls) == 0 {
			return a.finish(nil)
		}
		if err := a.runToolCalls(ctx, resp.Message.ToolCalls); err != nil {
			a.checkpoint()
			return a.finish(err)
		}
	}
	return a.finish(fmt.Errorf("stopped after %d turns", a.Cfg.MaxTurns))
}

type toolResult struct {
	content string
	err     error
	images  []provider.Image
}

func (a *Agent) runToolCalls(ctx context.Context, calls []provider.ToolCall) error {
	if a.Tools == nil {
		a.Messages = CloseOpenToolCalls(a.Messages, "no tools")
		return fmt.Errorf("no tools")
	}
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Name
	}
	if tools.AllParallelSafe(names) {
		return a.runToolCallsParallel(ctx, calls)
	}
	return a.runToolCallsSerial(ctx, calls)
}

func (a *Agent) runToolCallsSerial(ctx context.Context, calls []provider.ToolCall) error {
	for _, call := range calls {
		if err := ctx.Err(); err != nil {
			a.Messages = CloseOpenToolCalls(a.Messages, err.Error())
			return err
		}
		a.sink().ToolStart(call.Name, tools.RedactSecrets(tools.Detail(call.Name, call.Input)))
		out, err := a.Tools.Run(ctx, call.Name, call.Input)
		a.appendToolResult(call, toolResult{content: out, err: err, images: a.Tools.TakeImages()})
	}
	return nil
}

const maxParallelTools = 8

func (a *Agent) runToolCallsParallel(ctx context.Context, calls []provider.ToolCall) error {
	results := make([]toolResult, len(calls))
	sem := make(chan struct{}, maxParallelTools)
	var wg sync.WaitGroup
	for i, call := range calls {
		a.sink().ToolStart(call.Name, tools.RedactSecrets(tools.Detail(call.Name, call.Input)))
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = toolResult{err: ctx.Err()}
				return
			}
			if err := ctx.Err(); err != nil {
				results[i] = toolResult{err: err}
				return
			}
			out, err := a.Tools.Run(ctx, call.Name, call.Input)
			results[i] = toolResult{content: out, err: err, images: a.Tools.TakeImages()}
		}()
	}
	wg.Wait()
	for i, call := range calls {
		a.appendToolResult(call, results[i])
	}
	if err := ctx.Err(); err != nil {
		a.Messages = CloseOpenToolCalls(a.Messages, err.Error())
		return err
	}
	return nil
}

func (a *Agent) appendToolResult(call provider.ToolCall, res toolResult) {
	msg := provider.Message{
		Role:       provider.RoleTool,
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Images:     res.images,
	}
	if res.err != nil {
		a.sink().ToolDone("", res.err)
		msg.Content = "error: " + res.err.Error()
	} else {
		a.sink().ToolDone(Summarize(res.content), nil)
		msg.Content = res.content
	}
	a.Messages = append(a.Messages, msg)
	a.checkpoint()
}

func (a *Agent) chat(ctx context.Context) (provider.ChatResponse, error) {
	extra, _ := instructions.Load(a.Cfg.WorkDir)
	sk := skills.Load(a.Cfg.WorkDir)
	if a.Tools != nil {
		a.Tools.SetSkills(sk)
		a.Tools.SetMode(a.Cfg.Mode)
	}
	tools.HydrateImages(a.Cfg.WorkDir, a.Messages, a.Cfg.ExtraDirs...)
	var specs []provider.ToolSpec
	if a.Tools != nil {
		specs = a.Tools.Specs()
	}
	return a.Client.Chat(ctx, provider.ChatRequest{
		Model:           a.Cfg.Model,
		System:          systemPrompt(a.Cfg, extra, skills.Names(sk)),
		Messages:        tools.RedactMessages(a.Messages),
		Tools:           specs,
		MaxTokens:       a.Cfg.MaxTokens,
		ReasoningEffort: a.Cfg.ReasoningEffort,
		PromptCacheKey:  a.Cfg.PromptCacheKey,
		OnDelta: func(d provider.Delta) {
			a.sink().Delta(tools.RedactSecrets(d.Reasoning), tools.RedactSecrets(d.Content))
		},
	})
}

func (a *Agent) autoCompact(ctx context.Context) error {
	if err := a.Compact(ctx); err != nil {
		return err
	}
	a.LastUsage = provider.Usage{}
	a.sink().Compacted()
	return nil
}

func (a *Agent) needsCompact() bool {
	if a == nil || !a.Cfg.AutoCompact {
		return false
	}
	if len(a.Messages) < 3 {
		return false
	}
	tok := a.LastUsage.InputTokens
	if tok == 0 {
		tok = estimateTokens(a.Messages)
	}
	at := a.Cfg.CompactAt
	if at <= 0 {
		at = 100000
	}
	return tok >= at
}

func estimateTokens(msgs []provider.Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Content) + len(m.Reasoning)
		for _, tc := range m.ToolCalls {
			n += len(tc.Name) + len(tc.Input)
		}
		n += len(m.Images) * 8000
	}
	return n / 4
}

func Summarize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Count(s, "\n") + 1
	if lines == 1 && len(s) < 80 {
		return s
	}
	return fmt.Sprintf("%d lines", lines)
}

func (a *Agent) Compact(ctx context.Context) error {
	a.Messages = CloseOpenToolCalls(a.Messages, "interrupted")
	if len(a.Messages) < 3 {
		return fmt.Errorf("nothing to compact")
	}
	var b strings.Builder
	b.WriteString("Summarize this coding-agent transcript for continuity.\nKeep files touched, decisions, and remaining work. Match the user's language.\n\n")
	for _, m := range a.Messages {
		switch m.Role {
		case provider.RoleUser:
			fmt.Fprintf(&b, "USER: %s\n", trimRunes(m.Content, 800))
		case provider.RoleAssistant:
			if m.Content != "" {
				fmt.Fprintf(&b, "ASSISTANT: %s\n", trimRunes(m.Content, 800))
			}
			for _, tc := range m.ToolCalls {
				fmt.Fprintf(&b, "TOOLCALL %s %s\n", tc.Name, trimRunes(string(tc.Input), 200))
			}
		}
	}
	resp, err := a.Client.Chat(ctx, provider.ChatRequest{
		Model:           a.Cfg.Model,
		System:          "You compress agent transcripts. Output only the summary.",
		Messages:        []provider.Message{{Role: provider.RoleUser, Content: tools.RedactSecrets(b.String())}},
		MaxTokens:       2048,
		ReasoningEffort: "low",
	})
	if err != nil {
		return err
	}
	sum := strings.TrimSpace(resp.Message.Content)
	if sum == "" {
		return fmt.Errorf("empty compact summary")
	}
	a.Messages = []provider.Message{
		{Role: provider.RoleUser, Content: "이전 세션 요약:\n" + sum},
		{Role: provider.RoleAssistant, Content: "요약을 반영했다. 이어서 진행한다."},
	}
	a.checkpoint()
	return a.finish(nil)
}

func trimRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func systemPrompt(cfg config.Config, extra string, skillNames []string) string {
	mode := cfg.Mode
	if mode == "" {
		mode = "act"
	}
	s := fmt.Sprintf(`너는 고삐다. 사용자 머신에서 도는 한국형 코딩 에이전트 하네스다.
툴로 파일을 보고 고친다. 말은 짧게, 일은 직접 한다.

작업 디렉터리: %s
모드: %s

툴:
- read_file / write_file / edit_file / apply_patch / glob / grep / bash / bash_poll / bash_kill / diagnostics
- todo_write: 여러 단계 일의 체크리스트. 상태가 바뀌면 다시 쓴다.
- ask_user: 선택이 필요하면 옵션과 함께 물어라. 추측하지 마라.
- web_fetch: http(s) 문서. JS 없는 GET.
- read_skill: 프로젝트 skill(SKILL.md).
- document_parse / document_ocr: PDF·오피스·이미지 (Upstage일 때).
- mcp_<server>_<tool>: 신뢰된 MCP 서버. 부작용이 있을 수 있다.
- delegate: 읽기 전용 서브에이전트. 넓은 탐색이나 두 번째 패스. 수정·bash·MCP·재귀 delegate는 없다.
- read_image: 스크린샷·UI 이미지를 모델에 붙인다. 프롬프트의 @path 는 workdir 텍스트·이미지도 붙인다.

규칙:
- 고치기 전에 읽어라. 읽기 tool은 한 번에 여러 개를 같이 불러도 된다. diff는 작고 정확하게.
- 여러 곳·여러 파일이면 apply_patch. edit_file은 정확히 한 곳만.
- bash는 작업 디렉터리에서 돈다. sandbox=workspace이면 workdir·임시·캐시 밖 쓰기는 거부된다. strict면 네트워크도 없다. 서버는 background=true 로 켜고 bash_poll / bash_kill 로 본다.
- 고친 뒤에는 diagnostics로 언어 서버 진단을 확인하라.
- 여러 단계면 먼저 todo_write.
- 스캔·오피스 파일은 document_parse.
- 사용자 언어로 답해라. 끝나면 뭐가 바뀌었는지 한 줄로.
`, cfg.WorkDir, mode)
	if cfg.Worktree {
		s += "\n지금 git worktree에서 일한다. 사용자 메인 체크아웃은 그대로다. 커밋은 이 worktree 브랜치에만 한다.\n"
	}
	if mode == "plan" {
		s += `
지금 PLAN 모드다. 코드를 바꾸지 마라. write_file / edit_file / apply_patch / bash / MCP는 거부된다.
코드를 읽고, 질문을 하고, 실행 가능한 계획을 써라. 실행은 사용자가 /act 한 뒤에.
`
	}
	if len(skillNames) > 0 {
		s += "\n사용 가능한 skill: " + strings.Join(skillNames, ", ") + "\n해당되면 read_skill로 본문을 읽어라.\n"
	}
	if strings.TrimSpace(extra) != "" {
		s += "\n프로젝트 지시:\n" + extra + "\n"
	}
	return s
}
