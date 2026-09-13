package state

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrPipelineStageConflict means a run cursor, stage association, or standing
// task changed while a control was in flight. Callers must hydrate and retry
// through the pipeline controller; generic task mutation must not bypass it.
var ErrPipelineStageConflict = errors.New("state: pipeline stage conflict")

// CreatePipelineStageTaskParams contains the server-composed task and its
// provenance. The task's pipeline fields are deliberately not caller-owned.
type CreatePipelineStageTaskParams struct {
	RunID            string
	ExpectedRevision int64
	StageIndex       int
	AttemptNumber    int
	StageID          string
	Task             Task
	AssignmentDigest string
	ParentTaskID     string
	// OutputValues maps task-result local names to frozen pipeline value keys.
	OutputValues map[string]string
}

// PipelineStageTaskForAssignee resolves only a live standing-owner assignment;
// coordinators and descendants cannot be mistaken for the stage authority.
func (s *Store) PipelineStageTaskForAssignee(agentID, generation string) (PipelineStageTask, Task, error) {
	stage, err := scanPipelineStageTask(s.db.QueryRow(`
SELECT p.run_id, p.stage_index, p.attempt_number, p.stage_id, p.task_id, p.standing_agent_id, p.coordinator_task_id, p.assignment_digest, p.state, p.closure_revision, p.created_at, COALESCE(p.closed_at, '')
FROM pipeline_stage_tasks p JOIN tasks t ON t.task_id = p.task_id
WHERE t.assigned_agent_id = ? AND t.assigned_generation = ?`, agentID, generation))
	if err != nil {
		return PipelineStageTask{}, Task{}, err
	}
	task, err := s.ReadTask(stage.TaskID)
	return stage, task, err
}

// AcceptPipelineStageTaskResult is the stage branch of task result acceptance.
// It commits the immutable task result, named outputs, closure fence, and run
// projection in one transaction; no parallel pipeline report exists.
func (s *Store) AcceptPipelineStageTaskResult(taskID, agentID, generation string, expectedRunRevision int64, result TaskResult) (PipelineRunRecord, error) {
	if err := ValidateAgentReport(result.Outcome, result.Summary, result.Details, ""); err != nil {
		return PipelineRunRecord{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: begin accept pipeline task result: %w", err)
	}
	defer tx.Rollback()
	var runID, stageState, taskState, assignedID, assignedGeneration, outputJSON string
	var revision int64
	err = tx.QueryRow(`
SELECT p.run_id, p.state, r.revision, t.state, COALESCE(t.assigned_agent_id, ''), t.assigned_generation, p.output_values_json
FROM pipeline_stage_tasks p JOIN pipeline_runs r ON r.run_id = p.run_id JOIN tasks t ON t.task_id = p.task_id
WHERE p.task_id = ?`, taskID).Scan(&runID, &stageState, &revision, &taskState, &assignedID, &assignedGeneration, &outputJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineRunRecord{}, ErrNotFound
	}
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: read stage result authority: %w", err)
	}
	if revision != expectedRunRevision || stageState != "open" || !OwnsReportedWork(agentID, generation, assignedID, assignedGeneration) || (taskState != TaskStarting && taskState != TaskRunning) {
		return PipelineRunRecord{}, ErrPipelineStageConflict
	}
	declared := map[string]string{}
	if err := json.Unmarshal([]byte(outputJSON), &declared); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: decode stage output contract: %w", err)
	}
	for name := range result.Outputs {
		if _, ok := declared[name]; !ok {
			return PipelineRunRecord{}, ErrPipelineStageConflict
		}
	}
	if result.Outcome == OutcomeSuccess {
		for name := range declared {
			if result.Outputs[name] == "" {
				return PipelineRunRecord{}, ErrPipelineStageConflict
			}
		}
	}
	now := timeNow()
	stamp := formatTime(now)
	if _, err := tx.Exec(`UPDATE tasks SET state = ?, outcome = ?, outcome_source = 'agent', outcome_summary = ?, outcome_details = ?, attention_reason = '', pending_release = 1, finished_at = ?, revision = revision + 1, updated_at = ? WHERE task_id = ? AND state = ?`, TaskFinished, result.Outcome, result.Summary, result.Details, stamp, stamp, taskID, taskState); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: finish stage task: %w", err)
	}
	if err := RegisterWorkResultTx(tx, WorkResult{SourceKind: SourceTask, SourceID: taskID, Outcome: result.Outcome, Summary: result.Summary}, now); err != nil {
		return PipelineRunRecord{}, err
	}
	if err := insertTaskResultOutputs(tx, taskID, result.Outputs); err != nil {
		return PipelineRunRecord{}, err
	}
	for name, valueKey := range declared {
		if value, ok := result.Outputs[name]; ok {
			if _, err := tx.Exec(`INSERT INTO pipeline_values(run_id, name, value, source_kind, source_attempt_id, updated_at) VALUES (?, ?, ?, 'stage_task', ?, ?) ON CONFLICT(run_id, name) DO UPDATE SET value = excluded.value, source_kind = excluded.source_kind, source_attempt_id = excluded.source_attempt_id, updated_at = excluded.updated_at`, runID, valueKey, value, taskID, stamp); err != nil {
				return PipelineRunRecord{}, fmt.Errorf("state: store stage output: %w", err)
			}
		}
	}
	if _, err := tx.Exec(`UPDATE pipeline_stage_tasks SET state = 'closing', closure_revision = ?, closed_at = ? WHERE task_id = ? AND state = 'open'`, revision+1, stamp, taskID); err != nil {
		return PipelineRunRecord{}, err
	}
	attention := ""
	if result.Outcome == OutcomeFailure || result.Outcome == OutcomeBlocked {
		attention = result.Outcome
	}
	if _, err := tx.Exec(`UPDATE pipeline_runs SET state = 'finishing', pending_action = 'release_stage_task', attention_reason = ?, revision = revision + 1, updated_at = ? WHERE run_id = ? AND revision = ?`, attention, stamp, runID, revision); err != nil {
		return PipelineRunRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: commit pipeline task result: %w", err)
	}
	return s.ReadPipelineRun(runID)
}

