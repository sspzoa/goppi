package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAI struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
}

func NewOpenAI(apiKey, baseURL string) *OpenAI {
	return &OpenAI{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

type oaiReq struct {
	Model     string       `json:"model"`
	Messages  []oaiMessage `json:"messages"`
	Tools     []oaiTool    `json:"tools,omitempty"`
	MaxTokens int          `json:"max_tokens,omitempty"`
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	Name       string        `json:"name,omitempty"`
}

type oaiTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiResp struct {
	Choices []struct {
		Message      oaiMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *OpenAI) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := oaiReq{
		Model:     req.Model,
		Messages:  toOAIMessages(req.System, req.Messages),
		MaxTokens: req.MaxTokens,
	}
	for _, t := range req.Tools {
		schema := t.Parameters
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		item := oaiTool{Type: "function"}
		item.Function.Name = t.Name
		item.Function.Description = t.Description
		item.Function.Parameters = schema
		body.Tools = append(body.Tools, item)
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("authorization", "Bearer "+c.APIKey)

	httpResp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer httpResp.Body.Close()
	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return ChatResponse{}, err
	}

	var parsed oaiResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ChatResponse{}, fmt.Errorf("openai decode: %w\n%s", err, truncate(data, 400))
	}
	if parsed.Error != nil {
		return ChatResponse{}, fmt.Errorf("openai: %s", parsed.Error.Message)
	}
	if httpResp.StatusCode >= 300 {
		return ChatResponse{}, fmt.Errorf("openai %s: %s", httpResp.Status, truncate(data, 400))
	}
	if len(parsed.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("openai: empty choices")
	}

	choice := parsed.Choices[0]
	out := ChatResponse{
		StopReason: choice.FinishReason,
		Usage: Usage{
			InputTokens:  parsed.Usage.PromptTokens,
			OutputTokens: parsed.Usage.CompletionTokens,
		},
	}
	out.Message.Role = RoleAssistant
	out.Message.Content = choice.Message.Content
	for _, tc := range choice.Message.ToolCalls {
		out.Message.ToolCalls = append(out.Message.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}
	return out, nil
}

func toOAIMessages(system string, msgs []Message) []oaiMessage {
	var out []oaiMessage
	if strings.TrimSpace(system) != "" {
		out = append(out, oaiMessage{Role: "system", Content: system})
	}
	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			out = append(out, oaiMessage{Role: "user", Content: m.Content})
		case RoleAssistant:
			om := oaiMessage{Role: "assistant", Content: m.Content}
			for _, tc := range m.ToolCalls {
				call := oaiToolCall{ID: tc.ID, Type: "function"}
				call.Function.Name = tc.Name
				call.Function.Arguments = string(tc.Input)
				if call.Function.Arguments == "" {
					call.Function.Arguments = "{}"
				}
				om.ToolCalls = append(om.ToolCalls, call)
			}
			out = append(out, om)
		case RoleTool:
			out = append(out, oaiMessage{
				Role:       "tool",
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
				Name:       m.ToolName,
			})
		}
	}
	return out
}
