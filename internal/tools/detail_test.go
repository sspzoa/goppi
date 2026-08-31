package tools

import "testing"

func TestDetailBash(t *testing.T) {
	got := Detail("bash", []byte(`{"command":"go test ./..."}`))
	if got != "$ go test ./..." {
		t.Fatalf("got %q", got)
	}
}

func TestDetailDelegate(t *testing.T) {
	got := Detail("delegate", []byte(`{"prompt":"find the main package"}`))
	if got != "find the main package" {
		t.Fatalf("got %q", got)
	}
}

func TestDetailPath(t *testing.T) {
	got := Detail("read_file", []byte(`{"path":"README.md"}`))
	if got != "README.md" {
		t.Fatalf("got %q", got)
	}
}
