package server

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/AsaphNoam/Chuck/internal/runtime"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// The Think Tank engine progresses rooms over the ordinary lifecycle and
// runtime services (TS-14.R1–R3). Rooms are durable in state; this loop only
// selects each room's one admissible opportunity and starts it as a guarded
// think_tank activation. Each room progresses on its own bounded goroutine, so
// a slow launch or resume in one room does not delay another (INV §16), and a
// per-room claim keeps a room from racing itself to admit two turns; the
// state's revision check and one-running index are the authority either way.

const (
	thinkTankSweepInterval = 5 * time.Second
	thinkTankProgressBatch = 4
)

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
			s.dispatchThinkTanks(ctx)
		}
	}()
	return nil
}

// progressThinkTanks runs one dispatch pass and waits for it.
func (s *Server) progressThinkTanks(ctx context.Context) {
	s.dispatchThinkTanks(ctx).Wait()
}

// dispatchThinkTanks starts progression for each active room not already in
// flight, at most thinkTankProgressBatch at once. A room skipped because it
// is in flight or no slot is free is retried when a slot is released.
func (s *Server) dispatchThinkTanks(ctx context.Context) *sync.WaitGroup {
	var wg sync.WaitGroup
	ids, err := s.stateStore.ListActiveThinkTankRooms()
	if err != nil {
		s.log.Debug("list think tanks", "err", err)
		return &wg
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		if !s.claimThinkTankRoom(id) {
			continue
		}
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			defer s.releaseThinkTankRoom(id)
			s.progressThinkTank(ctx, id)
		}(id)
	}
	return &wg
}

func (s *Server) claimThinkTankRoom(roomID string) bool {
	s.thinkTankRoomMu.Lock()
	defer s.thinkTankRoomMu.Unlock()
	if _, busy := s.thinkTankRooms[roomID]; busy {
		s.thinkTankMissed = true
		return false
	}
	select {
	case s.thinkTankSlots <- struct{}{}:
	default:
		s.thinkTankMissed = true
		return false
	}
	s.thinkTankRooms[roomID] = struct{}{}
	return true
}

