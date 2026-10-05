package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AsaphNoam/Chuck/internal/runtime"
	"github.com/AsaphNoam/Chuck/internal/state"
)

func roomEvent(t *testing.T, agentID, generation, turnID string, seq int64, typ string, data any) runtime.Event {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return runtime.Event{AgentID: agentID, Generation: generation, TurnID: turnID, Seq: seq, Type: typ, Data: raw, Ts: "2026-10-06T09:00:00Z"}
}

// beginCapturedTurn admits the next room turn for agentID with a recorded
// source cwd and starts its capture, as the engine's before callback does.
func beginCapturedTurn(t *testing.T, srv *Server, room, agentID, turnID, cwd string) state.ThinkTankAttempt {
	t.Helper()
	if _, err := srv.stateStore.DB().Exec(`
INSERT OR REPLACE INTO sessions(agent_id, name, role, project, backend, model, interface, cwd, system_prompt, created_at, updated_at)
VALUES(?, ?, 'impl', 'alpha', 'claude', '', 'chat', ?, '', '2026-10-06T09:00:00Z', '2026-10-06T09:00:00Z')`,
		agentID, agentID, cwd); err != nil {
		t.Fatal(err)
	}
	d, err := srv.stateStore.ReadThinkTank(room)
	if err != nil {
		t.Fatal(err)
	}
	next, ok := state.NextThinkTankOpportunity(d)
	if !ok || next.AgentID != agentID {
		t.Fatalf("next = %+v, want %s", next, agentID)
	}
	a, err := srv.stateStore.BeginThinkTankAttempt(state.ThinkTankBegin{RoomID: room, Revision: d.Room.Revision,
		AgentID: agentID, Turn: next.Turn, Generation: "g1", TurnID: turnID})
	if err != nil {
		t.Fatal(err)
	}
	srv.beginThinkTankCapture(agentID, a, agentID, "alpha")
	return a
}

