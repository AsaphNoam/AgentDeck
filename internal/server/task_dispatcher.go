package server

import (
	"context"
	"errors"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/state"
)

const (
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
	// A task that already confirmed an assignee acts on that same agent, whatever
	// its target kind. A retried launch-spec task resumes the agent it created
	// rather than minting a second one, which would fork its transcript and its
	// attached-context membership (FS-16.R23).
	if task.AssignedAgentID != "" {
		return s.planExistingAgentStart(task, attemptID, task.AssignedAgentID)
	}
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
		return s.planExistingAgentStart(task, attemptID, task.TargetAgentID)
	}
}

// planExistingAgentStart decides how this start would treat an agent that
// already exists. Borrowing a runtime that is already up brings up no process
// and takes no budget; waking a stopped one does. Eligibility is deliberately
// not decided here: a target that can never be started parks its task, and
// parking is a transition out of starting, so it happens once the reservation
// exists.
func (s *Server) planExistingAgentStart(task state.Task, attemptID, agentID string) (state.TaskReservation, bool) {
	claim := state.ClaimWoke
	generation := attemptID
	if _, err := s.stateStore.ReadRunning(agentID); err == nil {
		claim = state.ClaimBorrowed
		generation = s.registry.Generation(agentID)
		if generation == "" {
			s.log.Debug("task target runtime has no local generation", "task", task.TaskID, "agent", agentID)
			return state.TaskReservation{}, false
		}
	} else if !errors.Is(err, state.ErrNotFound) {
		s.log.Debug("task target liveness read failed", "task", task.TaskID, "err", err)
		return state.TaskReservation{}, false
	}
	return state.TaskReservation{
		AttemptID: attemptID, AgentID: agentID, Generation: generation, Claim: claim,
	}, true
}

// startAdmittedTask performs the effect the reservation authorized. A task that
// brings up its own agent launches it; one that targets an existing agent crosses
// into that conversation through the dependency activation kind (FS-16.R6).
func (s *Server) startAdmittedTask(ctx context.Context, task state.Task) {
	s.taskStartMu.Lock()
	defer s.taskStartMu.Unlock()
	// Admission and the effect are deliberately separated, so cancellation can
	// finish in between. Re-read under the same server claim cancel takes before
	// launching, resuming, or prompting (TS-10.R4, INV §5).
	fresh, err := s.stateStore.ReadTask(task.TaskID)
	if err != nil || fresh.State != state.TaskStarting || fresh.StartAttemptID != task.StartAttemptID {
		return
	}
	task = fresh
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
			// The starting reservation is the only durable ownership of this
			// runtime. Keep it until recovery can stop the runtime (INV §4/§15).
			return
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
	if wakeErr == nil {
		if _, ok, err := s.stateStore.SetTaskStartGeneration(task.TaskID, task.StartAttemptID, s.registry.Generation(task.AssignedAgentID)); err != nil || !ok {
			s.log.Debug("record woken task generation failed", "task", task.TaskID, "err", err)
			s.releaseDependencyActivation(activation, token)
			return
		}
		task, _ = s.stateStore.ReadTask(task.TaskID)
	}
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
	if confirmed, ok, err := s.stateStore.ConfirmTaskStart(task.TaskID, task.StartAttemptID); err != nil {
		s.log.Debug("confirm task start failed", "task", task.TaskID, "err", err)
	} else if !ok {
		s.log.Debug("task start was settled by someone else", "task", task.TaskID)
	} else {
		s.publishTaskUpdate(confirmed)
	}
}

// parkTaskStart stops a task whose target can never become startable, spending no
// attempt, because retrying it is not a repair (FS-16.R8, R19).
func (s *Server) parkTaskStart(task state.Task, reason string) {
	if parked, ok, err := s.stateStore.ParkTaskStart(task.TaskID, task.StartAttemptID, reason); err != nil {
		s.log.Debug("park task start failed", "task", task.TaskID, "err", err)
	} else if ok {
		s.publishTaskUpdate(parked)
		s.propagateTaskFailure(parked)
	}
}

// deferTaskStart gives an admitted reservation back without spending an attempt:
// contention is not a failed start (FS-16.R25).
func (s *Server) deferTaskStart(task state.Task, why string) {
	if deferred, ok, err := s.stateStore.AbandonTaskStart(task.TaskID, task.StartAttemptID); err != nil {
		s.log.Debug("abandon task start failed", "task", task.TaskID, "why", why, "err", err)
	} else if ok {
		s.publishTaskUpdate(deferred)
	}
}

