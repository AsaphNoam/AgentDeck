package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/contextref"
	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// Bounded authoring limits for dependent work (FS-16.R15). They exist so one
// request cannot create a graph or an assignment the rest of the plane has to
// carry forever, not to express a product opinion about how work is organized.
const (
	maxTaskNameRunes        = 200
	maxTaskInstructionRunes = 16000
	maxTaskArms             = 32
	maxTaskAttachments      = 32
	maxSignalNameRunes      = 200
)

type createTaskRequest struct {
	Project     string `json:"project"`
	DisplayName string `json:"display_name"`
	Instruction string `json:"instruction"`

	TargetKind    string `json:"target_kind"`
	TargetAgentID string `json:"target_agent_id,omitempty"`
	Role          string `json:"role,omitempty"`
	Backend       string `json:"backend,omitempty"`
	Model         string `json:"model,omitempty"`

	Arms        []createArmRequest        `json:"arms,omitempty"`
	Attachments []createAttachmentRequest `json:"attachments,omitempty"`
}

type createArmRequest struct {
	Kind               string   `json:"kind"`
	SourceKind         string   `json:"source_kind,omitempty"`
	SourceID           string   `json:"source_id,omitempty"`
	SatisfyingOutcomes []string `json:"satisfying_outcomes,omitempty"`
	SignalName         string   `json:"signal_name,omitempty"`
}

type createAttachmentRequest struct {
	ContextRefID string `json:"context_ref_id"`
	Label        string `json:"label,omitempty"`
	Description  string `json:"description,omitempty"`
}

// handleCreateTask implements POST /api/tasks (TS-03.R28, FS-16.R12). A person
// creating work over the local API is the creator of record; an agent creating
// one does it through its own scoped tool, never by naming itself here
// (FS-16.R24).
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	task, ae := s.composeTask(req)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}
	created, err := s.stateStore.CreateTaskWithAttachments(task, taskAttachments(req.Attachments), "")
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	s.publishTaskUpdate(created)
	detail, err := s.taskDetail(created)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

// composeTask validates everything that is not a graph property; the graph
// checks themselves run inside the insert transaction, where a concurrent writer
// cannot slip past them (TS-10.R9).
func (s *Server) composeTask(req createTaskRequest) (state.Task, *runtime.APIError) {
	project := strings.TrimSpace(req.Project)
	if project == "" {
		return state.Task{}, apiError(runtime.CodeValidation, "project is required")
	}
	if _, err := s.configStore.ReadProject(project); errors.Is(err, config.ErrNotFound) {
		return state.Task{}, apiError(runtime.CodeNotFound, "unknown project: "+project)
	} else if err != nil {
		s.log.Error("read task project", "project", project, "err", err)
		return state.Task{}, apiError(runtime.CodeInternal, "The task operation could not be completed.")
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" || utf8.RuneCountInString(name) > maxTaskNameRunes {
		return state.Task{}, apiError(runtime.CodeValidation, "display_name is required and bounded")
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" || utf8.RuneCountInString(instruction) > maxTaskInstructionRunes {
		return state.Task{}, apiError(runtime.CodeValidation, "instruction is required and bounded")
	}
	if len(req.Arms) > maxTaskArms {
		return state.Task{}, apiError(runtime.CodeValidation, "too many prerequisites")
	}
	if len(req.Attachments) > maxTaskAttachments {
		return state.Task{}, apiError(runtime.CodeValidation, "too many attachments")
	}
	if ae := validateTaskAttachments(req.Attachments); ae != nil {
		return state.Task{}, ae
	}

	task := state.Task{
		Project: project, DisplayName: name, Instruction: instruction,
		TargetKind: req.TargetKind, CreatedByKind: "person",
	}
	switch req.TargetKind {
	case state.TargetAgent:
		agent, err := s.stateStore.ReadAgent(req.TargetAgentID)
		if errors.Is(err, state.ErrNotFound) {
			return state.Task{}, apiError(runtime.CodeNotFound, "unknown target agent")
		} else if err != nil {
			s.log.Error("read task target agent", "agent", req.TargetAgentID, "err", err)
			return state.Task{}, apiError(runtime.CodeInternal, "The task operation could not be completed.")
		}
		// A target that could never execute the work is rejected at authoring time
		// rather than parked later (FS-16.R2, R15).
		if agent.Project != project {
			return state.Task{}, apiError(runtime.CodeValidation, "target agent is in another project")
		}
		if agent.Interface != "chat" {
			return state.Task{}, apiError(runtime.CodeValidation, "target agent runs a terminal interface and cannot execute work")
		}
		if agent.Archived {
			return state.Task{}, apiError(runtime.CodeValidation, "target agent is archived")
		}
		task.TargetAgentID = agent.AgentID
	case state.TargetLaunch:
		if strings.TrimSpace(req.Role) == "" {
			return state.Task{}, apiError(runtime.CodeValidation, "role is required for a launch target")
		}
		if _, err := s.configStore.ReadRole(req.Role); errors.Is(err, config.ErrNotFound) {
			return state.Task{}, apiError(runtime.CodeNotFound, "unknown role: "+req.Role)
		} else if err != nil {
			s.log.Error("read task role", "role", req.Role, "err", err)
			return state.Task{}, apiError(runtime.CodeInternal, "The task operation could not be completed.")
		}
		task.Role, task.Backend, task.Model = req.Role, req.Backend, req.Model
	default:
		return state.Task{}, apiError(runtime.CodeValidation, "target_kind must be agent or launch")
	}

	arms, ae := composeTaskArms(req.Arms)
	if ae != nil {
		return state.Task{}, ae
	}
	task.Arms = arms

	id, err := s.stateStore.NewTaskID()
	if err != nil {
		return state.Task{}, apiError(runtime.CodeInternal, "mint task id: "+err.Error())
	}
	task.TaskID = id
	return task, nil
}

