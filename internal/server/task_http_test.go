package server

import (
	"encoding/json"
	"net/http"
	"testing"

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
