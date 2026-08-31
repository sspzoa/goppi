package provider

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sspzoa/goppi/internal/upstage"
)

type Solar struct {
	API *upstage.Client
}

func NewSolar(api *upstage.Client) *Solar {
	return &Solar{API: api}
}

type solarReq struct {
	Model             string           `json:"model"`
	Messages          []solarMessage   `json:"messages"`
	Tools             []solarTool      `json:"tools,omitempty"`
	MaxTokens         int              `json:"max_tokens,omitempty"`
	ReasoningEffort   string           `json:"reasoning_effort,omitempty"`
	ToolChoice        string           `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool            `json:"parallel_tool_calls,omitempty"`
	PromptCacheKey    string           `json:"prompt_cache_key,omitempty"`
	Stream            bool             `json:"stream"`
	StreamOptions     *solarStreamOpts `json:"stream_options,omitempty"`
}

type solarStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type solarMessage struct {
	Role       string      `json:"role"`
	Content    any         `json:"content,omitempty"`
	ToolCalls  []solarCall `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

type solarTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type solarCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (c *Solar) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := solarReq{
		Model:          req.Model,
		Messages:       toSolarMessages(req.System, req.Messages),
		MaxTokens:      req.MaxTokens,
		PromptCacheKey: req.PromptCacheKey,
		Stream:         true,
		StreamOptions:  &solarStreamOpts{IncludeUsage: true},
	}
	if upstage.SupportsReasoning(req.Model) && req.ReasoningEffort != "" {
		body.ReasoningEffort = req.ReasoningEffort
	}
	if len(req.Tools) > 0 {
		body.ToolChoice = "auto"
		yes := true
		body.ParallelToolCalls = &yes
	}
	for _, t := range req.Tools {
		schema := t.Parameters
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		item := solarTool{Type: "function"}
		item.Function.Name = t.Name
		item.Function.Description = t.Description
		item.Function.Parameters = schema
		body.Tools = append(body.Tools, item)
	}

	rc, err := c.API.PostJSONStream(ctx, "/chat/completions", body)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "stream_options") {
		body.StreamOptions = nil
		rc, err = c.API.PostJSONStream(ctx, "/chat/completions", body)
	}
	if err != nil {
		return ChatResponse{}, err
	}
	defer rc.Close()

	var acc streamAcc
	err = readSSE(rc, func(raw []byte) error {
		var beforeReason, beforeContent int
		beforeReason = acc.reasoning.Len()
		beforeContent = acc.content.Len()
		if err := acc.apply(raw); err != nil {
			return err
		}
		if req.OnDelta == nil {
			return nil
		}
		d := Delta{}
		if acc.reasoning.Len() > beforeReason {
			d.Reasoning = acc.reasoning.String()[beforeReason:]
		}
		if acc.content.Len() > beforeContent {
			d.Content = acc.content.String()[beforeContent:]
		}
		if d.Reasoning != "" || d.Content != "" {
			req.OnDelta(d)
		}
		return nil
	})
	if err != nil {
		return ChatResponse{}, err
	}
	out := acc.response()
	if out.Message.Role == "" {
		out.Message.Role = RoleAssistant
	}
	return out, nil
}

func toSolarMessages(system string, msgs []Message) []solarMessage {
	var out []solarMessage
	if strings.TrimSpace(system) != "" {
		out = append(out, solarMessage{Role: "system", Content: system})
	}
	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			out = append(out, solarMessage{Role: "user", Content: messageContent(m)})
		case RoleAssistant:
			om := solarMessage{Role: "assistant", Content: messageContent(m)}
			for _, tc := range m.ToolCalls {
				call := solarCall{ID: tc.ID, Type: "function"}
				call.Function.Name = tc.Name
				call.Function.Arguments = string(tc.Input)
				if call.Function.Arguments == "" {
					call.Function.Arguments = "{}"
				}
				om.ToolCalls = append(om.ToolCalls, call)
			}
			out = append(out, om)
		case RoleTool:
			out = append(out, solarMessage{
				Role:       "tool",
				Content:    messageContent(m),
				ToolCallID: m.ToolCallID,
				Name:       m.ToolName,
			})
		}
	}
	return out
}

func messageContent(m Message) any {
	if len(m.Images) == 0 {
		return m.Content
	}
	var parts []map[string]any
	if strings.TrimSpace(m.Content) != "" {
		parts = append(parts, map[string]any{"type": "text", "text": m.Content})
	}
	for _, img := range m.Images {
		if img.URL == "" {
			continue
		}
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": img.URL},
		})
	}
	if len(parts) == 0 {
		return m.Content
	}
	return parts
}
