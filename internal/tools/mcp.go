package tools

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/sspzoa/goppi/internal/mcp"
	"github.com/sspzoa/goppi/internal/provider"
)

type mcpTool struct {
	name        string
	description string
	schema      json.RawMessage
	orig        string
	sess        *mcp.Session
}

func (t mcpTool) Spec() provider.ToolSpec {
	params := t.schema
	if len(params) == 0 {
		params = schema(`{"type":"object"}`)
	}
	desc := t.description
	if desc == "" {
		desc = "MCP tool " + t.orig + " on server " + t.sess.Name
	}
	return provider.ToolSpec{Name: t.name, Description: desc, Parameters: params}
}

func (t mcpTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return t.sess.Conn.Call(ctx, t.orig, input)
}

func (r *Registry) AttachMCP(sessions []*mcp.Session) {
	if r == nil {
		return
	}
	r.mcp = append(r.mcp, sessions...)
	for _, s := range sessions {
		if s == nil {
			continue
		}
		for _, t := range s.Tools {
			name := mcp.ToolName(s.Name, t.Name)
			if _, ok := r.by[name]; ok {
				continue
			}
			if strings.TrimSpace(t.Name) == "" {
				continue
			}
			r.add(mcpTool{
				name:        name,
				description: t.Description,
				schema:      t.InputSchema,
				orig:        t.Name,
				sess:        s,
			})
		}
	}
}

func (r *Registry) MCPNames() []string {
	if r == nil {
		return nil
	}
	var out []string
	for _, t := range r.order {
		name := t.Spec().Name
		if strings.HasPrefix(name, "mcp_") {
			out = append(out, name)
		}
	}
	return out
}

func (r *Registry) Close() {
	if r == nil {
		return
	}
	for _, s := range r.mcp {
		s.Close()
	}
	r.mcp = nil
	if r.lsp != nil {
		r.lsp.Close()
		r.lsp = nil
	}
	if r.jobs != nil {
		r.jobs.killAll()
	}
}
