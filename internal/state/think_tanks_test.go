package state

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func ttMember(id string, limit int) ThinkTankMember {
	return ThinkTankMember{AgentID: id, AgentName: "Agent " + id, Project: "proj-" + id, Cap: limit, MayLeave: true}
}

func ttCreate(t *testing.T, st *Store, openings bool, members ...ThinkTankMember) ThinkTankDetail {
	t.Helper()
	d, err := st.CreateThinkTank(ThinkTankCreate{CommandID: "c-" + t.Name(), Goal: "Pick a cache design",
		OriginProject: "origin", Openings: openings, Members: members})
	if err != nil {
		t.Fatalf("CreateThinkTank: %v", err)
	}
	return d
}

var ttTurnSeq int

// ttTurn admits the room's next opportunity, reads the full context view and
// returns the attempt plus the read page sequence it saw.
func ttTurn(t *testing.T, st *Store, roomID string) (ThinkTankAttempt, []ThinkTankReadItem, string) {
	t.Helper()
	d, err := st.ReadThinkTank(roomID)
	if err != nil {
		t.Fatal(err)
	}
	next, ok := NextThinkTankOpportunity(d)
	if !ok {
		t.Fatalf("no opportunity: phase=%s control=%s hold=%q", d.Room.Phase, d.Room.Control, d.Room.Hold)
	}
	ttTurnSeq++
	a, err := st.BeginThinkTankAttempt(ThinkTankBegin{RoomID: roomID, Revision: d.Room.Revision,
		AgentID: next.AgentID, Turn: next.Turn, Generation: "g1", TurnID: fmt.Sprintf("t%d", ttTurnSeq)})
	if err != nil {
		t.Fatalf("BeginThinkTankAttempt: %v", err)
	}
	items := []ThinkTankReadItem{}
	cursor, receipt := "", ""
	for {
		page, err := st.ReadThinkTankPage(ThinkTankReadRequest{CallerAgentID: a.AgentID, Cursor: cursor})
		if err != nil {
			t.Fatalf("ReadThinkTankPage: %v", err)
		}
		items = append(items, page.Items...)
		if page.Complete {
			receipt = page.ReadReceipt
			if page.TurnToken != a.Token {
				t.Fatalf("turn token = %q, want current attempt token", page.TurnToken)
			}
			break
		}
		cursor = page.NextCursor
	}
	return a, items, receipt
}

func ttSubmit(t *testing.T, st *Store, a ThinkTankAttempt, receipt, disposition, msg string) ThinkTankFinish {
	t.Helper()
	if _, err := st.StageThinkTankTurn(a.AgentID, a.Token, disposition, msg, receipt); err != nil {
		t.Fatalf("StageThinkTankTurn: %v", err)
	}
	f, err := st.FinalizeThinkTankAttempt(a.AgentID, a.Generation, a.TurnID)
	if err != nil {
		t.Fatalf("FinalizeThinkTankAttempt: %v", err)
	}
	return f
}

