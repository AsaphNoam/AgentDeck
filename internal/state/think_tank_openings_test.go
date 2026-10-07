package state

import (
	"errors"
	"fmt"
	"testing"
)

// ttOpen admits one participant's opening independently of the room
// revision, then reads its full context view.
func ttOpen(t *testing.T, st *Store, roomID, agent string, revision int64) (ThinkTankAttempt, string) {
	t.Helper()
	ttTurnSeq++
	a, err := st.BeginThinkTankAttempt(ThinkTankBegin{RoomID: roomID, Revision: revision, AgentID: agent,
		Turn: ThinkTankTurnOpening, Generation: "g1", TurnID: fmt.Sprintf("t%d", ttTurnSeq)})
	if err != nil {
		t.Fatalf("open %s: %v", agent, err)
	}
	cursor := ""
	for {
		page, err := st.ReadThinkTankPage(ThinkTankReadRequest{CallerAgentID: agent, RoomID: roomID, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if item.Entry.Kind == ThinkTankEntryOpening {
				t.Fatalf("%s saw a peer opening before the barrier", agent)
			}
		}
		if page.Complete {
			return a, page.ReadReceipt
		}
		cursor = page.NextCursor
	}
}

func ttStage(t *testing.T, st *Store, a ThinkTankAttempt, receipt, msg string) {
	t.Helper()
	if _, err := st.StageThinkTankTurn(a.AgentID, a.Token, ThinkTankReply, msg, receipt); err != nil {
		t.Fatal(err)
	}
}

func ttKinds(t *testing.T, st *Store, room string) string {
	t.Helper()
	out := ""
	for _, e := range ttEntries(t, st, room) {
		out += e.Kind + ":" + e.AgentID + " "
	}
	return out
}

// FS-21.A35, TS-14.R23: openings admit concurrently from one stale room
// revision, finish out of order, stay hidden, and publish once in member
// order after the barrier. A member never runs two room turns at once.
func TestThinkTankConcurrentOpeningsPublishInOrder(t *testing.T) {
	st, _ := newTestStore(t)
	d := ttCreate(t, st, true, ttMember("a", 2), ttMember("b", 2))
	room := d.Room.RoomID
	if got := ThinkTankOpeningOpportunities(d); len(got) != 2 {
		t.Fatalf("opportunities = %+v", got)
	}
	a, ra := ttOpen(t, st, room, "a", d.Room.Revision)
	b, rb := ttOpen(t, st, room, "b", d.Room.Revision)
	if _, err := st.BeginThinkTankAttempt(ThinkTankBegin{RoomID: room, AgentID: "a", Turn: ThinkTankTurnOpening, Generation: "g1", TurnID: "dup"}); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("second opening for a = %v", err)
	}
	if cur := mustTT(t, st, room); len(cur.Running) != 2 {
		t.Fatalf("running = %d", len(cur.Running))
	}
	ttStage(t, st, b, rb, "B opens")
	if _, err := st.FinalizeThinkTankAttempt("b", "g1", b.TurnID); err != nil {
		t.Fatal(err)
	}
	if cur := mustTT(t, st, room); cur.Room.Phase != ThinkTankPhaseOpenings || len(ttEntries(t, st, room)) != 0 {
		t.Fatalf("published before barrier: %s %s", cur.Room.Phase, ttKinds(t, st, room))
	}
	ttStage(t, st, a, ra, "A opens")
	f, err := st.FinalizeThinkTankAttempt("a", "g1", a.TurnID)
	if err != nil {
		t.Fatal(err)
	}
	if f.Detail.Room.Phase != ThinkTankPhaseDiscussion || ttKinds(t, st, room) != "opening:a opening:b " {
		t.Fatalf("after barrier: %s %s", f.Detail.Room.Phase, ttKinds(t, st, room))
	}
	for _, m := range f.Detail.Members {
		if m.Completed != 1 {
			t.Fatalf("opening accounting: %+v", m)
		}
	}
}

