package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/instructions"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/tools"
	"github.com/sspzoa/goppi/internal/ui"
)

type Agent struct {
	Cfg       config.Config
	Client    provider.Client
	Tools     *tools.Registry
	Messages  []provider.Message
	SessionID string
	Quiet     bool
	LastUsage provider.Usage
}

func New(cfg config.Config, client provider.Client, registry *tools.Registry) *Agent {
	return &Agent{Cfg: cfg, Client: client, Tools: registry}
}

func (a *Agent) Reset() {
	a.Messages = nil
	a.SessionID = ""
}

func (a *Agent) Run(ctx context.Context, user string) error {
	a.Messages = append(a.Messages, provider.Message{Role: provider.RoleUser, Content: user})

	for turn := 0; turn < a.Cfg.MaxTurns; turn++ {
		stream := ui.NewStream()
		extra, _ := instructions.Load(a.Cfg.WorkDir)
		req := provider.ChatRequest{
			Model:           a.Cfg.Model,
			System:          systemPrompt(a.Cfg.WorkDir, extra),
			Messages:        a.Messages,
			Tools:           a.Tools.Specs(),
			MaxTokens:       a.Cfg.MaxTokens,
			ReasoningEffort: a.Cfg.ReasoningEffort,
			PromptCacheKey:  a.Cfg.PromptCacheKey,
		}
		if !a.Quiet {
			req.OnDelta = func(d provider.Delta) {
				stream.Write(d.Reasoning, d.Content)
			}
		}
		resp, err := a.Client.Chat(ctx, req)
		stream.Close()
		if err != nil {
			return err
		}
		a.Messages = append(a.Messages, resp.Message)
		a.LastUsage = resp.Usage
		if !a.Quiet {
			ui.Usage(resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.ReasoningTokens)
		}
		if len(resp.Message.ToolCalls) == 0 {
			return nil
		}
		for _, call := range resp.Message.ToolCalls {
			detail := toolDetail(call)
			if !a.Quiet {
				ui.ToolCall(call.Name, detail)
			}
			result, err := a.Tools.Run(ctx, call.Name, call.Input)
			msg := provider.Message{
				Role:       provider.RoleTool,
				ToolCallID: call.ID,
				ToolName:   call.Name,
			}
			if err != nil {
				if !a.Quiet {
					ui.ToolFail(err)
				}
				msg.Content = "error: " + err.Error()
			} else {
				if !a.Quiet {
					ui.ToolOK(summarize(result))
				}
				msg.Content = result
			}
			a.Messages = append(a.Messages, msg)
		}
	}
	return fmt.Errorf("stopped after %d turns", a.Cfg.MaxTurns)
}

func toolDetail(call provider.ToolCall) string {
	var raw map[string]any
	if err := json.Unmarshal(call.Input, &raw); err != nil {
		return strings.TrimSpace(string(call.Input))
	}
	switch call.Name {
	case "bash":
		if cmd, ok := raw["command"].(string); ok {
			return "$ " + cmd
		}
	case "read_file", "write_file", "edit_file", "document_parse", "document_ocr":
		if p, ok := raw["path"].(string); ok {
			return p
		}
	case "glob", "grep":
		if p, ok := raw["pattern"].(string); ok {
			return p
		}
	}
	b, _ := json.Marshal(raw)
	return string(b)
}

func summarize(s string) string {
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
