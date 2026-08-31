package tui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"sync"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/complete"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/ui"
)

type overlay int

const (
	overlayNone overlay = iota
	overlayQuit
	overlayPerm
)

type permAskMsg struct {
	name, detail string
	reply        chan bool
}

type deltaMsg struct{ reasoning, content string }
type turnEndMsg struct{}
type usageMsg struct{ in, out, reason int }
type toolStartMsg struct{ name, detail string }
type toolDoneMsg struct {
	summary string
	err     error
}
type doneMsg struct{ err, save error }

type model struct {
	ctx        context.Context
	agent      *agent.Agent
	program    *tea.Program
	closed     chan struct{}
	closeOnce  sync.Once
	turnCancel context.CancelFunc

	width, height int
	vpH           int
	st            styles
	vp            viewport.Model
	input         textarea.Model
	spin          spinner.Model

	blocks  []block
	overlay overlay
	perm    *permAskMsg

	busy, stick, showReason  bool
	phase                    string
	inTok, outTok, reasonTok int
	hist                     []string
	histIdx                  int
	draft                    string
	status                   string
	suggest                  []complete.Item
	suggestIdx               int
}

func Run(ctx context.Context, a *agent.Agent) error {
	m := newModel(ctx, a)
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithFilter(keepCtrlC))
	m.program = p
	a.Sink = &bridge{send: p.Send}
	a.Tools.SetAsk(m.ask)
	_, err := p.Run()
	m.shutdown()
	return err
}

// keepCtrlC turns SIGINT into a key so the model can cancel a turn or
// confirm quit. Bubble Tea v2 otherwise exits immediately on InterruptMsg.
func keepCtrlC(_ tea.Model, msg tea.Msg) tea.Msg {
	if _, ok := msg.(tea.InterruptMsg); ok {
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	return msg
}

func newModel(ctx context.Context, a *agent.Agent) *model {
	st := newStyles()
	ta := textarea.New()
	ta.Placeholder = "메시지를 입력하세요"
	ta.ShowLineNumbers = false
	ta.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "❯ "
		}
		return "  "
	})
	ta.SetHeight(1)
	ta.CharLimit = 0
	ta.SetVirtualCursor(true)
	km := textarea.DefaultKeyMap()
	km.InsertNewline = key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "newline"))
	ta.KeyMap = km
	applyInputStyles(&ta, st)

	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(st.spin))
	vp := viewport.New()
	vp.SoftWrap = true
	vp.KeyMap = viewport.KeyMap{
		PageDown: key.NewBinding(key.WithKeys("pgdown")),
		PageUp:   key.NewBinding(key.WithKeys("pgup")),
	}

	m := &model{
		ctx:     ctx,
		agent:   a,
		closed:  make(chan struct{}),
		st:      st,
		vp:      vp,
		input:   ta,
		spin:    sp,
		blocks:  blocksFromMessages(a.Messages),
		stick:   true,
		histIdx: -1,
	}
	if a.SessionID != "" && len(a.Messages) > 0 {
		m.blocks = append([]block{{
			kind: kindSystem,
			body: fmt.Sprintf("세션 %s · %d messages", shortID(a.SessionID), len(a.Messages)),
		}}, m.blocks...)
	}
	return m
}

func applyInputStyles(ta *textarea.Model, st styles) {
	s := textarea.DefaultDarkStyles()
	s.Focused.Prompt = st.brand
	s.Focused.Text = st.text
	s.Focused.Placeholder = st.mute
	s.Focused.CursorLine = lipgloss.NewStyle()
	s.Focused.Base = lipgloss.NewStyle()
	s.Blurred.Prompt = st.mute
	s.Blurred.Text = st.mute
	s.Blurred.Placeholder = st.mute
	s.Blurred.CursorLine = lipgloss.NewStyle()
	s.Cursor.Color = colBrand
	ta.SetStyles(s)
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.input.Focus(), m.spin.Tick, tea.RequestWindowSize)
}

func (m *model) View() tea.View {
	if m.width == 0 {
		m.width, m.height = 80, 24
	}
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "고삐"
	v.BackgroundColor = colInk
	v.ForegroundColor = colText
	return v
}

