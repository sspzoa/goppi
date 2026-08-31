package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/sspzoa/goppi/internal/config"
)

type Diagnostic struct {
	URI      string
	Path     string
	Line     int
	Col      int
	Severity string
	Source   string
	Message  string
}

type Session struct {
	Name     string
	Language string
	Root     string
	cmd      *exec.Cmd
	conn     *Conn
	mu       sync.Mutex
	diags    map[string][]Diagnostic
	opened   map[string]int
	updated  chan string
	errOut   *errCap
}

type Hub struct {
	mu       sync.Mutex
	sessions []*Session
}

func (h *Hub) add(s *Session) {
	if h == nil || s == nil {
		return
	}
	h.mu.Lock()
	h.sessions = append(h.sessions, s)
	h.mu.Unlock()
}

func (h *Hub) Names() []string {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.sessions))
	for _, s := range h.sessions {
		out = append(out, s.Name)
	}
	sort.Strings(out)
	return out
}

func (h *Hub) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	list := h.sessions
	h.sessions = nil
	h.mu.Unlock()
	for _, s := range list {
		s.Close()
	}
}

func (h *Hub) Query(ctx context.Context, workdir, path string) (string, error) {
	if h == nil {
		return "", fmt.Errorf("no language server")
	}
	h.mu.Lock()
	list := append([]*Session(nil), h.sessions...)
	h.mu.Unlock()
	if len(list) == 0 {
		return "", fmt.Errorf("no language server")
	}
	if strings.TrimSpace(path) == "" {
		return formatDiags(workdir, allDiags(list)), nil
	}
	s := pickSession(list, path)
	if s == nil {
		return "", fmt.Errorf("no language server for %s", path)
	}
	diags, err := s.OpenPath(ctx, path)
	if err != nil {
		return "", err
	}
	return formatDiags(workdir, diags), nil
}

func allDiags(sessions []*Session) []Diagnostic {
	var out []Diagnostic
	for _, s := range sessions {
		s.mu.Lock()
		for _, list := range s.diags {
			out = append(out, list...)
		}
		s.mu.Unlock()
	}
	return out
}

func pickSession(sessions []*Session, path string) *Session {
	lang := languageFor(path)
	var fallback *Session
	for _, s := range sessions {
		if fallback == nil {
			fallback = s
		}
		if s.Language == lang || s.Language == "" {
			return s
		}
	}
	return fallback
}

func (s *Session) Close() {
	if s == nil {
		return
	}
	if s.conn != nil {
		s.conn.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		killGroup(s.cmd)
		_ = s.cmd.Wait()
	}
}

func (s *Session) OpenPath(ctx context.Context, path string) ([]Diagnostic, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("not utf-8")
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("file too large for language server")
	}
	uri := fileURI(path)
	s.mu.Lock()
	if _, ok := s.opened[uri]; ok {
		diags := append([]Diagnostic(nil), s.diags[uri]...)
		s.mu.Unlock()
		return diags, nil
	}
	s.opened[uri] = 1
	s.mu.Unlock()

	lang := s.Language
	if lang == "" {
		lang = languageFor(path)
	}
	if err := s.conn.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": lang,
			"version":    1,
			"text":       string(data),
		},
	}); err != nil {
		return nil, err
	}
	wait, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	s.waitURI(wait, uri)
	s.mu.Lock()
	diags := append([]Diagnostic(nil), s.diags[uri]...)
	s.mu.Unlock()
	return diags, nil
}

func (s *Session) waitURI(ctx context.Context, uri string) {
	for {
		s.mu.Lock()
		if _, ok := s.diags[uri]; ok {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case got := <-s.updated:
			if got == uri {
				return
			}
		}
	}
}

type CmdHook func(*exec.Cmd) error

