package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

func runProposalRequest(requestID string) StartRequest {
	return StartRequest{
		RequestID: requestID, TemplateID: "quality", DisplayName: "Ship", Project: "app", Goal: "Implement the spec",
		Inputs: map[string]string{"spec": "Requirements"},
		Assignments: map[string]RuntimeAssignment{
			"work": {Backend: "codex", Model: "gpt"}, "review": {Backend: "claude", Model: "sonnet"},
		},
	}
}

func pendingRequestID(t *testing.T, manager *Manager) (string, string) {
	t.Helper()
	collections, err := manager.ListProposals()
	if err != nil || len(collections.Pending) != 1 {
		t.Fatalf("pending = %+v err=%v, want exactly one record", collections.Pending, err)
	}
	payload, ok := collections.Pending[0].Payload.(map[string]any)
	if !ok {
		t.Fatalf("durable payload = %#v", collections.Pending[0].Payload)
	}
	id, _ := payload["request_id"].(string)
	return collections.Pending[0].ProposalID, id
}

// INV §3 / TS-09.R15/R16: the caller's idempotency key is one-shot transport
// data and must not reach the content-addressed digest or the persisted payload.
// Before the fix the digest excluded it but the returned payload restored it,
// while the store keeps the first record on id conflict — so the second caller
// got a payload the approval surface did not hold, and approving replayed the
// first run. Both orderings must agree.
func TestRunProposalCanonicalizesTheCallerRequestIDInBothOrderings(t *testing.T) {
	for _, ordering := range [][2]string{{"caller-a", "caller-b"}, {"caller-b", "caller-a"}} {
		t.Run(ordering[0]+"-then-"+ordering[1], func(t *testing.T) {
			manager, _, _ := pipelineManagerFixture(t)
			first, err := manager.ProposeRun(context.Background(), runProposalRequest(ordering[0]))
			if err != nil {
				t.Fatal(err)
			}
			second, err := manager.ProposeRun(context.Background(), runProposalRequest(ordering[1]))
			if err != nil {
				t.Fatal(err)
			}
			if first.ProposalID != second.ProposalID || first.Digest != second.Digest {
				t.Fatalf("first = %+v second = %+v, want one canonical proposal", first, second)
			}
			for _, proposal := range []Proposal{first, second} {
				payload, ok := proposal.Payload.(StartRequest)
				if !ok || payload.RequestID != proposal.ProposalID {
					t.Fatalf("returned payload = %#v, want request_id %s", proposal.Payload, proposal.ProposalID)
				}
			}
			proposalID, durableRequestID := pendingRequestID(t, manager)
			if proposalID != second.ProposalID || durableRequestID != second.ProposalID {
				t.Fatalf("durable proposal = %s/%s, want the exact returned payload %s", proposalID, durableRequestID, second.ProposalID)
			}
		})
	}
}

// FS-14.R33 / TS-09.R26: an approved Start consumes its proposal only after the
// run is durable, and a reloaded page (a fresh manager over the same store) no
// longer offers that approval.
func TestApprovedRunProposalStopsBeingPendingAfterReload(t *testing.T) {
	manager, lifecycle, publisher := pipelineManagerFixture(t)
	proposal, err := manager.ProposeRun(context.Background(), runProposalRequest(""))
	if err != nil {
		t.Fatal(err)
	}
	if pending, err := manager.ListProposals(); err != nil || len(pending.Pending) != 1 {
		t.Fatalf("pending before approval = %+v err=%v", pending, err)
	}
	before := publisher.proposalUpdates

	detail, replay, err := manager.Start(context.Background(), proposal.Payload.(StartRequest))
	if err != nil || replay {
		t.Fatalf("Start = %+v replay=%v err=%v", detail, replay, err)
	}
	if publisher.proposalUpdates != before+1 {
		t.Fatalf("proposal updates = %d, want one publication after the approved mutation", publisher.proposalUpdates-before)
	}

	reloaded := NewManager(manager.store, manager.templates, lifecycle, publisher)
	pending, err := reloaded.ListProposals()
	if err != nil || len(pending.Pending) != 0 {
		t.Fatalf("pending after approval and reload = %+v err=%v", pending, err)
	}
}

