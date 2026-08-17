package server

import (
	"context"
	"errors"
	"time"

	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

type nudgeState struct {
	lastNudgeAt time.Time
	inFlight    bool
	startedAt   time.Time
}

func (s *Server) startMessagingLoops(ctx context.Context) {
	go s.runNudger(ctx)
	go s.runMessageJanitor(ctx)
}

func (s *Server) runNudger(ctx context.Context) {
	ticker := time.NewTicker(messaging.NudgeInterval)
	defer ticker.Stop()
	stateByAgent := map[string]nudgeState{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.nudgeOnce(ctx, "", stateByAgent)
		case agentID := <-s.nudgeCh:
			s.nudgeOnce(ctx, agentID, stateByAgent)
		}
	}
}

func (s *Server) nudgeOnce(ctx context.Context, onlyAgentID string, stateByAgent map[string]nudgeState) {
	if s.registry == nil {
		return
	}
	running, err := s.stateStore.ListRunning()
	if err != nil {
		s.log.Debug("nudger list running failed", "err", err)
		return
	}
	now := time.Now()
	for _, row := range running {
		if onlyAgentID != "" && row.AgentID != onlyAgentID {
			continue
		}
		ns := stateByAgent[row.AgentID]
		if ns.inFlight && now.Sub(ns.startedAt) > messaging.NudgeInFlightTimeout {
			ns.inFlight = false
		}
		status, err := s.stateStore.ReadStatus(row.AgentID)
		if err != nil {
			stateByAgent[row.AgentID] = ns
			continue
		}
		unread, err := s.stateStore.UnreadCount(row.AgentID)
		if err != nil {
			s.log.Debug("nudger unread count failed", "agent", row.AgentID, "err", err)
			stateByAgent[row.AgentID] = ns
			continue
		}
		if ns.inFlight && status.State == "idle" && unread == 0 {
			ns.inFlight = false
		}
		if status.State != "idle" || unread == 0 || ns.inFlight || now.Sub(ns.lastNudgeAt) < messaging.NudgeCooldown {
			stateByAgent[row.AgentID] = ns
			continue
		}
		ns.inFlight = true
		ns.startedAt = now
		ns.lastNudgeAt = now
		stateByAgent[row.AgentID] = ns
		if _, err := s.stateStore.MarkUnreadDeliveredVia(row.AgentID, state.DeliveryNudge); err != nil {
			s.log.Debug("nudger mark delivered failed", "agent", row.AgentID, "err", err)
		}
		go func(agentID string, pid int) {
			if err := s.registry.CheckMessages(ctx, pid); err != nil && !errors.Is(err, runtime.ErrNoHandle) {
				s.log.Debug("nudger check_messages failed", "agent", agentID, "err", err)
			}
		}(row.AgentID, row.PID)
	}
	s.wakeOnce(ctx, onlyAgentID, stateByAgent, now)
}

// wakeOnce resumes the stopped agents holding mail nobody has delivered yet
// (FS-06.R23). Candidacy is durable: it needs an unread row still marked
// "pending", and an attempt claims exactly the rows it is waking for out of
// "pending" before it spawns anything. That ordering is what bounds retries. If
// the attempt were only marked on failure, an adapter that completed its
// handshake and then died before the first check_messages nudge would leave the
// rows pending and be respawned by every two-second sweep — the process-spawn
// loop R23 forbids. Woken agents are nudged by the ordinary running path above
// once they report idle, and that nudge restamps the claimed rows "nudge".
func (s *Server) wakeOnce(ctx context.Context, onlyAgentID string, stateByAgent map[string]nudgeState, now time.Time) {
	waiting, err := s.stateStore.PendingWakeMailAgents()
	if err != nil {
		s.log.Debug("nudger pending wake mail failed", "err", err)
		return
	}
	for _, agentID := range waiting {
		if onlyAgentID != "" && agentID != onlyAgentID {
			continue
		}
		if _, ok, ae := s.wakeCandidate(agentID); ae != nil {
			// A gate could not be evaluated (storage or project-definition failure).
			// Leave the mail pending and retry on the next sweep rather than
			// consuming this recipient's one wake attempt on an undecided gate.
			s.log.Debug("mail wake candidacy failed", "agent", agentID, "err", ae.Message)
			continue
		} else if !ok {
			continue // running, archived, snapshot-less, terminal, or pipeline-owned
		}
		ns := stateByAgent[agentID]
		// The exclusive lifecycle claim is the re-entry guard: this loop must not
		// wait out a wake, and the nudge in-flight marker belongs to the running
		// path that delivers check_messages once the agent is back.
		if s.lifecycleInFlight(agentID) || now.Sub(ns.lastNudgeAt) < messaging.NudgeCooldown {
			continue
		}
		ns.lastNudgeAt = now
		stateByAgent[agentID] = ns
		go s.wakeForMail(ctx, agentID)
	}
}

// wakeForMail runs one bounded wake attempt for a stopped recipient. It claims
// the pending mail the attempt owns before waking, so the durable outcome is tied
// to the attempt rather than to whatever happened to be pending when it finished:
// mail that arrives while the wake is in flight stays "pending" and re-arms the
// wake, exactly as FS-06.R23 promises.
func (s *Server) wakeForMail(ctx context.Context, agentID string) {
	claimed, err := s.stateStore.ClaimPendingWakeMail(agentID)
	if err != nil {
		s.log.Debug("claim wake mail failed", "agent", agentID, "err", err)
		return
	}
	if len(claimed) == 0 {
		return // another attempt already owns this recipient's mail
	}
	_, ae := s.wakeAgent(ctx, agentID)
	if ae == nil {
		// The claim stands until the check_messages nudge that follows the wake
		// restamps these rows "nudge". If the woken adapter dies first they remain
		// claimed, which is precisely the no-respawn bound.
		return
	}
	s.log.Debug("mail wake failed", "agent", agentID, "err", ae.Message)
	outcome := state.DeliveryWakeFailed
	if ae.Code == runtime.CodeConflict {
		// Another lifecycle transition owns this agent; nothing was attempted here,
		// so release the claim and let the next sweep try again.
		outcome = state.DeliveryPending
	}
	if err := s.stateStore.SetDeliveredVia(claimed, outcome); err != nil {
		s.log.Debug("record wake outcome failed", "agent", agentID, "outcome", outcome, "err", err)
	}
}

func (s *Server) runMessageJanitor(ctx context.Context) {
	ticker := time.NewTicker(messaging.JanitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			readDeleted, hardDeleted, err := s.stateStore.DeleteExpiredMessages(time.Now().UTC(), messaging.MailReadTTL, messaging.MailHardTTL)
			if err != nil {
				s.log.Debug("message janitor failed", "err", err)
				continue
			}
			if readDeleted > 0 || hardDeleted > 0 {
				s.log.Debug("message janitor deleted messages", "read", readDeleted, "hard", hardDeleted)
			}
		}
	}
}
