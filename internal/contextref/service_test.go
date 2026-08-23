package contextref

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

type fixture struct {
	svc   *Service
	store *state.Store
	home  string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	home := t.TempDir()
	store, err := state.Open(home)
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return &fixture{svc: New(store, home), store: store, home: home}
}

func (f *fixture) agent(t *testing.T, id, name string) {
	t.Helper()
	if err := f.store.WriteAgent(state.Agent{
		AgentID: id, Name: name, Role: name, Project: "proj",
		Backend: "claude-acp", Model: "sonnet", Interface: "chat",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("write agent %s: %v", id, err)
	}
}

// appendEvents writes normalized events to the agent's durable transcript in
// order, exactly as the runtime does.
func (f *fixture) appendEvents(t *testing.T, agentID string, events ...runtime.Event) {
	t.Helper()
	w, err := transcript.Open(f.home, agentID, &runtime.SessionMetaData{Name: agentID, Backend: "claude-acp"})
	if err != nil {
		t.Fatalf("open transcript: %v", err)
	}
	defer w.Close()
	for _, ev := range events {
		if err := w.Append(ev); err != nil {
			t.Fatalf("append %s: %v", ev.Type, err)
		}
	}
}

func ev(t *testing.T, typ string, data any) runtime.Event {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal %s: %v", typ, err)
	}
	return runtime.Event{Type: typ, Data: raw, Ts: "2026-08-22T10:00:00Z"}
}

// evSeq is ev with an explicit sequence, which session_meta needs: the writer
// only auto-numbers records that are not session snapshots, so a resume marker
// has to carry the sequence the runtime would have given it.
func evSeq(t *testing.T, seq int64, typ string, data any) runtime.Event {
	t.Helper()
	e := ev(t, typ, data)
	e.Seq = seq
	return e
}

func (f *fixture) conversation(t *testing.T, agentID string) {
	t.Helper()
	f.appendEvents(t, agentID,
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "first question"}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "first "}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "answer"}),
		ev(t, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn"}),
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "second question"}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "the conclusion"}),
	)
}

func mustShare(t *testing.T, f *fixture, caller Caller, selector, to string) ShareResult {
	t.Helper()
	res, err := f.svc.Share(caller, selector, to, "label", "description")
	if err != nil {
		t.Fatalf("share %s: %v", selector, err)
	}
	return res
}

func code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// A2 — the two transcript selectors resolve to exact spans; current_turn stops
// at the highest sequence visible at the call, and latest_completed_turn is the
// exact range ending at the most recent turn_end (TS-04.R28, FS-15.R2/R4).
func TestShareResolvesTranscriptSelectors(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.conversation(t, "a_src")
	caller := Caller{AgentID: "a_src", Generation: "g1"}

	current := mustShare(t, f, caller, SelectorCurrentTurn, "a_dst")
	if current.Source.FirstSeq != 5 || current.Source.LastSeq != 6 {
		t.Fatalf("current turn span = %d..%d, want 5..6", current.Source.FirstSeq, current.Source.LastSeq)
	}
	completed := mustShare(t, f, caller, SelectorLatestCompletedTurn, "a_dst")
	if completed.Source.FirstSeq != 1 || completed.Source.LastSeq != 4 {
		t.Fatalf("completed turn span = %d..%d, want 1..4", completed.Source.FirstSeq, completed.Source.LastSeq)
	}
	if current.ContextRefID == completed.ContextRefID {
		t.Fatal("two different spans canonicalized onto one reference")
	}

	// The snapshot is immutable through the share call: text emitted afterwards
	// is deliberately outside the reference.
	f.appendEvents(t, "a_src", ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: " and an afterthought"}))
	page, err := f.svc.Read(Caller{AgentID: "a_dst"}, current.ContextRefID, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(page.Text, "the conclusion") {
		t.Fatalf("shared span lost its conclusion: %q", page.Text)
	}
	if strings.Contains(page.Text, "afterthought") {
		t.Fatalf("shared span absorbed later text: %q", page.Text)
	}
	if !strings.Contains(page.Text, "second question") {
		t.Fatalf("current turn dropped its prompt: %q", page.Text)
	}
	if strings.Contains(page.Text, "first question") {
		t.Fatalf("current turn leaked the previous turn: %q", page.Text)
	}
}