func composeTaskArms(requested []createArmRequest) ([]state.TaskArm, *runtime.APIError) {
	arms := make([]state.TaskArm, 0, len(requested))
	for _, arm := range requested {
		arms = append(arms, state.TaskArm{
			Kind: arm.Kind, SourceKind: arm.SourceKind, SourceID: arm.SourceID,
			SatisfyingOutcomes: arm.SatisfyingOutcomes, SignalName: strings.TrimSpace(arm.SignalName),
		})
	}
	if ae := validateTaskArmSet(arms); ae != nil {
		return nil, ae
	}
	return arms, nil
}

// validateTaskArmSet is the one check of an arm set's shape and vocabulary,
// shared by the HTTP surface and the agent-facing tool so the two cannot accept
// different graphs (INV §2). The graph properties themselves — cycles, unknown
// and cross-project sources — are checked inside the write transaction.
func validateTaskArmSet(arms []state.TaskArm) *runtime.APIError {
	if len(arms) > maxTaskArms {
		return apiError(runtime.CodeValidation, "too many prerequisites")
	}
	for _, arm := range arms {
		switch arm.Kind {
		case state.ArmSignal:
			if arm.SignalName == "" || utf8.RuneCountInString(arm.SignalName) > maxSignalNameRunes {
				return apiError(runtime.CodeValidation, "signal_name is required and bounded")
			}
		case state.ArmWorkResult:
			if arm.SourceKind != state.SourceTask && arm.SourceKind != state.SourcePipelineRun {
				return apiError(runtime.CodeValidation, "source_kind must be task or pipeline_run")
			}
			if strings.TrimSpace(arm.SourceID) == "" {
				return apiError(runtime.CodeValidation, "source_id is required")
			}
			if len(arm.SatisfyingOutcomes) == 0 {
				return apiError(runtime.CodeValidation, "satisfying_outcomes is required")
			}
			for _, outcome := range arm.SatisfyingOutcomes {
				if !state.AgentReportableOutcome(outcome) && outcome != state.OutcomeCancelled {
					return apiError(runtime.CodeValidation, "unknown outcome: "+outcome)
				}
			}
		default:
			return apiError(runtime.CodeValidation, "arm kind must be work_result or signal")
		}
	}
	return nil
}

func taskAttachments(requested []createAttachmentRequest) []state.TaskAttachment {
	out := make([]state.TaskAttachment, 0, len(requested))
	for _, a := range requested {
		out = append(out, state.TaskAttachment{
			ContextRefID: a.ContextRefID, Label: a.Label, Description: a.Description,
		})
	}
	return out
}

func validateTaskAttachments(attachments []createAttachmentRequest) *runtime.APIError {
	for _, attachment := range attachments {
		if strings.TrimSpace(attachment.ContextRefID) == "" {
			return apiError(runtime.CodeValidation, "context_ref_id is required")
		}
		if utf8.RuneCountInString(attachment.Label) > contextref.MaxLabelRunes {
			return apiError(runtime.CodeValidation, "attachment label is too long")
		}
		if utf8.RuneCountInString(attachment.Description) > contextref.MaxDescriptionRunes {
			return apiError(runtime.CodeValidation, "attachment description is too long")
		}
	}
	return nil
}

