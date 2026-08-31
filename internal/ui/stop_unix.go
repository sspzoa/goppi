//go:build unix

package ui

import (
	"os"
	"syscall"
)

func StopSignals() []os.Signal {
	return []os.Signal{syscall.SIGTERM, syscall.SIGHUP}
}
