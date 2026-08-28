package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/upstage"
)

type Runner interface {
	Spec() provider.ToolSpec
	Run(ctx context.Context, input json.RawMessage) (string, error)
}

type Registry struct {
	order []Runner
	by    map[string]Runner
}

func New(workdir string, api *upstage.Client) *Registry {
	r := &Registry{by: map[string]Runner{}}
	list := []Runner{
		readFile{workdir: workdir},
		writeFile{workdir: workdir},
		editFile{workdir: workdir},
		globFiles{workdir: workdir},
		grepFiles{workdir: workdir},
		bashCmd{workdir: workdir},
	}
	if api != nil {
		list = append(list,
			documentParse{workdir: workdir, api: api},
			documentOCR{workdir: workdir, api: api},
		)
	}
	for _, t := range list {
		r.order = append(r.order, t)
		r.by[t.Spec().Name] = t
	}
	return r
}

func (r *Registry) Specs() []provider.ToolSpec {
	out := make([]provider.ToolSpec, 0, len(r.order))
	for _, t := range r.order {
		out = append(out, t.Spec())
	}
	return out
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.order))
	for _, t := range r.order {
		out = append(out, t.Spec().Name)
	}
	return out
}

func (r *Registry) Run(ctx context.Context, name string, input json.RawMessage) (string, error) {
	t, ok := r.by[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return t.Run(ctx, input)
}

func schema(s string) json.RawMessage {
	return json.RawMessage(s)
}

func decode[T any](input json.RawMessage) (T, error) {
	var v T
	if len(input) == 0 {
		return v, fmt.Errorf("empty tool input")
	}
	if err := json.Unmarshal(input, &v); err != nil {
		return v, fmt.Errorf("tool input: %w", err)
	}
	return v, nil
}
