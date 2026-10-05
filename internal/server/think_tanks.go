package server

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/AsaphNoam/Chuck/internal/runtime"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// The Think Tank engine progresses rooms over the ordinary lifecycle and
// runtime services (TS-14.R1–R3). Rooms are durable in state; this loop only
// selects each room's one admissible opportunity and starts it as a guarded
// think_tank activation. One worker serializes progression, so a room never
// races itself to admit two turns; the state's revision check and one-running
// index are the authority either way.

const thinkTankSweepInterval = 5 * time.Second

func (s *Server) kickThinkTanks() {
	select {
	case s.thinkTankKick <- struct{}{}:
	default:
	}
}

// startThinkTanks fences attempts uncertain across a restart and holds
// unfinished rooms before the loop starts, so nothing is replayed (R14).
func (s *Server) startThinkTanks(ctx context.Context) error {
	if err := s.stateStore.RecoverThinkTanks(); err != nil {
		return err
	}
	go func() {
		ticker := time.NewTicker(thinkTankSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-s.thinkTankKick:
			}
			s.progressThinkTanks(ctx)
		}
	}()
	return nil
}

func (s *Server) progressThinkTanks(ctx context.Context) {
	ids, err := s.stateStore.ListActiveThinkTankRooms()
	if err != nil {
		s.log.Debug("list think tanks", "err", err)
		return
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		s.progressThinkTank(ctx, id)
	}
}

func (s *Server) progressThinkTank(ctx context.Context, roomID string) {
	d, err := s.stateStore.ReadThinkTank(roomID)
	if err != nil {
		if !errors.Is(err, state.ErrNotFound) {
			s.log.Debug("read think tank", "room", roomID, "err", err)
		}
		return
	}
	r := d.Room
	if r.Phase == state.ThinkTankPhaseSetup && r.Hold == "" && r.Control == state.ThinkTankRunning {
		s.launchThinkTankSetup(ctx, d)
		return
	}
	if r.Phase == state.ThinkTankPhaseEnded && r.JudgeStatus == state.ThinkTankJudgeReady {
		s.launchThinkTankJudge(ctx, d)
		return
	}
	next, ok := state.NextThinkTankOpportunity(d)
	if !ok {
		return
	}
	if reason := s.thinkTankIneligible(next.AgentID); reason != "" {
		if next.Turn == state.ThinkTankTurnJudge {
			reason = "The judge cannot start: " + reason
		}
		if held, err := s.stateStore.SetThinkTankHold(roomID, reason); err == nil {
			s.publishThinkTankUpdate(held)
		}
		return
	}
	// Assigned work holds the same speaker rather than racing or skipping it
	// (FS-21.R28, R34); private busy turns are refused by StartActivation.
	if _, err := s.stateStore.AssignedTask(next.AgentID); err == nil {
		return
	}
	if s.startThinkTankTurn(ctx, d, next) {
		if fresh, err := s.stateStore.ReadThinkTank(roomID); err == nil {
			s.publishThinkTankUpdate(fresh)
		}
	}
}

// thinkTankIneligible names why a participant cannot take a room turn now,
// or "" when it can. Deleted identities are never substituted (FS-21.R37).
func (s *Server) thinkTankIneligible(agentID string) string {
	agent, err := s.stateStore.ReadAgent(agentID)
	if errors.Is(err, state.ErrNotFound) {
		return "a participant was deleted. End the discussion to keep its history."
	}
	if err != nil {
		return ""
	}
	switch {
	case agent.Archived:
		return agent.Name + " is archived. Restore it and resume, or end the discussion."
	case agent.Interface != "chat":
		return agent.Name + " is no longer a chat agent. Switch it back and resume, or end the discussion."
	}
	if ae := s.projectArchiveGate(agent.Project, "project is archived"); ae != nil && ae.Code != runtime.CodeInternal {
		return agent.Name + "'s project is archived. Unarchive it and resume, or end the discussion."
	}
	return ""
}

