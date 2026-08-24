package messaging

import (
	"errors"
	"testing"
	"time"

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
