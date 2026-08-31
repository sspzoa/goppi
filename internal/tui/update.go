package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/complete"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/skills"
	"github.com/sspzoa/goppi/internal/tools"
	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/worktree"
)

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case permAskMsg:
		m.overlay = overlayPerm
		m.perm = &msg
		m.phase = msg.name
		m.input.Blur()
		return m, nil

	case askUserMsg:
		m.overlay = overlayAsk
		m.askq = &msg
		m.phase = "ask_user"
		m.input.Blur()
		return m, nil

	case deltaMsg:
		m.applyDelta(msg.reasoning, msg.content)
		return m, nil

	case turnEndMsg:
		if last := m.last(); last != nil {
			last.live = false
		}
		return m, nil

	case usageMsg:
		m.inTok += msg.in
		m.outTok += msg.out
		m.reasonTok += msg.reason
		return m, nil

	case toolStartMsg:
		m.phase = msg.name
		m.push(block{kind: kindTool, title: msg.name, body: msg.detail, state: "running"})
		return m, nil

	case compactMsg:
		m.push(block{kind: kindSystem, body: "세션을 자동 압축했습니다."})
		return m, nil

	case toolDoneMsg:
		if last := m.last(); last != nil && last.kind == kindTool {
			if msg.err != nil {
				last.state = "fail"
				last.note = tools.RedactSecrets(msg.err.Error())
			} else {
				last.state = "ok"
				last.note = tools.RedactSecrets(msg.summary)
			}
		}
		return m, nil

	case doneMsg:
		return m, m.finishTurn(msg)

	case tea.MouseWheelMsg:
		if m.overlay != overlayNone {
			return m, nil
		}
		if msg.Button == tea.MouseWheelUp {
			m.stick = false
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		if m.vp.AtBottom() {
			m.stick = true
		}
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	if m.overlay == overlayNone {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.syncSuggest()
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := keyStroke(msg)

	if m.overlay != overlayNone {
		return m.handleOverlayKey(msg)
	}

	if isCtrlC(msg) {
		if m.busy {
			if m.turnCancel != nil {
				m.turnCancel()
			}
			return m, nil
		}
		m.overlay = overlayQuit
		m.input.Blur()
		return m, nil
	}

	switch k {
	case "ctrl+d":
		if strings.TrimSpace(m.input.Value()) == "" && !m.busy {
			return m, m.quit()
		}
	case "ctrl+n":
		if !m.busy {
			m.resetSession()
			return m, nil
		}
	case "ctrl+o":
		m.showReason = !m.showReason
		return m, nil
	case "ctrl+l":
		m.stick = true
		m.vp.GotoBottom()
		return m, nil
	case "pgup", "pgdown":
		if k == "pgup" {
			m.stick = false
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		if m.vp.AtBottom() {
			m.stick = true
		}
		return m, cmd
	case "?":
		if strings.TrimSpace(m.input.Value()) == "" {
			m.push(block{kind: kindSystem, body: helpText()})
			m.stick = true
			return m, nil
		}
	case "tab":
		m.applySuggest(1)
		return m, nil
	case "shift+tab":
		m.applySuggest(-1)
		return m, nil
	case "enter":
		if len(m.suggest) > 0 && !complete.Ready(m.input.Value()) {
			m.applySuggest(0)
			// 선택으로 라인이 완성되면 enter 한 번으로 바로 실행한다.
			if !complete.Ready(m.input.Value()) {
				m.preselectCurrent()
				return m, nil
			}
		}
		return m.submit()
	case "up":
		if len(m.suggest) > 0 {
			m.moveSuggest(-1)
			return m, nil
		}
		if m.input.LineCount() <= 1 && m.input.Line() == 0 {
			return m, m.histPrev()
		}
	case "down":
		if len(m.suggest) > 0 {
			m.moveSuggest(1)
			return m, nil
		}
		if m.input.LineCount() <= 1 {
			return m, m.histNext()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.syncSuggest()
	return m, cmd
}

func (m *model) handleOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := keyStroke(msg)
	switch m.overlay {
	case overlayQuit:
		if isCtrlC(msg) || k == "y" || k == "enter" {
			return m, m.quit()
		}
		if k == "n" || k == "esc" {
			m.overlay = overlayNone
			_ = m.input.Focus()
		}
	case overlayPerm:
		if isCtrlC(msg) {
			m.replyPerm(tools.Denied)
			if m.turnCancel != nil {
				m.turnCancel()
			}
			break
		}
		if k == "n" || k == "esc" {
			m.replyPerm(tools.Denied)
			break
		}
		if k == "a" {
			m.replyPerm(tools.AllowedSession)
			break
		}
		if k == "y" || k == "enter" {
			m.replyPerm(tools.Allowed)
		}
	case overlayAsk:
		if isCtrlC(msg) {
			m.replyAsk("", fmt.Errorf("user skipped"))
			if m.turnCancel != nil {
				m.turnCancel()
			}
			break
		}
		if k == "n" || k == "esc" {
			if m.askq != nil && len(m.askq.options) == 0 {
				m.replyAsk("no", nil)
			} else {
				m.replyAsk("", fmt.Errorf("user skipped"))
			}
			break
		}
		if m.askq != nil && len(m.askq.options) == 0 && (k == "y" || k == "enter") {
			m.replyAsk("yes", nil)
			break
		}
		if m.askq != nil && len(k) == 1 && k[0] >= '1' && k[0] <= '8' {
			i := int(k[0] - '1')
			if i < len(m.askq.options) {
				m.replyAsk(m.askq.options[i], nil)
			}
		}
	}
	return m, nil
}

func (m *model) replyAsk(text string, err error) {
	if m.askq != nil && m.askq.reply != nil {
		select {
		case m.askq.reply <- askUserReply{text: text, err: err}:
		default:
		}
	}
	m.askq = nil
	m.overlay = overlayNone
	_ = m.input.Focus()
}

// keyStroke is the v2-stable name for a shortcut. String() can be just
// "c" when the terminal reports printable text for ctrl+c.
func keyStroke(msg tea.KeyPressMsg) string {
	if s := msg.Keystroke(); s != "" {
		return s
	}
	return msg.String()
}

func isCtrlC(msg tea.KeyPressMsg) bool {
	k := msg.Key()
	if k.Mod.Contains(tea.ModCtrl) && (k.Code == 'c' || k.Code == 'C') {
		return true
	}
	s := keyStroke(msg)
	return s == "ctrl+c"
}

func (m *model) replyPerm(v tools.Verdict) {
	if m.perm != nil && m.perm.reply != nil {
		select {
		case m.perm.reply <- v:
		default:
		}
	}
	m.perm = nil
	m.overlay = overlayNone
	_ = m.input.Focus()
}

func (m *model) syncSuggest() {
	next := complete.Query(m.input.Value())
	if !sameSuggest(m.suggest, next) {
		m.suggestIdx = 0
	}
	if m.suggestIdx >= len(next) {
		m.suggestIdx = 0
	}
	m.suggest = next
}

func sameSuggest(a, b []complete.Item) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
	}
	return true
}

func (m *model) moveSuggest(dir int) {
	if len(m.suggest) == 0 {
		return
	}
	m.suggestIdx = (m.suggestIdx + dir + len(m.suggest)) % len(m.suggest)
}

func (m *model) applySuggest(dir int) {
	m.syncSuggest()
	if len(m.suggest) == 0 {
		return
	}
	cur := strings.TrimSpace(m.input.Value())
	if dir != 0 && strings.TrimSpace(complete.Apply(m.input.Value(), m.suggest[m.suggestIdx])) == cur && len(m.suggest) > 1 {
		m.moveSuggest(dir)
	}
	m.input.SetValue(complete.Apply(m.input.Value(), m.suggest[m.suggestIdx]))
	m.input.MoveToEnd()
	m.syncSuggest()
}

const maxQueuedPrompts = 4

func (m *model) submit() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}
	m.input.Reset()
	m.hist = append(m.hist, text)
	m.histIdx = -1
	m.draft = ""
	if strings.HasPrefix(text, "/") {
		return m, m.runSlash(text)
	}
	if m.busy {
		m.enqueue(text)
		return m, nil
	}
	m.push(block{kind: kindUser, body: text})
	m.stick = true
	return m, m.startTurn(text)
}