// startThinkTankTurn starts the selected opportunity as one guarded
// activation. The room attempt commits inside the runtime's before callback
// with the actual generation and executing turn id, before the provider frame
// (TS-14.R3). A busy agent reports not-started and keeps the speaker.
func (s *Server) startThinkTankTurn(ctx context.Context, d state.ThinkTankDetail, next state.ThinkTankOpportunity) bool {
	agentID := next.AgentID
	var member state.ThinkTankMember
	for _, m := range d.Members {
		if m.AgentID == agentID {
			member = m
		}
	}
	var begun *state.ThinkTankAttempt
	begin := func(turnID string) error {
		a, err := s.stateStore.BeginThinkTankAttempt(state.ThinkTankBegin{
			RoomID: d.Room.RoomID, Revision: d.Room.Revision, AgentID: agentID, Turn: next.Turn,
			Generation: s.registry.Generation(agentID), TurnID: turnID,
		})
		if err == nil {
			begun = &a
			s.beginThinkTankCapture(agentID, a, member.AgentName, member.Project)
		}
		return err
	}
	// An attempt committed in before() whose provider frame then never went
	// out has no turn end to settle it; fail it now so the room shows the
	// intervention instead of a turn that never finishes (TS-14.R6).
	abandon := func(cause error) {
		if begun == nil {
			return
		}
		s.endThinkTankCapture(agentID, begun.Generation, begun.TurnID)
		reason := "The room turn could not start."
		if cause != nil {
			reason = "The room turn could not start: " + cause.Error()
		}
		if f, err := s.stateStore.FailThinkTankAttempt(agentID, begun.Generation, begun.TurnID, reason); err == nil {
			s.publishThinkTankUpdate(f.Detail)
		}
	}
	if _, err := s.stateStore.ReadRunning(agentID); err == nil {
		if !s.claimLifecycle(agentID) {
			return false
		}
		defer s.releaseLifecycle(agentID)
		started, err := s.registry.StartActivation(ctx, agentID, state.ActivationKindThinkTank, begin)
		if err != nil {
			s.log.Debug("start think tank turn", "room", d.Room.RoomID, "agent", agentID, "err", err)
		}
		if !started {
			abandon(err)
		}
		return started
	} else if !errors.Is(err, state.ErrNotFound) {
		return false
	}
	if _, ok, ae := s.wakeCandidate(agentID); ae != nil || !ok {
		return false
	}
	started := false
	ae := s.resumeSessionWithHooks(ctx, agentID, resumeOverride{}, func() error { return nil }, func() error {
		var err error
		started, err = s.registry.StartActivation(ctx, agentID, state.ActivationKindThinkTank, begin)
		if err == nil && !started {
			err = errors.New("think tank turn did not start")
		}
		return err
	})
	if ae != nil {
		s.log.Debug("resume for think tank turn", "room", d.Room.RoomID, "agent", agentID, "err", ae.Message)
		if !started {
			abandon(errors.New(ae.Message))
		}
	}
	return started
}

// launchThinkTankSetup launches reserved new participants one slot at a
// time. A slot whose agent already exists finished launching before an
// interruption and is marked ready rather than duplicated (TS-14.R2).
func (s *Server) launchThinkTankSetup(ctx context.Context, d state.ThinkTankDetail) {
	for _, m := range d.Members {
		if m.Role != state.ThinkTankRoleParticipant || m.SetupState != state.ThinkTankSetupPending {
			continue
		}
		name, launchErr := s.launchReservedThinkTankAgent(ctx, m.AgentID, m.SetupConfig)
		updated, err := s.stateStore.MarkThinkTankMemberSetup(d.Room.RoomID, m.AgentID, name, launchErr)
		if err != nil {
			s.log.Debug("mark think tank setup", "room", d.Room.RoomID, "agent", m.AgentID, "err", err)
			return
		}
		s.publishThinkTankUpdate(updated)
		if launchErr != "" {
			return
		}
	}
	s.kickThinkTanks()
}