func (m *model) render() string {
	header := m.renderHeader()
	sep := m.st.sep.Render(strings.Repeat("─", max(m.width, 1)))
	footer := m.renderFooter()
	action := m.renderAction() // 입력창 또는 패널
	suggest := m.renderSuggest()

	chrome := lipgloss.Height(header) + 1 + lipgloss.Height(action) + lipgloss.Height(footer)
	if suggest != "" {
		chrome += lipgloss.Height(suggest)
	}
	vpH := m.height - chrome
	if vpH < 3 {
		vpH = 3
	}
	m.vpH = vpH
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(vpH)
	m.refreshTranscript()

	parts := []string{header, sep, m.vp.View()}
	if suggest != "" {
		parts = append(parts, suggest)
	}
	parts = append(parts, action, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *model) renderHeader() string {
	left := m.st.brand.Render("고삐") + " " + m.st.tag.Render("한국형")
	right := m.st.mute.Render("v" + config.Version)

	effort := m.agent.Cfg.ReasoningEffort
	if m.agent.Cfg.Model == "solar-mini" {
		effort = "n/a"
	}
	parts := []string{m.agent.Cfg.Model, effort, ui.ShortPath(m.agent.Cfg.WorkDir)}
	if m.agent.SessionID != "" {
		parts = append(parts, shortID(m.agent.SessionID))
	}
	if m.inTok+m.outTok > 0 {
		parts = append(parts, fmt.Sprintf("%d tok", m.inTok+m.outTok))
	}
	room := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	meta := m.st.mute.Render(fit(strings.Join(parts, " · "), max(room, 0)))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(meta) - lipgloss.Width(right) - 2
	return left + "  " + meta + spacer(gap) + right
}

// renderAction renders the input box, or the panel that replaces it
// while a decision (permission, picker, quit) is pending.
func (m *model) renderAction() string {
	switch m.overlay {
	case overlayPerm:
		return m.renderPermPanel()
	case overlayQuit:
		return m.renderQuitPanel()
	}
	return m.renderInput()
}

func (m *model) renderInput() string {
	h := m.input.LineCount()
	if h < 1 {
		h = 1
	}
	if h > 5 {
		h = 5
	}
	m.input.SetHeight(h)
	m.input.SetWidth(max(m.width-2, 10))
	border := colBrand
	if m.busy {
		border = colLine
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(m.width - 2).
		Render(m.input.View())
}

func (m *model) panel(border color.Color, lines ...string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(m.width - 2).
		Render(strings.Join(lines, "\n"))
}

func (m *model) renderPermPanel() string {
	name, detail := "", ""
	if m.perm != nil {
		name, detail = m.perm.name, m.perm.detail
	}
	lines := []string{
		m.st.warn.Render("툴 실행을 허용할까요?") + "  " + m.st.brand.Render(name),
	}
	if d := firstLine(detail); d != "" {
		lines = append(lines, m.st.mute.Render(fit(d, max(m.width-6, 8))))
	}
	return m.panel(colWarn, lines...)
}

func (m *model) renderQuitPanel() string {
	return m.panel(colBrand, m.st.brand.Render("종료할까요?"))
}

func (m *model) renderSuggest() string {
	if m.overlay != overlayNone || m.busy || len(m.suggest) == 0 {
		return ""
	}
	const maxRows = 6
	start := 0
	if m.suggestIdx >= maxRows {
		start = m.suggestIdx - maxRows + 1
	}
	end := min(len(m.suggest), start+maxRows)
	var b strings.Builder
	for i := start; i < end; i++ {
		it := m.suggest[i]
		name := fmt.Sprintf("%-12s", it.Name)
		line := name + " " + it.Summary
		if i == m.suggestIdx {
			b.WriteString(" " + m.st.brand.Render("❯ "+fit(line, max(m.width-4, 8))))
		} else {
			b.WriteString(" " + m.st.mute.Render("  "+fit(line, max(m.width-4, 8))))
		}
		if i+1 < end {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (m *model) renderFooter() string {
	if m.status != "" {
		return " " + m.st.warn.Render(m.status)
	}
	if m.busy && m.overlay == overlayNone {
		return " " + m.spin.View() + " " + m.st.mute.Render(m.phaseLabel()+"   ctrl+c 취소")
	}
	switch m.overlay {
	case overlayPerm:
		return " " + m.st.hint.Render("y 허용  ·  n / esc 거부")
	case overlayQuit:
		return " " + m.st.hint.Render("y / enter / ctrl+c 종료  ·  n / esc 취소")
	}
	if len(m.suggest) > 0 {
		return " " + m.st.hint.Render("tab 완성  ·  ↑↓ 선택  ·  enter 실행")
	}
	return " " + m.st.hint.Render("enter 보내기  ·  / 명령  ·  tab 완성  ·  ? 도움말")
}

func (m *model) phaseLabel() string {
	if m.phase != "" {
		return m.phase
	}
	return "생각 중"
}

func helpText() string {
	return strings.TrimSpace(`
단축키
  enter        보내기
  tab          명령 자동완성
  ↑↓           제안 선택 / 입력 히스토리
  ctrl+j       줄바꿈
  ctrl+o       생각(reasoning) 펼치기/접기
  ctrl+c       생성 취소 / 종료
  pgup pgdn    스크롤
  ctrl+l       맨 아래로
  ctrl+n       새 세션
  ?            이 도움말

명령
  /help        도움말
  /model       모델
  /effort      reasoning 강도
  /new         세션 초기화
  /tools       등록된 툴
  /sessions    최근 세션
  /status      현재 설정
  /quit        종료`)
}

func (m *model) refreshTranscript() {
	var content string
	if len(m.blocks) == 0 {
		effort := m.agent.Cfg.ReasoningEffort
		if m.agent.Cfg.Model == "solar-mini" {
			effort = "n/a"
		}
		content = renderWelcome(m.st, m.width, m.vpH, m.agent.Cfg.Model, effort, ui.ShortPath(m.agent.Cfg.WorkDir))
	} else {
		body := renderBlocks(m.st, m.blocks, max(m.width-2, 20), m.spin.View(), m.showReason)
		content = lipgloss.NewStyle().PaddingLeft(1).Render(body)
	}
	follow := m.stick || m.vp.AtBottom()
	m.vp.SetContent(content)
	if follow {
		m.vp.GotoBottom()
		m.stick = true
	}
}

func (m *model) layout() {
	if m.width < 1 {
		m.width = 80
	}
	if m.height < 1 {
		m.height = 24
	}
	m.input.SetWidth(max(m.width-2, 10))
}

func (m *model) push(bl block) {
	m.blocks = append(m.blocks, bl)
}

func (m *model) last() *block {
	if len(m.blocks) == 0 {
		return nil
	}
	return &m.blocks[len(m.blocks)-1]
}

func (m *model) ask(name, detail string) bool {
	reply := make(chan bool, 1)
	if m.program == nil {
		return false
	}
	m.program.Send(permAskMsg{name: name, detail: detail, reply: reply})
	select {
	case ok := <-reply:
		return ok
	case <-m.ctx.Done():
		return false
	case <-m.closed:
		return false
	}
}

func (m *model) shutdown() {
	m.closeOnce.Do(func() { close(m.closed) })
	if m.turnCancel != nil {
		m.turnCancel()
	}
	if m.perm != nil && m.perm.reply != nil {
		select {
		case m.perm.reply <- false:
		default:
		}
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func spacer(n int) string {
	if n < 1 {
		return " "
	}
	return strings.Repeat(" ", n)
}

type bridge struct {
	send func(tea.Msg)
}

func (b *bridge) Delta(reasoning, content string) {
	b.send(deltaMsg{reasoning: reasoning, content: content})
}
func (b *bridge) TurnEnd() { b.send(turnEndMsg{}) }
func (b *bridge) Usage(in, out, reason int) {
	b.send(usageMsg{in: in, out: out, reason: reason})
}
func (b *bridge) ToolStart(name, detail string) {
	b.send(toolStartMsg{name: name, detail: detail})
}
func (b *bridge) ToolDone(summary string, err error) {
	b.send(toolDoneMsg{summary: summary, err: err})
}

