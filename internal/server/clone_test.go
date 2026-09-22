package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

func cloneServer(t *testing.T, env map[string]string) (*Server, *httptest.Server, string) {
	t.Helper()
	fake := buildFakeACP(t)
	t.Setenv("FAKEACP_SCENARIO", "stream_text")
	for k, v := range env {
		t.Setenv(k, v)
	}
	srv := testServer(t, true)
	srv.registry.Chat().SetCommand(fake)
	if err := srv.configStore.WriteProject("tmpproj", config.Project{Title: "Tmp", Cwd: t.TempDir()}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	if err := srv.configStore.WriteRole("impl", config.Role{Title: "Impl", SystemPrompt: "be helpful"}); err != nil {
		t.Fatalf("WriteRole: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	t.Cleanup(func() { srv.registry.Shutdown(context.Background()) })
	id := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	if resp, body := post(t, ts.URL+"/api/sessions/"+id+"/prompt", map[string]string{"text": "hello"}); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt = %d %s", resp.StatusCode, body)
	}
	waitStatus(t, srv, id, "idle")
	return srv, ts, id
}

func readEvents(t *testing.T, srv *Server, id string) []runtime.Event {
	t.Helper()
	evs, err := transcript.ReadFile(srv.configStore.Home(), id, transcript.ReadOptions{})
	if err != nil {
		t.Fatalf("read transcript %s: %v", id, err)
	}
	return evs
}

// FS-01.A20 / TS-03.R43: Clone creates one running agent with a distinct id,
// the same configured identity, a distinct native session, the source's visible
// history through its last completed turn plus a source-link marker; a later
// source turn stays on the source.
func TestCloneForksTheConversationIntoANewAgent(t *testing.T) {
	srv, ts, src := cloneServer(t, map[string]string{"FAKEACP_CAPS": "1"})
	update, err := srv.stateMgr.Touch(src)
	if err != nil || !update.Clone.Available {
		t.Fatalf("source clone affordance = %+v err %v", update.Clone, err)
	}
	resp, body := post(t, ts.URL+"/api/sessions/"+src+"/clone", map[string]string{})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("clone = %d %s", resp.StatusCode, body)
	}
	var out struct {
		Agent             state.Agent        `json:"agent"`
		Running           *state.RunningEntry `json:"running"`
		HistoryHandoff    string             `json:"history_handoff"`
		ForkedFromAgentID string             `json:"forked_from_agent_id"`
		ForkedFromSeq     int64              `json:"forked_from_seq"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	source, _ := srv.stateStore.ReadAgent(src)
	if out.Agent.AgentID == src || out.Agent.Role != source.Role || out.Agent.Project != source.Project ||
		out.Agent.Backend != source.Backend || out.Agent.Model != source.Model || out.Agent.Interface != source.Interface {
		t.Fatalf("clone identity = %+v, source %+v", out.Agent, source)
	}
	if out.Running == nil || out.Running.SessionID != "fake-fork-1" || out.HistoryHandoff != "native_fork" || out.ForkedFromAgentID != src {
		t.Fatalf("clone envelope = %s", body)
	}
	srcEvents := readEvents(t, srv, src)
	cloneEvents := readEvents(t, srv, out.Agent.AgentID)
	if out.ForkedFromSeq != srcEvents[len(srcEvents)-1].Seq {
		t.Fatalf("forked_from_seq = %d, source last = %d", out.ForkedFromSeq, srcEvents[len(srcEvents)-1].Seq)
	}
	if len(cloneEvents) != len(srcEvents)+1 || cloneEvents[len(cloneEvents)-1].Type != runtime.EvForkBoundary {
		t.Fatalf("clone transcript = %+v", cloneEvents)
	}
	for i, ev := range srcEvents {
		if cloneEvents[i].Type != ev.Type || string(cloneEvents[i].Data) != string(ev.Data) {
			t.Fatalf("copied event %d = %+v, source %+v", i, cloneEvents[i], ev)
		}
	}
	var linked string
	if err := srv.stateStore.DB().QueryRow(`SELECT forked_from_agent_id FROM sessions WHERE agent_id = ?`, out.Agent.AgentID).Scan(&linked); err != nil || linked != src {
		t.Fatalf("lineage = %q, %v", linked, err)
	}

	// A later source turn never enters the clone.
	if resp, body := post(t, ts.URL+"/api/sessions/"+src+"/prompt", map[string]string{"text": "again"}); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("source prompt = %d %s", resp.StatusCode, body)
	}
	waitStatus(t, srv, src, "idle")
	if got := readEvents(t, srv, out.Agent.AgentID); len(got) != len(cloneEvents) {
		t.Fatalf("source turn leaked into the clone: %d events, had %d", len(got), len(cloneEvents))
	}
}

// A stopped, non-archived source is cloneable from its last native session.
func TestCloneFromAStoppedSource(t *testing.T) {
	srv, ts, src := cloneServer(t, map[string]string{"FAKEACP_CAPS": "1"})
	if resp, body := post(t, ts.URL+"/api/sessions/"+src+"/stop", map[string]string{}); resp.StatusCode >= 300 {
		t.Fatalf("stop = %d %s", resp.StatusCode, body)
	}
	resp, body := post(t, ts.URL+"/api/sessions/"+src+"/clone", map[string]string{})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("clone of stopped source = %d %s", resp.StatusCode, body)
	}
	_ = srv
}

// A non-advertising runtime exposes no weaker clone path and leaves nothing.
func TestCloneUnavailableWithoutFork(t *testing.T) {
	srv, ts, src := cloneServer(t, nil)
	update, _ := srv.stateMgr.Touch(src)
	if update.Clone.Available || update.Clone.Reason != state.CloneReasonNoFork {
		t.Fatalf("affordance = %+v", update.Clone)
	}
	before, _ := filepath.Glob(filepath.Join(srv.configStore.Home(), "sessions", "*"))
	resp, body := post(t, ts.URL+"/api/sessions/"+src+"/clone", map[string]string{})
	if resp.StatusCode != http.StatusUnprocessableEntity || apiErrorCode(t, body) != "clone_unavailable" {
		t.Fatalf("clone without fork = %d %s", resp.StatusCode, body)
	}
	after, _ := filepath.Glob(filepath.Join(srv.configStore.Home(), "sessions", "*"))
	if len(after) != len(before) {
		t.Fatalf("a refused clone left state: %v -> %v", before, after)
	}
}

// A busy source cannot be cloned; a rejected fork creates no agent.
func TestCloneRefusalsLeaveNoPartialAgent(t *testing.T) {
	srv, ts, src := cloneServer(t, map[string]string{"FAKEACP_CAPS": "1"})
	if err := srv.stateStore.WriteStatus(state.Status{AgentID: src, State: "busy", Detail: "working"}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	update, _ := srv.stateMgr.Touch(src)
	if update.Clone.Available || update.Clone.Reason != state.CloneReasonBusy {
		t.Fatalf("busy affordance = %+v", update.Clone)
	}
	resp, body := post(t, ts.URL+"/api/sessions/"+src+"/clone", map[string]string{})
	if resp.StatusCode != http.StatusConflict || apiErrorCode(t, body) != "agent_busy" {
		t.Fatalf("busy clone = %d %s", resp.StatusCode, body)
	}
	if err := srv.stateStore.WriteStatus(state.Status{AgentID: src, State: "idle"}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	agents, _ := srv.stateStore.ListAgents()
	t.Setenv("FAKEACP_FORK_FAIL", "1")
	resp, body = post(t, ts.URL+"/api/sessions/"+src+"/clone", map[string]string{})
	if resp.StatusCode < 400 {
		t.Fatalf("rejected fork = %d %s", resp.StatusCode, body)
	}
	if after, _ := srv.stateStore.ListAgents(); len(after) != len(agents) {
		t.Fatalf("rejected fork left an agent: %d -> %d", len(agents), len(after))
	}
}
