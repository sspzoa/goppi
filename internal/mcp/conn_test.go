package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/rpcio"
)

func TestMain(m *testing.M) {
	if os.Getenv("GOPPI_FAKE_MCP") == "1" {
		_, _ = os.Stderr.WriteString("mcp-stderr-line\n")
		os.Exit(serveFakeMCP(os.Stdin, os.Stdout))
	}
	os.Exit(m.Run())
}

func TestConnInitializeListCall(t *testing.T) {
	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer serverW.Close()
		_ = serveFakeMCP(serverR, serverW)
	}()
	c := NewConn(clientR, clientW)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Initialize(ctx, "goppi", "test"); err != nil {
		t.Fatal(err)
	}
	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("%+v", tools)
	}
	out, err := c.Call(ctx, "echo", json.RawMessage(`{"text":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	if out != "hi" {
		t.Fatalf("got %q", out)
	}
	_ = clientW.Close()
	<-done
}

func TestStartStdioEcho(t *testing.T) {
	s, err := Start(context.Background(), "echo", config.MCPServer{
		Command: os.Args[0],
		Env:     map[string]string{"GOPPI_FAKE_MCP": "1"},
	}, t.TempDir(), "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if len(s.Tools) != 1 || s.Tools[0].Name != "echo" {
		t.Fatalf("%+v", s.Tools)
	}
	out, err := s.Conn.Call(context.Background(), "echo", json.RawMessage(`{"text":"ok"}`))
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Fatalf("got %q", out)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if strings.Contains(s.Stderr(), "mcp-stderr-line") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stderr %q", s.Stderr())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStartHookError(t *testing.T) {
	_, err := Start(context.Background(), "echo", config.MCPServer{Command: os.Args[0]}, t.TempDir(), "test", func(*exec.Cmd) error {
		return fmt.Errorf("blocked")
	})
	if err == nil || !strings.Contains(err.Error(), "sandbox") || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("got %v", err)
	}
}

func TestErrCapTruncates(t *testing.T) {
	e := &errCap{}
	big := strings.Repeat("a", maxErrCap+32)
	if _, err := e.Write([]byte(big)); err != nil {
		t.Fatal(err)
	}
	got := e.String()
	if len(got) != maxErrCap {
		t.Fatalf("len %d", len(got))
	}
	if !strings.HasSuffix(got, "aaa") || strings.Contains(got, "b") {
		t.Fatalf("keep tail")
	}
}

func TestReadFrameJSONLine(t *testing.T) {
	got, err := readFrame(bufio.NewReader(strings.NewReader("{\"a\":1}\n")))
	if err != nil || strings.TrimSpace(string(got)) != `{"a":1}` {
		t.Fatalf("%q %v", got, err)
	}
}

func TestReadFrameRejectsHugeHeader(t *testing.T) {
	raw := "Content-Length: " + strings.Repeat("1", rpcio.MaxHeader) + "\n\n"
	if _, err := readFrame(bufio.NewReader(strings.NewReader(raw))); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("got %v", err)
	}
}

func TestToolNameSanitizes(t *testing.T) {
	if got := ToolName("GitHub!", "list-repos"); got != "mcp_github_list_repos" {
		t.Fatalf("got %q", got)
	}
}

func serveFakeMCP(r io.Reader, w io.Writer) int {
	in := bufio.NewReader(r)
	for {
		raw, err := readFrame(in)
		if err != nil {
			if err == io.EOF {
				return 0
			}
			return 1
		}
		var req struct {
			ID     int64           `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			continue
		}
		switch req.Method {
		case "notifications/initialized":
			continue
		case "initialize":
			_ = writeJSON(w, req.ID, map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]string{"name": "fake"},
			})
		case "tools/list":
			_ = writeJSON(w, req.ID, map[string]any{
				"tools": []any{
					map[string]any{
						"name":        "echo",
						"description": "echo text",
						"inputSchema": map[string]any{
							"type":       "object",
							"properties": map[string]any{"text": map[string]any{"type": "string"}},
						},
					},
				},
			})
		case "tools/call":
			var p struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &p)
			text := "pong"
			if p.Name == "echo" && len(p.Arguments) > 0 {
				var a struct {
					Text string `json:"text"`
				}
				if json.Unmarshal(p.Arguments, &a) == nil && a.Text != "" {
					text = a.Text
				}
			}
			_ = writeJSON(w, req.ID, map[string]any{
				"content": []any{map[string]any{"type": "text", "text": text}},
				"isError": false,
			})
		}
	}
}

func writeJSON(w io.Writer, id int64, result any) error {
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	if err != nil {
		return err
	}
	return writeFrame(w, raw)
}
