package tools

import (
	"encoding/json"
	"strings"
)

func Detail(name string, input json.RawMessage) string {
	var raw map[string]any
	if err := json.Unmarshal(input, &raw); err != nil {
		return strings.TrimSpace(string(input))
	}
	switch name {
	case "bash":
		if cmd, ok := raw["command"].(string); ok {
			return "$ " + cmd
		}
	case "read_file", "write_file", "edit_file", "document_parse", "document_ocr":
		if p, ok := raw["path"].(string); ok {
			return p
		}
	case "glob", "grep":
		if p, ok := raw["pattern"].(string); ok {
			return p
		}
	}
	b, _ := json.Marshal(raw)
	return string(b)
}
