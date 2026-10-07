package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AsaphNoam/Chuck/internal/runtime"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// thinkTankTestServer runs fake ACP turns that stay open until the hold file
// exists, so a test can act as the agent's room tool calls mid-turn.
func thinkTankTestServer(t *testing.T) (*Server, string, string, string) {
	t.Helper()
	hold := filepath.Join(t.TempDir(), "hold")
	t.Setenv("FAKEACP_HOLD_FILE", hold)
	srv, ts, promptLog := activationTestServer(t)
	t.Setenv("FAKEACP_SCENARIO", "hold_turn")
	a := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	b := launchAndWaitIdle(t, ts, "impl", "tmpproj")
	return srv, promptLog, a + "," + b, hold
}

func createTestRoom(t *testing.T, srv *Server, agents []string, limit int) string {
	t.Helper()
	members := []state.ThinkTankMember{}
	for _, id := range agents {
		members = append(members, state.ThinkTankMember{AgentID: id, AgentName: id, Project: "tmpproj", Cap: limit, MayLeave: true})
	}
	d, err := srv.stateStore.CreateThinkTank(state.ThinkTankCreate{CommandID: "c1", Goal: "Choose a queue",
		OriginProject: "tmpproj", Members: members})
	if err != nil {
		t.Fatalf("CreateThinkTank: %v", err)
	}
	return d.Room.RoomID
}

func waitActiveAttempt(t *testing.T, srv *Server, room, agentID string) state.ThinkTankAttempt {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		d, err := srv.stateStore.ReadThinkTank(room)
		if err != nil {
			t.Fatal(err)
		}
		if d.Active != nil && d.Active.AgentID == agentID {
			return *d.Active
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no active room attempt for %s", agentID)
	return state.ThinkTankAttempt{}
}

func waitRoom(t *testing.T, srv *Server, room string, ok func(state.ThinkTankDetail) bool) state.ThinkTankDetail {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		d, err := srv.stateStore.ReadThinkTank(room)
		if err != nil {
			t.Fatal(err)
		}
		if ok(d) {
			return d
		}
		time.Sleep(10 * time.Millisecond)
	}
	d, _ := srv.stateStore.ReadThinkTank(room)
	t.Fatalf("room never reached the expected state: %+v", d.Room)
	return d
}

// actAsAgent performs the room tool calls an agent makes during its turn.
func actAsAgent(t *testing.T, srv *Server, a state.ThinkTankAttempt, disposition, msg string) {
	t.Helper()
	cursor := ""
	for {
		page, err := srv.stateStore.ReadThinkTankPage(state.ThinkTankReadRequest{CallerAgentID: a.AgentID, Cursor: cursor})
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if page.Complete {
			if _, err := srv.stateStore.StageThinkTankTurn(a.AgentID, page.TurnToken, disposition, msg, page.ReadReceipt); err != nil {
				t.Fatalf("stage: %v", err)
			}
			return
		}
		cursor = page.NextCursor
	}
}

func releaseTurn(t *testing.T, hold string, srv *Server, room string, wantRevisionAbove int64) {
	t.Helper()
	if err := os.WriteFile(hold, []byte("go"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Active == nil && d.Room.Revision > wantRevisionAbove })
	if err := os.Remove(hold); err != nil {
		t.Fatal(err)
	}
}

