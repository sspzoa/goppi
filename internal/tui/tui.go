package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sspzoa/goppi/internal/agent"
	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/ui"
	"github.com/sspzoa/goppi/internal/upstage"
)

type overlay int

const (
	overlayNone overlay = iota
	overlayHelp
	overlayQuit
	overlayPerm
	overlayModel
	overlayEffort
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

type pickItem struct{ id, summary string }

type model struct {
	ctx        context.Context
	agent      *agent.Agent
	program    *tea.Program
	closed     chan struct{}
	closeOnce  sync.Once
	turnCancel context.CancelFunc

	width, height int
	st            styles
	vp            viewport.Model
	input         textarea.Model
	spin          spinner.Model

	blocks  []block
	overlay overlay
	perm    *permAskMsg
	picks   []pickItem
	pickIdx int
	pickFor overlay

	busy, stick              bool
	phase                    string
	inTok, outTok, reasonTok int
	hist                     []string
	histIdx                  int
	draft                    string
	status                   string
}

func Run(ctx context.Context, a *agent.Agent) error {
	m := newModel(ctx, a)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	m.program = p
	a.Sink = &bridge{send: p.Send}
	a.Tools.SetAsk(m.ask)
	_, err := p.Run()
	m.shutdown()
	return err
}

func newModel(ctx context.Context, a *agent.Agent) *model {
	st := newStyles()
	ta := textarea.New()
	ta.Placeholder = "메시지를 입력하세요"
	ta.ShowLineNumbers = false
	ta.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "› "
		}
		return "  "
	})
	ta.SetHeight(3)
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
	s.Cursor.Color = colViolet
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
	v.WindowTitle = "goppi"
	v.BackgroundColor = colInk
	v.ForegroundColor = colText
	return v
}

func (m *model) render() string {
	header := m.renderHeader()
	footer := m.renderFooter()
	input := m.renderInput()
	chrome := lipgloss.Height(header) + lipgloss.Height(input) + lipgloss.Height(footer) + 2
	vpH := m.height - chrome
	if vpH < 3 {
		vpH = 3
	}
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(vpH)
	m.refreshTranscript()

	body := m.vp.View()
	if m.overlay != overlayNone {
		body = lipgloss.Place(m.width, vpH, lipgloss.Center, lipgloss.Center, m.renderOverlay())
	}

	sep := m.st.sep.Render(strings.Repeat("─", max(m.width, 1)))
	return lipgloss.JoinVertical(lipgloss.Left, header, sep, body, input, footer)
}

func (m *model) renderHeader() string {
	left := m.st.brand.Render("goppi") + "  " + m.st.tag.Render("UPSTAGE SOLAR")
	right := m.st.mute.Render("v" + config.Version)
	top := lipgloss.JoinHorizontal(lipgloss.Top, left, spacer(m.width-lipgloss.Width(left)-lipgloss.Width(right)), right)

	effort := m.agent.Cfg.ReasoningEffort
	if m.agent.Cfg.Model == "solar-mini" {
		effort = "n/a"
	}
	parts := []string{
		m.agent.Cfg.Model,
		effort,
		ui.ShortPath(m.agent.Cfg.WorkDir),
	}
	if m.agent.SessionID != "" {
		parts = append(parts, shortID(m.agent.SessionID))
	}
	if m.inTok+m.outTok > 0 {
		parts = append(parts, fmt.Sprintf("%d tok", m.inTok+m.outTok))
	}
	meta := m.st.mute.Render(strings.Join(parts, "  ·  "))
	return top + "\n" + fit(meta, m.width)
}

func (m *model) renderInput() string {
	m.input.SetWidth(max(m.width-2, 10))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colViolet).
		Width(m.width)
	if m.busy || m.overlay != overlayNone {
		box = box.BorderForeground(colLine)
	}
	return box.Render(m.input.View())
}

func (m *model) renderFooter() string {
	if m.status != "" {
		return m.st.warn.Render(m.status)
	}
	if m.busy {
		return m.st.brand.Render(m.spin.View()) + " " + m.st.mute.Render(m.phaseLabel()) +
			m.st.mute.Render("   ctrl+c 취소")
	}
	return m.st.hint.Render("enter 보내기  ·  ctrl+j 줄바꿈  ·  ctrl+c 종료  ·  ? 도움말")
}

func (m *model) phaseLabel() string {
	if m.phase != "" {
		return m.phase
	}
	return "thinking"
}

func (m *model) renderOverlay() string {
	switch m.overlay {
	case overlayHelp:
		return m.st.modal.Width(min(m.width-4, 56)).Render(helpText())
	case overlayQuit:
		return m.st.modal.Width(min(m.width-4, 40)).Render(
			m.st.brand.Render("종료할까요?") + "\n\n" + m.st.mute.Render("enter / y  종료    n / esc  취소"),
		)
	case overlayPerm:
		name, detail := "", ""
		if m.perm != nil {
			name, detail = m.perm.name, m.perm.detail
		}
		body := m.st.warn.Render("allow tool?") + "\n\n" +
			m.st.brand.Render(name) + "\n" +
			m.st.mute.Render(detail) + "\n\n" +
			m.st.mute.Render("y 허용    n / esc 거부")
		return m.st.modal.Width(min(m.width-4, 64)).Render(body)
	case overlayModel, overlayEffort:
		return m.renderPicker()
	default:
		return ""
	}
}

func (m *model) renderPicker() string {
	var b strings.Builder
	title := "model"
	if m.overlay == overlayEffort {
		title = "effort"
	}
	b.WriteString(m.st.brand.Render(title))
	b.WriteString("\n\n")
	for i, it := range m.picks {
		mark := "  "
		line := it.id
		if it.summary != "" {
			line += "  " + it.summary
		}
		if i == m.pickIdx {
			mark = m.st.brand.Render("● ")
			line = m.st.brand.Render(it.id)
			if it.summary != "" {
				line += "  " + m.st.mute.Render(it.summary)
			}
		} else {
			mark = m.st.mute.Render("○ ")
			line = m.st.text.Render(it.id)
			if it.summary != "" {
				line += "  " + m.st.mute.Render(it.summary)
			}
		}
		b.WriteString(mark + line + "\n")
	}
	b.WriteString("\n" + m.st.mute.Render("j/k 이동  ·  enter 선택  ·  esc 닫기"))
	return m.st.modal.Width(min(m.width-4, 64)).Render(strings.TrimRight(b.String(), "\n"))
}

func helpText() string {
	return strings.TrimSpace(`
단축키
  enter        보내기
  ctrl+j       줄바꿈
  ctrl+c       생성 취소 / 종료
  pgup pgdn    스크롤
  ctrl+l       맨 아래로
  ctrl+n       새 세션
  ?            이 도움말

명령
  /help        도움말
  /model       Solar 모델
  /effort      reasoning 강도
  /new         세션 초기화
  /tools       등록된 툴
  /sessions    최근 세션
  /status      현재 설정
  /quit        종료
`)
}

func (m *model) refreshTranscript() {
	content := renderBlocks(m.st, m.blocks, max(m.width, 20))
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

func modelsForPicker() []pickItem {
	out := make([]pickItem, 0, len(upstage.ChatModels))
	for _, m := range upstage.ChatModels {
		out = append(out, pickItem{id: m.ID, summary: m.Summary})
	}
	return out
}

func effortsForPicker() []pickItem {
	out := make([]pickItem, 0, len(config.Efforts))
	for _, e := range config.Efforts {
		out = append(out, pickItem{id: e})
	}
	return out
}