func ttEntries(t *testing.T, st *Store, roomID string) []ThinkTankEntry {
	t.Helper()
	e, err := st.ListThinkTankEntries(roomID, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestThinkTankAnnotationMailIsAtomicAndReplayed(t *testing.T) {
	st, _ := newTestStore(t)
	d := ttCreate(t, st, false, ttMember("a", 2), ttMember("b", 2))
	if err := st.WriteAgent(Agent{AgentID: "b", Name: "B", Interface: "chat", CreatedAt: timeNow()}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`CREATE TRIGGER refuse_annotation_mail BEFORE INSERT ON messages BEGIN SELECT RAISE(ABORT, 'mail unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	mail := Message{FromAgent: "user", ToAgent: "b", Subject: "Think Tank annotations"}
	if _, _, err := st.AddThinkTankAnnotationMail(d.Room.RoomID, "annotation", "instruction", "target:b", mail); err == nil {
		t.Fatal("mail failure accepted")
	}
	if entries := ttEntries(t, st, d.Room.RoomID); len(entries) != 0 {
		t.Fatalf("failed mail published room history: %+v", entries)
	}
	var inputs int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM think_tank_inputs WHERE room_id = ?`, d.Room.RoomID).Scan(&inputs); err != nil || inputs != 0 {
		t.Fatalf("failed mail retained input: %d %v", inputs, err)
	}
	if _, err := st.db.Exec(`DROP TRIGGER refuse_annotation_mail`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`CREATE TRIGGER refuse_annotation_wake BEFORE INSERT ON activations BEGIN SELECT RAISE(ABORT, 'wake unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.AddThinkTankAnnotationMail(d.Room.RoomID, "annotation", "instruction", "target:b", mail); err == nil {
		t.Fatal("wake failure accepted")
	}
	if messages, err := st.ListMessages("b", false, 10); err != nil || len(messages) != 0 || len(ttEntries(t, st, d.Room.RoomID)) != 0 {
		t.Fatalf("wake failure retained mail/history: %+v %v", messages, err)
	}
	if _, err := st.db.Exec(`DROP TRIGGER refuse_annotation_wake`); err != nil {
		t.Fatal(err)
	}
	first, _, err := st.AddThinkTankAnnotationMail(d.Room.RoomID, "annotation", "instruction", "target:b", mail)
	if err != nil {
		t.Fatal(err)
	}
	// Replaying after successful commit models a response lost to the caller.
	second, _, err := st.AddThinkTankAnnotationMail(d.Room.RoomID, "annotation", "instruction", "target:b", mail)
	if err != nil || first.InputID != second.InputID || first.EntrySeq != second.EntrySeq {
		t.Fatalf("replay: %+v %v", second, err)
	}
	messages, err := st.ListMessages("b", false, 10)
	if err != nil || len(messages) != 1 || messages[0].Body != "instruction" || !messages[0].Wake {
		t.Fatalf("mail: %+v %v", messages, err)
	}
	if len(ttEntries(t, st, d.Room.RoomID)) != 1 {
		t.Fatal("replay duplicated room history")
	}
	if wake, err := st.PendingActivations(ActivationKindMail, "b", 10); err != nil || len(wake) != 1 {
		t.Fatalf("wake receipt: %+v %v", wake, err)
	}
}

func TestThinkTankReadRejectsMalformedAndForeignContinuations(t *testing.T) {
	st, _ := newTestStore(t)
	room := ttCreate(t, st, false, ttMember("a", 3), ttMember("b", 3)).Room.RoomID
	if _, _, err := st.AddThinkTankInput(room, "text", ThinkTankEntryUser, "aéz", ""); err != nil {
		t.Fatal(err)
	}
	d, _ := st.ReadThinkTank(room)
	a, err := st.BeginThinkTankAttempt(ThinkTankBegin{RoomID: room, Revision: d.Room.Revision, AgentID: "a", Turn: ThinkTankTurnDiscussion, Generation: "g", TurnID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	base := thinkTankCursor{room: room, view: ThinkTankViewContext, head: a.Head, seq: 1, caller: "a", attempt: a.AttemptID}
	for name, change := range map[string]func(*thinkTankCursor){
		"past end":       func(c *thinkTankCursor) { c.off = 999 },
		"mid rune":       func(c *thinkTankCursor) { c.off = 2 },
		"at end":         func(c *thinkTankCursor) { c.off = 4 },
		"past head":      func(c *thinkTankCursor) { c.seq = c.head + 1 },
		"foreign caller": func(c *thinkTankCursor) { c.caller = "b" },
		"old attempt":    func(c *thinkTankCursor) { c.attempt = "old" },
	} {
		t.Run(name, func(t *testing.T) {
			c := base
			change(&c)
			if _, err := st.ReadThinkTankPage(ThinkTankReadRequest{CallerAgentID: "a", Cursor: c.encode()}); !errors.Is(err, ErrThinkTankCursor) {
				t.Fatalf("cursor refusal: %v", err)
			}
			var delivered, offset int
			if err := st.db.QueryRow(`SELECT delivered_to, delivered_at FROM think_tank_attempts WHERE attempt_id = ?`, a.AttemptID).Scan(&delivered, &offset); err != nil || delivered != 0 || offset != 0 {
				t.Fatalf("invalid cursor advanced delivery: %d %d %v", delivered, offset, err)
			}
		})
	}
	base.off = 1
	if p, err := st.ReadThinkTankPage(ThinkTankReadRequest{CallerAgentID: "a", Cursor: base.encode()}); err != nil || len(p.Items) != 1 || p.Items[0].Entry.Body != "éz" {
		t.Fatalf("valid rune boundary: %+v %v", p, err)
	}
}

func TestThinkTankCreateValidatesAndReplays(t *testing.T) {
	st, _ := newTestStore(t)
	bad := []ThinkTankCreate{
		{CommandID: "x", Goal: "g", OriginProject: "p", Members: []ThinkTankMember{ttMember("a", 1)}},
		{CommandID: "x", Goal: "g", OriginProject: "p", Members: []ThinkTankMember{ttMember("a", 1), ttMember("a", 1)}},
		{CommandID: "x", Goal: "g", OriginProject: "p", Members: []ThinkTankMember{ttMember("a", 0), ttMember("b", 1)}},
		{CommandID: "x", Goal: "  ", OriginProject: "p", Members: []ThinkTankMember{ttMember("a", 1), ttMember("b", 1)}},
		{CommandID: "x", Goal: strings.Repeat("é", ThinkTankMaxGoalRunes+1), OriginProject: "p", Members: []ThinkTankMember{ttMember("a", 1), ttMember("b", 1)}},
	}
	for i, c := range bad {
		if _, err := st.CreateThinkTank(c); !errors.Is(err, ErrThinkTankInvalid) {
			t.Fatalf("case %d: err = %v, want invalid", i, err)
		}
	}
	c := ThinkTankCreate{CommandID: "same", Goal: "g", OriginProject: "p", Members: []ThinkTankMember{ttMember("a", 2), ttMember("b", 3)}}
	first, err := st.CreateThinkTank(c)
	if err != nil {
		t.Fatal(err)
	}
	again, err := st.CreateThinkTank(c)
	if err != nil || again.Room.RoomID != first.Room.RoomID {
		t.Fatalf("replay = %v %v, want original room", again.Room.RoomID, err)
	}
	c.Goal = "different"
	if _, err := st.CreateThinkTank(c); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("conflicting replay err = %v", err)
	}
	if first.Room.Phase != ThinkTankPhaseDiscussion || first.Room.JudgeStatus != ThinkTankJudgeNone {
		t.Fatalf("defaults: phase=%s judge=%q", first.Room.Phase, first.Room.JudgeStatus)
	}
}

func TestThinkTankCreateReplayUsesImmutableIntent(t *testing.T) {
	st, _ := newTestStore(t)
	c := ThinkTankCreate{CommandID: "immutable", Goal: "g", OriginProject: "p", JudgeConfig: "original judge",
		Members: []ThinkTankMember{ttMember("a", 2), {Cap: 3, MayLeave: true, SetupConfig: "new participant"}}}
	first, err := st.CreateThinkTank(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE think_tanks SET judge_config = 'repaired judge' WHERE room_id = ?`, first.Room.RoomID); err != nil {
		t.Fatal(err)
	}
	again, err := st.CreateThinkTank(c)
	if err != nil || again.Room.RoomID != first.Room.RoomID || again.Members[1].AgentID != first.Members[1].AgentID {
		t.Fatalf("replay after judge repair: %+v %v", again, err)
	}
	c.Members[1].SetupConfig = "different participant"
	if _, err := st.CreateThinkTank(c); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("changed new participant intent accepted: %v", err)
	}
}

// FS-21.A33, TS-14.R22: explicit titles are normalized and fixed; omitted
// titles fall back to the folded goal without changing it; replay keeps the
// original title and a different title under the same command conflicts.
func TestThinkTankTitleFallbackAndReplay(t *testing.T) {
	st, _ := newTestStore(t)
	members := []ThinkTankMember{ttMember("a", 1), ttMember("b", 1)}
	titled := ThinkTankCreate{CommandID: "titled", Title: "  Cache \n plan ", Goal: "g", OriginProject: "p", Members: members}
	d, err := st.CreateThinkTank(titled)
	if err != nil || d.Room.Title != "Cache plan" {
		t.Fatalf("titled = %q %v", d.Room.Title, err)
	}
	if again, err := st.CreateThinkTank(titled); err != nil || again.Room.RoomID != d.Room.RoomID {
		t.Fatalf("replay = %v", err)
	}
	titled.Title = "Other"
	if _, err := st.CreateThinkTank(titled); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("retitled replay err = %v", err)
	}
	long := strings.Repeat("é", ThinkTankMaxTitleRunes+1)
	if _, err := st.CreateThinkTank(ThinkTankCreate{CommandID: "long", Title: long, Goal: "g", OriginProject: "p", Members: members}); !errors.Is(err, ErrThinkTankInvalid) {
		t.Fatalf("long title err = %v", err)
	}
	goal := "Pick\n\n  a   cache " + long
	fb, err := st.CreateThinkTank(ThinkTankCreate{CommandID: "fallback", Title: " ", Goal: goal, OriginProject: "p", Members: members})
	if err != nil {
		t.Fatal(err)
	}
	if fb.Room.Goal != goal || utf8.RuneCountInString(fb.Room.Title) != ThinkTankMaxTitleRunes || !strings.HasPrefix(fb.Room.Title, "Pick a cache é") {
		t.Fatalf("fallback title = %q goal changed=%v", fb.Room.Title, fb.Room.Goal != goal)
	}
	// The forward migration backfills only the title of existing rows.
	if _, err := st.db.Exec(`ALTER TABLE think_tanks DROP COLUMN title`); err != nil {
		t.Fatal(err)
	}
	tx, err := st.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrateThinkTankTitles(tx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	back, err := st.ReadThinkTank(d.Room.RoomID)
	if err != nil || back.Room.Title != "g" || back.Room.Goal != "g" {
		t.Fatalf("backfilled = %q/%q %v", back.Room.Title, back.Room.Goal, err)
	}
}

// FS-21.A1, A2, A8, A12: attributed single-floor turns, delta reads that skip
// one's own entries, ceilings, closing opportunity and stop reason.
func TestThinkTankDiscussionRotationReadsAndClosing(t *testing.T) {
	st, _ := newTestStore(t)
	d := ttCreate(t, st, false, ttMember("a", 1), ttMember("b", 2), ttMember("c", 3))
	room := d.Room.RoomID

	a1, items, receipt := ttTurn(t, st, room)
	if a1.AgentID != "a" || len(items) != 0 {
		t.Fatalf("first turn = %s with %d items", a1.AgentID, len(items))
	}
	// One floor: no second attempt can be admitted while one runs.
	if _, ok := NextThinkTankOpportunity(mustTT(t, st, room)); ok {
		t.Fatal("second opportunity offered while a turn is active")
	}
	ttSubmit(t, st, a1, receipt, ThinkTankReply, "A says LRU")

	b1, items, receipt := ttTurn(t, st, room)
	if b1.AgentID != "b" || len(items) != 1 || items[0].Entry.Body != "A says LRU" || items[0].Entry.AgentName != "Agent a" {
		t.Fatalf("b's first read = %+v", items)
	}
	ttSubmit(t, st, b1, receipt, ThinkTankReply, "B objects: ARC")

	c1, items, receipt := ttTurn(t, st, room)
	if c1.AgentID != "c" || len(items) != 2 {
		t.Fatalf("c's read = %d items", len(items))
	}
	ttSubmit(t, st, c1, receipt, ThinkTankReply, "C: measure first")

	// a is exhausted (limit 1); rotation skips it.
	b2, items, receipt := ttTurn(t, st, room)
	if b2.AgentID != "b" {
		t.Fatalf("rotation chose %s, want b", b2.AgentID)
	}
	if len(items) != 2 || !items[0].Own || items[0].Entry.Body != "" || items[1].Entry.Body != "C: measure first" {
		t.Fatalf("b's delta read = %+v", items)
	}
	f := ttSubmit(t, st, b2, receipt, ThinkTankReply, "B still objects")
	// b and a exhausted, only c remains: closing opportunity.
	if f.Detail.Room.Phase != ThinkTankPhaseClosing || f.Detail.Room.EndReason != ThinkTankEndAllowanceExhausted {
		t.Fatalf("after b2: phase=%s reason=%s", f.Detail.Room.Phase, f.Detail.Room.EndReason)
	}
	cl, _, receipt := ttTurn(t, st, room)
	if cl.AgentID != "c" || cl.Turn != ThinkTankTurnClosing {
		t.Fatalf("closing turn = %s/%s", cl.AgentID, cl.Turn)
	}
	f = ttSubmit(t, st, cl, receipt, ThinkTankDeclineClosing, "")
	if f.Detail.Room.Phase != ThinkTankPhaseEnded || f.Detail.Room.EndReason != ThinkTankEndAllowanceExhausted {
		t.Fatalf("after closing: %s/%s", f.Detail.Room.Phase, f.Detail.Room.EndReason)
	}
	counts := map[string]int{}
	for _, m := range f.Detail.Members {
		counts[m.AgentID] = m.Completed
	}
	if counts["a"] != 1 || counts["b"] != 2 || counts["c"] != 1 {
		t.Fatalf("declining the closing turn must not charge: %v", counts)
	}
	entries := ttEntries(t, st, room)
	if len(entries) != 4 || entries[3].Body != "B still objects" {
		t.Fatalf("entries = %+v", entries)
	}
	if _, ok := NextThinkTankOpportunity(f.Detail); ok {
		t.Fatal("ended room offers a turn")
	}
}

func mustTT(t *testing.T, st *Store, room string) ThinkTankDetail {
	t.Helper()
	d, err := st.ReadThinkTank(room)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// FS-21.A2: multi-page UTF-8-safe delivery; a read interrupted before
// publication leaves entries available to a retry.
func TestThinkTankPagedReadsAndInterruptedRetry(t *testing.T) {
	st, _ := newTestStore(t)
	room := ttCreate(t, st, false, ttMember("a", 5), ttMember("b", 5)).Room.RoomID
	big := strings.Repeat("ü", ThinkTankPageBytes) // 2 bytes per rune: spans pages
	a1, _, receipt := ttTurn(t, st, room)
	ttSubmit(t, st, a1, receipt, ThinkTankReply, big)

	d := mustTT(t, st, room)
	b1, err := st.BeginThinkTankAttempt(ThinkTankBegin{RoomID: room, Revision: d.Room.Revision, AgentID: "b",
		Turn: ThinkTankTurnDiscussion, Generation: "g1", TurnID: "tb1"})
	if err != nil {
		t.Fatal(err)
	}
	page, err := st.ReadThinkTankPage(ThinkTankReadRequest{CallerAgentID: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Complete || page.ReadReceipt != "" || !page.First || page.Room.Goal == "" {
		t.Fatalf("first page complete=%v receipt=%q", page.Complete, page.ReadReceipt)
	}
	if _, err := st.StageThinkTankTurn("b", b1.Token, ThinkTankReply, "early", "bogus"); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("submission before full delivery err = %v", err)
	}
	// Interrupt: the turn fails before publishing.
	f, err := st.FailThinkTankAttempt("b", "g1", "tb1", "cancelled")
	if err != nil {
		t.Fatal(err)
	}
	if f.Detail.Room.Hold == "" {
		t.Fatal("failed turn must hold the room")
	}
	if _, ok := NextThinkTankOpportunity(f.Detail); ok {
		t.Fatal("failed turn retried automatically")
	}
	if _, err := st.ResumeThinkTank(room); err != nil {
		t.Fatal(err)
	}
	b2, items, receipt := ttTurn(t, st, room)
	if b2.AgentID != "b" || b2.Token == b1.Token {
		t.Fatalf("retry = %s token reused=%v", b2.AgentID, b2.Token == b1.Token)
	}
	var got strings.Builder
	for i, it := range items {
		if !utf8.ValidString(it.Entry.Body) {
			t.Fatalf("page %d split a rune", i)
		}
		got.WriteString(it.Entry.Body)
	}
	if got.String() != big || len(items) < 2 {
		t.Fatalf("reassembled %d bytes over %d pages, want %d", got.Len(), len(items), len(big))
	}
	ttSubmit(t, st, b2, receipt, ThinkTankReply, "ok")
	for _, m := range mustTT(t, st, room).Members {
		if m.AgentID == "b" && m.Completed != 1 {
			t.Fatalf("failed attempt charged allowance: completed=%d", m.Completed)
		}
	}
	// History revisits older entries without being a current-attempt read.
	hist, err := st.ReadThinkTankPage(ThinkTankReadRequest{CallerAgentID: "a", RoomID: room, View: ThinkTankViewHistory})
	if err != nil || hist.Attempt != nil || hist.TurnToken != "" || len(hist.Items) == 0 || hist.Items[0].Entry.Seq != 1 {
		t.Fatalf("history read = %+v, %v", hist.Items, err)
	}
}

// FS-21.A3, A25: openings are withheld until the barrier; input during
// openings waits and publishes after them.
func TestThinkTankIndependentOpenings(t *testing.T) {
	st, _ := newTestStore(t)
	d := ttCreate(t, st, true, ttMember("a", 3), ttMember("b", 3))
	room := d.Room.RoomID
	if d.Room.Phase != ThinkTankPhaseOpenings {
		t.Fatalf("phase = %s", d.Room.Phase)
	}
	a1, _, receipt := ttTurn(t, st, room)
	ttSubmit(t, st, a1, receipt, ThinkTankReply, "A opening")
	if _, _, err := st.AddThinkTankInput(room, "u1", ThinkTankEntryUser, "operator note", ""); err != nil {
		t.Fatal(err)
	}
	b1, items, receipt := ttTurn(t, st, room)
	if b1.Turn != ThinkTankTurnOpening || len(items) != 0 {
		t.Fatalf("b's opening read %d items", len(items))
	}
	hist, err := st.ReadThinkTankPage(ThinkTankReadRequest{CallerAgentID: "b", RoomID: room, View: ThinkTankViewHistory})
	if err != nil || len(hist.Items) != 0 {
		t.Fatalf("history during openings exposed %d items", len(hist.Items))
	}
	f := ttSubmit(t, st, b1, receipt, ThinkTankReply, "B opening")
	if f.Detail.Room.Phase != ThinkTankPhaseDiscussion {
		t.Fatalf("phase after barrier = %s", f.Detail.Room.Phase)
	}
	e := ttEntries(t, st, room)
	if len(e) != 3 || e[0].Body != "A opening" || e[1].Body != "B opening" || e[2].Kind != ThinkTankEntryUser {
		t.Fatalf("barrier publication = %+v", e)
	}
	for _, m := range f.Detail.Members {
		if m.Completed != 1 {
			t.Fatalf("opening charge for %s = %d", m.AgentID, m.Completed)
		}
	}
	a2, items, _ := ttTurn(t, st, room)
	if a2.AgentID != "a" || len(items) != 3 || !items[0].Own || items[1].Entry.Body != "B opening" {
		t.Fatalf("first discussion read = %+v", items)
	}
}

// FS-21.A9, A22: pause and end requested during a turn take effect at its
// boundary; queued input publishes after the contribution.
func TestThinkTankPauseAndEndAtBoundary(t *testing.T) {
	st, _ := newTestStore(t)
	room := ttCreate(t, st, false, ttMember("a", 5), ttMember("b", 5)).Room.RoomID
	a1, _, receipt := ttTurn(t, st, room)
	d, err := st.PauseThinkTank(room)
	if err != nil || d.Room.Control != ThinkTankPauseRequested {
		t.Fatalf("pause during turn = %s %v", d.Room.Control, err)
	}
	if _, _, err := st.AddThinkTankInput(room, "u1", ThinkTankEntryUser, "queued", ""); err != nil {
		t.Fatal(err)
	}
	f := ttSubmit(t, st, a1, receipt, ThinkTankReply, "A")
	if f.Detail.Room.Control != ThinkTankPaused || len(f.Published) != 2 || f.Published[1].Body != "queued" {
		t.Fatalf("boundary: control=%s published=%+v", f.Detail.Room.Control, f.Published)
	}
	if _, ok := NextThinkTankOpportunity(f.Detail); ok {
		t.Fatal("paused room offers a turn")
	}
	in, _, err := st.AddThinkTankInput(room, "u2", ThinkTankEntryUser, "while paused", "")
	if err != nil || in.EntrySeq == 0 {
		t.Fatalf("input between turns should publish at once: %+v %v", in, err)
	}
	if again, _, err := st.AddThinkTankInput(room, "u2", ThinkTankEntryUser, "while paused", ""); err != nil || again.InputID != in.InputID {
		t.Fatalf("input replay = %v %v", again.InputID, err)
	}
	if _, err := st.ResumeThinkTank(room); err != nil {
		t.Fatal(err)
	}
	b1, items, receipt := ttTurn(t, st, room)
	if len(items) != 3 {
		t.Fatalf("b read %d items, want contribution and two inputs", len(items))
	}
	if d, _ := st.EndThinkTank(room); d.Room.Control != ThinkTankEndRequested {
		t.Fatalf("end during turn control = %s", d.Room.Control)
	}
	f = ttSubmit(t, st, b1, receipt, ThinkTankReply, "B objection stands")
	if f.Detail.Room.Phase != ThinkTankPhaseEnded || f.Detail.Room.EndReason != ThinkTankEndOperator {
		t.Fatalf("after end: %s/%s", f.Detail.Room.Phase, f.Detail.Room.EndReason)
	}
	if _, _, err := st.AddThinkTankInput(room, "u3", ThinkTankEntryUser, "late", ""); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("room input after end err = %v", err)
	}
}

// FS-21.A27, R37: End during openings publishes a partial set with missing
// markers and marks undiscussed input.
func TestThinkTankEndDuringOpenings(t *testing.T) {
	st, _ := newTestStore(t)
	room := ttCreate(t, st, true, ttMember("a", 3), ttMember("b", 3)).Room.RoomID
	a1, _, receipt := ttTurn(t, st, room)
	ttSubmit(t, st, a1, receipt, ThinkTankReply, "A opening")
	if _, _, err := st.AddThinkTankInput(room, "u1", ThinkTankEntryUser, "pending", ""); err != nil {
		t.Fatal(err)
	}
	d, err := st.EndThinkTank(room)
	if err != nil || d.Room.Phase != ThinkTankPhaseEnded {
		t.Fatalf("end = %s %v", d.Room.Phase, err)
	}
	e := ttEntries(t, st, room)
	if len(e) != 3 || e[0].Kind != ThinkTankEntryOpening || e[1].Kind != ThinkTankEntryMissingOpening ||
		e[1].AgentID != "b" || e[2].Kind != ThinkTankEntryUser || !e[2].Undiscussed {
		t.Fatalf("partial openings = %+v", e)
	}
}

// FS-21.A4, A6, A28: departure permission, departure with and without a
// message, stale/foreign submissions, and missing submission.
func TestThinkTankDepartureAndAuthority(t *testing.T) {
	st, _ := newTestStore(t)
	noLeave := ttMember("b", 5)
	noLeave.MayLeave = false
	room := ttCreate(t, st, false, ttMember("a", 5), noLeave, ttMember("c", 5)).Room.RoomID

	a1, _, receipt := ttTurn(t, st, room)
	if _, err := st.StageThinkTankTurn("b", a1.Token, ThinkTankReply, "x", receipt); !errors.Is(err, ErrThinkTankNotMember) {
		t.Fatalf("foreign token err = %v", err)
	}
	if _, err := st.FinalizeThinkTankAttempt("a", "g1", "wrong-turn"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale turn finalize err = %v", err)
	}
	f := ttSubmit(t, st, a1, receipt, ThinkTankLeave, "")
	if len(f.Published) != 1 || f.Published[0].Kind != ThinkTankEntryDeparture {
		t.Fatalf("silent departure = %+v", f.Published)
	}
	if _, err := st.StageThinkTankTurn("a", a1.Token, ThinkTankReply, "again", receipt); !errors.Is(err, ErrThinkTankStale) {
		t.Fatalf("finished token err = %v", err)
	}

	b1, _, receipt := ttTurn(t, st, room)
	if _, err := st.StageThinkTankTurn("b", b1.Token, ThinkTankLeave, "bye", receipt); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("unauthorized leave err = %v", err)
	}
	// No submission at all: the turn fails rather than inventing a reply.
	f, err := st.FinalizeThinkTankAttempt("b", "g1", b1.TurnID)
	if err != nil || f.Attempt.State != ThinkTankAttemptFailed || len(f.Published) != 0 {
		t.Fatalf("missing submission = %s %v", f.Attempt.State, err)
	}
	st.ResumeThinkTank(room)
	b2, _, receipt := ttTurn(t, st, room)
	ttSubmit(t, st, b2, receipt, ThinkTankReply, "B")
	c1, _, receipt := ttTurn(t, st, room)
	f = ttSubmit(t, st, c1, receipt, ThinkTankLeave, "C leaves with a note")
	if f.Detail.Room.Phase != ThinkTankPhaseClosing || f.Detail.Room.EndReason != ThinkTankEndParticipantsLeft {
		t.Fatalf("after departures: %s/%s", f.Detail.Room.Phase, f.Detail.Room.EndReason)
	}
	cl, _, receipt := ttTurn(t, st, room)
	f = ttSubmit(t, st, cl, receipt, ThinkTankReply, "B closing")
	e := ttEntries(t, st, room)
	if f.Detail.Room.Phase != ThinkTankPhaseEnded || e[len(e)-1].Kind != ThinkTankEntryClosing {
		t.Fatalf("closing publication = %+v", e[len(e)-1])
	}
	// Leavers keep retained read access; nonmembers do not.
	if _, err := st.ReadThinkTankPage(ThinkTankReadRequest{CallerAgentID: "a", RoomID: room, View: ThinkTankViewHistory}); err != nil {
		t.Fatalf("leaver read: %v", err)
	}
	if _, err := st.ReadThinkTankPage(ThinkTankReadRequest{CallerAgentID: "z", RoomID: room}); !errors.Is(err, ErrThinkTankNotMember) {
		t.Fatalf("nonmember read err = %v", err)
	}
}

// FS-21.A7, A21, A26: the judge starts only after discussion ends, publishes
// one synthesis, and a failed judge retries without reopening discussion.
func TestThinkTankJudgeLifecycle(t *testing.T) {
	st, _ := newTestStore(t)
	d, err := st.CreateThinkTank(ThinkTankCreate{CommandID: "j", Goal: "g", OriginProject: "p",
		JudgeConfig: `{"backend":"fake"}`, Members: []ThinkTankMember{ttMember("a", 1), ttMember("b", 1)}})
	if err != nil {
		t.Fatal(err)
	}
	room := d.Room.RoomID
	if d.Room.JudgeStatus != ThinkTankJudgeWaiting {
		t.Fatalf("judge = %q", d.Room.JudgeStatus)
	}
	if _, err := st.ReserveThinkTankJudge(room, "j1", "Judge", "p"); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("judge during discussion err = %v", err)
	}
	a1, _, r := ttTurn(t, st, room)
	ttSubmit(t, st, a1, r, ThinkTankReply, "A")
	b1, _, r := ttTurn(t, st, room)
	f := ttSubmit(t, st, b1, r, ThinkTankReply, "B")
	if f.Detail.Room.Phase != ThinkTankPhaseEnded || f.Detail.Room.JudgeStatus != ThinkTankJudgeReady {
		t.Fatalf("after budget: %s judge=%s", f.Detail.Room.Phase, f.Detail.Room.JudgeStatus)
	}
	if _, err := st.ReserveThinkTankJudge(room, "j1", "Judge", "p"); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteThinkTank(room); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("delete while judge launching err = %v", err)
	}
	if _, err := st.MarkThinkTankJudgeLaunched(room, "Judge", ""); err != nil {
		t.Fatal(err)
	}
	j, items, r := ttTurn(t, st, room)
	if j.AgentID != "j1" || j.Turn != ThinkTankTurnJudge || len(items) != 2 {
		t.Fatalf("judge turn %s/%s read %d", j.AgentID, j.Turn, len(items))
	}
	if _, err := st.FailThinkTankAttempt("j1", "g1", j.TurnID, "provider error"); err != nil {
		t.Fatal(err)
	}
	d = mustTT(t, st, room)
	if d.Room.JudgeStatus != ThinkTankJudgeFailed || d.Room.Phase != ThinkTankPhaseEnded {
		t.Fatalf("judge failure: %s/%s", d.Room.JudgeStatus, d.Room.Phase)
	}
	if _, err := st.RetryThinkTankJudge(room, `{"backend":"fixed"}`); err != nil {
		t.Fatal(err)
	}
	st.ReserveThinkTankJudge(room, "j2", "Judge 2", "p")
	st.MarkThinkTankJudgeLaunched(room, "Judge", "")
	j2, _, r := ttTurn(t, st, room)
	f = ttSubmit(t, st, j2, r, ThinkTankReply, "Synthesis; B's objection unresolved")
	if f.Detail.Room.JudgeStatus != ThinkTankJudgeCompleted || f.Published[0].Kind != ThinkTankEntrySynthesis {
		t.Fatalf("synthesis = %s %+v", f.Detail.Room.JudgeStatus, f.Published)
	}
	for _, m := range f.Detail.Members {
		if m.Role == ThinkTankRoleParticipant && m.Completed != 1 {
			t.Fatalf("judge changed participant budget: %+v", m)
		}
	}
	if err := st.DeleteThinkTank(room); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReadThinkTank(room); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted room read err = %v", err)
	}
}

