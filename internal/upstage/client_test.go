package upstage

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSupportsReasoning(t *testing.T) {
	if !SupportsReasoning("solar-pro4") {
		t.Fatal("solar-pro4 should support reasoning")
	}
	if SupportsReasoning("solar-mini") {
		t.Fatal("solar-mini must not send reasoning_effort")
	}
	if SupportsReasoning("gpt-4.1") {
		t.Fatal("openai models must not get solar reasoning_effort")
	}
}

func TestKnownModel(t *testing.T) {
	if !KnownModel("solar-pro4") {
		t.Fatal("expected solar-pro4")
	}
	if KnownModel("gpt-4.1") {
		t.Fatal("openai models are not in catalog")
	}
}

func TestPostJSONSetsUserAgent(t *testing.T) {
	var ua, auth string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("user-agent")
		auth = r.Header.Get("authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer s.Close()
	c := New("up_test", s.URL)
	c.UserAgent = "goppi/0.6.3"
	if _, err := c.PostJSON(context.Background(), "/x", map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
	if ua != "goppi/0.6.3" {
		t.Fatalf("user-agent %q", ua)
	}
	if auth != "Bearer up_test" {
		t.Fatalf("authorization %q", auth)
	}
}

func TestParseRetryAfter(t *testing.T) {
	if got := parseRetryAfter("2", 0); got != 2*time.Second {
		t.Fatalf("seconds %v", got)
	}
	if got := parseRetryAfter("99", 0); got != 10*time.Second {
		t.Fatalf("cap %v", got)
	}
	past := time.Now().UTC().Add(-time.Second).Format(http.TimeFormat)
	if got := parseRetryAfter(past, 0); got != 200*time.Millisecond {
		t.Fatalf("past http-date %v", got)
	}
	if got := parseRetryAfter("", 2); got != 600*time.Millisecond {
		t.Fatalf("empty %v", got)
	}
}

func TestHTTPClientMinTLS(t *testing.T) {
	c := NewHTTPClient(time.Second)
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr.TLSClientConfig == nil {
		t.Fatal("expected tls config")
	}
	if tr.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatalf("min TLS %d", tr.TLSClientConfig.MinVersion)
	}
}

func TestPostJSONRejectsHugeBody(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, maxResponseBytes+1))
	}))
	defer s.Close()
	c := New("k", s.URL)
	if _, err := c.PostJSON(context.Background(), "/x", map[string]string{"a": "b"}); err == nil {
		t.Fatal("expected too large")
	}
}

func TestPostJSON401Hint(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer s.Close()
	c := New("bad", s.URL)
	_, err := c.PostJSON(context.Background(), "/x", map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("expected 401")
	}
	got := err.Error()
	if !strings.Contains(got, "API 401") || !strings.Contains(got, "goppi login") || !strings.Contains(got, "invalid api key") {
		t.Fatalf("%q", got)
	}
	var se StatusError
	if !errors.As(err, &se) || se.Status != 401 {
		t.Fatalf("%T %v", err, err)
	}
}

func TestProbeKeyModelsOK(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer s.Close()
	if err := New("k", s.URL).ProbeKey(context.Background(), "solar-mini"); err != nil {
		t.Fatal(err)
	}
}

func TestProbeKeyRejects401(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	defer s.Close()
	err := New("bad", s.URL).ProbeKey(context.Background(), "solar-mini")
	if err == nil {
		t.Fatal("expected 401")
	}
	var se StatusError
	if !errors.As(err, &se) || se.Status != 401 {
		t.Fatalf("%v", err)
	}
}

func TestProbeKeyFallsBackToChat(t *testing.T) {
	var chat bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{}`)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		chat = true
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"."}}]}`)
	}))
	defer s.Close()
	if err := New("k", s.URL).ProbeKey(context.Background(), "solar-mini"); err != nil {
		t.Fatal(err)
	}
	if !chat {
		t.Fatal("expected chat fallback")
	}
}

func TestStatusErrorHints(t *testing.T) {
	if !strings.Contains(StatusError{Status: 429, Body: "slow"}.Error(), "요청이 많습니다") {
		t.Fatal("429")
	}
	if !strings.Contains(StatusError{Status: 503, Body: "no"}.Error(), "서버가 바쁩니다") {
		t.Fatal("503")
	}
	if strings.Contains(StatusError{Status: 400, Body: "bad"}.Error(), "\n  ") {
		t.Fatal("400 should not add a hint")
	}
}

func TestPostJSONRetries429(t *testing.T) {
	var n int
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"message":"slow down"}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer s.Close()
	c := New("k", s.URL)
	data, err := c.PostJSON(context.Background(), "/x", map[string]int{"n": 1})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("attempts %d", n)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("body %s", data)
	}
}
