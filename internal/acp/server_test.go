package acp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/tools"
)

type scriptClient struct {
	content string
}

func (s scriptClient) Chat(_ context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if req.OnDelta != nil {
		req.OnDelta(provider.Delta{Content: s.content})
	}
	return provider.ChatResponse{Message: provider.Message{Role: provider.RoleAssistant, Content: s.content}}, nil
}

func TestServeInitializeNewPrompt(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })

	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			reg := tools.New(cfg.WorkDir, nil, nil)
			return agent.New(cfg, scriptClient{content: "hello acp"}, reg), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()

	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	init := readRPC(t, cli)
	if !strings.Contains(string(init), `"protocolVersion":1`) || !strings.Contains(string(init), `"loadSession":true`) || !strings.Contains(string(init), `"image":true`) || !strings.Contains(string(init), `"embeddedContext":true`) || !strings.Contains(string(init), `"sessionCapabilities"`) || !strings.Contains(string(init), `"delete"`) || !strings.Contains(string(init), `"resume"`) || !strings.Contains(string(init), `"additionalDirectories"`) {
		t.Fatalf("initialize %s", init)
	}

	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir, "mcpServers": []any{}})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil || newResp.Result.SessionID == "" {
		t.Fatalf("session/new %s", created)
	}
	if !strings.Contains(string(created), `"currentModeId":"act"`) || !strings.Contains(string(created), `"configOptions"`) {
		t.Fatalf("session/new missing modes: %s", created)
	}

	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "hi"}},
	})
	var sawText, sawStop bool
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !(sawText && sawStop) {
		raw := readRPC(t, cli)
		if strings.Contains(string(raw), "hello acp") {
			sawText = true
		}
		if strings.Contains(string(raw), `"stopReason":"end_turn"`) {
			sawStop = true
		}
	}
	if !sawText || !sawStop {
		t.Fatalf("text=%v stop=%v", sawText, sawStop)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionNewPassesAdditionalDirs(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = fromCli.Close(); _ = fromSrv.Close() })
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	extra := t.TempDir()
	var got config.Config
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Ctx: ctx,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			got = cfg
			reg := tools.New(cfg.WorkDir, nil, nil)
			reg.SetExtraDirs(cfg.ExtraDirs)
			return agent.New(cfg, scriptClient{content: "ok"}, reg), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{
		"cwd":                   cfg.WorkDir,
		"additionalDirectories": []string{extra},
	})
	created := readRPC(t, cli)
	if !strings.Contains(string(created), "additionalDirectories") {
		t.Fatalf("state %s", created)
	}
	if len(got.ExtraDirs) != 1 {
		t.Fatalf("extra %v", got.ExtraDirs)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionNewPassesClientMCP(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = fromCli.Close(); _ = fromSrv.Close() })
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	cfg.MCPServers = map[string]config.MCPServer{"user": {Command: "user-mcp"}}
	var got config.Config
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Ctx: ctx,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			got = cfg
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{
		"cwd": cfg.WorkDir,
		"mcpServers": []any{
			map[string]any{"name": "fs", "command": "npx", "args": []any{"-y", "mcp-fs"}},
			map[string]any{"name": "remote", "type": "http", "url": "https://example.com/mcp"},
		},
	})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil || newResp.Result.SessionID == "" {
		t.Fatal(created)
	}
	if got.MCPServers["fs"].Command != "npx" || got.MCPServers["user"].Command != "user-mcp" {
		t.Fatalf("merged %+v", got.MCPServers)
	}
	if _, ok := got.MCPServers["remote"]; ok {
		t.Fatal("http MCP should be skipped")
	}
	if _, ok := srv.Cfg.MCPServers["fs"]; ok {
		t.Fatal("client MCP must not stick on the server config")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionNewRejectsRootCwd(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = fromCli.Close(); _ = fromSrv.Close() })
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Ctx: ctx,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": string(os.PathSeparator)})
	if !waitErrorID(t, cli, 2, "filesystem root") {
		t.Fatal("root cwd should be rejected")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionNewRejectsRelativeCwd(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = fromCli.Close(); _ = fromSrv.Close() })
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Ctx: ctx,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": ".."})
	if !waitErrorID(t, cli, 2, "absolute path") {
		t.Fatal("relative cwd should be rejected")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestAuthenticateSucceedsWithoutAuthMethods(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = fromCli.Close(); _ = fromSrv.Close() })
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Ctx: ctx,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "authenticate", map[string]any{"methodId": "ignored"})
	raw := readRPC(t, cli)
	if strings.Contains(string(raw), `"error"`) && !strings.Contains(string(raw), `"error":null`) {
		t.Fatalf("authenticate %s", raw)
	}
	if !strings.Contains(string(raw), `"result"`) {
		t.Fatalf("authenticate missing result %s", raw)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionSetModeAndConfig(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })

	var a *agent.Agent
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			reg := tools.New(cfg.WorkDir, nil, nil)
			a = agent.New(cfg, scriptClient{content: "ok"}, reg)
			return a, nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()

	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil {
		t.Fatal(err)
	}
	writeRPC(t, fromCli, 3, "session/set_mode", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"modeId":    "plan",
	})
	var sawMode bool
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw := readRPC(t, cli)
		if strings.Contains(string(raw), `"currentModeId":"plan"`) {
			sawMode = true
			break
		}
	}
	if !sawMode || a.Cfg.Mode != "plan" {
		t.Fatalf("mode plan: agent=%q saw=%v", a.Cfg.Mode, sawMode)
	}
	writeRPC(t, fromCli, 4, "session/set_config_option", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"configId":  "model",
		"value":     "solar-mini",
	})
	var sawModel bool
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw := readRPC(t, cli)
		if strings.Contains(string(raw), `"solar-mini"`) {
			sawModel = true
			break
		}
	}
	if !sawModel || a.Cfg.Model != "solar-mini" {
		t.Fatalf("model: agent=%q saw=%v", a.Cfg.Model, sawModel)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionListAndClose(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })

	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			reg := tools.New(cfg.WorkDir, nil, nil)
			return agent.New(cfg, scriptClient{content: "ok"}, reg), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()

	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil || newResp.Result.SessionID == "" {
		t.Fatal(created)
	}
	writeRPC(t, fromCli, 3, "session/list", map[string]any{"cwd": cfg.WorkDir})
	listed := readRPC(t, cli)
	if !strings.Contains(string(listed), newResp.Result.SessionID) {
		t.Fatalf("list missing session: %s", listed)
	}
	writeRPC(t, fromCli, 4, "session/close", map[string]any{"sessionId": newResp.Result.SessionID})
	closed := readRPC(t, cli)
	var closeMsg struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(closed, &closeMsg); err != nil || closeMsg.Error != nil {
		t.Fatalf("close %s", closed)
	}
	writeRPC(t, fromCli, 5, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "hi"}},
	})
	denied := readRPC(t, cli)
	if !strings.Contains(string(denied), "unknown session") {
		t.Fatalf("prompt after close: %s", denied)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionListFromDisk(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	cfg := config.Default()
	cfg.WorkDir = wd
	id, err := session.Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "listed later"}})
	if err != nil {
		t.Fatal(err)
	}
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/list", map[string]any{})
	listed := readRPC(t, cli)
	if !strings.Contains(string(listed), id) || !strings.Contains(string(listed), "listed later") {
		t.Fatalf("disk list: %s", listed)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionListRedactsTitle(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	cfg := config.Default()
	cfg.WorkDir = wd
	const secret = "sk-abcdefghijklmnopqrst"
	id, err := session.Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "use " + secret}})
	if err != nil {
		t.Fatal(err)
	}
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/list", map[string]any{})
	listed := string(readRPC(t, cli))
	if !strings.Contains(listed, id) {
		t.Fatalf("list missing session: %s", listed)
	}
	if strings.Contains(listed, secret) {
		t.Fatalf("list leaked title: %s", listed)
	}
	if !strings.Contains(listed, "[redacted]") {
		t.Fatalf("list missing redaction: %s", listed)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionDeleteRemovesFromList(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	cfg := config.Default()
	cfg.WorkDir = wd
	id, err := session.Persist(cfg, "", []provider.Message{{Role: provider.RoleUser, Content: "delete me"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := session.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	path, err := session.WriteMarkdown(f)
	if err != nil {
		t.Fatal(err)
	}
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/list", map[string]any{})
	listed := readRPC(t, cli)
	if !strings.Contains(string(listed), id) {
		t.Fatalf("list missing session: %s", listed)
	}
	writeRPC(t, fromCli, 3, "session/delete", map[string]any{"sessionId": id})
	deleted := readRPC(t, cli)
	var delMsg struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(deleted, &delMsg); err != nil || delMsg.Error != nil {
		t.Fatalf("delete %s", deleted)
	}
	if _, err := session.Load(id); err == nil {
		t.Fatal("session still on disk")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("export still on disk")
	}
	writeRPC(t, fromCli, 4, "session/list", map[string]any{})
	listed = readRPC(t, cli)
	if strings.Contains(string(listed), id) {
		t.Fatalf("list still has session: %s", listed)
	}
	writeRPC(t, fromCli, 5, "session/delete", map[string]any{"sessionId": id})
	again := readRPC(t, cli)
	if err := json.Unmarshal(again, &delMsg); err != nil || delMsg.Error != nil {
		t.Fatalf("idempotent delete %s", again)
	}
	writeRPC(t, fromCli, 6, "session/delete", map[string]any{"sessionId": "not-a-session"})
	ghost := readRPC(t, cli)
	if err := json.Unmarshal(ghost, &delMsg); err != nil || delMsg.Error != nil {
		t.Fatalf("unknown delete %s", ghost)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionDeleteClosesActive(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil || newResp.Result.SessionID == "" {
		t.Fatal(created)
	}
	writeRPC(t, fromCli, 3, "session/delete", map[string]any{"sessionId": newResp.Result.SessionID})
	deleted := readRPC(t, cli)
	var delMsg struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(deleted, &delMsg); err != nil || delMsg.Error != nil {
		t.Fatalf("delete %s", deleted)
	}
	if srv.getSession(newResp.Result.SessionID) != nil {
		t.Fatal("session still in memory")
	}
	writeRPC(t, fromCli, 4, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "hi"}},
	})
	denied := readRPC(t, cli)
	if !strings.Contains(string(denied), "unknown session") {
		t.Fatalf("prompt after delete: %s", denied)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestReplayHistoryRedactsSecrets(t *testing.T) {
	const secret = "sk-abcdefghijklmnopqrst"
	var buf bytes.Buffer
	srv := &Server{conn: newConn(bytes.NewReader(nil), &buf)}
	srv.replayHistory(&agent.Agent{
		SessionID: "aaaaaaaaaaaaaaaa",
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "key " + secret},
			{Role: provider.RoleAssistant, Content: "use " + secret, Reasoning: "think " + secret},
		},
	})
	out := buf.String()
	if strings.Contains(out, secret) {
		t.Fatalf("replay leaked: %s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("replay missing redaction: %s", out)
	}
}

func TestAcpSinkRedactsSecrets(t *testing.T) {
	const secret = "sk-abcdefghijklmnopqrst"
	var buf bytes.Buffer
	sink := acpSink{conn: newConn(bytes.NewReader(nil), &buf), sessionID: "aaaaaaaaaaaaaaaa"}
	sink.Delta("think "+secret, "say "+secret)
	sink.ToolStart("bash", "$ echo "+secret)
	sink.ToolDone("out "+secret, fmt.Errorf("err "+secret))
	out := buf.String()
	if strings.Contains(out, secret) {
		t.Fatalf("sink leaked: %s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("sink missing redaction: %s", out)
	}
}

func TestAskRedactsDetail(t *testing.T) {
	const secret = "sk-abcdefghijklmnopqrst"
	cliIn, srvOut := io.Pipe()
	srvIn, cliOut := io.Pipe()
	t.Cleanup(func() { _ = cliIn.Close(); _ = srvOut.Close(); _ = srvIn.Close(); _ = cliOut.Close() })
	srv := &Server{conn: newConn(srvIn, srvOut)}
	cli := newConn(cliIn, cliOut)
	go func() {
		for {
			if _, err := srv.conn.read(); err != nil {
				return
			}
		}
	}()
	got := make(chan []byte, 1)
	go func() {
		msg, err := cli.read()
		if err != nil {
			got <- []byte(err.Error())
			return
		}
		raw, err := json.Marshal(msg)
		if err != nil {
			got <- []byte(err.Error())
			return
		}
		_ = cli.reply(msg.ID, map[string]any{
			"outcome": map[string]string{"outcome": "selected", "optionId": "reject-once"},
		})
		got <- raw
	}()
	if v := srv.ask("aaaaaaaaaaaaaaaa", "bash", "$ echo "+secret); v != tools.Denied {
		t.Fatalf("verdict %v", v)
	}
	raw := string(<-got)
	if strings.Contains(raw, secret) {
		t.Fatalf("permission leaked: %s", raw)
	}
	if !strings.Contains(raw, "[redacted]") {
		t.Fatalf("permission missing redaction: %s", raw)
	}
}

func TestSessionLoadReplaysHistory(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	cfg := config.Default()
	cfg.WorkDir = wd
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "what is paris"},
		{Role: provider.RoleAssistant, Content: "a city", Reasoning: "think"},
	})
	if err != nil {
		t.Fatal(err)
	}
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/load", map[string]any{"sessionId": id, "cwd": wd})
	var sawUser, sawAgent, sawThought, sawResult bool
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !(sawUser && sawAgent && sawThought && sawResult) {
		raw := readRPC(t, cli)
		s := string(raw)
		if strings.Contains(s, "user_message_chunk") && strings.Contains(s, "what is paris") {
			sawUser = true
		}
		if strings.Contains(s, "agent_message_chunk") && strings.Contains(s, "a city") {
			sawAgent = true
		}
		if strings.Contains(s, "agent_thought_chunk") && strings.Contains(s, "think") {
			sawThought = true
		}
		if strings.Contains(s, `"sessionId":"`+id+`"`) && strings.Contains(s, `"result"`) && strings.Contains(s, `"modes"`) {
			sawResult = true
		}
	}
	if !sawUser || !sawAgent || !sawThought || !sawResult {
		t.Fatalf("replay user=%v agent=%v thought=%v result=%v", sawUser, sawAgent, sawThought, sawResult)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionResumeDoesNotReplayHistory(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	cfg := config.Default()
	cfg.WorkDir = wd
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "what is paris"},
		{Role: provider.RoleAssistant, Content: "a city", Reasoning: "think"},
	})
	if err != nil {
		t.Fatal(err)
	}
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/resume", map[string]any{"sessionId": id, "cwd": wd})
	raw := readRPC(t, cli)
	s := string(raw)
	if strings.Contains(s, "user_message_chunk") || strings.Contains(s, "agent_message_chunk") || strings.Contains(s, "agent_thought_chunk") {
		t.Fatalf("resume replayed history: %s", s)
	}
	if !strings.Contains(s, `"sessionId":"`+id+`"`) || !strings.Contains(s, `"result"`) || !strings.Contains(s, `"modes"`) {
		t.Fatalf("resume result %s", s)
	}
	if a := srv.getSession(id); a == nil || len(a.Messages) != 2 {
		t.Fatal("resume should restore the session without replay")
	}
	if a := srv.getSession(id); a == nil || !sameCwd(a.Cfg.WorkDir, wd) {
		t.Fatalf("resume workdir %q", a.Cfg.WorkDir)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionResumeRejectsCwdMismatch(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	other := t.TempDir()
	cfg := config.Default()
	cfg.WorkDir = wd
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "stay here"},
	})
	if err != nil {
		t.Fatal(err)
	}
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/resume", map[string]any{"sessionId": id, "cwd": other})
	raw := readRPC(t, cli)
	if !strings.Contains(string(raw), "cwd does not match session") {
		t.Fatalf("mismatch %s", raw)
	}
	if srv.getSession(id) != nil {
		t.Fatal("mismatched cwd must not attach the session")
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionResumeRejectsRelativeCwd(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	cfg := config.Default()
	cfg.WorkDir = wd
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "stay here"},
	})
	if err != nil {
		t.Fatal(err)
	}
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/resume", map[string]any{"sessionId": id, "cwd": ".."})
	raw := readRPC(t, cli)
	if !strings.Contains(string(raw), "absolute path") {
		t.Fatalf("relative cwd %s", raw)
	}
	if srv.getSession(id) != nil {
		t.Fatal("relative cwd must not attach the session")
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionLoadUsesStoredWorkdir(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	cfg := config.Default()
	cfg.WorkDir = wd
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "stored dir"},
	})
	if err != nil {
		t.Fatal(err)
	}
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	srvCfg := config.Default()
	srvCfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: srvCfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/load", map[string]any{"sessionId": id})
	deadline := time.Now().Add(5 * time.Second)
	var sawResult bool
	for time.Now().Before(deadline) && !sawResult {
		raw := readRPC(t, cli)
		if strings.Contains(string(raw), `"result"`) && strings.Contains(string(raw), id) {
			sawResult = true
		}
	}
	if !sawResult {
		t.Fatal("load did not return")
	}
	a := srv.getSession(id)
	if a == nil || !sameCwd(a.Cfg.WorkDir, wd) {
		t.Fatalf("load should restore stored workdir, got %v", a)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionLoadRejectsRelativeCwd(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	cfg := config.Default()
	cfg.WorkDir = wd
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "stay here"},
	})
	if err != nil {
		t.Fatal(err)
	}
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/load", map[string]any{"sessionId": id, "cwd": ".."})
	raw := readRPC(t, cli)
	if !strings.Contains(string(raw), "absolute path") {
		t.Fatalf("relative cwd %s", raw)
	}
	if srv.getSession(id) != nil {
		t.Fatal("relative cwd must not attach the session")
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionLoadRestoresPersistedExtraDirs(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	wd := t.TempDir()
	extra := t.TempDir()
	cfg := config.Default()
	cfg.WorkDir = wd
	cfg.ExtraDirs = []string{extra}
	id, err := session.Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "extra roots"},
	})
	if err != nil {
		t.Fatal(err)
	}
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	var got config.Config
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: config.Default(),
		New: func(cfg config.Config) (*agent.Agent, error) {
			got = cfg
			reg := tools.New(cfg.WorkDir, nil, nil)
			reg.SetExtraDirs(cfg.ExtraDirs)
			return agent.New(cfg, scriptClient{content: "ok"}, reg), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/load", map[string]any{"sessionId": id, "cwd": wd})
	deadline := time.Now().Add(5 * time.Second)
	var sawResult bool
	for time.Now().Before(deadline) && !sawResult {
		raw := readRPC(t, cli)
		if strings.Contains(string(raw), `"result"`) && strings.Contains(string(raw), id) {
			sawResult = true
		}
	}
	if !sawResult {
		t.Fatal("load did not return")
	}
	if len(got.ExtraDirs) != 1 || got.ExtraDirs[0] != extra {
		t.Fatalf("extra dirs %v want [%s]", got.ExtraDirs, extra)
	}
	a := srv.getSession(id)
	if a == nil || len(a.Cfg.ExtraDirs) != 1 || a.Cfg.ExtraDirs[0] != extra {
		t.Fatalf("agent extra dirs %v", a.Cfg.ExtraDirs)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestServePromptImage(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })

	var saw int
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			reg := tools.New(cfg.WorkDir, nil, nil)
			return agent.New(cfg, captureImages{n: &saw, next: scriptClient{content: "ok"}}, reg), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()

	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil {
		t.Fatal(err)
	}
	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "이게 뭐야"},
			{"type": "image", "mimeType": "image/png", "data": tinyPNG(t)},
		},
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw := readRPC(t, cli)
		if strings.Contains(string(raw), `"stopReason":"end_turn"`) {
			break
		}
	}
	if saw < 1 {
		t.Fatalf("images %d", saw)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestServePromptEmbeddedResource(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })

	var got string
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			reg := tools.New(cfg.WorkDir, nil, nil)
			return agent.New(cfg, captureText{got: &got, next: scriptClient{content: "ok"}}, reg), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()

	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil {
		t.Fatal(err)
	}
	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "리뷰해"},
			{"type": "resource", "resource": map[string]string{
				"uri":      "file:///tmp/main.py",
				"mimeType": "text/x-python",
				"text":     "def hello():\n    print('hi')",
			}},
		},
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw := readRPC(t, cli)
		if strings.Contains(string(raw), `"stopReason":"end_turn"`) {
			break
		}
	}
	if !strings.Contains(got, "리뷰해") || !strings.Contains(got, "def hello") || !strings.Contains(got, "file:///tmp/main.py") {
		t.Fatalf("prompt %q", got)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestServePromptResourceLink(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })

	wd := t.TempDir()
	inside := filepath.Join(wd, "note.go")
	if err := os.WriteFile(inside, []byte("package note\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("LEAK"), 0o600); err != nil {
		t.Fatal(err)
	}

	var got string
	cfg := config.Default()
	cfg.WorkDir = wd
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			reg := tools.New(cfg.WorkDir, nil, nil)
			return agent.New(cfg, captureText{got: &got, next: scriptClient{content: "ok"}}, reg), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()

	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": wd})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil {
		t.Fatal(err)
	}
	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "봐"},
			{"type": "resource_link", "uri": "file://" + inside, "name": "note.go"},
			{"type": "resource_link", "uri": "file://" + outside, "name": "secret.txt"},
		},
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw := readRPC(t, cli)
		if strings.Contains(string(raw), `"stopReason":"end_turn"`) {
			break
		}
	}
	if !strings.Contains(got, "package note") {
		t.Fatalf("missing workdir file: %q", got)
	}
	if strings.Contains(got, "LEAK") {
		t.Fatalf("leaked outside file: %q", got)
	}
	if !strings.Contains(got, "secret.txt") {
		t.Fatalf("should mention the link: %q", got)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestServePromptResourceLinkAdditionalDir(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })

	wd := t.TempDir()
	extra := t.TempDir()
	lib := filepath.Join(extra, "lib.go")
	if err := os.WriteFile(lib, []byte("package extraLib\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var got string
	cfg := config.Default()
	cfg.WorkDir = wd
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			reg := tools.New(cfg.WorkDir, nil, nil)
			reg.SetExtraDirs(cfg.ExtraDirs)
			return agent.New(cfg, captureText{got: &got, next: scriptClient{content: "ok"}}, reg), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()

	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{
		"cwd":                   wd,
		"additionalDirectories": []string{extra},
	})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil {
		t.Fatal(err)
	}
	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt": []map[string]any{
			{"type": "text", "text": "봐"},
			{"type": "resource_link", "uri": "file://" + lib, "name": "lib.go"},
		},
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw := readRPC(t, cli)
		if strings.Contains(string(raw), `"stopReason":"end_turn"`) {
			break
		}
	}
	if !strings.Contains(got, "package extraLib") {
		t.Fatalf("missing extra file: %q", got)
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestReadFileURIOutsideWorkdir(t *testing.T) {
	wd := t.TempDir()
	outside := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(outside, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFileURI(wd, nil, "file://"+outside, 1024); err == nil {
		t.Fatal("outside workdir should fail")
	}
}

func TestReadFileURIAllowsAdditionalDir(t *testing.T) {
	wd := t.TempDir()
	extra := t.TempDir()
	inside := filepath.Join(extra, "lib.go")
	if err := os.WriteFile(inside, []byte("package extra\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readFileURI(wd, []string{extra}, "file://"+inside, 1024)
	if err != nil || !strings.Contains(got, "package extra") {
		t.Fatalf("%q %v", got, err)
	}
	outside := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(outside, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFileURI(wd, []string{extra}, "file://"+outside, 1024); err == nil {
		t.Fatal("outside extra roots should fail")
	}
}

type captureText struct {
	got  *string
	next provider.Client
}

func (c captureText) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	for _, m := range req.Messages {
		if m.Role == provider.RoleUser {
			*c.got = m.Content
		}
	}
	return c.next.Chat(ctx, req)
}

type captureImages struct {
	n    *int
	next provider.Client
}

func (c captureImages) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	for _, m := range req.Messages {
		*c.n += len(m.Images)
	}
	return c.next.Chat(ctx, req)
}

type blockClient struct {
	started chan struct{}
}

func (b blockClient) Chat(ctx context.Context, _ provider.ChatRequest) (provider.ChatResponse, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return provider.ChatResponse{}, ctx.Err()
}

func TestServeCancelStopsPrompt(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	started := make(chan struct{}, 1)
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Ctx: ctx,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, blockClient{started: started}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil || newResp.Result.SessionID == "" {
		t.Fatal(created)
	}
	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "hi"}},
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("chat did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
}

func TestSessionPromptRejectsOverlap(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	started := make(chan struct{}, 1)
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Ctx: ctx,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, blockClient{started: started}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	t.Cleanup(func() {
		cancel()
		_ = fromCli.Close()
		_ = fromSrv.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	})
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil || newResp.Result.SessionID == "" {
		t.Fatal(created)
	}
	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "first"}},
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("chat did not start")
	}
	writeRPC(t, fromCli, 4, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "second"}},
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := cli.read()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `"id":4`) && strings.Contains(string(raw), "busy") {
			return
		}
	}
	t.Fatal("overlapping prompt should be busy")
}

