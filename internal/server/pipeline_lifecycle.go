package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/AsaphNoam/Chuck/internal/config"
	"github.com/AsaphNoam/Chuck/internal/pipeline"
	"github.com/AsaphNoam/Chuck/internal/runtime"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// AcquirePipelineStart is the control-plane's shared project start lease. The
// manager obtains it before advancing durable run state, preserving the public
// archive conflict code rather than converting it to a later paused failure.
func (s *Server) AcquirePipelineStart(_ context.Context, projectID string) (func(), error) {
	if ae := s.acquireProjectStart(projectID); ae != nil {
		return nil, &pipeline.ProjectGateError{Code: ae.Code, Message: ae.Message}
	}
	if ae := s.projectArchiveGate(projectID, "project is archived"); ae != nil {
		s.releaseProjectStart(projectID)
		return nil, &pipeline.ProjectGateError{Code: ae.Code, Message: ae.Message}
	}
	return func() { s.releaseProjectStart(projectID) }, nil
}

// errPipelineOrchestratorContext marks a stored binding or snapshot that cannot
// supply a known orchestrator's frozen guidance; retrying cannot repair it.
var errPipelineOrchestratorContext = errors.New("pipeline orchestrator context is unavailable")

// pipelineOrchestratorInstructions resolves the frozen shared guidance for a
// fresh standing owner or dedicated coordinator launch (TS-09.R58). Only the
// pipeline's own creator kinds are eligible, so person/agent-created work never
// reads a run; a known participant whose context cannot be read fails closed.
func (s *Server) pipelineOrchestratorInstructions(task state.Task) (string, error) {
	if task.CreatedByKind != "pipeline" && task.CreatedByKind != "pipeline_coordinator" {
		return "", nil
	}
	snapshot, err := s.stateStore.PipelineOrchestratorTemplateSnapshot(task.TaskID)
	if errors.Is(err, state.ErrNotFound) {
		return "", fmt.Errorf("%w: no stage binding", errPipelineOrchestratorContext)
	}
	if err != nil {
		return "", err
	}
	var template pipeline.Template
	if err := json.Unmarshal(snapshot, &template); err != nil {
		return "", fmt.Errorf("%w: unreadable template snapshot", errPipelineOrchestratorContext)
	}
	return template.OrchestratorInstructions, nil
}

// pipelineOrchestrationBlock labels operator-authored shared guidance for the
// frozen base prompt; whitespace-only text adds nothing (TS-09.R59).
func pipelineOrchestrationBlock(instructions string) string {
	if strings.TrimSpace(instructions) == "" {
		return ""
	}
	return "Pipeline orchestration instructions\n" +
		"The pipeline operator set these shared defaults for every orchestrator in this run. " +
		"An exception stated explicitly in a stage assignment overrides them; Chuck's stage and result authority, project access and permissions still apply.\n\n" +
		instructions
}

// RoomLaunchConfig composes a fresh room slot's opaque launch request through
// the same composer standalone rooms use, so the room engine launches it like
// any reserved participant (TS-14.R19, INV §2).
func (s *Server) RoomLaunchConfig(_ context.Context, execution pipeline.StageExecution) (string, error) {
	req := launchRequest{Role: execution.Role, Project: execution.Project, Backend: execution.Backend,
		Model: execution.Model, Effort: execution.Effort, Fast: execution.Fast, Name: execution.AgentName}
	config, ae := s.thinkTankLaunchConfig(&req, "a pipeline room slot")
	if ae != nil {
		return "", &pipeline.ProjectGateError{Code: ae.Code, Message: ae.Message}
	}
	return config, nil
}

