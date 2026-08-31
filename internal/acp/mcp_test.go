package acp

import (
	"encoding/json"
	"testing"

	"github.com/sspzoa/goppi/internal/config"
)

func TestParseACPServersStdioAndEnv(t *testing.T) {
	raw := json.RawMessage(`[
		{"name":"filesystem","command":"/bin/mcp","args":["--stdio"],"env":[{"name":"TOKEN","value":"abc"}]},
		{"name":"objenv","command":"npx","env":{"FOO":"bar"}},
		{"name":"remote","type":"http","url":"https://example.com/mcp","command":""},
		{"name":"sse","type":"sse","command":"ignored"},
		{"command":""}
	]`)
	got := parseACPServers(raw)
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	if got["filesystem"].Command != "/bin/mcp" || got["filesystem"].Env["TOKEN"] != "abc" {
		t.Fatalf("filesystem %+v", got["filesystem"])
	}
	if got["objenv"].Env["FOO"] != "bar" {
		t.Fatalf("objenv %+v", got["objenv"])
	}
	if _, ok := got["remote"]; ok {
		t.Fatal("http must be skipped")
	}
	if _, ok := got["sse"]; ok {
		t.Fatal("sse must be skipped")
	}
}

func TestApplyClientMCPDoesNotMutateSharedMap(t *testing.T) {
	cfg := config.Default()
	cfg.MCPServers = map[string]config.MCPServer{"user": {Command: "user-mcp"}}
	shared := cfg.MCPServers
	applyClientMCP(&cfg, json.RawMessage(`[{"name":"fs","command":"npx"}]`))
	if _, ok := shared["fs"]; ok {
		t.Fatal("client MCP leaked into shared config map")
	}
	if cfg.MCPServers["user"].Command != "user-mcp" || cfg.MCPServers["fs"].Command != "npx" {
		t.Fatalf("%+v", cfg.MCPServers)
	}
}

func TestApplyClientMCPEmptyLeavesUser(t *testing.T) {
	cfg := config.Default()
	cfg.MCPServers = map[string]config.MCPServer{"user": {Command: "user-mcp"}}
	applyClientMCP(&cfg, json.RawMessage(`[]`))
	if len(cfg.MCPServers) != 1 || cfg.MCPServers["user"].Command != "user-mcp" {
		t.Fatalf("%+v", cfg.MCPServers)
	}
}