// A2 — a caller cannot name another agent's transcript: the selector is
// server-resolved from the session identity alone (TS-05.R16).
func TestShareSelectorIsServerDerived(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.conversation(t, "a_src")

	// The peer has no transcript of its own, so no selector can reach a_src's.
	f.agent(t, "a_peer", "peer")
	_, err := f.svc.Share(Caller{AgentID: "a_peer"}, SelectorCurrentTurn, "a_dst", "", "")
	if code(err) != CodeSourceUnavailable {
		t.Fatalf("peer share = %v, want %s", err, CodeSourceUnavailable)
	}
	_, err = f.svc.Share(Caller{AgentID: "a_peer"}, "a_src:1-4", "a_dst", "", "")
	if code(err) != CodeValidation {
		t.Fatalf("raw locator argument = %v, want %s", err, CodeValidation)
	}
	var refs int
	if err := f.store.DB().QueryRow(`SELECT COUNT(*) FROM context_references`).Scan(&refs); err != nil {
		t.Fatalf("count: %v", err)
	}
	if refs != 0 {
		t.Fatalf("rejected shares created %d references", refs)
	}
}

func TestShareRejectsUnknownAndArchivedRecipients(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.conversation(t, "a_src")
	caller := Caller{AgentID: "a_src"}

	if _, err := f.svc.Share(caller, SelectorCurrentTurn, "nobody", "", ""); code(err) != CodeRecipientNotFound {
		t.Fatalf("unknown recipient = %v", err)
	}
	// An archived identity is not a context recipient.
	f.agent(t, "a_arch", "arch")
	if err := f.store.SetAgentsArchived([]string{"a_arch"}, true); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := f.svc.Share(caller, SelectorCurrentTurn, "a_arch", "", ""); code(err) != CodeRecipientNotFound {
		t.Fatalf("archived recipient = %v", err)
	}
	// So is a terminal agent.
	if err := f.store.WriteAgent(state.Agent{AgentID: "a_term", Name: "term", Role: "ops", Project: "proj",
		Backend: "claude-acp", Model: "sonnet", Interface: "terminal", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("write terminal agent: %v", err)
	}
	if _, err := f.svc.Share(caller, SelectorCurrentTurn, "a_term", "", ""); code(err) != CodeRecipientNotFound {
		t.Fatalf("terminal recipient = %v", err)
	}
}

// A7 — a stopped, pipeline-associated recipient is a valid context target: a
// grant starts no process, so mail's wake gates do not apply (FS-15.R17).
func TestStoppedPipelineAssociatedRecipientIsShareable(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_stage", "stage")
	f.conversation(t, "a_src")
	if _, err := f.store.DB().Exec(`
INSERT INTO pipeline_runs(run_id, template_id, display_name, project, goal, state, created_at, updated_at)
VALUES ('pr_1','t','run','proj','goal','running',?,?)`, "2026-08-22T10:00:00Z", "2026-08-22T10:00:00Z"); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if _, err := f.store.DB().Exec(`
INSERT INTO pipeline_attempts(attempt_id, run_id, stage_id, attempt_no, visit_no, agent_id, backend, model, state, created_at, updated_at)
VALUES ('pa_1','pr_1','s1',1,1,'a_stage','claude-acp','sonnet','running',?,?)`,
		"2026-08-22T10:00:00Z", "2026-08-22T10:00:00Z"); err != nil {
		t.Fatalf("insert attempt: %v", err)
	}
	res := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_stage")
	if res.To != "a_stage" {
		t.Fatalf("share target = %q", res.To)
	}
	// Sharing creates no mail and no activation (FS-15.R10).
	var messages, activations int
	if err := f.store.DB().QueryRow(
		`SELECT (SELECT COUNT(*) FROM messages), (SELECT COUNT(*) FROM activations)`).Scan(&messages, &activations); err != nil {
		t.Fatalf("count: %v", err)
	}
	if messages != 0 || activations != 0 {
		t.Fatalf("share created %d messages and %d activations", messages, activations)
	}
}

// A5 — missing and unauthorized ids are indistinguishable, and reads have no
// personal-state side effect (FS-15.R9, TS-05.R16).
func TestReadAuthorizationIsOpaque(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.agent(t, "a_snoop", "snoop")
	f.conversation(t, "a_src")
	res := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_dst")

	_, missing := f.svc.Read(Caller{AgentID: "a_snoop"}, "cr_doesnotexist", "")
	_, unauthorized := f.svc.Read(Caller{AgentID: "a_snoop"}, res.ContextRefID, "")
	if code(missing) != CodeNotFound || code(unauthorized) != CodeNotFound {
		t.Fatalf("missing = %v, unauthorized = %v; want the same %s", missing, unauthorized, CodeNotFound)
	}
	if missing.Error() != unauthorized.Error() {
		t.Fatalf("outcomes distinguishable: %q vs %q", missing, unauthorized)
	}

	if _, err := f.svc.Read(Caller{AgentID: "a_dst"}, res.ContextRefID, ""); err != nil {
		t.Fatalf("authorized read: %v", err)
	}
	var prefs int
	if err := f.store.DB().QueryRow(`SELECT COUNT(*) FROM context_grant_preferences`).Scan(&prefs); err != nil {
		t.Fatalf("count: %v", err)
	}
	if prefs != 0 {
		t.Fatalf("reading created %d personal-state rows", prefs)
	}

	// Revocation removes authorization on the very next page.
	if err := f.svc.Revoke(Caller{AgentID: "a_src"}, res.GrantID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := f.svc.Read(Caller{AgentID: "a_dst"}, res.ContextRefID, ""); code(err) != CodeNotFound {
		t.Fatalf("read after revocation = %v", err)
	}
}

// A5 — cursors traverse the fixed source deterministically and confer no
// authority of their own.
func TestReadPagesTraverseTheFixedSource(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.agent(t, "a_snoop", "snoop")
	long := strings.Repeat("paragraph of shared reasoning. ", 4000)
	f.appendEvents(t, "a_src",
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "explain"}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: long}),
	)
	res := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_dst")
	reader := Caller{AgentID: "a_dst"}

	var whole strings.Builder
	cursor := ""
	pages := 0
	for {
		page, err := f.svc.Read(reader, res.ContextRefID, cursor)
		if err != nil {
			t.Fatalf("page %d: %v", pages, err)
		}
		if len(page.Text) > MaxPageBytes {
			t.Fatalf("page %d is %d bytes, over the %d cap", pages, len(page.Text), MaxPageBytes)
		}
		whole.WriteString(page.Text)
		pages++
		if page.Complete {
			if page.NextCursor != "" {
				t.Fatal("a complete page still offered a continuation cursor")
			}
			break
		}
		if page.NextCursor == "" {
			t.Fatal("an incomplete page offered no continuation cursor")
		}
		cursor = page.NextCursor
		if pages > 20 {
			t.Fatal("traversal did not terminate")
		}
	}
	if pages < 2 {
		t.Fatalf("expected a multi-page source, got %d page(s)", pages)
	}
	if !strings.Contains(whole.String(), "explain") || strings.Count(whole.String(), "paragraph of shared reasoning.") != 4000 {
		t.Fatalf("traversal did not reproduce the source exactly (%d bytes)", whole.Len())
	}

	// A second traversal returns identical bytes: reading changes nothing.
	repeat, err := f.svc.Read(reader, res.ContextRefID, "")
	if err != nil {
		t.Fatalf("repeat read: %v", err)
	}
	first, _ := f.svc.Read(reader, res.ContextRefID, "")
	if repeat.Text != first.Text {
		t.Fatal("repeated reads of a fixed source differ")
	}

	// A cursor is not a capability.
	page, _ := f.svc.Read(reader, res.ContextRefID, "")
	if _, err := f.svc.Read(Caller{AgentID: "a_snoop"}, res.ContextRefID, page.NextCursor); code(err) != CodeNotFound {
		t.Fatalf("cursor granted access to an unauthorized reader: %v", err)
	}
}

