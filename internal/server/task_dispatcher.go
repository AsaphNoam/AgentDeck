package server

import (
	"context"
	"errors"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

const (
	// defaultTaskConcurrency is the shipped install-wide budget for runtimes
	// dependent work brings up (FS-16.R7, R21, FS-04.R43).
	defaultTaskConcurrency = 10
	// taskDispatchInterval is the dispatcher's ticker. Durable rows are the
	// authority, so a missed notification costs latency, never correctness
	// (TS-10.R2).
	taskDispatchInterval = 2 * time.Second
	// taskDispatchBatch bounds one pass both ways: how many ready rows it reads
	// and how many starts it may have in flight across passes (TS-10.R3).
	taskDispatchBatch = 8
)

// startTaskDispatcher runs the admission loop beside the activation executor,
// in this process and with no second scheduler (TS-10.R1).
func (s *Server) startTaskDispatcher(ctx context.Context) {
	go s.runTaskDispatcher(ctx)
}

func (s *Server) runTaskDispatcher(ctx context.Context) {
	ticker := time.NewTicker(taskDispatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.dispatchReadyTasks(ctx)
		}
	}
}

// dispatchReadyTasks admits ready work in the order it became ready, up to the
// budget. Admission is serial and the effect is not: the admitting statement is
// one fast write whose order is the promise (FS-16.R7), while a launch takes
// seconds and must not hold the pass behind it.
func (s *Server) dispatchReadyTasks(ctx context.Context) {
	if s.registry == nil {
		return
	}
	ready, err := s.stateStore.ReadyTasks(taskDispatchBatch)
	if err != nil {
		s.log.Debug("task dispatch list ready failed", "err", err)
		return
	}
	budget := s.taskConcurrencyBudget()
	for _, task := range ready {
		plan, ok := s.planTaskStart(task)
		if !ok {
			continue
		}
		// Take the worker slot before admitting, so a full worker pool leaves the
		// task untouched and still ready rather than starting-with-nobody-working.
		select {
		case s.taskStartSlots <- struct{}{}:
		default:
			return
		}
		admitted, ok, err := s.stateStore.AdmitReadyTask(task.TaskID, plan, budget)
		if err != nil {
			s.log.Debug("task admission failed", "task", task.TaskID, "err", err)
		}
		if err != nil || !ok {
			// Not admitted: no capacity, another task holds this agent, or the row
			// is no longer ready. None of those spends an attempt (FS-16.R25).
			<-s.taskStartSlots
			continue
		}
		go func(task state.Task) {
			defer func() { <-s.taskStartSlots }()
			s.startAdmittedTask(ctx, task)
		}(admitted)
	}
}

// planTaskStart decides what this start would do to a runtime before anything is
// claimed: the agent it acts on, the generation it acts under, and whether it
// creates, wakes, or borrows that runtime (TS-10.R4).
func (s *Server) planTaskStart(task state.Task) (state.TaskReservation, bool) {
	attemptID := "ta_" + mintHookToken()
	switch task.TargetKind {
	case state.TargetLaunch:
		agentID, err := s.stateStore.NewAgentID()
		if err != nil {
			s.log.Debug("mint task agent failed", "task", task.TaskID, "err", err)
			return state.TaskReservation{}, false
		}
		// The generation is the attempt id, as it is for a pipeline stage: one
		// start attempt is exactly one launch generation.
		return state.TaskReservation{
			AttemptID: attemptID, AgentID: agentID, Generation: attemptID,
			Claim: state.ClaimCreated,
		}, true
	default:
		// Borrowing a runtime that is already up brings up no process and takes no
		// budget; waking a stopped one does. Eligibility is deliberately not decided
		// here: a target that can never be started parks its task, and parking is a
		// transition out of starting, so it happens after the reservation exists.
		claim := state.ClaimWoke
		if _, err := s.stateStore.ReadRunning(task.TargetAgentID); err == nil {
			claim = state.ClaimBorrowed
		} else if !errors.Is(err, state.ErrNotFound) {
			s.log.Debug("task target liveness read failed", "task", task.TaskID, "err", err)
			return state.TaskReservation{}, false
		}
		return state.TaskReservation{
			AttemptID: attemptID, AgentID: task.TargetAgentID, Generation: attemptID,
			Claim: claim,
		}, true
	}
}

