package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

const maxClipboardBytes = 256 << 10

func CopyClipboard(text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("복사할 답이 없습니다")
	}
	if len(text) > maxClipboardBytes {
		text = text[:maxClipboardBytes]
	}
	if !isCharDevice(os.Stderr) {
		return nil
	}
	_, err := fmt.Fprint(os.Stderr, OSC52(text))
	return err
}

func OSC52(text string) string {
	return "\033]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\007"
}
