package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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

func splitProposals(t *testing.T, store *Store) (pending, declined []PipelineProposalRecord) {
	t.Helper()
	records, err := store.ListPipelineProposals(10)
	if err != nil {
		t.Fatalf("ListPipelineProposals: %v", err)
	}
	for _, record := range records {
		if record.DeclinedAt != nil {
			declined = append(declined, record)
			continue
		}
		pending = append(pending, record)
	}
	return pending, declined
}

// FS-14.R49 / TS-02.R29: rejecting an offer moves it out of the pending records
// and into the declined ones with the time it was declined, and that survives a
// reopen because the decline is a durable column rather than page state.
func TestDeclinedPipelineProposalLeavesThePendingRecordsAndSurvivesReload(t *testing.T) {
	store, home := newTestStore(t)
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	saveProposal(t, store, "pp_keep", now)
	saveProposal(t, store, "pp_reject", now.Add(time.Minute))

	declinedAt := now.Add(2 * time.Minute)
	record, err := store.DeclinePipelineProposal("pp_reject", declinedAt)
	if err != nil {
		t.Fatalf("DeclinePipelineProposal: %v", err)
	}
	if record.DeclinedAt == nil || !record.DeclinedAt.Equal(declinedAt) {
		t.Fatalf("declined record = %+v, want declined_at %s", record, declinedAt)
	}
	if record.Kind != "save_template" || string(record.Payload) != `{"id":"quality"}` {
		t.Fatalf("declined record = %+v, want the offer unchanged", record)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	pending, declined := splitProposals(t, reopened)
	if len(pending) != 1 || pending[0].ProposalID != "pp_keep" {
		t.Fatalf("pending after reload = %+v, want only pp_keep", pending)
	}
	if len(declined) != 1 || declined[0].ProposalID != "pp_reject" || declined[0].DeclinedAt == nil {
		t.Fatalf("declined after reload = %+v, want pp_reject with its decline time", declined)
	}
}

// FS-14.R57 / TS-02.R29: every proposal transition is one conditional claim, so
// two tabs racing the same offer produce exactly one effect and the loser is told
// the state the row is actually in rather than a second success or a silent
// no-op.
func TestPipelineProposalClaimLosersReportTheRealState(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	saveProposal(t, store, "pp_declined", now)
	saveProposal(t, store, "pp_consumed", now.Add(time.Minute))
	saveProposal(t, store, "pp_pending", now.Add(2*time.Minute))
	if _, err := store.DeclinePipelineProposal("pp_declined", now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if consumed, err := store.ConsumePipelineProposal("pp_consumed", now.Add(3*time.Minute)); err != nil || !consumed {
		t.Fatalf("consume = %v err=%v", consumed, err)
	}

	for _, tc := range []struct {
		name string
		act  func() error
		want error
	}{
		{"decline an already declined record", func() error {
			_, err := store.DeclinePipelineProposal("pp_declined", now.Add(4*time.Minute))
			return err
		}, ErrPipelineProposalDeclined},
		{"decline a consumed record", func() error {
			_, err := store.DeclinePipelineProposal("pp_consumed", now.Add(4*time.Minute))
			return err
		}, ErrPipelineProposalConsumed},
		{"decline an unknown record", func() error {
			_, err := store.DeclinePipelineProposal("pp_absent", now.Add(4*time.Minute))
			return err
		}, ErrNotFound},
		{"delete a still pending record", func() error {
			return store.DeletePipelineProposal("pp_pending")
		}, ErrPipelineProposalNotDeclined},
		{"delete an unknown record", func() error {
			return store.DeletePipelineProposal("pp_absent")
		}, ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.act(); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	// The refused claims changed nothing: the pending record is still pending and
	// the declined one is still exactly once in the declined records.
	pending, declined := splitProposals(t, store)
	if len(pending) != 1 || pending[0].ProposalID != "pp_pending" {
		t.Fatalf("pending = %+v, want only pp_pending", pending)
	}
	if len(declined) != 1 || declined[0].ProposalID != "pp_declined" {
		t.Fatalf("declined = %+v, want only pp_declined", declined)
	}
}

// FS-14.R49 / TS-02.R29: Delete is a hard row delete on a declined record and
// leaves no tombstone, so a record another tab already deleted reports as gone
// and a later identical proposal inserts a new row.
func TestDeletingADeclinedPipelineProposalLeavesNoTombstone(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	saveProposal(t, store, "pp_gone", now)
	if _, err := store.DeclinePipelineProposal("pp_gone", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePipelineProposal("pp_gone"); err != nil {
		t.Fatalf("DeletePipelineProposal: %v", err)
	}
	if err := store.DeletePipelineProposal("pp_gone"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v, want ErrNotFound", err)
	}
	if pending, declined := splitProposals(t, store); len(pending) != 0 || len(declined) != 0 {
		t.Fatalf("pending = %+v declined = %+v, want the record gone from both", pending, declined)
	}

	saveProposal(t, store, "pp_gone", now.Add(2*time.Minute))
	pending, declined := splitProposals(t, store)
	if len(pending) != 1 || pending[0].ProposalID != "pp_gone" || len(declined) != 0 {
		t.Fatalf("pending = %+v declined = %+v, want exactly one pending offer after re-proposal", pending, declined)
	}
}

// FS-14.R57 / TS-02.R29: approval consumes a declined row without clearing its
// decline, so the ordinary multi-tab order Reject → approval → stale Delete must
// refuse with the durable consumed state and leave the record intact. Deleting on
// declined_at alone erased the consumed record instead.
func TestDeletingAConsumedProposalIsRefusedAndKeepsTheRecord(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	saveProposal(t, store, "pp_stale", now)
	if _, err := store.DeclinePipelineProposal("pp_stale", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if consumed, err := store.ConsumePipelineProposal("pp_stale", now.Add(2*time.Minute)); err != nil || !consumed {
		t.Fatalf("consume after decline = %v err=%v, want the mutation to win", consumed, err)
	}

	if err := store.DeletePipelineProposal("pp_stale"); !errors.Is(err, ErrPipelineProposalConsumed) {
		t.Fatalf("stale delete = %v, want ErrPipelineProposalConsumed", err)
	}
	// The consumed record is listed in neither collection, so durability is read
	// from the row itself rather than from the projection that hides it.
	var consumedAt, declinedAt string
	if err := store.db.QueryRow(`SELECT consumed_at, declined_at FROM pipeline_proposals WHERE proposal_id = ?`, "pp_stale").
		Scan(&consumedAt, &declinedAt); err != nil {
		t.Fatalf("read pp_stale after refused delete: %v", err)
	}
	if consumedAt == "" || declinedAt == "" {
		t.Fatalf("row = consumed_at %q declined_at %q, want both marks kept", consumedAt, declinedAt)
	}
}

// FS-14.R50: declining refuses one offer, not its content. Proposing the same
// content again after a reject returns exactly one pending offer rather than
// leaving it silently declined, which would repeat the discoverability defect
// the durable record exists to remove.
func TestReproposingADeclinedProposalRearmsItAsPending(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	saveProposal(t, store, "pp_rearm", now)
	if _, err := store.DeclinePipelineProposal("pp_rearm", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	saveProposal(t, store, "pp_rearm", now.Add(2*time.Minute))
	pending, declined := splitProposals(t, store)
	if len(pending) != 1 || pending[0].ProposalID != "pp_rearm" || pending[0].DeclinedAt != nil {
		t.Fatalf("pending = %+v, want exactly one pending pp_rearm", pending)
	}
	if len(declined) != 0 {
		t.Fatalf("declined = %+v, want the decline cleared by the re-proposal", declined)
	}
}

// FS-14.R57 / TS-02.R29: the durable mutation wins every race. Consumption
// matches consumed_at alone, so an approval a person confirmed takes effect even
// when another tab rejected the same offer moments earlier, and the record then
// leaves both the pending and the declined records as consumed.
func TestAnApprovalConsumesAProposalAnotherTabAlreadyDeclined(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	saveProposal(t, store, "pp_race", now)
	if _, err := store.DeclinePipelineProposal("pp_race", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	consumed, err := store.ConsumePipelineProposal("pp_race", now.Add(2*time.Minute))
	if err != nil || !consumed {
		t.Fatalf("consume after decline = %v err=%v, want the mutation to win", consumed, err)
	}
	if pending, declined := splitProposals(t, store); len(pending) != 0 || len(declined) != 0 {
		t.Fatalf("pending = %+v declined = %+v, want the record listed as neither", pending, declined)
	}
}

// INV §5 / FS-14.R57: the conditional claim, not a read followed by a write, is
// what decides. Eight tabs rejecting and eight deleting the same offer at once
// produce exactly one decline and at most one delete, and every loser carries a
// state error rather than a second success.
func TestConcurrentPipelineProposalClaimsProduceOneEffect(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	saveProposal(t, store, "pp_contended", now)

	const racers = 8
	var wg sync.WaitGroup
	var declines, deletes atomic.Int64
	losses := make(chan error, 2*racers)
	for i := 0; i < racers; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := store.DeclinePipelineProposal("pp_contended", now.Add(time.Minute)); err != nil {
				losses <- err
				return
			}
			declines.Add(1)
		}()
		go func() {
			defer wg.Done()
			if err := store.DeletePipelineProposal("pp_contended"); err != nil {
				losses <- err
				return
			}
			deletes.Add(1)
		}()
	}
	wg.Wait()
	close(losses)

	if declines.Load() != 1 {
		t.Fatalf("declines = %d, want exactly one", declines.Load())
	}
	if got := deletes.Load(); got > 1 {
		t.Fatalf("deletes = %d, want at most one", got)
	}
	for err := range losses {
		switch {
		case errors.Is(err, ErrNotFound),
			errors.Is(err, ErrPipelineProposalDeclined),
			errors.Is(err, ErrPipelineProposalNotDeclined):
		default:
			t.Fatalf("loser err = %v, want a state the record is actually in", err)
		}
	}
}