// FS-14.R33 / TS-09.R26: the exact saved draft consumes its own proposal, so no
// proposal id has to travel through the template API.
func TestApprovedTemplateProposalIsConsumedByItsExactSave(t *testing.T) {
	manager, _, publisher := pipelineManagerFixture(t)
	template := Template{
		Version: 1, Title: "Revised loop",
		Inputs: []ValueDecl{{Name: "spec", Description: "Specification", Required: true}},
		Stages: []Stage{{
			ID: "work", Title: "Work", Role: "implementer", Instruction: "Implement it.", MaxVisits: 1,
			Inputs:      []StageInput{{Name: "specification", Value: "spec", Required: true}},
			Outputs:     []StageOutput{},
			Transitions: OutcomeTransitions{Success: Transition{Final: "success", Approval: "automatic"}, Failure: Transition{Final: "failure", Approval: "required"}},
		}},
	}
	proposal, err := manager.ProposeTemplate("quality", template)
	if err != nil {
		t.Fatal(err)
	}
	if pending, err := manager.ListProposals(); err != nil || len(pending.Pending) != 1 {
		t.Fatalf("pending before approval = %+v err=%v", pending, err)
	}

	record, err := manager.templates.Update("quality", template)
	if err != nil || !record.Valid {
		t.Fatalf("Update = %+v err=%v", record, err)
	}
	before := publisher.proposalUpdates
	manager.ConsumeTemplateProposal(record.ID, record.Template)
	if publisher.proposalUpdates != before+1 {
		t.Fatalf("proposal updates = %d, want one publication", publisher.proposalUpdates-before)
	}
	if pending, err := manager.ListProposals(); err != nil || len(pending.Pending) != 0 {
		t.Fatalf("pending after approval = %+v err=%v (proposal %s)", pending, err, proposal.ProposalID)
	}
	// Consumption is once: a repeated save publishes nothing new.
	manager.ConsumeTemplateProposal(record.ID, record.Template)
	if publisher.proposalUpdates != before+1 {
		t.Fatalf("proposal updates after repeat = %d, want no second publication", publisher.proposalUpdates-before)
	}
}

// INV §7 / TS-09.R15: one corrupt payload cannot be rendered for exact approval,
// but it must not abort the approval list either. Before the fix a single
// undecodable record failed ListProposals and hid every valid proposal.
func TestListProposalsIsolatesAnUndecodableRecord(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	valid, err := manager.ProposeRun(context.Background(), runProposalRequest(""))
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.store.SavePipelineProposal(state.PipelineProposalRecord{
		ProposalID: "pp_corrupt", Kind: "start_run", Digest: "corrupt",
		Payload: json.RawMessage(`{"template_id": `), CreatedAt: time.Now().UTC().Add(time.Minute),
	}, MaxProposalRecords); err != nil {
		t.Fatal(err)
	}

	pending, err := manager.ListProposals()
	if err != nil {
		t.Fatalf("ListProposals err = %v, want isolation", err)
	}
	if len(pending.Pending) != 1 || pending.Pending[0].ProposalID != valid.ProposalID {
		t.Fatalf("pending = %+v, want only the readable proposal", pending.Pending)
	}
}

func proposalTemplateFixture() Template {
	return Template{
		Version: 1, Title: "Revised loop",
		Inputs: []ValueDecl{{Name: "spec", Description: "Specification", Required: true}},
		Stages: []Stage{{
			ID: "work", Title: "Work", Role: "implementer", Instruction: "Implement it.", MaxVisits: 1,
			Inputs:      []StageInput{{Name: "specification", Value: "spec", Required: true}},
			Outputs:     []StageOutput{},
			Transitions: OutcomeTransitions{Success: Transition{Final: "success", Approval: "automatic"}, Failure: Transition{Final: "failure", Approval: "required"}},
		}},
	}
}

// FS-14.R49 / TS-09.R32: Reject is a pure record action. It moves the offer into
// the declined collection with its decline time, publishes the one refetch
// signal the surface already listens for, and Delete then removes it.
func TestRejectMovesAnOfferToTheDeclinedCollectionAndDeleteRemovesIt(t *testing.T) {
	manager, _, publisher := pipelineManagerFixture(t)
	proposal, err := manager.ProposeRun(context.Background(), runProposalRequest(""))
	if err != nil {
		t.Fatal(err)
	}
	before := publisher.proposalUpdates

	declined, err := manager.DeclineProposal(proposal.ProposalID)
	if err != nil {
		t.Fatalf("DeclineProposal: %v", err)
	}
	if declined.DeclinedAt == nil || declined.ProposalID != proposal.ProposalID {
		t.Fatalf("declined = %+v, want the same offer with a decline time", declined)
	}
	if publisher.proposalUpdates != before+1 {
		t.Fatalf("proposal updates = %d, want one publication for the decline", publisher.proposalUpdates-before)
	}
	collections, err := manager.ListProposals()
	if err != nil || len(collections.Pending) != 0 || len(collections.Declined) != 1 {
		t.Fatalf("collections after reject = %+v err=%v", collections, err)
	}
	if collections.Declined[0].Payload == nil || collections.Declined[0].CreatedAt.IsZero() {
		t.Fatalf("declined entry = %+v, want the payload and creation time it was offered with", collections.Declined[0])
	}

	if err := manager.DeleteProposal(proposal.ProposalID); err != nil {
		t.Fatalf("DeleteProposal: %v", err)
	}
	if publisher.proposalUpdates != before+2 {
		t.Fatalf("proposal updates = %d, want one more publication for the delete", publisher.proposalUpdates-before)
	}
	if collections, err = manager.ListProposals(); err != nil || len(collections.Pending) != 0 || len(collections.Declined) != 0 {
		t.Fatalf("collections after delete = %+v err=%v, want both empty", collections, err)
	}
}