func (s *Server) releaseThinkTankRoom(roomID string) {
	s.thinkTankRoomMu.Lock()
	delete(s.thinkTankRooms, roomID)
	<-s.thinkTankSlots
	missed := s.thinkTankMissed
	s.thinkTankMissed = false
	s.thinkTankRoomMu.Unlock()
	if missed {
		s.kickThinkTanks()
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
		return "A participant's identity cannot be read. Retry after restoring it, or end the discussion."
	}
	switch {
	case agent.Archived:
		return agent.Name + " is archived. Restore it and resume, or end the discussion."
	case agent.Interface != "chat":
		return agent.Name + " is no longer a chat agent. Switch it back and resume, or end the discussion."
	}
	project, err := s.configStore.ReadProject(agent.Project)
	if err != nil {
		return agent.Name + "'s project is missing or unreadable. Restore it and resume, or end the discussion."
	}
	if project.Archived {
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
		if reason := s.thinkTankIneligible(agentID); reason != "" {
			if held, err := s.stateStore.SetThinkTankHold(d.Room.RoomID, reason); err == nil {
				s.publishThinkTankUpdate(held)
			}
			return errors.New(reason)
		}
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
		agent, err := s.stateStore.ReadAgent(agentID)
		if err != nil {
			return false
		}
		if ae := s.acquireAgentStart(agent.Project, agentID); ae != nil {
			return false
		}
		defer s.releaseAgentStart(agent.Project, agentID)
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
	// A stopped speaker that cannot be resumed holds the room with a reason
	// instead of being retried by every sweep (FS-21.R37, TS-14.R6). A lost
	// resume race or an archival in progress is transient and left to the next
	// sweep, where eligibility names the lasting reason.
	who := member.AgentName
	if next.Turn == state.ThinkTankTurnJudge {
		who = "The judge"
	} else if who == "" {
		who = "A participant"
	}
	hold := func(reason string) {
		if held, err := s.stateStore.SetThinkTankHold(d.Room.RoomID, reason); err == nil {
			s.publishThinkTankUpdate(held)
		}
	}
	if _, ok, ae := s.wakeCandidate(agentID); ae != nil {
		hold(who + " could not start: " + ae.Message + ". Resume it manually, or end the discussion.")
		return false
	} else if !ok {
		if _, err := s.stateStore.ReadRunning(agentID); errors.Is(err, state.ErrNotFound) {
			hold(who + " has no saved session to resume. Resume it manually, or end the discussion.")
		}
		return false
	}
	started := false
	ae := s.resumeSessionWithHooks(ctx, agentID, resumeOverride{}, nil, func() error {
		var err error
		started, err = s.registry.StartActivation(ctx, agentID, state.ActivationKindThinkTank, begin)
		// Not admitted — the gate was busy or the room moved during the
		// resume — leaves the resumed agent running idle; the next sweep takes
		// the running branch instead of stopping it.
		if !started && begun == nil && (err == nil || errors.Is(err, state.ErrThinkTankConflict)) {
			return nil
		}
		return err
	})
	if ae != nil {
		s.log.Debug("resume for think tank turn", "room", d.Room.RoomID, "agent", agentID, "err", ae.Message)
		switch {
		case started:
		case begun != nil:
			abandon(errors.New(ae.Message))
		case ae.Code != runtime.CodeConflict && ae.Code != runtime.CodeAgentArchiving && ae.Code != runtime.CodeProjectArchiving:
			if s.thinkTankIneligible(agentID) == "" {
				hold(who + " could not start: " + ae.Message + ". Resume it manually, or end the discussion.")
			}
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
		if _, err := s.stateStore.ClaimThinkTankMemberSetup(d.Room.RoomID, m.AgentID); err != nil {
			return
		}
		name, launchErr := s.launchReservedThinkTankAgent(ctx, m.AgentID, m.SetupConfig, d.Room.Title)
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

// Requested Stop suppresses turn_end. Settle only attempts owned by the exiting
// generation, releasing capture and allowance-free intervention on every exit.
func (s *Server) interruptThinkTankOnExit(agentID, generation, cause string) {
	attempts, err := s.stateStore.RunningThinkTankAttempts(agentID, generation)
	if err != nil {
		s.log.Warn("read exiting think tank attempts", "agent", agentID, "err", err)
		return
	}
	for _, a := range attempts {
		s.endThinkTankCapture(agentID, generation, a.TurnID)
		f, err := s.stateStore.FailThinkTankAttempt(agentID, generation, a.TurnID, "The agent stopped ("+cause+"). Retry explicitly, or end the discussion.")
		if err == nil {
			s.publishThinkTankUpdate(f.Detail)
		} else if !errors.Is(err, state.ErrNotFound) {
			s.log.Warn("settle exiting think tank attempt", "agent", agentID, "err", err)
		}
	}
	s.kickThinkTanks()
}

// launchReservedThinkTankAgent launches a reserved room identity into the
// group named by the room title. An identity that already exists is never
// relaunched or regrouped (FS-21.R51, TS-14.R22).
func (s *Server) launchReservedThinkTankAgent(ctx context.Context, agentID, config, title string) (string, string) {
	if agent, err := s.stateStore.ReadAgent(agentID); err == nil {
		return agent.Name, ""
	}
	var req launchRequest
	if err := json.Unmarshal([]byte(config), &req); err != nil {
		return "", "the saved launch settings are unreadable"
	}
	req.Group = title
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
	name, launchErr := s.launchReservedThinkTankAgent(ctx, agentID, d.Room.JudgeConfig, d.Room.Title)
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
		finish, err = s.stateStore.FinalizeThinkTankAttemptAt(ev.AgentID, ev.Generation, ev.TurnID, ev.Seq)
	} else {
		finish, err = s.stateStore.FailThinkTankAttempt(ev.AgentID, ev.Generation, ev.TurnID, thinkTankStopText(td.StopReason))
	}
	if errors.Is(err, state.ErrNotFound) {
		return
	}
	if err != nil {
		// A store failure leaves the attempt running, which holds the room; it
		// is never published from assistant output instead (R6, R10). Say so in
		// the room: stopping the speaker fences the attempt.
		s.log.Warn("finish think tank turn", "agent", ev.AgentID, "err", err)
		attempts, _ := s.stateStore.RunningThinkTankAttempts(ev.AgentID, ev.Generation)
		for _, a := range attempts {
			if a.TurnID == ev.TurnID {
				if held, err := s.stateStore.SetThinkTankHold(a.RoomID, "The turn's result could not be saved. Stop the speaker to recover, then resume or end the discussion."); err == nil {
					s.publishThinkTankUpdate(held)
				}
			}
		}
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
