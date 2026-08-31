package provider

import (
	"errors"
	"testing"
)

func TestContextOverflow(t *testing.T) {
	yes := []string{
		"upstage 400: context_length_exceeded",
		"This model's maximum context length was exceeded",
		"prompt is too long",
		"too many tokens in the request",
		"requested tokens exceed the limit",
		"Please reduce the length of the messages",
	}
	for _, s := range yes {
		if !ContextOverflow(errors.New(s)) {
			t.Fatalf("expected overflow: %s", s)
		}
	}
	no := []string{"boom", "rate limit", "401 unauthorized", "tool input too large"}
	for _, s := range no {
		if ContextOverflow(errors.New(s)) {
			t.Fatalf("unexpected overflow: %s", s)
		}
	}
	if ContextOverflow(nil) {
		t.Fatal("nil")
	}
}
