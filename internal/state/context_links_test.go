package state

import (
	"errors"
	"testing"
	"time"
)

func contextAgent(t *testing.T, st *Store, id, name string) {
	t.Helper()
	if err := st.WriteAgent(Agent{
		AgentID: id, Name: name, Role: "dev", Project: "proj",
		Backend: "claude-acp", Model: "sonnet", Interface: "chat",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("write agent %s: %v", id, err)
	}
}

func transcriptSpan(agentID string, first, last int64) ContextSource {
	return ContextSource{Kind: ContextSourceTranscriptSpan, AgentID: agentID, FirstSeq: first, LastSeq: last}
}

// A1 — one locator canonicalizes to one reference id no matter how often it is
// shared, while each grant keeps its own presentation (FS-15.R1–R3).
func TestShareContextCanonicalizesOneReferencePerSource(t *testing.T) {
	st, _ := newTestStore(t)
	contextAgent(t, st, "a_src", "source")
	contextAgent(t, st, "a_one", "one")
	contextAgent(t, st, "a_two", "two")

	span := transcriptSpan("a_src", 4, 9)
	first, grantOne, err := st.ShareContext(span, "a_src", "a_one", "plan", "the plan")
	if err != nil {
		t.Fatalf("share to one: %v", err)
	}
	second, grantTwo, err := st.ShareContext(span, "a_src", "a_two", "review this", "different label")
	if err != nil {
		t.Fatalf("share to two: %v", err)
	}
	if first.ContextRefID != second.ContextRefID {
		t.Fatalf("reference ids diverged: %q vs %q", first.ContextRefID, second.ContextRefID)
	}
	if grantOne.GrantID == grantTwo.GrantID {
		t.Fatal("two recipients share one grant id")
	}
	if grantOne.Label != "plan" || grantTwo.Label != "review this" {
		t.Fatalf("grant labels leaked across grants: %q / %q", grantOne.Label, grantTwo.Label)
	}
	if first.Source != span {
		t.Fatalf("reference source = %+v, want %+v", first.Source, span)
	}

	// A different span is a different source, so a different reference.
	other, _, err := st.ShareContext(transcriptSpan("a_src", 4, 10), "a_src", "a_one", "", "")
	if err != nil {
		t.Fatalf("share other span: %v", err)
	}
	if other.ContextRefID == first.ContextRefID {
		t.Fatal("distinct spans collapsed onto one reference")
	}
}

func TestContextSourceValidateRejectsMalformedLocators(t *testing.T) {
	cases := map[string]ContextSource{
		"empty kind":       {},
		"unknown kind":     {Kind: "file"},
		"no agent":         {Kind: ContextSourceTranscriptSpan, FirstSeq: 1, LastSeq: 2},
		"reversed range":   transcriptSpan("a_1", 9, 4),
		"zero first":       transcriptSpan("a_1", 0, 4),
		"mixed locator":    {Kind: ContextSourceTranscriptSpan, AgentID: "a_1", FirstSeq: 1, LastSeq: 2, PipelineAttemptID: "pa_1"},
		"report no id":     {Kind: ContextSourcePipelineReport},
		"report with span": {Kind: ContextSourcePipelineReport, PipelineAttemptID: "pa_1", AgentID: "a_1"},
	}
	for name, src := range cases {
		if err := src.Validate(); !errors.Is(err, ErrInvalidContextSource) {
			t.Errorf("%s: Validate() = %v, want ErrInvalidContextSource", name, err)
		}
	}
	if err := transcriptSpan("a_1", 4, 4).Validate(); err != nil {
		t.Errorf("single-event span rejected: %v", err)
	}
	if err := (ContextSource{Kind: ContextSourcePipelineReport, PipelineAttemptID: "pa_1"}).Validate(); err != nil {
		t.Errorf("report locator rejected: %v", err)
	}
}

// A share that fails validation writes nothing at all (FS-15.R15).
func TestShareContextInvalidSourceIsAtomic(t *testing.T) {
	st, _ := newTestStore(t)
	contextAgent(t, st, "a_src", "source")
	contextAgent(t, st, "a_dst", "dest")
	if _, _, err := st.ShareContext(transcriptSpan("a_src", 9, 4), "a_src", "a_dst", "l", "d"); !errors.Is(err, ErrInvalidContextSource) {
		t.Fatalf("share reversed span: %v", err)
	}
	var refs, grants int
	if err := st.DB().QueryRow(`SELECT (SELECT COUNT(*) FROM context_references), (SELECT COUNT(*) FROM context_grants)`).
		Scan(&refs, &grants); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if refs != 0 || grants != 0 {
		t.Fatalf("rejected share left %d references and %d grants", refs, grants)
	}
}

// A4 — listing is recipient-scoped and hide/unhide is personal list state that
// changes no authorization (FS-15.R6–R7).
func TestContextGrantListingAndHiddenPreference(t *testing.T) {
	st, _ := newTestStore(t)
	contextAgent(t, st, "a_src", "source")
	contextAgent(t, st, "a_dst", "dest")
	contextAgent(t, st, "a_other", "other")

	ref, grant, err := st.ShareContext(transcriptSpan("a_src", 1, 2), "a_src", "a_dst", "l", "d")
	if err != nil {
		t.Fatalf("share: %v", err)
	}
	if got, err := st.ListContextGrantsForRecipient("a_other", false, 20, "", ""); err != nil || len(got) != 0 {
		t.Fatalf("other recipient sees %v (%v)", got, err)
	}
	got, err := st.ListContextGrantsForRecipient("a_dst", false, 20, "", "")
	if err != nil || len(got) != 1 || got[0].GrantID != grant.GrantID {
		t.Fatalf("recipient list = %+v (%v)", got, err)
	}
	if got[0].Source.Kind != ContextSourceTranscriptSpan || got[0].Source.AgentID != "a_src" {
		t.Fatalf("list lost intrinsic source metadata: %+v", got[0].Source)
	}

	if ok, err := st.SetContextGrantHidden(grant.GrantID, "a_dst", true); err != nil || !ok {
		t.Fatalf("hide: %v %v", ok, err)
	}
	if got, err := st.ListContextGrantsForRecipient("a_dst", false, 20, "", ""); err != nil || len(got) != 0 {
		t.Fatalf("hidden grant still listed: %+v (%v)", got, err)
	}
	if got, err := st.ListContextGrantsForRecipient("a_dst", true, 20, "", ""); err != nil || len(got) != 1 || !got[0].Hidden {
		t.Fatalf("include_hidden list = %+v (%v)", got, err)
	}
	// Hiding is not revocation: authorization is untouched.
	if ok, err := st.ContextReadAuthorized(ref.ContextRefID, "a_dst"); err != nil || !ok {
		t.Fatalf("hiding revoked authorization: %v %v", ok, err)
	}
	if ok, err := st.SetContextGrantHidden(grant.GrantID, "a_dst", false); err != nil || !ok {
		t.Fatalf("unhide: %v %v", ok, err)
	}
	if got, err := st.ListContextGrantsForRecipient("a_dst", false, 20, "", ""); err != nil || len(got) != 1 {
		t.Fatalf("unhidden grant missing: %+v (%v)", got, err)
	}
	// Only the recipient owns its personal projection.
	if ok, err := st.SetContextGrantHidden(grant.GrantID, "a_src", true); err != nil || ok {
		t.Fatalf("grantor changed the recipient's hidden state: %v %v", ok, err)
	}
}

// A4 — revocation is grantor-scoped and re-share restores exactly that grant
// with fresh presentation, leaving another recipient's grant alone (R5, R8).
func TestRevokeAndReshareContextGrant(t *testing.T) {
	st, _ := newTestStore(t)
	contextAgent(t, st, "a_src", "source")
	contextAgent(t, st, "a_dst", "dest")
	contextAgent(t, st, "a_peer", "peer")

	span := transcriptSpan("a_src", 3, 5)
	ref, grant, err := st.ShareContext(span, "a_src", "a_dst", "first", "first desc")
	if err != nil {
		t.Fatalf("share: %v", err)
	}
	if _, _, err := st.ShareContext(span, "a_src", "a_peer", "peer", "peer desc"); err != nil {
		t.Fatalf("share to peer: %v", err)
	}
	if ok, err := st.RevokeContextGrant(grant.GrantID, "a_peer"); err != nil || ok {
		t.Fatalf("non-grantor revoked a grant: %v %v", ok, err)
	}
	if ok, err := st.RevokeContextGrant(grant.GrantID, "a_src"); err != nil || !ok {
		t.Fatalf("revoke: %v %v", ok, err)
	}
	if ok, err := st.ContextReadAuthorized(ref.ContextRefID, "a_dst"); err != nil || ok {
		t.Fatalf("revoked grant still authorizes: %v %v", ok, err)
	}
	if got, err := st.ListContextGrantsForRecipient("a_dst", true, 20, "", ""); err != nil || len(got) != 0 {
		t.Fatalf("revoked grant still listed: %+v (%v)", got, err)
	}
	// The peer's independent grant survives, and the reference itself remains.
	if ok, err := st.ContextReadAuthorized(ref.ContextRefID, "a_peer"); err != nil || !ok {
		t.Fatalf("revocation cascaded into the peer grant: %v %v", ok, err)
	}
	if _, err := st.ReadContextReference(ref.ContextRefID); err != nil {
		t.Fatalf("revocation deleted the canonical reference: %v", err)
	}

	reshared, regrant, err := st.ShareContext(span, "a_src", "a_dst", "second", "second desc")
	if err != nil {
		t.Fatalf("re-share: %v", err)
	}
	if reshared.ContextRefID != ref.ContextRefID || regrant.GrantID != grant.GrantID {
		t.Fatalf("re-share minted new ids: ref %q grant %q", reshared.ContextRefID, regrant.GrantID)
	}
	if regrant.Label != "second" || regrant.Description != "second desc" {
		t.Fatalf("re-share kept stale presentation: %+v", regrant)
	}
	if ok, err := st.ContextReadAuthorized(ref.ContextRefID, "a_dst"); err != nil || !ok {
		t.Fatalf("re-share did not restore authorization: %v %v", ok, err)
	}
	var grants int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM context_grants WHERE granted_to_agent_id = 'a_dst'`).Scan(&grants); err != nil {
		t.Fatalf("count: %v", err)
	}
	if grants != 1 {
		t.Fatalf("re-share added a duplicate list entry (%d grants)", grants)
	}
}

// A re-share also returns the grant to the recipient's normal list: hiding is
// list state about one offer, not a standing refusal of later ones.
func TestReshareClearsHiddenPreference(t *testing.T) {
	st, _ := newTestStore(t)
	contextAgent(t, st, "a_src", "source")
	contextAgent(t, st, "a_dst", "dest")
	span := transcriptSpan("a_src", 1, 1)
	_, grant, err := st.ShareContext(span, "a_src", "a_dst", "l", "d")
	if err != nil {
		t.Fatalf("share: %v", err)
	}
	if _, err := st.SetContextGrantHidden(grant.GrantID, "a_dst", true); err != nil {
		t.Fatalf("hide: %v", err)
	}
	if _, regrant, err := st.ShareContext(span, "a_src", "a_dst", "again", "d2"); err != nil || regrant.Hidden {
		t.Fatalf("re-share left the grant hidden: %+v (%v)", regrant, err)
	}
	if got, err := st.ListContextGrantsForRecipient("a_dst", false, 20, "", ""); err != nil || len(got) != 1 {
		t.Fatalf("re-shared grant missing from the normal list: %+v (%v)", got, err)
	}
}

// A7 — the recipient cascade is defensive schema hygiene only: it removes that
// recipient's grants and preferences and nothing else (FS-15.R14, TS-02.R24).
func TestRecipientDeletionCascadesOnlyItsOwnRows(t *testing.T) {
	st, _ := newTestStore(t)
	contextAgent(t, st, "a_src", "source")
	contextAgent(t, st, "a_dst", "dest")
	contextAgent(t, st, "a_peer", "peer")

	span := transcriptSpan("a_src", 2, 6)
	ref, grant, err := st.ShareContext(span, "a_src", "a_dst", "l", "d")
	if err != nil {
		t.Fatalf("share: %v", err)
	}
	if _, err := st.SetContextGrantHidden(grant.GrantID, "a_dst", true); err != nil {
		t.Fatalf("hide: %v", err)
	}
	if _, _, err := st.ShareContext(span, "a_src", "a_peer", "l", "d"); err != nil {
		t.Fatalf("share to peer: %v", err)
	}
	if err := st.DeleteAgent("a_dst"); err != nil {
		t.Fatalf("delete recipient: %v", err)
	}
	var grants, prefs int
	if err := st.DB().QueryRow(`SELECT
  (SELECT COUNT(*) FROM context_grants WHERE granted_to_agent_id = 'a_dst'),
  (SELECT COUNT(*) FROM context_grant_preferences WHERE grant_id = ?)`, grant.GrantID).Scan(&grants, &prefs); err != nil {
		t.Fatalf("count: %v", err)
	}
	if grants != 0 || prefs != 0 {
		t.Fatalf("recipient deletion left %d grants and %d preferences", grants, prefs)
	}
	if _, err := st.ReadContextReference(ref.ContextRefID); err != nil {
		t.Fatalf("recipient deletion cascaded into the canonical reference: %v", err)
	}
	if ok, err := st.ContextReadAuthorized(ref.ContextRefID, "a_peer"); err != nil || !ok {
		t.Fatalf("recipient deletion cascaded into the peer grant: %v %v", ok, err)
	}

	// A grantor id is retained provenance with no foreign key, so deleting the
	// source agent leaves the grant and reference intact (the read path then
	// reports a tombstone rather than losing the row).
	if err := st.DeleteAgent("a_src"); err != nil {
		t.Fatalf("delete grantor: %v", err)
	}
	if ok, err := st.ContextReadAuthorized(ref.ContextRefID, "a_peer"); err != nil || !ok {
		t.Fatalf("grantor deletion revoked a grant: %v %v", ok, err)
	}
	if _, err := st.ReadContextReference(ref.ContextRefID); err != nil {
		t.Fatalf("grantor deletion removed the reference: %v", err)
	}
}

// Listing traverses newest-first in bounded pages without repeating or skipping.
func TestListContextGrantsPagination(t *testing.T) {
	st, _ := newTestStore(t)
	contextAgent(t, st, "a_src", "source")
	contextAgent(t, st, "a_dst", "dest")
	for i := int64(1); i <= 5; i++ {
		if _, _, err := st.ShareContext(transcriptSpan("a_src", i, i), "a_src", "a_dst", "l", "d"); err != nil {
			t.Fatalf("share %d: %v", i, err)
		}
	}
	seen := map[string]bool{}
	afterAt, afterID := "", ""
	for page := 0; page < 5; page++ {
		got, err := st.ListContextGrantsForRecipient("a_dst", false, 2, afterAt, afterID)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if len(got) == 0 {
			break
		}
		for _, g := range got {
			if seen[g.GrantID] {
				t.Fatalf("grant %s returned twice", g.GrantID)
			}
			seen[g.GrantID] = true
		}
		last := got[len(got)-1]
		afterAt, afterID = formatTime(last.UpdatedAt), last.GrantID
	}
	if len(seen) != 5 {
		t.Fatalf("traversal saw %d of 5 grants", len(seen))
	}
}
