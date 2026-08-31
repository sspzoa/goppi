package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReadSSEAndAccumulate(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning":"먼저 "}}]}`,
		``,
		`data: {"choices":[{"delta":{"reasoning":"계산한다.\n"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"답은 "}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"12다."},"finish_reason":"stop"}]}`,
		``,
		`data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":20,"completion_tokens_details":{"reasoning_tokens":5}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var acc streamAcc
	var reasons, contents []string
	err := readSSE(strings.NewReader(raw), func(b []byte) error {
		beforeR, beforeC := acc.reasoning.Len(), acc.content.Len()
		if err := acc.apply(b); err != nil {
			return err
		}
		if acc.reasoning.Len() > beforeR {
			reasons = append(reasons, acc.reasoning.String()[beforeR:])
		}
		if acc.content.Len() > beforeC {
			contents = append(contents, acc.content.String()[beforeC:])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out := acc.response()
	if out.Message.Reasoning != "먼저 계산한다." {
		t.Fatalf("reasoning = %q", out.Message.Reasoning)
	}
	if out.Message.Content != "답은 12다." {
		t.Fatalf("content = %q", out.Message.Content)
	}
	if out.StopReason != "stop" {
		t.Fatalf("stop = %q", out.StopReason)
	}
	if out.Usage.ReasoningTokens != 5 || out.Usage.InputTokens != 10 {
		t.Fatalf("usage = %+v", out.Usage)
	}
	if strings.Join(reasons, "") != "먼저 계산한다.\n" {
		t.Fatalf("streamed reasoning = %q", reasons)
	}
	if strings.Join(contents, "") != "답은 12다." {
		t.Fatalf("streamed content = %q", contents)
	}
}

func TestStreamToolCalls(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"bash","arguments":"{\"com"}}]}}]}`,
		``,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"mand\":\"ls\"}"}}]},"finish_reason":"tool_calls"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var acc streamAcc
	if err := readSSE(strings.NewReader(raw), acc.apply); err != nil {
		t.Fatal(err)
	}
	out := acc.response()
	if len(out.Message.ToolCalls) != 1 {
		t.Fatalf("calls = %d", len(out.Message.ToolCalls))
	}
	tc := out.Message.ToolCalls[0]
	if tc.Name != "bash" || tc.ID != "call_1" {
		t.Fatalf("call = %+v", tc)
	}
	if string(tc.Input) != `{"command":"ls"}` {
		t.Fatalf("args = %s", tc.Input)
	}
}

func TestStreamAccRejectsHuge(t *testing.T) {
	var acc streamAcc
	chunk, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"delta": map[string]any{"content": strings.Repeat("x", 8<<20+1)}}},
	})
	if err := acc.apply(chunk); err == nil {
		t.Fatal("expected stream too large")
	}
}

func TestStreamAccRejectsHugeToolArgs(t *testing.T) {
	var acc streamAcc
	chunk, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"delta": map[string]any{
			"tool_calls": []any{map[string]any{
				"index": 0,
				"id":    "c1",
				"function": map[string]any{
					"name":      "write_file",
					"arguments": `{"contents":"` + strings.Repeat("a", 8<<20+1) + `"}`,
				},
			}},
		}}},
	})
	if err := acc.apply(chunk); err == nil || !strings.Contains(err.Error(), "stream too large") {
		t.Fatalf("got %v", err)
	}
}
