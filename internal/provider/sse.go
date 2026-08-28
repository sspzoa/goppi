package provider

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type solarChunk struct {
	Choices []struct {
		Delta struct {
			Content   string           `json:"content"`
			Reasoning string           `json:"reasoning"`
			ToolCalls []solarDeltaCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		Details          struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type solarDeltaCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type streamAcc struct {
	content   strings.Builder
	reasoning strings.Builder
	stop      string
	usage     Usage
	calls     map[int]*accCall
}

type accCall struct {
	id, name string
	args     strings.Builder
}

func readSSE(r io.Reader, handle func([]byte) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 2<<20)
	var data []string
	flush := func() error {
		if len(data) == 0 {
			return nil
		}
		payload := strings.TrimSpace(strings.Join(data, "\n"))
		data = nil
		if payload == "" || payload == "[DONE]" {
			return nil
		}
		return handle([]byte(payload))
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				return flush()
			}
			data = append(data, payload)
		}
	}
	if err := flush(); err != nil {
		return err
	}
	return sc.Err()
}

func (a *streamAcc) apply(raw []byte) error {
	var chunk solarChunk
	if err := json.Unmarshal(raw, &chunk); err != nil {
		return fmt.Errorf("upstage stream decode: %w\n%s", err, truncate(raw, 240))
	}
	if chunk.Error != nil {
		return fmt.Errorf("upstage: %s", chunk.Error.Message)
	}
	if chunk.Usage != nil {
		a.usage = Usage{
			InputTokens:     chunk.Usage.PromptTokens,
			OutputTokens:    chunk.Usage.CompletionTokens,
			ReasoningTokens: chunk.Usage.Details.ReasoningTokens,
		}
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	ch := chunk.Choices[0]
	if ch.FinishReason != "" {
		a.stop = ch.FinishReason
	}
	if ch.Delta.Reasoning != "" {
		a.reasoning.WriteString(ch.Delta.Reasoning)
	}
	if ch.Delta.Content != "" {
		a.content.WriteString(ch.Delta.Content)
	}
	for _, tc := range ch.Delta.ToolCalls {
		if a.calls == nil {
			a.calls = map[int]*accCall{}
		}
		call := a.calls[tc.Index]
		if call == nil {
			call = &accCall{}
			a.calls[tc.Index] = call
		}
		if tc.ID != "" {
			call.id = tc.ID
		}
		if tc.Function.Name != "" {
			call.name = tc.Function.Name
		}
		call.args.WriteString(tc.Function.Arguments)
	}
	return nil
}

func (a *streamAcc) response() ChatResponse {
	out := ChatResponse{
		StopReason: a.stop,
		Usage:      a.usage,
	}
	out.Message.Role = RoleAssistant
	out.Message.Content = strings.TrimSpace(a.content.String())
	out.Message.Reasoning = strings.TrimSpace(a.reasoning.String())
	if len(a.calls) == 0 {
		return out
	}
	for i := 0; i < len(a.calls); i++ {
		call, ok := a.calls[i]
		if !ok {
			continue
		}
		args := strings.TrimSpace(call.args.String())
		if args == "" {
			args = "{}"
		}
		out.Message.ToolCalls = append(out.Message.ToolCalls, ToolCall{
			ID:    call.id,
			Name:  call.name,
			Input: json.RawMessage(args),
		})
	}
	return out
}

func truncate(b []byte, n int) string {
	b = bytes.TrimSpace(b)
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
