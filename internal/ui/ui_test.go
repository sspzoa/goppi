package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestBannerHasUpstageBrand(t *testing.T) {
	out := capture(t, func() {
		Banner("0.3.0", "solar-pro4", "", "/Users/sspzoa/goppi")
	})
	for _, want := range []string{"goppi", "UPSTAGE SOLAR", "solar-pro4", "model", "effort", "workdir"} {
		if !strings.Contains(out, want) {
			t.Fatalf("banner missing %q\n%s", want, out)
		}
	}
}

func capture(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