// FS-21.A27, A30: restart fences a running attempt and holds the room;
// deletion is refused while a turn is active.
func TestThinkTankRecoveryAndDeletionGuard(t *testing.T) {
	st, _ := newTestStore(t)
	room := ttCreate(t, st, false, ttMember("a", 2), ttMember("b", 2)).Room.RoomID
	a1, _, _ := ttTurn(t, st, room)
	st.PauseThinkTank(room)
	if err := st.DeleteThinkTank(room); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("delete with active turn err = %v", err)
	}
	if err := st.RecoverThinkTanks(); err != nil {
		t.Fatal(err)
	}
	d := mustTT(t, st, room)
	if d.Active != nil || d.Room.Control != ThinkTankPaused || d.Room.Hold == "" {
		t.Fatalf("after recovery active=%v control=%s hold=%q", d.Active, d.Room.Control, d.Room.Hold)
	}
	if _, err := st.FinalizeThinkTankAttempt("a", "g1", a1.TurnID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pre-restart completion finalized: %v", err)
	}
	for _, m := range d.Members {
		if m.Completed != 0 {
			t.Fatalf("recovery charged %s", m.AgentID)
		}
	}
	if err := st.DeleteThinkTank(room); err != nil {
		t.Fatal(err)
	}
}

func TestThinkTankSetupSlots(t *testing.T) {
	st, _ := newTestStore(t)
	fresh := ttMember("n", 2)
	fresh.SetupConfig = `{"backend":"fake"}`
	d := ttCreate(t, st, false, ttMember("a", 2), fresh)
	if d.Room.Phase != ThinkTankPhaseSetup {
		t.Fatalf("phase = %s", d.Room.Phase)
	}
	if _, ok := NextThinkTankOpportunity(d); ok {
		t.Fatal("setup room offers a turn")
	}
	d, err := st.MarkThinkTankMemberSetup(d.Room.RoomID, "n", "", "launch failed")
	if err != nil || d.Room.Hold == "" || d.Room.Phase != ThinkTankPhaseSetup {
		t.Fatalf("failed slot: %+v %v", d.Room, err)
	}
	d, _ = st.RetryThinkTankSetup(d.Room.RoomID)
	d, err = st.MarkThinkTankMemberSetup(d.Room.RoomID, "n", "Nova", "")
	if err != nil || d.Room.Phase != ThinkTankPhaseDiscussion || d.Room.Hold != "" {
		t.Fatalf("after setup: %s hold=%q %v", d.Room.Phase, d.Room.Hold, err)
	}
	if _, err := st.MarkThinkTankMemberSetup(d.Room.RoomID, "a", "", ""); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("relaunching a ready slot err = %v", err)
	}
}