// FS-14.R57 / A27: both orderings of the Reject-versus-approval race, for both
// proposal kinds, against the durable store. The mutation always wins: an
// approval that commits first leaves a later Reject refused as consumed with no
// declined entry, and a Reject that lands first does not prevent the approval,
// after which the record is listed in neither collection while its template or
// run exists exactly once.
func TestRejectVersusApprovalResolvesToTheDurableMutation(t *testing.T) {
	kinds := map[string]struct {
		propose func(t *testing.T, m *Manager) Proposal
		approve func(t *testing.T, m *Manager, p Proposal)
		exists  func(t *testing.T, m *Manager) int
	}{
		"save_template": {
			propose: func(t *testing.T, m *Manager) Proposal {
				proposal, err := m.ProposeTemplate("quality", proposalTemplateFixture())
				if err != nil {
					t.Fatal(err)
				}
				return proposal
			},
			approve: func(t *testing.T, m *Manager, _ Proposal) {
				record, err := m.templates.Update("quality", proposalTemplateFixture())
				if err != nil || !record.Valid {
					t.Fatalf("Update = %+v err=%v", record, err)
				}
				m.ConsumeTemplateProposal(record.ID, record.Template)
			},
			exists: func(t *testing.T, m *Manager) int {
				record, err := m.templates.Read("quality")
				if err != nil {
					t.Fatalf("Read: %v", err)
				}
				if record.Template.Title != "Revised loop" {
					return 0
				}
				return 1
			},
		},
		"start_run": {
			propose: func(t *testing.T, m *Manager) Proposal {
				proposal, err := m.ProposeRun(context.Background(), runProposalRequest(""))
				if err != nil {
					t.Fatal(err)
				}
				return proposal
			},
			approve: func(t *testing.T, m *Manager, p Proposal) {
				detail, replay, err := m.Start(context.Background(), p.Payload.(StartRequest))
				if err != nil || replay {
					t.Fatalf("Start = %+v replay=%v err=%v", detail, replay, err)
				}
			},
			exists: func(t *testing.T, m *Manager) int {
				runs, err := m.List(10, 0)
				if err != nil {
					t.Fatalf("List: %v", err)
				}
				return len(runs)
			},
		},
	}

	for kind, tc := range kinds {
		t.Run(kind+"/approval first", func(t *testing.T) {
			manager, _, _ := pipelineManagerFixture(t)
			proposal := tc.propose(t, manager)
			tc.approve(t, manager, proposal)

			_, err := manager.DeclineProposal(proposal.ProposalID)
			if !errors.Is(err, state.ErrPipelineProposalConsumed) {
				t.Fatalf("decline after approval = %v, want the consumed refusal", err)
			}
			collections, err := manager.ListProposals()
			if err != nil || len(collections.Pending) != 0 || len(collections.Declined) != 0 {
				t.Fatalf("collections = %+v err=%v, want the entry gone as consumed", collections, err)
			}
			if got := tc.exists(t, manager); got != 1 {
				t.Fatalf("committed mutations = %d, want exactly one", got)
			}
		})

		t.Run(kind+"/reject first", func(t *testing.T) {
			manager, _, _ := pipelineManagerFixture(t)
			proposal := tc.propose(t, manager)
			if _, err := manager.DeclineProposal(proposal.ProposalID); err != nil {
				t.Fatalf("DeclineProposal: %v", err)
			}
			tc.approve(t, manager, proposal)

			collections, err := manager.ListProposals()
			if err != nil || len(collections.Pending) != 0 || len(collections.Declined) != 0 {
				t.Fatalf("collections = %+v err=%v, want the record consumed rather than declined", collections, err)
			}
			if got := tc.exists(t, manager); got != 1 {
				t.Fatalf("committed mutations = %d, want exactly one", got)
			}
		})
	}
}

// FS-14.R57 / TS-09.R26: the accepted failure mode. A mutation that commits
// while its consumption mark never lands leaves exactly one listed offer for
// content that already exists, and the next identical proposal re-arms that same
// record rather than adding a second. Nothing repairs it by guessing.
func TestAMutationWithoutItsConsumptionMarkLeavesOneRearmableOffer(t *testing.T) {
	manager, _, _ := pipelineManagerFixture(t)
	template := proposalTemplateFixture()
	proposal, err := manager.ProposeTemplate("quality", template)
	if err != nil {
		t.Fatal(err)
	}
	// The Save commits; the process ends before ConsumeTemplateProposal runs.
	if record, err := manager.templates.Update("quality", template); err != nil || !record.Valid {
		t.Fatalf("Update = %+v err=%v", record, err)
	}

	collections, err := manager.ListProposals()
	if err != nil || len(collections.Pending) != 1 || collections.Pending[0].ProposalID != proposal.ProposalID {
		t.Fatalf("collections = %+v err=%v, want the one leftover offer", collections, err)
	}
	if _, err := manager.ProposeTemplate("quality", template); err != nil {
		t.Fatal(err)
	}
	if collections, err = manager.ListProposals(); err != nil || len(collections.Pending) != 1 {
		t.Fatalf("collections after re-proposal = %+v err=%v, want exactly one offer", collections, err)
	}
	// A person can then reject the leftover offer.
	if _, err := manager.DeclineProposal(proposal.ProposalID); err != nil {
		t.Fatalf("DeclineProposal: %v", err)
	}
}
