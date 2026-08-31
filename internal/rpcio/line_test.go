package rpcio

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

func TestReadLineCaps(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(strings.Repeat("a", 80) + "\n"))
	if _, err := ReadLine(r, 40); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("got %v", err)
	}
}

func TestReadLineKeepsNewlineJSON(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("{\"ok\":true}\n"))
	got, err := ReadLine(r, MaxFrame)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\"ok\":true}\n" {
		t.Fatalf("%q", got)
	}
}

func TestReadLineEOF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(""))
	if _, err := ReadLine(r, 16); err != io.EOF {
		t.Fatalf("got %v", err)
	}
}