func TestReadRejectsForeignAndMalformedCursors(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.conversation(t, "a_src")
	one := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_dst")
	two := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorLatestCompletedTurn, "a_dst")
	reader := Caller{AgentID: "a_dst"}

	foreign := encodeCursor(two.ContextRefID, 0)
	if _, err := f.svc.Read(reader, one.ContextRefID, foreign); code(err) != CodeInvalidCursor {
		t.Fatalf("cross-reference cursor = %v, want %s", err, CodeInvalidCursor)
	}
	if _, err := f.svc.Read(reader, one.ContextRefID, "!!!not-base64!!!"); code(err) != CodeInvalidCursor {
		t.Fatalf("malformed cursor = %v", err)
	}
}

// A7 — archive keeps a source readable; deletion makes it an honest tombstone
// without remapping or cascading (FS-15.R12–R13).
func TestArchiveKeepsContextAndDeletionTombstones(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.conversation(t, "a_src")
	res := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorLatestCompletedTurn, "a_dst")
	reader := Caller{AgentID: "a_dst"}

	if err := f.store.SetAgentsArchived([]string{"a_src"}, true); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := f.svc.Read(reader, res.ContextRefID, ""); err != nil {
		t.Fatalf("archived source became unreadable: %v", err)
	}
	if err := f.store.DeleteAgent("a_src"); err != nil {
		t.Fatalf("delete source: %v", err)
	}
	if _, err := f.svc.Read(reader, res.ContextRefID, ""); code(err) != CodeSourceGone {
		t.Fatalf("deleted source = %v, want %s", err, CodeSourceGone)
	}
	// The reference itself survives as an identifiable tombstone.
	if _, err := f.store.ReadContextReference(res.ContextRefID); err != nil {
		t.Fatalf("tombstone lost its reference row: %v", err)
	}
}