// CreatePipelineStageTask creates exactly one standing-owner task for the
// current cursor. The unique association is the durable idempotency fence: a
// replay can read the existing row but cannot create a second task.
func (s *Store) CreatePipelineStageTask(p CreatePipelineStageTaskParams) (PipelineStageTask, Task, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: begin create pipeline stage task: %w", err)
	}
	defer tx.Rollback()
	var revision int64
	var stateName, currentStage string
	err = tx.QueryRow(`SELECT revision, state, current_stage_id FROM pipeline_runs WHERE run_id = ?`, p.RunID).Scan(&revision, &stateName, &currentStage)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineStageTask{}, Task{}, false, ErrNotFound
	}
	if err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: read pipeline run: %w", err)
	}
	if revision != p.ExpectedRevision || currentStage != p.StageID || stateName == "stopping" || stateName == "stopped" || stateName == "completed" {
		return PipelineStageTask{}, Task{}, false, ErrPipelineStageConflict
	}
	if existing, err := readPipelineStageTaskTx(tx, p.RunID, p.StageIndex, p.AttemptNumber); err == nil {
		task, readErr := scanTask(tx.QueryRow(taskSelect+` WHERE task_id = ?`, existing.TaskID))
		if readErr != nil {
			return PipelineStageTask{}, Task{}, false, readErr
		}
		if err := tx.Commit(); err != nil {
			return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: commit pipeline stage replay: %w", err)
		}
		return existing, task, true, nil
	} else if !errors.Is(err, ErrNotFound) {
		return PipelineStageTask{}, Task{}, false, err
	}

	now := timeNow()
	p.Task.CreatedAt, p.Task.UpdatedAt, p.Task.Revision = now, now, 1
	p.Task.State, p.Task.Arms = TaskReady, nil
	p.Task.ReadyAt = &now
	if _, err := tx.Exec(`
INSERT INTO tasks(task_id, project, display_name, instruction, target_kind, target_agent_id, role, backend, model, effort, fast, state, attention_reason, created_by_kind, created_by_agent_id, created_by_generation, revision, ready_at, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?)`,
		p.Task.TaskID, p.Task.Project, p.Task.DisplayName, p.Task.Instruction, p.Task.TargetKind, p.Task.TargetAgentID,
		p.Task.Role, p.Task.Backend, p.Task.Model, p.Task.Effort, p.Task.Fast, p.Task.State,
		p.Task.CreatedByKind, p.Task.CreatedByAgentID, p.Task.CreatedByGeneration, p.Task.Revision,
		formatTime(now), formatTime(now), formatTime(now)); err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: insert pipeline stage task: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO task_lineage(task_id, parent_task_id, pipeline_run_id, pipeline_stage_id, creation_attempt_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		p.Task.TaskID, p.ParentTaskID, p.RunID, p.StageID, fmt.Sprintf("%d", p.AttemptNumber), formatTime(now)); err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: insert pipeline task lineage: %w", err)
	}
	outputJSON, err := json.Marshal(p.OutputValues)
	if err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: encode stage output contract: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO pipeline_stage_tasks(run_id, stage_index, attempt_number, stage_id, task_id, assignment_digest, output_values_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.RunID, p.StageIndex, p.AttemptNumber, p.StageID, p.Task.TaskID, p.AssignmentDigest, string(outputJSON), formatTime(now)); err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: insert pipeline stage association: %w", err)
	}
	if _, err := tx.Exec(`UPDATE pipeline_runs SET pending_action = 'dispatch_stage_task', revision = revision + 1, updated_at = ? WHERE run_id = ? AND revision = ?`, formatTime(now), p.RunID, revision); err != nil {
		return PipelineStageTask{}, Task{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: commit create pipeline stage task: %w", err)
	}
	stage, err := s.ReadPipelineStageTaskByTask(p.Task.TaskID)
	if err != nil {
		return PipelineStageTask{}, Task{}, false, err
	}
	task, err := s.ReadTask(p.Task.TaskID)
	return stage, task, false, err
}

