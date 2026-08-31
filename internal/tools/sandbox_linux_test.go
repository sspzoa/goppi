//go:build linux

package tools

import (
	"os/exec"
	"strings"
	"testing"
)

func TestWrapSandboxScrubsFallbackEnv(t *testing.T) {
	t.Setenv("UPSTAGE_API_KEY", "up_must_not_appear")
	t.Setenv("GITHUB_TOKEN", "ghp_must_not")
	dir := t.TempDir()
	cmd := exec.Command("true")
	if err := wrapSandbox(cmd, dir, "workspace"); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd.Env, "\n")
	for _, leak := range []string{"UPSTAGE_API_KEY", "GITHUB_TOKEN", "up_must_not_appear"} {
		if strings.Contains(joined, leak) {
			t.Fatalf("leaked %s: %q", leak, joined)
		}
	}
	if !strings.Contains(joined, "GOPPI_SANDBOX_HELPER=1") {
		t.Fatal("missing helper env")
	}
}
