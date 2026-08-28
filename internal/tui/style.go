package tui

import "charm.land/lipgloss/v2"

var (
	colViolet = lipgloss.Color("#5B52FF")
	colSoft   = lipgloss.Color("#9B96FF")
	colMute   = lipgloss.Color("#8B8F98")
	colText   = lipgloss.Color("#E8E9ED")
	colInk    = lipgloss.Color("#0A0D14")
	colLine   = lipgloss.Color("#2A2E38")
	colOK     = lipgloss.Color("#2EB88A")
	colWarn   = lipgloss.Color("#D69E2E")
	colErr    = lipgloss.Color("#E85D5D")
	colPanel  = lipgloss.Color("#12151C")
)

type styles struct {
	brand, tag, mute, text, ok, warn, err, title, user, assistant, reason lipgloss.Style
	sep, card, modal, hint, spin                                          lipgloss.Style
}

func newStyles() styles {
	return styles{
		brand:     lipgloss.NewStyle().Foreground(colViolet).Bold(true),
		tag:       lipgloss.NewStyle().Foreground(colSoft),
		mute:      lipgloss.NewStyle().Foreground(colMute),
		text:      lipgloss.NewStyle().Foreground(colText),
		ok:        lipgloss.NewStyle().Foreground(colOK),
		warn:      lipgloss.NewStyle().Foreground(colWarn),
		err:       lipgloss.NewStyle().Foreground(colErr),
		title:     lipgloss.NewStyle().Foreground(colViolet).Bold(true),
		user:      lipgloss.NewStyle().Foreground(colViolet).Bold(true),
		assistant: lipgloss.NewStyle().Foreground(colSoft).Bold(true),
		reason:    lipgloss.NewStyle().Foreground(colMute).Italic(true),
		sep:       lipgloss.NewStyle().Foreground(colLine),
		card: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colLine).
			Padding(0, 1),
		modal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colViolet).
			Background(colPanel).
			Foreground(colText).
			Padding(1, 2),
		hint: lipgloss.NewStyle().Foreground(colMute),
		spin: lipgloss.NewStyle().Foreground(colViolet),
	}
}

func fit(s string, n int) string {
	if n <= 1 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	var out []rune
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw >= n-1 {
			break
		}
		out = append(out, r)
		w += rw
	}
	return string(out) + "…"
}
