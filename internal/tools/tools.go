package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/lsp"
	"github.com/sspzoa/goppi/internal/mcp"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/skills"
	"github.com/sspzoa/goppi/internal/upstage"
)

type Runner interface {
	Spec() provider.ToolSpec
	Run(ctx context.Context, input json.RawMessage) (string, error)
}

type Verdict int

const (
	Denied Verdict = iota
	Allowed
	AllowedSession
)

func (v Verdict) OK() bool { return v != Denied }

func AlwaysAllow(string, string) Verdict { return Allowed }
func AlwaysDeny(string, string) Verdict  { return Denied }

type Ask func(name, detail string) Verdict

type Registry struct {
	order         []Runner
	by            map[string]Runner
	ask           Ask
	askUser       AskUser
	mode          string
	todos         *TodoList
	skill         *readSkill
	snaps         *snapStack
	mcp           []*mcp.Session
	lsp           *lsp.Hub
	pendingImages []provider.Image
	hooks         config.Hooks
	workdir       string
	root          *fileRoot
	sandbox       string
	jobs          *jobHub
	sessionOK     map[string]bool
	mu            sync.Mutex
}

func Dangerous(name string) bool {
	return Mutating(name)
}

func Mutating(name string) bool {
	switch name {
	case "bash", "bash_kill", "write_file", "edit_file", "apply_patch":
		return true
	}
	return strings.HasPrefix(name, "mcp_")
}

// ParallelSafe tools have no user prompt and no shared mutable
// workspace side effects, so a batch of them can run together.
func ParallelSafe(name string) bool {
	switch name {
	case "read_file", "glob", "grep", "web_fetch", "read_skill", "diagnostics", "document_parse", "document_ocr", "bash_poll":
		return true
	default:
		return false
	}
}

func AllParallelSafe(names []string) bool {
	if len(names) < 2 {
		return false
	}
	for _, n := range names {
		if !ParallelSafe(n) {
			return false
		}
	}
	return true
}

func New(workdir string, api *upstage.Client, ask Ask) *Registry {
	todos := NewTodoList()
	sk := &readSkill{}
	snaps := &snapStack{}
	jobs := newJobHub()
	root := &fileRoot{primary: workdir}
	r := &Registry{by: map[string]Runner{}, ask: ask, mode: "act", todos: todos, skill: sk, snaps: snaps, workdir: workdir, root: root, sandbox: "workspace", jobs: jobs, sessionOK: map[string]bool{}}
	list := []Runner{
		readFile{workdir: workdir, root: root},
		writeFile{workdir: workdir, root: root, snaps: snaps},
		editFile{workdir: workdir, root: root, snaps: snaps},
		applyPatch{workdir: workdir, root: root, snaps: snaps},
		globFiles{workdir: workdir, root: root},
		grepFiles{workdir: workdir, root: root},
		&bashCmd{workdir: workdir, root: root, sandbox: "workspace", jobs: jobs},
		bashPoll{jobs: jobs},
		bashKill{jobs: jobs},
		&diagnostics{workdir: workdir, root: root},
		todoWrite{list: todos},
		webFetch{},
		readImage{workdir: workdir, root: root, reg: r},
		askUserTool{reg: r},
		sk,
	}
	if api != nil {
		list = append(list,
			documentParse{workdir: workdir, root: root, api: api},
			documentOCR{workdir: workdir, root: root, api: api},
		)
	}
	for _, t := range list {
		r.add(t)
	}
	return r
}

func (r *Registry) add(t Runner) {
	name := t.Spec().Name
	if _, ok := r.by[name]; ok {
		return
	}
	r.order = append(r.order, t)
	r.by[name] = t
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

func (r *Registry) SetAsk(ask Ask) {
	r.ask = ask
}

func (r *Registry) AllowSession(name string) {
	if r == nil || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessionOK == nil {
		r.sessionOK = map[string]bool{}
	}
	r.sessionOK[name] = true
}

func (r *Registry) SessionAllowed(name string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sessionOK[name]
}

func (r *Registry) ResetRuntime() {
	if r == nil {
		return
	}
	if r.jobs != nil {
		r.jobs.killAll()
	}
	r.ClearSessionAllows()
	r.ClearEdits()
}

func (r *Registry) ClearSessionAllows() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessionOK = map[string]bool{}
}

