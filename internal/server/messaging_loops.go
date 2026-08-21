package server

import (
	"context"
	"errors"
	"time"

	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/state"
)

func (s *Server) startMessagingLoops(ctx context.Context) {
	if err := s.stateStore.RecoverMailActivations(); err != nil {
		s.log.Debug("recover mail activations failed", "err", err)
	}
	go s.runActivationExecutor(ctx)
	go s.runMessageJanitor(ctx)
}

// nudgeState/nudgeOnce keep older focused tests source-compatible while their
// behavior now exercises the activation executor rather than unread polling.
type nudgeState struct{}

func (s *Server) nudgeOnce(ctx context.Context, onlyAgentID string, _ map[string]nudgeState) {
	s.executePendingMailActivations(ctx, onlyAgentID)
}

// runActivationExecutor uses its channel only as a fast path. Durable pending
// activation rows, checked at startup and on every sweep, remain authoritative.
func (s *Server) runActivationExecutor(ctx context.Context) {
	ticker := time.NewTicker(messaging.NudgeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.executePendingMailActivations(ctx, "")
		case agentID := <-s.activationCh:
			s.executePendingMailActivations(ctx, agentID)
		}
	}
}

func (s *Server) executePendingMailActivations(ctx context.Context, onlyAgentID string) {
	if s.registry == nil {
		return
	}
	activations, err := s.stateStore.PendingMailActivations(onlyAgentID)
	if err != nil {
		s.log.Debug("activation list pending mail failed", "err", err)
		return
	}
	for _, activation := range activations {
		go s.executeMailActivation(ctx, activation)
	}
}

func (s *Server) executeMailActivation(ctx context.Context, activation state.Activation) {
	available, err := s.stateStore.HasUnreadMail(activation.AgentID)
	if err != nil {
		s.log.Debug("activation source check failed", "agent", activation.AgentID, "err", err)
		return
	}
	if !available {
		if err := s.stateStore.RetirePendingMailActivation(activation.ActivationID); err != nil {
			s.log.Debug("retire empty mail activation failed", "activation", activation.ActivationID, "err", err)
		}
		return
	}
	token, claimed, err := s.stateStore.ClaimMailActivation(activation.ActivationID)
	if err != nil {
		s.log.Debug("claim mail activation failed", "activation", activation.ActivationID, "err", err)
		return
	}
	if !claimed {
		return
	}
	if _, err := s.stateStore.ReadRunning(activation.AgentID); err == nil {
		s.startRunningMailActivation(ctx, activation, token)
		return
	} else if !errors.Is(err, state.ErrNotFound) {
		s.log.Debug("activation read running failed", "agent", activation.AgentID, "err", err)
		_ = s.stateStore.ReleaseMailActivation(activation.ActivationID, token)
		return
	}
	s.startStoppedMailActivation(ctx, activation, token)
}

func (s *Server) startRunningMailActivation(ctx context.Context, activation state.Activation, token string) {
	attempted := false
	started, err := s.registry.StartActivation(ctx, activation.AgentID, state.ActivationKindMail, func(turnID string) error {
		var err error
		attempted, err = s.stateStore.AttemptMailActivation(activation.ActivationID, token, turnID)
		if err == nil && !attempted {
			return errors.New("mail activation claim lost")
		}
		return err
	})
	if err != nil {
		s.log.Debug("start running mail activation failed", "agent", activation.AgentID, "err", err)
	}
	if !started && !attempted {
		_ = s.stateStore.ReleaseMailActivation(activation.ActivationID, token)
		return
	}
	if attempted {
		if err := s.stateStore.RetireMailActivation(activation.ActivationID, token); err != nil {
			s.log.Debug("retire running mail activation failed", "activation", activation.ActivationID, "err", err)
		}
	}
}

func (s *Server) startStoppedMailActivation(ctx context.Context, activation state.Activation, token string) {
	if _, ok, ae := s.wakeCandidate(activation.AgentID); ae != nil {
		s.log.Debug("mail activation wake candidacy failed", "agent", activation.AgentID, "err", ae.Message)
		_ = s.stateStore.ReleaseMailActivation(activation.ActivationID, token)
		return
	} else if !ok {
		_ = s.stateStore.ReleaseMailActivation(activation.ActivationID, token)
		return
	}
	attempted := false
	ae := s.resumeSessionWithHooks(ctx, activation.AgentID, resumeOverride{}, func() error {
		var err error
		attempted, err = s.stateStore.AttemptMailActivation(activation.ActivationID, token, "")
		if err == nil && !attempted {
			return errors.New("mail activation claim lost")
		}
		return err
	}, func() error {
		started, err := s.registry.StartActivation(ctx, activation.AgentID, state.ActivationKindMail, func(turnID string) error {
			return s.stateStore.StartAttemptedMailTurn(activation.ActivationID, token, turnID)
		})
		if err != nil {
			return err
		}
		if !started {
			return errors.New("mail activation turn did not start")
		}
		return nil
	})
	if ae != nil {
		s.log.Debug("start stopped mail activation failed", "agent", activation.AgentID, "err", ae.Message)
	}
	if attempted {
		if err := s.stateStore.RetireMailActivation(activation.ActivationID, token); err != nil {
			s.log.Debug("retire stopped mail activation failed", "activation", activation.ActivationID, "err", err)
		}
		return
	}
	_ = s.stateStore.ReleaseMailActivation(activation.ActivationID, token)
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
