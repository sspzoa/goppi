package upstage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const maxDocChars = 200_000

type parseResp struct {
	Content struct {
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
		HTML     string `json:"html"`
	} `json:"content"`
	Usage struct {
		Pages int `json:"pages"`
	} `json:"usage"`
	Model string `json:"model"`
}

type ocrResp struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Pages      []struct {
		ID   int    `json:"id"`
		Text string `json:"text"`
	} `json:"pages"`
}

func (c *Client) ParseDocument(ctx context.Context, filePath, mode, ocr string) (string, error) {
	if mode == "" {
		mode = "auto"
	}
	if ocr == "" {
		ocr = "auto"
	}
	data, err := c.PostMultipart(ctx, "/document-digitization", map[string]string{
		"model":          "document-parse",
		"mode":           mode,
		"ocr":            ocr,
		"output_formats": `["markdown"]`,
		"coordinates":    "false",
	}, filePath)
	if err != nil {
		return "", err
	}
	var parsed parseResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("document-parse decode: %w", err)
	}
	text := parsed.Content.Markdown
	if text == "" {
		text = parsed.Content.Text
	}
	if text == "" {
		return "", fmt.Errorf("document-parse: empty content")
	}
	var b strings.Builder
	if parsed.Usage.Pages > 0 {
		fmt.Fprintf(&b, "pages=%d model=%s\n\n", parsed.Usage.Pages, parsed.Model)
	}
	b.WriteString(clip(text, maxDocChars))
	return b.String(), nil
}

func (c *Client) OCR(ctx context.Context, filePath string) (string, error) {
	data, err := c.PostMultipart(ctx, "/document-digitization", map[string]string{
		"model": "ocr",
	}, filePath)
	if err != nil {
		return "", err
	}
	var parsed ocrResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("ocr decode: %w", err)
	}
	text := strings.TrimSpace(parsed.Text)
	if text == "" {
		var pages []string
		for _, p := range parsed.Pages {
			if strings.TrimSpace(p.Text) != "" {
				pages = append(pages, fmt.Sprintf("--- page %d ---\n%s", p.ID+1, p.Text))
			}
		}
		text = strings.Join(pages, "\n\n")
	}
	if text == "" {
		return "", fmt.Errorf("ocr: empty text")
	}
	var b strings.Builder
	if parsed.Confidence > 0 {
		fmt.Fprintf(&b, "confidence=%.2f\n\n", parsed.Confidence)
	}
	b.WriteString(clip(text, maxDocChars))
	return b.String(), nil
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n… truncated"
}
