package tools

import (
	"context"
	"encoding/json"

	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/upstage"
)

type documentParse struct {
	workdir string
	api     *upstage.Client
}

func (documentParse) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name: "document_parse",
		Description: "Parse a document with Upstage Document Parse into Markdown. " +
			"Use for PDF, HWP, HWPX, DOCX, PPTX, XLSX, TIFF, and images (JPEG, PNG, BMP, HEIC). " +
			"Preserves headings, tables, figures, and charts. Prefer this over bash/pdftotext.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"path":{"type":"string","description":"Document path, relative to workdir or absolute"},
				"mode":{"type":"string","enum":["standard","enhanced","auto"],"description":"auto for mixed docs, enhanced for complex tables/charts, standard for text-heavy. Default auto"},
				"ocr":{"type":"string","enum":["auto","force"],"description":"force for scans. Default auto"}
			},
			"required":["path"]
		}`),
	}
}

func (t documentParse) Run(ctx context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
		OCR  string `json:"ocr"`
	}](input)
	if err != nil {
		return "", err
	}
	path, err := resolve(t.workdir, args.Path)
	if err != nil {
		return "", err
	}
	return t.api.ParseDocument(ctx, path, args.Mode, args.OCR)
}

type documentOCR struct {
	workdir string
	api     *upstage.Client
}

func (documentOCR) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name: "document_ocr",
		Description: "Extract plain text with Upstage OCR. " +
			"Use when you only need raw text and coordinates are unnecessary. " +
			"For layout, tables, or Markdown, use document_parse instead.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"path":{"type":"string","description":"Document path, relative to workdir or absolute"}
			},
			"required":["path"]
		}`),
	}
}

func (t documentOCR) Run(ctx context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Path string `json:"path"`
	}](input)
	if err != nil {
		return "", err
	}
	path, err := resolve(t.workdir, args.Path)
	if err != nil {
		return "", err
	}
	return t.api.OCR(ctx, path)
}
