package agent

import "github.com/sspzoa/goppi/internal/provider"

// CloseOpenToolCalls appends error tool results for any assistant
// tool_calls that never got a matching role=tool message. Resume and
// cancel must leave a valid transcript for the next Chat.
func CloseOpenToolCalls(msgs []provider.Message, reason string) []provider.Message {
	if reason == "" {
		reason = "interrupted"
	}
	var last int
	found := false
	for i, m := range msgs {
		if m.Role == provider.RoleAssistant && len(m.ToolCalls) > 0 {
			last = i
			found = true
		}
	}
	if !found {
		return msgs
	}
	have := map[string]bool{}
	for _, m := range msgs[last+1:] {
		if m.Role == provider.RoleTool && m.ToolCallID != "" {
			have[m.ToolCallID] = true
		}
	}
	for _, call := range msgs[last].ToolCalls {
		if have[call.ID] {
			continue
		}
		msgs = append(msgs, provider.Message{
			Role:       provider.RoleTool,
			ToolCallID: call.ID,
			ToolName:   call.Name,
			Content:    "error: " + reason,
		})
	}
	return msgs
}
