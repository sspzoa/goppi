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
	ToolCalls  []ToolCall
	ToolCallID string
	ToolName   string
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
	InputTokens  int
	OutputTokens int
}

type ChatRequest struct {
	Model     string
	System    string
	Messages  []Message
	Tools     []ToolSpec
	MaxTokens int
}

type ChatResponse struct {
	Message    Message
	StopReason string
	Usage      Usage
}

type Client interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
