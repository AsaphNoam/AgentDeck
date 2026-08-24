package messaging

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// assignTask creates a task already assigned to agentID in the given state, the
// way the dispatcher's reservation and confirmation leave it.
func assignTask(t *testing.T, st *state.Store, agentID, name, instruction, taskState string) state.Task {
	t.Helper()
	id, err := st.NewTaskID()
	if err != nil {
		t.Fatalf("NewTaskID: %v", err)
	}
	task, err := st.CreateTask(state.Task{
		TaskID: id, Project: "my-app", DisplayName: name, Instruction: instruction,
		TargetKind: state.TargetAgent, TargetAgentID: agentID, CreatedByKind: "person",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if _, err := st.DB().Exec(`
UPDATE tasks SET assigned_agent_id = ?, assigned_generation = ?, runtime_claim = ?, state = ?
WHERE task_id = ?`, agentID, "gen-"+agentID, state.ClaimBorrowed, taskState, task.TaskID); err != nil {
		t.Fatalf("assign task: %v", err)
	}
	read, err := st.ReadTask(task.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	return read
}

// FS-16.R11 / TS-10.R13 — an assigned agent reads its instruction and its
// attached reference ids in one bounded call, without scanning the global
// direct-share list. Identity comes from the session token: the tool takes no
// task id, so no caller can name another agent's work.
func TestGetAssignedTaskAnswersTheCallersOwnAssignment(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, f.store, "a_other", "Nova", "reviewer", "my-app")
	f.srv.RegisterSession("tok-impl", "a_impl", "gen-a_impl")
	f.srv.RegisterSession("tok-other", "a_other", "gen-a_other")

	task := assignTask(t, f.store, "a_impl", "migrate the schema", "write migration 19", state.TaskRunning)
	if _, err := f.store.DB().Exec(`
INSERT INTO task_attachments(task_id, context_ref_id, label, description, created_at)
VALUES(?, ?, ?, ?, ?)`, task.TaskID, "cx_review", "the review thread", "what to fix",
		time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("attach: %v", err)
	}

	impl := connect(t, f.srv, "tok-impl")
	res, isErr := call(t, impl, "get_assigned_task", map[string]any{})
	if isErr || res["ok"] != true || res["assigned"] != true {
		t.Fatalf("get_assigned_task = %v", res)
	}
	if res["task_id"] != task.TaskID || res["instruction"] != "write migration 19" {
		t.Fatalf("assignment = %v, want the caller's own task", res)
	}
	attachments, ok := res["attachments"].([]any)
	if !ok || len(attachments) != 1 {
		t.Fatalf("attachments = %v, want one", res["attachments"])
	}
	first, _ := attachments[0].(map[string]any)
	if first["context_ref_id"] != "cx_review" || first["label"] != "the review thread" {
		t.Fatalf("attachment = %v, want the reference id with this task's own label", first)
	}

	// The other agent has no assignment, which is an honest answer rather than an
	// error, and it cannot see the first agent's task at all.
	other := connect(t, f.srv, "tok-other")
	res, isErr = call(t, other, "get_assigned_task", map[string]any{})
	if isErr || res["ok"] != true || res["assigned"] != false {
		t.Fatalf("unassigned caller = %v, want ok with assigned=false", res)
	}
	if _, present := res["instruction"]; present {
		t.Fatalf("an unassigned caller was told about %v", res)
	}
}

// TS-10.R4 — the assignment is readable while the task is still starting: the
// agent is told to read it during the very turn that confirms the start.
func TestGetAssignedTaskAnswersDuringAStart(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	f.srv.RegisterSession("tok-impl", "a_impl", "gen-a_impl")
	assignTask(t, f.store, "a_impl", "starting work", "begin now", state.TaskStarting)

	res, isErr := call(t, connect(t, f.srv, "tok-impl"), "get_assigned_task", map[string]any{})
	if isErr || res["assigned"] != true || res["state"] != state.TaskStarting {
		t.Fatalf("get_assigned_task during a start = %v", res)
	}
}

// TS-05.R14 — an unregistered token is not a caller, so a revoked session reads
// nothing.
func TestGetAssignedTaskRefusesAnUnknownSession(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	assignTask(t, f.store, "a_impl", "work", "do it", state.TaskRunning)

	res, isErr := call(t, connect(t, f.srv, "tok-nobody"), "get_assigned_task", map[string]any{})
	if !isErr || res["error"] != "session_unknown" {
		t.Fatalf("unknown session = %v, want session_unknown", res)
	}
}

// FS-16.R3, R20 / TS-10.R7 — an agent records the authoritative outcome for its
// own assignment. The vocabulary is the one a pipeline stage report accepts, the
// caller is the session token, and a second report is refused because a recorded
// result is immutable.
func TestReportTaskResultRecordsTheCallersOwnOutcome(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	f.srv.RegisterSession("tok-impl", "a_impl", "gen-a_impl")
	task := assignTask(t, f.store, "a_impl", "migrate", "write migration 19", state.TaskRunning)

	recorded := ""
	f.srv.SetTaskResultSink(func(taskID string) { recorded = taskID })

	impl := connect(t, f.srv, "tok-impl")
	res, isErr := call(t, impl, "report_task_result", map[string]any{
		"outcome": "success", "summary": "migration 19 is in",
	})
	if isErr || res["ok"] != true || res["outcome"] != state.OutcomeSuccess {
		t.Fatalf("report_task_result = %v", res)
	}
	if recorded != task.TaskID {
		t.Fatalf("arm evaluation was notified about %q, want %q", recorded, task.TaskID)
	}
	finished, err := f.store.ReadTask(task.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if finished.State != state.TaskFinished || finished.OutcomeSource != "agent" {
		t.Fatalf("task = %s recorded by %q, want finished by the agent", finished.State, finished.OutcomeSource)
	}
	// The claim is still held: the release waits for this turn to end, so the
	// reporting agent always receives the response above (TS-10.R19).
	if !finished.PendingRelease || finished.RuntimeClaim == "" {
		t.Fatalf("result released the runtime inside the tool call: %+v", finished)
	}
	// The outcome is registered for arms to read, keyed to the task.
	if result, err := f.store.ReadWorkResult(state.SourceTask, task.TaskID); err != nil ||
		result.Outcome != state.OutcomeSuccess {
		t.Fatalf("registered result = %+v, %v", result, err)
	}

	res, isErr = call(t, impl, "report_task_result", map[string]any{
		"outcome": "failure", "summary": "actually no",
	})
	if !isErr || res["error"] != "not_assigned" {
		t.Fatalf("second report = %v, want a refusal", res)
	}
}

// FS-16.R20 / TS-05.R14 — a caller cannot report for work it does not hold, and
// every rejection mutates nothing.
func TestReportTaskResultRefusesWorkTheCallerDoesNotHold(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, f.store, "a_other", "Nova", "reviewer", "my-app")
	f.srv.RegisterSession("tok-impl", "a_impl", "gen-a_impl")
	// A stale session: the same agent, an earlier generation than the assignment.
	f.srv.RegisterSession("tok-stale", "a_impl", "gen-old")
	f.srv.RegisterSession("tok-other", "a_other", "gen-a_other")
	task := assignTask(t, f.store, "a_impl", "migrate", "write migration 19", state.TaskRunning)

	cases := []struct {
		name, token, outcome, summary, wantErr string
	}{
		{"another agent", "tok-other", "success", "done", "not_assigned"},
		{"an earlier generation", "tok-stale", "success", "done", "not_assigned"},
		{"a host-only outcome", "tok-impl", "cancelled", "done", "invalid_outcome"},
		{"an outcome outside the vocabulary", "tok-impl", "finished", "done", "invalid_outcome"},
		{"no summary", "tok-impl", "success", "  ", "validation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, isErr := call(t, connect(t, f.srv, tc.token), "report_task_result", map[string]any{
				"outcome": tc.outcome, "summary": tc.summary,
			})
			if !isErr || res["error"] != tc.wantErr {
				t.Fatalf("report = %v, want %s", res, tc.wantErr)
			}
		})
	}
	unchanged, err := f.store.ReadTask(task.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if unchanged.State != state.TaskRunning || unchanged.Outcome != "" {
		t.Fatalf("a refused report changed the task: %+v", unchanged)
	}
	if _, err := f.store.ReadWorkResult(state.SourceTask, task.TaskID); !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("a refused report registered a result: %v", err)
	}
}

