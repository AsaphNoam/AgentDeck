package state

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// NewPipelineRunID and NewPipelineAttemptID mint durable opaque ids. Identity
// display names and template stage ids never substitute for these keys.
func (s *Store) NewPipelineRunID() (string, error) {
	return s.newPipelineID("pr_", "pipeline_runs", "run_id")
}

func (s *Store) NewPipelineAttemptID() (string, error) {
	return s.newPipelineID("pa_", "pipeline_attempts", "attempt_id")
}

func (s *Store) newPipelineID(prefix, table, column string) (string, error) {
	for i := 0; i < 10; i++ {
		var bytes [8]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			return "", fmt.Errorf("state: mint pipeline id: %w", err)
		}
		id := prefix + hex.EncodeToString(bytes[:])
		var exists int
		query := "SELECT 1 FROM " + table + " WHERE " + column + " = ?"
		err := s.db.QueryRow(query, id).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return id, nil
		}
		if err != nil {
			return "", fmt.Errorf("state: check pipeline id collision: %w", err)
		}
	}
	return "", errors.New("state: could not mint unique pipeline id")
}

// CreatePipelineRun persists the immutable snapshot, initial machine state,
// initial values, and caller idempotency key in one transaction. Exact replay
// returns the original row; mismatched content is a conflict.
func (s *Store) CreatePipelineRun(params CreatePipelineRunParams) (PipelineRunRecord, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, false, fmt.Errorf("state: begin pipeline run: %w", err)
	}
	defer tx.Rollback()

	var existingRunID, existingHash string
	err = tx.QueryRow(`SELECT run_id, request_hash FROM pipeline_requests WHERE request_id = ?`, params.RequestID).Scan(&existingRunID, &existingHash)
	if err == nil {
		if existingHash != params.RequestHash {
			return PipelineRunRecord{}, false, ErrPipelineRequestConflict
		}
		run, readErr := readPipelineRunQuery(tx, existingRunID)
		if readErr != nil {
			return PipelineRunRecord{}, false, readErr
		}
		return run, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return PipelineRunRecord{}, false, fmt.Errorf("state: read pipeline request: %w", err)
	}

	run := params.Run
	if run.CreatedAt.IsZero() {
		run.CreatedAt = timeNow()
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}
	if run.Revision <= 0 {
		run.Revision = 1
	}
	run.TemplateSnapshot = nonEmptyJSON(run.TemplateSnapshot, `{}`)
	run.Inputs = nonEmptyJSON(run.Inputs, `{}`)
	run.Assignments = nonEmptyJSON(run.Assignments, `{}`)
	if _, err := tx.Exec(`
INSERT INTO pipeline_runs(
  run_id, template_id, template_snapshot_json, display_name, project, goal,
  inputs_json, assignments_json, state, revision, pending_action,
  current_stage_id, current_attempt_id, current_agent_id, attention_reason,
  final_outcome, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.RunID, run.TemplateID, string(run.TemplateSnapshot), run.DisplayName, run.Project, run.Goal,
		string(run.Inputs), string(run.Assignments), run.State, run.Revision, run.PendingAction,
		run.CurrentStageID, run.CurrentAttemptID, run.CurrentAgentID, run.AttentionReason,
		run.FinalOutcome, formatTime(run.CreatedAt), formatTime(run.UpdatedAt),
	); err != nil {
		return PipelineRunRecord{}, false, fmt.Errorf("state: insert pipeline run: %w", err)
	}
	for _, value := range params.Values {
		updatedAt := value.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = run.CreatedAt
		}
		if _, err := tx.Exec(`
INSERT INTO pipeline_values(run_id, name, value, source_kind, source_attempt_id, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`, run.RunID, value.Name, value.Value, value.SourceKind, value.SourceAttemptID, formatTime(updatedAt)); err != nil {
			return PipelineRunRecord{}, false, fmt.Errorf("state: insert pipeline value: %w", err)
		}
	}
	if params.InitialAttempt != nil {
		attempt := *params.InitialAttempt
		if attempt.RunID == "" {
			attempt.RunID = run.RunID
		}
		if attempt.CreatedAt.IsZero() {
			attempt.CreatedAt = run.CreatedAt
		}
		if attempt.UpdatedAt.IsZero() {
			attempt.UpdatedAt = attempt.CreatedAt
		}
		if err := insertPipelineAttemptTx(tx, attempt); err != nil {
			return PipelineRunRecord{}, false, err
		}
	}
	if _, err := tx.Exec(`
INSERT INTO pipeline_requests(request_id, request_hash, run_id, created_at)
VALUES (?, ?, ?, ?)`, params.RequestID, params.RequestHash, run.RunID, formatTime(run.CreatedAt)); err != nil {
		return PipelineRunRecord{}, false, fmt.Errorf("state: insert pipeline request: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, false, fmt.Errorf("state: commit pipeline run: %w", err)
	}
	return run, false, nil
}

func (s *Store) ReadPipelineRequest(requestID string) (runID, requestHash string, err error) {
	err = s.db.QueryRow(`SELECT run_id, request_hash FROM pipeline_requests WHERE request_id = ?`, requestID).Scan(&runID, &requestHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("state: read pipeline request: %w", err)
	}
	return runID, requestHash, nil
}

// SavePipelineProposal publishes one canonical proposal durably before its MCP
// caller receives success. proposal_id is content-derived, so a transport retry
// leaves one reviewable record rather than duplicating the approval surface.
// The same transaction applies the caller's retention bound, so a backlog of
// never-approved proposals cannot grow without limit.
func (s *Store) SavePipelineProposal(proposal PipelineProposalRecord, retain int) error {
	if proposal.CreatedAt.IsZero() {
		proposal.CreatedAt = timeNow()
	}
	if retain <= 0 {
		retain = 100
	}
	proposal.Payload = nonEmptyJSON(proposal.Payload, `{}`)
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin save pipeline proposal: %w", err)
	}
	defer tx.Rollback()
	// Proposing again re-arms the one record rather than adding a second: the id is
	// content-derived, so a transport retry and a genuine re-proposal are
	// indistinguishable, and both must leave exactly one reviewable offer. Clearing
	// consumed_at matters because an agent that re-proposes something already saved
	// would otherwise get MCP success with nothing on the approval surface, and
	// clearing declined_at matters for the same reason: a decline refuses one
	// offer, never the content, so a later re-proposal is pending again and is not
	// silently held in the declined list (FS-14.R50).
	if _, err := tx.Exec(`
INSERT INTO pipeline_proposals(proposal_id, kind, digest, payload_json, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(proposal_id) DO UPDATE SET consumed_at = '', declined_at = '', created_at = excluded.created_at`,
		proposal.ProposalID, proposal.Kind, proposal.Digest, string(proposal.Payload), formatTime(proposal.CreatedAt)); err != nil {
		return fmt.Errorf("state: save pipeline proposal: %w", err)
	}
	if _, err := tx.Exec(`
DELETE FROM pipeline_proposals WHERE proposal_id NOT IN (
  SELECT proposal_id FROM pipeline_proposals ORDER BY created_at DESC, proposal_id LIMIT ?)`, retain); err != nil {
		return fmt.Errorf("state: prune pipeline proposals: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit pipeline proposal: %w", err)
	}
	return nil
}

// ConsumePipelineProposal retires one pending proposal after the exact mutation
// it proposed committed, so the approval surface stops offering an action that
// already happened. It reports whether this call was the one that consumed the
// record, so a replayed approval publishes no second update.
func (s *Store) ConsumePipelineProposal(proposalID string, at time.Time) (bool, error) {
	if at.IsZero() {
		at = timeNow()
	}
	result, err := s.db.Exec(`
UPDATE pipeline_proposals SET consumed_at = ? WHERE proposal_id = ? AND consumed_at = ''`,
		formatTime(at), proposalID)
	if err != nil {
		return false, fmt.Errorf("state: consume pipeline proposal: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("state: consume pipeline proposal count: %w", err)
	}
	return count == 1, nil
}

// DeclinePipelineProposal claims one pending record for a person's Reject with
// the same conditional-write shape consumption uses: the WHERE names the state
// the caller expects, so two tabs racing the same offer produce exactly one
// decline and the loser is told the state the row is actually in (INV §5,
// TS-02.R29). Declining refuses the offer, never its content, so an approval
// that commits later may still consume this row (FS-14.R57).
func (s *Store) DeclinePipelineProposal(proposalID string, at time.Time) (PipelineProposalRecord, error) {
	if at.IsZero() {
		at = timeNow()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineProposalRecord{}, fmt.Errorf("state: begin decline pipeline proposal: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.Exec(`
UPDATE pipeline_proposals SET declined_at = ?
WHERE proposal_id = ? AND consumed_at = '' AND declined_at = ''`, formatTime(at), proposalID)
	if err != nil {
		return PipelineProposalRecord{}, fmt.Errorf("state: decline pipeline proposal: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return PipelineProposalRecord{}, fmt.Errorf("state: decline pipeline proposal count: %w", err)
	}
	if count != 1 {
		return PipelineProposalRecord{}, proposalClaimLoss(tx, proposalID)
	}
	// Read back inside the same transaction so the record the caller renders is
	// the one this claim produced rather than a later tab's state.
	proposal, err := readPipelineProposal(tx, proposalID)
	if err != nil {
		return PipelineProposalRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return PipelineProposalRecord{}, fmt.Errorf("state: commit decline pipeline proposal: %w", err)
	}
	return proposal, nil
}

// DeletePipelineProposal removes a declined record permanently. Rejecting first
// is what makes the removal deliberate, so the claim requires a decline rather
// than accepting any row (FS-14.R49, TS-02.R29). Delete is a hard row delete and
// leaves no tombstone, so a later identical proposal inserts a new record.
func (s *Store) DeletePipelineProposal(proposalID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin delete pipeline proposal: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.Exec(`DELETE FROM pipeline_proposals WHERE proposal_id = ? AND declined_at != ''`, proposalID)
	if err != nil {
		return fmt.Errorf("state: delete pipeline proposal: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("state: delete pipeline proposal count: %w", err)
	}
	if count != 1 {
		return proposalClaimLoss(tx, proposalID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit delete pipeline proposal: %w", err)
	}
	return nil
}

// proposalClaimLoss turns zero affected rows into the state the row is actually
// in. Zero rows is never success and never a failure to write: it is the loser
// of a race, and the surface needs the real state to explain what happened.
func proposalClaimLoss(tx *sql.Tx, proposalID string) error {
	var consumedAt, declinedAt string
	err := tx.QueryRow(`SELECT consumed_at, declined_at FROM pipeline_proposals WHERE proposal_id = ?`, proposalID).
		Scan(&consumedAt, &declinedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("state: read pipeline proposal state: %w", err)
	}
	switch {
	case consumedAt != "":
		return ErrPipelineProposalConsumed
	case declinedAt != "":
		return ErrPipelineProposalDeclined
	default:
		return ErrPipelineProposalNotDeclined
	}
}

func readPipelineProposal(q pipelineRunQueryer, proposalID string) (PipelineProposalRecord, error) {
	var proposal PipelineProposalRecord
	var payload, createdAt, declinedAt string
	err := q.QueryRow(`
SELECT proposal_id, kind, digest, payload_json, created_at, declined_at
FROM pipeline_proposals WHERE proposal_id = ?`, proposalID).
		Scan(&proposal.ProposalID, &proposal.Kind, &proposal.Digest, &payload, &createdAt, &declinedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineProposalRecord{}, ErrNotFound
	}
	if err != nil {
		return PipelineProposalRecord{}, fmt.Errorf("state: read pipeline proposal: %w", err)
	}
	proposal.Payload = nonEmptyJSON([]byte(payload), `{}`)
	if proposal.CreatedAt, err = parseTime(createdAt); err != nil {
		return PipelineProposalRecord{}, fmt.Errorf("state: read pipeline proposal created_at: %w", err)
	}
	if declinedAt != "" {
		declined, err := parseTime(declinedAt)
		if err != nil {
			return PipelineProposalRecord{}, fmt.Errorf("state: read pipeline proposal declined_at: %w", err)
		}
		proposal.DeclinedAt = &declined
	}
	return proposal, nil
}

// ListPipelineProposals returns every record still on the approval surface:
// pending records and the declined ones a person has not deleted yet. Callers
// split the two by DeclinedAt. A record whose durable columns cannot be read is
// isolated and skipped rather than aborting the whole list, because one corrupt
// row must not hide every approvable proposal (INV §7).
func (s *Store) ListPipelineProposals(limit int) ([]PipelineProposalRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
SELECT proposal_id, kind, digest, payload_json, created_at, declined_at
FROM pipeline_proposals
WHERE consumed_at = ''
ORDER BY created_at DESC, proposal_id
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("state: list pipeline proposals: %w", err)
	}
	defer rows.Close()
	proposals := []PipelineProposalRecord{}
	for rows.Next() {
		var proposal PipelineProposalRecord
		var payload, createdAt, declinedAt string
		if err := rows.Scan(&proposal.ProposalID, &proposal.Kind, &proposal.Digest, &payload, &createdAt, &declinedAt); err != nil {
			return nil, fmt.Errorf("state: scan pipeline proposal: %w", err)
		}
		proposal.Payload = nonEmptyJSON([]byte(payload), `{}`)
		created, timeErr := parseTime(createdAt)
		if timeErr != nil {
			slog.Warn("state: skip pipeline proposal with unreadable created_at", "proposal", proposal.ProposalID, "err", timeErr)
			continue
		}
		proposal.CreatedAt = created
		if declinedAt != "" {
			declined, timeErr := parseTime(declinedAt)
			if timeErr != nil {
				slog.Warn("state: skip pipeline proposal with unreadable declined_at", "proposal", proposal.ProposalID, "err", timeErr)
				continue
			}
			proposal.DeclinedAt = &declined
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate pipeline proposals: %w", err)
	}
	return proposals, nil
}

func (s *Store) ReadPipelineRun(runID string) (PipelineRunRecord, error) {
	return readPipelineRunQuery(s.db, runID)
}

type pipelineRunQueryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

const pipelineRunColumns = `
run_id, template_id, template_snapshot_json, display_name, project, goal,
inputs_json, assignments_json, state, revision, pending_action,
current_stage_id, current_attempt_id, current_agent_id, attention_reason,
final_outcome, created_at, updated_at`

func readPipelineRunQuery(q pipelineRunQueryer, runID string) (PipelineRunRecord, error) {
	run, err := scanPipelineRun(q.QueryRow(`SELECT `+pipelineRunColumns+` FROM pipeline_runs WHERE run_id = ?`, runID))
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineRunRecord{}, ErrNotFound
	}
	return run, err
}

func scanPipelineRun(row rowScanner) (PipelineRunRecord, error) {
	var run PipelineRunRecord
	var snapshot, inputs, assignments, createdAt, updatedAt string
	err := row.Scan(
		&run.RunID, &run.TemplateID, &snapshot, &run.DisplayName, &run.Project, &run.Goal,
		&inputs, &assignments, &run.State, &run.Revision, &run.PendingAction,
		&run.CurrentStageID, &run.CurrentAttemptID, &run.CurrentAgentID, &run.AttentionReason,
		&run.FinalOutcome, &createdAt, &updatedAt,
	)
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: read pipeline run: %w", err)
	}
	run.TemplateSnapshot = nonEmptyJSON([]byte(snapshot), `{}`)
	run.Inputs = nonEmptyJSON([]byte(inputs), `{}`)
	run.Assignments = nonEmptyJSON([]byte(assignments), `{}`)
	if run.CreatedAt, err = parseTime(createdAt); err != nil {
		return PipelineRunRecord{}, wrapTimeErr("pipeline_run.created_at", err)
	}
	if run.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return PipelineRunRecord{}, wrapTimeErr("pipeline_run.updated_at", err)
	}
	return run, nil
}

func (s *Store) ListPipelineRuns(limit, offset int) ([]PipelineRunRecord, error) {
	page, err := s.ListPipelineRunPage(limit, offset)
	if err != nil {
		return nil, err
	}
	return page.Runs, nil
}

// PipelineRunPage is one retained-history page and its exact total from the
// same read transaction. It keeps list pagination independent of full details.
type PipelineRunPage struct {
	Runs  []PipelineRunRecord
	Total int
}

// ListPipelineRunPage returns retained runs newest first with the exact total.
// Count and page share one transaction so a response cannot advertise an end
// state from a different database snapshot than its rows (TS-03.R30).
func (s *Store) ListPipelineRunPage(limit, offset int) (PipelineRunPage, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunPage{}, fmt.Errorf("state: begin pipeline run page: %w", err)
	}
	defer tx.Rollback()
	var total int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM pipeline_runs`).Scan(&total); err != nil {
		return PipelineRunPage{}, fmt.Errorf("state: count pipeline runs: %w", err)
	}
	rows, err := tx.Query(`SELECT `+pipelineRunColumns+`
FROM pipeline_runs ORDER BY updated_at DESC, run_id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return PipelineRunPage{}, fmt.Errorf("state: list pipeline runs: %w", err)
	}
	defer rows.Close()
	out := make([]PipelineRunRecord, 0, limit)
	for rows.Next() {
		run, err := scanPipelineRun(rows)
		if err != nil {
			return PipelineRunPage{}, err
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return PipelineRunPage{}, fmt.Errorf("state: iterate pipeline runs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return PipelineRunPage{}, fmt.Errorf("state: close pipeline runs: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunPage{}, fmt.Errorf("state: commit pipeline run page: %w", err)
	}
	return PipelineRunPage{Runs: out, Total: total}, nil
}

// ListActivePipelineRuns returns every non-terminal run for startup recovery
// and shared-workspace checks. Those correctness paths must not silently stop
// at the first UI page when more than MaxListPage historical rows exist.
func (s *Store) ListActivePipelineRuns() ([]PipelineRunRecord, error) {
	rows, err := s.db.Query(`
SELECT run_id FROM pipeline_runs
WHERE state NOT IN ('completed', 'stopped')
ORDER BY created_at, run_id`)
	if err != nil {
		return nil, fmt.Errorf("state: list active pipeline runs: %w", err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("state: scan active pipeline run: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate active pipeline runs: %w", err)
	}
	out := make([]PipelineRunRecord, 0, len(ids))
	for _, id := range ids {
		run, err := s.ReadPipelineRun(id)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

func (s *Store) UpdatePipelineRunCAS(runID string, expectedRevision int64, update PipelineRunUpdate) (PipelineRunRecord, error) {
	if update.UpdatedAt.IsZero() {
		update.UpdatedAt = timeNow()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: begin update pipeline run: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.Exec(`
UPDATE pipeline_runs SET
  state = ?, revision = revision + 1, pending_action = ?, current_stage_id = ?,
  current_attempt_id = ?, current_agent_id = ?, attention_reason = ?,
  final_outcome = ?, updated_at = ?
WHERE run_id = ? AND revision = ?`,
		update.State, update.PendingAction, update.CurrentStageID, update.CurrentAttemptID,
		update.CurrentAgentID, update.AttentionReason, update.FinalOutcome, formatTime(update.UpdatedAt),
		runID, expectedRevision,
	)
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: update pipeline run: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: pipeline update count: %w", err)
	}
	if count == 0 {
		// This transaction owns the store's single connection, so the check runs
		// on it rather than through the store.
		var exists int
		switch err := tx.QueryRow(`SELECT 1 FROM pipeline_runs WHERE run_id = ?`, runID).Scan(&exists); {
		case errors.Is(err, sql.ErrNoRows):
			return PipelineRunRecord{}, ErrNotFound
		case err != nil:
			return PipelineRunRecord{}, fmt.Errorf("state: read pipeline run after conflict: %w", err)
		}
		return PipelineRunRecord{}, ErrPipelineConflict
	}
	if err := registerPipelineRunOutcomeTx(tx, runID, update.State, update.FinalOutcome, update.UpdatedAt); err != nil {
		return PipelineRunRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: commit update pipeline run: %w", err)
	}
	return s.ReadPipelineRun(runID)
}

// registerPipelineRunOutcomeTx normalizes a run's terminal outcome into the
// shared result layer, in the same transaction that commits that terminal state.
// That is what makes a run usable as a prerequisite, and doing it in the same
// commit is why deleting a run afterwards can never strand an arm: the
// registration is keyed to the source and holds no foreign key into the run
// (FS-14.R34, FS-16.R13, TS-09.R27, TS-10.R21).
//
// A completed run maps its template-defined final outcome onto the shared
// vocabulary — `success` for success and `failure` for every other final label,
// whose raw value is kept as a display detail — and a stopped run is `cancelled`.
func registerPipelineRunOutcomeTx(tx *sql.Tx, runID, runState, finalOutcome string, at time.Time) error {
	outcome := OutcomeCancelled
	switch runState {
	case "completed":
		outcome = OutcomeFailure
		if finalOutcome == "success" {
			outcome = OutcomeSuccess
		}
	case "stopped":
	default:
		return nil
	}
	err := RegisterWorkResultTx(tx, WorkResult{
		SourceKind: SourcePipelineRun, SourceID: runID, Outcome: outcome, RawLabel: finalOutcome,
	}, at)
	// A registration is immutable, so a run that somehow re-commits its terminal
	// state keeps the outcome it already published rather than re-deciding arms.
	if errors.Is(err, ErrWorkResultRecorded) {
		return nil
	}
	return err
}

func (s *Store) DeletePipelineRun(runID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin delete pipeline run: %w", err)
	}
	defer tx.Rollback()
	var runState string
	if err := tx.QueryRow(`SELECT state FROM pipeline_runs WHERE run_id = ?`, runID).Scan(&runState); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("state: read pipeline run for delete: %w", err)
	}
	if runState != "completed" && runState != "stopped" {
		return ErrPipelineActive
	}
	if _, err := tx.Exec(`DELETE FROM pipeline_runs WHERE run_id = ?`, runID); err != nil {
		return fmt.Errorf("state: delete pipeline run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit pipeline delete: %w", err)
	}
	return nil
}

func (s *Store) InsertPipelineAttempt(attempt PipelineAttemptRecord) error {
	if attempt.CreatedAt.IsZero() {
		attempt.CreatedAt = timeNow()
	}
	if attempt.UpdatedAt.IsZero() {
		attempt.UpdatedAt = attempt.CreatedAt
	}
	return insertPipelineAttemptExec(s.db, attempt)
}

type pipelineAttemptExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertPipelineAttemptTx(tx *sql.Tx, attempt PipelineAttemptRecord) error {
	return insertPipelineAttemptExec(tx, attempt)
}

func insertPipelineAttemptExec(exec pipelineAttemptExecer, attempt PipelineAttemptRecord) error {
	attempt.ReportOutputs = nonEmptyJSON(attempt.ReportOutputs, `{}`)
	_, err := exec.Exec(`
INSERT INTO pipeline_attempts(
  attempt_id, run_id, stage_id, attempt_no, visit_no, parent_attempt_id,
  agent_id, agent_generation, backend, model, effort, state, assignment_text,
  assignment_hash, assignment_version, report_outcome, report_summary,
  report_details, report_checks, report_outputs_json, reported_at,
  quiescent_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attempt.AttemptID, attempt.RunID, attempt.StageID, attempt.AttemptNo, attempt.VisitNo, attempt.ParentAttemptID,
		attempt.AgentID, attempt.AgentGeneration, attempt.Backend, attempt.Model, attempt.Effort, attempt.State, attempt.AssignmentText,
		attempt.AssignmentHash, attempt.AssignmentVersion, attempt.ReportOutcome, attempt.ReportSummary,
		attempt.ReportDetails, attempt.ReportChecks, string(attempt.ReportOutputs), nullableTime(attempt.ReportedAt),
		nullableTime(attempt.QuiescentAt), formatTime(attempt.CreatedAt), formatTime(attempt.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("state: insert pipeline attempt: %w", err)
	}
	return nil
}

// CreatePipelineAttemptCAS installs one immutable next attempt and advances the
// run pointer in the same transaction, so a retry/route/control race cannot
// create an orphan or two current attempts.
func (s *Store) CreatePipelineAttemptCAS(runID string, expectedRevision int64, attempt PipelineAttemptRecord, update PipelineRunUpdate) (PipelineRunRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: begin next pipeline attempt: %w", err)
	}
	defer tx.Rollback()
	var revision int64
	if err := tx.QueryRow(`SELECT revision FROM pipeline_runs WHERE run_id = ?`, runID).Scan(&revision); errors.Is(err, sql.ErrNoRows) {
		return PipelineRunRecord{}, ErrNotFound
	} else if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: read pipeline revision: %w", err)
	}
	if revision != expectedRevision {
		return PipelineRunRecord{}, ErrPipelineConflict
	}
	if attempt.RunID == "" {
		attempt.RunID = runID
	}
	if err := insertPipelineAttemptTx(tx, attempt); err != nil {
		return PipelineRunRecord{}, err
	}
	if update.UpdatedAt.IsZero() {
		update.UpdatedAt = timeNow()
	}
	result, err := tx.Exec(`
UPDATE pipeline_runs SET
  state = ?, revision = revision + 1, pending_action = ?, current_stage_id = ?,
  current_attempt_id = ?, current_agent_id = ?, attention_reason = ?,
  final_outcome = ?, updated_at = ?
WHERE run_id = ? AND revision = ?`,
		update.State, update.PendingAction, update.CurrentStageID, update.CurrentAttemptID,
		update.CurrentAgentID, update.AttentionReason, update.FinalOutcome, formatTime(update.UpdatedAt),
		runID, expectedRevision,
	)
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: point pipeline at attempt: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return PipelineRunRecord{}, ErrPipelineConflict
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: commit next pipeline attempt: %w", err)
	}
	return s.ReadPipelineRun(runID)
}