// failTaskStart spends one of the bounded attempts and parks the task on the
// last one, so a start that keeps failing surfaces instead of retrying forever
// (FS-16.R8, R25, INV §8).
func (s *Server) failTaskStart(task state.Task, reason string) {
	if failed, ok, err := s.stateStore.FailTaskStart(task.TaskID, task.StartAttemptID, reason); err != nil {
		s.log.Debug("fail task start failed", "task", task.TaskID, "err", err)
	} else if ok {
		s.publishTaskUpdate(failed)
		s.propagateTaskFailure(failed)
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
		return config.DefaultTaskConcurrency
	}
	return cfg.TaskConcurrency
}

// dispatchTurnEnd fans one completed turn out to every control plane that waits
// for a turn boundary. Both wait for the same thing and for the same reason —
// an agent that reported a result is still receiving that call's response, so
// nothing may stop it until the turn ends — so this is one dispatch with two
// subscribers rather than two paths (TS-10.R19, TS-09.R9–R11, INV §2).
func (s *Server) dispatchTurnEnd(agentID, generation string) {
	if s.pipelineMgr != nil {
		if err := s.pipelineMgr.OnTurnEnd(agentID, generation); err != nil {
			s.log.Warn("pipeline turn boundary", "agent_id", agentID, "err", err)
		}
	}
	s.releaseReportedTask(context.Background(), agentID, generation)
}

// releaseReportedTask completes the release a recorded result already promised:
// it stops a runtime this task created or woke and leaves a borrowed one alone,
// then drops the claim (FS-16.R4, TS-10.R19). A stop that cannot happen now
// leaves the durable intent standing for recovery to finish, because dropping
// the claim first would abandon a live runtime nothing owns (INV §15).
func (s *Server) releaseReportedTask(ctx context.Context, agentID, generation string) {
	task, err := s.stateStore.PendingReleaseTask(agentID, generation)
	if errors.Is(err, state.ErrNotFound) {
		return
	}
	if err != nil {
		s.log.Debug("read pending task release failed", "agent", agentID, "err", err)
		return
	}
	if task.RuntimeClaim == state.ClaimCreated || task.RuntimeClaim == state.ClaimWoke {
		if err := s.StopStage(ctx, agentID); err != nil {
			s.log.Debug("stop task runtime failed", "task", task.TaskID, "agent", agentID, "err", err)
			return
		}
	}
	if err := s.stateStore.CompleteTaskRelease(task.TaskID); err != nil {
		s.log.Debug("complete task release failed", "task", task.TaskID, "err", err)
	}
}

// evaluateTaskResult releases the arms waiting on a task that just recorded its
// outcome. Evaluation reads the durable registration rather than an outcome
// handed to it, so a dropped or repeated notification changes nothing
// (TS-10.R3).
func (s *Server) evaluateTaskResult(taskID string) {
	if changed, err := s.stateStore.EvaluateSource(state.SourceTask, taskID); err != nil {
		s.log.Debug("evaluate task result failed", "task", taskID, "err", err)
	} else {
		for _, task := range changed {
			s.publishTaskUpdate(task)
			s.propagateTaskFailure(task)
		}
	}
}

func (s *Server) propagateTaskFailure(task state.Task) {
	if task.State != state.TaskDependencyFailed {
		return
	}
	changed, err := s.stateStore.MarkSourceUnsatisfiable(state.SourceTask, task.TaskID)
	if err != nil {
		s.log.Debug("propagate task failure", "task", task.TaskID, "err", err)
		return
	}
	for _, dependent := range changed {
		s.publishTaskUpdate(dependent)
		s.propagateTaskFailure(dependent)
	}
}

