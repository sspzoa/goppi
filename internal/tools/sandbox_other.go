//go:build !darwin && !linux

package tools

import (
	"fmt"
	"os/exec"
)

func wrapSandbox(cmd *exec.Cmd, workdir, mode string, extra ...string) error {
	if !sandboxOn(mode) {
		return nil
	}
	return fmt.Errorf("sandbox workspace is not supported on this OS (set GOPPI_SANDBOX=off)")
}
