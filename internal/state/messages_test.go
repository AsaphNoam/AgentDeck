package state

import (
	"errors"
	"testing"
	"time"
)

// directory is the addressable set these tests resolve against, assembled the
// way the dashboard assembles it (FS-06.R22): the running registry plus the
// stopped chat agents a message can wake.
func directory(t *testing.T, st *Store) []LiveAgent {
	t.Helper()
	live, err := st.LiveAgents()
	if err != nil {
		t.Fatalf("LiveAgents: %v", err)
	}
	stopped, err := st.StoppedWakeCandidates("")
	if err != nil {
		t.Fatalf("StoppedWakeCandidates: %v", err)
	}
	return append(live, stopped...)
}

// liveAgent writes an agent + running row so it shows up in LiveAgents.
func liveAgent(t *testing.T, st *Store, id, name, role, project string) {
	t.Helper()
	a := Agent{
		AgentID: id, Name: name, Role: role, Project: project,
		Backend: "claude", Model: "sonnet", Interface: "chat",
		CreatedAt: time.Now().UTC(),
	}
	if err := st.WriteAgent(a); err != nil {
		t.Fatalf("WriteAgent %s: %v", id, err)
	}
	if err := st.WriteRunning(RunningEntry{
		AgentID: id, PID: 100, SessionID: "s_" + id, Interface: "chat", StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteRunning %s: %v", id, err)
	}
}

// liveTerminalAgent writes an agent + running row with interface "terminal".
func liveTerminalAgent(t *testing.T, st *Store, id, name, role, project string) {
	t.Helper()
	a := Agent{
		AgentID: id, Name: name, Role: role, Project: project,
		Backend: "claude", Model: "sonnet", Interface: "terminal",
		CreatedAt: time.Now().UTC(),
	}
	if err := st.WriteAgent(a); err != nil {
		t.Fatalf("WriteAgent %s: %v", id, err)
	}
	if err := st.WriteRunning(RunningEntry{
		AgentID: id, PID: 200, SessionID: "s_" + id, Interface: "terminal", Driver: "xterm", StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteRunning %s: %v", id, err)
	}
}

// FS-06.A13 / TS-02.R23 — mail carries one durable, payload-free activation
// while pending; a later insert after claim creates the next opportunity.
func TestMailActivationCoalescesClaimsAndRecovers(t *testing.T) {
	st, _ := newTestStore(t)
	liveAgent(t, st, "a_mail", "Nova", "reviewer", "my-app")
	for _, body := range []string{"first", "second"} {
		if _, err := st.InsertMessage(Message{FromAgent: "a_sender", FromAddress: "impl@my-app", FromName: "Atlas", ToAgent: "a_mail", Body: body}); err != nil {
			t.Fatalf("InsertMessage %q: %v", body, err)
		}
	}
	pending, err := st.PendingMailActivations("a_mail")
	if err != nil || len(pending) != 1 {
		t.Fatalf("PendingMailActivations = %+v, %v; want one", pending, err)
	}
	claimedID := pending[0].ActivationID
	token, claimed, err := st.ClaimMailActivation(claimedID)
	if err != nil || !claimed {
		t.Fatalf("ClaimMailActivation = %q, %v, %v; want claim", token, claimed, err)
	}
	if _, err := st.InsertMessage(Message{FromAgent: "a_sender", FromAddress: "impl@my-app", FromName: "Atlas", ToAgent: "a_mail", Body: "later"}); err != nil {
		t.Fatalf("InsertMessage later: %v", err)
	}
	pending, err = st.PendingMailActivations("a_mail")
	if err != nil || len(pending) != 1 {
		t.Fatalf("PendingMailActivations after claim = %+v, %v; want later activation", pending, err)
	}
	if attempted, err := st.AttemptMailActivation(claimedID, token, "t_000000000001"); err != nil || !attempted {
		t.Fatalf("AttemptMailActivation = %v, %v; want true, nil", attempted, err)
	}
	if err := st.RetireMailActivation(claimedID, token); err != nil {
		t.Fatalf("RetireMailActivation: %v", err)
	}
	pending, err = st.PendingMailActivations("a_mail")
	if err != nil || len(pending) != 1 {
		t.Fatalf("PendingMailActivations after retire = %+v, %v; want later activation", pending, err)
	}
}

// TS-02.R23 / INV §5 — mail inserted after a claim owns the replacement pending
// opportunity. Releasing the older claim must delete it instead of colliding with
// the mail-only pending uniqueness index and leaving it stranded as claimed.
func TestReleaseMailActivationCoalescesNewerPendingMail(t *testing.T) {
	st, _ := newTestStore(t)
	liveAgent(t, st, "a_release", "Nova", "reviewer", "my-app")
	if _, err := st.InsertMessage(Message{FromAgent: "a_sender", FromAddress: "impl@my-app", FromName: "Atlas", ToAgent: "a_release", Body: "first"}); err != nil {
		t.Fatalf("InsertMessage first: %v", err)
	}
	pending, err := st.PendingMailActivations("a_release")
	if err != nil || len(pending) != 1 {
		t.Fatalf("PendingMailActivations = %+v, %v; want one", pending, err)
	}
	token, claimed, err := st.ClaimMailActivation(pending[0].ActivationID)
	if err != nil || !claimed {
		t.Fatalf("ClaimMailActivation = %q, %v, %v; want claim", token, claimed, err)
	}
	if _, err := st.InsertMessage(Message{FromAgent: "a_sender", FromAddress: "impl@my-app", FromName: "Atlas", ToAgent: "a_release", Body: "later"}); err != nil {
		t.Fatalf("InsertMessage later: %v", err)
	}
	if err := st.ReleaseMailActivation(pending[0].ActivationID, token); err != nil {
		t.Fatalf("ReleaseMailActivation: %v", err)
	}
	left, err := st.PendingMailActivations("a_release")
	if err != nil || len(left) != 1 {
		t.Fatalf("PendingMailActivations after release = %+v, %v; want newer pending row", left, err)
	}
	var claimedRows int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM activations WHERE agent_id = ? AND kind = ? AND state = ?`, "a_release", ActivationKindMail, ActivationClaimed).Scan(&claimedRows); err != nil {
		t.Fatalf("count claimed activations: %v", err)
	}
	if claimedRows != 0 {
		t.Fatalf("claimed mail activations = %d, want none", claimedRows)
	}
}

func TestDiscardClaimedMailActivationRequiresClaimToken(t *testing.T) {
	st, _ := newTestStore(t)
	liveAgent(t, st, "a_discard", "Nova", "reviewer", "my-app")
	if _, err := st.InsertMessage(Message{FromAgent: "a_sender", FromAddress: "impl@my-app", FromName: "Atlas", ToAgent: "a_discard", Body: "first"}); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	pending, err := st.PendingMailActivations("a_discard")
	if err != nil || len(pending) != 1 {
		t.Fatalf("PendingMailActivations = %+v, %v; want one", pending, err)
	}
	token, claimed, err := st.ClaimMailActivation(pending[0].ActivationID)
	if err != nil || !claimed {
		t.Fatalf("ClaimMailActivation = %q, %v, %v; want claim", token, claimed, err)
	}
	if err := st.DiscardClaimedMailActivation(pending[0].ActivationID, "wrong-token"); err != nil {
		t.Fatalf("DiscardClaimedMailActivation(wrong token): %v", err)
	}
	var claimedRows int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM activations WHERE activation_id = ? AND state = ?`, pending[0].ActivationID, ActivationClaimed).Scan(&claimedRows); err != nil {
		t.Fatalf("count claimed activation: %v", err)
	}
	if claimedRows != 1 {
		t.Fatalf("claimed rows after wrong token = %d, want one", claimedRows)
	}
	if err := st.DiscardClaimedMailActivation(pending[0].ActivationID, token); err != nil {
		t.Fatalf("DiscardClaimedMailActivation: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM activations WHERE activation_id = ?`, pending[0].ActivationID).Scan(&claimedRows); err != nil {
		t.Fatalf("count discarded activation: %v", err)
	}
	if claimedRows != 0 {
		t.Fatalf("rows after discard = %d, want none", claimedRows)
	}
}

// Finding 4 regression: a terminal-interface agent has no check_messages tool, so
// delivering mail to it would spin the nudger indefinitely (up to the 7-day mail
// TTL). ResolveRecipient must therefore never select a terminal agent as a
// recipient, by any address form — stopping both delivery and the nudge loop.
func TestResolveRecipientExcludesTerminalAgents(t *testing.T) {
	st, _ := newTestStore(t)
	liveTerminalAgent(t, st, "a_term1", "Zephyr", "runner", "my-app")

	for _, to := range []string{"a_term1", "runner@my-app", "zephyr"} {
		if id, _, err := ResolveRecipient(directory(t, st), to); !errors.Is(err, ErrRecipientNotFound) {
			t.Fatalf("ResolveRecipient(%q) = %q,%v want ErrRecipientNotFound (terminal agents must not receive mail)", to, id, err)
		}
	}
}

func TestResolveRecipient(t *testing.T) {
	st, _ := newTestStore(t)
	liveAgent(t, st, "a_impl1", "Atlas", "implementer", "my-app")
	liveAgent(t, st, "a_rev1", "Nova", "reviewer", "my-app")

	cases := []struct {
		name, to, wantID string
	}{
		{"exact agent_id", "a_rev1", "a_rev1"},
		{"role@project", "reviewer@my-app", "a_rev1"},
		{"name case-insensitive", "nOvA", "a_rev1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, cands, err := ResolveRecipient(directory(t, st), c.to)
			if err != nil {
				t.Fatalf("ResolveRecipient(%q) err = %v", c.to, err)
			}
			if id != c.wantID || cands != nil {
				t.Fatalf("ResolveRecipient(%q) = %q,%v want %q,nil", c.to, id, cands, c.wantID)
			}
		})
	}

	t.Run("not found", func(t *testing.T) {
		if _, _, err := ResolveRecipient(directory(t, st), "ghost@my-app"); !errors.Is(err, ErrRecipientNotFound) {
			t.Fatalf("err = %v, want ErrRecipientNotFound", err)
		}
	})

	t.Run("stopped recipient without a snapshot not addressable", func(t *testing.T) {
		// Removing the running row makes the agent non-live. It has no sessions
		// row either, so no message can wake it and it stays unaddressable
		// (FS-06.R22 narrows R8 only for wakeable agents).
		if err := st.DeleteRunning("a_rev1"); err != nil {
			t.Fatalf("DeleteRunning: %v", err)
		}
		if _, _, err := ResolveRecipient(directory(t, st), "reviewer@my-app"); !errors.Is(err, ErrRecipientNotFound) {
			t.Fatalf("err = %v, want ErrRecipientNotFound for stopped agent", err)
		}
	})
}

func TestResolveRecipientAmbiguous(t *testing.T) {
	st, _ := newTestStore(t)
	liveAgent(t, st, "a_r1", "Echo", "reviewer", "my-app")
	liveAgent(t, st, "a_r2", "Echo", "reviewer", "my-app") // same role@project AND name

	// role@project ambiguity.
	_, cands, err := ResolveRecipient(directory(t, st), "reviewer@my-app")
	var ambErr *AmbiguousError
	if !errors.As(err, &ambErr) || !errors.Is(err, ErrAmbiguousRecipient) {
		t.Fatalf("err = %v, want AmbiguousError", err)
	}
	if len(ambErr.Candidates) != 2 || len(cands) != 0 {
		t.Fatalf("candidates from error = %d (return slice %d), want 2/0", len(ambErr.Candidates), len(cands))
	}

	// name ambiguity.
	if _, _, err := ResolveRecipient(directory(t, st), "Echo"); !errors.Is(err, ErrAmbiguousRecipient) {
		t.Fatalf("name ambiguity err = %v, want ErrAmbiguousRecipient", err)
	}
}

// FS-06.A6: mailbox limits retain and return the newest messages first.
func TestListMessagesOrderingAndLimit(t *testing.T) {
	st, _ := newTestStore(t)
	base := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if _, err := st.InsertMessage(Message{
			FromAgent: "a_from", FromAddress: "x@y", FromName: "X",
			ToAgent: "a_to", Body: "m", CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("InsertMessage %d: %v", i, err)
		}
	}
	msgs, err := st.ListMessages("a_to", true, 2)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("limit not applied: got %d, want 2", len(msgs))
	}
	// When the mailbox exceeds the limit, the NEWEST N must be returned (the two
	// most recent: base+1m and base+2m), newest-first — not the oldest N. The
	// oldest message (base+0m) must be dropped, not the newest.
	if !msgs[0].CreatedAt.After(msgs[1].CreatedAt) {
		t.Fatalf("not descending by created_at: %v then %v", msgs[0].CreatedAt, msgs[1].CreatedAt)
	}
	if !msgs[0].CreatedAt.Equal(base.Add(2 * time.Minute)) {
		t.Fatalf("newest message not returned first: got %v, want %v", msgs[0].CreatedAt, base.Add(2*time.Minute))
	}
	for _, m := range msgs {
		if m.CreatedAt.Equal(base) {
			t.Fatalf("oldest message should have been dropped by the limit, but it is present")
		}
	}
}