func (s *Server) launchReservedThinkTankAgent(ctx context.Context, agentID, config string) (string, string) {
	if agent, err := s.stateStore.ReadAgent(agentID); err == nil {
		return agent.Name, ""
	}
	var req launchRequest
	if err := json.Unmarshal([]byte(config), &req); err != nil {
		return "", "the saved launch settings are unreadable"
	}
	resp, ae := s.launchAgent(ctx, req, launchOptions{AgentID: agentID})
	if ae != nil {
		return "", ae.Message
	}
	return resp.Agent.Name, ""
}

// launchThinkTankJudge launches the configured fresh judge after discussion
// ends. Its identity is reserved durably before the launch effect (R13).
func (s *Server) launchThinkTankJudge(ctx context.Context, d state.ThinkTankDetail) {
	var req launchRequest
	if err := json.Unmarshal([]byte(d.Room.JudgeConfig), &req); err != nil {
		s.log.Debug("think tank judge config", "room", d.Room.RoomID, "err", err)
		return
	}
	agentID, err := s.stateStore.NewAgentID()
	if err != nil {
		s.log.Debug("reserve think tank judge id", "err", err)
		return
	}
	reserved, err := s.stateStore.ReserveThinkTankJudge(d.Room.RoomID, agentID, req.Name, req.Project)
	if err != nil {
		s.log.Debug("reserve think tank judge", "room", d.Room.RoomID, "err", err)
		return
	}
	s.publishThinkTankUpdate(reserved)
	name, launchErr := s.launchReservedThinkTankAgent(ctx, agentID, d.Room.JudgeConfig)
	updated, err := s.stateStore.MarkThinkTankJudgeLaunched(d.Room.RoomID, name, launchErr)
	if err != nil {
		s.log.Debug("mark think tank judge", "room", d.Room.RoomID, "err", err)
		return
	}
	s.publishThinkTankUpdate(updated)
	s.kickThinkTanks()
}

// finishThinkTankTurn settles the room attempt owned by exactly this event's
// generation and executing turn id (TS-14.R6). Any other turn end may free a
// waiting speaker, so it only nudges the loop.
func (s *Server) finishThinkTankTurn(ev runtime.Event) {
	defer s.kickThinkTanks()
	if ev.TurnID == "" {
		return
	}
	var td runtime.TurnEndData
	if err := json.Unmarshal(ev.Data, &td); err != nil {
		td.StopReason = "error"
	}
	var finish state.ThinkTankFinish
	var err error
	if s.endThinkTankCapture(ev.AgentID, ev.Generation, ev.TurnID) {
		// Storage failure prevents final publication (TS-14.R10).
		finish, err = s.stateStore.FailThinkTankAttempt(ev.AgentID, ev.Generation, ev.TurnID,
			"Room activity for this turn could not be saved.")
	} else if td.StopReason == "end_turn" {
		finish, err = s.stateStore.FinalizeThinkTankAttempt(ev.AgentID, ev.Generation, ev.TurnID)
	} else {
		finish, err = s.stateStore.FailThinkTankAttempt(ev.AgentID, ev.Generation, ev.TurnID, thinkTankStopText(td.StopReason))
	}
	if errors.Is(err, state.ErrNotFound) {
		return
	}
	if err != nil {
		// A store failure leaves the attempt running, which holds the room; it
		// is never published from assistant output instead (R6, R10).
		s.log.Warn("finish think tank turn", "agent", ev.AgentID, "err", err)
		return
	}
	s.publishThinkTankUpdate(finish.Detail)
}

func thinkTankStopText(reason string) string {
	switch reason {
	case "cancelled":
		return "The turn was cancelled."
	case "max_tokens":
		return "The turn was cut off at the provider's output limit."
	default:
		return "The provider reported an error."
	}
}
