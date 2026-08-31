package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDocumentParseRejectsEscape(t *testing.T) {
	d := documentParse{workdir: t.TempDir()}
	_, err := d.Run(context.Background(), json.RawMessage(`{"path":"../secret.pdf"}`))
	if err == nil {
		t.Fatal("expected workdir escape to fail before upload")
	}
}

func TestDocumentOCRRejectsEscape(t *testing.T) {
	d := documentOCR{workdir: t.TempDir()}
	_, err := d.Run(context.Background(), json.RawMessage(`{"path":"/etc/passwd"}`))
	if err == nil {
		t.Fatal("expected absolute escape to fail before upload")
	}
}
