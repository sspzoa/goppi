package upstage

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultBaseURL   = "https://api.upstage.ai/v1"
	DefaultModel     = "solar-pro4"
	ConsoleURL       = "https://console.upstage.ai"
	MaxUploadBytes   = 50 << 20
	maxResponseBytes = 8 << 20
)

var ChatModels = []Model{
	{ID: "solar-pro4", Summary: "플래그십. 512K 컨텍스트, 기본 reasoning on"},
	{ID: "solar-pro3", Summary: "이전 플래그십. reasoning은 medium/high"},
	{ID: "solar-pro2", Summary: "이전 세대"},
	{ID: "solar-mini", Summary: "가볍고 빠름. reasoning_effort 불가"},
}

type Model struct {
	ID      string
	Summary string
}

type Client struct {
	APIKey    string
	BaseURL   string
	UserAgent string
	HTTP      *http.Client
}

func New(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		APIKey:    apiKey,
		BaseURL:   strings.TrimRight(baseURL, "/"),
		UserAgent: "goppi",
		HTTP:      NewHTTPClient(180 * time.Second),
	}
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: secureTransport()}
}

func secureTransport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	}
	tr := base.Clone()
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		cfg := tr.TLSClientConfig.Clone()
		cfg.MinVersion = tls.VersionTLS12
		tr.TLSClientConfig = cfg
	}
	return tr
}

func ResolveAPIKey(explicit string) string {
	if explicit != "" {
		return explicit
	}
	for _, key := range []string{"UPSTAGE_API_KEY", "OPENAI_API_KEY", "GOPPI_API_KEY"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func MissingKeyError() error {
	return fmt.Errorf("API 키가 없습니다. goppi login 을 실행하거나 UPSTAGE_API_KEY 를 설정하세요\n  %s", ConsoleURL)
}

func KnownModel(id string) bool {
	for _, m := range ChatModels {
		if m.ID == id {
			return true
		}
	}
	return false
}

func SupportsReasoning(model string) bool {
	if model == "solar-mini" || model == "" {
		return false
	}
	return strings.HasPrefix(model, "solar-")
}

func (c *Client) GetJSON(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	return c.do(req)
}

// ProbeKey checks that the key is accepted. GET /models is enough
// for OpenAI-compatible hosts. Upstage has no /models, so a 404
// falls through to a 1-token chat. 401/403 fail immediately.
func (c *Client) ProbeKey(ctx context.Context, model string) error {
	_, err := c.GetJSON(ctx, "/models")
	if err == nil {
		return nil
	}
	var se StatusError
	if !errors.As(err, &se) {
		return err
	}
	if se.Status == http.StatusUnauthorized || se.Status == http.StatusForbidden {
		return err
	}
	if model == "" {
		model = DefaultModel
	}
	_, err = c.PostJSON(ctx, "/chat/completions", map[string]any{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "."}},
	})
	return err
}

func (c *Client) PostJSON(ctx context.Context, path string, body any) ([]byte, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if err := waitRetry(ctx, attempt); err != nil {
			return nil, err
		}
		resp, err := c.postJSON(ctx, path, body, false)
		if err != nil {
			last = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		data, err := readCapped(resp.Body, maxResponseBytes)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 300 {
			return data, nil
		}
		last = statusError(resp.StatusCode, data)
		if !retryableStatus(resp.StatusCode) {
			return nil, last
		}
		if err := sleepRetryAfter(ctx, resp, attempt); err != nil {
			return nil, err
		}
	}
	return nil, last
}

func (c *Client) PostJSONStream(ctx context.Context, path string, body any) (io.ReadCloser, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if err := waitRetry(ctx, attempt); err != nil {
			return nil, err
		}
		resp, err := c.postJSON(ctx, path, body, true)
		if err != nil {
			last = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		if resp.StatusCode < 300 {
			return resp.Body, nil
		}
		data, _ := readCapped(resp.Body, maxResponseBytes)
		resp.Body.Close()
		last = statusError(resp.StatusCode, data)
		if !retryableStatus(resp.StatusCode) {
			return nil, last
		}
		if err := sleepRetryAfter(ctx, resp, attempt); err != nil {
			return nil, err
		}
	}
	return nil, last
}