// handleTasks implements GET /api/tasks?project= (TS-03.R28, FS-16.R14).
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeAPIError(w, apiError(runtime.CodeValidation, "project is required"))
		return
	}
	tasks, err := s.stateStore.ListTasks(project)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	details := make([]taskDetailResponse, 0, len(tasks))
	for _, task := range tasks {
		detail, err := s.taskDetail(task)
		if err != nil {
			s.writeTaskError(w, err)
			return
		}
		details = append(details, detail)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": details})
}

// handleTaskDetail implements GET /api/tasks/{id}.
func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	task, err := s.stateStore.ReadTask(r.PathValue("id"))
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	detail, err := s.taskDetail(task)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

type fireSignalRequest struct {
	Project string `json:"project"`
	Name    string `json:"name"`
}

// handleFireSignal implements POST /api/signals (FS-16.R9). A signal is not a
// stored object: firing a name no arm is waiting on succeeds and changes
// nothing, and firing one outside the caller's project reaches nothing.
func (s *Server) handleFireSignal(w http.ResponseWriter, r *http.Request) {
	var req fireSignalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if req.Project == "" || name == "" || utf8.RuneCountInString(name) > maxSignalNameRunes {
		writeAPIError(w, apiError(runtime.CodeValidation, "project and a bounded signal name are required"))
		return
	}
	if _, err := s.configStore.ReadProject(req.Project); err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "unknown project: "+req.Project))
		return
	}
	released, err := s.stateStore.FireSignal(req.Project, name)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	for _, task := range released {
		s.publishTaskUpdate(task)
	}
	writeJSON(w, http.StatusOK, map[string]any{"released": len(released)})
}

type taskDetailResponse struct {
	state.Task
	Attachments []state.TaskAttachment `json:"attachments"`
}

func (s *Server) taskDetail(task state.Task) (taskDetailResponse, error) {
	attachments, err := s.stateStore.ListTaskAttachments(task.TaskID)
	if err != nil {
		return taskDetailResponse{}, err
	}
	return taskDetailResponse{Task: task, Attachments: attachments}, nil
}

// publishTaskUpdate publishes after the authoritative commit, never before, and
// carries only the bounded fields a client needs to decide whether to refetch
// (TS-10.R11, TS-03.R28).
func (s *Server) publishTaskUpdate(task state.Task) {
	s.eventBus.Publish("task_update", nil, map[string]any{
		"task_id":          task.TaskID,
		"revision":         task.Revision,
		"state":            task.State,
		"outcome":          task.Outcome,
		"attention_reason": task.AttentionReason,
	})
}

// writeTaskError maps the state layer's typed refusals onto the shared envelope
// (TS-03.R3, FS-16.R20).
func (s *Server) writeTaskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, state.ErrNotFound):
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such task"))
	case errors.Is(err, state.ErrTaskCycle):
		ae := apiError(runtime.CodeValidation, err.Error())
		ae.Details = map[string]any{"code": "dependency_cycle"}
		writeAPIError(w, ae)
	case errors.Is(err, state.ErrTaskArmSource):
		writeAPIError(w, apiError(runtime.CodeValidation, err.Error()))
	case errors.Is(err, state.ErrTaskConflict):
		writeAPIError(w, apiError(runtime.CodeConflict, "the task changed while this request was in flight"))
	case errors.Is(err, state.ErrTaskHoldsRuntime):
		ae := apiError(runtime.CodeConflict, err.Error())
		ae.Details = map[string]any{"code": "task_holds_runtime"}
		writeAPIError(w, ae)
	case errors.Is(err, state.ErrRetryRequiresRearm):
		ae := apiError(runtime.CodeValidation, err.Error())
		ae.Details = map[string]any{"code": "retry_requires_rearm"}
		writeAPIError(w, ae)
	case errors.Is(err, state.ErrTaskNotRetryable), errors.Is(err, state.ErrTaskNotRearmable),
		errors.Is(err, state.ErrTaskNotReportable):
		ae := apiError(runtime.CodeValidation, err.Error())
		ae.Details = map[string]any{"code": "invalid_state"}
		writeAPIError(w, ae)
	case errors.Is(err, state.ErrInvalidOutcome):
		ae := apiError(runtime.CodeValidation, err.Error())
		ae.Details = map[string]any{"code": "invalid_outcome"}
		writeAPIError(w, ae)
	case errors.Is(err, state.ErrInvalidReportFields):
		writeAPIError(w, apiError(runtime.CodeValidation, err.Error()))
	default:
		s.log.Error("task API operation failed", "err", err)
		writeAPIError(w, apiError(runtime.CodeInternal, "The task operation could not be completed."))
	}
}

