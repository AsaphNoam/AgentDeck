package runtime

import (
	"context"
	"encoding/json"
	"os"
	"syscall"
	"time"
)

// defaultPermissionTimeout is the auto-deny deadline for a pending permission
// (techspec §5.4). Overridable via PERMISSION_TIMEOUT (a Go duration string).
const defaultPermissionTimeout = 180 * time.Second

func permissionTimeout() time.Duration {
	if v := os.Getenv("PERMISSION_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultPermissionTimeout
}

// onRequest handles a server→client JSON-RPC request. Only
// session/request_permission is meaningful this phase; the pause is achieved by
// withholding the response (techspec §5.1).
func (c *ChatRuntime) onRequest(as *agentState, req *IncomingRequest) {
	if req.Method != "session/request_permission" {
		_ = req.RespondError(-32601, "method not found: "+req.Method)
		return
	}

	// skip_permissions: auto-approve without entering waiting_input (§5.2).
	if as.skipPerms {
		data, byKind := mapPermissionRequest(req.Params, "", true)
		c.emit(as, EvPermissionRequest, data)
		optID, ok := selectOption(byKind, "approve")
		if !ok {
			_ = req.Respond(cancelledOutcome())
			c.emit(as, EvError, ErrorData{Scope: "tool", Message: "no allow option offered"})
			return
		}
		_ = req.Respond(selectedOutcome(optID))
		c.emit(as, EvPermissionResolved, PermissionResolvedData{ToolCallID: data.ToolCallID, Decision: "auto_approve"})
		return
	}

	timeout := permissionTimeout()
	expiresAt := time.Now().UTC().Add(timeout).Format(time.RFC3339)
	data, byKind := mapPermissionRequest(req.Params, expiresAt, false)

	p := &pendingPerm{req: req, name: data.Name, optByKind: byKind}
	toolCallID := data.ToolCallID
	p.timer = time.AfterFunc(timeout, func() { c.onPermissionTimeout(as, toolCallID) })

	as.mu.Lock()
	as.pending[toolCallID] = p
	as.mu.Unlock()

	// waiting_input while withheld (techspec §4.4).
	c.updateStatus(as, "waiting_input", "Permission: "+data.Name, "PermissionRequest: "+data.Name, keepBusySince)
	c.emit(as, EvPermissionRequest, data)
	// NB: no req.Respond here — withholding the response IS the pause.
}

// Permission relays an approve/deny decision for a pending request (techspec §5).
func (c *ChatRuntime) Permission(ctx context.Context, agentID, toolCallID, decision string) error {
	as, err := c.lookup(agentID)
	if err != nil {
		return err
	}

	as.mu.Lock()
	p, ok := as.pending[toolCallID]
	as.mu.Unlock()
	if !ok {
		return ErrNoPendingPermission
	}

	optID, ok := selectOption(p.optByKind, decision)
	if !ok {
		// Either an invalid decision or the adapter offered no usable option.
		if decision != "approve" && decision != "deny" {
			return ErrInvalidDecision
		}
		c.resolvePending(as, toolCallID, "cancelled", "")
		c.emit(as, EvError, ErrorData{Scope: "tool", Message: "adapter offered no usable permission option"})
		return nil
	}

	c.resolvePending(as, toolCallID, "selected", optID)
	c.emit(as, EvPermissionResolved, PermissionResolvedData{ToolCallID: toolCallID, Decision: decision})
	c.updateStatus(as, "busy", "thinking", "PermissionResolved", keepBusySince)
	return nil
}

// onPermissionTimeout auto-denies a request that was never decided (techspec §5.4).
func (c *ChatRuntime) onPermissionTimeout(as *agentState, toolCallID string) {
	as.mu.Lock()
	p, ok := as.pending[toolCallID]
	as.mu.Unlock()
	if !ok {
		return
	}
	optID, found := selectOption(p.optByKind, "deny")
	if found {
		c.resolvePending(as, toolCallID, "selected", optID)
	} else {
		c.resolvePending(as, toolCallID, "cancelled", "")
	}
	c.emit(as, EvPermissionResolved, PermissionResolvedData{ToolCallID: toolCallID, Decision: "timeout"})
	c.emit(as, EvError, ErrorData{Scope: "tool", Message: "permission timed out"})
	c.updateStatus(as, "busy", "thinking", "PermissionResolved", keepBusySince)
}

// Cancel interrupts the in-progress turn. Any pending permission is first
// resolved as cancelled (freeing the agent), then an ACP session/cancel
// notification is sent (techspec §8.4). When idle it is a no-op and reports
// false. If the turn was active it also arms a grace-then-SIGINT escalation so a
// peer that ignores session/cancel does not stay busy until a hard Stop.
func (c *ChatRuntime) Cancel(ctx context.Context, agentID string) (bool, error) {
	as, err := c.lookup(agentID)
	if err != nil {
		return false, err
	}

	as.mu.Lock()
	ids := make([]string, 0, len(as.pending))
	for id := range as.pending {
		ids = append(ids, id)
	}
	active := as.turnActive
	armedTurn := as.turnSeq
	as.mu.Unlock()

	for _, id := range ids {
		c.resolvePending(as, id, "cancelled", "")
	}

	cancelled := active || len(ids) > 0
	if cancelled {
		_ = as.transport.Notify("session/cancel", map[string]any{"sessionId": as.sessionID})
	}
	if active {
		c.escalateCancel(as, armedTurn)
	}
	return cancelled, nil
}

// escalateCancel sends SIGINT to the agent's process group if the turn is still
// active after the cancel grace window — a fallback for an ACP peer that ignores
// session/cancel (techspec §8.4). It stops short of a hard kill (SIGTERM/SIGKILL);
// that remains Stop's job. A non-positive grace disables escalation.
//
// armedTurn is the turn generation captured when the escalation was armed. At fire
// time we SIGINT only if that generation is STILL the current turn — a peer that
// honored the cancel quickly (turn ends) followed by a fresh re-prompt increments
// turnSeq, so this stale escalation is a no-op against the healthy next turn rather
// than interrupting it (review Finding 11).
func (c *ChatRuntime) escalateCancel(as *agentState, armedTurn int64) {
	grace := c.cancelGrace
	if grace <= 0 {
		return
	}
	go func() {
		select {
		case <-as.ctx.Done():
			return
		case <-time.After(grace):
		}
		as.mu.Lock()
		// Only escalate if the SAME turn we armed against is still active. A new
		// turn (re-prompt) bumped turnSeq, making this escalation stale.
		stillBusy := as.turnActive && !as.stopped && as.turnSeq == armedTurn
		pgid := as.pgid
		hasProc := as.cmd != nil && as.cmd.Process != nil
		as.mu.Unlock()
		if stillBusy && hasProc && pgid > 0 {
			_ = syscall.Kill(-pgid, syscall.SIGINT)
		}
	}()
}

// resolvePending answers a withheld permission request and removes it. Returns
// false if no such pending request remains.
func (c *ChatRuntime) resolvePending(as *agentState, toolCallID, outcome, optionID string) bool {
	as.mu.Lock()
	p, ok := as.pending[toolCallID]
	if ok {
		delete(as.pending, toolCallID)
	}
	as.mu.Unlock()
	if !ok {
		return false
	}
	if p.timer != nil {
		p.timer.Stop()
	}
	if outcome == "selected" {
		_ = p.req.Respond(selectedOutcome(optionID))
	} else {
		_ = p.req.Respond(cancelledOutcome())
	}
	return true
}

func selectedOutcome(optionID string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": optionID}})
	return b
}

func cancelledOutcome() json.RawMessage {
	b, _ := json.Marshal(map[string]any{"outcome": map[string]any{"outcome": "cancelled"}})
	return b
}

// StopAll stops every live agent. The server calls this on shutdown so no
// orphaned CLI process groups survive (techspec §8.5).
func (c *ChatRuntime) StopAll(ctx context.Context) {
	c.mu.Lock()
	ids := make([]string, 0, len(c.agents))
	for id := range c.agents {
		ids = append(ids, id)
	}
	c.mu.Unlock()
	for _, id := range ids {
		_ = c.Stop(ctx, id)
	}
}