func (s *Store) UpdatePipelineAttemptState(attemptID, stateValue, agentGeneration string, updatedAt time.Time) error {
	if updatedAt.IsZero() {
		updatedAt = timeNow()
	}
	result, err := s.db.Exec(`
UPDATE pipeline_attempts SET state = ?, agent_generation = CASE WHEN ? = '' THEN agent_generation ELSE ? END, updated_at = ?
WHERE attempt_id = ?`, stateValue, agentGeneration, agentGeneration, formatTime(updatedAt), attemptID)
	if err != nil {
		return fmt.Errorf("state: update pipeline attempt: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePipelineAttemptAndRunCAS commits an attempt boundary and its run
// projection together. Lifecycle launch/resume/exit handling uses this instead
// of exposing split state where one side advanced without the other.
func (s *Store) UpdatePipelineAttemptAndRunCAS(runID string, expectedRevision int64, attemptID, attemptState, agentGeneration string, update PipelineRunUpdate) (PipelineRunRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: begin pipeline attempt transition: %w", err)
	}
	defer tx.Rollback()
	var currentAttempt string
	var revision int64
	if err := tx.QueryRow(`SELECT current_attempt_id, revision FROM pipeline_runs WHERE run_id = ?`, runID).Scan(&currentAttempt, &revision); errors.Is(err, sql.ErrNoRows) {
		return PipelineRunRecord{}, ErrNotFound
	} else if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: read pipeline attempt transition: %w", err)
	}
	if revision != expectedRevision || currentAttempt != attemptID {
		return PipelineRunRecord{}, ErrPipelineConflict
	}
	if update.UpdatedAt.IsZero() {
		update.UpdatedAt = timeNow()
	}
	result, err := tx.Exec(`
UPDATE pipeline_attempts SET state = ?, agent_generation = CASE WHEN ? = '' THEN agent_generation ELSE ? END, updated_at = ?
WHERE attempt_id = ? AND run_id = ?`, attemptState, agentGeneration, agentGeneration, formatTime(update.UpdatedAt), attemptID, runID)
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: update pipeline attempt boundary: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return PipelineRunRecord{}, ErrPipelineConflict
	}
	result, err = tx.Exec(`
UPDATE pipeline_runs SET
  state = ?, revision = revision + 1, pending_action = ?, current_stage_id = ?,
  current_attempt_id = ?, current_agent_id = ?, attention_reason = ?,
  final_outcome = ?, updated_at = ?
WHERE run_id = ? AND revision = ?`,
		update.State, update.PendingAction, update.CurrentStageID, update.CurrentAttemptID,
		update.CurrentAgentID, update.AttentionReason, update.FinalOutcome, formatTime(update.UpdatedAt),
		runID, expectedRevision,
	)
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: update pipeline run boundary: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return PipelineRunRecord{}, ErrPipelineConflict
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: commit pipeline attempt transition: %w", err)
	}
	return s.ReadPipelineRun(runID)
}

