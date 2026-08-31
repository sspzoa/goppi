package tools

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sspzoa/goppi/internal/mcp"
)

func TestRunRejectsHugeInput(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	huge := []byte(`{"path":"` + strings.Repeat("a", maxWriteBytes+70<<10) + `"}`)
	_, err := reg.Run(context.Background(), "read_file", huge)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("got %v", err)
	}
}

func TestTodoWrite(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	in, _ := json.Marshal(map[string]any{
		"todos": []map[string]string{
			{"id": "1", "content": "read", "status": "done"},
			{"id": "2", "content": "write", "status": "pending"},
		},
	})
	out, err := reg.Run(context.Background(), "todo_write", in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[x]") || !strings.Contains(out, "write") {
		t.Fatalf("%q", out)
	}
}

func TestParallelSafe(t *testing.T) {
	if !ParallelSafe("read_file") || !ParallelSafe("glob") || !ParallelSafe("grep") {
		t.Fatal("reads")
	}
	if ParallelSafe("bash") || ParallelSafe("bash_kill") || ParallelSafe("write_file") || ParallelSafe("edit_file") || ParallelSafe("apply_patch") {
		t.Fatal("writes")
	}
	if !ParallelSafe("bash_poll") {
		t.Fatal("poll")
	}
	if ParallelSafe("delegate") || ParallelSafe("todo_write") || ParallelSafe("read_image") {
		t.Fatal("shared")
	}
	if ParallelSafe("mcp_fs_read") {
		t.Fatal("mcp")
	}
	if AllParallelSafe([]string{"read_file"}) {
		t.Fatal("single")
	}
	if !AllParallelSafe([]string{"read_file", "glob"}) {
		t.Fatal("batch")
	}
	if AllParallelSafe([]string{"read_file", "write_file"}) {
		t.Fatal("mixed")
	}
}

func TestDiagnosticsWithoutServer(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	_, err := reg.Run(context.Background(), "diagnostics", json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "no language server") {
		t.Fatalf("got %v", err)
	}
}

func TestPlanModeDeniesBash(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	reg.SetMode("plan")
	_, err := reg.Run(context.Background(), "bash", json.RawMessage(`{"command":"echo hi"}`))
	if err == nil || !strings.Contains(err.Error(), "plan mode") {
		t.Fatalf("got %v", err)
	}
}

func TestWebFetch(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/html")
		_, _ = w.Write([]byte("<html><body><h1>Hello</h1><p>world</p></body></html>"))
	}))
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	out, err := (webFetch{client: s.Client()}).get(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "world") {
		t.Fatalf("%q", out)
	}
}

func TestWebFetchRejectsMetadataIP(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	_, err := reg.Run(context.Background(), "web_fetch", json.RawMessage(`{"url":"http://169.254.169.254/latest"}`))
	if err == nil || !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("got %v", err)
	}
}

func TestWebFetchRejectsMetadataLookup(t *testing.T) {
	orig := lookupIP
	lookupIP = func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("169.254.169.254")}, nil
	}
	t.Cleanup(func() { lookupIP = orig })
	if err := checkFetchHost("evil.example"); err == nil || !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("got %v", err)
	}
}

func TestWebFetchRejectsPrivateAndLocal(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	for _, raw := range []string{
		`{"url":"http://127.0.0.1/"}`,
		`{"url":"http://localhost/"}`,
		`{"url":"http://[::1]/"}`,
		`{"url":"http://10.0.0.1/"}`,
		`{"url":"http://192.168.1.1/"}`,
		`{"url":"http://172.16.0.1/"}`,
		`{"url":"http://100.64.0.1/"}`,
		`{"url":"http://0.0.0.0/"}`,
	} {
		_, err := reg.Run(context.Background(), "web_fetch", json.RawMessage(raw))
		if err == nil || !strings.Contains(err.Error(), "private or local") {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	_, err := reg.Run(context.Background(), "web_fetch", json.RawMessage(`{"url":"http://user:secret@example.com/"}`))
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("userinfo: %v", err)
	}
}

func TestWebFetchBlocksDNSRebind(t *testing.T) {
	n := 0
	orig := lookupIP
	lookupIP = func(string) ([]net.IP, error) {
		n++
		if n == 1 {
			return []net.IP{net.ParseIP("1.1.1.1")}, nil
		}
		return []net.IP{net.ParseIP("169.254.169.254")}, nil
	}
	t.Cleanup(func() { lookupIP = orig })
	reg := New(t.TempDir(), nil, nil)
	_, err := reg.Run(context.Background(), "web_fetch", json.RawMessage(`{"url":"http://evil.example/"}`))
	if err == nil || !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("rebind: %v", err)
	}
}

func TestWebFetchRejectsPrivateLookup(t *testing.T) {
	orig := lookupIP
	lookupIP = func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.1.2.3")}, nil
	}
	t.Cleanup(func() { lookupIP = orig })
	if err := checkFetchHost("intranet.example"); err == nil || !strings.Contains(err.Error(), "private or local") {
		t.Fatalf("got %v", err)
	}
}

func TestWebFetchRejectsFile(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	_, err := reg.Run(context.Background(), "web_fetch", json.RawMessage(`{"url":"file:///etc/passwd"}`))
	if err == nil {
		t.Fatal("expected reject")
	}
}

func TestMutatingMCPPrefix(t *testing.T) {
	if !Mutating("mcp_gh_list") {
		t.Fatal("mcp tools are mutating")
	}
	if Mutating("delegate") {
		t.Fatal("delegate is read-only")
	}
	if Mutating("read_file") {
		t.Fatal("read_file is not mutating")
	}
}

func TestAttachMCPRegistersTools(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	reg.AttachMCP([]*mcp.Session{{
		Name:  "echo",
		Tools: []mcp.Tool{{Name: "ping", Description: "p"}},
	}})
	names := reg.MCPNames()
	if len(names) != 1 || names[0] != "mcp_echo_ping" {
		t.Fatalf("names %v", names)
	}
}

func TestPlanModeDeniesMCP(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	reg.add(mcpTool{name: "mcp_echo_ping", description: "ping", orig: "ping"})
	reg.SetMode("plan")
	_, err := reg.Run(context.Background(), "mcp_echo_ping", json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "plan mode") {
		t.Fatalf("got %v", err)
	}
}

func TestReadSkill(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	reg.SetSkills(nil)
	_, err := reg.Run(context.Background(), "read_skill", json.RawMessage(`{"name":"nope"}`))
	if err == nil {
		t.Fatal("expected unknown skill")
	}
}