// FS-21.A1, A10, TS-14.R3–R4/R6: the engine starts each speaker as a guarded
// think_tank activation carrying only the fixed instruction, the attempt
// records the executing generation/turn, and the matching turn end publishes
// the staged contribution under that speaker.
func TestThinkTankEngineRunsGuardedTurns(t *testing.T) {
	srv, promptLog, ids, hold := thinkTankTestServer(t)
	agents := strings.Split(ids, ",")
	room := createTestRoom(t, srv, agents, 1)
	ctx := context.Background()

	srv.progressThinkTanks(ctx)
	first := waitActiveAttempt(t, srv, room, agents[0])
	waitPrompts(t, promptLog, 1)
	raw, _ := os.ReadFile(promptLog)
	kind, _ := runtime.LookupActivationKind(state.ActivationKindThinkTank)
	if !strings.Contains(string(raw), "read_think_tank") || !strings.Contains(string(raw), "submit_think_tank_turn") ||
		strings.Contains(string(raw), "Choose a queue") {
		t.Fatalf("room prompt = %s, want the fixed instruction %q without the goal", raw, kind.Instruction)
	}
	if first.Generation != srv.registry.Generation(agents[0]) || first.TurnID == "" {
		t.Fatalf("attempt ownership = %q/%q, runtime generation %q", first.Generation, first.TurnID, srv.registry.Generation(agents[0]))
	}
	// One floor: progression admits nothing else while the turn runs.
	srv.progressThinkTanks(ctx)
	if promptCount(t, promptLog) != 1 {
		t.Fatal("a second room turn started during the first")
	}
	actAsAgent(t, srv, first, state.ThinkTankReply, "Use a durable queue")
	releaseTurn(t, hold, srv, room, first.Head)

	srv.progressThinkTanks(ctx)
	second := waitActiveAttempt(t, srv, room, agents[1])
	page, err := srv.stateStore.ReadThinkTankPage(state.ThinkTankReadRequest{CallerAgentID: agents[1]})
	if err != nil || len(page.Items) != 1 || page.Items[0].Entry.AgentID != agents[0] || page.Items[0].Entry.Body != "Use a durable queue" {
		t.Fatalf("second speaker read = %+v %v", page.Items, err)
	}
	actAsAgent(t, srv, second, state.ThinkTankReply, "Objection: ordering")
	d, _ := srv.stateStore.ReadThinkTank(room)
	releaseTurn(t, hold, srv, room, d.Room.Revision)

	d = waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Room.Phase == state.ThinkTankPhaseEnded })
	if d.Room.EndReason != state.ThinkTankEndAllowanceExhausted {
		t.Fatalf("end reason = %s", d.Room.EndReason)
	}
	entries, _ := srv.stateStore.ListThinkTankEntries(room, 0, 10)
	if len(entries) != 2 || entries[1].AgentID != agents[1] {
		t.Fatalf("entries = %+v", entries)
	}
	if promptCount(t, promptLog) != 2 {
		t.Fatalf("provider prompts = %d, want exactly one per speaker", promptCount(t, promptLog))
	}
	// Participants remain ordinary running agents after the room ends.
	for _, id := range agents {
		if _, err := srv.stateStore.ReadRunning(id); err != nil {
			t.Fatalf("participant %s stopped: %v", id, err)
		}
	}
}

// FS-21.A11, A13, R10: a turn that ends without a staged submission holds the
// room without charge, publication or automatic retry.
func TestThinkTankEngineHoldsMissingSubmission(t *testing.T) {
	srv, promptLog, ids, hold := thinkTankTestServer(t)
	agents := strings.Split(ids, ",")
	room := createTestRoom(t, srv, agents, 2)
	srv.progressThinkTanks(context.Background())
	first := waitActiveAttempt(t, srv, room, agents[0])
	releaseTurn(t, hold, srv, room, first.Head)
	d := waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Room.Hold != "" })
	srv.progressThinkTanks(context.Background())
	if promptCount(t, promptLog) != 1 {
		t.Fatal("a held room started another turn")
	}
	for _, m := range d.Members {
		if m.Completed != 0 {
			t.Fatalf("failed turn charged %s", m.AgentID)
		}
	}
	if entries, _ := srv.stateStore.ListThinkTankEntries(room, 0, 10); len(entries) != 0 {
		t.Fatalf("assistant output was published: %+v", entries)
	}
}