func (r *Registry) SetMode(mode string) {
	if mode == "" {
		mode = "act"
	}
	r.mode = mode
}

func (r *Registry) SetSandbox(mode string) {
	if r == nil {
		return
	}
	r.sandbox = normalizeSandbox(mode)
	if b, ok := r.by["bash"].(*bashCmd); ok {
		b.sandbox = r.sandbox
	}
}

func (r *Registry) sandboxMode() string {
	if r == nil || r.sandbox == "" {
		return "workspace"
	}
	return r.sandbox
}

func normalizeSandbox(mode string) string {
	switch mode {
	case "off", "strict":
		return mode
	default:
		return "workspace"
	}
}

func (r *Registry) Mode() string {
	if r == nil || r.mode == "" {
		return "act"
	}
	return r.mode
}

func (r *Registry) Todos() *TodoList {
	if r == nil || r.todos == nil {
		return NewTodoList()
	}
	return r.todos
}

func (r *Registry) SetSkills(list []skills.Skill) {
	if r != nil && r.skill != nil {
		r.skill.list = list
	}
}

func (r *Registry) SetHooks(h config.Hooks) {
	if r != nil {
		r.hooks = h
	}
}

func (r *Registry) JobSummary() string {
	if r == nil || r.jobs == nil {
		return "(no jobs)"
	}
	return r.jobs.summary()
}

func (r *Registry) JobCounts() (running, total int) {
	if r == nil || r.jobs == nil {
		return 0, 0
	}
	return r.jobs.counts()
}

func (r *Registry) Run(ctx context.Context, name string, input json.RawMessage) (string, error) {
	out, err := r.execute(ctx, name, input)
	if err != nil {
		err = fmt.Errorf("%s", RedactSecrets(err.Error()))
	}
	return RedactSecrets(out), err
}

func (r *Registry) execute(ctx context.Context, name string, input json.RawMessage) (string, error) {
	if len(input) > maxWriteBytes+64<<10 {
		return "", fmt.Errorf("tool input too large")
	}
	t, ok := r.by[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	if r.mode == "plan" && Mutating(name) {
		return "", fmt.Errorf("plan mode denies %s — /act 로 전환하세요", name)
	}
	if _, err := runHooks(ctx, r.workdir, r.sandboxMode(), r.hooks.PreTool, hookEvent{
		Event:   "pre_tool",
		Name:    name,
		Input:   input,
		WorkDir: r.workdir,
	}, r.root.Extra()...); err != nil {
		return "", err
	}
	if Dangerous(name) && r.ask != nil && !r.SessionAllowed(name) {
		switch r.ask(name, AskDetail(r.workdir, name, input, r.root.Extra()...)) {
		case AllowedSession:
			r.AllowSession(name)
		case Allowed:
		default:
			return "", fmt.Errorf("permission denied: %s", name)
		}
	}
	result, err := t.Run(ctx, input)
	result = RedactSecrets(result)
	if err != nil {
		err = fmt.Errorf("%s", RedactSecrets(err.Error()))
	}
	post := hookEvent{Event: "post_tool", Name: name, Input: input, WorkDir: r.workdir, Result: result}
	if err != nil {
		post.Error = err.Error()
	}
	if extra, _ := runHooks(ctx, r.workdir, r.sandboxMode(), r.hooks.PostTool, post, r.root.Extra()...); extra != "" {
		if result != "" {
			result += "\n"
		}
		result += extra
	}
	return RedactSecrets(result), err
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