func (m *model) enqueue(text string) {
	if len(m.queue) >= maxQueuedPrompts {
		m.push(block{kind: kindSystem, body: "대기열이 가득입니다 (4)"})
		return
	}
	m.queue = append(m.queue, text)
	m.push(block{kind: kindSystem, body: fmt.Sprintf("대기 %d  ·  %s", len(m.queue), firstLine(text))})
}

func (m *model) dropQueue() {
	if len(m.queue) == 0 {
		return
	}
	n := len(m.queue)
	m.queue = nil
	m.push(block{kind: kindSystem, body: fmt.Sprintf("대기 %d개를 버렸습니다.", n)})
}

func (m *model) kickQueue() tea.Cmd {
	if len(m.queue) == 0 {
		_ = m.input.Focus()
		return nil
	}
	text := m.queue[0]
	m.queue = append([]string(nil), m.queue[1:]...)
	m.push(block{kind: kindUser, body: text})
	m.stick = true
	return m.startTurn(text)
}

func (m *model) finishTurn(msg doneMsg) tea.Cmd {
	m.busy = false
	m.phase = ""
	m.turnCancel = nil
	if msg.compact && msg.err == nil {
		m.blocks = blocksFromMessages(m.agent.Messages)
		m.push(block{kind: kindSystem, body: "세션을 압축했습니다."})
	} else if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
		m.push(block{kind: kindError, body: msg.err.Error()})
	} else if errors.Is(msg.err, context.Canceled) {
		m.push(block{kind: kindSystem, body: "취소했습니다."})
		m.dropQueue()
		_ = m.input.Focus()
		if msg.save != nil {
			m.push(block{kind: kindSystem, body: "세션 저장 실패: " + msg.save.Error()})
		}
		return nil
	}
	ui.NotifyDone()
	if msg.save != nil {
		m.push(block{kind: kindSystem, body: "세션 저장 실패: " + msg.save.Error()})
	}
	return m.kickQueue()
}