func TestSessionSetModeRejectsBusy(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	started := make(chan struct{}, 1)
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Ctx: ctx,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, blockClient{started: started}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	t.Cleanup(func() {
		cancel()
		_ = fromCli.Close()
		_ = fromSrv.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	})
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil || newResp.Result.SessionID == "" {
		t.Fatal(created)
	}
	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "hi"}},
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("chat did not start")
	}
	writeRPC(t, fromCli, 4, "session/set_mode", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"modeId":    "plan",
	})
	if !waitBusyID(t, cli, 4) {
		t.Fatal("set_mode should be busy")
	}
	writeRPC(t, fromCli, 5, "session/set_config_option", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"configId":  "model",
		"value":     "solar-mini",
	})
	if !waitBusyID(t, cli, 5) {
		t.Fatal("set_config_option should be busy")
	}
}

func TestSessionPromptPersistFailure(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil || newResp.Result.SessionID == "" {
		t.Fatal(created)
	}
	breakSessionsDir(t)
	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "hi"}},
	})
	if !waitErrorID(t, cli, 3, "session save") {
		t.Fatal("prompt should fail when the session cannot be saved")
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionClosePersistFailure(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	t.Cleanup(func() { _ = fromCli.Close(); _ = fromSrv.Close() })
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, scriptClient{content: "ok"}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil || newResp.Result.SessionID == "" {
		t.Fatal(created)
	}
	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "hi"}},
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw := readRPC(t, cli)
		if strings.Contains(string(raw), `"stopReason":"end_turn"`) {
			break
		}
	}
	breakSessionsDir(t)
	writeRPC(t, fromCli, 4, "session/close", map[string]any{"sessionId": newResp.Result.SessionID})
	if !waitErrorID(t, cli, 4, "session save") {
		t.Fatal("close should fail when the session cannot be saved")
	}
	if srv.getSession(newResp.Result.SessionID) != nil {
		t.Fatal("session still in memory after failed close save")
	}
	_ = fromCli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func TestSessionCloseWaitsForPrompt(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	toSrv, fromCli := io.Pipe()
	toCli, fromSrv := io.Pipe()
	cli := newConn(toCli, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	started := make(chan struct{}, 1)
	cfg := config.Default()
	cfg.WorkDir = t.TempDir()
	srv := &Server{
		In:  toSrv,
		Out: fromSrv,
		Ctx: ctx,
		Cfg: cfg,
		New: func(cfg config.Config) (*agent.Agent, error) {
			return agent.New(cfg, blockClient{started: started}, tools.New(cfg.WorkDir, nil, nil)), nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve() }()
	t.Cleanup(func() {
		cancel()
		_ = fromCli.Close()
		_ = fromSrv.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	})
	writeRPC(t, fromCli, 1, "initialize", map[string]any{"protocolVersion": 1})
	_ = readRPC(t, cli)
	writeRPC(t, fromCli, 2, "session/new", map[string]any{"cwd": cfg.WorkDir})
	created := readRPC(t, cli)
	var newResp struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(created, &newResp); err != nil || newResp.Result.SessionID == "" {
		t.Fatal(created)
	}
	writeRPC(t, fromCli, 3, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "hi"}},
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("chat did not start")
	}
	writeRPC(t, fromCli, 4, "session/close", map[string]any{
		"sessionId": newResp.Result.SessionID,
	})
	var sawClose, sawCancel bool
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !(sawClose && sawCancel) {
		msg, err := cli.read()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `"id":4`) && (msg.Error == nil || msg.Error.Message == "") {
			sawClose = true
		}
		if strings.Contains(string(raw), `"id":3`) && strings.Contains(string(raw), "cancelled") {
			sawCancel = true
		}
	}
	if !sawClose {
		t.Fatal("close should finish after canceling the prompt")
	}
	writeRPC(t, fromCli, 5, "session/prompt", map[string]any{
		"sessionId": newResp.Result.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "again"}},
	})
	if !waitErrorID(t, cli, 5, "unknown") {
		t.Fatal("closed session should be unknown")
	}
}

func waitErrorID(t *testing.T, in *Conn, id int, substr string) bool {
	t.Helper()
	want := fmt.Sprintf(`"id":%d`, id)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := in.read()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), want) && strings.Contains(string(raw), substr) {
			return true
		}
	}
	return false
}

func waitBusyID(t *testing.T, in *Conn, id int) bool {
	t.Helper()
	want := fmt.Sprintf(`"id":%d`, id)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := in.read()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), want) && strings.Contains(string(raw), "busy") {
			return true
		}
	}
	return false
}

func tinyPNG(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func breakSessionsDir(t *testing.T) {
	t.Helper()
	root := os.Getenv("GOPPI_DATA_DIR")
	if root == "" {
		t.Fatal("GOPPI_DATA_DIR unset")
	}
	dir := filepath.Join(root, "sessions")
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeRPC(t *testing.T, w io.Writer, id int, method string, params any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(append(raw, '\n')); err != nil {
		t.Fatal(err)
	}
}

func readRPC(t *testing.T, in *Conn) []byte {
	t.Helper()
	msg, err := in.read()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
