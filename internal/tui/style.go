package tui

import "charm.land/lipgloss/v2"

var (
	colBrand = lipgloss.Color("#C23D2A") // 주홍
	colSoft  = lipgloss.Color("#E07A5F")
	colMute  = lipgloss.Color("#9A9188")
	colText  = lipgloss.Color("#F2EDE6") // 한지
	colInk   = lipgloss.Color("#161411") // 먹
	colLine  = lipgloss.Color("#3A342E")
	colOK    = lipgloss.Color("#2F8F78")
	colWarn  = lipgloss.Color("#C9943A")
	colErr   = lipgloss.Color("#C4544A")
)

type styles struct {
	brand, tag, mute, text, ok, warn, err, reason, hint, spin, sep lipgloss.Style
}

func newStyles() styles {
	return styles{
		brand:  lipgloss.NewStyle().Foreground(colBrand).Bold(true),
		tag:    lipgloss.NewStyle().Foreground(colSoft),
		mute:   lipgloss.NewStyle().Foreground(colMute),
		text:   lipgloss.NewStyle().Foreground(colText),
		ok:     lipgloss.NewStyle().Foreground(colOK),
		warn:   lipgloss.NewStyle().Foreground(colWarn),
		err:    lipgloss.NewStyle().Foreground(colErr),
		reason: lipgloss.NewStyle().Foreground(colMute).Italic(true),
		hint:   lipgloss.NewStyle().Foreground(colMute),
		spin:   lipgloss.NewStyle().Foreground(colBrand),
		sep:    lipgloss.NewStyle().Foreground(colLine),
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
