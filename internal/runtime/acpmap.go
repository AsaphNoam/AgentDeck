package runtime

import (
	"encoding/json"

	"github.com/agentdeck/agentdeck/internal/strutil"
)

// Bounds on the decoded available-commands snapshot so provider output cannot
// create an unbounded user-facing payload (TS-04.R24, INV §8).
const (
	maxCommandEntries = 256
	maxCommandName    = 256
	maxCommandText    = 2000
)

type acpSessionConfigOption struct {
	ID           string          `json:"id"`
	ConfigID     string          `json:"configId"`
	CurrentValue json.RawMessage `json:"currentValue"`
}

// sessionConfigAdvertisement is the normalized answer to "which configuration
// options does this session offer, and what is each one effectively set to"
// (TS-04.R46). It carries no ACP option structs, no type/category vocabulary,
// and no provider display copy past this boundary (INV §8).
type sessionConfigAdvertisement map[string]string

func (a sessionConfigAdvertisement) has(id string) bool {
	if id == "" {
		return false
	}
	_, ok := a[id]
	return ok
}

// replace overwrites the advertisement in place so every holder of the map sees
// the peer's rebuilt option list. Applying a model resets which efforts exist and
// whether fast mode exists at all (TS-04.R47), so a later lookup against the
// pre-model list would gate the next setting on the wrong session state.
func (a sessionConfigAdvertisement) replace(next sessionConfigAdvertisement) {
	for id := range a {
		delete(a, id)
	}
	for id, value := range next {
		a[id] = value
	}
}

// decodeSessionConfigOptions reads a `configOptions` array from any response that
// carries one — `session/new`, `session/load`, and `session/set_config_option`
// alike. The value is the peer's own report of the option's effective setting, so
// a caller can check that a call it made was honored rather than only accepted
// (INV §12). An absent, null, or unparseable list advertises nothing, which fails
// open under TS-04.R45/R46 rather than failing the session (INV §7).
func decodeSessionConfigOptions(result json.RawMessage) sessionConfigAdvertisement {
	options, _ := decodeSessionConfigOptionsWithPresence(result)
	return options
}

// decodeSessionConfigOptionsWithPresence additionally reports whether the
// response carried a valid configOptions array. Callers replacing an existing
// advertisement need this distinction because an explicit [] is the peer's
// complete, empty replacement rather than an omitted update (TS-04.R46).
func decodeSessionConfigOptionsWithPresence(result json.RawMessage) (sessionConfigAdvertisement, bool) {
	var response struct {
		ConfigOptions json.RawMessage `json:"configOptions"`
	}
	out := sessionConfigAdvertisement{}
	if json.Unmarshal(result, &response) != nil {
		return out, false
	}
	var options []acpSessionConfigOption
	if len(response.ConfigOptions) == 0 || json.Unmarshal(response.ConfigOptions, &options) != nil || options == nil {
		return out, false
	}
	for _, option := range options {
		id := option.ID
		if id == "" {
			id = option.ConfigID
		}
		if id != "" {
			out[id] = normalizeConfigValue(option.CurrentValue)
		}
	}
	return out, true
}

// normalizeConfigValue reduces a reported option value to the string vocabulary
// AgentDeck sends. AgentDeck advertises no boolean client capability, so both
// pinned adapters degrade to a two-value select and a string is the expected
// form (TS-04.R45); a boolean or a wrapped select entry is folded to the same
// spelling instead of being rejected, and any other shape reports nothing so the
// caller treats the value as unreported rather than as a mismatch (INV §12).
func normalizeConfigValue(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var flag bool
	if json.Unmarshal(raw, &flag) == nil {
		if flag {
			return "on"
		}
		return "off"
	}
	var wrapped struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &wrapped) == nil && wrapped.ID != "" {
		return wrapped.ID
	}
	return wrapped.Value
}

// CommandItem is one entry of a chat runtime's advertised ACP command snapshot,
// projected verbatim to the composer command picker (TS-03.R24). InputHint is the
// unstructured `input.hint` when the adapter supplies one.
type CommandItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputHint   string `json:"input_hint,omitempty"`
}