// handleCancelTask implements POST /api/tasks/{id}/cancel (FS-16.R3, R20).
func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	s.taskStartMu.Lock()
	defer s.taskStartMu.Unlock()
	task, err := s.stateStore.CancelTask(r.PathValue("id"))
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	// A cancel has no reporting turn to wait for, so the stop follows its commit
	// immediately (TS-10.R19).
	s.finishInterruptedRelease(r.Context(), task)
	s.evaluateTaskResult(task.TaskID)
	s.publishTaskUpdate(task)
	detail, err := s.taskDetail(s.rereadTask(task))
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

type personResultRequest struct {
	Outcome string `json:"outcome"`
	Summary string `json:"summary"`
	Details string `json:"details,omitempty"`
}

// handleRecordTaskResult implements POST /api/tasks/{id}/result (FS-16.R22). It
// is the only non-cancelling way to resolve work whose agent went away, and it
// is marked person-recorded so it is never mistaken for an agent's own report.
func (s *Server) handleRecordTaskResult(w http.ResponseWriter, r *http.Request) {
	var req personResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	task, err := s.stateStore.RecordPersonTaskResult(r.PathValue("id"), state.TaskResult{
		Outcome: req.Outcome, Summary: req.Summary, Details: req.Details,
	})
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	s.finishInterruptedRelease(r.Context(), task)
	s.evaluateTaskResult(task.TaskID)
	s.publishTaskUpdate(task)
	detail, err := s.taskDetail(s.rereadTask(task))
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// handleRetryTask implements POST /api/tasks/{id}/retry (FS-16.R23, R25).
func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	existing, err := s.stateStore.ReadTask(r.PathValue("id"))
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	// Retry acts on the assignee the task already has, so an assignee that has
	// since gone is a typed refusal rather than a start that will fail three
	// times: the work is restarted by creating a new task (FS-16.R23).
	if ae := s.retryAssigneeGate(existing); ae != nil {
		writeAPIError(w, ae)
		return
	}
	task, err := s.stateStore.RetryTask(existing.TaskID)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	s.publishTaskUpdate(task)
	detail, err := s.taskDetail(task)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) retryAssigneeGate(task state.Task) *runtime.APIError {
	target := task.AssignedAgentID
	if target == "" {
		target = task.TargetAgentID
	}
	if target == "" {
		return nil
	}
	agent, err := s.stateStore.ReadAgent(target)
	if err != nil {
		ae := apiError(runtime.CodeValidation, "the agent this task was assigned to no longer exists")
		ae.Details = map[string]any{"code": "target_ineligible"}
		return ae
	}
	if agent.Archived {
		ae := apiError(runtime.CodeValidation, "the agent this task was assigned to is archived")
		ae.Details = map[string]any{"code": "target_ineligible"}
		return ae
	}
	return nil
}

type rearmRequest struct {
	Arms []createArmRequest `json:"arms"`
}

// handleRearmTask implements POST /api/tasks/{id}/rearm (FS-16.R23).
func (s *Server) handleRearmTask(w http.ResponseWriter, r *http.Request) {
	var req rearmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	if len(req.Arms) > maxTaskArms {
		writeAPIError(w, apiError(runtime.CodeValidation, "too many prerequisites"))
		return
	}
	arms, ae := composeTaskArms(req.Arms)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}
	task, err := s.stateStore.RearmTask(r.PathValue("id"), arms)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	s.publishTaskUpdate(task)
	detail, err := s.taskDetail(task)
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// handleDeleteTask implements DELETE /api/tasks/{id} (FS-16.R18).
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	parked, err := s.stateStore.DeleteTask(r.PathValue("id"))
	if err != nil {
		s.writeTaskError(w, err)
		return
	}
	for _, dependent := range parked {
		s.publishTaskUpdate(dependent)
		s.propagateTaskFailure(dependent)
	}
	w.WriteHeader(http.StatusNoContent)
}

// rereadTask re-reads a task after an effect that may have changed it, falling
// back to what the caller already holds.
func (s *Server) rereadTask(task state.Task) state.Task {
	if fresh, err := s.stateStore.ReadTask(task.TaskID); err == nil {
		return fresh
	}
	return task
}

