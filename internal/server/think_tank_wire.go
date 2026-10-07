package server

import (
	"encoding/json"
	"time"

	"github.com/AsaphNoam/Chuck/internal/state"
)

// thinkTankWireVersion versions every Think Tank REST/SSE shape (TS-14.R16).
const thinkTankWireVersion = 1

// thinkTankUpdate is the versioned think_tank_update summary, emitted after a
// durable commit. It is a notification, not history: a client that sees a
// revision gap refetches the room (TS-14.R16).
type thinkTankUpdate struct {
	Version      int     `json:"version"`
	RoomID       string  `json:"room_id"`
	Revision     int64   `json:"revision"`
	Phase        string  `json:"phase,omitempty"`
	Control      string  `json:"control,omitempty"`
	Hold         string  `json:"hold,omitempty"`
	EndReason    string  `json:"end_reason,omitempty"`
	JudgeStatus  string  `json:"judge_status,omitempty"`
	CurrentActor *string `json:"current_actor,omitempty"`
	Deleted      bool    `json:"deleted,omitempty"`
}

func thinkTankUpdateFor(d state.ThinkTankDetail) thinkTankUpdate {
	u := thinkTankUpdate{
		Version: thinkTankWireVersion, RoomID: d.Room.RoomID, Revision: d.Room.Revision,
		Phase: d.Room.Phase, Control: d.Room.Control, Hold: d.Room.Hold, EndReason: d.Room.EndReason,
		JudgeStatus: d.Room.JudgeStatus,
	}
	if d.Active != nil {
		actor := d.Active.AgentID
		u.CurrentActor = &actor
	}
	return u
}

func (s *Server) publishThinkTankUpdate(d state.ThinkTankDetail) {
	s.eventBus.Publish("think_tank_update", nil, thinkTankUpdateFor(d))
}

