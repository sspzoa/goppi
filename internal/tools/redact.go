package tools

import (
	"os"
	"regexp"
	"strings"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
)

var secretPat = regexp.MustCompile(`(?i)(` +
	`sk-[A-Za-z0-9_-]{10,}|` +
	`up_[A-Za-z0-9_-]{10,}|` +
	`ghp_[A-Za-z0-9]{20,}|` +
	`github_pat_[A-Za-z0-9_]{20,}|` +
	`xox[baprs]-[A-Za-z0-9-]{10,}|` +
	`AKIA[0-9A-Z]{16}|` +
	`AIza[0-9A-Za-z_-]{35}` +
	`)`)

func RedactSecrets(s string) string {
	if s == "" {
		return s
	}
	out := s
	for _, e := range os.Environ() {
		k, v, ok := strings.Cut(e, "=")
		if !ok || !config.SecretEnvKey(k) {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) < 8 {
			continue
		}
		out = strings.ReplaceAll(out, v, "[redacted]")
	}
	return secretPat.ReplaceAllString(out, "[redacted]")
}

func RedactMessages(msgs []provider.Message) []provider.Message {
	if len(msgs) == 0 {
		return msgs
	}
	out := make([]provider.Message, len(msgs))
	copy(out, msgs)
	for i := range out {
		out[i].Content = RedactSecrets(out[i].Content)
		out[i].Reasoning = RedactSecrets(out[i].Reasoning)
		if n := len(out[i].ToolCalls); n > 0 {
			tcs := make([]provider.ToolCall, n)
			copy(tcs, out[i].ToolCalls)
			for j := range tcs {
				if len(tcs[j].Input) > 0 {
					tcs[j].Input = []byte(RedactSecrets(string(tcs[j].Input)))
				}
			}
			out[i].ToolCalls = tcs
		}
	}
	return out
}