func Start(ctx context.Context, name string, spec config.LSPServer, workdir, version string, hook CmdHook) (*Session, error) {
	if strings.TrimSpace(spec.Command) == "" {
		return nil, fmt.Errorf("lsp %s: empty command", name)
	}
	cmd := exec.Command(spec.Command, spec.Args...)
	cmd.Dir = workdir
	cmd.Env = cleanEnv(os.Environ())
	errOut := &errCap{}
	cmd.Stderr = errOut
	setKillGroup(cmd)
	if hook != nil {
		if err := hook(cmd); err != nil {
			return nil, fmt.Errorf("lsp %s: sandbox: %w", name, err)
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
		return nil, fmt.Errorf("lsp %s: start: %w", name, err)
	}
	s := &Session{
		Name:     name,
		Language: spec.Language,
		Root:     workdir,
		cmd:      cmd,
		diags:    map[string][]Diagnostic{},
		opened:   map[string]int{},
		updated:  make(chan string, 8),
		errOut:   errOut,
	}
	conn := newConn(stdout, stdin)
	conn.onNotify = s.handleNotify
	conn.reply = func(method string) any {
		if method == "workspace/workspaceFolders" {
			return []map[string]string{{"uri": fileURI(workdir), "name": filepath.Base(workdir)}}
		}
		if method == "workspace/configuration" {
			return []any{map[string]any{}}
		}
		return nil
	}
	s.conn = conn
	initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := s.initialize(initCtx, version); err != nil {
		s.Close()
		return nil, fmt.Errorf("lsp %s: initialize: %w", name, err)
	}
	return s, nil
}

func (s *Session) Stderr() string {
	if s == nil {
		return ""
	}
	return s.errOut.String()
}

func (s *Session) initialize(ctx context.Context, version string) error {
	root := fileURI(s.Root)
	_, err := s.conn.call(ctx, "initialize", map[string]any{
		"processId": os.Getpid(),
		"clientInfo": map[string]string{
			"name":    "goppi",
			"version": version,
		},
		"rootUri": root,
		"capabilities": map[string]any{
			"workspace": map[string]any{
				"configuration":    true,
				"workspaceFolders": true,
			},
			"textDocument": map[string]any{
				"publishDiagnostics": map[string]any{"relatedInformation": true},
				"synchronization":    map[string]any{"didSave": true},
			},
			"window": map[string]any{"workDoneProgress": true},
		},
		"workspaceFolders": []map[string]string{{"uri": root, "name": filepath.Base(s.Root)}},
	})
	if err != nil {
		return err
	}
	return s.conn.notify("initialized", map[string]any{})
}

func (s *Session) handleNotify(method string, params json.RawMessage) {
	if method != "textDocument/publishDiagnostics" {
		return
	}
	var p struct {
		URI         string `json:"uri"`
		Diagnostics []struct {
			Range struct {
				Start struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"start"`
			} `json:"range"`
			Severity int    `json:"severity"`
			Source   string `json:"source"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}
	list := make([]Diagnostic, 0, len(p.Diagnostics))
	path := pathFromURI(p.URI)
	for _, d := range p.Diagnostics {
		list = append(list, Diagnostic{
			URI:      p.URI,
			Path:     path,
			Line:     d.Range.Start.Line + 1,
			Col:      d.Range.Start.Character + 1,
			Severity: severityName(d.Severity),
			Source:   d.Source,
			Message:  d.Message,
		})
	}
	s.mu.Lock()
	s.diags[p.URI] = list
	s.mu.Unlock()
	select {
	case s.updated <- p.URI:
	default:
	}
}

func StartAll(ctx context.Context, servers map[string]config.LSPServer, workdir, version string, hook CmdHook) (*Hub, []error) {
	hub := &Hub{}
	if os.Getenv("GOPPI_LSP") == "0" || strings.EqualFold(os.Getenv("GOPPI_LSP"), "off") {
		return hub, nil
	}
	var errs []error
	started := map[string]bool{}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 4 {
		names = names[:4]
	}
	for _, name := range names {
		spec := servers[name]
		s, err := Start(ctx, name, spec, workdir, version, hook)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		hub.add(s)
		started[filepath.Base(spec.Command)] = true
	}
	if !started["gopls"] {
		if auto, ok := autoGopls(workdir); ok {
			s, err := Start(ctx, "go", auto, workdir, version, hook)
			if err != nil {
				errs = append(errs, err)
			} else {
				hub.add(s)
			}
		}
	}
	return hub, errs
}

func formatDiags(workdir string, diags []Diagnostic) string {
	if len(diags) == 0 {
		return "(no diagnostics)"
	}
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].Path != diags[j].Path {
			return diags[i].Path < diags[j].Path
		}
		if diags[i].Line != diags[j].Line {
			return diags[i].Line < diags[j].Line
		}
		return diags[i].Col < diags[j].Col
	})
	var b strings.Builder
	for i, d := range diags {
		if i >= 80 {
			fmt.Fprintf(&b, "… %d more\n", len(diags)-80)
			break
		}
		src := d.Source
		if src == "" {
			src = "lsp"
		}
		fmt.Fprintf(&b, "%s:%d:%d [%s] %s: %s\n", displayPath(workdir, d.Path), d.Line, d.Col, d.Severity, src, d.Message)
	}
	return strings.TrimRight(b.String(), "\n")
}

func severityName(n int) string {
	switch n {
	case 1:
		return "error"
	case 2:
		return "warning"
	case 3:
		return "info"
	case 4:
		return "hint"
	default:
		return "error"
	}
}

func languageFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".py":
		return "python"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	default:
		return "plaintext"
	}
}

func fileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	return u.String()
}

func pathFromURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	return filepath.FromSlash(u.Path)
}

func displayPath(workdir, path string) string {
	if path == "" {
		return path
	}
	if rel, err := filepath.Rel(workdir, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}