type thinkTankSummaryWire struct {
	Version       int        `json:"version"`
	RoomID        string     `json:"room_id"`
	Title         string     `json:"title"`
	Goal          string     `json:"goal"`
	OriginProject string     `json:"origin_project"`
	Phase         string     `json:"phase"`
	Control       string     `json:"control"`
	Hold          string     `json:"hold,omitempty"`
	EndReason     string     `json:"end_reason,omitempty"`
	JudgeStatus   string     `json:"judge_status,omitempty"`
	Participants  []string   `json:"participants"`
	Revision      int64      `json:"revision"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	// ActiveAgentID names the agent taking the room's running turn, so that
	// agent's own conversation can identify the room turn (FS-03.R69).
	ActiveAgentID string `json:"active_agent_id,omitempty"`
}

func thinkTankSummaryFor(d state.ThinkTankDetail) thinkTankSummaryWire {
	r := d.Room
	names := []string{}
	for _, m := range d.Members {
		if m.Role == state.ThinkTankRoleParticipant {
			names = append(names, m.AgentName)
		}
	}
	out := thinkTankSummaryWire{
		Version: thinkTankWireVersion, RoomID: r.RoomID, Title: r.Title, Goal: r.Goal, OriginProject: r.OriginProject,
		Phase: r.Phase, Control: r.Control, Hold: r.Hold, EndReason: r.EndReason, JudgeStatus: r.JudgeStatus,
		Participants: names, Revision: r.Revision, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, EndedAt: r.EndedAt,
	}
	if d.Active != nil {
		out.ActiveAgentID = d.Active.AgentID
	}
	return out
}

type thinkTankMemberWire struct {
	AgentID    string `json:"agent_id"`
	Name       string `json:"name"`
	Project    string `json:"project"`
	Role       string `json:"role"`
	Order      int    `json:"order"`
	Limit      int    `json:"limit"`
	Completed  int    `json:"completed"`
	MayLeave   bool   `json:"may_leave"`
	State      string `json:"state"`
	SetupState string `json:"setup_state"`
	SetupError string `json:"setup_error,omitempty"`
	// Live reports the normal agent behind this retained identity; a deleted
	// agent keeps its attribution without an active-chat link (FS-21.R25).
	Exists      bool   `json:"exists"`
	Archived    bool   `json:"archived,omitempty"`
	Running     bool   `json:"running"`
	AgentStatus string `json:"agent_status,omitempty"`
}

type thinkTankAttemptWire struct {
	AttemptID string    `json:"attempt_id"`
	AgentID   string    `json:"agent_id"`
	Turn      string    `json:"turn"`
	State     string    `json:"state"`
	Failure   string    `json:"failure,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

type thinkTankNextWire struct {
	AgentID string `json:"agent_id"`
	Turn    string `json:"turn"`
	// Waiting names why the selected speaker has not started yet.
	Waiting string `json:"waiting,omitempty"`
}

type thinkTankInputWire struct {
	InputID   string          `json:"input_id"`
	Kind      string          `json:"kind"`
	Body      string          `json:"body"`
	Context   json.RawMessage `json:"context,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type thinkTankJudgeWire struct {
	Enabled bool            `json:"enabled"`
	Status  string          `json:"status,omitempty"`
	AgentID string          `json:"agent_id,omitempty"`
	Error   string          `json:"error,omitempty"`
	Config  json.RawMessage `json:"config,omitempty"`
}

type thinkTankDetailWire struct {
	thinkTankSummaryWire
	Openings  bool                   `json:"openings"`
	Members   []thinkTankMemberWire  `json:"members"`
	Active    *thinkTankAttemptWire  `json:"active,omitempty"`
	Next      *thinkTankNextWire     `json:"next,omitempty"`
	Pending   []thinkTankInputWire   `json:"pending"`
	Failed    []thinkTankAttemptWire `json:"failed"`
	Judge     thinkTankJudgeWire     `json:"judge"`
	Deletable bool                   `json:"deletable"`
}

type thinkTankEntryWire struct {
	Seq         int64           `json:"seq"`
	Kind        string          `json:"kind"`
	AgentID     string          `json:"agent_id,omitempty"`
	AgentName   string          `json:"agent_name,omitempty"`
	Project     string          `json:"project,omitempty"`
	Body        string          `json:"body"`
	AttemptID   string          `json:"attempt_id,omitempty"`
	InputID     string          `json:"input_id,omitempty"`
	Context     json.RawMessage `json:"context,omitempty"`
	Undiscussed bool            `json:"undiscussed,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

func rawJSONOrNil(s string) json.RawMessage {
	if s == "" || !json.Valid([]byte(s)) {
		return nil
	}
	return json.RawMessage(s)
}

func thinkTankEntryFor(e state.ThinkTankEntry) thinkTankEntryWire {
	return thinkTankEntryWire{
		Seq: e.Seq, Kind: e.Kind, AgentID: e.AgentID, AgentName: e.AgentName, Project: e.Project,
		Body: e.Body, AttemptID: e.AttemptID, InputID: e.InputID, Context: rawJSONOrNil(e.Context),
		Undiscussed: e.Undiscussed, CreatedAt: e.CreatedAt,
	}
}

func thinkTankAttemptFor(a state.ThinkTankAttempt) thinkTankAttemptWire {
	return thinkTankAttemptWire{AttemptID: a.AttemptID, AgentID: a.AgentID, Turn: a.Turn, State: a.State,
		Failure: a.Failure, StartedAt: a.CreatedAt}
}

// thinkTankDetailWire joins durable room state with each member's live agent
// state, so the room can show the current speaker or the reason it waits.
func (s *Server) thinkTankDetailWire(d state.ThinkTankDetail) thinkTankDetailWire {
	r := d.Room
	out := thinkTankDetailWire{
		thinkTankSummaryWire: thinkTankSummaryFor(d), Openings: r.Openings,
		Members: []thinkTankMemberWire{}, Pending: []thinkTankInputWire{}, Failed: []thinkTankAttemptWire{},
		Judge: thinkTankJudgeWire{Enabled: r.JudgeConfig != "", Status: r.JudgeStatus, AgentID: r.JudgeAgentID,
			Error: r.JudgeError, Config: rawJSONOrNil(r.JudgeConfig)},
	}
	statuses := map[string]string{}
	for _, m := range d.Members {
		w := thinkTankMemberWire{AgentID: m.AgentID, Name: m.AgentName, Project: m.Project, Role: m.Role,
			Order: m.Order, Limit: m.Cap, Completed: m.Completed, MayLeave: m.MayLeave, State: m.State,
			SetupState: m.SetupState, SetupError: m.SetupError}
		if agent, err := s.stateStore.ReadAgent(m.AgentID); err == nil {
			w.Exists, w.Archived, w.Name = true, agent.Archived, agent.Name
			if _, err := s.stateStore.ReadRunning(m.AgentID); err == nil {
				w.Running = true
				if st, err := s.stateStore.ReadStatus(m.AgentID); err == nil {
					w.AgentStatus = st.State
					statuses[m.AgentID] = st.State
				}
			}
		}
		out.Members = append(out.Members, w)
	}
	if d.Active != nil {
		a := thinkTankAttemptFor(*d.Active)
		out.Active = &a
	}
	for _, a := range d.Attempts {
		if a.State == state.ThinkTankAttemptFailed {
			out.Failed = append(out.Failed, thinkTankAttemptFor(a))
		}
	}
	for _, in := range d.Pending {
		out.Pending = append(out.Pending, thinkTankInputWire{InputID: in.InputID, Kind: in.Kind, Body: in.Body,
			Context: rawJSONOrNil(in.Context), CreatedAt: in.CreatedAt})
	}
	if next, ok := state.NextThinkTankOpportunity(d); ok {
		n := &thinkTankNextWire{AgentID: next.AgentID, Turn: next.Turn}
		name := next.AgentID
		for _, m := range out.Members {
			if m.AgentID == next.AgentID {
				name = m.Name
			}
		}
		if _, err := s.stateStore.AssignedTask(next.AgentID); err == nil {
			n.Waiting = "Waiting for " + name + " to finish assigned work"
		} else if st := statuses[next.AgentID]; st != "" && st != "idle" {
			n.Waiting = "Waiting for " + name + " to finish private work"
		}
		out.Next = n
	}
	out.Deletable = d.Active == nil && r.JudgeStatus != state.ThinkTankJudgeLaunching &&
		(r.Phase == state.ThinkTankPhaseEnded || r.Control == state.ThinkTankPaused)
	return out
}
