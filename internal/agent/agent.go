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
	s := fmt.Sprintf(`You are goppi (고삐), a local coding agent running on Upstage Solar.
You work on the user's machine and use tools to inspect and change files.

Working directory: %s

Upstage tools:
- document_parse: PDF/HWP/Office/images → Markdown with layout. Default for documents.
- document_ocr: plain text only, when layout does not matter.

Rules:
- Prefer doing the work over describing a plan.
- Read before you edit. Keep diffs small and exact.
- edit_file must match exactly one occurrence; widen or shrink old_string if it is not unique.
- bash runs in the working directory. Do not start long-lived servers.
- For scans and office files, use document_parse instead of guessing from binary bytes.
- Reply in the user's language. Be concise. When you finish, say what changed.
`, workdir)
	if strings.TrimSpace(extra) != "" {
		s += "\nProject instructions:\n" + extra + "\n"
	}
	return s
}
