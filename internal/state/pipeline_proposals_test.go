package state

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func saveProposal(t *testing.T, store *Store, id string, at time.Time) {
	t.Helper()
	if err := store.SavePipelineProposal(PipelineProposalRecord{
		ProposalID: id, Kind: "save_template", Digest: id,
		Payload: json.RawMessage(`{"id":"quality"}`), CreatedAt: at,
	}, 3); err != nil {
		t.Fatalf("save %s: %v", id, err)
	}
}

// TS-09.R26 / FS-14.R33: an approved proposal's record stays durable but stops
// offering its approval, and the pending surface survives a reopen.
func TestConsumedPipelineProposalLeavesThePendingSurface(t *testing.T) {
	store, home := newTestStore(t)
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	saveProposal(t, store, "pp_keep", now)
	saveProposal(t, store, "pp_approved", now.Add(time.Minute))

	consumed, err := store.ConsumePipelineProposal("pp_approved", now.Add(2*time.Minute))
	if err != nil || !consumed {
		t.Fatalf("consume = %v err=%v", consumed, err)
	}
	again, err := store.ConsumePipelineProposal("pp_approved", now.Add(3*time.Minute))
	if err != nil || again {
		t.Fatalf("second consume = %v err=%v, want false", again, err)
	}
	if missing, err := store.ConsumePipelineProposal("pp_absent", now); err != nil || missing {
		t.Fatalf("absent consume = %v err=%v, want false", missing, err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	pending, err := reopened.ListPipelineProposals(10)
	if err != nil || len(pending) != 1 || pending[0].ProposalID != "pp_keep" {
		t.Fatalf("pending after reload = %+v err=%v", pending, err)
	}
}

// TS-09.R26 / TS-02.R22: never-approved proposals are bounded by retention, so
// the durable approval surface cannot grow without limit.
func TestPipelineProposalRetentionKeepsTheNewestRecords(t *testing.T) {
	store, _ := newTestStore(t)
	base := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		saveProposal(t, store, fmt.Sprintf("pp_%d", i), base.Add(time.Duration(i)*time.Minute))
	}
	pending, err := store.ListPipelineProposals(10)
	if err != nil || len(pending) != 3 {
		t.Fatalf("pending = %+v err=%v, want 3 retained", pending, err)
	}
	for i, want := range []string{"pp_4", "pp_3", "pp_2"} {
		if pending[i].ProposalID != want {
			t.Fatalf("pending[%d] = %s, want %s", i, pending[i].ProposalID, want)
		}
	}
}

// INV §7 / TS-09.R15: one unreadable row must not abort the entire approval
// list. Before the fix a single malformed created_at failed ListPipelineProposals
// and hid every valid proposal with it.
func TestListPipelineProposalsIsolatesAnUnreadableRecord(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	saveProposal(t, store, "pp_valid_old", now)
	saveProposal(t, store, "pp_valid_new", now.Add(2*time.Minute))
	if _, err := store.DB().Exec(`
INSERT INTO pipeline_proposals(proposal_id, kind, digest, payload_json, created_at, consumed_at)
VALUES ('pp_broken', 'save_template', 'digest', '{}', 'not-a-timestamp', '')`); err != nil {
		t.Fatal(err)
	}

	pending, err := store.ListPipelineProposals(10)
	if err != nil {
		t.Fatalf("ListPipelineProposals err = %v, want isolation", err)
	}
	if len(pending) != 2 || pending[0].ProposalID != "pp_valid_new" || pending[1].ProposalID != "pp_valid_old" {
		t.Fatalf("pending = %+v, want both valid proposals", pending)
	}
}

// TestReproposingAConsumedProposalRearmsIt — proposing the same content again
// after its approval was consumed must put exactly one pending offer back on the
// approval surface. Silently keeping it consumed would repeat the defect the
// durable proposal record exists to fix: an MCP caller told the proposal
// succeeded while no human surface holds it (FS-14.R33, TS-09.R26).
func TestReproposingAConsumedProposalRearmsIt(t *testing.T) {
	st, _ := newTestStore(t)
	record := PipelineProposalRecord{
		ProposalID: "pp_rearm", Kind: "save_template", Digest: "d_rearm",
		Payload: []byte(`{"id":"review"}`), CreatedAt: time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC),
	}
	if err := st.SavePipelineProposal(record, 10); err != nil {
		t.Fatalf("SavePipelineProposal: %v", err)
	}
	if consumed, err := st.ConsumePipelineProposal("pp_rearm", time.Now().UTC()); err != nil || !consumed {
		t.Fatalf("ConsumePipelineProposal = %v,%v want true,nil", consumed, err)
	}
	pending, err := st.ListPipelineProposals(10)
	if err != nil {
		t.Fatalf("ListPipelineProposals: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending after consumption = %+v, want none", pending)
	}

	record.CreatedAt = record.CreatedAt.Add(time.Hour)
	if err := st.SavePipelineProposal(record, 10); err != nil {
		t.Fatalf("re-propose: %v", err)
	}
	pending, err = st.ListPipelineProposals(10)
	if err != nil {
		t.Fatalf("ListPipelineProposals after re-propose: %v", err)
	}
	if len(pending) != 1 || pending[0].ProposalID != "pp_rearm" {
		t.Fatalf("pending after re-propose = %+v, want exactly one pp_rearm", pending)
	}
}