func (s *Store) ReadPipelineStageTaskByTask(taskID string) (PipelineStageTask, error) {
	return scanPipelineStageTask(s.db.QueryRow(`SELECT run_id, stage_index, attempt_number, stage_id, task_id, standing_agent_id, coordinator_task_id, assignment_digest, state, closure_revision, created_at, COALESCE(closed_at, '') FROM pipeline_stage_tasks WHERE task_id = ?`, taskID))
}

func (s *Store) ListPipelineStageTasks(runID string) ([]PipelineStageTask, error) {
	rows, err := s.db.Query(`SELECT run_id, stage_index, attempt_number, stage_id, task_id, standing_agent_id, coordinator_task_id, assignment_digest, state, closure_revision, created_at, COALESCE(closed_at, '') FROM pipeline_stage_tasks WHERE run_id = ? ORDER BY stage_index, attempt_number`, runID)
	if err != nil {
		return nil, fmt.Errorf("state: list pipeline stage tasks: %w", err)
	}
	defer rows.Close()
	out := []PipelineStageTask{}
	for rows.Next() {
		v, err := scanPipelineStageTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate pipeline stage tasks: %w", err)
	}
	return out, nil
}

func readPipelineStageTaskTx(tx *sql.Tx, runID string, stageIndex, attempt int) (PipelineStageTask, error) {
	return scanPipelineStageTask(tx.QueryRow(`SELECT run_id, stage_index, attempt_number, stage_id, task_id, standing_agent_id, coordinator_task_id, assignment_digest, state, closure_revision, created_at, COALESCE(closed_at, '') FROM pipeline_stage_tasks WHERE run_id = ? AND stage_index = ? AND attempt_number = ?`, runID, stageIndex, attempt))
}