func (m *model) startTurn(prompt string) tea.Cmd {
	m.agent.EnsureSession()
	turnCtx, cancel := context.WithCancel(m.ctx)
	m.turnCancel = cancel
	m.busy = true
	m.phase = "생각 중"
	_ = m.input.Focus()
	a := m.agent
	end := m.trackWork()
	return func() tea.Msg {
		defer end()
		err := a.Run(turnCtx, prompt)
		id, save := session.PersistFull(a.Cfg, a.SessionSnapshot())
		if save == nil {
			a.SessionID = id
		}
		return doneMsg{err: err, save: save}
	}
}

func (m *model) quit() tea.Cmd {
	if err := persistCurrent(m.agent); err != nil {
		m.push(block{kind: kindError, body: "세션 저장 실패: " + err.Error()})
	}
	m.shutdown()
	return tea.Quit
}

func (m *model) applyDelta(reasoning, content string) {
	reasoning = tools.RedactSecrets(reasoning)
	content = tools.RedactSecrets(content)
	if reasoning != "" {
		m.phase = "생각 중"
		if last := m.last(); last != nil && last.kind == kindReason && last.live {
			last.body += reasoning
		} else {
			m.push(block{kind: kindReason, body: reasoning, live: true})
		}
	}
	if content != "" {
		m.phase = "쓰는 중"
		if last := m.last(); last != nil && last.kind == kindAssistant && last.live {
			last.body += content
		} else {
			m.push(block{kind: kindAssistant, body: content, live: true})
		}
	}
}

func (m *model) histPrev() tea.Cmd {
	if len(m.hist) == 0 {
		return nil
	}
	if m.histIdx < 0 {
		m.draft = m.input.Value()
		m.histIdx = len(m.hist) - 1
	} else if m.histIdx > 0 {
		m.histIdx--
	}
	m.input.SetValue(m.hist[m.histIdx])
	m.input.MoveToEnd()
	m.syncSuggest()
	return nil
}

func (m *model) histNext() tea.Cmd {
	if m.histIdx < 0 {
		return nil
	}
	if m.histIdx+1 >= len(m.hist) {
		m.histIdx = -1
		m.input.SetValue(m.draft)
		m.input.MoveToEnd()
		m.syncSuggest()
		return nil
	}
	m.histIdx++
	m.input.SetValue(m.hist[m.histIdx])
	m.input.MoveToEnd()
	m.syncSuggest()
	return nil
}