// A5 — an oversized physical record inside the span becomes a bounded marker
// rather than a silently clean page (TS-01.R22, TS-04.R28).
func TestReadMarksOversizedRecords(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.appendEvents(t, "a_src",
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "before"}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: strings.Repeat("x", 9*1024*1024)}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "after"}),
	)
	res := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_dst")
	page, err := f.svc.Read(Caller{AgentID: "a_dst"}, res.ContextRefID, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(page.Text, OversizedRecordMarker) {
		t.Fatalf("oversized record was omitted silently: %q", page.Text)
	}
	if !strings.Contains(page.Text, "before") || !strings.Contains(page.Text, "after") {
		t.Fatalf("surrounding records were lost: %q", page.Text)
	}
}

// A5 — an unknown normalized event kind is an explicit bounded marker.
func TestReadMarksUnknownEventKinds(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.appendEvents(t, "a_src",
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "before"}),
		ev(t, "future_kind", map[string]string{"payload": "opaque"}),
	)
	res := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_dst")
	page, err := f.svc.Read(Caller{AgentID: "a_dst"}, res.ContextRefID, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(page.Text, `unknown event type "future_kind"`) {
		t.Fatalf("unknown event vanished from the page: %q", page.Text)
	}
	if strings.Contains(page.Text, "opaque") {
		t.Fatalf("unknown event leaked its payload: %q", page.Text)
	}
}

