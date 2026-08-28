package ui

import (
	"fmt"
	"os"
)

// 고삐 브랜드: 주홍 고삐, 먹 바탕, 한지 글자.
const (
	Reset = "\033[0m"
	Bold  = "\033[1m"
	Dim   = "\033[2m"

	violetR, violetG, violetB = 194, 61, 42
	softR, softG, softB       = 224, 122, 95
	muteR, muteG, muteB       = 154, 145, 136
	okR, okG, okB             = 47, 143, 120
	warnR, warnG, warnB       = 201, 148, 58
	errR, errG, errB          = 196, 84, 74
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
	return os.Getenv("TERM") != "dumb"
}

func fg(r, g, b int) string {
	if !colorEnabled() {
		return ""
	}
	if truecolor() {
		return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
	}
	return "\033[38;5;166m"
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
