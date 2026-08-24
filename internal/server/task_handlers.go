package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

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
	created, err := s.stateStore.CreateTask(task)
	if err != nil {
		writeTaskError(w, err)
		return
	}
	if len(req.Attachments) > 0 {
		if err := s.stateStore.AttachTaskContext(created.TaskID, taskAttachments(req.Attachments)); err != nil {
			writeTaskError(w, err)
			return
		}
	}
	s.publishTaskUpdate(created)
	writeJSON(w, http.StatusCreated, s.taskDetail(created))
}

// composeTask validates everything that is not a graph property; the graph
// checks themselves run inside the insert transaction, where a concurrent writer
// cannot slip past them (TS-10.R9).
func (s *Server) composeTask(req createTaskRequest) (state.Task, *runtime.APIError) {
	project := strings.TrimSpace(req.Project)
	if project == "" {
		return state.Task{}, apiError(runtime.CodeValidation, "project is required")
	}
	if _, err := s.configStore.ReadProject(project); err != nil {
		return state.Task{}, apiError(runtime.CodeNotFound, "unknown project: "+project)
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

	task := state.Task{
		Project: project, DisplayName: name, Instruction: instruction,
		TargetKind: req.TargetKind, CreatedByKind: "person",
	}
	switch req.TargetKind {
	case state.TargetAgent:
		agent, err := s.stateStore.ReadAgent(req.TargetAgentID)
		if err != nil {
			return state.Task{}, apiError(runtime.CodeNotFound, "unknown target agent")
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
		if _, err := s.configStore.ReadRole(req.Role); err != nil {
			return state.Task{}, apiError(runtime.CodeNotFound, "unknown role: "+req.Role)
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
		switch arm.Kind {
		case state.ArmSignal:
			name := strings.TrimSpace(arm.SignalName)
			if name == "" || utf8.RuneCountInString(name) > maxSignalNameRunes {
				return nil, apiError(runtime.CodeValidation, "signal_name is required and bounded")
			}
			arms = append(arms, state.TaskArm{Kind: state.ArmSignal, SignalName: name})
		case state.ArmWorkResult:
			if arm.SourceKind != state.SourceTask && arm.SourceKind != state.SourcePipelineRun {
				return nil, apiError(runtime.CodeValidation, "source_kind must be task or pipeline_run")
			}
			if strings.TrimSpace(arm.SourceID) == "" {
				return nil, apiError(runtime.CodeValidation, "source_id is required")
			}
			if len(arm.SatisfyingOutcomes) == 0 {
				return nil, apiError(runtime.CodeValidation, "satisfying_outcomes is required")
			}
			for _, outcome := range arm.SatisfyingOutcomes {
				if !state.AgentReportableOutcome(outcome) && outcome != state.OutcomeCancelled {
					return nil, apiError(runtime.CodeValidation, "unknown outcome: "+outcome)
				}
			}
			arms = append(arms, state.TaskArm{
				Kind: state.ArmWorkResult, SourceKind: arm.SourceKind, SourceID: arm.SourceID,
				SatisfyingOutcomes: arm.SatisfyingOutcomes,
			})
		default:
			return nil, apiError(runtime.CodeValidation, "arm kind must be work_result or signal")
		}
	}
	return arms, nil
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

// handleTasks implements GET /api/tasks?project= (TS-03.R28, FS-16.R14).
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeAPIError(w, apiError(runtime.CodeValidation, "project is required"))
		return
	}
	tasks, err := s.stateStore.ListTasks(project)
	if err != nil {
		writeTaskError(w, err)
		return
	}
	details := make([]taskDetailResponse, 0, len(tasks))
	for _, task := range tasks {
		details = append(details, s.taskDetail(task))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": details})
}

// handleTaskDetail implements GET /api/tasks/{id}.
func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	task, err := s.stateStore.ReadTask(r.PathValue("id"))
	if err != nil {
		writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.taskDetail(task))
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
		writeTaskError(w, err)
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

func (s *Server) taskDetail(task state.Task) taskDetailResponse {
	attachments, err := s.stateStore.ListTaskAttachments(task.TaskID)
	if err != nil {
		s.log.Debug("list task attachments failed", "task", task.TaskID, "err", err)
		attachments = []state.TaskAttachment{}
	}
	return taskDetailResponse{Task: task, Attachments: attachments}
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
func writeTaskError(w http.ResponseWriter, err error) {
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
	default:
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
	}
}