// recoverTasks is the one bounded startup sweep over dependent work. It never
// asks whether a pre-crash runtime survived, because the registry starts empty
// and stale reconciliation deliberately never re-adopts a live process, so that
// question is unanswerable by construction. Every `starting` row is resolved from
// what it durably reserved instead (TS-10.R15, FS-16.R17).
//
// Failing as a whole is fatal to startup for the same reason it is for mail
// activations: a task left `starting` is invisible to the dispatcher, which only
// ever lists ready rows, so its work would never run again. A single task's
// failure is isolated and does not abort the sweep (INV §7).
func (s *Server) recoverTasks(ctx context.Context) error {
	if err := s.stateStore.DiscardDependencyActivations(); err != nil {
		return err
	}
	awaiting, err := s.stateStore.TasksAwaitingRelease()
	if err != nil {
		return err
	}
	for _, task := range awaiting {
		s.finishInterruptedRelease(ctx, task)
	}
	unfinished, err := s.stateStore.TasksInStates(state.TaskStarting, state.TaskRunning)
	if err != nil {
		return err
	}
	for _, task := range unfinished {
		if task.State == state.TaskRunning {
			// Its agent is an unowned orphan now, which the ordinary reconciliation
			// sweep reaps. Nothing is resumed on a guess and no outcome is invented.
			s.interruptTask(task, "the server restarted while this task was running")
			continue
		}
		s.recoverStartAttempt(ctx, task)
	}
	// Arms may have been satisfied by a result committed just before the crash.
	// Re-evaluating is a no-op when nothing changed, because evaluation reads the
	// durable registration rather than an event.
	armed, err := s.stateStore.TasksInStates(state.TaskArmed)
	if err != nil {
		return err
	}
	for _, task := range armed {
		for _, arm := range task.Arms {
			if arm.Kind != state.ArmWorkResult || arm.State != state.ArmUnsatisfied {
				continue
			}
			if _, err := s.stateStore.EvaluateSource(arm.SourceKind, arm.SourceID); err != nil {
				s.log.Warn("task recovery re-evaluation failed", "task", task.TaskID, "err", err)
			}
		}
	}
	return nil
}

// recoverStartAttempt resolves one interrupted start by the runtime claim it
// reserved. A runtime this attempt would have created or woken is reaped and the
// task is started once more within its attempt limit; a runtime it merely
// borrowed is never touched, because it belongs to someone else and this
// feature's promise does not lapse because AgentDeck restarted. That task becomes
// interrupted, since whether the assignment reached that conversation cannot be
// known and delivering it twice is worse than asking a person (FS-16.R4, R17).
func (s *Server) recoverStartAttempt(ctx context.Context, task state.Task) {
	if task.RuntimeClaim == state.ClaimBorrowed {
		s.interruptTask(task, "the server restarted while this task was starting in an existing conversation")
		return
	}
	if task.AssignedAgentID != "" {
		if err := s.StopStage(ctx, task.AssignedAgentID); err != nil {
			s.log.Warn("reap interrupted task runtime failed", "task", task.TaskID, "err", err)
			// The starting row still owns this runtime. Do not clear that only
			// durable claim after a failed reap; the next recovery pass can try
			// again without admitting a second agent (INV §4/§15).
			return
		}
	}
	if failed, ok, err := s.stateStore.FailTaskStart(task.TaskID, task.StartAttemptID,
		"the server restarted during this start attempt"); err != nil {
		s.log.Warn("recover start attempt failed", "task", task.TaskID, "err", err)
	} else if ok {
		s.publishTaskUpdate(failed)
		s.propagateTaskFailure(failed)
	}
}

// finishInterruptedRelease completes a stop and release a reporting turn never
// finished, so a recorded result can never leave a task-owned runtime up with
// nothing owning it (TS-10.R19, INV §15).
func (s *Server) finishInterruptedRelease(ctx context.Context, task state.Task) {
	if task.RuntimeClaim == state.ClaimCreated || task.RuntimeClaim == state.ClaimWoke {
		if err := s.StopStage(ctx, task.AssignedAgentID); err != nil {
			s.log.Warn("finish interrupted task release failed", "task", task.TaskID, "err", err)
			return
		}
	}
	if err := s.stateStore.CompleteTaskRelease(task.TaskID); err != nil {
		s.log.Warn("complete interrupted task release failed", "task", task.TaskID, "err", err)
	}
}

func (s *Server) interruptTask(task state.Task, reason string) {
	if interrupted, ok, err := s.stateStore.InterruptTask(task.TaskID, reason); err != nil {
		s.log.Warn("interrupt task failed", "task", task.TaskID, "err", err)
	} else if ok {
		s.publishTaskUpdate(interrupted)
	}
}

// interruptTaskOnExit is the runtime-exit half of FS-16.R16: an assignee that
// goes away before recording a result leaves its task interrupted, needing
// attention, with its claim and slot released — never converted into a result.
// A task that already recorded one is untouched, which is what makes the stop
// that follows a report safe.
func (s *Server) interruptTaskOnExit(agentID, generation, cause string) {
	if task, ok, err := s.stateStore.InterruptTaskForAgent(agentID, generation,
		"the assigned agent went away before recording a result: "+cause); err != nil {
		s.log.Warn("interrupt task on agent exit", "agent_id", agentID, "err", err)
	} else if ok {
		s.publishTaskUpdate(task)
		s.log.Info("task interrupted by agent exit", "agent_id", agentID, "cause", cause)
	}
}
