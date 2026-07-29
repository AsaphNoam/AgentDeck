package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/state"
)

// FS-05.A16 / INV §4: Archive must use the same orphan reaper as Stop. A
// restart has no in-memory runtime handle, but it must not leave a live process
// behind when the archive bit commits.
func TestArchiveAgentReapsLiveOrphanBeforeCommit(t *testing.T) {
	srv := testServer(t, true)
	cmd := exec.Command("sh", "-c", "exec sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start orphan process: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	agent := state.Agent{AgentID: "a_archive_orphan", Name: "Orphan", Role: "implementer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatal(err)
	}
	if err := srv.stateStore.WriteRunning(state.RunningEntry{AgentID: agent.AgentID, PID: cmd.Process.Pid, SessionID: "orphan", Interface: "chat", StartedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	req := newLocalRequest(http.MethodPost, "/api/sessions/"+agent.AgentID+"/archive", nil)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("archive status = %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := srv.stateStore.ReadRunning(agent.AgentID); !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("orphan running row = %v, want removed", err)
	}
	archived, err := srv.stateStore.ReadAgent(agent.AgentID)
	if err != nil || !archived.Archived {
		t.Fatalf("archived agent = %+v err=%v", archived, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("archive left the orphan process alive")
	}
}

// TS-02.R20 / INV §15: a failed final project write restores each member's
// previous archive state, including an agent that was individually archived.
func TestArchiveProjectConfigFailureRestoresMixedAgentFlags(t *testing.T) {
	srv := testServer(t, true)
	for _, agent := range []state.Agent{
		{AgentID: "a_previously_archived", Name: "Archived", Role: "implementer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC(), Archived: true},
		{AgentID: "a_previously_active", Name: "Active", Role: "reviewer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()},
	} {
		if err := srv.stateStore.WriteAgent(agent); err != nil {
			t.Fatal(err)
		}
	}
	srv.writeProject = func(string, config.Project) error { return errors.New("injected project write failure") }

	req := newLocalRequest(http.MethodPost, "/api/projects/my-app/archive", nil)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("archive status = %d: %s", rec.Code, rec.Body.String())
	}
	for id, wantArchived := range map[string]bool{"a_previously_archived": true, "a_previously_active": false} {
		agent, err := srv.stateStore.ReadAgent(id)
		if err != nil || agent.Archived != wantArchived {
			t.Fatalf("agent %s after failed archive = %+v err=%v, want archived=%v", id, agent, err, wantArchived)
		}
	}
}

// TS-03.R20 / INV §11: project archive always returns its documented, non-null
// action lists, including a harmless repeated archive request.
func TestArchiveProjectRespondsWithActionLists(t *testing.T) {
	srv := testServer(t, true)
	if err := srv.stateStore.WriteAgent(state.Agent{AgentID: "a_project_action", Name: "Project action", Role: "implementer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	archive := func() archiveProjectActionResponse {
		rec := httptest.NewRecorder()
		srv.routes().ServeHTTP(rec, newLocalRequest(http.MethodPost, "/api/projects/my-app/archive", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("archive status = %d: %s", rec.Code, rec.Body.String())
		}
		var response archiveProjectActionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}

	first := archive()
	if !first.Project.Archived || len(first.StoppedAgentIDs) != 1 || first.StoppedAgentIDs[0] != "a_project_action" || len(first.ArchivedAgentIDs) != 1 || first.ArchivedAgentIDs[0] != "a_project_action" {
		t.Fatalf("first archive response = %+v", first)
	}
	second := archive()
	if second.StoppedAgentIDs == nil || second.ArchivedAgentIDs == nil || len(second.StoppedAgentIDs) != 0 || len(second.ArchivedAgentIDs) != 0 {
		t.Fatalf("repeated archive response lists = stopped %#v archived %#v", second.StoppedAgentIDs, second.ArchivedAgentIDs)
	}
}

// FS-05.A19 / INV §7: the grouped projection must reach durable sessions
// beyond the archive package's 200-row page cap, and the per-project endpoint
// must independently page that same full corpus.
func TestGroupedArchivePagesPastTwoHundredSessions(t *testing.T) {
	srv := testServer(t, true)
	stmt, err := srv.stateStore.DB().Prepare(`
INSERT INTO sessions(agent_id, name, role, project, backend, model, interface, cwd, system_prompt, created_at, updated_at)
VALUES (?, ?, 'implementer', 'my-app', 'claude', 'sonnet', 'chat', '/tmp', 'prompt', ?, ?)`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 201; i++ {
		id := fmt.Sprintf("a_archive_%03d", i)
		at := base.Add(time.Duration(i) * time.Second)
		if err := srv.stateStore.WriteAgent(state.Agent{AgentID: id, Name: id, Role: "implementer", Project: "my-app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: at, Archived: true}); err != nil {
			t.Fatal(err)
		}
		if _, err := stmt.Exec(id, id, at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}

	rec := doGET(t, srv.routes(), "/api/archive?limit=50")
	if rec.Code != http.StatusOK {
		t.Fatalf("group archive status = %d: %s", rec.Code, rec.Body.String())
	}
	var groups archiveGroupsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &groups); err != nil {
		t.Fatal(err)
	}
	if groups.Total != 1 || len(groups.Results) != 1 || len(groups.Results[0].Results) != 201 {
		t.Fatalf("grouped archive = total %d groups %d rows %d, want 1/1/201", groups.Total, len(groups.Results), len(groups.Results[0].Results))
	}

	rec = doGET(t, srv.routes(), "/api/archive/projects/my-app?limit=1&offset=200")
	if rec.Code != http.StatusOK {
		t.Fatalf("project archive status = %d: %s", rec.Code, rec.Body.String())
	}
	var page struct {
		Total   int                  `json:"total"`
		Results []archiveAgentResult `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 201 || len(page.Results) != 1 || page.Results[0].AgentID != "a_archive_000" {
		t.Fatalf("project page = %+v, want final durable row", page)
	}
}
