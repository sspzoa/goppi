//go:build unix

package ui

import (
	"syscall"
	"testing"
)

func TestStopSignalsIncludeHUP(t *testing.T) {
	for _, s := range StopSignals() {
		if s == syscall.SIGHUP {
			return
		}
	}
	t.Fatal("SIGHUP missing")
}
