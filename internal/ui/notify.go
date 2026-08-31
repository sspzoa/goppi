package ui

import (
	"fmt"
	"os"
	"strings"
)

func NotifyEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GOPPI_NOTIFY"))) {
	case "0", "off", "false", "no":
		return false
	case "1", "on", "true", "yes":
		return true
	default:
		return isCharDevice(os.Stderr)
	}
}

func isCharDevice(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// NotifyDone rings the terminal when a turn finishes and the user
// may have looked away. GOPPI_NOTIFY=off disables it.
func NotifyDone() {
	if !NotifyEnabled() {
		return
	}
	fmt.Fprint(os.Stderr, "\a")
	if os.Getenv("TERM_PROGRAM") == "iTerm.app" || os.Getenv("ITERM_SESSION_ID") != "" {
		fmt.Fprint(os.Stderr, "\033]9;고삐\007")
	}
}
