package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sspzoa/goppi/internal/upstage"
)

func TestSolarChatStreams(t *testing.T) {
	var gotStream bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("body: %v", err)
		}
		gotStream, _ = req["stream"].(bool)
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"reasoning":"생각 "}}]}`,
			``,
			`data: {"choices":[{"delta":{"content":"안녕"},"finish_reason":"stop"}]}`,
			``,
			`data: {"usage":{"prompt_tokens":2,"completion_tokens":1,"completion_tokens_details":{"reasoning_tokens":1}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	defer s.Close()

	var reasons, contents []string
	client := NewSolar(upstage.New("k", s.URL))
	resp, err := client.Chat(context.Background(), ChatRequest{
		Model:  "solar-pro4",
		System: "sys",
		Messages: []Message{
			{Role: RoleUser, Content: "hi"},
		},
		OnDelta: func(d Delta) {
			if d.Reasoning != "" {
				reasons = append(reasons, d.Reasoning)
			}
			if d.Content != "" {
				contents = append(contents, d.Content)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !gotStream {
		t.Fatal("expected stream=true")
	}
	if resp.Message.Content != "안녕" || resp.Message.Reasoning != "생각" {
		t.Fatalf("msg %+v", resp.Message)
	}
	if resp.Usage.InputTokens != 2 || resp.Usage.ReasoningTokens != 1 {
		t.Fatalf("usage %+v", resp.Usage)
	}
	if strings.Join(reasons, "") != "생각 " || strings.Join(contents, "") != "안녕" {
		t.Fatalf("deltas %v %v", reasons, contents)
	}
}

func TestSolarChatSendsImageParts(t *testing.T) {
	var raw []byte
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n data: [DONE]\n\n")
	}))
	defer s.Close()
	_, err := NewSolar(upstage.New("k", s.URL)).Chat(context.Background(), ChatRequest{
		Model: "solar-pro4",
		Messages: []Message{{
			Role:    RoleUser,
			Content: "what is this",
			Images:  []Image{{URL: "data:image/png;base64,aaa", Path: "a.png", MIME: "image/png"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"type":"image_url"`) || !strings.Contains(string(raw), "data:image/png;base64,aaa") {
		t.Fatalf("missing image parts: %s", raw)
	}
	if !strings.Contains(string(raw), "what is this") {
		t.Fatalf("missing text: %s", raw)
	}
}

func TestSolarChatCancel(t *testing.T) {
	started := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flush", 500)
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"content":"a"}}]}`+"\n\n")
		fl.Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		<-started
		cancel()
	}()
	_, err := NewSolar(upstage.New("k", s.URL)).Chat(ctx, ChatRequest{Model: "solar-pro4"})
	if err == nil {
		t.Fatal("expected cancel error")
	}
}

func TestSolarChatHTTPError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"bad model"}}`)
	}))
	defer s.Close()
	_, err := NewSolar(upstage.New("k", s.URL)).Chat(context.Background(), ChatRequest{Model: "solar-pro4"})
	if err == nil || !strings.Contains(err.Error(), "bad model") {
		t.Fatalf("got %v", err)
	}
}
