package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
)

type bashCmd struct {
	workdir string
	root    *fileRoot
	sandbox string
	jobs    *jobHub
}

func (bashCmd) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "bash",
		Description: "Run a shell command in the workdir. sandbox=workspace confines writes to workdir/temp/cache. sandbox=strict also blocks network. background=true starts a job and returns immediately (servers). Then bash_poll / bash_kill.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"command":{"type":"string"},
				"timeout_sec":{"type":"integer","description":"Defaults to 60, maximum 300. Ignored when background is true"},
				"background":{"type":"boolean","description":"Start and return a job id. Turn cancel does not kill it"}
			},
			"required":["command"]
		}`),
	}
}

func (t bashCmd) Run(ctx context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Command    string `json:"command"`
		TimeoutSec int    `json:"timeout_sec"`
		Background bool   `json:"background"`
	}](input)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(args.Command) == "" {
		return "", fmt.Errorf("empty command")
	}
	cmd, err := t.prepare(args.Command)
	if err != nil {
		return "", err
	}
	guards := snapshotGitGuards(append([]string{t.workdir}, t.root.Extra()...))
	if args.Background {
		id, err := t.jobs.start(cmd, func() error { return revertGitGuards(guards) })
		if err != nil {
			return "", err
		}
		pid := 0
		if cmd.Process != nil {
			pid = cmd.Process.Pid
		}
		return fmt.Sprintf("started job %d (pid %d). use bash_poll / bash_kill.", id, pid), nil
	}
	sec := args.TimeoutSec
	if sec <= 0 {
		sec = 60
	}
	if sec > 300 {
		sec = 300
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(sec)*time.Second)
	defer cancel()
	setKillGroup(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	var waitErr error
	select {
	case <-ctx.Done():
		killGroup(cmd)
		waitErr = <-wait
	case waitErr = <-wait:
	}
	err = waitErr

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
	var runErr error
	if ctx.Err() == context.DeadlineExceeded {
		runErr = fmt.Errorf("timed out after %ds", sec)
	} else {
		runErr = err
	}
	return finishBash(out, runErr, revertGitGuards(guards))
}

func (t bashCmd) prepare(command string) (*exec.Cmd, error) {
	cmd := exec.Command("bash", "-lc", command)
	cmd.Dir = t.workdir
	cmd.Env = scrubEnv(os.Environ())
	if err := wrapSandbox(cmd, t.workdir, t.sandbox, t.root.Extra()...); err != nil {
		return nil, err
	}
	setKillGroup(cmd)
	return cmd, nil
}

func finishBash(out string, runErr, revert error) (string, error) {
	if revert != nil {
		if out == "" {
			return "", revert
		}
		return out, revert
	}
	if runErr != nil {
		if strings.HasPrefix(runErr.Error(), "timed out") {
			return out, runErr
		}
		if out == "" {
			return "", runErr
		}
		return fmt.Sprintf("%s\n(%v)", out, runErr), nil
	}
	if out == "" {
		return "(ok, no output)", nil
	}
	return out, nil
}

type bashPoll struct{ jobs *jobHub }

func (bashPoll) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "bash_poll",
		Description: "Read output of a background bash job. Omit id to list jobs. wait_sec waits for exit or more output (max 30).",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"integer","description":"Job id from bash background=true. 0 lists all"},
				"wait_sec":{"type":"integer","description":"Seconds to wait, default 0"}
			}
		}`),
	}
}

func (t bashPoll) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		ID      int `json:"id"`
		WaitSec int `json:"wait_sec"`
	}](input)
	if err != nil {
		if len(bytes.TrimSpace(input)) == 0 || string(input) == "{}" {
			return t.jobs.poll(0, 0)
		}
		return "", err
	}
	if args.WaitSec < 0 {
		args.WaitSec = 0
	}
	return t.jobs.poll(args.ID, args.WaitSec)
}

type bashKill struct{ jobs *jobHub }

func (bashKill) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "bash_kill",
		Description: "Stop a background bash job and its process group.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"integer","description":"Job id from bash background=true"}
			},
			"required":["id"]
		}`),
	}
}

func (t bashKill) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		ID int `json:"id"`
	}](input)
	if err != nil {
		return "", err
	}
	if args.ID <= 0 {
		return "", fmt.Errorf("id is required")
	}
	return t.jobs.kill(args.ID)
}

// scrubEnv drops credentials so a model-chosen command cannot dump them
// with env/printenv. PATH, HOME, and the rest of the user environment stay.
func scrubEnv(env []string) []string { return config.ScrubEnv(env) }