// Session snapshots and backend-switch markers stay out of composed context.
func TestReadOmitsSessionMetadataAndSwitchMarkers(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.appendEvents(t, "a_src",
		ev(t, runtime.EvBackendSwitch, runtime.BackendSwitchData{From: "claude-acp", To: "codex-acp"}),
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "carry on"}),
	)
	res := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_dst")
	if res.Source.FirstSeq != 2 {
		t.Fatalf("span started at %d, want the first content event", res.Source.FirstSeq)
	}
	page, err := f.svc.Read(Caller{AgentID: "a_dst"}, res.ContextRefID, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(page.Text, "codex-acp") || strings.Contains(page.Text, "claude-acp") {
		t.Fatalf("metadata leaked into composed context: %q", page.Text)
	}
}

func TestSharePresentationBounds(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.conversation(t, "a_src")
	caller := Caller{AgentID: "a_src"}
	if _, err := f.svc.Share(caller, SelectorCurrentTurn, "a_dst", strings.Repeat("l", MaxLabelRunes+1), ""); code(err) != CodeValidation {
		t.Fatalf("over-limit label = %v", err)
	}
	if _, err := f.svc.Share(caller, SelectorCurrentTurn, "a_dst", "", strings.Repeat("d", MaxDescriptionRunes+1)); code(err) != CodeValidation {
		t.Fatalf("over-limit description = %v", err)
	}
	var refs int
	if err := f.store.DB().QueryRow(`SELECT COUNT(*) FROM context_references`).Scan(&refs); err != nil {
		t.Fatalf("count: %v", err)
	}
	if refs != 0 {
		t.Fatalf("rejected presentation created %d references", refs)
	}
}

func TestListBoundsAndCursorScope(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.agent(t, "a_peer", "peer")
	f.conversation(t, "a_src")
	dst := Caller{AgentID: "a_dst"}
	mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_dst")

	if _, err := f.svc.List(dst, false, MaxListLimit+1, ""); code(err) != CodeValidation {
		t.Fatalf("over-limit list = %v", err)
	}
	page, err := f.svc.List(dst, false, 1, "")
	if err != nil || len(page.Links) != 1 {
		t.Fatalf("list = %+v (%v)", page, err)
	}
	if page.Links[0].Source.Kind != state.ContextSourceTranscriptSpan {
		t.Fatalf("list lost intrinsic source metadata: %+v", page.Links[0])
	}
	if page.NextCursor == "" {
		t.Fatal("a full page offered no cursor")
	}
	if _, err := f.svc.List(Caller{AgentID: "a_peer"}, false, 20, page.NextCursor); code(err) != CodeInvalidCursor {
		t.Fatalf("another agent replayed a list cursor: %v", err)
	}
}

