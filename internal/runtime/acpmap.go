package runtime

import "encoding/json"

// acpmap.go is the ONLY place ACP wire shapes are decoded (techspec §12.1
// isolation rule). Everything else in the package sees normalized Events. This
// keeps the blast radius of an ACP version bump — or a Codex backend — localized
// here. The shapes coded against are the §12.1 "assumed wire shapes"; the gated
// real-CLI test (1.6) verifies them and any drift is fixed in this file alone.

// acpSessionUpdate is the params of a `session/update` notification.
type acpSessionUpdate struct {
	SessionID string    `json:"sessionId"`
	Update    acpUpdate `json:"update"`
}

// acpUpdate is one streamed update, discriminated by SessionUpdate.
type acpUpdate struct {
	SessionUpdate string `json:"sessionUpdate"`

	// agent_message_chunk / agent_thought_chunk: a single content block.
	Content json.RawMessage `json:"content"`

	// tool_call / tool_call_update.
	ToolCallID string          `json:"toolCallId"`
	Title      string          `json:"title"`
	Kind       string          `json:"kind"`
	Status     string          `json:"status"`
	RawInput   json.RawMessage `json:"rawInput"`
}

// acpContentBlock is a single content item (text or diff). Tool-call content is
// an array of these; message-chunk content is one of these.
type acpContentBlock struct {
	Type string `json:"type"` // "text" | "diff" | ...
	Text string `json:"text,omitempty"`
	// diff fields (Type == "diff")
	Path    string `json:"path,omitempty"`
	OldText string `json:"oldText,omitempty"`
	NewText string `json:"newText,omitempty"`
}

// acpPromptResult is the result of our `session/prompt` request — it ends the turn.
type acpPromptResult struct {
	StopReason string    `json:"stopReason"`
	Usage      *acpUsage `json:"usage,omitempty"`
}

// acpUsage carries token usage when the adapter reports it (techspec §4.3).
type acpUsage struct {
	Used   int `json:"used"`
	Window int `json:"window"`
}

// mappedEvent pairs a normalized event type with its typed payload. The runtime
// stamps seq/agent_id/ts and marshals Data into the Event envelope.
type mappedEvent struct {
	Type string
	Data any
}

// mapSessionUpdate converts one `session/update` notification's params into zero
// or more normalized events (techspec §4.3). Unknown / dropped update kinds
// (agent_thought_chunk, plan) yield no events. A decode failure yields nil so the
// caller can log and continue (the transport already resyncs on bad frames).
func mapSessionUpdate(params json.RawMessage) []mappedEvent {
	var su acpSessionUpdate
	if err := json.Unmarshal(params, &su); err != nil {
		return nil
	}
	u := su.Update
	switch u.SessionUpdate {
	case "agent_message_chunk":
		var block acpContentBlock
		if err := json.Unmarshal(u.Content, &block); err != nil || block.Text == "" {
			return nil
		}
		return []mappedEvent{{Type: EvAssistantText, Data: AssistantTextData{Delta: block.Text}}}

	case "tool_call":
		name := toolName(u)
		return []mappedEvent{{Type: EvToolCall, Data: ToolCallData{
			ToolCallID: u.ToolCallID,
			Name:       name,
			Title:      firstNonEmpty(u.Title, name),
			Args:       nonNullRaw(u.RawInput),
			Status:     defaultStr(u.Status, "in_progress"),
		}}}

	case "tool_call_update":
		blocks := decodeContentArray(u.Content)
		evs := make([]mappedEvent, 0, 1+len(blocks))
		evs = append(evs, mappedEvent{Type: EvToolResult, Data: ToolResultData{
			ToolCallID: u.ToolCallID,
			Status:     defaultStr(u.Status, "completed"),
			Content:    nonNullRaw(u.Content),
		}})
		// A tool_call_update may carry one or more diff blocks (one per file).
		for _, b := range blocks {
			if b.Type == "diff" {
				evs = append(evs, mappedEvent{Type: EvDiff, Data: DiffData{
					ToolCallID: u.ToolCallID,
					Path:       b.Path,
					OldText:    b.OldText,
					NewText:    b.NewText,
				}})
			}
		}
		return evs

	default:
		// agent_thought_chunk, plan, and any unrecognized kind: dropped this phase.
		return nil
	}
}

// mapPromptResult converts the `session/prompt` result into a turn_end event and
// the turn's context_pct (0 when usage is absent — documented limitation §4.3).
func mapPromptResult(result json.RawMessage) (TurnEndData, bool) {
	var r acpPromptResult
	if err := json.Unmarshal(result, &r); err != nil {
		return TurnEndData{StopReason: "end_turn"}, false
	}
	td := TurnEndData{StopReason: defaultStr(r.StopReason, "end_turn")}
	hasPct := false
	if r.Usage != nil && r.Usage.Window > 0 {
		td.ContextPct = float64(r.Usage.Used) / float64(r.Usage.Window)
		hasPct = true
	}
	return td, hasPct
}

// toolName picks a normalized tool name: prefer the ACP kind, else the title.
// (The §4.3 mapping table does not pin which ACP field becomes Name; kind is the
// closest stable discriminator. See HANDOFF autonomous decisions.)
func toolName(u acpUpdate) string {
	return firstNonEmpty(u.Kind, u.Title, "tool")
}

func decodeContentArray(raw json.RawMessage) []acpContentBlock {
	if len(raw) == 0 {
		return nil
	}
	var blocks []acpContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}
	return blocks
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func nonNullRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}
