package provider

import (
	"strings"
)

// ContextOverflow reports whether err is a model context-window failure.
// Those are retried after Compact; other API errors are not.
func ContextOverflow(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, p := range []string{
		"context_length",
		"context length",
		"context window",
		"maximum context",
		"prompt is too long",
		"too many tokens",
		"token limit",
		"requested tokens exceed",
		"reduce the length",
	} {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
