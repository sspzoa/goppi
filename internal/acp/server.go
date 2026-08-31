package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/base64"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/tools"
	"github.com/sspzoa/goppi/internal/upstage"
	"github.com/sspzoa/goppi/internal/worktree"
)

const protocolVersion = 1

type NewFunc func(config.Config) (*agent.Agent, error)

type Server struct {
	In  io.Reader
	Out io.Writer
	Ctx context.Context
	Cfg config.Config
	New NewFunc

	mu       sync.Mutex
	conn     *Conn
	sessions map[string]*agent.Agent
	cancel   map[string]context.CancelFunc
	done     map[string]chan struct{}
	inflight sync.WaitGroup
}

func (s *Server) serveCtx() context.Context {
	if s != nil && s.Ctx != nil {
		return s.Ctx
	}
	return context.Background()
}

func (s *Server) Serve() error {
	if s.New == nil {
		return fmt.Errorf("acp: missing agent factory")
	}
	if s.In == nil {
		s.In = os.Stdin
	}
	if s.Out == nil {
		s.Out = os.Stdout
	}
	s.sessions = map[string]*agent.Agent{}
	s.cancel = map[string]context.CancelFunc{}
	s.done = map[string]chan struct{}{}
	s.conn = newConn(s.In, s.Out)
	ctx, stop := context.WithCancel(s.serveCtx())
	defer stop()
	defer s.closeAll()
	go func() {
		<-ctx.Done()
		if c, ok := s.In.(io.Closer); ok {
			_ = c.Close()
		}
	}()
	for {
		msg, err := s.conn.read()
		if err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return nil
			}
			return err
		}
		if msg.Method == "" {
			continue
		}
		switch msg.Method {
		case "session/prompt":
			go s.handle(msg)
		case "session/cancel":
			s.handleCancel(msg)
		default:
			s.handle(msg)
		}
	}
}

func (s *Server) handle(msg incoming) {
	if !hasJSONID(msg.ID) {
		return
	}
	var err error
	switch msg.Method {
	case "authenticate":
		err = s.conn.reply(msg.ID, map[string]any{})
	case "initialize":
		err = s.conn.reply(msg.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"agentCapabilities": map[string]any{
				"loadSession":        true,
				"promptCapabilities": map[string]any{"image": true, "audio": false, "embeddedContext": true},
				"sessionCapabilities": map[string]any{
					"list":                  map[string]any{},
					"close":                 map[string]any{},
					"delete":                map[string]any{},
					"resume":                map[string]any{},
					"additionalDirectories": map[string]any{},
				},
			},
			"agentInfo": map[string]string{"name": "goppi", "title": "고삐", "version": config.Version},
		})
	case "session/new":
		err = s.sessionNew(msg)
	case "session/load":
		err = s.sessionLoad(msg)
	case "session/resume":
		err = s.sessionResume(msg)
	case "session/prompt":
		err = s.sessionPrompt(msg)
	case "session/set_mode":
		err = s.sessionSetMode(msg)
	case "session/set_config_option":
		err = s.sessionSetConfig(msg)
	case "session/list":
		err = s.sessionList(msg)
	case "session/close":
		err = s.sessionClose(msg)
	case "session/delete":
		err = s.sessionDelete(msg)
	default:
		err = s.conn.replyErr(msg.ID, -32601, "method not found: "+msg.Method)
	}
	if err != nil {
		if s.serveCtx().Err() != nil {
			return
		}
		_ = s.conn.replyErr(msg.ID, -32000, err.Error())
	}
}

func (s *Server) sessionNew(msg incoming) error {
	var p struct {
		Cwd                   string          `json:"cwd"`
		MCPServers            json.RawMessage `json:"mcpServers"`
		AdditionalDirectories []string        `json:"additionalDirectories"`
	}
	_ = json.Unmarshal(msg.Params, &p)
	cfg := s.Cfg
	if cwd := strings.TrimSpace(p.Cwd); cwd != "" {
		if !filepath.IsAbs(cwd) {
			return fmt.Errorf("cwd must be an absolute path")
		}
		abs, err := filepath.Abs(cwd)
		if err != nil {
			return err
		}
		cfg.WorkDir = abs
	}
	cfg.ExtraDirs = p.AdditionalDirectories
	if err := cfg.Normalize(); err != nil {
		return err
	}
	applyClientMCP(&cfg, p.MCPServers)
	a, err := s.New(cfg)
	if err != nil {
		return err
	}
	if a.SessionID == "" {
		a.SessionID = session.NewID()
	}
	a.EnsureSession()
	s.attachAsk(a)
	s.mu.Lock()
	s.sessions[a.SessionID] = a
	s.mu.Unlock()
	return s.conn.reply(msg.ID, sessionState(a))
}