func scanPipelineStageTask(row interface{ Scan(...any) error }) (PipelineStageTask, error) {
	var v PipelineStageTask
	var created, closed string
	if err := row.Scan(&v.RunID, &v.StageIndex, &v.AttemptNumber, &v.StageID, &v.TaskID, &v.StandingAgentID, &v.CoordinatorTaskID, &v.AssignmentDigest, &v.State, &v.ClosureRevision, &created, &closed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PipelineStageTask{}, ErrNotFound
		}
		return PipelineStageTask{}, fmt.Errorf("state: scan pipeline stage task: %w", err)
	}
	var err error
	if v.CreatedAt, err = parseTime(created); err != nil {
		return PipelineStageTask{}, err
	}
	if closed != "" {
		if t, err := parseTime(closed); err != nil {
			return PipelineStageTask{}, err
		} else {
			v.ClosedAt = &t
		}
	}
	return v, nil
}

// BindPipelineStageTaskStandingAgent records the dispatcher-confirmed identity.
// A non-empty different identity is never silently substituted.
func (s *Store) BindPipelineStageTaskStandingAgent(taskID, agentID string) error {
	res, err := s.db.Exec(`UPDATE pipeline_stage_tasks SET standing_agent_id = ? WHERE task_id = ? AND state = 'open' AND (standing_agent_id = '' OR standing_agent_id = ?)`, agentID, taskID, agentID)
	if err != nil {
		return fmt.Errorf("state: bind pipeline standing agent: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrPipelineStageConflict
	}
	return nil
}

// ClosePipelineStageTask fences descendant creation before release/cleanup.
func (s *Store) ClosePipelineStageTask(taskID string, closureRevision int64) error {
	res, err := s.db.Exec(`UPDATE pipeline_stage_tasks SET state = 'closing', closure_revision = ?, closed_at = ? WHERE task_id = ? AND state = 'open'`, closureRevision, formatTime(timeNow()), taskID)
	if err != nil {
		return fmt.Errorf("state: close pipeline stage task: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrPipelineStageConflict
	}
	return nil
}

// RetryInterruptedPipelineStageTask returns the current interrupted assignment
// to ordinary task admission while advancing the governing run in the same
// transaction. The confirmed standing agent id remains on the task, so the
// dispatcher resumes that identity rather than minting a replacement.
func (s *Store) RetryInterruptedPipelineStageTask(runID, taskID string, expectedRevision int64) (PipelineRunRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, err
	}
	defer tx.Rollback()
	now := formatTime(timeNow())
	res, err := tx.Exec(`UPDATE tasks SET state = ?, attention_reason = '', start_attempt_count = 0, ready_at = ?, revision = revision + 1, updated_at = ? WHERE task_id = ? AND state = ? AND EXISTS (SELECT 1 FROM pipeline_stage_tasks p WHERE p.task_id = tasks.task_id AND p.run_id = ? AND p.state = 'open')`, TaskReady, now, now, taskID, TaskInterrupted, runID)
	if err != nil {
		return PipelineRunRecord{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return PipelineRunRecord{}, ErrPipelineStageConflict
	}
	res, err = tx.Exec(`UPDATE pipeline_runs SET state = 'queued', pending_action = 'dispatch_stage_task', attention_reason = '', revision = revision + 1, updated_at = ? WHERE run_id = ? AND revision = ? AND state = 'paused'`, now, runID, expectedRevision)
	if err != nil {
		return PipelineRunRecord{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return PipelineRunRecord{}, ErrPipelineStageConflict
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, err
	}
	return s.ReadPipelineRun(runID)
}

// ReadTaskLineage reads durable provenance without exposing task detail.
func (s *Store) ReadTaskLineage(taskID string) (TaskLineage, error) {
	var v TaskLineage
	var created string
	err := s.db.QueryRow(`SELECT task_id, parent_task_id, pipeline_run_id, pipeline_stage_id, creation_attempt_id, created_at FROM task_lineage WHERE task_id = ?`, taskID).Scan(&v.TaskID, &v.ParentTaskID, &v.PipelineRunID, &v.PipelineStageID, &v.CreationAttemptID, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return TaskLineage{}, ErrNotFound
	}
	if err != nil {
		return TaskLineage{}, fmt.Errorf("state: read task lineage: %w", err)
	}
	v.CreatedAt, err = parseTime(created)
	return v, err
}
