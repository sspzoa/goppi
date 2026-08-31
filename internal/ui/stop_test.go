package ui

import (
	"os"
	"syscall"
	"testing"
)

func TestStopSignalsIncludeTerm(t *testing.T) {
	got := StopSignals()
	if len(got) == 0 {
		t.Fatal("empty")
	}
	for _, s := range got {
		if s == syscall.SIGTERM {
			return
		}
	}
	t.Fatalf("SIGTERM missing in %v", got)
}

func TestHeadlessStopSignalsIncludeInterrupt(t *testing.T) {
	for _, s := range HeadlessStopSignals() {
		if s == os.Interrupt {
			return
		}
	}
	t.Fatalf("SIGINT missing in %v", HeadlessStopSignals())
}