// AcceptPipelineReport commits the immutable report, output values, and
// await-quiescence intent atomically before the MCP tool returns success.
func (s *Store) AcceptPipelineReport(input PipelineReportInput) (PipelineRunRecord, PipelineAttemptRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, fmt.Errorf("state: begin pipeline report: %w", err)
	}
	defer tx.Rollback()
	run, err := readPipelineRunQuery(tx, input.RunID)
	if err != nil {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, err
	}
	if run.Revision != input.ExpectedRevision || run.CurrentAttemptID != input.AttemptID || run.CurrentAgentID != input.AgentID || run.State == "stopped" || run.State == "completed" {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, ErrPipelineConflict
	}
	attempt, err := readPipelineAttemptQuery(tx, input.AttemptID)
	if err != nil {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, err
	}
	if attempt.AgentID != input.AgentID || attempt.AgentGeneration != input.AgentGeneration || attempt.ReportOutcome != "" || attempt.State != "running" {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, ErrPipelineConflict
	}
	when := input.ReportedAt
	if when.IsZero() {
		when = timeNow()
	}
	outputs := map[string]string{}
	for _, value := range input.Outputs {
		outputs[value.Name] = value.Value
	}
	outputsJSON, err := json.Marshal(outputs)
	if err != nil {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, fmt.Errorf("state: marshal pipeline outputs: %w", err)
	}
	result, err := tx.Exec(`
UPDATE pipeline_attempts SET state = 'reported', report_outcome = ?, report_summary = ?,
  report_details = ?, report_checks = ?, report_outputs_json = ?, reported_at = ?, updated_at = ?
WHERE attempt_id = ? AND report_outcome = ''`,
		input.Outcome, input.Summary, input.Details, input.Checks, string(outputsJSON), formatTime(when), formatTime(when), input.AttemptID)
	if err != nil {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, fmt.Errorf("state: write pipeline report: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, ErrPipelineConflict
	}
	for _, value := range input.Outputs {
		if _, err := tx.Exec(`
INSERT INTO pipeline_values(run_id, name, value, source_kind, source_attempt_id, updated_at)
VALUES (?, ?, ?, 'stage_output', ?, ?)
ON CONFLICT(run_id, name) DO UPDATE SET
  value = excluded.value, source_kind = excluded.source_kind,
  source_attempt_id = excluded.source_attempt_id, updated_at = excluded.updated_at`,
			input.RunID, value.Name, value.Value, input.AttemptID, formatTime(when)); err != nil {
			return PipelineRunRecord{}, PipelineAttemptRecord{}, fmt.Errorf("state: write pipeline output value: %w", err)
		}
	}
	result, err = tx.Exec(`
UPDATE pipeline_runs SET revision = revision + 1, pending_action = 'await_quiescence',
  attention_reason = '', updated_at = ? WHERE run_id = ? AND revision = ?`,
		formatTime(when), input.RunID, input.ExpectedRevision)
	if err != nil {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, fmt.Errorf("state: await pipeline quiescence: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, ErrPipelineConflict
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, fmt.Errorf("state: commit pipeline report: %w", err)
	}
	updatedRun, err := s.ReadPipelineRun(input.RunID)
	if err != nil {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, err
	}
	updatedAttempt, err := s.ReadPipelineAttempt(input.AttemptID)
	return updatedRun, updatedAttempt, err
}

// MarkPipelineQuiescent consumes the normalized turn boundary exactly once.
// Blocked pauses with the same idle agent; success/failure records stop intent.
func (s *Store) MarkPipelineQuiescent(runID, attemptID, agentID, generation string, expectedRevision int64, at time.Time) (PipelineRunRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: begin pipeline quiescence: %w", err)
	}
	defer tx.Rollback()
	run, err := readPipelineRunQuery(tx, runID)
	if err != nil {
		return PipelineRunRecord{}, err
	}
	if run.Revision != expectedRevision || run.PendingAction != "await_quiescence" || run.CurrentAttemptID != attemptID || run.CurrentAgentID != agentID {
		return PipelineRunRecord{}, ErrPipelineConflict
	}
	attempt, err := readPipelineAttemptQuery(tx, attemptID)
	if err != nil {
		return PipelineRunRecord{}, err
	}
	if attempt.AgentGeneration != generation || attempt.ReportOutcome == "" || attempt.QuiescentAt != nil {
		return PipelineRunRecord{}, ErrPipelineConflict
	}
	if at.IsZero() {
		at = timeNow()
	}
	if _, err := tx.Exec(`UPDATE pipeline_attempts SET state = 'completed', quiescent_at = ?, updated_at = ? WHERE attempt_id = ?`, formatTime(at), formatTime(at), attemptID); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: complete pipeline attempt: %w", err)
	}
	nextState, nextAction, reason := run.State, "stop_agent", ""
	if attempt.ReportOutcome == "blocked" {
		nextState, nextAction, reason = "paused", "", "blocked"
	}
	result, err := tx.Exec(`
UPDATE pipeline_runs SET state = ?, revision = revision + 1, pending_action = ?, attention_reason = ?, updated_at = ?
WHERE run_id = ? AND revision = ?`, nextState, nextAction, reason, formatTime(at), runID, expectedRevision)
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: cross pipeline quiescence: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return PipelineRunRecord{}, ErrPipelineConflict
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: commit pipeline quiescence: %w", err)
	}
	return s.ReadPipelineRun(runID)
}