// stubTaskControl records what the tools hand the control plane, so the tool's
// own contract — identity, resolution, shape — is tested without a server.
type stubTaskControl struct {
	created   AgentTaskRequest
	cancelled struct{ taskID, creator string }
	err       error
}

func (s *stubTaskControl) CreateAgentTask(req AgentTaskRequest) (state.Task, error) {
	s.created = req
	if s.err != nil {
		return state.Task{}, s.err
	}
	return state.Task{TaskID: "tk_new", State: state.TaskArmed}, nil
}

func (s *stubTaskControl) CancelAgentTask(taskID, creatorAgentID string) (state.Task, error) {
	s.cancelled.taskID, s.cancelled.creator = taskID, creatorAgentID
	if s.err != nil {
		return state.Task{}, s.err
	}
	return state.Task{TaskID: taskID, State: state.TaskFinished, Outcome: state.OutcomeCancelled}, nil
}

// FS-16.R12, R24 / TS-05.R17 — an agent creates work without a person in the
// loop. The creator, its generation, and the project come from the session
// token, and the target is the friendly selector coordination already resolves.
func TestCreateTaskDerivesItsCreatorAndResolvesItsTarget(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	liveAgent(t, f.store, "a_rev", "Nova", "reviewer", "my-app")
	f.srv.RegisterSession("tok-impl", "a_impl", "gen-a_impl")
	f.srv.SetAddressableAgents(func() ([]state.LiveAgent, error) {
		return f.store.AddressableAgents()
	})
	control := &stubTaskControl{}
	f.srv.SetTaskControl(control)

	impl := connect(t, f.srv, "tok-impl")
	res, isErr := call(t, impl, "create_task", map[string]any{
		"display_name": "review the migration", "instruction": "check migration 19",
		"to": "reviewer@my-app",
		"arms": []map[string]any{{
			"kind": "work_result", "source_kind": "task", "source_id": "tk_first",
			"satisfying_outcomes": []string{"success"},
		}},
	})
	if isErr || res["ok"] != true || res["task_id"] != "tk_new" {
		t.Fatalf("create_task = %v", res)
	}
	got := control.created
	if got.CreatorAgentID != "a_impl" || got.CreatorGeneration != "gen-a_impl" {
		t.Fatalf("creator = %q/%q, want the session's", got.CreatorAgentID, got.CreatorGeneration)
	}
	if got.Project != "my-app" {
		t.Fatalf("project = %q, want the creator's own", got.Project)
	}
	if got.TargetAgentID != "a_rev" {
		t.Fatalf("target = %q, want the resolved reviewer", got.TargetAgentID)
	}
	if len(got.Arms) != 1 || got.Arms[0].SourceID != "tk_first" {
		t.Fatalf("arms = %+v", got.Arms)
	}

	res, isErr = call(t, impl, "create_task", map[string]any{
		"display_name": "x", "instruction": "x", "to": "nobody@my-app",
	})
	if !isErr || res["error"] != "recipient_not_found" {
		t.Fatalf("unknown target = %v", res)
	}
}

