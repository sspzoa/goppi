package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/sspzoa/goppi/internal/config"
)

type CmdHook func(*exec.Cmd) error

type Session struct {
	Name   string
	Tools  []Tool
	Conn   *Conn
	cmd    *exec.Cmd
	errOut *errCap
}

func Start(ctx context.Context, name string, spec config.MCPServer, workdir, version string, hook CmdHook) (*Session, error) {
	if strings.TrimSpace(spec.Command) == "" {
		return nil, fmt.Errorf("mcp %s: empty command", name)
	}
	args := spec.Args
	cmd := exec.Command(spec.Command, args...)
	cmd.Dir = workdir
	cmd.Env = withExtraEnv(cleanEnv(os.Environ()), spec.Env)
	errOut := &errCap{}
	cmd.Stderr = errOut
	setKillGroup(cmd)
	if hook != nil {
		if err := hook(cmd); err != nil {
			return nil, fmt.Errorf("mcp %s: sandbox: %w", name, err)
		}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp %s: start: %w", name, err)
	}
	conn := NewConn(stdout, stdin)
	initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := conn.Initialize(initCtx, "goppi", version); err != nil {
		conn.Close()
		killGroup(cmd)
		_ = cmd.Wait()
		return nil, fmt.Errorf("mcp %s: initialize: %w", name, err)
	}
	listCtx, cancelList := context.WithTimeout(ctx, 15*time.Second)
	defer cancelList()
	tools, err := conn.ListTools(listCtx)
	if err != nil {
		conn.Close()
		killGroup(cmd)
		_ = cmd.Wait()
		return nil, fmt.Errorf("mcp %s: tools/list: %w", name, err)
	}
	if len(tools) > 32 {
		tools = tools[:32]
	}
	return &Session{Name: name, Tools: tools, Conn: conn, cmd: cmd, errOut: errOut}, nil
}

func (s *Session) Stderr() string {
	if s == nil {
		return ""
	}
	return s.errOut.String()
}

func (s *Session) Close() {
	if s == nil {
		return
	}
	if s.Conn != nil {
		s.Conn.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		killGroup(s.cmd)
		_ = s.cmd.Wait()
	}
}

func StartAll(ctx context.Context, servers map[string]config.MCPServer, workdir, version string, hook CmdHook) ([]*Session, []error) {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []*Session
	var errs []error
	for i, name := range names {
		if i >= 8 {
			errs = append(errs, fmt.Errorf("mcp: more than 8 servers, extra ignored"))
			break
		}
		spec := servers[name]
		s, err := Start(ctx, name, spec, workdir, version, hook)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, s)
	}
	return out, errs
}

func withExtraEnv(base []string, extra map[string]string) []string {
	out := append([]string{}, base...)
	for k, v := range extra {
		if k == "" {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

func cleanEnv(env []string) []string { return config.ScrubEnv(env) }

func ToolName(server, tool string) string {
	return "mcp_" + sanitize(server) + "_" + sanitize(tool)
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "tool"
	}
	return out
}