func (s *Store) ReadPipelineAttempt(attemptID string) (PipelineAttemptRecord, error) {
	return readPipelineAttemptQuery(s.db, attemptID)
}

func readPipelineAttemptQuery(q pipelineRunQueryer, attemptID string) (PipelineAttemptRecord, error) {
	row := q.QueryRow(`
SELECT attempt_id, run_id, stage_id, attempt_no, visit_no, COALESCE(parent_attempt_id, ''),
       agent_id, agent_generation, backend, model, effort, state, assignment_text,
       assignment_hash, assignment_version, report_outcome, report_summary,
       report_details, report_checks, report_outputs_json, reported_at,
       quiescent_at, created_at, updated_at
FROM pipeline_attempts WHERE attempt_id = ?`, attemptID)
	return scanPipelineAttempt(row)
}

// CurrentPipelineAttemptForAgent resolves server-derived MCP/runtime identity to
// the one current durable attempt. Old/non-current associations are rejected.
func (s *Store) CurrentPipelineAttemptForAgent(agentID string) (PipelineRunRecord, PipelineAttemptRecord, error) {
	var runID, attemptID string
	err := s.db.QueryRow(`
SELECT r.run_id, r.current_attempt_id
FROM pipeline_runs r
JOIN pipeline_attempts a ON a.attempt_id = r.current_attempt_id
WHERE r.current_agent_id = ? AND a.agent_id = ? AND r.state NOT IN ('completed', 'stopped')
ORDER BY r.updated_at DESC LIMIT 1`, agentID, agentID).Scan(&runID, &attemptID)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, ErrNotFound
	}
	if err != nil {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, fmt.Errorf("state: resolve current pipeline agent: %w", err)
	}
	run, err := s.ReadPipelineRun(runID)
	if err != nil {
		return PipelineRunRecord{}, PipelineAttemptRecord{}, err
	}
	attempt, err := s.ReadPipelineAttempt(attemptID)
	return run, attempt, err
}