// StopRoom closes a stopped run's stage room, then cancels each of that room's
// own running turns through the guarded generation/turn seam, so a newer
// private turn is never killed (TS-09.R54, FS-21.R42).
func (s *Server) StopRoom(ctx context.Context, roomID string) error {
	prior, err := s.stateStore.ReadThinkTank(roomID)
	if err != nil {
		return err
	}
	d, err := s.stateStore.ClosePipelineThinkTank(roomID)
	if err != nil {
		return err
	}
	if d.Room.Revision != prior.Room.Revision {
		s.publishThinkTankUpdate(d)
	}
	for _, a := range d.Running {
		if a.TurnID == "" {
			continue
		}
		if _, err := s.registry.CancelGuarded(ctx, a.AgentID, a.Generation, a.TurnID); err != nil && !errors.Is(err, runtime.ErrNoHandle) {
			return fmt.Errorf("cancel pipeline room turn: %w", err)
		}
	}
	for _, member := range d.Members {
		if !member.StopTeardown || member.LaunchGeneration == "" {
			continue
		}
		if member.SetupState == state.ThinkTankSetupLaunching ||
			(member.Role == state.ThinkTankRoleJudge && d.Room.JudgeStatus == state.ThinkTankJudgeLaunching) {
			continue
		}
		if err := s.stopRoomOwnedGeneration(ctx, member.AgentID, member.LaunchGeneration); err != nil {
			return fmt.Errorf("stop pipeline room agent %s: %w", member.AgentID, err)
		}
		if err := s.stateStore.ClearThinkTankStopTeardown(roomID, member.AgentID, member.LaunchGeneration); err != nil {
			return err
		}
	}
	return nil
}

// A room may finish launching after Stop. The lifecycle claim serializes this
// generation check with ordinary launch/resume, preserving later private work.
func (s *Server) stopRoomOwnedGeneration(ctx context.Context, agentID, generation string) error {
	if !s.claimLifecycle(agentID) {
		return errors.New("a lifecycle transition is already in progress")
	}
	defer s.releaseLifecycle(agentID)
	if s.registry.Generation(agentID) == "" {
		// After restart the registry no longer owns the old process. The
		// durable running generation distinguishes it from a later private
		// launch before the ordinary orphan-stop seam reaps it.
		running, err := s.stateStore.ReadRunning(agentID)
		if errors.Is(err, state.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if running.Generation == generation {
			return s.stopStageLocked(ctx, agentID)
		}
		return nil
	}
	_, err := s.registry.StopGuardedIdle(ctx, agentID, generation)
	if err != nil {
		return err
	}
	return nil
}

// ValidateStage checks the same chat role/project/backend/model boundary before
// a run snapshot is committed, without registering or starting a process.
func (s *Server) ValidateStage(_ context.Context, execution pipeline.StageExecution) error {
	if _, err := s.configStore.ReadRole(execution.Role); err != nil {
		return fmt.Errorf("unknown role %q", execution.Role)
	}
	project, err := s.configStore.ReadProject(execution.Project)
	if err != nil {
		return fmt.Errorf("unknown project %q", execution.Project)
	}
	if project.Archived {
		return fmt.Errorf("project is archived")
	}
	cwd, err := config.ExpandTilde(project.Cwd)
	if err != nil {
		return fmt.Errorf("invalid project directory")
	}
	// ValidateStage is the manager's read-only per-stage pre-flight, run before a
	// run snapshot is committed and possibly for stages that never execute, so it
	// must not create anything. A worktree project whose owned checkout is
	// currently missing still validates: the start path re-materializes it
	// through ensureWorktreeCheckout, or fails there with an actionable error
	// (FS-19.R7, TS-12.R4).
	if !isExistingDir(cwd) {
		if row, owned := s.ownedWorktree(execution.Project); !owned || !sameCheckoutPath(row.CheckoutPath, cwd) {
			return fmt.Errorf("project directory does not exist")
		}
	}
	backends, err := s.configStore.ReadBackends()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			backends = config.DefaultBackends()
		} else {
			return fmt.Errorf("backend catalog is unavailable")
		}
	}
	backend, ok := backends.Backends[execution.Backend]
	if !ok {
		return fmt.Errorf("unknown backend %q", execution.Backend)
	}
	model, ok := backend.Models[execution.Model]
	if !ok {
		return fmt.Errorf("unknown model %q", execution.Model)
	}
	if err := config.ValidateModelEffort(backend, model, execution.Effort); err != nil {
		return err
	}
	if err := config.ValidateModelFast(backend, model, execution.Fast); err != nil {
		return err
	}
	return nil
}