func (f *fixture) pipelineAttempt(t *testing.T, agentID, generation string, reported, quiescent bool) {
	t.Helper()
	now := "2026-08-22T10:00:00Z"
	if _, err := f.store.DB().Exec(`
INSERT INTO pipeline_runs(run_id, template_id, display_name, project, goal, state,
                          current_stage_id, current_attempt_id, current_agent_id, created_at, updated_at)
VALUES ('pr_1','t','run','proj','goal','running','s1','pa_1',?,?,?)`, agentID, now, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	var reportedAt, quiescentAt any
	if reported {
		reportedAt = now
	}
	if quiescent {
		quiescentAt = now
	}
	if _, err := f.store.DB().Exec(`
INSERT INTO pipeline_attempts(attempt_id, run_id, stage_id, attempt_no, visit_no, agent_id, agent_generation,
                              backend, model, state, report_outcome, report_summary, report_details,
                              report_checks, report_outputs_json, reported_at, quiescent_at, created_at, updated_at)
VALUES ('pa_1','pr_1','s1',1,1,?,?,'claude-acp','sonnet','reported','success','the stage passed','full details',
        'make test','{"artifact":"bin/agentdeck"}',?,?,?,?)`,
		agentID, generation, reportedAt, quiescentAt, now, now); err != nil {
		t.Fatalf("insert attempt: %v", err)
	}
}

// A3 — an accepted pipeline report is shareable inside the reporting turn; once
// the run advances past quiescence the friendly selector is unavailable, while
// the already-created reference stays readable (FS-15.R4, TS-04.R28).
func TestSharePipelineReportWindow(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_stage", "stage")
	f.agent(t, "a_dst", "dst")
	f.pipelineAttempt(t, "a_stage", "gen1", true, false)
	caller := Caller{AgentID: "a_stage", Generation: "gen1"}

	res := mustShare(t, f, caller, SelectorCurrentPipelineReport, "a_dst")
	if res.Source.Kind != state.ContextSourcePipelineReport || res.Source.PipelineAttemptID != "pa_1" {
		t.Fatalf("report source = %+v", res.Source)
	}
	page, err := f.svc.Read(Caller{AgentID: "a_dst"}, res.ContextRefID, "")
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	for _, want := range []string{"success", "the stage passed", "full details", "make test", "bin/agentdeck"} {
		if !strings.Contains(page.Text, want) {
			t.Errorf("report page missing %q: %q", want, page.Text)
		}
	}
	// Run assignment text and mutable named values are not part of this source.
	if strings.Contains(page.Text, "goal") {
		t.Errorf("report page leaked run state: %q", page.Text)
	}

	// Past quiescence the selector is unavailable and creates nothing, but the
	// canonical reference remains readable.
	if _, err := f.store.DB().Exec(`UPDATE pipeline_attempts SET quiescent_at = ? WHERE attempt_id = 'pa_1'`,
		"2026-08-22T10:05:00Z"); err != nil {
		t.Fatalf("quiesce: %v", err)
	}
	if _, err := f.svc.Share(caller, SelectorCurrentPipelineReport, "a_dst", "", ""); code(err) != CodeSourceUnavailable {
		t.Fatalf("post-quiescence share = %v, want %s", err, CodeSourceUnavailable)
	}
	if _, err := f.svc.Read(Caller{AgentID: "a_dst"}, res.ContextRefID, ""); err != nil {
		t.Fatalf("existing reference stopped reading after quiescence: %v", err)
	}
}

// An unreported attempt and another generation's attempt are both rejected
// without creating a reference (TS-05.R16, FS-15.R15).
func TestSharePipelineReportRejectsUnreportedAndForeignGeneration(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_stage", "stage")
	f.agent(t, "a_dst", "dst")
	f.pipelineAttempt(t, "a_stage", "gen1", false, false)

	if _, err := f.svc.Share(Caller{AgentID: "a_stage", Generation: "gen1"},
		SelectorCurrentPipelineReport, "a_dst", "", ""); code(err) != CodeSourceUnavailable {
		t.Fatalf("unreported attempt = %v", err)
	}
	if _, err := f.store.DB().Exec(`UPDATE pipeline_attempts SET reported_at = ? WHERE attempt_id = 'pa_1'`,
		"2026-08-22T10:00:00Z"); err != nil {
		t.Fatalf("report: %v", err)
	}
	if _, err := f.svc.Share(Caller{AgentID: "a_stage", Generation: "gen2"},
		SelectorCurrentPipelineReport, "a_dst", "", ""); code(err) != CodeSourceUnavailable {
		t.Fatalf("foreign generation = %v", err)
	}
	var refs int
	if err := f.store.DB().QueryRow(`SELECT COUNT(*) FROM context_references`).Scan(&refs); err != nil {
		t.Fatalf("count: %v", err)
	}
	if refs != 0 {
		t.Fatalf("rejected report shares created %d references", refs)
	}
}

// A deleted attempt tombstones its reference rather than aliasing a newer one.
func TestDeletedPipelineAttemptTombstones(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_stage", "stage")
	f.agent(t, "a_dst", "dst")
	f.pipelineAttempt(t, "a_stage", "gen1", true, false)
	res := mustShare(t, f, Caller{AgentID: "a_stage", Generation: "gen1"}, SelectorCurrentPipelineReport, "a_dst")

	// A run is deletable only once terminal, exactly as the Pipelines surface
	// requires; the attempt row cascades with it.
	if _, err := f.store.DB().Exec(`UPDATE pipeline_runs SET state = 'completed' WHERE run_id = 'pr_1'`); err != nil {
		t.Fatalf("complete run: %v", err)
	}
	if err := f.store.DeletePipelineRun("pr_1"); err != nil {
		t.Fatalf("delete run: %v", err)
	}
	if _, err := f.svc.Read(Caller{AgentID: "a_dst"}, res.ContextRefID, ""); code(err) != CodeSourceGone {
		t.Fatalf("deleted attempt = %v, want %s", err, CodeSourceGone)
	}
	if _, err := f.store.ReadContextReference(res.ContextRefID); err != nil {
		t.Fatalf("tombstone lost its reference row: %v", err)
	}
}

// A2 — latest_completed_turn is available only from inside a later turn started
// for some independent reason. An idle session token holds no shareable
// finished work, and the refusal mutates nothing (TS-04.R28, FS-15.R4/R15).
func TestLatestCompletedTurnNeedsALaterTurn(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.appendEvents(t, "a_src",
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "first question"}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "first answer"}),
		ev(t, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn"}),
	)
	caller := Caller{AgentID: "a_src", Generation: "g1"}

	if _, err := f.svc.Share(caller, SelectorLatestCompletedTurn, "a_dst", "", ""); code(err) != CodeSourceUnavailable {
		t.Fatalf("idle share = %v, want %s", err, CodeSourceUnavailable)
	}
	var refs, grants int
	if err := f.store.DB().QueryRow(
		`SELECT (SELECT COUNT(*) FROM context_references), (SELECT COUNT(*) FROM context_grants)`).Scan(&refs, &grants); err != nil {
		t.Fatalf("count: %v", err)
	}
	if refs != 0 || grants != 0 {
		t.Fatalf("idle share created %d references and %d grants", refs, grants)
	}

	// A separate later turn makes the same selector resolvable.
	f.appendEvents(t, "a_src", ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "second question"}))
	res := mustShare(t, f, caller, SelectorLatestCompletedTurn, "a_dst")
	if res.Source.FirstSeq != 1 || res.Source.LastSeq != 3 {
		t.Fatalf("completed span = %d..%d, want 1..3", res.Source.FirstSeq, res.Source.LastSeq)
	}
}

