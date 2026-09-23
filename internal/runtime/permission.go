package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"
)

func permissionTimeout() time.Duration {
	if v := os.Getenv("PERMISSION_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 0
}

// onRequest handles a server→client JSON-RPC request. Only
// session/request_permission is meaningful this phase; the pause is achieved by
// withholding the response (techspec §5.1).
func (c *ChatRuntime) onRequest(as *agentState, req *IncomingRequest) {
	if req.Method != "session/request_permission" {
		_ = req.RespondError(-32601, "method not found: "+req.Method)
		return
	}

	// A child's request arrives under its own session id and resolves through
	// this same single-winner gate under its composite tool-call id (TS-04.R63).
	scope, known := as.scopeFor(sessionIDOf(req.Params))
	if !known {
		as.noteUnknownChild(sessionIDOf(req.Params))
		_ = req.Respond(cancelledOutcome())
		return
	}

	identity := permissionToolIdentity(req.Params)
	_, agentDeckTool := as.autoApproveTools[identity]
	autoData, autoKinds := mapPermissionRequest(req.Params, "", true)
	autoData.ToolCallID = scope.toolCallID(autoData.ToolCallID)
	_, hasAllowOnce := autoKinds["allow_once"]
	// skip_permissions auto-approves any tool. AgentDeck-owned actions use the
	// same recorded shape, but only when an allow-once option is available.
	if as.skipPerms || (agentDeckTool && hasAllowOnce) {
		c.emitIn(as, scope, EvPermissionRequest, autoData)
		optID, ok := selectOption(autoKinds, "approve")
		if agentDeckTool && !as.skipPerms {
			optID, ok = autoKinds["allow_once"]
		}
		if !ok {
			_ = req.Respond(cancelledOutcome())
			c.emit(as, EvError, ErrorData{Scope: "tool", Message: "no allow option offered"})
			return
		}
		c.emitIn(as, scope, EvPermissionResolved, PermissionResolvedData{ToolCallID: autoData.ToolCallID, Decision: "auto_approve"})
		_ = req.Respond(selectedOutcome(optID))
		return
	}

	timeout := permissionTimeout()
	expiresAt := ""
	if timeout > 0 {
		expiresAt = time.Now().UTC().Add(timeout).Format(time.RFC3339)
	}
	data, byKind := mapPermissionRequest(req.Params, expiresAt, false)
	data.ToolCallID = scope.toolCallID(data.ToolCallID)

	p := &pendingPerm{req: req, name: data.Name, optByKind: byKind, scope: scope}
	toolCallID := data.ToolCallID
	if timeout > 0 {
		p.timer = time.AfterFunc(timeout, func() { c.onPermissionTimeout(as, toolCallID) })
	}

	as.mu.Lock()
	as.pending[toolCallID] = p
	as.mu.Unlock()

	// waiting_input while withheld (techspec §4.4).
	c.updateStatus(as, "waiting_input", "Permission: "+data.Name, "PermissionRequest: "+data.Name, keepBusySince)
	c.emitIn(as, scope, EvPermissionRequest, data)
	// NB: no req.Respond here — withholding the response IS the pause.
}

// Permission relays an approve/deny decision for a pending request (techspec §5).
func (c *ChatRuntime) Permission(ctx context.Context, agentID, toolCallID, decision string) error {
	as, err := c.lookup(agentID)
	if err != nil {
		return err
	}

	p, err := c.takePending(as, toolCallID)
	if err != nil {
		return err
	}

	optID, ok := selectOption(p.optByKind, decision)
	if !ok {
		// Either an invalid decision or the adapter offered no usable option.
		if decision != "approve" && decision != "deny" {
			c.restorePending(as, toolCallID, p)
			return ErrInvalidDecision
		}
		p.resolve("cancelled", "")
		c.markResolved(as, toolCallID)
		c.emit(as, EvError, ErrorData{Scope: "tool", Message: "adapter offered no usable permission option"})
		return nil
	}

	c.markResolved(as, toolCallID)
	// Write the resolved-but-active state before releasing the ACP peer. A peer
	// may finish session/prompt as soon as it receives this response; writing busy
	// afterwards could then overwrite the prompt goroutine's final idle status.
	c.updateStatus(as, "busy", "thinking", "PermissionResolved", keepBusySince)
	c.emitIn(as, p.scope, EvPermissionResolved, PermissionResolvedData{ToolCallID: toolCallID, Decision: decision})
	p.resolve("selected", optID)
	return nil
}

// onPermissionTimeout auto-denies a request that was never decided (techspec §5.4).
func (c *ChatRuntime) onPermissionTimeout(as *agentState, toolCallID string) {
	p, err := c.takePending(as, toolCallID)
	if err != nil {
		return
	}
	optID, found := selectOption(p.optByKind, "deny")
	c.markResolved(as, toolCallID)
	// As with a user decision, write busy before answering the peer so a fast
	// prompt completion is the final owner of the idle transition.
	c.updateStatus(as, "busy", "thinking", "PermissionResolved", keepBusySince)
	c.emitIn(as, p.scope, EvPermissionResolved, PermissionResolvedData{ToolCallID: toolCallID, Decision: "timeout"})
	c.emit(as, EvError, ErrorData{Scope: "tool", Message: "permission timed out"})
	if found {
		p.resolve("selected", optID)
	} else {
		p.resolve("cancelled", "")
	}
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
	return c.cancel(as, "", "")
}

// CancelGuarded interrupts only the expected launch generation and active turn.
// It is the task-cleanup path: a stale task must leave a resumed generation or
// later human turn untouched. The guard is checked while the turn lock remains
// held through the initial ACP cancel notification.
func (c *ChatRuntime) CancelGuarded(ctx context.Context, agentID, expectedGeneration, expectedTurn string) (bool, error) {
	as, err := c.lookup(agentID)
	if err != nil {
		return false, err
	}
	return c.cancel(as, expectedGeneration, expectedTurn)
}

func (c *ChatRuntime) cancel(as *agentState, expectedGeneration, expectedTurn string) (bool, error) {
	guarded := expectedGeneration != "" || expectedTurn != ""

	as.mu.Lock()
	if guarded && (as.generation != expectedGeneration || !as.turnActive || expectedTurn != fmt.Sprintf("t_%012d", as.turnSeq)) {
		as.mu.Unlock()
		return false, nil
	}
	ids := make([]string, 0, len(as.pending))
	for id := range as.pending {
		ids = append(ids, id)
	}
	active := as.turnActive
	armedTurn := as.turnSeq
	cancelled := active || len(ids) > 0
	// Keep the turn lock through the initial cancel. A turn cannot settle and a
	// later turn cannot start between the guarded check and this side effect.
	if cancelled {
		_ = as.transport.Notify("session/cancel", map[string]any{"sessionId": as.sessionID})
	}
	as.mu.Unlock()

	for _, id := range ids {
		c.resolvePending(as, id, "cancelled", "")
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
		if stillBusy && hasProc && pgid > 0 {
			if err := syscall.Kill(-pgid, syscall.SIGINT); err == nil {
				as.cancelEscalated = true
			}
		}
		as.mu.Unlock()
	}()
}

// resolvePending answers a withheld permission request as cancelled and removes
// it. Like the approve/deny and timeout paths, it emits and persists a
// permission_resolved (decision "cancelled") before releasing the ACP peer, so
// the live UI and the durable transcript both learn the prompt is dead and render
// a resolved chip instead of leaving Approve/Deny actionable forever (FS-03.R9,
// R14–R16). Returns false if no such pending request remains.
func (c *ChatRuntime) resolvePending(as *agentState, toolCallID, outcome, optionID string) bool {
	p, err := c.takePending(as, toolCallID)
	if err != nil {
		return false
	}
	c.markResolved(as, toolCallID)
	c.emitIn(as, p.scope, EvPermissionResolved, PermissionResolvedData{ToolCallID: toolCallID, Decision: "cancelled"})
	p.resolve(outcome, optionID)
	return true
}

func (c *ChatRuntime) takePending(as *agentState, toolCallID string) (*pendingPerm, error) {
	as.mu.Lock()
	defer as.mu.Unlock()
	p, ok := as.pending[toolCallID]
	if ok {
		delete(as.pending, toolCallID)
		return p, nil
	}
	if _, resolved := as.resolved[toolCallID]; resolved {
		return nil, ErrPermissionAlreadyResolved
	}
	return nil, ErrNoPendingPermission
}

func (c *ChatRuntime) restorePending(as *agentState, toolCallID string, p *pendingPerm) {
	as.mu.Lock()
	defer as.mu.Unlock()
	if _, exists := as.pending[toolCallID]; exists {
		return
	}
	as.pending[toolCallID] = p
}

func (c *ChatRuntime) markResolved(as *agentState, toolCallID string) {
	as.mu.Lock()
	defer as.mu.Unlock()
	if as.resolved == nil {
		as.resolved = map[string]struct{}{}
	}
	as.resolved[toolCallID] = struct{}{}
}

func (p *pendingPerm) resolve(outcome, optionID string) {
	if p.timer != nil {
		p.timer.Stop()
	}
	if outcome == "selected" {
		_ = p.req.Respond(selectedOutcome(optionID))
		return
	}
	_ = p.req.Respond(cancelledOutcome())
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
