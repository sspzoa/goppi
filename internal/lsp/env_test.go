package lsp

import (
	"strings"
	"testing"
)

func TestCleanEnvDropsSecrets(t *testing.T) {
	got := cleanEnv([]string{
		"PATH=/bin",
		"UPSTAGE_API_KEY=up_secret",
		"GOPPI_API_KEY=up_other",
		"GITHUB_TOKEN=ghp_x",
		"AWS_SECRET_ACCESS_KEY=w",
		"HOME=/tmp",
	})
	joined := strings.Join(got, "\n")
	for _, leak := range []string{"UPSTAGE_API_KEY", "GOPPI_API_KEY", "GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY"} {
		if strings.Contains(joined, leak) {
			t.Fatalf("leaked %s: %q", leak, got)
		}
	}
	if !strings.Contains(joined, "PATH=/bin") || !strings.Contains(joined, "HOME=/tmp") {
		t.Fatalf("%q", got)
	}
}
