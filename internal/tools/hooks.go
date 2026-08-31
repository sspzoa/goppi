package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/sspzoa/goppi/internal/config"
)

const (
	hookTimeout   = 15 * time.Second
	hookOutputCap = 8 << 10
)

type hookEvent struct {
	Event     string          `json:"event"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Result    string          `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	WorkDir   string          `json:"workdir,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Reason    string          `json:"reason,omitempty"`
}

func matchHook(matcher, name string) bool {
	matcher = strings.TrimSpace(matcher)
	if matcher == "" || matcher == "*" {
		return true
	}
	if matcher == name {
		return true
	}
	if strings.HasSuffix(matcher, "*") && strings.HasPrefix(name, strings.TrimSuffix(matcher, "*")) {
		return true
	}
	ok, err := path.Match(matcher, name)
	return err == nil && ok
}

func runHooks(ctx context.Context, workdir, sandbox string, list []config.Hook, ev hookEvent, extraDirs ...string) (string, error) {
	if len(list) == 0 {
		return "", nil
	}
	ev.Input = json.RawMessage(RedactSecrets(string(ev.Input)))
	ev.Result = RedactSecrets(ev.Result)
	ev.Error = RedactSecrets(ev.Error)
	payload, err := json.Marshal(ev)
	if err != nil {
		return "", err
	}
	var extra strings.Builder
	for _, h := range list {
		if ev.Name != "" && !matchHook(h.Matcher, ev.Name) {
			continue
		}
		out, code, err := runHookCmd(ctx, workdir, sandbox, h.Command, payload, extraDirs...)
		if extra.Len() > 0 && out != "" {
			extra.WriteByte('\n')
		}
		extra.WriteString(out)
		if err != nil {
			return extra.String(), err
		}
		if ev.Event == "pre_tool" && code != 0 {
			if out == "" {
				out = fmt.Sprintf("hook exited %d", code)
			}
			return extra.String(), fmt.Errorf("hook denied %s: %s", ev.Name, strings.TrimSpace(out))
		}
	}
	return extra.String(), nil
}

func runHookCmd(ctx context.Context, workdir, sandbox, command string, stdin []byte, extraDirs ...string) (string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()
	cmd := exec.Command("bash", "-lc", command)
	cmd.Dir = workdir
	cmd.Env = scrubEnv(os.Environ())
	cmd.Stdin = bytes.NewReader(stdin)
	if err := wrapSandbox(cmd, workdir, sandbox, extraDirs...); err != nil {
		return "", -1, err
	}
	setKillGroup(cmd)
	guards := snapshotGitGuards(append([]string{workdir}, extraDirs...))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", -1, err
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	var err error
	select {
	case <-ctx.Done():
		killHook(cmd)
		err = <-wait
	case err = <-wait:
	}
	out := stdout.String()
	if stderr.Len() > 0 {
		if out != "" {
			out += "\n"
		}
		out += stderr.String()
	}
	if len(out) > hookOutputCap {
		out = out[:hookOutputCap] + "…"
	}
	if revert := revertGitGuards(guards); revert != nil {
		return out, -1, revert
	}
	if ctx.Err() != nil {
		return out, -1, fmt.Errorf("hook timed out")
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return out, ee.ExitCode(), nil
		}
		return out, -1, err
	}
	return out, 0, nil
}

func killHook(cmd *exec.Cmd) {
	killGroup(cmd)
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func hookSandbox(cfg config.Config) string {
	return normalizeSandbox(cfg.Sandbox)
}

func FireSessionStart(ctx context.Context, cfg config.Config, sessionID string) error {
	_, err := runHooks(ctx, cfg.WorkDir, hookSandbox(cfg), cfg.Hooks.SessionStart, hookEvent{
		Event:     "session_start",
		WorkDir:   cfg.WorkDir,
		SessionID: sessionID,
	}, cfg.ExtraDirs...)
	return err
}

func FireSessionEnd(ctx context.Context, cfg config.Config, sessionID, reason string) error {
	_, err := runHooks(ctx, cfg.WorkDir, hookSandbox(cfg), cfg.Hooks.SessionEnd, hookEvent{
		Event:     "session_end",
		WorkDir:   cfg.WorkDir,
		SessionID: sessionID,
		Reason:    reason,
	}, cfg.ExtraDirs...)
	return err
}
