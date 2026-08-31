//go:build darwin

package tools

import (
	"strings"
	"testing"
)

func TestDarwinProfileDeniesSudo(t *testing.T) {
	p := darwinProfile(t.TempDir(), "workspace")
	if !strings.Contains(p, "process-exec") || !strings.Contains(p, "sudo") {
		t.Fatalf("profile missing sudo deny:\n%s", p)
	}
}