// startAdmittedTask performs the effect the reservation authorized. A task that
// brings up its own agent launches it; one that targets an existing agent crosses
// into that conversation through the dependency activation kind (FS-16.R6).
func (s *Server) startAdmittedTask(ctx context.Context, task state.Task) {
	if task.RuntimeClaim == state.ClaimCreated {
		s.startLaunchedTask(ctx, task)
		return
	}
	s.startExistingAgentTask(ctx, task)
}

// startLaunchedTask brings up a new agent for a launch-spec task and then settles
// the row: confirmed into running, given back on a deferral, or spent as one of
// the bounded attempts on a real failure (FS-16.R25, TS-10.R4).
func (s *Server) startLaunchedTask(ctx context.Context, task state.Task) {
	// Launch mints the same agent-keyed registration as resume, so the claim
	// covers the launch and its first assignment prompt (TS-01.R16, INV §4/§5).
	if !s.claimLifecycle(task.AssignedAgentID) {
		s.deferTaskStart(task, "lifecycle busy")
		return
	}
	defer s.releaseLifecycle(task.AssignedAgentID)

	_, ae := s.launchAgent(ctx, launchRequest{
		Role: task.Role, Project: task.Project, Backend: task.Backend, Model: task.Model,
		Interface: "chat", Name: task.DisplayName,
	}, launchOptions{AgentID: task.AssignedAgentID, Generation: task.AssignedGeneration})
	if ae != nil {
		s.failTaskStart(task, "launch failed: "+ae.Message)
		return
	}
	// The task's instruction is the assignment input FS-00.R14–R15 permits, sent
	// as an ordinary turn exactly as a pipeline stage's assignment is.
	if err := s.registry.SendPrompt(ctx, task.AssignedAgentID, task.Instruction); err != nil {
		// The claim is already held, so use the unclaimed stop core.
		if stopErr := s.stopStageLocked(ctx, task.AssignedAgentID); stopErr != nil {
			s.log.Debug("stop failed task runtime failed", "agent", task.AssignedAgentID, "err", stopErr)
		}
		s.failTaskStart(task, "assignment was not delivered: "+err.Error())
		return
	}
	s.confirmTaskStart(task)
}

// startExistingAgentTask crosses into an agent's existing conversation with one
// host-owned dependency activation: the same primitive mail uses, with its own
// instruction and its own retry policy. The activation stays actionable until the
// task confirms its start, so a deferral returns both the row and the task rather
// than spending either (TS-10.R5, R6).
func (s *Server) startExistingAgentTask(ctx context.Context, task state.Task) {
	agent, err := s.stateStore.ReadAgent(task.AssignedAgentID)
	if errors.Is(err, state.ErrNotFound) {
		s.parkTaskStart(task, "the target agent no longer exists")
		return
	}
	if err != nil {
		s.deferTaskStart(task, "read target agent")
		return
	}
	// FS-07's messaging boundary: a terminal agent has no model turn to give, so
	// this can never become startable and retrying it is not a repair (FS-16.R2).
	if agent.Interface != "chat" {
		s.parkTaskStart(task, "the target agent runs a terminal interface and cannot execute work")
		return
	}
	if agent.Archived {
		s.parkTaskStart(task, "the target agent is archived")
		return
	}
	activation, err := s.stateStore.EnsurePendingDependencyActivation(task.AssignedAgentID, task.TaskID)
	if err != nil {
		s.log.Debug("ensure dependency activation failed", "task", task.TaskID, "err", err)
		s.deferTaskStart(task, "ensure activation")
		return
	}
	token, claimed, err := s.stateStore.ClaimActivation(state.ActivationKindDependency, activation.ActivationID)
	if err != nil {
		s.log.Debug("claim dependency activation failed", "task", task.TaskID, "err", err)
	}
	if err != nil || !claimed {
		s.deferTaskStart(task, "claim activation")
		return
	}

	running := false
	if _, err := s.stateStore.ReadRunning(task.AssignedAgentID); err == nil {
		running = true
	} else if !errors.Is(err, state.ErrNotFound) {
		s.log.Debug("task target liveness read failed", "task", task.TaskID, "err", err)
		s.settleActivationStart(task, activation, token, false, false)
		return
	}
	// The reservation decides whether finishing stops this runtime, so a target
	// that changed liveness between planning and starting is re-planned on the
	// next pass rather than started under a claim that is now wrong (FS-16.R4).
	if running != (task.RuntimeClaim == state.ClaimBorrowed) {
		s.settleActivationStart(task, activation, token, false, false)
		return
	}
	if running {
		attempted, started := s.runActivationTurn(ctx, state.ActivationKindDependency, activation, token)
		s.settleActivationStart(task, activation, token, attempted, started)
		return
	}
	// A stopped target is woken on exactly the terms mail is (FS-16.R19).
	_, ok, ae := s.wakeCandidate(task.AssignedAgentID)
	if ae != nil {
		s.log.Debug("task wake candidacy failed", "task", task.TaskID, "err", ae.Message)
		s.settleActivationStart(task, activation, token, false, false)
		return
	}
	if !ok {
		s.releaseDependencyActivation(activation, token)
		s.parkTaskStart(task, "the target agent cannot be woken for work")
		return
	}
	attempted, wakeErr := s.wakeForActivation(ctx, state.ActivationKindDependency, activation, token)
	s.settleActivationStart(task, activation, token, attempted, wakeErr == nil)
}

