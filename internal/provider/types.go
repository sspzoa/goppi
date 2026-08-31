package provider

import (
	"context"
	"encoding/json"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role
	Content    string
	Reasoning  string  `json:"reasoning,omitempty"`
	Images     []Image `json:"images,omitempty"`
	ToolCalls  []ToolCall
	ToolCallID string
	ToolName   string
}

type Image struct {
	Path string `json:"path,omitempty"`
	MIME string `json:"mime,omitempty"`
	URL  string `json:"-"`
}

type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

type ToolSpec struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type Usage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ReasoningTokens int `json:"reasoning_tokens"`
}

type Delta struct {
	Reasoning string
	Content   string
}

type ChatRequest struct {
	Model           string
	System          string
	Messages        []Message
	Tools           []ToolSpec
	MaxTokens       int
	ReasoningEffort string
	PromptCacheKey  string
	OnDelta         func(Delta)
}

type ChatResponse struct {
	Message    Message
	StopReason string
	Usage      Usage
}

type Client interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