// A2 — a turn interrupted without turn_end is closed by the resume marker, so
// the turn after the resume cannot absorb stale pre-crash text
// (TS-04.R28, FS-15.R4/R15, INV §1).
func TestCurrentTurnStartsAfterAResumeBoundary(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.appendEvents(t, "a_src",
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "finished question"}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "finished answer"}),
		ev(t, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn"}),
		// The agent crashed here: this turn never reached turn_end.
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "abandoned question"}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "stale abandoned reasoning"}),
		evSeq(t, 6, runtime.EvSessionMeta, runtime.SessionMetaData{
			Name: "src", Backend: "claude-acp", CreatedAt: "2026-08-22T10:00:00Z"}),
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "fresh question"}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "fresh conclusion"}),
	)

	res := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_dst")
	if res.Source.FirstSeq != 7 || res.Source.LastSeq != 8 {
		t.Fatalf("current span = %d..%d, want 7..8 (starting after the resume marker)",
			res.Source.FirstSeq, res.Source.LastSeq)
	}
	page, err := f.svc.Read(Caller{AgentID: "a_dst"}, res.ContextRefID, "")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(page.Text, "fresh conclusion") {
		t.Fatalf("current turn lost its own content: %q", page.Text)
	}
	if strings.Contains(page.Text, "abandoned") || strings.Contains(page.Text, "stale") {
		t.Fatalf("current turn absorbed the interrupted pre-resume turn: %q", page.Text)
	}
}