// acpCommandsParams decodes an `available_commands_update` session/update. The
// standard item carries name, description, and an optional unstructured input hint.
type acpCommandsParams struct {
	Update struct {
		SessionUpdate     string            `json:"sessionUpdate"`
		AvailableCommands []acpCommandEntry `json:"availableCommands"`
	} `json:"update"`
}

type acpCommandEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Input       *struct {
		Hint string `json:"hint"`
	} `json:"input"`
}

// decodeAvailableCommands recognizes an `available_commands_update` notification
// and returns its validated, bounded snapshot (TS-04.R24). ok is false for any
// other update kind so the caller falls through to normalized-event mapping. A
// valid update with no items returns an empty (non-nil) slice, which clears the
// snapshot. Entries without a name are skipped without dropping the valid ones
// (INV §7); names and text are rune-clamped at the decode boundary (INV §8).
func decodeAvailableCommands(params json.RawMessage) ([]CommandItem, bool) {
	var p acpCommandsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, false
	}
	if p.Update.SessionUpdate != "available_commands_update" {
		return nil, false
	}
	out := make([]CommandItem, 0, len(p.Update.AvailableCommands))
	for _, c := range p.Update.AvailableCommands {
		if len(out) >= maxCommandEntries {
			break
		}
		if c.Name == "" {
			continue
		}
		item := CommandItem{
			Name:        strutil.ClipRunes(c.Name, maxCommandName),
			Description: strutil.ClipRunes(c.Description, maxCommandText),
		}
		if c.Input != nil {
			item.InputHint = strutil.ClipRunes(c.Input.Hint, maxCommandText)
		}
		out = append(out, item)
	}
	return out, true
}

// decodeContextUsage recognizes the ACP usage_update notification that carries
// the current context usage and window size (TS-04.R25). Its value is bounded
// at the provider boundary because context_pct is a 0..1 dashboard contract.
func decodeContextUsage(params json.RawMessage) (float64, bool) {
	var su acpSessionUpdate
	if err := json.Unmarshal(params, &su); err != nil || su.Update.SessionUpdate != "usage_update" {
		return 0, false
	}
	if su.Update.Used < 0 || su.Update.Size <= 0 {
		return 0, false
	}
	pct := float64(su.Update.Used) / float64(su.Update.Size)
	if pct > 1 {
		pct = 1
	}
	return pct, true
}

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

	// usage_update: the current context usage and its window size.
	Used int `json:"used"`
	Size int `json:"size"`
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

// acpUsage is the token accounting returned by the pinned Claude adapter after
// a prompt. It does not contain a context-window size, so it cannot itself
// produce context_pct (TS-04.R25).
type acpUsage struct {
	InputTokens       int `json:"inputTokens"`
	OutputTokens      int `json:"outputTokens"`
	CachedReadTokens  int `json:"cachedReadTokens"`
	CachedWriteTokens int `json:"cachedWriteTokens"`
	TotalTokens       int `json:"totalTokens"`
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

func permissionToolIdentity(params json.RawMessage) string {
	var pr acpPermissionRequest
	if json.Unmarshal(params, &pr) != nil {
		return ""
	}
	return pr.ToolCall.Title
}

// mapPermissionRequest converts the ACP permission request into normalized
// PermissionRequestData plus a kind→optionId table for choosing the reply option
// (techspec §5.3). expiresAt is RFC3339; autoApproved reflects skip_permissions.
func mapPermissionRequest(params json.RawMessage, expiresAt string, autoApproved bool) (PermissionRequestData, map[string]string) {
	var pr acpPermissionRequest
	_ = json.Unmarshal(params, &pr)

	name := strutil.FirstNonEmpty(pr.ToolCall.Title, pr.ToolCall.Kind, "tool")
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

// mapPromptResult converts the `session/prompt` result into a turn_end event.
// Context percentage arrives separately in usage_update (TS-04.R25).
func mapPromptResult(result json.RawMessage) (TurnEndData, bool) {
	var r acpPromptResult
	if err := json.Unmarshal(result, &r); err != nil {
		return TurnEndData{StopReason: "end_turn"}, false
	}
	td := TurnEndData{StopReason: defaultStr(r.StopReason, "end_turn")}
	return td, false
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
