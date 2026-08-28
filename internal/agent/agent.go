package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/instructions"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/tools"
)

type Agent struct {
	Cfg       config.Config
	Client    provider.Client
	Tools     *tools.Registry
	Messages  []provider.Message
	SessionID string
	Quiet     bool
	Sink      Sink
	LastUsage provider.Usage
}

func New(cfg config.Config, client provider.Client, registry *tools.Registry) *Agent {
	return &Agent{Cfg: cfg, Client: client, Tools: registry}
}

func (a *Agent) Reset() {
	a.Messages = nil
	a.SessionID = ""
}

func (a *Agent) sink() Sink {
	if a.Sink != nil {
		return a.Sink
	}
	return nopSink{}
}

func (a *Agent) Run(ctx context.Context, user string) error {
	a.Messages = append(a.Messages, provider.Message{Role: provider.RoleUser, Content: user})

	for turn := 0; turn < a.Cfg.MaxTurns; turn++ {
		extra, _ := instructions.Load(a.Cfg.WorkDir)
		req := provider.ChatRequest{
			Model:           a.Cfg.Model,
			System:          systemPrompt(a.Cfg.WorkDir, extra),
			Messages:        a.Messages,
			Tools:           a.Tools.Specs(),
			MaxTokens:       a.Cfg.MaxTokens,
			ReasoningEffort: a.Cfg.ReasoningEffort,
			PromptCacheKey:  a.Cfg.PromptCacheKey,
			OnDelta: func(d provider.Delta) {
				a.sink().Delta(d.Reasoning, d.Content)
			},
		}
		resp, err := a.Client.Chat(ctx, req)
		a.sink().TurnEnd()
		if err != nil {
			return err
		}
		a.Messages = append(a.Messages, resp.Message)
		a.LastUsage = resp.Usage
		a.sink().Usage(resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.ReasoningTokens)
		if len(resp.Message.ToolCalls) == 0 {
			return nil
		}
		for _, call := range resp.Message.ToolCalls {
			detail := tools.Detail(call.Name, call.Input)
			a.sink().ToolStart(call.Name, detail)
			result, err := a.Tools.Run(ctx, call.Name, call.Input)
			msg := provider.Message{
				Role:       provider.RoleTool,
				ToolCallID: call.ID,
				ToolName:   call.Name,
			}
			if err != nil {
				a.sink().ToolDone("", err)
				msg.Content = "error: " + err.Error()
			} else {
				a.sink().ToolDone(Summarize(result), nil)
				msg.Content = result
			}
			a.Messages = append(a.Messages, msg)
		}
	}
	return fmt.Errorf("stopped after %d turns", a.Cfg.MaxTurns)
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

func systemPrompt(workdir, extra string) string {
	s := fmt.Sprintf(`너는 고삐다. 사용자 머신에서 도는 한국형 코딩 에이전트 하네스다.
툴로 파일을 보고 고친다. 말은 짧게, 일은 직접 한다.

작업 디렉터리: %s

문서 툴:
- document_parse: PDF/HWP/Office/이미지 → 레이아웃 있는 Markdown. 문서 기본.
- document_ocr: 레이아웃이 필요 없을 때 텍스트만.

규칙:
- 계획만 말하지 말고 일을 해라.
- 고치기 전에 읽어라. diff는 작고 정확하게.
- edit_file은 정확히 한 곳만 바꿔야 한다. 안 맞으면 old_string을 넓히거나 줄여라.
- bash는 작업 디렉터리에서 돈다. 오래 사는 서버는 켜지 마라.
- 스캔·오피스 파일은 바이너리를 추측하지 말고 document_parse를 써라.
- 사용자 언어로 답해라. 끝나면 뭐가 바뀌었는지 한 줄로.
`, workdir)
	if strings.TrimSpace(extra) != "" {
		s += "\n프로젝트 지시:\n" + extra + "\n"
	}
	return s
}
