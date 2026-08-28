package ui

import (
	"fmt"
	"os"
)

// Upstage public brand tokens (2026 site):
// primary action #5B52FF, ink #0A0D14, body #52525B, border #CDD0D5.
const (
	Reset = "\033[0m"
	Bold  = "\033[1m"
	Dim   = "\033[2m"

	violetR, violetG, violetB = 91, 82, 255
	softR, softG, softB       = 155, 150, 255
	muteR, muteG, muteB       = 139, 143, 152
	okR, okG, okB             = 46, 184, 138
	warnR, warnG, warnB       = 214, 158, 46
	errR, errG, errB          = 232, 93, 93
)

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func truecolor() bool {
	if !colorEnabled() {
		return false
	}
	switch os.Getenv("COLORTERM") {
	case "truecolor", "24bit":
		return true
	}
	// Most modern terminals accept 24-bit even without COLORTERM.
	return os.Getenv("TERM") != "dumb"
}

func fg(r, g, b int) string {
	if !colorEnabled() {
		return ""
	}
	if truecolor() {
		return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
	}
	return "\033[38;5;105m"
}

func Violet() string { return fg(violetR, violetG, violetB) }
func Soft() string   { return fg(softR, softG, softB) }
func Mute() string   { return fg(muteR, muteG, muteB) }
func OK() string     { return fg(okR, okG, okB) }
func WarnC() string  { return fg(warnR, warnG, warnB) }
func ErrC() string   { return fg(errR, errG, errB) }

func Paint(code, s string) string {
	if !colorEnabled() || code == "" {
		return s
	}
	return code + s + Reset
}

func brand(s string) string { return Paint(Bold+Violet(), s) }
func mute(s string) string  { return Paint(Mute(), s) }
func soft(s string) string  { return Paint(Soft(), s) }
