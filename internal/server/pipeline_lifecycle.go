package server

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/runtime"
)

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
	cwd, err := config.ExpandTilde(project.Cwd)
	if err != nil {
		return fmt.Errorf("invalid project directory")
	}
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		return fmt.Errorf("project directory does not exist")
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
	if _, ok := backend.Models[execution.Model]; !ok {
		return fmt.Errorf("unknown model %q", execution.Model)
	}
	return nil
}

// LaunchStage uses the exact manual launch transaction with a pre-associated
// durable agent id, then starts the assignment as an ordinary user turn.
func (s *Server) LaunchStage(ctx context.Context, execution pipeline.StageExecution) error {
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
		Model: execution.Model, Interface: "chat", Name: execution.AgentName,
	}, launchOptions{AgentID: execution.AgentID, Generation: execution.Generation})
	if ae != nil {
		return errors.New(ae.Message)
	}
	if err := s.registry.SendPrompt(ctx, execution.AgentID, execution.Assignment); err != nil {
		_ = s.StopStage(ctx, execution.AgentID)
		return err
	}
	return nil
}

// ContinueStage reuses a live blocked agent or resumes its ordinary persisted
// session after restart, then submits the durable continuation assignment.
func (s *Server) ContinueStage(ctx context.Context, execution pipeline.StageExecution) error {
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
		_ = s.StopStage(ctx, agent.AgentID)
		return err
	}
	return nil
}

func (s *Server) StopStage(ctx context.Context, agentID string) error {
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
}

func (s *Server) PublishPipelineNotification(update pipeline.PipelineUpdate, kind string) {
	s.eventBus.PublishPipelineNotification(update.RunID, update.CurrentAgentID, kind, update.AttentionReason)
}

var _ pipeline.Lifecycle = (*Server)(nil)
var _ pipeline.Publisher = (*Server)(nil)
