package acp

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/sspzoa/goppi/internal/config"
)

type acpMCPServer struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Command string          `json:"command"`
	Args    []string        `json:"args"`
	Env     json.RawMessage `json:"env"`
}

func applyClientMCP(cfg *config.Config, raw json.RawMessage) {
	if cfg == nil {
		return
	}
	extra := parseACPServers(raw)
	if len(extra) == 0 {
		return
	}
	cfg.MCPServers = copyMCP(cfg.MCPServers)
	for name, spec := range extra {
		cfg.MCPServers[name] = spec
	}
}

func parseACPServers(raw json.RawMessage) map[string]config.MCPServer {
	if len(raw) == 0 {
		return nil
	}
	var list []acpMCPServer
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	out := map[string]config.MCPServer{}
	for _, s := range list {
		if !stdioMCP(s) {
			continue
		}
		name := strings.TrimSpace(s.Name)
		if name == "" {
			name = filepath.Base(s.Command)
		}
		if name == "" || name == "." || name == string(filepath.Separator) {
			continue
		}
		out[name] = config.MCPServer{Command: strings.TrimSpace(s.Command), Args: append([]string{}, s.Args...), Env: parseACPEnv(s.Env)}
	}
	return out
}

func stdioMCP(s acpMCPServer) bool {
	switch strings.ToLower(strings.TrimSpace(s.Type)) {
	case "http", "sse", "http-stream", "streamable-http":
		return false
	}
	return strings.TrimSpace(s.Command) != ""
}

func parseACPEnv(raw json.RawMessage) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err == nil && obj != nil {
		return obj
	}
	var list []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	out := map[string]string{}
	for _, e := range list {
		if strings.TrimSpace(e.Name) == "" {
			continue
		}
		out[e.Name] = e.Value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func copyMCP(in map[string]config.MCPServer) map[string]config.MCPServer {
	out := map[string]config.MCPServer{}
	for k, v := range in {
		spec := v
		if v.Args != nil {
			spec.Args = append([]string{}, v.Args...)
		}
		if v.Env != nil {
			spec.Env = map[string]string{}
			for ek, ev := range v.Env {
				spec.Env[ek] = ev
			}
		}
		out[k] = spec
	}
	return out
}