func (s *Store) PipelineAssociationForAgent(agentID string) (*PipelineAssociation, error) {
	var association PipelineAssociation
	err := s.db.QueryRow(`
SELECT a.run_id, r.display_name, a.stage_id, a.attempt_id, a.attempt_no
FROM pipeline_attempts a
JOIN pipeline_runs r ON r.run_id = a.run_id
WHERE a.agent_id = ?
ORDER BY a.created_at DESC, a.attempt_no DESC LIMIT 1`, agentID).Scan(
		&association.RunID, &association.RunName, &association.StageID, &association.AttemptID, &association.AttemptNo,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state: read pipeline association: %w", err)
	}
	return &association, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPipelineAttempt(row rowScanner) (PipelineAttemptRecord, error) {
	var attempt PipelineAttemptRecord
	var outputs, createdAt, updatedAt string
	var reportedAt, quiescentAt sql.NullString
	err := row.Scan(
		&attempt.AttemptID, &attempt.RunID, &attempt.StageID, &attempt.AttemptNo, &attempt.VisitNo, &attempt.ParentAttemptID,
		&attempt.AgentID, &attempt.AgentGeneration, &attempt.Backend, &attempt.Model, &attempt.Effort, &attempt.State, &attempt.AssignmentText,
		&attempt.AssignmentHash, &attempt.AssignmentVersion, &attempt.ReportOutcome, &attempt.ReportSummary,
		&attempt.ReportDetails, &attempt.ReportChecks, &outputs, &reportedAt, &quiescentAt, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineAttemptRecord{}, ErrNotFound
	}
	if err != nil {
		return PipelineAttemptRecord{}, fmt.Errorf("state: scan pipeline attempt: %w", err)
	}
	attempt.ReportOutputs = nonEmptyJSON([]byte(outputs), `{}`)
	if attempt.CreatedAt, err = parseTime(createdAt); err != nil {
		return PipelineAttemptRecord{}, wrapTimeErr("pipeline_attempt.created_at", err)
	}
	if attempt.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return PipelineAttemptRecord{}, wrapTimeErr("pipeline_attempt.updated_at", err)
	}
	if reportedAt.Valid {
		parsed, parseErr := parseTime(reportedAt.String)
		if parseErr != nil {
			return PipelineAttemptRecord{}, wrapTimeErr("pipeline_attempt.reported_at", parseErr)
		}
		attempt.ReportedAt = &parsed
	}
	if quiescentAt.Valid {
		parsed, parseErr := parseTime(quiescentAt.String)
		if parseErr != nil {
			return PipelineAttemptRecord{}, wrapTimeErr("pipeline_attempt.quiescent_at", parseErr)
		}
		attempt.QuiescentAt = &parsed
	}
	return attempt, nil
}