// FS-16.R24 / TS-05.R14 — cancel authority is the durably recorded creator, and
// a task the caller did not create is refused with the same answer an unknown
// task gets, so no caller can probe for work it does not own.
func TestCancelTaskAsksTheControlPlaneWhoCreatedIt(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_impl", "Atlas", "implementer", "my-app")
	f.srv.RegisterSession("tok-impl", "a_impl", "gen-a_impl")
	control := &stubTaskControl{}
	f.srv.SetTaskControl(control)

	impl := connect(t, f.srv, "tok-impl")
	res, isErr := call(t, impl, "cancel_task", map[string]any{"task_id": "tk_mine"})
	if isErr || res["outcome"] != state.OutcomeCancelled {
		t.Fatalf("cancel_task = %v", res)
	}
	if control.cancelled.taskID != "tk_mine" || control.cancelled.creator != "a_impl" {
		t.Fatalf("cancel asked about %+v, want the caller's own identity", control.cancelled)
	}

	control.err = &ToolError{Code: "not_creator", Message: "No such task."}
	res, isErr = call(t, impl, "cancel_task", map[string]any{"task_id": "tk_theirs"})
	if !isErr || res["error"] != "not_creator" || res["message"] != "No such task." {
		t.Fatalf("cancelling another creator's task = %v", res)
	}
}

