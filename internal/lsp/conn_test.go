package lsp

import (
	"bufio"
	"strings"
	"testing"

	"github.com/sspzoa/goppi/internal/rpcio"
)

func TestReadFrameJSONLine(t *testing.T) {
	got, err := readFrame(bufio.NewReader(strings.NewReader("{\"a\":1}\n")))
	if err != nil || strings.TrimSpace(string(got)) != `{"a":1}` {
		t.Fatalf("%q %v", got, err)
	}
}

func TestReadFrameRejectsHugeHeader(t *testing.T) {
	raw := "Content-Length: " + strings.Repeat("1", rpcio.MaxHeader) + "\n\n"
	if _, err := readFrame(bufio.NewReader(strings.NewReader(raw))); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("got %v", err)
	}
}