func (s *Store) ListPipelineAttempts(runID string) ([]PipelineAttemptRecord, error) {
	rows, err := s.db.Query(`
SELECT attempt_id, run_id, stage_id, attempt_no, visit_no, COALESCE(parent_attempt_id, ''),
       agent_id, agent_generation, backend, model, effort, state, assignment_text,
       assignment_hash, assignment_version, report_outcome, report_summary,
       report_details, report_checks, report_outputs_json, reported_at,
       quiescent_at, created_at, updated_at
FROM pipeline_attempts WHERE run_id = ? ORDER BY attempt_no`, runID)
	if err != nil {
		return nil, fmt.Errorf("state: list pipeline attempts: %w", err)
	}
	defer rows.Close()
	out := []PipelineAttemptRecord{}
	for rows.Next() {
		attempt, err := scanPipelineAttempt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate pipeline attempts: %w", err)
	}
	return out, nil
}

func (s *Store) ListPipelineValues(runID string) ([]PipelineValueRecord, error) {
	rows, err := s.db.Query(`
SELECT run_id, name, value, source_kind, source_attempt_id, updated_at
FROM pipeline_values WHERE run_id = ? ORDER BY name`, runID)
	if err != nil {
		return nil, fmt.Errorf("state: list pipeline values: %w", err)
	}
	defer rows.Close()
	out := []PipelineValueRecord{}
	for rows.Next() {
		var value PipelineValueRecord
		var updatedAt string
		if err := rows.Scan(&value.RunID, &value.Name, &value.Value, &value.SourceKind, &value.SourceAttemptID, &updatedAt); err != nil {
			return nil, fmt.Errorf("state: scan pipeline value: %w", err)
		}
		value.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, wrapTimeErr("pipeline_value.updated_at", err)
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate pipeline values: %w", err)
	}
	return out, nil
}

func nonEmptyJSON(value []byte, fallback string) []byte {
	if strings.TrimSpace(string(value)) == "" || strings.TrimSpace(string(value)) == "null" {
		return []byte(fallback)
	}
	return value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}