// A5 — an oversized record at either edge of the selected span is marked, and a
// record skipped outside the span still is not (TS-01.R22, TS-04.R28, INV §7).
func TestReadMarksOversizedRecordsAtSpanEdges(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	huge := runtime.AssistantTextData{Delta: strings.Repeat("x", 9*1024*1024)}
	f.appendEvents(t, "a_src",
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "finished question"}),
		ev(t, runtime.EvTurnEnd, runtime.TurnEndData{StopReason: "end_turn"}),
		ev(t, runtime.EvAssistantText, huge), // first record of the next turn
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "current question"}),
		ev(t, runtime.EvAssistantText, huge), // last record of the still-open turn
	)
	reader := Caller{AgentID: "a_dst"}

	current := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_dst")
	page, err := f.svc.Read(reader, current.ContextRefID, "")
	if err != nil {
		t.Fatalf("read current turn: %v", err)
	}
	if got := strings.Count(page.Text, OversizedRecordMarker); got != 2 {
		t.Fatalf("leading and trailing oversized records produced %d markers: %q", got, page.Text)
	}
	if !strings.Contains(page.Text, "current question") {
		t.Fatalf("the readable record between the markers was lost: %q", page.Text)
	}

	// The same skipped record sits outside the finished turn, which ends at its
	// turn_end, so that page stays clean.
	completed := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorLatestCompletedTurn, "a_dst")
	done, err := f.svc.Read(reader, completed.ContextRefID, "")
	if err != nil {
		t.Fatalf("read completed turn: %v", err)
	}
	if strings.Contains(done.Text, OversizedRecordMarker) {
		t.Fatalf("a record skipped after turn_end was marked inside the finished turn: %q", done.Text)
	}
	if !strings.Contains(done.Text, "finished question") {
		t.Fatalf("completed turn lost its content: %q", done.Text)
	}
}

// A5 — an altered page offset fails as invalid_cursor instead of splitting a
// rune or reporting a false completion (FS-15.R9/R11, TS-05.R16).
func TestReadRejectsForgedCursorOffsets(t *testing.T) {
	f := newFixture(t)
	f.agent(t, "a_src", "src")
	f.agent(t, "a_dst", "dst")
	f.appendEvents(t, "a_src",
		ev(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "explain"}),
		ev(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: strings.Repeat("é", 40000)}),
	)
	res := mustShare(t, f, Caller{AgentID: "a_src"}, SelectorCurrentTurn, "a_dst")
	reader := Caller{AgentID: "a_dst"}

	page, err := f.svc.Read(reader, res.ContextRefID, "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if page.Complete || page.NextCursor == "" {
		t.Fatal("expected a multi-page source")
	}
	offset, cerr := decodeCursor(res.ContextRefID, page.NextCursor)
	if cerr != nil {
		t.Fatalf("decode issued cursor: %v", cerr)
	}

	// One byte past the issued boundary lands inside a two-byte rune.
	interior := encodeCursor(res.ContextRefID, offset+1)
	if _, err := f.svc.Read(reader, res.ContextRefID, interior); code(err) != CodeInvalidCursor {
		t.Fatalf("interior-rune offset = %v, want %s", err, CodeInvalidCursor)
	}
	if _, err := f.svc.Read(reader, res.ContextRefID, encodeCursor(res.ContextRefID, 1<<30)); code(err) != CodeInvalidCursor {
		t.Fatalf("past-end offset = %v, want %s", err, CodeInvalidCursor)
	}

	// The honestly issued cursor still traverses the source.
	next, err := f.svc.Read(reader, res.ContextRefID, page.NextCursor)
	if err != nil {
		t.Fatalf("issued cursor stopped working: %v", err)
	}
	if !utf8.ValidString(next.Text) || next.Text == "" {
		t.Fatalf("continuation page is not valid UTF-8 text: %q", next.Text[:min(64, len(next.Text))])
	}
}
