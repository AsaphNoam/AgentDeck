package state

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
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

func (s *Store) ReadPipelineRun(runID string) (PipelineRunRecord, error) {
	return readPipelineRunQuery(s.db, runID)
}

type pipelineRunQueryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

func readPipelineRunQuery(q pipelineRunQueryer, runID string) (PipelineRunRecord, error) {
	var run PipelineRunRecord
	var snapshot, inputs, assignments, createdAt, updatedAt string
	err := q.QueryRow(`
SELECT run_id, template_id, template_snapshot_json, display_name, project, goal,
       inputs_json, assignments_json, state, revision, pending_action,
       current_stage_id, current_attempt_id, current_agent_id, attention_reason,
       final_outcome, created_at, updated_at
FROM pipeline_runs WHERE run_id = ?`, runID).Scan(
		&run.RunID, &run.TemplateID, &snapshot, &run.DisplayName, &run.Project, &run.Goal,
		&inputs, &assignments, &run.State, &run.Revision, &run.PendingAction,
		&run.CurrentStageID, &run.CurrentAttemptID, &run.CurrentAgentID, &run.AttentionReason,
		&run.FinalOutcome, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineRunRecord{}, ErrNotFound
	}
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
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(`
SELECT run_id FROM pipeline_runs ORDER BY updated_at DESC, run_id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("state: list pipeline runs: %w", err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("state: scan pipeline run: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate pipeline runs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("state: close pipeline runs: %w", err)
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
	result, err := s.db.Exec(`
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
		if _, err := s.ReadPipelineRun(runID); errors.Is(err, ErrNotFound) {
			return PipelineRunRecord{}, ErrNotFound
		}
		return PipelineRunRecord{}, ErrPipelineConflict
	}
	return s.ReadPipelineRun(runID)
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
	attempt.ReportOutputs = nonEmptyJSON(attempt.ReportOutputs, `{}`)
	_, err := s.db.Exec(`
INSERT INTO pipeline_attempts(
  attempt_id, run_id, stage_id, attempt_no, visit_no, parent_attempt_id,
  agent_id, agent_generation, backend, model, state, assignment_text,
  assignment_hash, assignment_version, report_outcome, report_summary,
  report_details, report_checks, report_outputs_json, reported_at,
  quiescent_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attempt.AttemptID, attempt.RunID, attempt.StageID, attempt.AttemptNo, attempt.VisitNo, attempt.ParentAttemptID,
		attempt.AgentID, attempt.AgentGeneration, attempt.Backend, attempt.Model, attempt.State, attempt.AssignmentText,
		attempt.AssignmentHash, attempt.AssignmentVersion, attempt.ReportOutcome, attempt.ReportSummary,
		attempt.ReportDetails, attempt.ReportChecks, string(attempt.ReportOutputs), nullableTime(attempt.ReportedAt),
		nullableTime(attempt.QuiescentAt), formatTime(attempt.CreatedAt), formatTime(attempt.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("state: insert pipeline attempt: %w", err)
	}
	return nil
}

func (s *Store) ReadPipelineAttempt(attemptID string) (PipelineAttemptRecord, error) {
	row := s.db.QueryRow(`
SELECT attempt_id, run_id, stage_id, attempt_no, visit_no, COALESCE(parent_attempt_id, ''),
       agent_id, agent_generation, backend, model, state, assignment_text,
       assignment_hash, assignment_version, report_outcome, report_summary,
       report_details, report_checks, report_outputs_json, reported_at,
       quiescent_at, created_at, updated_at
FROM pipeline_attempts WHERE attempt_id = ?`, attemptID)
	return scanPipelineAttempt(row)
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
		&attempt.AgentID, &attempt.AgentGeneration, &attempt.Backend, &attempt.Model, &attempt.State, &attempt.AssignmentText,
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
       agent_id, agent_generation, backend, model, state, assignment_text,
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