// stoppedAgent writes an identity plus the frozen session snapshot a resume needs,
// with no running row — the shape of a stopped, wakeable chat agent.
func stoppedAgent(t *testing.T, st *Store, id, name, role, project string) {
	t.Helper()
	a := Agent{
		AgentID: id, Name: name, Role: role, Project: project,
		Backend: "claude", Model: "sonnet", Interface: "chat",
		CreatedAt: time.Now().UTC(),
	}
	if err := st.WriteAgent(a); err != nil {
		t.Fatalf("WriteAgent %s: %v", id, err)
	}
	if _, err := st.db.Exec(`
INSERT INTO sessions(agent_id, name, role, project, backend, model, interface, cwd, system_prompt, created_at, updated_at)
VALUES (?,?,?,?,'claude','sonnet','chat','/tmp','prompt','2026-08-16T10:00:00Z','2026-08-16T10:01:00Z')`,
		id, name, role, project); err != nil {
		t.Fatalf("insert session %s: %v", id, err)
	}
}

// FS-06.R22 / TS-04.R26 — the addressable set adds stopped chat agents a message
// can wake, and only those: the wake gates exclude an archived agent, a terminal
// agent, an agent with no snapshot to resume, and any agent a pipeline attempt
// owns (its state machine stopped it deliberately).
func TestStoppedWakeCandidates(t *testing.T) {
	st, _ := newTestStore(t)
	stoppedAgent(t, st, "a_wake", "Atlas", "implementer", "my-app")
	stoppedAgent(t, st, "a_arch", "Archer", "implementer", "my-app")
	stoppedAgent(t, st, "a_pipe", "Piper", "implementer", "my-app")
	liveAgent(t, st, "a_live", "Nova", "reviewer", "my-app")
	// No snapshot: identity only.
	if err := st.WriteAgent(Agent{AgentID: "a_bare", Name: "Bare", Role: "runner", Project: "my-app",
		Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("WriteAgent bare: %v", err)
	}
	if err := st.SetAgentsArchived([]string{"a_arch"}, true); err != nil {
		t.Fatalf("SetAgentsArchived: %v", err)
	}
	if _, err := st.db.Exec(`
INSERT INTO pipeline_runs(run_id, template_id, display_name, project, goal, state, created_at, updated_at)
VALUES ('pr_1','t_1','Build','my-app','ship','running','2026-08-16T10:00:00Z','2026-08-16T10:00:00Z')`); err != nil {
		t.Fatalf("insert pipeline run: %v", err)
	}
	if err := st.InsertPipelineAttempt(PipelineAttemptRecord{
		AttemptID: "pa_1", RunID: "pr_1", StageID: "build", AttemptNo: 1, VisitNo: 1,
		AgentID: "a_pipe", Backend: "claude", Model: "sonnet", State: "done",
	}); err != nil {
		t.Fatalf("InsertPipelineAttempt: %v", err)
	}

	got, err := st.StoppedWakeCandidates("")
	if err != nil {
		t.Fatalf("StoppedWakeCandidates: %v", err)
	}
	if len(got) != 1 || got[0].AgentID != "a_wake" {
		t.Fatalf("candidates = %+v, want only a_wake", got)
	}
	if got[0].Availability != AvailabilityStoppedWakeable {
		t.Fatalf("availability = %q, want %q", got[0].Availability, AvailabilityStoppedWakeable)
	}

	// The single-agent form answers the same question for one id.
	for _, c := range []struct {
		id   string
		want int
	}{{"a_wake", 1}, {"a_pipe", 0}, {"a_arch", 0}, {"a_bare", 0}, {"a_live", 0}} {
		one, err := st.StoppedWakeCandidates(c.id)
		if err != nil {
			t.Fatalf("StoppedWakeCandidates(%s): %v", c.id, err)
		}
		if len(one) != c.want {
			t.Fatalf("StoppedWakeCandidates(%s) = %d rows, want %d", c.id, len(one), c.want)
		}
	}

	// Running agents keep their own availability, and a stopped wakeable agent is
	// now resolvable as a recipient (FS-06.R22 narrowing R4/R8).
	live, err := st.LiveAgents()
	if err != nil {
		t.Fatalf("LiveAgents: %v", err)
	}
	if len(live) != 1 || live[0].Availability != AvailabilityRunning {
		t.Fatalf("live = %+v, want one running agent", live)
	}
	id, _, err := ResolveRecipient(directory(t, st), "implementer@my-app")
	if err != nil || id != "a_wake" {
		t.Fatalf("ResolveRecipient = %q,%v want a_wake", id, err)
	}
}

// FS-06.R25 — availability and the claim are one decision. Testing unread mail
// and claiming in separate statements let a concurrent check_messages drain the
// last row in between, after which the claim crossed the non-replayable attempt
// boundary and started an empty model turn (INV §5/§15). A drained mailbox must
// retire the opportunity instead of yielding a claim.
func TestClaimMailActivationRetiresDrainedMailbox(t *testing.T) {
	st, _ := newTestStore(t)
	liveAgent(t, st, "a_drain", "Nova", "reviewer", "my-app")
	if _, err := st.InsertMessage(Message{
		FromAgent: "a_sender", FromAddress: "impl@my-app", FromName: "Atlas",
		ToAgent: "a_drain", Body: "only message",
	}); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	pending, err := st.PendingMailActivations("a_drain")
	if err != nil || len(pending) != 1 {
		t.Fatalf("PendingMailActivations = %+v, %v; want one", pending, err)
	}
	activationID := pending[0].ActivationID

	// The drain a check_messages call or a dashboard mailbox read performs.
	if _, err := st.DB().Exec(`UPDATE messages SET read = 1 WHERE to_agent = 'a_drain'`); err != nil {
		t.Fatalf("drain mailbox: %v", err)
	}

	token, claimed, err := st.ClaimMailActivation(activationID)
	if err != nil {
		t.Fatalf("ClaimMailActivation: %v", err)
	}
	if claimed || token != "" {
		t.Fatalf("ClaimMailActivation = %q, %v; want no claim over a drained mailbox", token, claimed)
	}
	after, err := st.PendingMailActivations("a_drain")
	if err != nil || len(after) != 0 {
		t.Fatalf("PendingMailActivations after drain = %+v, %v; want the opportunity retired", after, err)
	}

	// New mail re-arms a fresh opportunity, which still claims normally.
	if _, err := st.InsertMessage(Message{
		FromAgent: "a_sender", FromAddress: "impl@my-app", FromName: "Atlas",
		ToAgent: "a_drain", Body: "later",
	}); err != nil {
		t.Fatalf("InsertMessage later: %v", err)
	}
	rearmed, err := st.PendingMailActivations("a_drain")
	if err != nil || len(rearmed) != 1 {
		t.Fatalf("PendingMailActivations after new mail = %+v, %v; want one", rearmed, err)
	}
	if token, claimed, err := st.ClaimMailActivation(rearmed[0].ActivationID); err != nil || !claimed || token == "" {
		t.Fatalf("ClaimMailActivation(rearmed) = %q, %v, %v; want a claim", token, claimed, err)
	}
}
