package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Native fork (FS-01.R36, TS-04.R65). Clone launches a fresh adapter process
// that forks the source's provider session through standard ACP session/fork;
// the provider conversation already holds the source context, so no system
// prompt or primer is sent. Replayed history is dropped like session/load's:
// AgentDeck's own durable copy of the source transcript is the clone's record.

// EvForkBoundary marks where a clone's copied history ends and its own begins.
const EvForkBoundary = "fork_boundary"

// ForkBoundaryData links a clone back to its source.
type ForkBoundaryData struct {
	ForkedFromAgentID string `json:"forked_from_agent_id"`
	ForkedFromSeq     int64  `json:"forked_from_seq"`
}

// ErrForkUnavailable: the new peer did not advertise session fork, so no weaker
// clone path is attempted (FS-01.R36).
var ErrForkUnavailable = errors.New("runtime: this agent's runtime cannot fork its conversation")

const forkDeleteTimeout = 5 * time.Second

func sessionForkParams(spec LaunchSpec, sourceSessionID string) map[string]any {
	mcp := make([]map[string]any, 0, len(spec.MCPServers))
	for _, m := range spec.MCPServers {
		mcp = append(mcp, mcpServerParam(m))
	}
	return map[string]any{
		"sessionId":             sourceSessionID,
		"cwd":                   spec.Cwd,
		"mcpServers":            mcp,
		"additionalDirectories": spec.StartAddDirs(),
	}
}

// openFork forks the plan's source session and returns the new provider
// session id and its response. Only a non-empty returned id is trusted.
func (c *ChatRuntime) openFork(ctx context.Context, as *agentState, spec LaunchSpec) (string, json.RawMessage, error) {
	if !as.capabilities().Fork {
		return "", nil, ErrForkUnavailable
	}
	as.mu.Lock()
	as.loadReplay = true
	as.mu.Unlock()
	res, err := c.startupCall(ctx, as.transport, "session/fork", sessionForkParams(spec, spec.Fork.SourceSessionID))
	as.mu.Lock()
	as.loadReplay = false
	as.mu.Unlock()
	if err != nil {
		return "", nil, err
	}
	var body struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(res, &body) != nil || body.SessionID == "" {
		return "", nil, fmt.Errorf("runtime: session/fork returned no sessionId")
	}
	return body.SessionID, res, nil
}

// deleteForkedSession removes a forked provider session whose local commit
// failed, best-effort, before the process is torn down (TS-01.R35, INV §15).
func (c *ChatRuntime) deleteForkedSession(as *agentState, sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), forkDeleteTimeout)
	defer cancel()
	if _, err := as.transport.Call(ctx, "session/delete", map[string]any{"sessionId": sessionID}); err != nil {
		slog.Warn("runtime: delete forked session after failed clone", "agent", as.agentID, "err", err)
	}
}

// writeForkPrefix commits the durable copy of the source's visible transcript
// under the clone's identity and sequence, then marks the boundary. The copy
// is persisted and indexed but not published: it is history, not live work.
func (c *ChatRuntime) writeForkPrefix(as *agentState, plan *ForkPlan) {
	for _, ev := range plan.Prefix {
		as.mu.Lock()
		as.seq++
		ev.AgentID, ev.Generation, ev.Seq = as.agentID, as.generation, as.seq
		as.transcript = append(as.transcript, ev)
		as.mu.Unlock()
		c.persistEvent(as, ev)
	}
	c.emit(as, EvForkBoundary, ForkBoundaryData{ForkedFromAgentID: plan.SourceAgentID, ForkedFromSeq: plan.SourceSeq})
}
