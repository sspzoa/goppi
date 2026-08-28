package complete

import (
	"fmt"
	"strings"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/session"
	"github.com/sspzoa/goppi/internal/upstage"
)

type Item struct {
	Name    string
	Summary string
	Arg     bool
	Alias   bool
}

func SlashCmds() []Item {
	return []Item{
		{Name: "/help", Summary: "도움말"},
		{Name: "/model", Summary: "Solar 모델", Arg: true},
		{Name: "/effort", Summary: "reasoning 강도", Arg: true},
		{Name: "/new", Summary: "세션 초기화"},
		{Name: "/tools", Summary: "등록된 툴"},
		{Name: "/sessions", Summary: "최근 세션"},
		{Name: "/status", Summary: "현재 설정"},
		{Name: "/quit", Summary: "종료"},
		{Name: "/clear", Summary: "세션 초기화", Alias: true},
		{Name: "/exit", Summary: "종료", Alias: true},
		{Name: "/q", Summary: "종료", Alias: true},
		{Name: "/?", Summary: "도움말", Alias: true},
	}
}

func CLICommands() []Item {
	return []Item{
		{Name: "login", Summary: "Save an Upstage API key"},
		{Name: "logout", Summary: "Remove the saved key"},
		{Name: "models", Summary: "List Solar chat models"},
		{Name: "doctor", Summary: "Check local setup"},
		{Name: "init", Summary: "Write GOPPI.md"},
		{Name: "inspect", Summary: "Show resolved config"},
		{Name: "sessions", Summary: "List and manage sessions"},
		{Name: "export", Summary: "Export a session as Markdown"},
		{Name: "completions", Summary: "Shell completion scripts"},
		{Name: "version", Summary: "Print version"},
		{Name: "help", Summary: "Show help"},
	}
}

func Models() []Item {
	out := make([]Item, 0, len(upstage.ChatModels))
	for _, m := range upstage.ChatModels {
		out = append(out, Item{Name: m.ID, Summary: m.Summary})
	}
	return out
}

func Efforts() []Item {
	out := make([]Item, 0, len(config.Efforts))
	for _, e := range config.Efforts {
		out = append(out, Item{Name: e})
	}
	return out
}

func Formats() []Item {
	return []Item{{Name: "plain"}, {Name: "json"}}
}

func Shells() []Item {
	return []Item{{Name: "zsh"}, {Name: "bash"}, {Name: "fish"}}
}

func Flags() []Item {
	return []Item{
		{Name: "-p", Summary: "Headless prompt"},
		{Name: "--prompt", Summary: "Headless prompt"},
		{Name: "-m", Summary: "Model"},
		{Name: "--model", Summary: "Model"},
		{Name: "--effort", Summary: "Reasoning effort"},
		{Name: "-C", Summary: "Working directory"},
		{Name: "--cwd", Summary: "Working directory"},
		{Name: "-c", Summary: "Resume last session"},
		{Name: "--continue", Summary: "Resume last session"},
		{Name: "-r", Summary: "Resume session"},
		{Name: "--resume", Summary: "Resume session"},
		{Name: "--max-turns", Summary: "Max agent turns"},
		{Name: "--output-format", Summary: "plain | json"},
		{Name: "--always-approve", Summary: "Skip permission prompts"},
		{Name: "--yolo", Summary: "Skip permission prompts"},
		{Name: "-h", Summary: "Help"},
		{Name: "--help", Summary: "Help"},
		{Name: "-v", Summary: "Version"},
		{Name: "--version", Summary: "Version"},
	}
}

func SessionIDs() []string {
	items, err := session.List()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, f := range items {
		out = append(out, f.ID)
	}
	return out
}

func Query(line string) []Item {
	if !strings.HasPrefix(strings.TrimSpace(line), "/") {
		return nil
	}
	fields := strings.Fields(line)
	trailing := strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t")
	if len(fields) == 0 {
		return visible(SlashCmds(), "/", false)
	}
	if len(fields) == 1 && !trailing {
		return visible(SlashCmds(), fields[0], fields[0] != "/")
	}
	cmd := lookup(fields[0])
	if cmd == nil || !cmd.Arg {
		return nil
	}
	arg := ""
	if len(fields) > 1 {
		arg = fields[1]
	}
	switch cmd.Name {
	case "/model":
		return visible(Models(), arg, true)
	case "/effort":
		return visible(Efforts(), arg, true)
	default:
		return nil
	}
}

func Apply(line string, item Item) string {
	fields := strings.Fields(line)
	trailing := strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t")
	if len(fields) == 0 || (len(fields) == 1 && !trailing) {
		if item.Arg {
			return item.Name + " "
		}
		return item.Name
	}
	return fields[0] + " " + item.Name
}

func Ready(line string) bool {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "/") {
		return true
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false
	}
	cmd := lookup(fields[0])
	if cmd == nil {
		return len(Query(line)) == 0
	}
	if cmd.Arg && len(fields) >= 2 {
		arg := fields[1]
		for _, it := range Query(line) {
			if it.Name == arg {
				return true
			}
		}
		return false
	}
	return true
}

func Names(kind, prefix string) ([]string, error) {
	var items []Item
	switch kind {
	case "commands", "command":
		items = CLICommands()
	case "models", "model":
		items = Models()
	case "efforts", "effort":
		items = Efforts()
	case "formats", "format":
		items = Formats()
	case "shells", "shell":
		items = Shells()
	case "flags", "flag":
		items = Flags()
	case "slash":
		items = SlashCmds()
	case "sessions", "session":
		return filterNames(SessionIDs(), prefix), nil
	default:
		return nil, fmt.Errorf("usage: goppi complete commands|models|efforts|sessions|formats|shells|flags|slash")
	}
	var out []string
	for _, it := range items {
		if prefix == "" || hasPrefix(it.Name, prefix) {
			out = append(out, it.Name)
		}
	}
	return out, nil
}

func visible(items []Item, prefix string, aliases bool) []Item {
	var out []Item
	for _, it := range items {
		if it.Alias && !aliases {
			continue
		}
		if prefix == "" || prefix == "/" || hasPrefix(it.Name, prefix) {
			out = append(out, it)
		}
	}
	return out
}

func lookup(name string) *Item {
	for _, it := range SlashCmds() {
		if it.Name == name {
			it := it
			return &it
		}
	}
	return nil
}

func hasPrefix(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
}

func filterNames(ids []string, prefix string) []string {
	if prefix == "" {
		return ids
	}
	var out []string
	for _, id := range ids {
		if hasPrefix(id, prefix) {
			out = append(out, id)
		}
	}
	return out
}