func (m *model) runSlash(line string) tea.Cmd {
	fields := strings.Fields(line)
	cmd := fields[0]
	arg := strings.TrimSpace(strings.TrimPrefix(line, cmd))
	if m.busy && !slashWhileBusy(cmd) {
		m.push(block{kind: kindSystem, body: cmd + " 는 턴이 끝난 뒤에"})
		return nil
	}
	switch cmd {
	case "/help", "/?":
		m.push(block{kind: kindSystem, body: helpText()})
		m.stick = true
	case "/quit", "/exit", "/q":
		m.overlay = overlayQuit
		m.input.Blur()
	case "/new", "/clear":
		m.resetSession()
	case "/tools":
		m.push(block{kind: kindSystem, body: strings.Join(m.agent.Tools.Names(), "  ·  ")})
	case "/sessions":
		if arg == "" {
			if len(complete.Sessions()) == 0 {
				m.push(block{kind: kindSystem, body: "세션이 없습니다."})
				return nil
			}
			m.startArgPick("/sessions ")
			return nil
		}
		if m.busy {
			m.push(block{kind: kindSystem, body: "/sessions 이어가기는 턴이 끝난 뒤에"})
			return nil
		}
		m.loadSession(arg)
	case "/delete":
		m.deleteSession(arg)
	case "/status":
		m.showStatus()
	case "/model":
		if arg == "" {
			m.startArgPick("/model ")
			return nil
		}
		m.setModel(arg)
	case "/effort":
		if arg == "" {
			m.startArgPick("/effort ")
			return nil
		}
		m.setEffort(arg)
	case "/plan":
		m.setMode("plan")
	case "/act":
		m.setMode("act")
	case "/undo":
		if m.agent.Tools == nil {
			m.push(block{kind: kindError, body: "nothing to undo"})
			return nil
		}
		msg, err := m.agent.Tools.UndoLast()
		if err != nil {
			m.push(block{kind: kindError, body: err.Error()})
			return nil
		}
		m.push(block{kind: kindSystem, body: msg})
	case "/compact":
		return m.startCompact()
	case "/skills":
		m.showSkills()
	case "/mcp":
		m.showMCP()
	case "/jobs":
		m.showJobs()
	case "/diff":
		m.showDiff()
	case "/export":
		m.exportSession(arg)
	case "/copy":
		m.copyLast()
	case "/retry":
		return m.retryTurn()
	default:
		m.push(block{kind: kindSystem, body: "모르는 명령입니다. /help"})
	}
	return nil
}

func (m *model) resetSession() {
	if err := persistCurrent(m.agent); err != nil {
		m.push(block{kind: kindError, body: "세션 저장 실패: " + err.Error()})
		return
	}
	m.agent.Reset()
	m.agent.Cfg.PromptCacheKey = session.NewCacheKey()
	m.agent.EnsureSession()
	m.blocks = nil
	m.inTok, m.outTok, m.reasonTok = 0, 0, 0
	m.queue = nil
	m.push(block{kind: kindSystem, body: "세션을 초기화했습니다. " + shortID(m.agent.SessionID)})
}

func persistCurrent(a *agent.Agent) error {
	if a == nil || len(a.Messages) == 0 {
		return nil
	}
	id, err := session.PersistFull(a.Cfg, a.SessionSnapshot())
	if err != nil {
		return fmt.Errorf("session save: %w", err)
	}
	a.SessionID = id
	return nil
}

// startArgPick fills the input with a slash command prefix so the
// autocomplete list becomes the picker.
func (m *model) startArgPick(prefix string) {
	m.input.SetValue(prefix)
	m.input.MoveToEnd()
	m.syncSuggest()
	m.preselectCurrent()
}

// preselectCurrent moves the suggestion cursor to the currently
// configured value when picking an argument for /model or /effort.
func (m *model) preselectCurrent() {
	fields := strings.Fields(m.input.Value())
	if len(fields) != 1 {
		return
	}
	var current string
	switch fields[0] {
	case "/model":
		current = m.agent.Cfg.Model
	case "/effort":
		current = m.agent.Cfg.ReasoningEffort
	case "/sessions":
		current = m.agent.SessionID
	default:
		return
	}
	for i, it := range m.suggest {
		if it.Name == current {
			m.suggestIdx = i
			break
		}
	}
}

