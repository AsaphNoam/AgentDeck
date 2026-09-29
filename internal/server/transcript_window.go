package server

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

// A windowed transcript read is bounded in events and bytes at admission, so a
// phone refresh of a long-lived agent never decodes, retains, or ships the
// whole session (FS-20.R13, INV §16). Older windows continue by before_seq.
const (
	transcriptWindowDefault  = 150     // events when a windowed request names no limit
	transcriptWindowMax      = 500     // larger limits are clamped
	transcriptWindowMaxBytes = 1 << 20 // approximate encoded event bytes per window
	transcriptPendingKeep    = 4       // unresolved permission requests tracked
	latestAssistantRunes     = 600
)

// transcriptWindow streams one transcript into a bounded tail. Alongside the
// tail it tracks, in constant space, what the phone derives from the whole
// session: the newest unresolved permission request and the latest assistant
// message, so both stay correct when they fall before the window.
type transcriptWindow struct {
	sinceSeq, beforeSeq int64
	limit               int

	events  []runtime.Event
	start   int
	bytes   int
	hasMore bool

	pending            []pendingPermission
	current, completed string
}

type pendingPermission struct {
	toolCallID string
	event      runtime.Event
}

func (tw *transcriptWindow) visit(ev runtime.Event) error {
	tw.track(ev)
	if (tw.sinceSeq > 0 && ev.Seq <= tw.sinceSeq) || (tw.beforeSeq > 0 && ev.Seq >= tw.beforeSeq) {
		return nil
	}
	tw.events = append(tw.events, ev)
	tw.bytes += eventSize(ev)
	// Drop the oldest until both bounds hold; the newest event is always kept
	// (a single record is already capped by the reader).
	for n := len(tw.events) - tw.start; n > tw.limit || (tw.bytes > transcriptWindowMaxBytes && n > 1); n-- {
		tw.bytes -= eventSize(tw.events[tw.start])
		tw.events[tw.start] = runtime.Event{}
		tw.start++
		tw.hasMore = true
	}
	if tw.start > tw.limit {
		tw.events = slices.Delete(tw.events, 0, tw.start)
		tw.start = 0
	}
	return nil
}

func (tw *transcriptWindow) track(ev runtime.Event) {
	switch ev.Type {
	case runtime.EvPermissionRequest, runtime.EvPermissionResolved:
		var d struct {
			ToolCallID string `json:"tool_call_id"`
		}
		if json.Unmarshal(ev.Data, &d) != nil {
			return
		}
		tw.pending = slices.DeleteFunc(tw.pending, func(p pendingPermission) bool { return p.toolCallID == d.ToolCallID })
		if ev.Type == runtime.EvPermissionRequest {
			tw.pending = append(tw.pending, pendingPermission{toolCallID: d.ToolCallID, event: ev})
			if len(tw.pending) > transcriptPendingKeep {
				tw.pending = slices.Delete(tw.pending, 0, 1)
			}
		}
	case runtime.EvAssistantText:
		// A native child's text is not the agent's reply (FS-03.R58).
		var d runtime.AssistantTextData
		if ev.ActivityID == "" && json.Unmarshal(ev.Data, &d) == nil {
			tw.current = keepTailBytes(tw.current+d.Delta, 4*latestAssistantRunes)
		}
	case runtime.EvTurnEnd:
		if ev.ActivityID != "" {
			return
		}
		if strings.TrimSpace(tw.current) != "" {
			tw.completed = tw.current
		}
		tw.current = ""
	}
}

// response keeps the loopback transcript shape and adds the window fields.
func (tw *transcriptWindow) response(agentID string) map[string]any {
	var pending *runtime.Event
	if n := len(tw.pending); n > 0 {
		pending = &tw.pending[n-1].event
	}
	latest := tw.current
	if strings.TrimSpace(latest) == "" {
		latest = tw.completed
	}
	return map[string]any{
		"agent_id":           agentID,
		"events":             append([]runtime.Event{}, tw.events[tw.start:]...),
		"has_more":           tw.hasMore,
		"pending_permission": pending,
		"latest_assistant":   clipPreview(strings.TrimSpace(latest), latestAssistantRunes),
	}
}

// eventSize approximates an event's encoded size: its payload plus envelope.
func eventSize(ev runtime.Event) int {
	return len(ev.Data) + len(ev.AgentID) + len(ev.Type) + len(ev.Ts) + len(ev.ActivityID) + len(ev.ParentActivityID) + 96
}

// keepTailBytes keeps at most the last n bytes of s without splitting a rune.
func keepTailBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[len(s)-n:]
	for len(s) > 0 && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	return s
}
