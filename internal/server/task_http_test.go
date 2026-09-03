package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/state"
)

func decodeTask(t *testing.T, body []byte) state.Task {
	t.Helper()
	var task state.Task
	if err := json.Unmarshal(body, &task); err != nil {
		t.Fatalf("decode task: %v (%s)", err, body)
	}
	return task
}

// FS-16.R1, R5, R12 / TS-03.R28 — a person creates dependent work over the local
// API. A task with no arms is ready at once; one armed on another task waits.
func TestCreateAndReadTasksOverHTTP(t *testing.T) {
	_, ts := wakeTestServer(t)

	resp, body := post(t, ts.URL+"/api/tasks", map[string]any{
		"project": "tmpproj", "display_name": "first", "instruction": "do the first thing",
		"target_kind": "launch", "role": "impl",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d: %s", resp.StatusCode, body)
	}
	first := decodeTask(t, body)
	if first.State != state.TaskReady || first.CreatedByKind != "person" {
		t.Fatalf("created task = %s by %q, want ready and person-created", first.State, first.CreatedByKind)
	}

	resp, body = post(t, ts.URL+"/api/tasks", map[string]any{
		"project": "tmpproj", "display_name": "second", "instruction": "then the second",
		"target_kind": "launch", "role": "impl",
		"arms": []map[string]any{{
			"kind": "work_result", "source_kind": "task", "source_id": first.TaskID,
			"satisfying_outcomes": []string{"success"},
		}},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create dependent status = %d: %s", resp.StatusCode, body)
	}
	second := decodeTask(t, body)
	if second.State != state.TaskArmed || len(second.Arms) != 1 {
		t.Fatalf("dependent = %s with %d arms, want armed with one", second.State, len(second.Arms))
	}

	res, err := http.Get(ts.URL + "/api/tasks?project=tmpproj")
	if err != nil {
		t.Fatalf("GET tasks: %v", err)
	}
	defer res.Body.Close()
	var list struct {
		Tasks []taskDetailResponse `json:"tasks"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Tasks) != 2 {
		t.Fatalf("listed %d tasks, want 2", len(list.Tasks))
	}
	for _, task := range list.Tasks {
		if task.Attachments == nil {
			t.Fatalf("attachments serialized as null for %s", task.TaskID)
		}
	}

	res, err = http.Get(ts.URL + "/api/tasks/" + second.TaskID)
	if err != nil {
		t.Fatalf("GET task: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("detail status = %d", res.StatusCode)
	}
}

// FS-16.R15, R20 / TS-03.R3 — every invalid authoring request is a typed error
// in the shared envelope and creates nothing.
func TestCreateTaskRejectionsAreTypedAndAtomic(t *testing.T) {
	srv, ts := wakeTestServer(t)
	_, body := post(t, ts.URL+"/api/tasks", map[string]any{
		"project": "tmpproj", "display_name": "anchor", "instruction": "anchor",
		"target_kind": "launch", "role": "impl",
	})
	anchor := decodeTask(t, body)

	cases := []struct {
		name    string
		request map[string]any
		want    int
	}{
		{"no project", map[string]any{"display_name": "x", "instruction": "x", "target_kind": "launch", "role": "impl"}, 422},
		{"unknown project", map[string]any{"project": "nope", "display_name": "x", "instruction": "x", "target_kind": "launch", "role": "impl"}, 404},
		{"no instruction", map[string]any{"project": "tmpproj", "display_name": "x", "target_kind": "launch", "role": "impl"}, 422},
		{"unknown role", map[string]any{"project": "tmpproj", "display_name": "x", "instruction": "x", "target_kind": "launch", "role": "ghost"}, 404},
		{"unknown target kind", map[string]any{"project": "tmpproj", "display_name": "x", "instruction": "x", "target_kind": "whatever"}, 422},
		{"unknown target agent", map[string]any{"project": "tmpproj", "display_name": "x", "instruction": "x", "target_kind": "agent", "target_agent_id": "a_ghost"}, 404},
		{"unknown context attachment", map[string]any{"project": "tmpproj", "display_name": "x", "instruction": "x", "target_kind": "launch", "role": "impl", "attachments": []map[string]any{{"context_ref_id": "cx_ghost"}}}, 422},
		{"arm with no source", map[string]any{"project": "tmpproj", "display_name": "x", "instruction": "x", "target_kind": "launch", "role": "impl",
			"arms": []map[string]any{{"kind": "work_result", "source_kind": "task", "satisfying_outcomes": []string{"success"}}}}, 422},
		{"arm with an empty outcome set", map[string]any{"project": "tmpproj", "display_name": "x", "instruction": "x", "target_kind": "launch", "role": "impl",
			"arms": []map[string]any{{"kind": "work_result", "source_kind": "task", "source_id": anchor.TaskID}}}, 422},
		{"arm on an unknown task", map[string]any{"project": "tmpproj", "display_name": "x", "instruction": "x", "target_kind": "launch", "role": "impl",
			"arms": []map[string]any{{"kind": "work_result", "source_kind": "task", "source_id": "tk_ghost", "satisfying_outcomes": []string{"success"}}}}, 422},
		{"signal arm with no name", map[string]any{"project": "tmpproj", "display_name": "x", "instruction": "x", "target_kind": "launch", "role": "impl",
			"arms": []map[string]any{{"kind": "signal"}}}, 422},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := post(t, ts.URL+"/api/tasks", tc.request)
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d: %s", resp.StatusCode, tc.want, body)
			}
			if code := apiErrorCode(t, body); code == "" {
				t.Fatalf("rejection carried no typed code: %s", body)
			}
		})
	}
	tasks, err := srv.stateStore.ListTasks("tmpproj")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("rejected creates left %d tasks, want only the anchor", len(tasks))
	}
}

// FS-16.R9 — firing a signal releases every arm waiting on that name in that
// project at that moment, and firing an unwatched name succeeds and changes
// nothing.
func TestUnknownTaskAttachmentUsesContextWording(t *testing.T) {
	_, ts := wakeTestServer(t)
	resp, body := post(t, ts.URL+"/api/tasks", map[string]any{
		"project": "tmpproj", "display_name": "x", "instruction": "x",
		"target_kind": "launch", "role": "impl",
		"attachments": []map[string]any{{"context_ref_id": "cx_ghost"}},
	})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	response := string(body)
	if !strings.Contains(response, `"code":"context_not_found"`) ||
		!strings.Contains(response, `"message":"unknown context reference"`) || strings.Contains(response, "arm") {
		t.Fatalf("attachment refusal = %s", body)
	}
}

func TestFiringASignalReleasesItsArms(t *testing.T) {
	srv, ts := wakeTestServer(t)
	_, body := post(t, ts.URL+"/api/tasks", map[string]any{
		"project": "tmpproj", "display_name": "waiting on ci", "instruction": "ship it",
		"target_kind": "launch", "role": "impl",
		"arms": []map[string]any{{"kind": "signal", "signal_name": "ci-green"}},
	})
	task := decodeTask(t, body)
	if task.State != state.TaskArmed {
		t.Fatalf("signal-armed task = %s, want armed", task.State)
	}

	resp, body := post(t, ts.URL+"/api/signals", map[string]any{"project": "tmpproj", "name": "nobody-waits"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fire unwatched status = %d: %s", resp.StatusCode, body)
	}
	if still, err := srv.stateStore.ReadTask(task.TaskID); err != nil || still.State != state.TaskArmed {
		t.Fatalf("an unwatched signal changed %+v, %v", still, err)
	}

	resp, body = post(t, ts.URL+"/api/signals", map[string]any{"project": "tmpproj", "name": "ci-green"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fire status = %d: %s", resp.StatusCode, body)
	}
	released, err := srv.stateStore.ReadTask(task.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if released.State != state.TaskReady {
		t.Fatalf("task = %s after its signal fired, want ready", released.State)
	}

	resp, body = post(t, ts.URL+"/api/signals", map[string]any{"project": "ghost", "name": "ci-green"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("signal outside a known project = %d: %s", resp.StatusCode, body)
	}
}

// createTaskHTTP creates a task over the API and returns it.
func createTaskHTTP(t *testing.T, ts *httptest.Server, body map[string]any) state.Task {
	t.Helper()
	resp, raw := post(t, ts.URL+"/api/tasks", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d: %s", resp.StatusCode, raw)
	}
	return decodeTask(t, raw)
}

func launchTaskBody(name string) map[string]any {
	return map[string]any{
		"project": "tmpproj", "display_name": name, "instruction": "do " + name,
		"target_kind": "launch", "role": "impl",
	}
}

// FS-16.R3, R4 — cancelling is the host's own outcome. It resolves the work,
// releases the runtime the task brought up, and resolves dependents rather than
// leaving them waiting for a result that will never come.
func TestCancellingATaskFinishesItAndReleasesItsRuntime(t *testing.T) {
	srv, ts := wakeTestServer(t)
	task := createTaskHTTP(t, ts, launchTaskBody("cancel me"))
	dependent := createTaskHTTP(t, ts, map[string]any{
		"project": "tmpproj", "display_name": "after", "instruction": "after it succeeds",
		"target_kind": "launch", "role": "impl",
		"arms": []map[string]any{{
			"kind": "work_result", "source_kind": "task", "source_id": task.TaskID,
			"satisfying_outcomes": []string{"success"},
		}},
	})
	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)

	resp, body := post(t, ts.URL+"/api/tasks/"+task.TaskID+"/cancel", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel status = %d: %s", resp.StatusCode, body)
	}
	cancelled := decodeTask(t, body)
	if cancelled.State != state.TaskFinished || cancelled.Outcome != state.OutcomeCancelled {
		t.Fatalf("cancelled task = %s/%s", cancelled.State, cancelled.Outcome)
	}
	if cancelled.OutcomeSource != "" {
		t.Fatalf("cancel recorded a reporter %q; it is host-written", cancelled.OutcomeSource)
	}
	if !cancelled.PendingRelease && cancelled.RuntimeClaim != "" {
		t.Fatalf("cancel dropped the release intent while still holding a claim: %+v", cancelled)
	}
	// The stop follows the commit and cannot be part of it, so the response promises the
	// terminal state and an intent that is either already discharged or standing for
	// recovery to finish (FS-16.R4, TS-10.R19). A cancel that lands while the task's own
	// launch still holds the lifecycle claim gets its stop refused, which is why this
	// asserts the released state rather than reading it off the response: driving the
	// recovery backstop is a no-op once the cancel's own stop has succeeded.
	released := srv.rereadTask(state.Task{TaskID: task.TaskID})
	for deadline := time.Now().Add(15 * time.Second); released.PendingRelease; {
		if time.Now().After(deadline) {
			t.Fatalf("cancel left the release unfinished: %+v", released)
		}
		srv.finishInterruptedRelease(context.Background(), released)
		released = srv.rereadTask(state.Task{TaskID: task.TaskID})
		time.Sleep(20 * time.Millisecond)
	}
	if released.RuntimeClaim != "" {
		t.Fatalf("cancel completed the release but kept the claim: %+v", released)
	}
	waitRunning(t, srv, running.AssignedAgentID, false)

	parked, err := srv.stateStore.ReadTask(dependent.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if parked.State != state.TaskDependencyFailed {
		t.Fatalf("dependent = %s after its prerequisite was cancelled, want parked", parked.State)
	}

	// A finished task is terminal.
	resp, body = post(t, ts.URL+"/api/tasks/"+task.TaskID+"/cancel", nil)
	if resp.StatusCode != 422 {
		t.Fatalf("second cancel = %d: %s", resp.StatusCode, body)
	}
}

// FS-16.R22 — a person resolves work whose agent went away, and it is marked
// person-recorded. It is rejected in every state where an agent will report or
// where the work never ran.
func TestAPersonRecordsAResultOnlyWhereNoAgentWill(t *testing.T) {
	srv, ts := wakeTestServer(t)
	armed := createTaskHTTP(t, ts, map[string]any{
		"project": "tmpproj", "display_name": "armed", "instruction": "wait",
		"target_kind": "launch", "role": "impl",
		"arms": []map[string]any{{"kind": "signal", "signal_name": "never"}},
	})
	resp, body := post(t, ts.URL+"/api/tasks/"+armed.TaskID+"/result",
		map[string]any{"outcome": "success", "summary": "no"})
	if resp.StatusCode != 422 {
		t.Fatalf("result on an armed task = %d: %s", resp.StatusCode, body)
	}

	task := createTaskHTTP(t, ts, launchTaskBody("abandoned"))
	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)
	if resp, body := post(t, ts.URL+"/api/sessions/"+running.AssignedAgentID+"/stop", nil); resp.StatusCode != 200 {
		t.Fatalf("stop status = %d: %s", resp.StatusCode, body)
	}
	waitTaskState(t, srv, task.TaskID, state.TaskInterrupted)

	resp, body = post(t, ts.URL+"/api/tasks/"+task.TaskID+"/result",
		map[string]any{"outcome": "blocked", "summary": "needs a person"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("person result = %d: %s", resp.StatusCode, body)
	}
	finished := decodeTask(t, body)
	if finished.State != state.TaskFinished || finished.OutcomeSource != "person" {
		t.Fatalf("person result = %s by %q", finished.State, finished.OutcomeSource)
	}
	if result, err := srv.stateStore.ReadWorkResult(state.SourceTask, task.TaskID); err != nil ||
		result.Outcome != state.OutcomeBlocked {
		t.Fatalf("registered result = %+v, %v", result, err)
	}

	resp, body = post(t, ts.URL+"/api/tasks/"+task.TaskID+"/result",
		map[string]any{"outcome": "success", "summary": "changed my mind"})
	if resp.StatusCode != 422 {
		t.Fatalf("second person result = %d: %s", resp.StatusCode, body)
	}
	if apiErrorCode(t, body) == "" {
		t.Fatalf("rejection carried no code: %s", body)
	}
}

// FS-16.R23, R25 — retry and re-arm repair different things and say so. Retry on
// a task parked by an unsatisfiable arm is refused naming re-arm; re-arming it
// onto an already-satisfied prerequisite makes it ready and it starts.
func TestRetryAndRearmRepairDifferentThings(t *testing.T) {
	srv, ts := wakeTestServer(t)
	failing := createTaskHTTP(t, ts, launchTaskBody("will fail"))
	succeeding := createTaskHTTP(t, ts, launchTaskBody("will succeed"))
	dependent := createTaskHTTP(t, ts, map[string]any{
		"project": "tmpproj", "display_name": "dependent", "instruction": "after success",
		"target_kind": "launch", "role": "impl",
		"arms": []map[string]any{{
			"kind": "work_result", "source_kind": "task", "source_id": failing.TaskID,
			"satisfying_outcomes": []string{"success"},
		}},
	})

	// Both prerequisites run and record opposite outcomes.
	for _, prereq := range []struct {
		task    state.Task
		outcome string
	}{{failing, state.OutcomeFailure}, {succeeding, state.OutcomeSuccess}} {
		running := dispatchUntilTaskState(t, srv, prereq.task.TaskID, state.TaskRunning)
		if _, err := srv.stateStore.RecordAgentTaskResult(prereq.task.TaskID,
			running.AssignedAgentID, running.AssignedGeneration,
			state.TaskResult{Outcome: prereq.outcome, Summary: "done"}); err != nil {
			t.Fatalf("RecordAgentTaskResult: %v", err)
		}
		srv.evaluateTaskResult(prereq.task.TaskID)
		srv.dispatchTurnEnd(running.AssignedAgentID, running.AssignedGeneration)
	}
	parked, err := srv.stateStore.ReadTask(dependent.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if parked.State != state.TaskDependencyFailed {
		t.Fatalf("dependent = %s, want parked", parked.State)
	}

	resp, body := post(t, ts.URL+"/api/tasks/"+dependent.TaskID+"/retry", nil)
	if resp.StatusCode != 422 {
		t.Fatalf("retry on an unsatisfiable park = %d: %s", resp.StatusCode, body)
	}
	var envelope struct {
		Error struct {
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Error.Details["code"] != "retry_requires_rearm" {
		t.Fatalf("retry refusal = %v, want it to name re-arm", envelope.Error.Details)
	}

	resp, body = post(t, ts.URL+"/api/tasks/"+dependent.TaskID+"/rearm", map[string]any{
		"arms": []map[string]any{{
			"kind": "work_result", "source_kind": "task", "source_id": succeeding.TaskID,
			"satisfying_outcomes": []string{"success"},
		}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rearm = %d: %s", resp.StatusCode, body)
	}
	rearmed := decodeTask(t, body)
	if rearmed.State != state.TaskReady {
		t.Fatalf("rearmed onto a satisfied prerequisite = %s, want ready", rearmed.State)
	}
	srv.dispatchReadyTasks(context.Background())
	waitTaskState(t, srv, dependent.TaskID, state.TaskRunning)
}

// FS-16.R23, R25 — retry on an interrupted task restores the full allowance and
// runs it again on the assignee it already had, rather than minting a second one.
func TestRetryRunsAgainOnTheSameAssignee(t *testing.T) {
	srv, ts := wakeTestServer(t)
	task := createTaskHTTP(t, ts, launchTaskBody("interrupted"))
	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)
	first := running.AssignedAgentID
	deadline := time.Now().Add(2 * time.Second)
	for srv.lifecycleInFlight(first) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if srv.lifecycleInFlight(first) {
		t.Fatal("task start lifecycle claim did not settle")
	}

	if resp, body := post(t, ts.URL+"/api/sessions/"+first+"/stop", nil); resp.StatusCode != 200 {
		t.Fatalf("stop status = %d: %s", resp.StatusCode, body)
	}
	waitTaskState(t, srv, task.TaskID, state.TaskInterrupted)

	resp, body := post(t, ts.URL+"/api/tasks/"+task.TaskID+"/retry", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("retry = %d: %s", resp.StatusCode, body)
	}
	if retried := decodeTask(t, body); retried.State != state.TaskReady || retried.StartAttemptCount != 0 {
		t.Fatalf("retried task = %s after %d attempts, want ready with a full allowance",
			retried.State, retried.StartAttemptCount)
	}
	srv.dispatchReadyTasks(context.Background())
	again := waitTaskState(t, srv, task.TaskID, state.TaskRunning)
	if again.AssignedAgentID != first {
		t.Fatalf("retry ran on %q, want the assignee it already had (%q)", again.AssignedAgentID, first)
	}
	if again.AssignedGeneration == running.AssignedGeneration {
		t.Fatalf("retry reused generation %q, want a new one", again.AssignedGeneration)
	}
	if again.RuntimeClaim != state.ClaimWoke {
		t.Fatalf("retry claim = %q, want %q for an agent it woke", again.RuntimeClaim, state.ClaimWoke)
	}
}

// FS-16.R18 / TS-10.R16 — deletion is refused while a task still owns something
// live, removes only its own rows once it does not, and parks only a dependent
// whose arm was still waiting.
func TestDeletionIsRefusedWhileATaskOwnsARuntime(t *testing.T) {
	srv, ts := wakeTestServer(t)
	task := createTaskHTTP(t, ts, launchTaskBody("busy"))
	satisfied := createTaskHTTP(t, ts, launchTaskBody("finishes first"))
	srv.dispatchReadyTasks(context.Background())
	running := waitTaskState(t, srv, task.TaskID, state.TaskRunning)
	satisfiedRunning := waitTaskState(t, srv, satisfied.TaskID, state.TaskRunning)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/tasks/"+task.TaskID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("delete of a running task = %d, want 409", resp.StatusCode)
	}
	if _, err := srv.stateStore.ReadTask(task.TaskID); err != nil {
		t.Fatalf("a refused delete removed the task: %v", err)
	}

	// A dependent whose arm the prerequisite already satisfied is untouched by
	// that prerequisite's deletion; one still waiting is parked.
	if _, err := srv.stateStore.RecordAgentTaskResult(satisfied.TaskID,
		satisfiedRunning.AssignedAgentID, satisfiedRunning.AssignedGeneration,
		state.TaskResult{Outcome: state.OutcomeSuccess, Summary: "done"}); err != nil {
		t.Fatalf("RecordAgentTaskResult: %v", err)
	}
	srv.dispatchTurnEnd(satisfiedRunning.AssignedAgentID, satisfiedRunning.AssignedGeneration)
	settled := createTaskHTTP(t, ts, map[string]any{
		"project": "tmpproj", "display_name": "already satisfied", "instruction": "go",
		"target_kind": "launch", "role": "impl",
		"arms": []map[string]any{{
			"kind": "work_result", "source_kind": "task", "source_id": satisfied.TaskID,
			"satisfying_outcomes": []string{"success"},
		}},
	})
	if settled.State != state.TaskReady {
		t.Fatalf("a task armed on an already-satisfied prerequisite = %s, want ready", settled.State)
	}
	waiting := createTaskHTTP(t, ts, map[string]any{
		"project": "tmpproj", "display_name": "still waiting", "instruction": "go",
		"target_kind": "launch", "role": "impl",
		"arms": []map[string]any{{
			"kind": "work_result", "source_kind": "task", "source_id": task.TaskID,
			"satisfying_outcomes": []string{"success"},
		}},
	})

	if _, err := srv.stateStore.CancelTask(task.TaskID); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	srv.finishInterruptedRelease(context.Background(), srv.rereadTask(state.Task{TaskID: task.TaskID}))
	waitRunning(t, srv, running.AssignedAgentID, false)

	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/tasks/"+task.TaskID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete of a released task = %d, want 204", resp.StatusCode)
	}
	if _, err := srv.stateStore.ReadTask(task.TaskID); err == nil {
		t.Fatal("the task survived its deletion")
	}
	// Its agent, and the dependent its result already satisfied, are untouched.
	if _, err := srv.stateStore.ReadAgent(running.AssignedAgentID); err != nil {
		t.Fatalf("deleting a task deleted its agent: %v", err)
	}
	if still, err := srv.stateStore.ReadTask(settled.TaskID); err != nil || still.State != state.TaskReady {
		t.Fatalf("an unrelated dependent changed: %+v, %v", still, err)
	}
	parked, err := srv.stateStore.ReadTask(waiting.TaskID)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if parked.State != state.TaskDependencyFailed {
		t.Fatalf("a dependent still waiting on the deleted task = %s, want parked", parked.State)
	}
}

// FS-16.R24 / TS-10.R20, TS-05.R17 — creator provenance is server-derived and
// durable. An agent may cancel work it created, still may after being stopped
// and resumed, and may not cancel a peer's or a person's work.
func TestAgentCancelAuthorityIsTheRecordedCreator(t *testing.T) {
	srv, ts := wakeTestServer(t)
	creator := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	mine, err := srv.CreateAgentTask(messaging.AgentTaskRequest{
		CreatorAgentID: creator, CreatorGeneration: srv.registry.Generation(creator),
		Project: "tmpproj", DisplayName: "mine", Instruction: "do it", Role: "impl",
	})
	if err != nil {
		t.Fatalf("CreateAgentTask: %v", err)
	}
	if mine.CreatedByKind != "agent" || mine.CreatedByAgentID != creator {
		t.Fatalf("creator = %s/%s, want the calling agent", mine.CreatedByKind, mine.CreatedByAgentID)
	}
	if mine.CreatedByGeneration == "" {
		t.Fatal("the creating generation was not recorded as provenance")
	}

	// A peer cannot cancel it, and the refusal says nothing about its existence.
	_, err = srv.CancelAgentTask(mine.TaskID, "a_someone_else")
	var toolErr *messaging.ToolError
	if !errors.As(err, &toolErr) || toolErr.Code != "not_creator" {
		t.Fatalf("peer cancel = %v, want not_creator", err)
	}
	// Neither can an agent cancel a person's work.
	theirs := createTaskHTTP(t, ts, launchTaskBody("person's"))
	if _, err := srv.CancelAgentTask(theirs.TaskID, creator); !errors.As(err, &toolErr) ||
		toolErr.Code != "not_creator" {
		t.Fatalf("cancelling a person's task = %v, want not_creator", err)
	}

	// The stable id is the authority, so a new generation is not a new principal:
	// stop the creator and cancel with the same id afterwards.
	if resp, body := post(t, ts.URL+"/api/sessions/"+creator+"/stop", nil); resp.StatusCode != 200 {
		t.Fatalf("stop status = %d: %s", resp.StatusCode, body)
	}
	cancelled, err := srv.CancelAgentTask(mine.TaskID, creator)
	if err != nil {
		t.Fatalf("cancel after a stop: %v", err)
	}
	if cancelled.State != state.TaskFinished || cancelled.Outcome != state.OutcomeCancelled {
		t.Fatalf("cancelled = %s/%s", cancelled.State, cancelled.Outcome)
	}
}

// FS-16.R10 / TS-05.R17 — attaching is not a way to reach context: an agent may
// attach only what it can already read, and the attachment grants the assignee a
// work-derived route rather than a direct share.
func TestAnAgentAttachesOnlyContextItCanRead(t *testing.T) {
	srv, ts := wakeTestServer(t)
	creator := launchAndWaitIdle(t, ts, "impl", "tmpproj")

	_, err := srv.CreateAgentTask(messaging.AgentTaskRequest{
		CreatorAgentID: creator, CreatorGeneration: srv.registry.Generation(creator),
		Project: "tmpproj", DisplayName: "with context", Instruction: "read it", Role: "impl",
		Attachments: []state.TaskAttachment{{ContextRefID: "cx_not_mine", Label: "someone else's"}},
	})
	var toolErr *messaging.ToolError
	if !errors.As(err, &toolErr) || toolErr.Code != "validation" {
		t.Fatalf("attaching unreadable context = %v, want a validation refusal", err)
	}
	tasks, err := srv.stateStore.ListTasks("tmpproj")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("a refused attachment created %d tasks", len(tasks))
	}
}
