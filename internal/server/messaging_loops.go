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
