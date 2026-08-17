package pipeline

import (
	"context"
	"encoding/json"
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
	pending, err := manager.ListProposals()
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %+v err=%v, want exactly one record", pending, err)
	}
	payload, ok := pending[0].Payload.(map[string]any)
	if !ok {
		t.Fatalf("durable payload = %#v", pending[0].Payload)
	}
	id, _ := payload["request_id"].(string)
	return pending[0].ProposalID, id
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
	if pending, err := manager.ListProposals(); err != nil || len(pending) != 1 {
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
	if err != nil || len(pending) != 0 {
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
	if pending, err := manager.ListProposals(); err != nil || len(pending) != 1 {
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
	if pending, err := manager.ListProposals(); err != nil || len(pending) != 0 {
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
	if len(pending) != 1 || pending[0].ProposalID != valid.ProposalID {
		t.Fatalf("pending = %+v, want only the readable proposal", pending)
	}
}
