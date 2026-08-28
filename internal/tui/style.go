package tui

import "charm.land/lipgloss/v2"

var (
	colViolet = lipgloss.Color("#C23D2A")
	colSoft   = lipgloss.Color("#E07A5F")
	colMute   = lipgloss.Color("#9A9188")
	colText   = lipgloss.Color("#F2EDE6")
	colInk    = lipgloss.Color("#161411")
	colLine   = lipgloss.Color("#3A342E")
	colOK     = lipgloss.Color("#2F8F78")
	colWarn   = lipgloss.Color("#C9943A")
	colErr    = lipgloss.Color("#C4544A")
	colPanel  = lipgloss.Color("#1E1B18")
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