// settleActivationStart resolves the task and its activation together. A turn
// that never crossed the attempt boundary spent nothing: both go back. A turn
// that crossed it consumed the opportunity, so the row is retired either way and
// only the task's fate differs (TS-10.R5, FS-16.R25).
func (s *Server) settleActivationStart(task state.Task, activation state.Activation, token string, attempted, started bool) {
	if !attempted {
		s.releaseDependencyActivation(activation, token)
		s.deferTaskStart(task, "the activation turn did not start")
		return
	}
	if started {
		s.confirmTaskStart(task)
	} else {
		s.failTaskStart(task, "the assignment turn did not complete")
	}
	if err := s.stateStore.RetireActivation(state.ActivationKindDependency, activation.ActivationID, token); err != nil {
		s.log.Debug("retire dependency activation failed", "activation", activation.ActivationID, "err", err)
	}
}

func (s *Server) releaseDependencyActivation(activation state.Activation, token string) {
	if err := s.stateStore.ReleaseActivation(state.ActivationKindDependency, activation.ActivationID, token); err != nil {
		s.log.Debug("release dependency activation failed", "activation", activation.ActivationID, "err", err)
	}
}

// confirmTaskStart is the only transition into running: the assignment is in a
// live runtime that holds it (FS-16.R6, TS-10.R4).
func (s *Server) confirmTaskStart(task state.Task) {
	if _, ok, err := s.stateStore.ConfirmTaskStart(task.TaskID, task.StartAttemptID); err != nil {
		s.log.Debug("confirm task start failed", "task", task.TaskID, "err", err)
	} else if !ok {
		s.log.Debug("task start was settled by someone else", "task", task.TaskID)
	}
}

// parkTaskStart stops a task whose target can never become startable, spending no
// attempt, because retrying it is not a repair (FS-16.R8, R19).
func (s *Server) parkTaskStart(task state.Task, reason string) {
	if _, _, err := s.stateStore.ParkTaskStart(task.TaskID, task.StartAttemptID, reason); err != nil {
		s.log.Debug("park task start failed", "task", task.TaskID, "err", err)
	}
}

// deferTaskStart gives an admitted reservation back without spending an attempt:
// contention is not a failed start (FS-16.R25).
func (s *Server) deferTaskStart(task state.Task, why string) {
	if _, _, err := s.stateStore.AbandonTaskStart(task.TaskID, task.StartAttemptID); err != nil {
		s.log.Debug("abandon task start failed", "task", task.TaskID, "why", why, "err", err)
	}
}

// failTaskStart spends one of the bounded attempts and parks the task on the
// last one, so a start that keeps failing surfaces instead of retrying forever
// (FS-16.R8, R25, INV §8).
func (s *Server) failTaskStart(task state.Task, reason string) {
	if _, _, err := s.stateStore.FailTaskStart(task.TaskID, task.StartAttemptID, reason); err != nil {
		s.log.Debug("fail task start failed", "task", task.TaskID, "err", err)
	}
}

// taskConcurrencyBudget reads the install-wide budget fresh, so changing it takes
// effect on the next pass without a restart (FS-16.R21).
func (s *Server) taskConcurrencyBudget() int {
	cfg := s.cfg
	if fromDisk, err := s.configStore.ReadConfig(); err == nil {
		cfg = fromDisk
	}
	if cfg.TaskConcurrency <= 0 {
		return defaultTaskConcurrency
	}
	return cfg.TaskConcurrency
}