// TS-14.R10, R12, FS-21.A15, A19, A29: room-turn activity is copied with
// provenance, Chuck's own tool calls become receipts, unrelated turns are not
// imported, and equal relative paths in two workspaces stay distinct.
func TestThinkTankCaptureProjectsRoomActivity(t *testing.T) {
	srv, h := roomRESTServer(t)
	rec := doJSON(t, h, http.MethodPost, "/api/think-tanks", roomBody("a_one", "a_two"))
	var room thinkTankDetailWire
	_ = json.Unmarshal(rec.Body.Bytes(), &room)
	cwdA, cwdB := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(cwdA, "notes.md"), []byte("from A"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwdB, "notes.md"), []byte("from B"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := beginCapturedTurn(t, srv, room.RoomID, "a_one", "t1", cwdA)
	events := []runtime.Event{
		roomEvent(t, "a_one", "g1", "t1", 1, runtime.EvUserPrompt, runtime.UserPromptData{Text: "private instruction"}),
		roomEvent(t, "a_one", "g1", "t1", 2, runtime.EvToolCall, runtime.ToolCallData{ToolCallID: "c1", Name: "mcp__chuck-messaging__read_think_tank", Args: json.RawMessage(`{"cursor":"secret-cursor"}`)}),
		roomEvent(t, "a_one", "g1", "t1", 3, runtime.EvToolResult, runtime.ToolResultData{ToolCallID: "c1", Status: "completed", Content: json.RawMessage(`"turn_token ttk_secret"`)}),
		roomEvent(t, "a_one", "g1", "t1", 4, runtime.EvToolCall, runtime.ToolCallData{ToolCallID: "c2", Name: "Bash", Args: json.RawMessage(`{"command":"go test ./..."}`)}),
		roomEvent(t, "a_one", "g1", "t1", 5, runtime.EvToolResult, runtime.ToolResultData{ToolCallID: "c2", Status: "completed"}),
		roomEvent(t, "a_one", "g1", "t1", 6, runtime.EvDiff, runtime.DiffData{ToolCallID: "c3", Path: "notes.md", NewText: "x"}),
		roomEvent(t, "a_one", "g1", "t0", 7, runtime.EvToolCall, runtime.ToolCallData{ToolCallID: "c4", Name: "Bash", Args: json.RawMessage(`{"command":"private"}`)}),
		roomEvent(t, "a_two", "g1", "t1", 8, runtime.EvDiff, runtime.DiffData{ToolCallID: "c5", Path: "other.md"}),
	}
	for _, ev := range events {
		srv.captureThinkTankEvent(ev)
	}
	if _, err := srv.stateStore.StageThinkTankTurn("a_one", a.Token, state.ThinkTankReply, "A", "ttr_"+strings.TrimPrefix(a.AttemptID, "tta_")+"_0"); err != nil {
		t.Fatal(err)
	}
	srv.finishThinkTankTurn(roomEvent(t, "a_one", "g1", "t1", 9, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn"}))
	beginCapturedTurn(t, srv, room.RoomID, "a_two", "t2", cwdB)
	srv.captureThinkTankEvent(roomEvent(t, "a_two", "g1", "t2", 1, runtime.EvDiff, runtime.DiffData{ToolCallID: "d1", Path: "notes.md"}))

	base := "/api/think-tanks/" + room.RoomID
	act := doJSON(t, h, http.MethodGet, base+"/activity", "")
	body := act.Body.String()
	for _, leaked := range []string{"private instruction", "secret-cursor", "ttk_secret", `"private"`, "other.md"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("activity leaked %q: %s", leaked, body)
		}
	}
	if !strings.Contains(body, "Read the room") || !strings.Contains(body, "go test ./...") {
		t.Fatalf("activity = %s", body)
	}
	var page struct {
		Activity []thinkTankActivityWire `json:"activity"`
	}
	_ = json.Unmarshal(act.Body.Bytes(), &page)
	if len(page.Activity) != 6 || page.Activity[0].AgentName != "a_one" || page.Activity[0].SourceSeq != 2 {
		t.Fatalf("activity rows = %+v", page.Activity)
	}

	var files struct {
		Sources []thinkTankSourceWire `json:"sources"`
		Files   []thinkTankFileWire   `json:"files"`
	}
	_ = json.Unmarshal(doJSON(t, h, http.MethodGet, base+"/files", "").Body.Bytes(), &files)
	if len(files.Files) != 2 || files.Files[0].SourceID == files.Files[1].SourceID {
		t.Fatalf("equal relative paths conflated: %+v", files.Files)
	}
	for i, want := range []string{"from A", "from B"} {
		got := doJSON(t, h, http.MethodGet, base+"/sources/"+files.Files[i].SourceID+"/file?path=notes.md", "")
		if !strings.Contains(got.Body.String(), want) {
			t.Fatalf("source %d file = %s", i, got.Body.String())
		}
	}
	cmds := doJSON(t, h, http.MethodGet, base+"/commands", "").Body.String()
	if !strings.Contains(cmds, `"command":"go test ./..."`) || !strings.Contains(cmds, `"status":"completed"`) || strings.Contains(cmds, "private") {
		t.Fatalf("commands = %s", cmds)
	}
	// Retained activity survives the source agent's deletion.
	if err := srv.stateStore.DeleteAgent("a_one"); err != nil {
		t.Fatal(err)
	}
	if again := doJSON(t, h, http.MethodGet, base+"/activity", ""); !strings.Contains(again.Body.String(), "go test ./...") {
		t.Fatal("retained activity lost with its agent")
	}
}

// FS-21.A20, FS-13.R26–R27: Room delivery is shared input refused after the
// discussion ends; selected-agent delivery records the attributed batch in
// room history first and then delivers it as ordinary annotation mail.
func TestThinkTankAnnotationDestinations(t *testing.T) {
	srv, h := roomRESTServer(t)
	rec := doJSON(t, h, http.MethodPost, "/api/think-tanks", roomBody("a_one", "a_two"))
	var room thinkTankDetailWire
	_ = json.Unmarshal(rec.Body.Bytes(), &room)
	base := "/api/think-tanks/" + room.RoomID
	doJSON(t, h, http.MethodPost, base+"/messages", `{"command_id":"m1","body":"Consider eviction"}`)
	batch := func(cmd, target string) string {
		return `{"command_id":"` + cmd + `","annotations":[{"anchor":"entry","seq":1,"excerpt":"eviction","instruction":"expand on this"}],"target":` + target + `}`
	}
	if rec := doJSON(t, h, http.MethodPost, base+"/annotations", batch("a1", `{"kind":"room"}`)); rec.Code != http.StatusOK {
		t.Fatalf("room annotation = %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, base+"/annotations",
		`{"command_id":"bad","annotations":[{"anchor":"entry","seq":99,"excerpt":"x","instruction":"y"}],"target":{"kind":"room"}}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown anchor = %d", rec.Code)
	}
	doJSON(t, h, http.MethodPost, base+"/end", "")
	if rec := doJSON(t, h, http.MethodPost, base+"/annotations", batch("a2", `{"kind":"room"}`)); rec.Code != http.StatusConflict {
		t.Fatalf("room annotation after end = %d", rec.Code)
	}
	if err := srv.stateStore.WriteRunning(state.RunningEntry{AgentID: "a_two", PID: 1, SessionID: "s", Interface: "chat"}); err != nil {
		t.Fatal(err)
	}
	if rec := doJSON(t, h, http.MethodPost, base+"/annotations", batch("a3", `{"kind":"agent","agent_id":"a_two"}`)); rec.Code != http.StatusOK {
		t.Fatalf("agent annotation after end = %d %s", rec.Code, rec.Body.String())
	}
	entries, _ := srv.stateStore.ListThinkTankEntries(room.RoomID, 0, 10)
	if len(entries) != 3 || entries[1].Kind != state.ThinkTankEntryAnnotation ||
		!strings.Contains(entries[1].Body, "Room entry 1 by the user") || !strings.Contains(entries[2].Context, `"recipient":"Bea"`) {
		t.Fatalf("entries = %+v", entries)
	}
	mail, err := srv.stateStore.ListMessages("a_two", false, 10)
	if err != nil || len(mail) != 1 || !strings.Contains(mail[0].Body, "expand on this") {
		t.Fatalf("delivered mail = %+v %v", mail, err)
	}
}

// TS-14.R16, FS-21.A3: an opening's activity is withheld until publication.
func TestThinkTankOpeningActivityIsWithheld(t *testing.T) {
	srv, h := roomRESTServer(t)
	body := strings.Replace(roomBody("a_one", "a_two"), `"participants"`, `"openings":true,"participants"`, 1)
	rec := doJSON(t, h, http.MethodPost, "/api/think-tanks", body)
	var room thinkTankDetailWire
	_ = json.Unmarshal(rec.Body.Bytes(), &room)
	beginCapturedTurn(t, srv, room.RoomID, "a_one", "t1", t.TempDir())
	srv.captureThinkTankEvent(roomEvent(t, "a_one", "g1", "t1", 1, runtime.EvToolCall, runtime.ToolCallData{ToolCallID: "c", Name: "Bash", Args: json.RawMessage(`{"command":"opening secret"}`)}))
	if got := doJSON(t, h, http.MethodGet, "/api/think-tanks/"+room.RoomID+"/activity", "").Body.String(); strings.Contains(got, "opening secret") {
		t.Fatalf("withheld opening activity offered: %s", got)
	}
}
