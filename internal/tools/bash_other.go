//go:build !unix

package tools

import "os/exec"

func setKillGroup(*exec.Cmd) {}

func killGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
