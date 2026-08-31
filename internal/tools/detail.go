package tools

import (
	"encoding/json"
	"fmt"
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
			if bg, _ := raw["background"].(bool); bg {
				return "$ " + cmd + " &"
			}
			return "$ " + cmd
		}
	case "ask_user":
		if q, ok := raw["question"].(string); ok {
			q = strings.TrimSpace(q)
			if len(q) > 80 {
				return q[:80] + "…"
			}
			return q
		}
	case "bash_poll", "bash_kill":
		if id, ok := raw["id"].(float64); ok && id > 0 {
			return fmt.Sprintf("job %d", int(id))
		}
		return "jobs"
	case "apply_patch":
		if p, ok := raw["patch"].(string); ok {
			return firstPatchPath(p)
		}
	case "read_file", "write_file", "edit_file", "document_parse", "document_ocr", "read_image", "diagnostics":
		if p, ok := raw["path"].(string); ok {
			return p
		}
	case "web_fetch":
		if p, ok := raw["url"].(string); ok {
			return p
		}
	case "read_skill":
		if p, ok := raw["name"].(string); ok {
			return p
		}
	case "todo_write":
		return "todos"
	case "delegate":
		if p, ok := raw["prompt"].(string); ok {
			p = strings.TrimSpace(p)
			if len(p) > 80 {
				return p[:80] + "…"
			}
			return p
		}
	case "glob", "grep":
		if p, ok := raw["pattern"].(string); ok {
			return p
		}
	}
	if strings.HasPrefix(name, "mcp_") {
		b, _ := json.Marshal(raw)
		s := string(b)
		if len(s) > 80 {
			return s[:80] + "…"
		}
		return s
	}
	b, _ := json.Marshal(raw)
	return string(b)
}
