package upstage

import (
	"bytes"
	"context"
	"encoding/json"
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
	DefaultBaseURL = "https://api.upstage.ai/v1"
	DefaultModel   = "solar-pro4"
	ConsoleURL     = "https://console.upstage.ai"
	MaxUploadBytes = 50 << 20
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
	APIKey  string
	BaseURL string
	HTTP    *http.Client
}

func New(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 180 * time.Second},
	}
}

func ResolveAPIKey(explicit string) string {
	if explicit != "" {
		return explicit
	}
	for _, key := range []string{"UPSTAGE_API_KEY", "GOPPI_API_KEY"} {
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
	return model != "solar-mini"
}

func (c *Client) PostJSON(ctx context.Context, path string, body any) ([]byte, error) {
	resp, err := c.postJSON(ctx, path, body, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstage %s: %s", resp.Status, decodeError(data))
	}
	return data, nil
}

func (c *Client) PostJSONStream(ctx context.Context, path string, body any) (io.ReadCloser, error) {
	resp, err := c.postJSON(ctx, path, body, true)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("upstage %s: %s", resp.Status, decodeError(data))
	}
	return resp.Body, nil
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
	req.Header.Set("authorization", "Bearer "+c.APIKey)
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
	req.Header.Set("authorization", "Bearer "+c.APIKey)
	return c.do(req)
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstage %s: %s", resp.Status, decodeError(data))
	}
	return data, nil
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
