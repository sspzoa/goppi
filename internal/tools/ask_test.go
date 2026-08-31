package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAskUserHeadless(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	_, err := reg.Run(context.Background(), "ask_user", json.RawMessage(`{"question":"어느 쪽?"}`))
	if err == nil || !strings.Contains(err.Error(), "headless") {
		t.Fatalf("got %v", err)
	}
}

func TestAskUserReturnsAnswer(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	reg.SetAskUser(func(q string, opts []string) (string, error) {
		if q != "어느 쪽?" || len(opts) != 2 {
			t.Fatalf("%q %v", q, opts)
		}
		return opts[1], nil
	})
	in, _ := json.Marshal(map[string]any{"question": "어느 쪽?", "options": []string{"A", "B"}})
	out, err := reg.Run(context.Background(), "ask_user", in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "B") {
		t.Fatalf("%q", out)
	}
}

func TestAskUserEmptyQuestion(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	reg.SetAskUser(func(string, []string) (string, error) { return "x", nil })
	_, err := reg.Run(context.Background(), "ask_user", json.RawMessage(`{"question":"  "}`))
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("got %v", err)
	}
}

func TestAskUserPlanAllowed(t *testing.T) {
	reg := New(t.TempDir(), nil, nil)
	reg.SetMode("plan")
	reg.SetAskUser(func(string, []string) (string, error) { return "ok", nil })
	out, err := reg.Run(context.Background(), "ask_user", json.RawMessage(`{"question":"계속?"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("%q", out)
	}
}

func TestClipAskOptions(t *testing.T) {
	in := make([]string, 12)
	for i := range in {
		in[i] = "x"
	}
	if got := clipAskOptions(in); len(got) != maxAskOptions {
		t.Fatalf("%d", len(got))
	}
	if clipAskOptions([]string{"", " a "})[0] != "a" {
		t.Fatal("trim")
	}
}

func TestAskUserNotParallelSafe(t *testing.T) {
	if ParallelSafe("ask_user") {
		t.Fatal("ask_user blocks")
	}
}
