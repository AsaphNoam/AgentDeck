package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
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
		Model: execution.Model, Effort: execution.Effort, Interface: "chat", Name: execution.AgentName,
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
	agent.Interface = "chat"
	spec, ae := s.composeResumeSpecWithGeneration(agent, snapshot, backend, model, execution.Generation)
	if ae != nil {
		return errors.New(ae.Message)
	}
	if _, err := s.registry.Resume(ctx, spec); err != nil {
		s.teardownAgentRegistration(agent.AgentID)
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
	// A run that has reached a terminal state has already registered its outcome
	// in the commit that made it terminal, so the arms waiting on it can be
	// released now. Evaluation reads that registration rather than this
	// notification, so a dropped or repeated publish changes nothing (TS-10.R3).
	if update.State == "completed" || update.State == "stopped" {
		s.evaluateSourceResult(state.SourcePipelineRun, update.RunID)
	}
}

func (s *Server) PublishPipelineProposalUpdate() {
	s.eventBus.Publish("pipeline_proposal_update", nil, map[string]any{})
}

func (s *Server) PublishPipelineNotification(update pipeline.PipelineUpdate, kind string) {
	s.eventBus.PublishPipelineNotification(update.RunID, update.DisplayName, update.CurrentAgentID, kind, update.AttentionReason, update.FinalOutcome)
}

var _ pipeline.Lifecycle = (*Server)(nil)
var _ pipeline.Publisher = (*Server)(nil)
