package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/sspzoa/goppi/internal/complete"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/upstage"
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

	case toolDoneMsg:
		if last := m.last(); last != nil && last.kind == kindTool {
			if msg.err != nil {
				last.state = "fail"
				last.note = msg.err.Error()
			} else {
				last.state = "ok"
				last.note = msg.summary
			}
		}
		return m, nil

	case doneMsg:
		m.busy = false
		m.phase = ""
		m.turnCancel = nil
		_ = m.input.Focus()
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			m.push(block{kind: kindError, body: msg.err.Error()})
		} else if errors.Is(msg.err, context.Canceled) {
			m.push(block{kind: kindSystem, body: "취소했습니다."})
		}
		if msg.save != nil {
			m.push(block{kind: kindSystem, body: "세션 저장 실패: " + msg.save.Error()})
		}
		return m, nil

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

	if m.overlay == overlayNone && !m.busy {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.syncSuggest()
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if m.overlay != overlayNone {
		return m.handleOverlayKey(k)
	}

	switch k {
	case "ctrl+c":
		if m.busy {
			if m.turnCancel != nil {
				m.turnCancel()
			}
			return m, nil
		}
		m.overlay = overlayQuit
		m.input.Blur()
		return m, nil
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
		if !m.busy {
			m.applySuggest(1)
			return m, nil
		}
	case "shift+tab":
		if !m.busy {
			m.applySuggest(-1)
			return m, nil
		}
	case "enter":
		if m.busy {
			return m, nil
		}
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

	if !m.busy {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.syncSuggest()
		return m, cmd
	}
	return m, nil
}

func (m *model) handleOverlayKey(k string) (tea.Model, tea.Cmd) {
	switch m.overlay {
	case overlayQuit:
		switch k {
		case "y", "enter":
			return m, m.quit()
		case "n", "esc", "ctrl+c":
			m.overlay = overlayNone
			_ = m.input.Focus()
		}
	case overlayPerm:
		switch k {
		case "y", "enter":
			m.replyPerm(true)
		case "n", "esc", "ctrl+c":
			m.replyPerm(false)
		}
	}
	return m, nil
}

func (m *model) replyPerm(ok bool) {
	if m.perm != nil && m.perm.reply != nil {
		select {
		case m.perm.reply <- ok:
		default:
		}
	}
	m.perm = nil
	m.overlay = overlayNone
	if m.busy {
		m.input.Blur()
	} else {
		_ = m.input.Focus()
	}
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
	m.push(block{kind: kindUser, body: text})
	m.stick = true
	return m, m.startTurn(text)
}

func (m *model) startTurn(prompt string) tea.Cmd {
	turnCtx, cancel := context.WithCancel(m.ctx)
	m.turnCancel = cancel
	m.busy = true
	m.phase = "생각 중"
	m.input.Blur()
	a := m.agent
	return func() tea.Msg {
		err := a.Run(turnCtx, prompt)
		var save error
		if !errors.Is(err, context.Canceled) {
			id, serr := session.Persist(a.Cfg, a.SessionID, a.Messages)
			save = serr
			if serr == nil {
				a.SessionID = id
			}
		}
		return doneMsg{err: err, save: save}
	}
}

func (m *model) quit() tea.Cmd {
	m.shutdown()
	return tea.Quit
}

func (m *model) applyDelta(reasoning, content string) {
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
		m.showSessions()
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
	default:
		m.push(block{kind: kindSystem, body: "모르는 명령입니다. /help"})
	}
	return nil
}

func (m *model) resetSession() {
	m.agent.Reset()
	m.agent.SessionID = session.NewID()
	m.agent.Cfg.PromptCacheKey = session.NewCacheKey()
	m.blocks = nil
	m.inTok, m.outTok, m.reasonTok = 0, 0, 0
	m.push(block{kind: kindSystem, body: "세션을 초기화했습니다. " + shortID(m.agent.SessionID)})
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

func (m *model) showSessions() {
	items, err := session.List()
	if err != nil {
		m.push(block{kind: kindError, body: err.Error()})
		return
	}
	if len(items) == 0 {
		m.push(block{kind: kindSystem, body: "세션이 없습니다."})
		return
	}
	var b strings.Builder
	limit := min(len(items), 8)
	for i := 0; i < limit; i++ {
		f := items[i]
		fmt.Fprintf(&b, "%s  %s  %s\n", shortID(f.ID), f.UpdatedAt.Local().Format("01-02 15:04"), f.Title)
	}
	m.push(block{kind: kindSystem, body: strings.TrimRight(b.String(), "\n")})
}

func (m *model) showStatus() {
	effort := m.agent.Cfg.ReasoningEffort
	if m.agent.Cfg.Model == "solar-mini" {
		effort = "n/a"
	}
	m.push(block{kind: kindSystem, body: fmt.Sprintf(
		"%s  ·  %s  ·  %s  ·  session %s\n%s",
		m.agent.Cfg.Model,
		effort,
		upstage.DefaultBaseURL,
		shortID(m.agent.SessionID),
		m.agent.Cfg.WorkDir,
	)})
}
