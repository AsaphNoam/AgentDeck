package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/runtime"
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
	attempted, started := s.runActivationTurn(ctx, state.ActivationKindMail, activation, token)
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

// runActivationTurn starts one host-owned turn of this kind on a live agent and
// reports whether the kind's non-replayable boundary was crossed. It owns the
// turn start and nothing else: what to do with a claim that never crossed one is
// the calling kind's own policy.
//
// The turn gate it takes is the one a pipeline stage's own prompt needs, so
// taking it has to be arbitrated against the exclusive lifecycle claim in one
// place rather than sampled beforehand: a Continue that claims after the caller's
// hint read false still composes its assignment into an agent this activation has
// since made busy, and ErrTurnInFlight pauses the run. The stopped branch
// arbitrates the same way through resumeSessionWithHooks; a running recipient
// needs the same claim held across the turn start (TS-01.R21, INV §5). Losing it
// is pre-attempt, so it is reported as neither started nor attempted.
func (s *Server) runActivationTurn(ctx context.Context, kind string, activation state.Activation, token string) (attempted, started bool) {
	if !s.claimLifecycle(activation.AgentID) {
		return false, false
	}
	defer s.releaseLifecycle(activation.AgentID)
	started, err := s.registry.StartActivation(ctx, activation.AgentID, kind, func(turnID string) error {
		var err error
		attempted, err = s.stateStore.AttemptActivation(kind, activation.ActivationID, token, turnID)
		if err == nil && !attempted {
			return errors.New("activation claim lost")
		}
		return err
	})
	if err != nil {
		s.log.Debug("start running activation failed", "kind", kind, "agent", activation.AgentID, "err", err)
	}
	return attempted, started
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
	// Mail spends its opportunity at the boundary: a resume that failed afterwards
	// is not replayed, which is what at-most-once means for it.
	if attempted, _ := s.wakeForActivation(ctx, state.ActivationKindMail, activation, token); attempted {
		if err := s.stateStore.RetireActivation(state.ActivationKindMail, activation.ActivationID, token); err != nil {
			s.log.Debug("retire stopped mail activation failed", "activation", activation.ActivationID, "err", err)
		}
		return
	}
	s.releaseMailActivation(activation, token)
}

// wakeForActivation resumes a stopped agent and starts one turn of this kind
// inside that resume. It reports whether the non-replayable boundary was crossed
// and how the resume itself ended, because those are different facts: the
// boundary is crossed in the pre-side-effect hook, so a resume can fail after a
// kind has already spent its opportunity (INV §15).
func (s *Server) wakeForActivation(ctx context.Context, kind string, activation state.Activation, token string) (bool, *runtime.APIError) {
	attempted := false
	ae := s.resumeSessionWithHooks(ctx, activation.AgentID, resumeOverride{}, func() error {
		var err error
		attempted, err = s.stateStore.AttemptActivation(kind, activation.ActivationID, token, "")
		if err == nil && !attempted {
			return errors.New("activation claim lost")
		}
		return err
	}, func() error {
		started, err := s.registry.StartActivation(ctx, activation.AgentID, kind, func(turnID string) error {
			return s.stateStore.StartAttemptedTurn(kind, activation.ActivationID, token, turnID)
		})
		if err != nil {
			return err
		}
		if !started {
			return errors.New("activation turn did not start")
		}
		return nil
	})
	if ae != nil {
		s.log.Debug("start stopped activation failed", "kind", kind, "agent", activation.AgentID, "err", ae.Message)
	}
	return attempted, ae
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