func (c *Client) postJSON(ctx context.Context, path string, body any, stream bool) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	c.setAuth(req)
	if stream {
		req.Header.Set("accept", "text/event-stream")
	}
	client := c.HTTP
	if stream {
		client = &http.Client{Timeout: 0, Transport: c.HTTP.Transport}
	}
	return client.Do(req)
}

func (c *Client) PostMultipart(ctx context.Context, path string, fields map[string]string, filePath string) ([]byte, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxUploadBytes {
		return nil, fmt.Errorf("파일이 50MB를 넘습니다 (%d bytes)", info.Size())
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("document", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, err
	}
	for k, v := range fields {
		if v == "" {
			continue
		}
		if err := w.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", w.FormDataContentType())
	c.setAuth(req)
	return c.do(req)
}

func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("authorization", "Bearer "+c.APIKey)
	ua := c.UserAgent
	if ua == "" {
		ua = "goppi"
	}
	req.Header.Set("user-agent", ua)
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := readCapped(resp.Body, maxResponseBytes)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, statusError(resp.StatusCode, data)
	}
	return data, nil
}

func retryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func waitRetry(ctx context.Context, attempt int) error {
	if attempt == 0 {
		return ctx.Err()
	}
	return sleepCtx(ctx, time.Duration(attempt)*200*time.Millisecond)
}

func sleepRetryAfter(ctx context.Context, resp *http.Response, attempt int) error {
	v := ""
	if resp != nil {
		v = resp.Header.Get("Retry-After")
	}
	return sleepCtx(ctx, parseRetryAfter(v, attempt))
}

func parseRetryAfter(v string, attempt int) time.Duration {
	fallback := time.Duration(attempt+1) * 200 * time.Millisecond
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	if sec, err := time.ParseDuration(v + "s"); err == nil && sec > 0 {
		return capRetry(sec)
	}
	if t, err := http.ParseTime(v); err == nil {
		if until := time.Until(t); until > 0 {
			return capRetry(until)
		}
	}
	return fallback
}

func capRetry(d time.Duration) time.Duration {
	if d > 10*time.Second {
		return 10 * time.Second
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type StatusError struct {
	Status int
	Body   string
}

func (e StatusError) Error() string {
	msg := fmt.Sprintf("API %d: %s", e.Status, e.Body)
	if hint := hintForStatus(e.Status); hint != "" {
		return msg + "\n  " + hint
	}
	return msg
}

func hintForStatus(code int) string {
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "키가 거부되었습니다. goppi login 또는 API key를 확인하세요. " + ConsoleURL
	case http.StatusNotFound:
		return "모델 또는 경로를 확인하세요. goppi models"
	case http.StatusTooManyRequests:
		return "요청이 많습니다. 잠시 후 다시 시도하세요."
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "서버가 바쁩니다. 재시도해도 같으면 나중에."
	default:
		return ""
	}
}

func statusError(code int, data []byte) error {
	return StatusError{Status: code, Body: decodeError(data)}
}

func decodeError(data []byte) string {
	var wrap struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &wrap); err == nil {
		if wrap.Error.Message != "" {
			return wrap.Error.Message
		}
		if wrap.Message != "" {
			return wrap.Message
		}
	}
	s := strings.TrimSpace(string(data))
	if len(s) > 400 {
		return s[:400] + "…"
	}
	if s == "" {
		return "empty error body"
	}
	return s
}

func readCapped(r io.Reader, n int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(n)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > n {
		return nil, fmt.Errorf("response too large (%d bytes)", n)
	}
	return data, nil
}
