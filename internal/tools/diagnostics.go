package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sspzoa/goppi/internal/lsp"
	"github.com/sspzoa/goppi/internal/provider"
)

type diagnostics struct {
	workdir string
	root    *fileRoot
	hub     *lsp.Hub
}

func (diagnostics) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name:        "diagnostics",
		Description: "Language-server diagnostics for a file in the workdir. Omit path to see cached results. Read-only.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"path":{"type":"string","description":"File to analyze (workdir relative or absolute)"}
			}
		}`),
	}
}

func (t diagnostics) Run(ctx context.Context, input json.RawMessage) (string, error) {
	if t.hub == nil {
		return "", fmt.Errorf("no language server")
	}
	args, err := decode[struct {
		Path string `json:"path"`
	}](input)
	if err != nil && len(input) > 0 && string(input) != "{}" && string(input) != "null" {
		return "", err
	}
	path := args.Path
	if path != "" {
		abs, err := scopedResolve(t.workdir, t.root, path)
		if err != nil {
			return "", err
		}
		path = abs
	}
	return t.hub.Query(ctx, t.workdir, path)
}

func (r *Registry) AttachLSP(hub *lsp.Hub) {
	if r == nil {
		return
	}
	r.lsp = hub
	if d, ok := r.by["diagnostics"].(*diagnostics); ok {
		d.hub = hub
	}
}
