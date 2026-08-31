//go:build !unix

package ui

import (
	"os"
	"syscall"
)

func StopSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