// TT2-04, TS-14.R17: the per-attempt activity budget counts bytes, so
// multibyte payloads reach the cap at its byte size, not its character count.
func TestThinkTankActivityBudgetCountsBytes(t *testing.T) {
	st, _ := newTestStore(t)
	room := ttCreate(t, st, false, ttMember("a", 2), ttMember("b", 2)).Room.RoomID
	a, _, _ := ttTurn(t, st, room)
	payload := `"` + strings.Repeat("é", 7<<20/2) + `"` // ~7 MiB, half as many characters
	var truncated []int
	for i := 0; i < 10; i++ {
		rec, err := st.AppendThinkTankActivity(ThinkTankActivity{RoomID: room, AttemptID: a.AttemptID, AgentID: a.AgentID,
			Generation: a.Generation, TurnID: a.TurnID, Payload: payload}, "tool_call")
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		if rec.Truncated {
			truncated = append(truncated, i)
		}
	}
	if len(truncated) != 1 || truncated[0] != 9 {
		t.Fatalf("truncated records = %v, want only the one past 64 MiB", truncated)
	}
}

// FS-21.A36, TS-14.R24: a live ceiling increase restores an exhausted
// member's eligibility without changing counters or control, replays exactly,
// refuses stale or conflicting commands and closes with discussion.
func TestThinkTankTurnLimitIncrease(t *testing.T) {
	st, _ := newTestStore(t)
	d := ttCreate(t, st, false, ttMember("a", 1), ttMember("b", 3), ttMember("c", 3))
	room := d.Room.RoomID
	a1, _, receipt := ttTurn(t, st, room)
	ttSubmit(t, st, a1, receipt, ThinkTankReply, "A")
	if _, err := st.PauseThinkTank(room); err != nil {
		t.Fatal(err)
	}
	change := ThinkTankLimitChange{RoomID: room, AgentID: "a", CommandID: "lim-1", Expected: 1, Limit: 3}
	got, err := st.IncreaseThinkTankTurnLimit(change)
	if err != nil {
		t.Fatal(err)
	}
	var a ThinkTankMember
	for _, m := range got.Members {
		if m.AgentID == "a" {
			a = m
		}
	}
	if a.Cap != 3 || a.Completed != 1 || a.State != ThinkTankMemberActive || got.Room.Control != ThinkTankPaused {
		t.Fatalf("after increase: %+v control=%s", a, got.Room.Control)
	}
	if again, err := st.IncreaseThinkTankTurnLimit(change); err != nil || again.Room.Revision != got.Room.Revision {
		t.Fatalf("replay = %v rev %d/%d", err, again.Room.Revision, got.Room.Revision)
	}
	conflicting := change
	conflicting.Limit = 4
	if _, err := st.IncreaseThinkTankTurnLimit(conflicting); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("conflicting reuse = %v", err)
	}
	stale := ThinkTankLimitChange{RoomID: room, AgentID: "a", CommandID: "lim-2", Expected: 1, Limit: 5}
	if _, err := st.IncreaseThinkTankTurnLimit(stale); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("stale expected = %v", err)
	}
	for _, bad := range []ThinkTankLimitChange{
		{RoomID: room, AgentID: "a", CommandID: "x", Expected: 3, Limit: 2},
		{RoomID: room, AgentID: "a", CommandID: "x", Expected: 3, Limit: ThinkTankMaxTurnLimit + 1},
		{RoomID: room, AgentID: "nobody", CommandID: "x", Expected: 3, Limit: 4},
	} {
		if _, err := st.IncreaseThinkTankTurnLimit(bad); !errors.Is(err, ErrThinkTankInvalid) {
			t.Fatalf("%+v = %v, want invalid", bad, err)
		}
	}
	if _, err := st.EndThinkTank(room); err != nil {
		t.Fatal(err)
	}
	late := ThinkTankLimitChange{RoomID: room, AgentID: "b", CommandID: "lim-3", Expected: 3, Limit: 4}
	if _, err := st.IncreaseThinkTankTurnLimit(late); !errors.Is(err, ErrThinkTankConflict) {
		t.Fatalf("ended room increase = %v", err)
	}
	if err := st.DeleteThinkTank(room); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM think_tank_limit_commands`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("receipts after delete = %d %v", n, err)
	}
}
