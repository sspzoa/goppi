package provider

import (
	"context"
	"encoding/json"
	"fmt"
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
	Model             string         `json:"model"`
	Messages          []solarMessage `json:"messages"`
	Tools             []solarTool    `json:"tools,omitempty"`
	MaxTokens         int            `json:"max_tokens,omitempty"`
	ReasoningEffort   string         `json:"reasoning_effort,omitempty"`
	ToolChoice        string         `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool          `json:"parallel_tool_calls,omitempty"`
	PromptCacheKey    string         `json:"prompt_cache_key,omitempty"`
}

type solarMessage struct {
	Role       string      `json:"role"`
	Content    string      `json:"content,omitempty"`
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

type solarResp struct {
	Choices []struct {
		Message struct {
			Role      string      `json:"role"`
			Content   string      `json:"content"`
			Reasoning string      `json:"reasoning"`
			ToolCalls []solarCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		Details          struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Solar) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := solarReq{
		Model:          req.Model,
		Messages:       toSolarMessages(req.System, req.Messages),
		MaxTokens:      req.MaxTokens,
		PromptCacheKey: req.PromptCacheKey,
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

	data, err := c.API.PostJSON(ctx, "/chat/completions", body)
	if err != nil {
		return ChatResponse{}, err
	}
	var parsed solarResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ChatResponse{}, fmt.Errorf("upstage decode: %w", err)
	}
	if parsed.Error != nil {
		return ChatResponse{}, fmt.Errorf("upstage: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("upstage: empty choices")
	}

	choice := parsed.Choices[0]
	out := ChatResponse{
		StopReason: choice.FinishReason,
		Usage: Usage{
			InputTokens:     parsed.Usage.PromptTokens,
			OutputTokens:    parsed.Usage.CompletionTokens,
			ReasoningTokens: parsed.Usage.Details.ReasoningTokens,
		},
	}
	out.Message.Role = RoleAssistant
	out.Message.Content = strings.TrimSpace(choice.Message.Content)
	out.Message.Reasoning = strings.TrimSpace(choice.Message.Reasoning)
	for _, tc := range choice.Message.ToolCalls {
		out.Message.ToolCalls = append(out.Message.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
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
			out = append(out, solarMessage{Role: "user", Content: m.Content})
		case RoleAssistant:
			om := solarMessage{Role: "assistant", Content: m.Content}
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
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
				Name:       m.ToolName,
			})
		}
	}
	return out
}
