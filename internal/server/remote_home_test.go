package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/state"
)

func TestRemoteHomeClassifiesAttention(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")

	now := time.Now().UTC()
	old := now.Add(-2 * time.Hour)
	agent := func(id, st string, at time.Time) {
		s.eventBus.SetSnapshot(state.AgentStateUpdate{AgentState: state.AgentState{
			AgentID: id, Role: "implementer", Project: "my-app", State: st, UpdatedAt: at.UnixMilli()}})
	}
	agent("a-perm", "waiting_input", old)
	agent("a-ask", "waiting_input", now.Add(-time.Hour))
	agent("a-err", "error", now.Add(-30*time.Minute))
	agent("a-busy", "busy", now)
	agent("a-done", "done", now)
	agent("a-done-old", "done", old.Add(-time.Hour))
	s.permissionTools[permissionToolKey("a-perm", "g1", "call-1")] = "Bash"

	task := func(id, st string, at time.Time) {
		if _, err := s.stateStore.CreateTask(state.Task{TaskID: id, Project: "my-app", DisplayName: id, Instruction: "x", TargetKind: state.TargetLaunch, Role: "implementer", CreatedByKind: "person"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.stateStore.DB().Exec(`UPDATE tasks SET state = ?, outcome = ?, updated_at = ? WHERE task_id = ?`,
			st, map[bool]string{true: "success"}[st == state.TaskFinished], at.Format(time.RFC3339), id); err != nil {
			t.Fatal(err)
		}
	}
	task("t-int", state.TaskInterrupted, now.Add(-90*time.Minute))
	task("t-dep", state.TaskDependencyFailed, now.Add(-10*time.Minute))
	task("t-run", state.TaskRunning, now)
	task("t-fin", state.TaskFinished, now)
	task("t-fin-old", state.TaskFinished, old.Add(-time.Hour))
	task("t-armed", state.TaskArmed, now)
	r := phoneRequest(http.MethodGet, "/api/remote/home", "", token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != 200 {
		t.Fatalf("home = %d %s", rec.Code, rec.Body)
	}
	var home attentionLists
	if err := json.Unmarshal(rec.Body.Bytes(), &home); err != nil {
		t.Fatal(err)
	}
	ids := func(items []attentionItem) []string {
		out := []string{}
		for _, it := range items {
			out = append(out, it.ID+":"+it.Reason)
		}
		return out
	}
	wantNeeds := []string{"a-perm:" + reasonPermission, "a-ask:" + reasonQuestion, "a-err:" + reasonError}
	if got := ids(home.NeedsYou); !equalStrings(got, wantNeeds) {
		t.Fatalf("needs you = %v, want %v (oldest first)", got, wantNeeds)
	}
	if len(home.ActiveRuns) != 0 {
		t.Fatalf("active runs = %v", home.ActiveRuns)
	}
	for _, it := range home.NeedsYou {
		if it.Kind == "agent" && it.Title != "implementer@my-app" {
			t.Fatalf("agent title = %q", it.Title)
		}
	}

	// Home is tailnet-only; the loopback mux does not serve it.
	if rec := doGET(t, s.routes(), "/api/remote/home"); rec.Code != 404 {
		t.Fatalf("loopback home = %d", rec.Code)
	}
}

func TestRemoteHomeKeepsOlderRunAttention(t *testing.T) {
	s := testServer(t, true)
	now := time.Now().UTC()
	old := now.Add(-2 * time.Hour)
	snapshot, err := json.Marshal(pipeline.Template{Version: 2, Title: "Two", OrchestratorRole: "implementer", Stages: []pipeline.Stage{{ID: "one", Title: "One"}, {ID: "two", Title: "Two"}}})
	if err != nil {
		t.Fatal(err)
	}
	createRun := func(id, stateName string, at time.Time) {
		t.Helper()
		if _, _, err := s.stateStore.CreatePipelineRun(state.CreatePipelineRunParams{Run: state.PipelineRunRecord{
			RunID: id, TemplateID: "two", TemplateSnapshot: snapshot, DisplayName: id, Project: "my-app", Goal: "x",
			State: stateName, CurrentStageID: "two", CreatedAt: at, UpdatedAt: at,
		}, RequestID: id}); err != nil {
			t.Fatal(err)
		}
	}
	createRun("older-paused", "paused", old)
	for i := 0; i < 101; i++ {
		createRun(fmt.Sprintf("new-completed-%03d", i), "completed", now)
	}
	home, err := s.attention()
	if err != nil {
		t.Fatal(err)
	}
	if !containsAttention(home.NeedsYou, "older-paused") {
		t.Fatalf("older attention missing: %+v", home.NeedsYou)
	}
	for _, item := range home.NeedsYou {
		if item.ID == "older-paused" && (item.StageNumber != 2 || item.StageCount != 2) {
			t.Fatalf("run stage fields = %d/%d", item.StageNumber, item.StageCount)
		}
	}
}

func containsAttention(items []attentionItem, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
