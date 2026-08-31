//go:build unix

package mcp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestKillGroupReapsChildren(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	cmd := exec.Command("sh", "-c", "sleep 60 & echo $! >"+strconv.Quote(pidfile)+"; wait")
	setKillGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var child int
	for i := 0; i < 50; i++ {
		raw, err := os.ReadFile(pidfile)
		if err == nil {
			n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
			if err == nil && n > 0 {
				child = n
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if child == 0 {
		_ = cmd.Process.Kill()
		t.Fatal("child pid not written")
	}
	killGroup(cmd)
	_ = cmd.Wait()
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := syscall.Kill(child, 0)
		if err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("child %d still alive", child)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