// FS-21.A6, A7, A21, TS-14.R2, R13: a reserved new participant launches before
// discussion starts, and the fresh judge launches only after discussion ends,
// reads the whole discussion and publishes one attributed synthesis.
func TestThinkTankEngineLaunchesNewParticipantAndJudge(t *testing.T) {
	srv, promptLog, ids, hold := thinkTankTestServer(t)
	existing := strings.Split(ids, ",")[0]
	config := `{"role":"impl","project":"tmpproj","interface":"chat"}`
	fresh, err := srv.stateStore.NewAgentID()
	if err != nil {
		t.Fatal(err)
	}
	d, err := srv.stateStore.CreateThinkTank(state.ThinkTankCreate{CommandID: "j", Goal: "Decide", OriginProject: "tmpproj",
		JudgeConfig: config, Members: []state.ThinkTankMember{
			{AgentID: existing, AgentName: "Existing", Project: "tmpproj", Cap: 1},
			{AgentID: fresh, AgentName: "New agent", Project: "tmpproj", Cap: 1, SetupConfig: config},
		}})
	if err != nil {
		t.Fatal(err)
	}
	room := d.Room.RoomID
	ctx := context.Background()
	srv.progressThinkTanks(ctx)
	d = waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Room.Phase == state.ThinkTankPhaseDiscussion })
	if _, err := srv.stateStore.ReadRunning(fresh); err != nil {
		t.Fatalf("reserved participant was not launched under its identity: %v", err)
	}
	for _, id := range []string{existing, fresh} {
		srv.progressThinkTanks(ctx)
		a := waitActiveAttempt(t, srv, room, id)
		if d.Room.JudgeAgentID != "" {
			t.Fatal("judge launched during discussion")
		}
		actAsAgent(t, srv, a, state.ThinkTankReply, "view of "+id)
		cur, _ := srv.stateStore.ReadThinkTank(room)
		releaseTurn(t, hold, srv, room, cur.Room.Revision)
	}
	d = waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Room.JudgeStatus == state.ThinkTankJudgeReady })
	srv.progressThinkTanks(ctx) // launches the judge
	srv.progressThinkTanks(ctx) // starts its turn
	d = waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool {
		return d.Active != nil && d.Active.Turn == state.ThinkTankTurnJudge
	})
	judge := *d.Active
	if judge.AgentID == existing || judge.AgentID == fresh {
		t.Fatal("a participant was adopted as the judge")
	}
	page, err := srv.stateStore.ReadThinkTankPage(state.ThinkTankReadRequest{CallerAgentID: judge.AgentID})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("judge read %d entries: %v", len(page.Items), err)
	}
	actAsAgent(t, srv, judge, state.ThinkTankReply, "Synthesis: unresolved")
	releaseTurn(t, hold, srv, room, d.Room.Revision)
	d = waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Room.JudgeStatus == state.ThinkTankJudgeCompleted })
	entries, _ := srv.stateStore.ListThinkTankEntries(room, 0, 10)
	if last := entries[len(entries)-1]; last.Kind != state.ThinkTankEntrySynthesis || last.AgentID != judge.AgentID {
		t.Fatalf("last entry = %+v", last)
	}
	if promptCount(t, promptLog) != 3 {
		t.Fatalf("provider prompts = %d, want two participants and one judge", promptCount(t, promptLog))
	}
}

// FS-21.R37: an archived participant holds the room with a reason instead of
// being skipped or substituted.
func TestThinkTankEngineHoldsIneligibleSpeaker(t *testing.T) {
	srv, promptLog, ids, _ := thinkTankTestServer(t)
	agents := strings.Split(ids, ",")
	room := createTestRoom(t, srv, agents, 2)
	if err := srv.stateStore.SetAgentsArchived([]string{agents[0]}, true); err != nil {
		t.Fatal(err)
	}
	srv.progressThinkTanks(context.Background())
	d := waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Room.Hold != "" })
	if !strings.Contains(d.Room.Hold, "archived") || promptCount(t, promptLog) != 0 {
		t.Fatalf("hold = %q prompts = %d", d.Room.Hold, promptCount(t, promptLog))
	}
}

func stopForRoom(t *testing.T, srv *Server, id string) {
	t.Helper()
	if rec := doJSON(t, srv.routes(), http.MethodPost, "/api/sessions/"+id+"/stop", `{}`); rec.Code != http.StatusOK {
		t.Fatalf("stop: %d %s", rec.Code, rec.Body.String())
	}
	waitRunning(t, srv, id, false)
}

