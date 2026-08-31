package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/sspzoa/goppi/internal/provider"
)

type DelegateFunc func(ctx context.Context, prompt string) (string, error)

type delegateTool struct {
	fn DelegateFunc
}

func (t delegateTool) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "delegate",
		Description: "Ask a read-only subagent to explore the repo and return a summary. Use for wide search or a second pass. The subagent cannot edit files, run bash, call MCP, or delegate again.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"prompt":{"type":"string","description":"What the subagent should investigate"}
			},
			"required":["prompt"]
		}`),
	}
}

func (t delegateTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	if t.fn == nil {
		return "", fmt.Errorf("delegate is not available")
	}
	args, err := decode[struct {
		Prompt string `json:"prompt"`
	}](input)
	if err != nil {
		return "", err
	}
	prompt := strings.TrimSpace(args.Prompt)
	if prompt == "" {
		return "", fmt.Errorf("empty prompt")
	}
	if utf8.RuneCountInString(prompt) > 8000 {
		return "", fmt.Errorf("delegate prompt too long")
	}
	return t.fn(ctx, prompt)
}

func (r *Registry) EnableDelegate(fn DelegateFunc) {
	if r == nil || fn == nil {
		return
	}
	if existing, ok := r.by["delegate"]; ok {
		if d, ok := existing.(delegateTool); ok {
			d.fn = fn
			r.by["delegate"] = d
			for i, t := range r.order {
				if t.Spec().Name == "delegate" {
					r.order[i] = d
					break
				}
			}
		}
		return
	}
	r.add(delegateTool{fn: fn})
}
