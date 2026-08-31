package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/skills"
)

type readSkill struct{ list []skills.Skill }

func (readSkill) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "read_skill",
		Description: "Load a project skill (SKILL.md) by name. Use when the system prompt lists a skill that matches the task.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"name":{"type":"string"}
			},
			"required":["name"]
		}`),
	}
}

func (t *readSkill) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Name string `json:"name"`
	}](input)
	if err != nil {
		return "", err
	}
	s, ok := skills.Lookup(t.list, args.Name)
	if !ok {
		return "", fmt.Errorf("unknown skill %q", args.Name)
	}
	return s.Body, nil
}