// TT2-01, FS-21.R37: a stopped speaker whose resume fails before the room
// attempt begins holds the room with a reason after one attempt, instead of
// relaunching on every sweep.
func TestThinkTankResumeFailureHoldsRoomOnce(t *testing.T) {
	srv, promptLog, ids, _ := thinkTankTestServer(t)
	agents := strings.Split(ids, ",")
	room := createTestRoom(t, srv, agents, 2)
	stopForRoom(t, srv, agents[0])
	count := filepath.Join(t.TempDir(), "launches")
	script := filepath.Join(t.TempDir(), "failing-acp")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho x >> '"+count+"'\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	srv.registry.Chat().SetCommand(script)

	srv.progressThinkTanks(context.Background())
	d := waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Room.Hold != "" })
	if !strings.Contains(d.Room.Hold, "could not start") || d.Active != nil {
		t.Fatalf("hold = %q active = %+v", d.Room.Hold, d.Active)
	}
	srv.progressThinkTanks(context.Background())
	srv.progressThinkTanks(context.Background())
	raw, _ := os.ReadFile(count)
	if n := strings.Count(string(raw), "x"); n != 1 {
		t.Fatalf("resume launches = %d, want one", n)
	}
	if promptCount(t, promptLog) != 0 {
		t.Fatal("a held room sent a turn")
	}
}

// TT2-02: a room that moves while its stopped speaker resumes leaves the
// resumed agent running idle; the next sweep admits it on the running branch.
func TestThinkTankRoomChangeDuringResumeKeepsAgentRunning(t *testing.T) {
	srv, promptLog, ids, _ := thinkTankTestServer(t)
	agents := strings.Split(ids, ",")
	room := createTestRoom(t, srv, agents, 2)
	stopForRoom(t, srv, agents[0])
	d, err := srv.stateStore.ReadThinkTank(room)
	if err != nil {
		t.Fatal(err)
	}
	next, ok := state.NextThinkTankOpportunity(d)
	if !ok || next.AgentID != agents[0] {
		t.Fatalf("next = %+v %v", next, ok)
	}
	// The selection's revision goes stale, as a room message mid-resume does.
	if _, _, err := srv.stateStore.AddThinkTankRecord(room, "note", "context", ""); err != nil {
		t.Fatal(err)
	}
	if srv.startThinkTankTurn(context.Background(), d, next) {
		t.Fatal("a stale selection was admitted")
	}
	if _, err := srv.stateStore.ReadRunning(agents[0]); err != nil {
		t.Fatalf("resumed agent was stopped: %v", err)
	}
	if cur, _ := srv.stateStore.ReadThinkTank(room); cur.Room.Hold != "" {
		t.Fatalf("room held: %q", cur.Room.Hold)
	}
	srv.progressThinkTanks(context.Background())
	waitActiveAttempt(t, srv, room, agents[0])
	waitPrompts(t, promptLog, 1)
}

// FS-21.A35, TS-14.R23, R27: one progression pass starts both openings as
// overlapping provider turns; the room reports both attempts and no singular
// speaker, and publishes once both settle.
func TestThinkTankOpeningsRunConcurrently(t *testing.T) {
	srv, promptLog, ids, hold := thinkTankTestServer(t)
	agents := strings.Split(ids, ",")
	members := []state.ThinkTankMember{}
	for _, id := range agents {
		members = append(members, state.ThinkTankMember{AgentID: id, AgentName: id, Project: "tmpproj", Cap: 2, MayLeave: true})
	}
	d, err := srv.stateStore.CreateThinkTank(state.ThinkTankCreate{CommandID: "open", Goal: "Choose", OriginProject: "tmpproj", Openings: true, Members: members})
	if err != nil {
		t.Fatal(err)
	}
	room := d.Room.RoomID
	srv.progressThinkTanks(context.Background())
	running := waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return len(d.Running) == 2 })
	waitPrompts(t, promptLog, 2)
	summary := thinkTankSummaryFor(running)
	if summary.ActiveAgentID != "" || len(summary.ActiveAttempts) != 2 {
		t.Fatalf("summary actors = %q %+v", summary.ActiveAgentID, summary.ActiveAttempts)
	}
	if u := thinkTankUpdateFor(running); u.CurrentActor == nil || *u.CurrentActor != "" {
		t.Fatalf("update actor = %v", u.CurrentActor)
	}
	for _, a := range running.Running {
		actAsAgent(t, srv, a, state.ThinkTankReply, "opening from "+a.AgentID)
	}
	if err := os.WriteFile(hold, []byte("go"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Room.Phase == state.ThinkTankPhaseDiscussion })
	entries, _ := srv.stateStore.ListThinkTankEntries(room, 0, 10)
	if len(entries) != 2 || entries[0].AgentID != agents[0] || entries[1].AgentID != agents[1] || done.Room.Hold != "" {
		t.Fatalf("published = %+v hold=%q", entries, done.Room.Hold)
	}
}
