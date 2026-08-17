package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

type Proposal struct {
	ProposalID string `json:"proposal_id"`
	Kind       string `json:"kind"`
	Digest     string `json:"digest"`
	Payload    any    `json:"payload"`
}

type templateProposalPayload struct {
	ID       string   `json:"id"`
	Template Template `json:"template"`
}

// templateProposalIdentity is the one place a saved-template draft becomes a
// content-addressed proposal, so proposing and consuming an exact draft cannot
// derive different ids (INV §2).
func templateProposalIdentity(id string, template Template) (Proposal, error) {
	payload := templateProposalPayload{ID: id, Template: template}
	digest, err := Digest(payload)
	if err != nil {
		return Proposal{}, err
	}
	return Proposal{ProposalID: proposalIDFor(digest), Kind: "save_template", Digest: digest, Payload: payload}, nil
}

func proposalIDFor(digest string) string {
	return "pp_" + digest[:16]
}

func (m *Manager) ProposeTemplate(id string, template Template) (Proposal, error) {
	record := m.templates.Validate(id, template)
	if !record.Valid {
		return Proposal{}, validationError("template proposal is invalid", record.Diagnostics)
	}
	proposal, err := templateProposalIdentity(id, record.Template)
	if err != nil {
		return Proposal{}, err
	}
	if err := validateProposalSize(proposal.Payload); err != nil {
		return Proposal{}, err
	}
	return m.saveProposal(proposal)
}

func (m *Manager) ProposeRun(ctx context.Context, request StartRequest) (Proposal, error) {
	// The proposal's durable key is its exact canonical payload, and the record is
	// kept on id conflict. A caller-supplied idempotency key must therefore never
	// reach the digest or the payload: two otherwise-identical proposals would
	// otherwise return the second payload to MCP while the approval surface still
	// held the first, letting approval replay an older run (TS-09.R15/R16).
	// Start validation still requires a key, so use a bounded placeholder that is
	// cleared again before anything is derived from it.
	request.RequestID = "proposal-validation"
	if _, err := m.validateStart(ctx, &request); err != nil {
		return Proposal{}, err
	}
	request.RequestID = ""
	if err := validateProposalSize(request); err != nil {
		return Proposal{}, err
	}
	digest, err := Digest(request)
	if err != nil {
		return Proposal{}, err
	}
	request.RequestID = proposalIDFor(digest)
	return m.saveProposal(Proposal{ProposalID: request.RequestID, Kind: "start_run", Digest: digest, Payload: request})
}

// ConsumeTemplateProposal retires the pending record whose exact draft a
// completed Save just committed. It is keyed by the same content-addressed id
// the proposal was created with, so no proposal id has to travel through the
// template API (FS-14.R33, TS-09.R26).
func (m *Manager) ConsumeTemplateProposal(id string, template Template) {
	proposal, err := templateProposalIdentity(id, template)
	if err != nil {
		slog.Warn("pipeline: derive saved template proposal id", "template", id, "err", err)
		return
	}
	m.consumeProposal(proposal.ProposalID)
}

// consumeProposal runs only after the approved mutation is durable, so a failure
// here can leave a stale pending record but never undoes committed work
// (INV §15). It publishes only for the call that actually consumed the record.
func (m *Manager) consumeProposal(proposalID string) {
	consumed, err := m.store.ConsumePipelineProposal(proposalID, time.Now().UTC())
	if err != nil {
		slog.Warn("pipeline: consume proposal", "proposal", proposalID, "err", err)
		return
	}
	if consumed && m.publisher != nil {
		m.publisher.PublishPipelineProposalUpdate()
	}
}

// ListProposals is the Pipelines page's server-owned authority. It deliberately
// does not reconstruct proposals from runtime transcript events, because an ACP
// adapter may omit terminal tool-result content.
func (m *Manager) ListProposals() ([]Proposal, error) {
	records, err := m.store.ListPipelineProposals(MaxListPage)
	if err != nil {
		return nil, err
	}
	proposals := make([]Proposal, 0, len(records))
	for _, record := range records {
		var payload any
		if err := json.Unmarshal(record.Payload, &payload); err != nil {
			// One corrupt payload cannot be rendered for exact approval, but it
			// must not hide every other approvable proposal either (INV §7).
			slog.Warn("pipeline: skip proposal with undecodable payload", "proposal", record.ProposalID, "err", err)
			continue
		}
		proposals = append(proposals, Proposal{ProposalID: record.ProposalID, Kind: record.Kind, Digest: record.Digest, Payload: payload})
	}
	return proposals, nil
}

func (m *Manager) saveProposal(proposal Proposal) (Proposal, error) {
	payload, err := json.Marshal(proposal.Payload)
	if err != nil {
		return Proposal{}, err
	}
	if err := m.store.SavePipelineProposal(state.PipelineProposalRecord{
		ProposalID: proposal.ProposalID, Kind: proposal.Kind, Digest: proposal.Digest, Payload: payload, CreatedAt: time.Now().UTC(),
	}, MaxProposalRecords); err != nil {
		return Proposal{}, err
	}
	if m.publisher != nil {
		m.publisher.PublishPipelineProposalUpdate()
	}
	return proposal, nil
}

func validateProposalSize(payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if len(data) > MaxProposalBytes {
		return validationError("pipeline proposal is too large", []Diagnostic{{
			Field: "", Code: "too_large", Message: fmt.Sprintf("proposal must be at most %d bytes", MaxProposalBytes),
		}})
	}
	return nil
}