// LaunchStage uses the exact manual launch transaction with a pre-associated
// durable agent id, then starts the assignment as an ordinary user turn.
func (s *Server) LaunchStage(ctx context.Context, execution pipeline.StageExecution) error {
	// Launch mints the same agent-keyed registration as resume, so the claim
	// covers both Registry.Launch and the first assignment prompt. A concurrent
	// Stop otherwise reads the registry's launch sentinel as an already-stopped
	// agent and tears down this launch's registration (TS-01.R16, INV §4/§5).
	if !s.claimLifecycle(execution.AgentID) {
		return errors.New("a lifecycle transition is already in progress")
	}
	defer s.releaseLifecycle(execution.AgentID)

	if !s.registry.Owns(execution.AgentID) {
		if _, err := s.stateStore.ReadRunning(execution.AgentID); err == nil {
			if err := s.reapOrphanRuntime(execution.AgentID); err != nil {
				return err
			}
			s.teardownAgentRegistration(execution.AgentID)
		}
	}
	_, ae := s.launchAgent(ctx, launchRequest{
		Role: execution.Role, Project: execution.Project, Backend: execution.Backend,
		Model: execution.Model, Effort: execution.Effort, Fast: execution.Fast, Interface: "chat", Name: execution.AgentName,
		// The stage agent's own name is also its ordinary group label, so a
		// stage's agents land in one dashboard section (FS-14.R58, TS-09.R33).
		Group: execution.AgentName,
	}, launchOptions{AgentID: execution.AgentID, Generation: execution.Generation})
	if ae != nil {
		return errors.New(ae.Message)
	}
	if err := s.registry.SendPrompt(ctx, execution.AgentID, execution.Assignment); err != nil {
		// The claim is already held, so use the unclaimed core rather than
		// StopStage, which would reject its own claim.
		_ = s.stopStageLocked(ctx, execution.AgentID)
		return err
	}
	return nil
}

// ContinueStage reuses a live blocked agent or resumes its ordinary persisted
// session after restart, then submits the durable continuation assignment.
func (s *Server) ContinueStage(ctx context.Context, execution pipeline.StageExecution) error {
	// Resuming a stage agent mints a fresh registration, so it takes the shared
	// exclusive lifecycle claim (TS-01.R16, INV §4/§5) before any registration side
	// effect — acquireAgentStart is only a counting lease. Without it an explicit
	// stop/resume of the same agent could tear down or duplicate the registration
	// inside this resume window.
	if !s.claimLifecycle(execution.AgentID) {
		return errors.New("a lifecycle transition is already in progress")
	}
	defer s.releaseLifecycle(execution.AgentID)
	if ae := s.acquireAgentStart(execution.Project, execution.AgentID); ae != nil {
		return errors.New(ae.Message)
	}
	defer s.releaseAgentStart(execution.Project, execution.AgentID)
	if project, err := s.configStore.ReadProject(execution.Project); err != nil || project.Archived {
		return errors.New("project is archived")
	}
	if s.IsRunning(execution.AgentID) {
		return s.registry.SendPrompt(ctx, execution.AgentID, execution.Assignment)
	}
	if _, err := s.stateStore.ReadRunning(execution.AgentID); err == nil {
		if err := s.reapOrphanRuntime(execution.AgentID); err != nil {
			return err
		}
		s.teardownAgentRegistration(execution.AgentID)
	}
	agent, err := s.stateStore.ReadAgent(execution.AgentID)
	if err != nil {
		return err
	}
	snapshot, err := s.stateStore.ReadSession(execution.AgentID)
	if err != nil {
		return err
	}
	backends, err := s.configStore.ReadBackends()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			backends = config.DefaultBackends()
		} else {
			return err
		}
	}
	backend, ok := backends.Backends[execution.Backend]
	if !ok {
		return fmt.Errorf("unknown backend %q", execution.Backend)
	}
	model, ok := backend.Models[execution.Model]
	if !ok {
		return fmt.Errorf("unknown model %q", execution.Model)
	}
	agent.Backend = execution.Backend
	agent.Model = execution.Model
	agent.Effort = execution.Effort
	agent.Fast = execution.Fast
	agent.Interface = "chat"
	spec, ae := s.composeResumeSpecContext(ctx, agent, snapshot, backend, model, execution.Generation, nil)
	if ae != nil {
		return errors.New(ae.Message)
	}
	handle, err := s.registry.Resume(ctx, spec)
	if err != nil {
		s.teardownAgentRegistration(agent.AgentID)
		return err
	}
	agent.Fast = handle.Fast
	if err := s.stateStore.WriteAgent(agent); err != nil {
		_ = s.stopStageLocked(ctx, agent.AgentID)
		return err
	}
	if err := s.registry.SendPrompt(ctx, agent.AgentID, execution.Assignment); err != nil {
		// Already holding the lifecycle claim here — tear down through the unclaimed
		// core rather than the public StopStage, which would fail to re-take it.
		_ = s.stopStageLocked(ctx, agent.AgentID)
		return err
	}
	return nil
}

