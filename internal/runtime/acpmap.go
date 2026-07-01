package runtime

import (
	"encoding/json"

	"github.com/agentdeck/agentdeck/internal/strutil"
)

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

// acpPermissionRequest is the params of a server→client `session/request_permission`
// request (techspec §12.1). The runtime withholds its response to pause the agent.
type acpPermissionRequest struct {
	SessionID string             `json:"sessionId"`
	Reason    string             `json:"reason"`
	ToolCall  acpPermToolCall    `json:"toolCall"`
	Options   []acpPermissionOpt `json:"options"`
}

type acpPermToolCall struct {
	ToolCallID string          `json:"toolCallId"`
	Title      string          `json:"title"`
	Kind       string          `json:"kind"`
	RawInput   json.RawMessage `json:"rawInput"`
}

type acpPermissionOpt struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // allow_once | allow_always | reject_once | reject_always
}

// mapPermissionRequest converts the ACP permission request into normalized
// PermissionRequestData plus a kind→optionId table for choosing the reply option
// (techspec §5.3). expiresAt is RFC3339; autoApproved reflects skip_permissions.
func mapPermissionRequest(params json.RawMessage, expiresAt string, autoApproved bool) (PermissionRequestData, map[string]string) {
	var pr acpPermissionRequest
	_ = json.Unmarshal(params, &pr)

	name := strutil.FirstNonEmpty(pr.ToolCall.Kind, pr.ToolCall.Title, "tool")
	opts := make([]PermOption, 0, len(pr.Options))
	byKind := make(map[string]string, len(pr.Options))
	for _, o := range pr.Options {
		opts = append(opts, PermOption{OptionID: o.OptionID, Label: o.Name, Kind: o.Kind})
		if _, dup := byKind[o.Kind]; !dup {
			byKind[o.Kind] = o.OptionID
		}
	}
	data := PermissionRequestData{
		ToolCallID:   pr.ToolCall.ToolCallID,
		Name:         name,
		Reason:       pr.Reason,
		Args:         nonNullRaw(pr.ToolCall.RawInput),
		Options:      opts,
		AutoApproved: autoApproved,
		ExpiresAt:    expiresAt,
	}
	return data, byKind
}

// selectOption picks the ACP optionId for an approve/deny decision (techspec
// §5.3): approve prefers allow_once then allow_always; deny prefers reject_once
// then reject_always. ok is false when no usable option exists.
func selectOption(byKind map[string]string, decision string) (string, bool) {
	var order []string
	switch decision {
	case "approve":
		order = []string{"allow_once", "allow_always"}
	case "deny":
		order = []string{"reject_once", "reject_always"}
	default:
		return "", false
	}
	for _, k := range order {
		if id, ok := byKind[k]; ok {
			return id, true
		}
	}
	return "", false
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
			Title:      strutil.FirstNonEmpty(u.Title, name),
			Args:       nonNullRaw(u.RawInput),
			Status:     defaultStr(u.Status, "in_progress"),
		}}}

	case "tool_call_update":
		blocks := decodeContentArray(u.Content)
		evs := make([]mappedEvent, 0, 1+len(blocks))
		// Only a TERMINAL update produces a tool_result (ToolResultData.Status is
		// "completed" | "failed"). An intermediate/in-progress update — or one that
		// omits status — must not be mapped to a completed result, or the status
		// flips to done prematurely and the transcript repeats tool_results.
		if u.Status == "completed" || u.Status == "failed" {
			evs = append(evs, mappedEvent{Type: EvToolResult, Data: ToolResultData{
				ToolCallID: u.ToolCallID,
				Status:     u.Status,
				Content:    nonNullRaw(u.Content),
			}})
		}
		// A tool_call_update may carry one or more diff blocks (one per file); diffs
		// can stream on in-progress updates, so emit them regardless of status.
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
	return strutil.FirstNonEmpty(u.Kind, u.Title, "tool")
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
