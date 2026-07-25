package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
)

type Proposal struct {
	ProposalID string `json:"proposal_id"`
	Kind       string `json:"kind"`
	Digest     string `json:"digest"`
	Payload    any    `json:"payload"`
}

func (m *Manager) ProposeTemplate(id string, template Template) (Proposal, error) {
	record := m.templates.Validate(id, template)
	if !record.Valid {
		return Proposal{}, validationError("template proposal is invalid", record.Diagnostics)
	}
	payload := struct {
		ID       string   `json:"id"`
		Template Template `json:"template"`
	}{ID: id, Template: record.Template}
	if err := validateProposalSize(payload); err != nil {
		return Proposal{}, err
	}
	digest, err := Digest(payload)
	if err != nil {
		return Proposal{}, err
	}
	return Proposal{ProposalID: "pp_" + digest[:16], Kind: "save_template", Digest: digest, Payload: payload}, nil
}

func (m *Manager) ProposeRun(ctx context.Context, request StartRequest) (Proposal, error) {
	requestedID := request.RequestID
	if request.RequestID == "" {
		// Start validation requires an idempotency key, but the proposal's durable
		// key is derived from its exact canonical payload below. Use a bounded
		// placeholder only for validation; it must never escape in the proposal.
		request.RequestID = "proposal-validation"
	}
	if _, err := m.validateStart(ctx, &request); err != nil {
		return Proposal{}, err
	}
	digestPayload := request
	digestPayload.RequestID = ""
	if err := validateProposalSize(digestPayload); err != nil {
		return Proposal{}, err
	}
	digest, err := Digest(digestPayload)
	if err != nil {
		return Proposal{}, err
	}
	proposalID := "pp_" + digest[:16]
	if requestedID == "" {
		request.RequestID = proposalID
	} else {
		request.RequestID = requestedID
	}
	return Proposal{ProposalID: proposalID, Kind: "start_run", Digest: digest, Payload: request}, nil
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
