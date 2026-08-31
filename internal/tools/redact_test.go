package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sspzoa/goppi/internal/provider"
)

func TestRedactSecretsEnvAndPatterns(t *testing.T) {
	t.Setenv("UPSTAGE_API_KEY", "up_env_secret_value")
	t.Setenv("SHORT_TOKEN", "abcd")
	got := RedactSecrets("key=up_env_secret_value sk-abcdefghijklmnopqrst and ghp_abcdefghijklmnopqrstuvwx leftover")
	if strings.Contains(got, "up_env_secret_value") || strings.Contains(got, "sk-abcdefgh") || strings.Contains(got, "ghp_") {
		t.Fatalf("%q", got)
	}
	if !strings.Contains(got, "[redacted]") {
		t.Fatalf("%q", got)
	}
	if RedactSecrets("SHORT_TOKEN=abcd") != "SHORT_TOKEN=abcd" {
		t.Fatal("short env values must stay")
	}
}

func TestReadFileRedactsEnvSecret(t *testing.T) {
	t.Setenv("UPSTAGE_API_KEY", "up_must_not_appear_in_file")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("UPSTAGE_API_KEY=up_must_not_appear_in_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := New(dir, nil, nil)
	in, _ := json.Marshal(map[string]string{"path": ".env"})
	out, err := reg.Run(context.Background(), "read_file", in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "up_must_not_appear_in_file") {
		t.Fatalf("leaked: %q", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("%q", out)
	}
}

func TestRedactMessagesCopies(t *testing.T) {
	in := []provider.Message{{
		Role:      provider.RoleUser,
		Content:   "use sk-abcdefghijklmnopqrst",
		Reasoning: "think up_abcdefghijklmnop",
		ToolCalls: []provider.ToolCall{{Name: "bash", Input: json.RawMessage(`{"command":"echo sk-abcdefghijklmnopqrst"}`)}},
	}}
	out := RedactMessages(in)
	if strings.Contains(out[0].Content, "sk-") || strings.Contains(string(out[0].ToolCalls[0].Input), "sk-") {
		t.Fatalf("%+v", out[0])
	}
	if !strings.Contains(in[0].Content, "sk-abcdefghijklmnopqrst") {
		t.Fatal("must not mutate the live transcript")
	}
}