// FS-21.A35: a failed opening holds the room without cancelling its running
// peers; Pause settles when the last opening finishes; Resume does not
// readmit the failure; an explicit retry does, and only that member.
func TestThinkTankOpeningFailurePauseAndRetry(t *testing.T) {
	st, _ := newTestStore(t)
	d := ttCreate(t, st, true, ttMember("a", 2), ttMember("b", 2), ttMember("c", 2))
	room := d.Room.RoomID
	a, ra := ttOpen(t, st, room, "a", d.Room.Revision)
	b, _ := ttOpen(t, st, room, "b", d.Room.Revision)
	c, rc := ttOpen(t, st, room, "c", d.Room.Revision)
	if _, err := st.FailThinkTankAttempt("b", "g1", b.TurnID, "provider error"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PauseThinkTank(room); err != nil {
		t.Fatal(err)
	}
	ttStage(t, st, a, ra, "A")
	if _, err := st.FinalizeThinkTankAttempt("a", "g1", a.TurnID); err != nil {
		t.Fatal(err)
	}
	if cur := mustTT(t, st, room); cur.Room.Control != ThinkTankPauseRequested || cur.Room.Hold == "" {
		t.Fatalf("with c running: control=%s hold=%q", cur.Room.Control, cur.Room.Hold)
	}
	ttStage(t, st, c, rc, "C")
	if _, err := st.FinalizeThinkTankAttempt("c", "g1", c.TurnID); err != nil {
		t.Fatal(err)
	}
	cur := mustTT(t, st, room)
	if cur.Room.Control != ThinkTankPaused || cur.Room.Phase != ThinkTankPhaseOpenings {
		t.Fatalf("after last opening: control=%s phase=%s", cur.Room.Control, cur.Room.Phase)
	}
	cur, err := st.ResumeThinkTank(room)
	if err != nil || cur.Room.Hold == "" || len(ThinkTankOpeningOpportunities(cur)) != 0 {
		t.Fatalf("resume readmitted the failure: hold=%q %v", cur.Room.Hold, err)
	}
	if _, err := st.RetryThinkTankOpening(room, a.AttemptID); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("retry of a completed opening = %v", err)
	}
	cur, err = st.RetryThinkTankOpening(room, b.AttemptID)
	if err != nil || cur.Room.Hold != "" {
		t.Fatalf("retry = hold %q %v", cur.Room.Hold, err)
	}
	if again, err := st.RetryThinkTankOpening(room, b.AttemptID); err != nil || again.Room.Revision != cur.Room.Revision {
		t.Fatalf("retry replay = %v", err)
	}
	next := ThinkTankOpeningOpportunities(cur)
	if len(next) != 1 || next[0].AgentID != "b" {
		t.Fatalf("after retry = %+v", next)
	}
	b2, rb := ttOpen(t, st, room, "b", 0)
	ttStage(t, st, b2, rb, "B")
	if _, err := st.FinalizeThinkTankAttempt("b", "g1", b2.TurnID); err != nil {
		t.Fatal(err)
	}
	if got := ttKinds(t, st, room); got != "opening:a opening:b opening:c " {
		t.Fatalf("entries = %s", got)
	}
}

// FS-21.A35: End during concurrent openings waits for running openings, then
// publishes the completed partial set with missing markers; two failures make
// an unqualified retry ambiguous.
func TestThinkTankOpeningEndAndAmbiguousRetry(t *testing.T) {
	st, _ := newTestStore(t)
	d := ttCreate(t, st, true, ttMember("a", 2), ttMember("b", 2), ttMember("c", 2))
	room := d.Room.RoomID
	a, ra := ttOpen(t, st, room, "a", d.Room.Revision)
	b, _ := ttOpen(t, st, room, "b", d.Room.Revision)
	c, _ := ttOpen(t, st, room, "c", d.Room.Revision)
	st.FailThinkTankAttempt("b", "g1", b.TurnID, "x")
	st.FailThinkTankAttempt("c", "g1", c.TurnID, "y")
	if _, err := st.RetryThinkTankOpening(room, ""); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("ambiguous retry = %v", err)
	}
	if _, err := st.EndThinkTank(room); err != nil {
		t.Fatal(err)
	}
	if cur := mustTT(t, st, room); cur.Room.Control != ThinkTankEndRequested {
		t.Fatalf("end with a running: %s", cur.Room.Control)
	}
	ttStage(t, st, a, ra, "A")
	f, err := st.FinalizeThinkTankAttempt("a", "g1", a.TurnID)
	if err != nil {
		t.Fatal(err)
	}
	if f.Detail.Room.Phase != ThinkTankPhaseEnded || ttKinds(t, st, room) != "opening:a missing_opening:b missing_opening:c " {
		t.Fatalf("partial end: %s %s", f.Detail.Room.Phase, ttKinds(t, st, room))
	}
	if _, err := st.RetryThinkTankOpening(room, b.AttemptID); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("retry after end = %v", err)
	}
}

// FS-21.A35: restart fences every running opening for explicit recovery and
// never replays a completed peer.
func TestThinkTankRestartFencesConcurrentOpenings(t *testing.T) {
	st, _ := newTestStore(t)
	d := ttCreate(t, st, true, ttMember("a", 2), ttMember("b", 2), ttMember("c", 2))
	room := d.Room.RoomID
	a, ra := ttOpen(t, st, room, "a", d.Room.Revision)
	ttStage(t, st, a, ra, "A")
	if _, err := st.FinalizeThinkTankAttempt("a", "g1", a.TurnID); err != nil {
		t.Fatal(err)
	}
	ttOpen(t, st, room, "b", 0)
	ttOpen(t, st, room, "c", 0)
	if err := st.RecoverThinkTanks(); err != nil {
		t.Fatal(err)
	}
	cur := mustTT(t, st, room)
	if len(cur.Running) != 0 || len(unretriedThinkTankOpenings(cur)) != 2 || cur.Room.Hold == "" {
		t.Fatalf("after restart: running=%d failed=%d hold=%q", len(cur.Running), len(unretriedThinkTankOpenings(cur)), cur.Room.Hold)
	}
	if _, err := st.ResumeThinkTank(room); err != nil {
		t.Fatal(err)
	}
	if next := ThinkTankOpeningOpportunities(mustTT(t, st, room)); len(next) != 0 {
		t.Fatalf("resume readmitted fenced openings: %+v", next)
	}
}
