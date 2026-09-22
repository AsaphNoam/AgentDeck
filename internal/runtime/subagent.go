package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"

	"github.com/agentdeck/agentdeck/internal/strutil"
)

// Native child sessions (FS-03.R58, TS-01.R35, TS-04.R63). With negotiated
// subagents the adapter announces each child under its immediate parent's
// session id and then sends the child's ordinary updates and permission
// requests under the child's own session id. Runtime maps that provider id to
// an AgentDeck activity scope; no provider id crosses this package.

const (
	maxChildren       = 256
	maxActivityName   = 120
	maxActivityTask   = 500
	maxChildDiagnosis = 3
)

// ActivityStartedData announces a child before any of its output.
type ActivityStartedData struct {
	Name string `json:"name,omitempty"`
	Task string `json:"task,omitempty"`
}

// ActivityStateData is a child's terminal lifecycle state:
// completed | failed | stopped | disconnected.
type ActivityStateData struct {
	State string `json:"state"`
}

// activityScope places an event under a child; the zero value is the root.
type activityScope struct {
	ActivityID       string
	ParentActivityID string
}

// activityIDFor derives a stable, bounded AgentDeck id from the provider's
// child session id, so the same child always maps to the same scope.
func activityIDFor(childSessionID string) string {
	sum := sha256.Sum256([]byte(childSessionID))
	return "act_" + hex.EncodeToString(sum[:8])
}

// scopedToolCallID is the normalized composite id for a child tool call. Root
// ids stay unchanged; sibling children with equal raw ids cannot collide in
// the one pending-permission map (INV §5/§11).
func (s activityScope) toolCallID(raw string) string {
	if s.ActivityID == "" || raw == "" {
		return raw
	}
	return s.ActivityID + "/" + raw
}

// sessionIDOf reads the top-level ACP session id of a notification or request.
func sessionIDOf(params json.RawMessage) string {
	var body struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(params, &body)
	return body.SessionID
}

// scopeFor resolves a provider session id to its activity scope. The root
// session (or an unaddressed frame) is the root scope. An unknown id is refused
// only once subagents are negotiated; before that no frame can belong to a
// child, so an adapter that does not echo the root id exactly keeps working.
func (as *agentState) scopeFor(sessionID string) (activityScope, bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	if sessionID == "" || as.sessionID == "" || sessionID == as.sessionID {
		return activityScope{}, true
	}
	if scope, ok := as.children[sessionID]; ok {
		return scope, true
	}
	return activityScope{}, !as.caps.Subagents
}

// noteUnknownChild logs a bounded diagnostic for a frame addressed to a child
// session this generation never announced; no identity is invented.
func (as *agentState) noteUnknownChild(sessionID string) {
	as.mu.Lock()
	as.unknownChildFrames++
	n := as.unknownChildFrames
	as.mu.Unlock()
	if n <= maxChildDiagnosis {
		slog.Warn("runtime: frame for unannounced child session dropped", "agent", as.agentID, "count", n)
	}
}

// childStateFor maps ACP subagent states onto AgentDeck's declared enum.
func childStateFor(state string) (string, bool) {
	switch state {
	case "completed":
		return "completed", true
	case "failed":
		return "failed", true
	case "cancelled", "interrupted":
		return "stopped", true
	case "disconnected":
		return "disconnected", true
	}
	return "", false
}

// onSubagentUpdate handles child lifecycle updates, reporting whether params
// was one. The announcement arrives under the child's immediate parent.
func (c *ChatRuntime) onSubagentUpdate(as *agentState, params json.RawMessage) bool {
	var su struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			SessionUpdate     string `json:"sessionUpdate"`
			SubagentSessionID string `json:"subagentSessionId"`
			Name              string `json:"name"`
			Task              string `json:"task"`
			State             string `json:"state"`
		} `json:"update"`
	}
	if json.Unmarshal(params, &su) != nil {
		return false
	}
	u := su.Update
	switch u.SessionUpdate {
	case "subagent_spawned":
		parent, ok := as.scopeFor(su.SessionID)
		if !ok || u.SubagentSessionID == "" {
			as.noteUnknownChild(su.SessionID)
			return true
		}
		scope := activityScope{ActivityID: activityIDFor(u.SubagentSessionID), ParentActivityID: parent.ActivityID}
		as.mu.Lock()
		_, known := as.children[u.SubagentSessionID]
		full := len(as.children) >= maxChildren
		if !known && !full {
			if as.children == nil {
				as.children = map[string]activityScope{}
			}
			as.children[u.SubagentSessionID] = scope
		}
		as.mu.Unlock()
		if known {
			return true
		}
		if full {
			as.noteUnknownChild(u.SubagentSessionID)
			return true
		}
		c.emitIn(as, scope, EvActivityStarted, ActivityStartedData{
			Name: strutil.ClipRunes(u.Name, maxActivityName),
			Task: strutil.ClipRunes(u.Task, maxActivityTask),
		})
		return true
	case "subagent_state_update":
		as.mu.Lock()
		scope, ok := as.children[u.SubagentSessionID]
		as.mu.Unlock()
		state, valid := childStateFor(u.State)
		if !ok || !valid {
			as.noteUnknownChild(u.SubagentSessionID)
			return true
		}
		c.emitIn(as, scope, EvActivityState, ActivityStateData{State: state})
		return true
	}
	return false
}

// scopeMapped rewrites a child's mapped tool identities to the composite id.
func scopeMapped(scope activityScope, m mappedEvent) mappedEvent {
	if scope.ActivityID == "" {
		return m
	}
	switch d := m.Data.(type) {
	case ToolCallData:
		d.ToolCallID = scope.toolCallID(d.ToolCallID)
		m.Data = d
	case ToolResultData:
		d.ToolCallID = scope.toolCallID(d.ToolCallID)
		m.Data = d
	case DiffData:
		d.ToolCallID = scope.toolCallID(d.ToolCallID)
		m.Data = d
	}
	return m
}