// CreateAgentTask creates a task on behalf of a token-bound agent (FS-16.R12,
// R24, TS-10.R20). It shares every validation the HTTP surface uses; what
// differs is only where the facts come from. The creator, its generation, and
// the project are the session's, so no tool argument can name another creator
// or reach into another project.
func (s *Server) CreateAgentTask(req messaging.AgentTaskRequest) (state.Task, error) {
	task := state.Task{
		Project: req.Project, DisplayName: strings.TrimSpace(req.DisplayName),
		Instruction:   strings.TrimSpace(req.Instruction),
		Arms:          req.Arms,
		CreatedByKind: "agent", CreatedByAgentID: req.CreatorAgentID,
		CreatedByGeneration: req.CreatorGeneration,
	}
	if task.DisplayName == "" || utf8.RuneCountInString(task.DisplayName) > maxTaskNameRunes {
		return state.Task{}, &messaging.ToolError{Code: "validation", Message: "display_name is required and bounded"}
	}
	if task.Instruction == "" || utf8.RuneCountInString(task.Instruction) > maxTaskInstructionRunes {
		return state.Task{}, &messaging.ToolError{Code: "validation", Message: "instruction is required and bounded"}
	}
	if ae := validateTaskArmSet(task.Arms); ae != nil {
		return state.Task{}, &messaging.ToolError{Code: "validation", Message: ae.Message}
	}
	if req.TargetAgentID != "" {
		agent, err := s.stateStore.ReadAgent(req.TargetAgentID)
		if err != nil {
			return state.Task{}, &messaging.ToolError{Code: "target_ineligible", Message: "that agent cannot be assigned work"}
		}
		if agent.Project != req.Project || agent.Interface != "chat" || agent.Archived {
			return state.Task{}, &messaging.ToolError{Code: "target_ineligible", Message: "that agent cannot be assigned work"}
		}
		task.TargetKind, task.TargetAgentID = state.TargetAgent, agent.AgentID
	} else {
		role := strings.TrimSpace(req.Role)
		if role == "" {
			return state.Task{}, &messaging.ToolError{Code: "validation", Message: "name a target agent or a role to launch"}
		}
		if _, err := s.configStore.ReadRole(role); err != nil {
			return state.Task{}, &messaging.ToolError{Code: "validation", Message: "unknown role: " + role}
		}
		task.TargetKind, task.Role, task.Backend, task.Model = state.TargetLaunch, role, req.Backend, req.Model
	}
	if len(req.Attachments) > maxTaskAttachments {
		return state.Task{}, &messaging.ToolError{Code: "validation", Message: "too many attachments"}
	}
	for _, attachment := range req.Attachments {
		if utf8.RuneCountInString(attachment.Label) > contextref.MaxLabelRunes || utf8.RuneCountInString(attachment.Description) > contextref.MaxDescriptionRunes || strings.TrimSpace(attachment.ContextRefID) == "" {
			return state.Task{}, &messaging.ToolError{Code: "validation", Message: "task attachment is invalid"}
		}
	}
	// Attaching is not a way to reach context: a caller may only attach what it
	// can already read, and the attachment grants the assignee a work-derived
	// route rather than synthesizing a direct grant (TS-05.R17, FS-16.R10).
	for _, attachment := range req.Attachments {
		authorized, err := s.stateStore.ContextReadAuthorized(attachment.ContextRefID, req.CreatorAgentID)
		if err != nil {
			return state.Task{}, err
		}
		if !authorized {
			return state.Task{}, &messaging.ToolError{
				Code: "validation", Message: "you cannot read " + attachment.ContextRefID,
			}
		}
	}
	id, err := s.stateStore.NewTaskID()
	if err != nil {
		return state.Task{}, err
	}
	task.TaskID = id
	created, err := s.stateStore.CreateTaskWithAttachments(task, req.Attachments, req.CreatorAgentID)
	if err != nil {
		return state.Task{}, err
	}
	s.publishTaskUpdate(created)
	return created, nil
}

// CancelAgentTask cancels a task the calling agent created. Authority is the
// durably recorded creator id, so a stopped-and-resumed agent keeps it, and a
// task it did not create is refused with the same answer an unknown task gets:
// a caller must not be able to probe for work it does not own (FS-16.R24,
// TS-05.R14, R17).
func (s *Server) CancelAgentTask(taskID, creatorAgentID string) (state.Task, error) {
	s.taskStartMu.Lock()
	defer s.taskStartMu.Unlock()
	existing, err := s.stateStore.ReadTask(taskID)
	if err != nil {
		return state.Task{}, err
	}
	if existing.CreatedByKind != "agent" || existing.CreatedByAgentID != creatorAgentID {
		return state.Task{}, &messaging.ToolError{Code: "not_creator", Message: "No such task."}
	}
	task, err := s.stateStore.CancelTask(taskID)
	if err != nil {
		return state.Task{}, err
	}
	s.finishInterruptedRelease(context.Background(), task)
	s.evaluateTaskResult(task.TaskID)
	s.publishTaskUpdate(task)
	return s.rereadTask(task), nil
}