func (m *model) setModel(id string) {
	m.agent.Cfg.Model = id
	if id == "solar-mini" {
		m.agent.Cfg.ReasoningEffort = ""
	}
	if err := m.agent.Cfg.Normalize(); err != nil {
		m.push(block{kind: kindError, body: err.Error()})
		return
	}
	m.push(block{kind: kindSystem, body: "model → " + m.agent.Cfg.Model})
}

func (m *model) setMode(mode string) {
	m.agent.Cfg.Mode = mode
	if m.agent.Tools != nil {
		m.agent.Tools.SetMode(mode)
	}
	m.push(block{kind: kindSystem, body: "mode → " + mode})
}

func (m *model) startCompact() tea.Cmd {
	if m.busy {
		return nil
	}
	m.busy = true
	m.phase = "압축 중"
	m.input.Blur()
	end := m.trackWork()
	return func() tea.Msg {
		defer end()
		err := m.agent.Compact(m.ctx)
		var save error
		if err == nil {
			id, serr := session.PersistFull(m.agent.Cfg, m.agent.SessionSnapshot())
			save = serr
			if serr == nil {
				m.agent.SessionID = id
			}
		}
		return doneMsg{err: err, save: save, compact: true}
	}
}

func (m *model) showMCP() {
	configured := m.agent.Cfg.MCPNames()
	var live []string
	if m.agent.Tools != nil {
		live = m.agent.Tools.MCPNames()
	}
	if len(configured) == 0 && len(live) == 0 {
		m.push(block{kind: kindSystem, body: "mcp 없음. ~/.config/goppi/config.json 의 mcp_servers"})
		return
	}
	var b strings.Builder
	if len(configured) > 0 {
		fmt.Fprintf(&b, "servers  %s", strings.Join(configured, ", "))
	}
	if len(live) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "tools    %s", strings.Join(live, ", "))
	}
	m.push(block{kind: kindSystem, body: b.String()})
}

func (m *model) showSkills() {
	names := skills.Names(skills.Load(m.agent.Cfg.WorkDir))
	if len(names) == 0 {
		m.push(block{kind: kindSystem, body: "skill 없음. .goppi/skills/<name>/SKILL.md"})
		return
	}
	m.push(block{kind: kindSystem, body: strings.Join(names, "  ·  ")})
}

func (m *model) setEffort(level string) {
	if m.agent.Cfg.Model == "solar-mini" {
		m.push(block{kind: kindSystem, body: "solar-mini 는 reasoning_effort 를 쓰지 않습니다"})
		return
	}
	m.agent.Cfg.ReasoningEffort = level
	if err := m.agent.Cfg.Normalize(); err != nil {
		m.push(block{kind: kindError, body: err.Error()})
		return
	}
	m.push(block{kind: kindSystem, body: "effort → " + m.agent.Cfg.ReasoningEffort})
}

func (m *model) deleteSession(arg string) {
	id := strings.TrimSpace(arg)
	if id == "" {
		id = m.agent.SessionID
	} else {
		f, err := session.Resolve(id)
		if err != nil {
			m.push(block{kind: kindError, body: err.Error()})
			return
		}
		id = f.ID
	}
	if id == "" {
		m.push(block{kind: kindSystem, body: "지울 세션이 없습니다."})
		return
	}
	current := id == m.agent.SessionID
	if current {
		m.agent.End("delete")
		m.agent.ReleaseSession()
	} else {
		_ = tools.FireSessionEnd(context.Background(), m.agent.Cfg, id, "delete")
	}
	if err := session.Delete(id); err != nil {
		m.push(block{kind: kindError, body: err.Error()})
		return
	}
	if err := worktree.Remove(id); err != nil {
		m.push(block{kind: kindSystem, body: "worktree: " + err.Error()})
	}
	if !current {
		m.push(block{kind: kindSystem, body: "세션을 지웠습니다. " + shortID(id)})
		return
	}
	m.agent.Discard()
	m.agent.Cfg.PromptCacheKey = session.NewCacheKey()
	m.agent.EnsureSession()
	m.blocks = nil
	m.inTok, m.outTok, m.reasonTok = 0, 0, 0
	m.queue = nil
	m.push(block{kind: kindSystem, body: "세션을 지웠습니다. " + shortID(m.agent.SessionID)})
}