func (s *Server) sessionLoad(msg incoming) error {
	return s.restoreSession(msg, true)
}

func (s *Server) sessionResume(msg incoming) error {
	return s.restoreSession(msg, false)
}

func (s *Server) restoreSession(msg incoming, replay bool) error {
	var p struct {
		Cwd                   string          `json:"cwd"`
		SessionID             string          `json:"sessionId"`
		MCPServers            json.RawMessage `json:"mcpServers"`
		AdditionalDirectories []string        `json:"additionalDirectories"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	f, err := session.Load(p.SessionID)
	if err != nil {
		return err
	}
	cfg := s.Cfg
	if f.WorkDir != "" {
		cfg.WorkDir = f.WorkDir
	}
	if cwd := strings.TrimSpace(p.Cwd); cwd != "" {
		if !filepath.IsAbs(cwd) {
			return fmt.Errorf("cwd must be an absolute path")
		}
		abs, err := filepath.Abs(cwd)
		if err != nil {
			return err
		}
		if f.WorkDir != "" && !sameCwd(f.WorkDir, abs) {
			return fmt.Errorf("cwd does not match session")
		}
		cfg.WorkDir = abs
	}
	cfg.ExtraDirs = p.AdditionalDirectories
	if len(cfg.ExtraDirs) == 0 && len(f.ExtraDirs) > 0 {
		cfg.ExtraDirs = append([]string(nil), f.ExtraDirs...)
	}
	if err := cfg.Normalize(); err != nil {
		return err
	}
	applyClientMCP(&cfg, p.MCPServers)
	a, err := s.New(cfg)
	if err != nil {
		return err
	}
	if s.getSession(f.ID) != nil {
		_ = s.closeSession(f.ID)
	}
	if err := a.LoadFile(f); err != nil {
		a.Close()
		return err
	}
	s.attachAsk(a)
	s.mu.Lock()
	s.sessions[a.SessionID] = a
	s.mu.Unlock()
	if replay {
		s.replayHistory(a)
	}
	return s.conn.reply(msg.ID, sessionState(a))
}

func (s *Server) replayHistory(a *agent.Agent) {
	if a == nil || s.conn == nil {
		return
	}
	for _, m := range a.Messages {
		switch m.Role {
		case provider.RoleUser:
			if strings.TrimSpace(m.Content) == "" {
				continue
			}
			_ = s.conn.notify("session/update", map[string]any{
				"sessionId": a.SessionID,
				"update": map[string]any{
					"sessionUpdate": "user_message_chunk",
					"content":       map[string]string{"type": "text", "text": tools.RedactSecrets(m.Content)},
				},
			})
		case provider.RoleAssistant:
			if m.Reasoning != "" {
				_ = s.conn.notify("session/update", map[string]any{
					"sessionId": a.SessionID,
					"update": map[string]any{
						"sessionUpdate": "agent_thought_chunk",
						"content":       map[string]string{"type": "text", "text": tools.RedactSecrets(m.Reasoning)},
					},
				})
			}
			if strings.TrimSpace(m.Content) != "" {
				_ = s.conn.notify("session/update", map[string]any{
					"sessionId": a.SessionID,
					"update": map[string]any{
						"sessionUpdate": "agent_message_chunk",
						"content":       map[string]string{"type": "text", "text": tools.RedactSecrets(m.Content)},
					},
				})
			}
		}
	}
}

const sessionListPage = 32

func (s *Server) sessionList(msg incoming) error {
	var p struct {
		Cwd    string `json:"cwd"`
		Cursor string `json:"cursor"`
	}
	_ = json.Unmarshal(msg.Params, &p)
	items, err := session.List()
	if err != nil {
		return err
	}
	var infos []map[string]any
	seen := map[string]bool{}
	for _, f := range items {
		if p.Cwd != "" && !sameCwd(f.WorkDir, p.Cwd) {
			continue
		}
		seen[f.ID] = true
		infos = append(infos, sessionInfo(f))
	}
	s.mu.Lock()
	for id, a := range s.sessions {
		if seen[id] {
			continue
		}
		if p.Cwd != "" && !sameCwd(a.Cfg.WorkDir, p.Cwd) {
			continue
		}
		info := map[string]any{"sessionId": id, "cwd": a.Cfg.WorkDir}
		if title := session.TitleFrom(a.Messages); title != "" && title != "untitled" {
			info["title"] = title
		}
		infos = append(infos, info)
	}
	s.mu.Unlock()

	off := 0
	if p.Cursor != "" {
		n, err := strconv.Atoi(p.Cursor)
		if err != nil || n < 0 {
			return fmt.Errorf("invalid cursor")
		}
		off = n
	}
	if off > len(infos) {
		off = len(infos)
	}
	end := off + sessionListPage
	if end > len(infos) {
		end = len(infos)
	}
	page := infos[off:end]
	if page == nil {
		page = []map[string]any{}
	}
	out := map[string]any{"sessions": page}
	if end < len(infos) {
		out["nextCursor"] = strconv.Itoa(end)
	}
	return s.conn.reply(msg.ID, out)
}

func sessionInfo(f session.File) map[string]any {
	info := map[string]any{
		"sessionId": f.ID,
		"cwd":       f.WorkDir,
	}
	if title := session.SafeTitle(f.Title); title != "" {
		info["title"] = title
	}
	if !f.UpdatedAt.IsZero() {
		info["updatedAt"] = f.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return info
}

func sameCwd(a, b string) bool {
	aa, err := filepath.Abs(a)
	if err != nil {
		return a == b
	}
	bb, err := filepath.Abs(b)
	if err != nil {
		return a == b
	}
	if ra, err := filepath.EvalSymlinks(aa); err == nil {
		aa = ra
	}
	if rb, err := filepath.EvalSymlinks(bb); err == nil {
		bb = rb
	}
	return aa == bb
}

func (s *Server) sessionClose(msg incoming) error {
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	if err := s.releaseSession(p.SessionID, true); err != nil {
		return err
	}
	return s.conn.reply(msg.ID, map[string]any{})
}

func (s *Server) sessionDelete(msg incoming) error {
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	id := strings.TrimSpace(p.SessionID)
	if !session.ValidID(id) {
		if f, err := session.Resolve(id); err == nil {
			id = f.ID
		} else {
			return s.conn.reply(msg.ID, map[string]any{})
		}
	}
	if a := s.getSession(id); a != nil {
		s.stopPrompt(id)
		a.End("delete")
		_ = s.releaseSession(id, false)
	} else {
		_ = tools.FireSessionEnd(context.Background(), s.Cfg, id, "delete")
	}
	_ = session.Delete(id)
	_ = worktree.Remove(id)
	return s.conn.reply(msg.ID, map[string]any{})
}

func (s *Server) closeAll() {
	s.mu.Lock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	cancels := make([]context.CancelFunc, 0, len(s.cancel))
	for _, c := range s.cancel {
		if c != nil {
			cancels = append(cancels, c)
		}
	}
	s.mu.Unlock()
	for _, c := range cancels {
		c()
	}
	s.inflight.Wait()
	for _, id := range ids {
		_ = s.releaseSession(id, true)
	}
}

func (s *Server) closeSession(id string) error {
	return s.releaseSession(id, true)
}

func (s *Server) stopPrompt(id string) {
	s.mu.Lock()
	cancel := s.cancel[id]
	ch := s.done[id]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if ch != nil {
		<-ch
	}
}

func (s *Server) releaseSession(id string, persist bool) error {
	s.stopPrompt(id)
	s.mu.Lock()
	a := s.sessions[id]
	delete(s.sessions, id)
	s.mu.Unlock()
	if a == nil {
		if persist {
			return fmt.Errorf("unknown session %s", id)
		}
		return nil
	}
	var persistErr error
	if persist && len(a.Messages) > 0 {
		persistErr = a.Save()
	}
	a.Close()
	return persistErr
}

func (s *Server) getSession(id string) *agent.Agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}

func (s *Server) sessionBusy(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancel[id] != nil
}

func sessionMode(a *agent.Agent) string {
	if a == nil || a.Cfg.Mode == "" {
		return "act"
	}
	return a.Cfg.Mode
}

func sessionState(a *agent.Agent) map[string]any {
	out := map[string]any{
		"sessionId":     a.SessionID,
		"modes":         sessionModes(a),
		"configOptions": sessionConfigOptions(a),
	}
	if a != nil && len(a.Cfg.ExtraDirs) > 0 {
		out["additionalDirectories"] = a.Cfg.ExtraDirs
	}
	return out
}

func sessionModes(a *agent.Agent) map[string]any {
	return map[string]any{
		"currentModeId": sessionMode(a),
		"availableModes": []map[string]string{
			{"id": "act", "name": "Act", "description": "Write and run tools"},
			{"id": "plan", "name": "Plan", "description": "Read-only planning"},
		},
	}
}

func sessionConfigOptions(a *agent.Agent) []map[string]any {
	mode := sessionMode(a)
	models := make([]map[string]string, 0, len(upstage.ChatModels))
	for _, m := range upstage.ChatModels {
		models = append(models, map[string]string{"value": m.ID, "name": m.ID})
	}
	opts := []map[string]any{
		{
			"id":           "mode",
			"name":         "Mode",
			"category":     "mode",
			"type":         "select",
			"currentValue": mode,
			"options": []map[string]string{
				{"value": "act", "name": "Act"},
				{"value": "plan", "name": "Plan"},
			},
		},
		{
			"id":           "model",
			"name":         "Model",
			"category":     "model",
			"type":         "select",
			"currentValue": a.Cfg.Model,
			"options":      models,
		},
	}
	if a.Cfg.Model != "solar-mini" {
		efforts := make([]map[string]string, 0, len(config.Efforts))
		for _, e := range config.Efforts {
			efforts = append(efforts, map[string]string{"value": e, "name": e})
		}
		cur := a.Cfg.ReasoningEffort
		if cur == "" {
			cur = "medium"
		}
		opts = append(opts, map[string]any{
			"id":           "effort",
			"name":         "Reasoning",
			"category":     "thought_level",
			"type":         "select",
			"currentValue": cur,
			"options":      efforts,
		})
	}
	return opts
}

func applyMode(a *agent.Agent, mode string) error {
	if mode != "act" && mode != "plan" {
		return fmt.Errorf("unknown mode %s", mode)
	}
	a.Cfg.Mode = mode
	if a.Tools != nil {
		a.Tools.SetMode(mode)
	}
	return nil
}

func applyConfig(a *agent.Agent, id, value string) error {
	switch id {
	case "mode":
		return applyMode(a, value)
	case "model":
		a.Cfg.Model = value
		if value == "solar-mini" {
			a.Cfg.ReasoningEffort = ""
		}
		return a.Cfg.Normalize()
	case "effort":
		if a.Cfg.Model == "solar-mini" {
			return fmt.Errorf("solar-mini does not use reasoning_effort")
		}
		a.Cfg.ReasoningEffort = value
		return a.Cfg.Normalize()
	default:
		return fmt.Errorf("unknown config %s", id)
	}
}

func (s *Server) sessionSetMode(msg incoming) error {
	var p struct {
		SessionID string `json:"sessionId"`
		ModeID    string `json:"modeId"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	a := s.getSession(p.SessionID)
	if a == nil {
		return fmt.Errorf("unknown session %s", p.SessionID)
	}
	if s.sessionBusy(p.SessionID) {
		return fmt.Errorf("session %s is busy", p.SessionID)
	}
	if err := applyMode(a, p.ModeID); err != nil {
		return err
	}
	_ = s.conn.notify("session/update", map[string]any{
		"sessionId": p.SessionID,
		"update":    map[string]any{"sessionUpdate": "current_mode_update", "modeId": p.ModeID},
	})
	return s.conn.reply(msg.ID, sessionState(a))
}

func (s *Server) sessionSetConfig(msg incoming) error {
	var p struct {
		SessionID string `json:"sessionId"`
		ConfigID  string `json:"configId"`
		Value     any    `json:"value"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	a := s.getSession(p.SessionID)
	if a == nil {
		return fmt.Errorf("unknown session %s", p.SessionID)
	}
	if s.sessionBusy(p.SessionID) {
		return fmt.Errorf("session %s is busy", p.SessionID)
	}
	value, _ := p.Value.(string)
	if err := applyConfig(a, p.ConfigID, value); err != nil {
		return err
	}
	_ = s.conn.notify("session/update", map[string]any{
		"sessionId": p.SessionID,
		"update": map[string]any{
			"sessionUpdate": "config_option_update",
			"configOptions": sessionConfigOptions(a),
		},
	})
	if p.ConfigID == "mode" {
		_ = s.conn.notify("session/update", map[string]any{
			"sessionId": p.SessionID,
			"update":    map[string]any{"sessionUpdate": "current_mode_update", "modeId": value},
		})
	}
	return s.conn.reply(msg.ID, sessionState(a))
}

func (s *Server) attachAsk(a *agent.Agent) {
	if a == nil || a.Tools == nil || a.Cfg.AlwaysApprove {
		return
	}
	id := a.SessionID
	a.Tools.SetAsk(func(name, detail string) tools.Verdict {
		return s.ask(id, name, detail)
	})
	a.Tools.SetAskUser(func(question string, options []string) (string, error) {
		return s.askUser(id, question, options)
	})
}

func (s *Server) sessionPrompt(msg incoming) error {
	s.inflight.Add(1)
	defer s.inflight.Done()
	var p struct {
		SessionID string `json:"sessionId"`
		Prompt    []struct {
			Type     string          `json:"type"`
			Text     string          `json:"text"`
			Data     string          `json:"data"`
			MimeType string          `json:"mimeType"`
			URI      string          `json:"uri"`
			Name     string          `json:"name"`
			Title    string          `json:"title"`
			Resource json.RawMessage `json:"resource"`
		} `json:"prompt"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	s.mu.Lock()
	a := s.sessions[p.SessionID]
	if a == nil {
		s.mu.Unlock()
		return fmt.Errorf("unknown session %s", p.SessionID)
	}
	if s.cancel[p.SessionID] != nil {
		s.mu.Unlock()
		return fmt.Errorf("session %s is busy", p.SessionID)
	}
	parent := s.serveCtx()
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	s.cancel[p.SessionID] = cancel
	s.done[p.SessionID] = done
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.cancel, p.SessionID)
		delete(s.done, p.SessionID)
		s.mu.Unlock()
		close(done)
	}()
	var b strings.Builder
	var imgs []provider.Image
	var attached int
	for _, part := range p.Prompt {
		switch part.Type {
		case "text", "":
			b.WriteString(part.Text)
		case "image":
			if len(imgs) >= 3 {
				continue
			}
			img, err := imageFromACP(part.Data, part.MimeType)
			if err != nil {
				return err
			}
			imgs = append(imgs, img)
		case "resource":
			if attached >= maxACPResources {
				continue
			}
			block, img, err := embeddedFromACP(part.Resource)
			if err != nil {
				return err
			}
			if img != nil && len(imgs) < 3 {
				imgs = append(imgs, *img)
			}
			if block != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(block)
				attached++
			}
		case "resource_link":
			if attached >= maxACPResources {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(resourceLinkBlock(a.Cfg.WorkDir, a.Cfg.ExtraDirs, part.URI, part.Name, part.Title, part.MimeType))
			attached++
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" && len(imgs) == 0 {
		return fmt.Errorf("empty prompt")
	}
	if text == "" {
		text = "이 이미지를 봐."
	}
	a.Incoming = imgs

	a.Quiet = true
	a.Sink = acpSink{conn: s.conn, sessionID: p.SessionID}
	runErr := a.Run(ctx, text)
	if parent.Err() != nil {
		return parent.Err()
	}
	if err := a.Save(); err != nil {
		return err
	}
	if runErr != nil && strings.Contains(runErr.Error(), "session save") {
		return runErr
	}

	reason := "end_turn"
	if runErr != nil {
		if ctx.Err() != nil || strings.Contains(runErr.Error(), "canceled") || strings.Contains(runErr.Error(), "cancelled") {
			reason = "cancelled"
		} else if strings.Contains(runErr.Error(), "stopped after") {
			reason = "max_turn_requests"
		} else {
			return runErr
		}
	}
	return s.conn.reply(msg.ID, map[string]any{"stopReason": reason})
}

func (s *Server) handleCancel(msg incoming) {
	var p struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(msg.Params, &p)
	s.mu.Lock()
	fn := s.cancel[p.SessionID]
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (s *Server) ask(sessionID, name, detail string) tools.Verdict {
	raw, err := s.conn.call("session/request_permission", map[string]any{
		"sessionId": sessionID,
		"toolCall": map[string]any{
			"toolCallId": name,
			"title":      name,
			"kind":       toolKind(name),
			"rawInput":   map[string]string{"detail": tools.RedactSecrets(detail)},
		},
		"options": []map[string]string{
			{"optionId": "allow-once", "name": "Allow once", "kind": "allow_once"},
			{"optionId": "allow-session", "name": "Allow for session", "kind": "allow_always"},
			{"optionId": "reject-once", "name": "Reject", "kind": "reject_once"},
		},
	})
	if err != nil {
		return tools.Denied
	}
	var out struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	if json.Unmarshal(raw, &out) != nil || out.Outcome.Outcome != "selected" {
		return tools.Denied
	}
	switch out.Outcome.OptionID {
	case "allow-session", "allow_always", "allow-always":
		return tools.AllowedSession
	case "allow-once", "allow_once":
		return tools.Allowed
	default:
		return tools.Denied
	}
}

func (s *Server) askUser(sessionID, question string, options []string) (string, error) {
	if len(options) == 0 {
		options = []string{"yes", "no"}
	}
	opts := make([]map[string]string, 0, len(options))
	for i, o := range options {
		opts = append(opts, map[string]string{
			"optionId": fmt.Sprintf("opt-%d", i+1),
			"name":     o,
			"kind":     "allow_once",
		})
	}
	raw, err := s.conn.call("session/request_permission", map[string]any{
		"sessionId": sessionID,
		"toolCall": map[string]any{
			"toolCallId": "ask_user",
			"title":      tools.RedactSecrets(question),
			"kind":       "other",
		},
		"options": opts,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	if json.Unmarshal(raw, &out) != nil || out.Outcome.Outcome != "selected" {
		return "", fmt.Errorf("user skipped")
	}
	var n int
	if _, err := fmt.Sscanf(out.Outcome.OptionID, "opt-%d", &n); err == nil && n >= 1 && n <= len(options) {
		return options[n-1], nil
	}
	return "", fmt.Errorf("user skipped")
}

func toolKind(name string) string {
	switch name {
	case "bash":
		return "execute"
	case "write_file", "edit_file", "apply_patch":
		return "edit"
	default:
		if strings.HasPrefix(name, "mcp_") {
			return "other"
		}
		return "other"
	}
}

func hasJSONID(id json.RawMessage) bool {
	s := strings.TrimSpace(string(id))
	return s != "" && s != "null"
}

type acpSink struct {
	conn      *Conn
	sessionID string
}

func (s acpSink) Delta(reasoning, content string) {
	if reasoning != "" {
		_ = s.conn.notify("session/update", map[string]any{
			"sessionId": s.sessionID,
			"update": map[string]any{
				"sessionUpdate": "agent_thought_chunk",
				"content":       map[string]string{"type": "text", "text": tools.RedactSecrets(reasoning)},
			},
		})
	}
	if content != "" {
		_ = s.conn.notify("session/update", map[string]any{
			"sessionId": s.sessionID,
			"update": map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]string{"type": "text", "text": tools.RedactSecrets(content)},
			},
		})
	}
}
func (acpSink) TurnEnd()            {}
func (acpSink) Usage(int, int, int) {}
func (s acpSink) Compacted() {
	_ = s.conn.notify("session/update", map[string]any{
		"sessionId": s.sessionID,
		"update": map[string]any{
			"sessionUpdate": "agent_thought_chunk",
			"content":       map[string]string{"type": "text", "text": "auto-compacted"},
		},
	})
}
func (s acpSink) ToolStart(name, detail string) {
	_ = s.conn.notify("session/update", map[string]any{
		"sessionId": s.sessionID,
		"update": map[string]any{
			"sessionUpdate": "tool_call",
			"toolCallId":    name,
			"title":         name + " " + tools.RedactSecrets(detail),
			"kind":          toolKind(name),
			"status":        "in_progress",
		},
	})
}
func (s acpSink) ToolDone(summary string, err error) {
	status := "completed"
	if err != nil {
		status = "failed"
		if summary == "" {
			summary = err.Error()
		}
	}
	_ = s.conn.notify("session/update", map[string]any{
		"sessionId": s.sessionID,
		"update": map[string]any{
			"sessionUpdate": "tool_call_update",
			"status":        status,
			"content":       []map[string]any{{"type": "content", "content": map[string]string{"type": "text", "text": tools.RedactSecrets(summary)}}},
		},
	})
}

const (
	maxACPResources     = 8
	maxACPResourceBytes = 256 << 10
)

type embeddedResource struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
	Blob     string `json:"blob"`
}

func embeddedFromACP(raw json.RawMessage) (string, *provider.Image, error) {
	if len(raw) == 0 {
		return "", nil, fmt.Errorf("resource: empty")
	}
	var res embeddedResource
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", nil, fmt.Errorf("resource: %w", err)
	}
	if res.Blob != "" && strings.HasPrefix(res.MimeType, "image/") {
		img, err := imageFromACP(res.Blob, res.MimeType)
		if err != nil {
			return "", nil, err
		}
		return formatResource(res.URI, res.MimeType, "(image attached)"), &img, nil
	}
	text := res.Text
	if text == "" && res.Blob != "" {
		return formatResource(res.URI, res.MimeType, "(binary omitted)"), nil, nil
	}
	if text == "" {
		return formatResource(res.URI, res.MimeType, ""), nil, nil
	}
	return formatResource(res.URI, res.MimeType, clipResource(text)), nil, nil
}

