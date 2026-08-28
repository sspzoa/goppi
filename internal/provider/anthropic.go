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

type Anthropic struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
}

func NewAnthropic(apiKey, baseURL string) *Anthropic {
	return &Anthropic{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

type anthReq struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system,omitempty"`
	Messages  []anthMessage `json:"messages"`
	Tools     []anthTool    `json:"tools,omitempty"`
}

type anthMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthResp struct {
	Content    []anthBlock `json:"content"`
	StopReason string      `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (c *Anthropic) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := anthReq{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		System:    req.System,
		Messages:  toAnthMessages(req.Messages),
	}
	for _, t := range req.Tools {
		schema := t.Parameters
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		body.Tools = append(body.Tools, anthTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer httpResp.Body.Close()
	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return ChatResponse{}, err
	}

	var parsed anthResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ChatResponse{}, fmt.Errorf("anthropic decode: %w\n%s", err, truncate(data, 400))
	}
	if parsed.Error != nil {
		return ChatResponse{}, fmt.Errorf("anthropic: %s", parsed.Error.Message)
	}
	if httpResp.StatusCode >= 300 {
		return ChatResponse{}, fmt.Errorf("anthropic %s: %s", httpResp.Status, truncate(data, 400))
	}

	out := ChatResponse{
		StopReason: parsed.StopReason,
		Usage: Usage{
			InputTokens:  parsed.Usage.InputTokens,
			OutputTokens: parsed.Usage.OutputTokens,
		},
	}
	out.Message.Role = RoleAssistant
	var texts []string
	for _, b := range parsed.Content {
		switch b.Type {
		case "text":
			texts = append(texts, b.Text)
		case "tool_use":
			out.Message.ToolCalls = append(out.Message.ToolCalls, ToolCall{
				ID:    b.ID,
				Name:  b.Name,
				Input: b.Input,
			})
		}
	}
	out.Message.Content = strings.Join(texts, "\n")
	return out, nil
}

func toAnthMessages(msgs []Message) []anthMessage {
	var out []anthMessage
	var pendingTools []anthBlock

	flushTools := func() {
		if len(pendingTools) == 0 {
			return
		}
		out = append(out, anthMessage{Role: "user", Content: pendingTools})
		pendingTools = nil
	}

	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			flushTools()
			out = append(out, anthMessage{Role: "user", Content: m.Content})
		case RoleAssistant:
			flushTools()
			var blocks []anthBlock
			if strings.TrimSpace(m.Content) != "" {
				blocks = append(blocks, anthBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				input := tc.Input
				if len(input) == 0 {
					input = json.RawMessage(`{}`)
				}
				blocks = append(blocks, anthBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, anthBlock{Type: "text", Text: ""})
			}
			out = append(out, anthMessage{Role: "assistant", Content: blocks})
		case RoleTool:
			pendingTools = append(pendingTools, anthBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			})
		}
	}
	flushTools()
	return out
}

func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
