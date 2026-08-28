package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sspzoa/goppi/internal/provider"
)

type bashCmd struct{ workdir string }

func (bashCmd) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "bash",
		Description: "Run a shell command in the workdir. Use for git, builds, tests, and inspection. Do not use for long-running servers.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"command":{"type":"string"},
				"timeout_sec":{"type":"integer","description":"Defaults to 60"}
			},
			"required":["command"]
		}`),
	}
}

func (t bashCmd) Run(ctx context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Command    string `json:"command"`
		TimeoutSec int    `json:"timeout_sec"`
	}](input)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(args.Command) == "" {
		return "", fmt.Errorf("empty command")
	}
	sec := args.TimeoutSec
	if sec <= 0 {
		sec = 60
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(sec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-lc", args.Command)
	cmd.Dir = t.workdir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

	var b strings.Builder
	if stdout.Len() > 0 {
		b.Write(stdout.Bytes())
	}
	if stderr.Len() > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(stderr.String())
	}
	out := strings.TrimRight(b.String(), "\n")
	if len(out) > 80*1024 {
		out = out[:80*1024] + "\n… truncated"
	}
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("timed out after %ds", sec)
	}
	if err != nil {
		if out == "" {
			return "", err
		}
		return fmt.Sprintf("%s\n(%v)", out, err), nil
	}
	if out == "" {
		return "(ok, no output)", nil
	}
	return out, nil
}
