package server

import (
	"context"
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
		// An existing-agent target crosses into a live conversation through the
		// dependency activation kind, which is the next piece; until it exists such
		// a task stays ready rather than being started the wrong way.
		return state.TaskReservation{}, false
	}
}

// startAdmittedTask performs the effect the reservation authorized and then
// settles the row: confirmed into running, given back on a deferral, or spent as
// one of the bounded attempts on a real failure (FS-16.R25, TS-10.R4).
func (s *Server) startAdmittedTask(ctx context.Context, task state.Task) {
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
	// Only now is the assignment in a confirmed live runtime, which is the whole
	// meaning of running (FS-16.R6, TS-10.R4).
	if _, ok, err := s.stateStore.ConfirmTaskStart(task.TaskID, task.StartAttemptID); err != nil {
		s.log.Debug("confirm task start failed", "task", task.TaskID, "err", err)
	} else if !ok {
		s.log.Debug("task start was settled by someone else", "task", task.TaskID)
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
