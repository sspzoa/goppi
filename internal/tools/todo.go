package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/sspzoa/goppi/internal/provider"
)

type Todo struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Status  string `json:"status"`
}

type TodoList struct {
	mu    sync.Mutex
	items []Todo
}

func NewTodoList() *TodoList { return &TodoList{} }

func (l *TodoList) Items() []Todo {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Todo, len(l.items))
	copy(out, l.items)
	return out
}

func (l *TodoList) Replace(items []Todo) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = append([]Todo(nil), items...)
}

func (l *TodoList) Render() string {
	items := l.Items()
	if len(items) == 0 {
		return "(no todos)"
	}
	var b strings.Builder
	for _, it := range items {
		mark := "- [ ]"
		switch it.Status {
		case "in_progress":
			mark = "- [~]"
		case "done", "completed":
			mark = "- [x]"
		}
		fmt.Fprintf(&b, "%s %s %s\n", mark, it.ID, it.Content)
	}
	return strings.TrimRight(b.String(), "\n")
}

type todoWrite struct{ list *TodoList }

func (todoWrite) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name: "todo_write",
		Description: "Replace the session task list. Use for multi-step work: track pending, in_progress, and done items. " +
			"Keep at most one in_progress. Call again whenever the plan changes.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"todos":{
					"type":"array",
					"items":{
						"type":"object",
						"properties":{
							"id":{"type":"string"},
							"content":{"type":"string"},
							"status":{"type":"string","enum":["pending","in_progress","done"]}
						},
						"required":["id","content","status"]
					}
				}
			},
			"required":["todos"]
		}`),
	}
}

func (t todoWrite) Run(_ context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		Todos []Todo `json:"todos"`
	}](input)
	if err != nil {
		return "", err
	}
	if t.list == nil {
		return "", fmt.Errorf("todo list unavailable")
	}
	for i, it := range args.Todos {
		if strings.TrimSpace(it.ID) == "" || strings.TrimSpace(it.Content) == "" {
			return "", fmt.Errorf("todo %d missing id or content", i)
		}
		switch it.Status {
		case "pending", "in_progress", "done":
		default:
			return "", fmt.Errorf("todo %s: bad status %q", it.ID, it.Status)
		}
	}
	t.list.Replace(args.Todos)
	return t.list.Render(), nil
}