func (m *model) loadSession(id string) {
	if err := persistCurrent(m.agent); err != nil {
		m.push(block{kind: kindError, body: "세션 저장 실패: " + err.Error()})
		return
	}
	f, err := session.Resolve(id)
	if err != nil {
		m.push(block{kind: kindError, body: err.Error()})
		return
	}
	if err := m.agent.LoadFile(f); err != nil {
		m.push(block{kind: kindError, body: err.Error()})
		return
	}
	m.blocks = blocksFromMessages(m.agent.Messages)
	m.inTok, m.outTok, m.reasonTok = m.agent.TotalUsage.InputTokens, m.agent.TotalUsage.OutputTokens, m.agent.TotalUsage.ReasoningTokens
	m.queue = nil
	m.stick = true
	m.push(block{kind: kindSystem, body: fmt.Sprintf("세션을 이었습니다. %s · %d messages", shortID(m.agent.SessionID), len(m.agent.Messages))})
}

func compactLabel(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func slashWhileBusy(cmd string) bool {
	switch cmd {
	case "/help", "/?", "/tools", "/sessions", "/status", "/jobs", "/diff", "/export", "/copy", "/mcp", "/skills", "/quit", "/exit", "/q":
		return true
	}
	return false
}

func (m *model) jobCounts() (running, total int) {
	if m.agent == nil || m.agent.Tools == nil {
		return 0, 0
	}
	return m.agent.Tools.JobCounts()
}

func (m *model) showDiff() {
	body := "(no edits)"
	if m.agent != nil && m.agent.Tools != nil {
		body = m.agent.Tools.SessionDiff()
	}
	m.push(block{kind: kindSystem, body: body})
}

func (m *model) retryTurn() tea.Cmd {
	if m.agent == nil {
		m.push(block{kind: kindError, body: "다시 보낼 메시지가 없습니다"})
		return nil
	}
	text, err := m.agent.RewindLastUser()
	if err != nil {
		m.push(block{kind: kindError, body: err.Error()})
		return nil
	}
	m.blocks = blocksFromMessages(m.agent.Messages)
	m.push(block{kind: kindUser, body: text})
	m.push(block{kind: kindSystem, body: "다시 보냅니다."})
	m.stick = true
	return m.startTurn(text)
}

func (m *model) copyLast() {
	if m.agent == nil {
		m.push(block{kind: kindError, body: "복사할 답이 없습니다"})
		return
	}
	text := m.agent.LastAssistant()
	if err := ui.CopyClipboard(text); err != nil {
		m.push(block{kind: kindError, body: err.Error()})
		return
	}
	m.push(block{kind: kindSystem, body: "클립보드에 복사했습니다."})
}

func (m *model) exportSession(arg string) {
	if m.agent == nil {
		m.push(block{kind: kindError, body: "내보낼 세션이 없습니다"})
		return
	}
	path, err := m.agent.ExportMarkdown(arg)
	if err != nil {
		m.push(block{kind: kindError, body: err.Error()})
		return
	}
	m.push(block{kind: kindSystem, body: "내보냄 " + path})
}

func (m *model) showJobs() {
	body := "(no jobs)"
	if m.agent != nil && m.agent.Tools != nil {
		body = m.agent.Tools.JobSummary()
	}
	m.push(block{kind: kindSystem, body: body})
}

func (m *model) showStatus() {
	effort := m.agent.Cfg.ReasoningEffort
	if m.agent.Cfg.Model == "solar-mini" {
		effort = "n/a"
	}
	run, total := m.jobCounts()
	u := m.agent.LastUsage
	m.push(block{kind: kindSystem, body: fmt.Sprintf(
		"%s  ·  %s  ·  %s  ·  sandbox %s  ·  worktree %v  ·  compact %s  ·  jobs %d/%d  ·  last %d→%d r%d  ·  Σ %d→%d  ·  %s  ·  session %s\n%s",
		m.agent.Cfg.Mode,
		m.agent.Cfg.Model,
		effort,
		m.agent.Cfg.Sandbox,
		m.agent.Cfg.Worktree,
		compactLabel(m.agent.Cfg.AutoCompact),
		run, total,
		u.InputTokens, u.OutputTokens, u.ReasoningTokens,
		m.inTok, m.outTok,
		m.agent.Cfg.BaseURL,
		shortID(m.agent.SessionID),
		m.agent.Cfg.WorkDir,
	)})
}
