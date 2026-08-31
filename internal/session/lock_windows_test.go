//go:build windows

package session

import (
	"strings"
	"testing"
)

func TestHoldExclusiveWindows(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	id := "aaaaaaaaaaaaaaaa"
	lk, err := Hold(id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lk.Release)
	if _, err := Hold(id); err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("second hold: %v", err)
	}
}
