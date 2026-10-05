package server

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/AsaphNoam/Chuck/internal/runtime"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// Room activity capture (TS-14.R10). While a room attempt runs, the normalized
// events of exactly its generation and executing turn are copied into
// room-owned storage as they arrive. Private prompts, assistant prose and
// reasoning are not copied; the contribution is the staged submission.

// thinkTankLiveActivityLimit bounds one think_tank_activity SSE payload; a
// larger record is announced and refetched over REST (TS-14.R16, INV §16).
const thinkTankLiveActivityLimit = 256 << 10

type thinkTankCapture struct {
	roomID, attemptID, generation, turnID string
	agentName, project, cwd               string
	opening                               bool
	// internal names Chuck's own tool calls in this turn, so their results
	// are projected as receipts too.
	internal map[string]bool
	// failed records a storage failure; the turn then cannot publish.
	failed bool
}

var thinkTankCapturedTypes = map[string]bool{
	runtime.EvToolCall: true, runtime.EvToolResult: true, runtime.EvDiff: true,
	runtime.EvPermissionRequest: true, runtime.EvPermissionResolved: true, runtime.EvError: true,
	runtime.EvActivityStarted: true, runtime.EvActivityState: true,
}

func (s *Server) beginThinkTankCapture(agentID string, a state.ThinkTankAttempt, name, project string) {
	cwd := ""
	if snap, err := s.stateStore.ReadSession(agentID); err == nil {
		cwd = snap.Cwd
	}
	s.thinkTankCaptureMu.Lock()
	s.thinkTankCaptures[agentID] = &thinkTankCapture{
		roomID: a.RoomID, attemptID: a.AttemptID, generation: a.Generation, turnID: a.TurnID,
		agentName: name, project: project, cwd: cwd, opening: a.Turn == state.ThinkTankTurnOpening,
		internal: map[string]bool{},
	}
	s.thinkTankCaptureMu.Unlock()
}

// endThinkTankCapture removes the capture owned by exactly this turn and
// reports whether its storage failed.
func (s *Server) endThinkTankCapture(agentID, generation, turnID string) (failed bool) {
	s.thinkTankCaptureMu.Lock()
	defer s.thinkTankCaptureMu.Unlock()
	c := s.thinkTankCaptures[agentID]
	if c == nil || c.generation != generation || c.turnID != turnID {
		return false
	}
	delete(s.thinkTankCaptures, agentID)
	return c.failed
}

// captureThinkTankEvent runs on the runtime event sink, so capture order is
// emission order. Events without the attempt's exact generation and turn are
// never guessed into a room (TS-14.R4).
func (s *Server) captureThinkTankEvent(ev runtime.Event) {
	if ev.TurnID == "" || !thinkTankCapturedTypes[ev.Type] {
		return
	}
	s.thinkTankCaptureMu.Lock()
	defer s.thinkTankCaptureMu.Unlock()
	c := s.thinkTankCaptures[ev.AgentID]
	if c == nil || c.generation != ev.Generation || c.turnID != ev.TurnID || c.failed {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"type": ev.Type, "seq": ev.Seq, "ts": ev.Ts, "activity_id": ev.ActivityID,
		"parent_activity_id": ev.ParentActivityID, "data": s.projectThinkTankData(c, ev),
	})
	if err != nil {
		c.failed = true
		return
	}
	rec, err := s.stateStore.AppendThinkTankActivity(state.ThinkTankActivity{
		RoomID: c.roomID, AttemptID: c.attemptID, AgentID: ev.AgentID, AgentName: c.agentName,
		Project: c.project, Cwd: c.cwd, Generation: c.generation, TurnID: c.turnID,
		SourceSeq: ev.Seq, Payload: string(payload),
	}, ev.Type)
	if err != nil {
		if !errors.Is(err, state.ErrThinkTankStale) {
			s.log.Warn("capture think tank activity", "room", c.roomID, "err", err)
			c.failed = true
		}
		return
	}
	// Blind openings are not offered on the live stream before publication.
	if !c.opening {
		s.eventBus.Publish("think_tank_activity", nil, thinkTankActivityLive(rec))
	}
}

func (s *Server) isChuckTool(name string) bool {
	for _, tool := range s.messaging.ToolNames() {
		if name == tool || strings.HasSuffix(name, "__"+tool) {
			return true
		}
	}
	return false
}

// projectThinkTankData is the shared public projector for Chuck's own tool
// calls: action and outcome only. Their raw arguments and results can carry
// turn tokens, read receipts, repeated peer bodies or private mail, context
// and task payloads, none of which may enter room rows, REST or SSE (R10).
func (s *Server) projectThinkTankData(c *thinkTankCapture, ev runtime.Event) any {
	switch ev.Type {
	case runtime.EvToolCall:
		var d runtime.ToolCallData
		if json.Unmarshal(ev.Data, &d) == nil && s.isChuckTool(d.Name) {
			c.internal[d.ToolCallID] = true
			return runtime.ToolCallData{ToolCallID: d.ToolCallID, Name: d.Name, Title: chuckToolReceipt(d.Name),
				Args: json.RawMessage(`{}`), Status: d.Status}
		}
	case runtime.EvToolResult:
		var d runtime.ToolResultData
		if json.Unmarshal(ev.Data, &d) == nil && c.internal[d.ToolCallID] {
			return runtime.ToolResultData{ToolCallID: d.ToolCallID, Status: d.Status, Content: json.RawMessage(`null`)}
		}
	case runtime.EvPermissionRequest:
		var d runtime.PermissionRequestData
		if json.Unmarshal(ev.Data, &d) == nil && s.isChuckTool(d.Name) {
			d.Args, d.Reason = json.RawMessage(`{}`), ""
			return d
		}
	}
	return ev.Data
}

func chuckToolReceipt(name string) string {
	switch {
	case strings.HasSuffix(name, "read_think_tank"):
		return "Read the room"
	case strings.HasSuffix(name, "submit_think_tank_turn"):
		return "Submitted a room turn"
	default:
		short := name
		if i := strings.LastIndex(name, "__"); i >= 0 {
			short = name[i+2:]
		}
		return "Used Chuck " + strings.ReplaceAll(short, "_", " ")
	}
}

type thinkTankActivityWire struct {
	Version   int             `json:"version"`
	RoomID    string          `json:"room_id"`
	Seq       int64           `json:"seq"`
	AttemptID string          `json:"attempt_id"`
	AgentID   string          `json:"agent_id"`
	AgentName string          `json:"agent_name"`
	Project   string          `json:"project"`
	SourceSeq int64           `json:"source_seq"`
	Truncated bool            `json:"truncated,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
	CreatedAt string          `json:"created_at"`
}

func thinkTankActivityFor(a state.ThinkTankActivity) thinkTankActivityWire {
	return thinkTankActivityWire{Version: thinkTankWireVersion, RoomID: a.RoomID, Seq: a.Seq, AttemptID: a.AttemptID,
		AgentID: a.AgentID, AgentName: a.AgentName, Project: a.Project, SourceSeq: a.SourceSeq,
		Truncated: a.Truncated, Event: json.RawMessage(a.Payload), CreatedAt: a.CreatedAt.Format("2006-01-02T15:04:05Z07:00")}
}

// thinkTankActivityLive omits an oversized event body; the client refetches
// that record over REST.
func thinkTankActivityLive(a state.ThinkTankActivity) thinkTankActivityWire {
	w := thinkTankActivityFor(a)
	if len(a.Payload) > thinkTankLiveActivityLimit {
		w.Event = nil
	}
	return w
}
