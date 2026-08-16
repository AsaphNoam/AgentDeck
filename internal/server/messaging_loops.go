package server

import (
	"context"
	"errors"
	"time"

	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/runtime"
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
		if _, err := s.stateStore.MarkUnreadDeliveredVia(row.AgentID, "nudge"); err != nil {
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
// "pending", so a failed wake — which stamps those rows "wake_failed" — cannot
// respawn the same broken agent on the next tick or after a dashboard restart,
// while new mail always inserts as "pending" and re-arms the wake. Woken agents
// are nudged by the ordinary running path above once they report idle.
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
		if _, ok := s.wakeCandidate(agentID); !ok {
			continue // running, archived, snapshot-less, terminal, or pipeline-owned
		}
		ns := stateByAgent[agentID]
		// The exclusive resume claim is the re-entry guard: this loop must not
		// wait out a wake, and the nudge in-flight marker belongs to the running
		// path that delivers check_messages once the agent is back.
		if s.resumeInFlight(agentID) || now.Sub(ns.lastNudgeAt) < messaging.NudgeCooldown {
			continue
		}
		ns.lastNudgeAt = now
		stateByAgent[agentID] = ns
		go func(agentID string) {
			_, ae := s.wakeAgent(ctx, agentID)
			if ae == nil {
				return
			}
			s.log.Debug("mail wake failed", "agent", agentID, "err", ae.Message)
			if ae.Code == runtime.CodeConflict {
				// Another resume owns this agent; nothing was attempted here, so
				// the mail stays pending for the next sweep.
				return
			}
			if _, err := s.stateStore.MarkUnreadDeliveredVia(agentID, "wake_failed"); err != nil {
				s.log.Debug("mark wake failed", "agent", agentID, "err", err)
			}
		}(agentID)
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