// StopStage stops a pipeline stage agent under the shared exclusive lifecycle
// claim (TS-01.R16, INV §4/§5), so its Stop + teardown cannot race an explicit
// resume/stop or a switch of the same agent.
func (s *Server) StopStage(ctx context.Context, agentID string) error {
	if !s.claimLifecycle(agentID) {
		return errors.New("a lifecycle transition is already in progress")
	}
	defer s.releaseLifecycle(agentID)
	return s.stopStageLocked(ctx, agentID)
}

// stopStageLocked performs the stop + teardown. The caller must already hold the
// lifecycle claim for the agent (ContinueStage calls it from inside its own claim
// on a failed send).
func (s *Server) stopStageLocked(ctx context.Context, agentID string) error {
	if err := s.registry.Stop(ctx, agentID); err != nil {
		if !errors.Is(err, runtime.ErrNoHandle) {
			return err
		}
		if err := s.reapOrphanRuntime(agentID); err != nil {
			return err
		}
	}
	s.teardownAgentRegistration(agentID)
	return nil
}

func (s *Server) IsRunning(agentID string) bool {
	return s.registry != nil && s.registry.Owns(agentID)
}

func (s *Server) PublishPipelineUpdate(update pipeline.PipelineUpdate) {
	s.eventBus.Publish("pipeline_update", nil, update)
	// A committed room-backed stage is launched by the room engine; kick it
	// rather than waiting for the sweep (TS-09.R52).
	if update.State == "running" {
		s.kickThinkTanks()
	}
	// A run that has reached a terminal state has already registered its outcome
	// in the commit that made it terminal, so the arms waiting on it can be
	// released now. Evaluation reads that registration rather than this
	// notification, so a dropped or repeated publish changes nothing (TS-10.R3).
	if update.State == "completed" || update.State == "stopped" {
		s.evaluateSourceResult(state.SourcePipelineRun, update.RunID)
	}
}

// publishPipelineRun republishes a run whose durable state the server changed
// outside the pipeline manager.
func (s *Server) publishPipelineRun(run state.PipelineRunRecord) {
	s.PublishPipelineUpdate(pipeline.PipelineUpdate{
		RunID: run.RunID, DisplayName: run.DisplayName, Revision: run.Revision,
		State: run.State, CurrentStageID: run.CurrentStageID, CurrentAgentID: run.CurrentAgentID,
		AttentionReason: run.AttentionReason, FinalOutcome: run.FinalOutcome,
	})
}

func (s *Server) PublishPipelineProposalUpdate() {
	s.eventBus.Publish("pipeline_proposal_update", nil, map[string]any{})
}

func (s *Server) PublishPipelineNotification(update pipeline.PipelineUpdate, kind string) {
	s.eventBus.PublishPipelineNotification(update.RunID, update.DisplayName, update.CurrentAgentID, kind, update.AttentionReason, update.FinalOutcome)
}

var _ pipeline.Lifecycle = (*Server)(nil)
var _ pipeline.Publisher = (*Server)(nil)