func resourceLinkBlock(workdir string, extra []string, uri, name, title, mime string) string {
	label := name
	if label == "" {
		label = title
	}
	if label == "" {
		label = uri
	}
	body, err := readFileURI(workdir, extra, uri, maxACPResourceBytes)
	if err != nil {
		note := label
		if mime != "" {
			note += " " + mime
		}
		return formatResource(uri, mime, "(link) "+note)
	}
	return formatResource(uri, mime, clipResource(body))
}

func formatResource(uri, mime, body string) string {
	var b strings.Builder
	b.WriteString("[resource")
	if uri != "" {
		fmt.Fprintf(&b, " uri=%q", uri)
	}
	if mime != "" {
		fmt.Fprintf(&b, " mime=%q", mime)
	}
	b.WriteByte(']')
	if body != "" {
		b.WriteByte('\n')
		b.WriteString(body)
	}
	b.WriteString("\n[/resource]")
	return b.String()
}

func clipResource(s string) string {
	if len(s) <= maxACPResourceBytes {
		return s
	}
	return s[:maxACPResourceBytes] + "\n… truncated"
}

func readFileURI(workdir string, extra []string, uri string, max int) (string, error) {
	path, err := fileURIPath(uri)
	if err != nil {
		return "", err
	}
	abs, err := tools.ResolveInRoots(workdir, extra, path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	if len(data) > max {
		data = data[:max]
	}
	return string(data), nil
}

func fileURIPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("not a file uri")
	}
	p := u.Path
	if p == "" {
		return "", fmt.Errorf("empty file uri")
	}
	return filepath.Clean(p), nil
}

func imageFromACP(data, mime string) (provider.Image, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(data))
	}
	if err != nil {
		return provider.Image{}, fmt.Errorf("image: bad base64")
	}
	img, err := tools.ImageFromBytes(raw, mime)
	if err != nil {
		return provider.Image{}, fmt.Errorf("image: %w", err)
	}
	return img, nil
}
