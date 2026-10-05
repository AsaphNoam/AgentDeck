package server

import "github.com/AsaphNoam/Chuck/internal/state"

// thinkTankWireVersion versions every Think Tank REST/SSE shape (TS-14.R16).
const thinkTankWireVersion = 1

// thinkTankUpdate is the versioned think_tank_update summary, emitted after a
// durable commit. It is a notification, not history: a client that sees a
// revision gap refetches the room (TS-14.R16).
type thinkTankUpdate struct {
	Version      int     `json:"version"`
	RoomID       string  `json:"room_id"`
	Revision     int64   `json:"revision"`
	Phase        string  `json:"phase"`
	Control      string  `json:"control"`
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
