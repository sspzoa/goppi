package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/sspzoa/goppi/internal/provider"
)

type AskUser func(question string, options []string) (string, error)

const (
	maxAskOptions  = 8
	maxAskQuestion = 500
	maxAskOption   = 80
)

type askUserTool struct{ reg *Registry }

func (askUserTool) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "ask_user",
		Description: "Ask the human a clarifying question. Prefer options when the answer is a choice. Do not use this to narrate; only when you cannot proceed.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"question":{"type":"string","description":"Question in the user's language"},
				"options":{"type":"array","items":{"type":"string"},"description":"Up to 8 choices. Omit for yes/no or a short free answer"}
			},
			"required":["question"]
		}`),
	}
}

func (t askUserTool) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}](input)
	if err != nil {
		return "", err
	}
	q := strings.TrimSpace(args.Question)
	if q == "" {
		return "", fmt.Errorf("empty question")
	}
	if utf8.RuneCountInString(q) > maxAskQuestion {
		q = string([]rune(q)[:maxAskQuestion])
	}
	opts := clipAskOptions(args.Options)
	if t.reg == nil || t.reg.askUser == nil {
		return "", fmt.Errorf("no user to ask (headless). Decide and continue")
	}
	ans, err := t.reg.askUser(q, opts)
	if err != nil {
		return "", err
	}
	ans = strings.TrimSpace(ans)
	if ans == "" {
		return "", fmt.Errorf("user skipped")
	}
	return "user: " + ans, nil
}

func clipAskOptions(in []string) []string {
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if utf8.RuneCountInString(s) > maxAskOption {
			s = string([]rune(s)[:maxAskOption])
		}
		out = append(out, s)
		if len(out) >= maxAskOptions {
			break
		}
	}
	return out
}

func (r *Registry) SetAskUser(fn AskUser) {
	if r != nil {
		r.askUser = fn
	}
}
