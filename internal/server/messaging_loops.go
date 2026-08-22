package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/state"
)

// startMessagingLoops recovers durable activation state and then starts the
// executor and retention loops. Recovery failure is fatal to startup by design:
// the executor only ever lists `pending` rows, so a claimed pre-attempt row that
// recovery could not release stays invisible for the life of the process and its
// mail never activates. Failing loudly beats silently stranding work (INV §9/§15).
func (s *Server) startMessagingLoops(ctx context.Context) error {
	if err := s.stateStore.RecoverMailActivations(); err != nil {
		return fmt.Errorf("recover mail activations: %w", err)
	}
	go s.runActivationExecutor(ctx)
	go s.runMessageJanitor(ctx)
	return nil
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
	// An exclusive lifecycle transition (pipeline stage launch/continue, runtime
	// switch, stop, resume) can already own this agent while its runtime is
	// registered and idle — a stage's first durable assignment is composed inside
	// that claim. Starting an activation in that window lets mail win the runtime
	// turn gate and fail the transition's own prompt with ErrTurnInFlight, pausing
	// or stopping the stage. The opportunity is durable, so deferring it to the
	// next sweep costs nothing (TS-01.R21, INV §5).
	if s.lifecycleInFlight(activation.AgentID) {
		return
	}
	// One statement decides availability and the claim together. Reading unread
	// mail and claiming separately let a concurrent check_messages drain the last
	// row in between, after which the claim crossed the non-replayable attempt
	// boundary and sent an empty provider turn (FS-06.R25, INV §5/§15).
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
