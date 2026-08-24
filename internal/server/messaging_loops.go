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

// runActivationExecutor uses its channel only as a fast path. Durable pending
// activation rows, checked at startup and on every sweep, remain authoritative.
func (s *Server) runActivationExecutor(ctx context.Context) {
	ticker := time.NewTicker(messaging.ActivationSweepInterval)
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
	activations, err := s.stateStore.PendingActivations(state.ActivationKindMail, onlyAgentID, messaging.ActivationBatch)
	if err != nil {
		s.log.Debug("activation list pending mail failed", "err", err)
		return
	}
	for _, activation := range activations {
		// Admission is bounded across sweeps, not just within one: an activation
		// that resumes a stopped recipient outlives the two-second tick, so
		// counting only per-sweep rows would still let ticks stack their process
		// launches. A row we do not admit is untouched and still pending
		// (TS-01.R20, INV §9).
		select {
		case s.activationSlots <- struct{}{}:
		default:
			return
		}
		go func() {
			defer func() { <-s.activationSlots }()
			s.executeMailActivation(ctx, activation)
		}()
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
	//
	// This is only a fast path that avoids taking a mail claim we would give
	// straight back. It is a non-claiming hint, so it cannot decide the race by
	// itself; both branches below arbitrate on the claim itself — the stopped one
	// through resumeSessionWithHooks, the running one directly.
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
	// A pending activation can outlive the recipient's eligibility: a runtime
	// switch to terminal happens after the sender inserted mail. A terminal agent
	// cannot drain a mailbox at all (FS-07 §6), so this exclusion is durable on
	// both branches: discard the opportunity without crossing mail's
	// non-replayable boundary.
	//
	// The other durable exclusions — archived agent, archived project, pipeline
	// association — are *stopped-only* wake gates (FS-01.R33, FS-06.R22/R27), and
	// belong to the stopped branch alone, where startStoppedMailActivation applies
	// them through the one shared wakeCandidate/stoppedWakeGates predicate (INV
	// §2). Repeating them here excluded running recipients the addressable set
	// still accepts mail for: a pipeline attempt row is permanent, so a running
	// agent that had ever run a stage silently stopped being activated forever.
	agent, err := s.stateStore.ReadAgent(activation.AgentID)
	if err != nil {
		s.log.Debug("activation read agent failed", "agent", activation.AgentID, "err", err)
		s.releaseMailActivation(activation, token)
		return
	}
	if agent.Interface != "chat" {
		s.discardMailActivation(activation, token)
		return
	}
	if _, err := s.stateStore.ReadRunning(activation.AgentID); err == nil {
		s.startRunningMailActivation(ctx, activation, token)
		return
	} else if !errors.Is(err, state.ErrNotFound) {
		s.log.Debug("activation read running failed", "agent", activation.AgentID, "err", err)
		s.releaseMailActivation(activation, token)
		return
	}
	s.startStoppedMailActivation(ctx, activation, token)
}

func (s *Server) startRunningMailActivation(ctx context.Context, activation state.Activation, token string) {
	// The turn gate this is about to take is the one a stage's own prompt needs,
	// so taking it has to be arbitrated against the exclusive lifecycle claim in
	// one place rather than sampled beforehand: a Continue that claims after the
	// caller's hint read false still composes its assignment into an agent this
	// activation has since made busy, and ErrTurnInFlight pauses the run. The
	// stopped branch already arbitrates this way through resumeSessionWithHooks;
	// a running recipient needs the same claim held across the turn start
	// (TS-01.R21, INV §5). Losing it is pre-attempt, so mail returns to pending.
	if !s.claimLifecycle(activation.AgentID) {
		s.releaseMailActivation(activation, token)
		return
	}
	defer s.releaseLifecycle(activation.AgentID)
	attempted := false
	started, err := s.registry.StartActivation(ctx, activation.AgentID, state.ActivationKindMail, func(turnID string) error {
		var err error
		attempted, err = s.stateStore.AttemptActivation(state.ActivationKindMail, activation.ActivationID, token, turnID)
		if err == nil && !attempted {
			return errors.New("mail activation claim lost")
		}
		return err
	})
	if err != nil {
		s.log.Debug("start running mail activation failed", "agent", activation.AgentID, "err", err)
	}
	if !started && !attempted {
		s.releaseMailActivation(activation, token)
		return
	}
	if attempted {
		if err := s.stateStore.RetireActivation(state.ActivationKindMail, activation.ActivationID, token); err != nil {
			s.log.Debug("retire running mail activation failed", "activation", activation.ActivationID, "err", err)
		}
	}
}

func (s *Server) startStoppedMailActivation(ctx context.Context, activation state.Activation, token string) {
	if _, ok, ae := s.wakeCandidate(activation.AgentID); ae != nil {
		s.log.Debug("mail activation wake candidacy failed", "agent", activation.AgentID, "err", ae.Message)
		s.releaseMailActivation(activation, token)
		return
	} else if !ok {
		// A concurrent ordinary resume can make a formerly stopped recipient
		// running between the earlier running-row read and the wake gate. That
		// is not an exclusion: route it through the running activation path.
		if _, err := s.stateStore.ReadRunning(activation.AgentID); err == nil {
			s.startRunningMailActivation(ctx, activation, token)
			return
		} else if !errors.Is(err, state.ErrNotFound) {
			s.log.Debug("activation re-read running failed", "agent", activation.AgentID, "err", err)
			s.releaseMailActivation(activation, token)
			return
		}
		s.discardMailActivation(activation, token)
		return
	}
	attempted := false
	ae := s.resumeSessionWithHooks(ctx, activation.AgentID, resumeOverride{}, func() error {
		var err error
		attempted, err = s.stateStore.AttemptActivation(state.ActivationKindMail, activation.ActivationID, token, "")
		if err == nil && !attempted {
			return errors.New("mail activation claim lost")
		}
		return err
	}, func() error {
		started, err := s.registry.StartActivation(ctx, activation.AgentID, state.ActivationKindMail, func(turnID string) error {
			return s.stateStore.StartAttemptedTurn(state.ActivationKindMail, activation.ActivationID, token, turnID)
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
		if err := s.stateStore.RetireActivation(state.ActivationKindMail, activation.ActivationID, token); err != nil {
			s.log.Debug("retire stopped mail activation failed", "activation", activation.ActivationID, "err", err)
		}
		return
	}
	s.releaseMailActivation(activation, token)
}

func (s *Server) releaseMailActivation(activation state.Activation, token string) {
	if err := s.stateStore.ReleaseMailActivation(activation.ActivationID, token); err != nil {
		s.log.Debug("release mail activation failed", "activation", activation.ActivationID, "err", err)
	}
}

func (s *Server) discardMailActivation(activation state.Activation, token string) {
	if err := s.stateStore.DiscardClaimedActivation(state.ActivationKindMail, activation.ActivationID, token); err != nil {
		s.log.Debug("discard mail activation failed", "activation", activation.ActivationID, "err", err)
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
