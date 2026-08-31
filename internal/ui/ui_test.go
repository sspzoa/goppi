package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestOSC52EncodesPayload(t *testing.T) {
	seq := OSC52("hello")
	if !strings.HasPrefix(seq, "\033]52;c;") || !strings.HasSuffix(seq, "\007") {
		t.Fatalf("%q", seq)
	}
	if !strings.Contains(seq, "aGVsbG8=") {
		t.Fatalf("missing base64: %q", seq)
	}
}

func TestCopyClipboardEmpty(t *testing.T) {
	if err := CopyClipboard(""); err == nil {
		t.Fatal("empty should fail")
	}
	if err := CopyClipboard("   "); err == nil {
		t.Fatal("blank should fail")
	}
}

func TestNotifyDisabled(t *testing.T) {
	t.Setenv("GOPPI_NOTIFY", "off")
	if NotifyEnabled() {
		t.Fatal("off")
	}
	t.Setenv("GOPPI_NOTIFY", "on")
	if !NotifyEnabled() {
		t.Fatal("on")
	}
}

func TestBannerHasChrome(t *testing.T) {
	out := capture(t, func() {
		Banner("0.3.0", "solar-pro4", "", "/Users/sspzoa/goppi")
	})
	for _, want := range []string{"고삐", "solar-pro4", "model", "effort", "workdir"} {
		if !strings.Contains(out, want) {
			t.Fatalf("banner missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "UPSTAGE SOLAR") {
		t.Fatalf("banner still has exclusive brand\n%s", out)
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