// FS-16.R10, R11 / TS-10.R12 — an assignee reads a task's attached reference
// through a work-derived route: no direct grant is created, the route survives
// the task finishing, it does not depend on the global share list, and deleting
// the task ends it while leaving the canonical reference intact.
func TestAnAssigneeReadsAttachedContextWithoutAGrant(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_owner", "Atlas", "implementer", "my-app")
	liveAgent(t, f.store, "a_worker", "Nova", "reviewer", "my-app")
	f.transcriptEvents(t, "a_owner",
		contextEvent(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "assess the migration"}),
		contextEvent(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "the migration is forward-only"}),
	)
	f.srv.RegisterSession("tok-owner", "a_owner", "gen-owner")
	f.srv.RegisterSession("tok-worker", "a_worker", "gen-a_worker")

	// The owner shares with itself only to mint a canonical reference; the worker
	// is deliberately given no grant.
	owner := connect(t, f.srv, "tok-owner")
	shared, isErr := call(t, owner, "share_context", map[string]any{
		"to": "implementer@my-app", "source": "current_turn", "label": "migration analysis",
	})
	if isErr || shared["ok"] != true {
		t.Fatalf("share_context: %v", shared)
	}
	refID, _ := shared["context_ref_id"].(string)

	worker := connect(t, f.srv, "tok-worker")
	if res, isErr := call(t, worker, "read_context_link", map[string]any{"context_ref_id": refID}); !isErr {
		t.Fatalf("an unrelated agent read the reference: %v", res)
	}

	task := assignTask(t, f.store, "a_worker", "review", "read the analysis", state.TaskRunning)
	if _, err := f.store.DB().Exec(`UPDATE tasks SET started_at = ? WHERE task_id = ?`,
		time.Now().UTC().Format(time.RFC3339), task.TaskID); err != nil {
		t.Fatalf("confirm start: %v", err)
	}
	if err := f.store.AttachTaskContext(task.TaskID, []state.TaskAttachment{
		{ContextRefID: refID, Label: "what to review"},
	}); err != nil {
		t.Fatalf("AttachTaskContext: %v", err)
	}

	res, isErr := call(t, worker, "read_context_link", map[string]any{"context_ref_id": refID})
	if isErr || res["ok"] == false {
		t.Fatalf("assignee read = %v", res)
	}
	if text, _ := res["text"].(string); !strings.Contains(text, "forward-only") {
		t.Fatalf("read text = %q, want the shared span", text)
	}
	// The attachment created no grant, so the worker's own list stays empty.
	list, isErr := call(t, worker, "list_context_links", nil)
	if isErr || len(list["links"].([]any)) != 0 {
		t.Fatalf("attachment appeared on the global share list: %v", list)
	}

	// A terminal outcome does not revoke the route.
	if _, err := f.store.DB().Exec(`UPDATE tasks SET state = ? WHERE task_id = ?`,
		state.TaskFinished, task.TaskID); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if res, isErr := call(t, worker, "read_context_link", map[string]any{"context_ref_id": refID}); isErr {
		t.Fatalf("finishing the task revoked its route: %v", res)
	}

	// Deleting the task ends the route and leaves the reference itself alone.
	if _, err := f.store.DeleteTask(task.TaskID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if res, isErr := call(t, worker, "read_context_link", map[string]any{"context_ref_id": refID}); !isErr {
		t.Fatalf("the route outlived its task: %v", res)
	}
	if res, isErr := call(t, owner, "read_context_link", map[string]any{"context_ref_id": refID}); isErr {
		t.Fatalf("deleting a task disturbed the canonical reference: %v", res)
	}
}
