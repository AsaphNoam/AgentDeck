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
	Coordinator  *Task
}

const pipelineStageTaskColumns = `p.run_id, p.stage_index, p.attempt_number, p.stage_id, p.task_id, p.standing_agent_id, p.coordinator_task_id, p.assignment_digest, p.state, p.closure_revision, p.created_at, COALESCE(p.closed_at, '')`

// latestStageOrder is the run cursor's definition: the newest attempt of the
// newest stage. The per-stage partial unique index names the open attempt of one
// stage, not the run's latest cursor, so ordering is what makes these reads
// deterministic (TS-09.R40, INV §2/§8).
const latestStageOrder = ` ORDER BY p.stage_index DESC, p.attempt_number DESC LIMIT 1`

// PipelineStageTaskForAssignee resolves only a live standing-owner assignment;
// coordinators and descendants cannot be mistaken for the stage authority.
//
// Two successive stage tasks can share an agent and generation on a borrowed
// runtime, so this is ordered rather than left to return whichever row the
// engine reached first: live attention and reporting must land on the current
// cursor, not on an already-closed predecessor.
func (s *Store) PipelineStageTaskForAssignee(agentID, generation string) (PipelineStageTask, Task, error) {
	stage, err := scanPipelineStageTask(s.db.QueryRow(`
SELECT `+pipelineStageTaskColumns+`
FROM pipeline_stage_tasks p JOIN tasks t ON t.task_id = p.task_id
WHERE t.assigned_agent_id = ? AND t.assigned_generation = ?`+latestStageOrder, agentID, generation))
	if err != nil {
		return PipelineStageTask{}, Task{}, err
	}
	task, err := s.ReadTask(stage.TaskID)
	return stage, task, err
}

// LatestPipelineStageTask resolves a run's current cursor in one bounded query.
// Control and projection paths listed every stage attempt to take the last one,
// which grows with the run and repeats the cursor definition at each site.
//
// The cursor is deliberately not filtered to open stages: acceptance marks the
// stage `closing` while turn-end release and progression still act on it
// (TS-09.R40/R41).
func (s *Store) LatestPipelineStageTask(runID string) (PipelineStageTask, error) {
	return scanPipelineStageTask(s.db.QueryRow(`
SELECT `+pipelineStageTaskColumns+` FROM pipeline_stage_tasks p WHERE p.run_id = ?`+latestStageOrder, runID))
}

// rowQuerier is satisfied by both *sql.DB and *sql.Tx, so one authority
// predicate serves a plain read and a mutation that must decide inside its own
// transaction.
type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

// AgentManagesPipelineWork reports whether agentID may read, watch and manage
// taskID because it is the confirmed standing owner of that run's current stage,
// not because it created the task.
//
// Creation provenance is immutable and deliberately not rewritten: a dedicated
// coordinator is created by the host, and a replacement standing owner inherits
// retained work it never created. Deriving authority from the live stage binding
// instead is what lets a standing owner perform the delegation/wait workflow the
// run requires of it, while an unrelated caller still matches nothing
// (TS-09.R37/R39/R41/R49, FS-14.R73).
//
// Stage tasks themselves are excluded: those mutate only through the pipeline
// controller's own transactions (TS-09.R41).
func (s *Store) AgentManagesPipelineWork(agentID, taskID string) (bool, error) {
	return agentManagesPipelineWorkTx(s.db, agentID, taskID)
}

func agentManagesPipelineWorkTx(q rowQuerier, agentID, taskID string) (bool, error) {
	if agentID == "" || taskID == "" {
		return false, nil
	}
	var one int
	err := q.QueryRow(`
SELECT 1
FROM task_lineage l
JOIN pipeline_stage_tasks p ON p.run_id = l.pipeline_run_id AND p.state = 'open'
WHERE l.task_id = ? AND p.standing_agent_id = ?
  AND NOT EXISTS (SELECT 1 FROM pipeline_stage_tasks q WHERE q.task_id = l.task_id)
LIMIT 1`, taskID, agentID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("state: read managed pipeline work authority: %w", err)
	}
	return true, nil
}

// AcceptedPipelineStageTaskReport resolves a caller's own accepted stage result
// during its share window: the task committed its immutable result and its
// release has not completed yet, which is exactly "accepted report through the
// matching reporting turn-end" (TS-09.R48, FS-15.R4). Acceptance itself moves
// the stage to `closing`, so stage state is deliberately not a filter here; the
// still-standing release intent is what bounds the window.
func (s *Store) AcceptedPipelineStageTaskReport(agentID string) (PipelineStageTask, Task, error) {
	stage, err := scanPipelineStageTask(s.db.QueryRow(`
SELECT p.run_id, p.stage_index, p.attempt_number, p.stage_id, p.task_id, p.standing_agent_id, p.coordinator_task_id, p.assignment_digest, p.state, p.closure_revision, p.created_at, COALESCE(p.closed_at, '')
FROM pipeline_stage_tasks p JOIN tasks t ON t.task_id = p.task_id
WHERE t.assigned_agent_id = ? AND t.state = ? AND t.outcome <> '' AND t.pending_release = 1
ORDER BY p.stage_index DESC, p.attempt_number DESC
LIMIT 1`, agentID, TaskFinished))
	if err != nil {
		return PipelineStageTask{}, Task{}, err
	}
	task, err := s.ReadTask(stage.TaskID)
	return stage, task, err
}

// AcceptPipelineStageTaskResult is the stage branch of task result acceptance.
// It commits the immutable task result, named outputs, closure fence, and run
// projection in one transaction; no parallel pipeline report exists.
//
// The caller's execution handle is required and matched inside this transaction
// (TS-09.R40, TS-10.R28/R31). Agent plus generation alone is not enough: a
// runtime can be borrowed again under the same generation, so a stale report
// left over from an earlier assignment would otherwise land on whatever task is
// assigned now. A standing yield intent is likewise refused: the owner already
// asked to release this execution to watch child work, and accepting a result
// in the same turn would commit simultaneous yield and release intents.
func (s *Store) AcceptPipelineStageTaskResult(taskID, agentID, generation, executionHandle string, expectedRunRevision int64, result TaskResult) (PipelineRunRecord, error) {
	if err := ValidateAgentReport(result.Outcome, result.Summary, result.Details, ""); err != nil {
		return PipelineRunRecord{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: begin accept pipeline task result: %w", err)
	}
	defer tx.Rollback()
	var runID, runState, stageState, taskState, assignedID, assignedGeneration, storedHandle, outputJSON string
	var revision int64
	var pendingYield int
	err = tx.QueryRow(`
SELECT p.run_id, r.state, p.state, r.revision, t.state, COALESCE(t.assigned_agent_id, ''), t.assigned_generation, t.execution_handle, t.pending_yield, p.output_values_json
FROM pipeline_stage_tasks p JOIN pipeline_runs r ON r.run_id = p.run_id JOIN tasks t ON t.task_id = p.task_id
WHERE p.task_id = ?`, taskID).Scan(&runID, &runState, &stageState, &revision, &taskState, &assignedID, &assignedGeneration, &storedHandle, &pendingYield, &outputJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineRunRecord{}, ErrNotFound
	}
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: read stage result authority: %w", err)
	}
	if revision != expectedRunRevision || stageState != "open" || !OwnsReportedWork(agentID, generation, assignedID, assignedGeneration) || (taskState != TaskStarting && taskState != TaskRunning) {
		return PipelineRunRecord{}, ErrPipelineStageConflict
	}
	// The run state, not the revision, is the stop fence. A caller reads the
	// current revision immediately before reporting, so a report that arrives
	// after Stop committed `stopping` carries the *new* revision and would
	// otherwise overwrite it with `finishing`, un-stopping the run before
	// cancellation reached this task (TS-09.R42, INV §5/§15).
	if runState == "stopping" || runState == "stopped" || runState == "completed" {
		return PipelineRunRecord{}, ErrPipelineStageConflict
	}
	if executionHandle == "" || executionHandle != storedHandle || pendingYield != 0 {
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
	// The same fence again as the write's own condition, so a Stop that commits
	// between the read above and this statement loses nothing.
	res, err := tx.Exec(`UPDATE pipeline_runs SET state = 'finishing', pending_action = 'release_stage_task', attention_reason = ?, revision = revision + 1, updated_at = ? WHERE run_id = ? AND revision = ? AND state NOT IN ('stopping', 'stopped', 'completed')`, attention, stamp, runID, revision)
	if err != nil {
		return PipelineRunRecord{}, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return PipelineRunRecord{}, err
	} else if n != 1 {
		return PipelineRunRecord{}, ErrPipelineStageConflict
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
	var stateName string
	err = tx.QueryRow(`SELECT revision, state FROM pipeline_runs WHERE run_id = ?`, p.RunID).Scan(&revision, &stateName)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineStageTask{}, Task{}, false, ErrNotFound
	}
	if err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: read pipeline run: %w", err)
	}
	if revision != p.ExpectedRevision || stateName == "stopping" || stateName == "stopped" || stateName == "completed" {
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
	p.Task.AttentionReason, p.Task.ReadyAt = "", &now
	if err := insertTaskRowTx(tx, p.Task); err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: insert pipeline stage task: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO task_lineage(task_id, parent_task_id, pipeline_run_id, pipeline_stage_id, creation_attempt_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		p.Task.TaskID, p.ParentTaskID, p.RunID, p.StageID, fmt.Sprintf("%d", p.AttemptNumber), formatTime(now)); err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: insert pipeline task lineage: %w", err)
	}
	coordinatorID := ""
	if p.Coordinator != nil {
		c := *p.Coordinator
		c.State, c.Revision, c.CreatedAt, c.UpdatedAt = TaskArmed, 1, now, now
		// Armed until its standing owner is confirmed, so no ready time (TS-09.R49).
		c.AttentionReason, c.ReadyAt = "", nil
		if err := insertTaskRowTx(tx, c); err != nil {
			return PipelineStageTask{}, Task{}, false, err
		}
		if _, err := tx.Exec(`INSERT INTO task_lineage(task_id, parent_task_id, pipeline_run_id, pipeline_stage_id, creation_attempt_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`, c.TaskID, p.Task.TaskID, p.RunID, p.StageID, fmt.Sprintf("%d", p.AttemptNumber), formatTime(now)); err != nil {
			return PipelineStageTask{}, Task{}, false, err
		}
		coordinatorID = c.TaskID
	}
	outputJSON, err := json.Marshal(p.OutputValues)
	if err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: encode stage output contract: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO pipeline_stage_tasks(run_id, stage_index, attempt_number, stage_id, task_id, coordinator_task_id, assignment_digest, output_values_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.RunID, p.StageIndex, p.AttemptNumber, p.StageID, p.Task.TaskID, coordinatorID, p.AssignmentDigest, string(outputJSON), formatTime(now)); err != nil {
		return PipelineStageTask{}, Task{}, false, fmt.Errorf("state: insert pipeline stage association: %w", err)
	}
	if _, err := tx.Exec(`UPDATE pipeline_runs SET state = 'queued', pending_action = 'dispatch_stage_task', current_stage_id = ?, revision = revision + 1, updated_at = ? WHERE run_id = ? AND revision = ?`, p.StageID, formatTime(now), p.RunID, revision); err != nil {
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
	return scanPipelineStageTask(s.db.QueryRow(`SELECT `+pipelineStageTaskColumns+` FROM pipeline_stage_tasks p WHERE p.task_id = ?`, taskID))
}

// ListPipelineRunTaskLineage reads a run's task provenance in one query instead
// of one read per task, which is what made a long run's supervision read grow
// with its descendant count (TS-09.R42, INV §16).
func (s *Store) ListPipelineRunTaskLineage(runID string, limit int) (map[string]TaskLineage, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT task_id, parent_task_id, pipeline_run_id, pipeline_stage_id, creation_attempt_id, created_at FROM task_lineage WHERE pipeline_run_id = ? ORDER BY created_at, task_id LIMIT ?`, runID, limit)
	if err != nil {
		return nil, fmt.Errorf("state: list pipeline run task lineage: %w", err)
	}
	defer rows.Close()
	out := map[string]TaskLineage{}
	for rows.Next() {
		var v TaskLineage
		var created string
		if err := rows.Scan(&v.TaskID, &v.ParentTaskID, &v.PipelineRunID, &v.PipelineStageID, &v.CreationAttemptID, &created); err != nil {
			return nil, fmt.Errorf("state: scan pipeline run task lineage: %w", err)
		}
		if v.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		out[v.TaskID] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate pipeline run task lineage: %w", err)
	}
	return out, nil
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
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var coordinator string
	res, err := tx.Exec(`UPDATE pipeline_stage_tasks SET standing_agent_id = ? WHERE task_id = ? AND state = 'open' AND (standing_agent_id = '' OR standing_agent_id = ?)`, agentID, taskID, agentID)
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
	if err := tx.QueryRow(`SELECT coordinator_task_id FROM pipeline_stage_tasks WHERE task_id = ?`, taskID).Scan(&coordinator); err != nil {
		return err
	}
	if coordinator != "" {
		if _, err := tx.Exec(`UPDATE tasks SET state = ?, ready_at = ?, revision = revision + 1, updated_at = ? WHERE task_id = ? AND state = ?`, TaskReady, formatTime(timeNow()), formatTime(timeNow()), coordinator, TaskArmed); err != nil {
			return err
		}
	}
	return tx.Commit()
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

// PreparePipelineStageReplacement fences the interrupted owner and records its
// armed successor in one transaction. The successor is admitted separately so
// recovery can replay that single committed intent without minting another task.
func (s *Store) PreparePipelineStageReplacement(p CreatePipelineStageTaskParams, oldTaskID string) (PipelineRunRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, err
	}
	defer tx.Rollback()
	var revision int64
	var runState, currentStage, oldState, coordinatorID string
	err = tx.QueryRow(`SELECT r.revision, r.state, r.current_stage_id, t.state, ps.coordinator_task_id FROM pipeline_runs r JOIN pipeline_stage_tasks ps ON ps.run_id = r.run_id AND ps.task_id = ? JOIN tasks t ON t.task_id = ps.task_id WHERE r.run_id = ? AND ps.state = 'open'`, oldTaskID, p.RunID).Scan(&revision, &runState, &currentStage, &oldState, &coordinatorID)
	if err != nil || revision != p.ExpectedRevision || runState != "paused" || currentStage != p.StageID || oldState != TaskInterrupted {
		return PipelineRunRecord{}, ErrPipelineStageConflict
	}
	now := timeNow()
	stamp := formatTime(now)
	if _, err := tx.Exec(`UPDATE pipeline_stage_tasks SET state = 'replaced', closure_revision = ?, closed_at = ? WHERE task_id = ? AND state = 'open'`, revision+1, stamp, oldTaskID); err != nil {
		return PipelineRunRecord{}, err
	}
	if _, err := tx.Exec(`UPDATE tasks SET state = ?, outcome = ?, outcome_source = 'host', outcome_summary = 'replaced', attention_reason = '', finished_at = ?, revision = revision + 1, updated_at = ? WHERE task_id = ? AND state = ?`, TaskFinished, OutcomeCancelled, stamp, stamp, oldTaskID, TaskInterrupted); err != nil {
		return PipelineRunRecord{}, err
	}
	if err := RegisterWorkResultTx(tx, WorkResult{SourceKind: SourceTask, SourceID: oldTaskID, Outcome: OutcomeCancelled, Summary: "replaced"}, now); err != nil && !errors.Is(err, ErrWorkResultRecorded) {
		return PipelineRunRecord{}, err
	}
	p.Task.State, p.Task.Revision, p.Task.CreatedAt, p.Task.UpdatedAt = TaskArmed, 1, now, now
	// The successor is armed and admitted by a separate committed intent, so
	// recovery can replay that one step without minting a second task (R41).
	p.Task.AttentionReason, p.Task.ReadyAt = "", nil
	if err := insertTaskRowTx(tx, p.Task); err != nil {
		return PipelineRunRecord{}, err
	}
	if _, err := tx.Exec(`INSERT INTO task_lineage(task_id, parent_task_id, pipeline_run_id, pipeline_stage_id, creation_attempt_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`, p.Task.TaskID, oldTaskID, p.RunID, p.StageID, fmt.Sprintf("%d", p.AttemptNumber), stamp); err != nil {
		return PipelineRunRecord{}, err
	}
	outputs, err := json.Marshal(p.OutputValues)
	if err != nil {
		return PipelineRunRecord{}, err
	}
	if _, err := tx.Exec(`INSERT INTO pipeline_stage_tasks(run_id, stage_index, attempt_number, stage_id, task_id, coordinator_task_id, assignment_digest, output_values_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, p.RunID, p.StageIndex, p.AttemptNumber, p.StageID, p.Task.TaskID, coordinatorID, p.AssignmentDigest, string(outputs), stamp); err != nil {
		return PipelineRunRecord{}, err
	}
	if _, err := tx.Exec(`UPDATE pipeline_runs SET state = 'paused', pending_action = 'activate_replacement', current_agent_id = '', attention_reason = '', revision = revision + 1, updated_at = ? WHERE run_id = ? AND revision = ?`, stamp, p.RunID, revision); err != nil {
		return PipelineRunRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, err
	}
	return s.ReadPipelineRun(p.RunID)
}

func (s *Store) ActivatePipelineStageReplacement(runID, taskID string, expectedRevision int64) (PipelineRunRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, err
	}
	defer tx.Rollback()
	now := formatTime(timeNow())
	res, err := tx.Exec(`UPDATE tasks SET state = ?, ready_at = ?, revision = revision + 1, updated_at = ? WHERE task_id = ? AND state = ?`, TaskReady, now, now, taskID, TaskArmed)
	if err != nil {
		return PipelineRunRecord{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return PipelineRunRecord{}, ErrPipelineStageConflict
	}
	res, err = tx.Exec(`UPDATE pipeline_runs SET state = 'queued', pending_action = 'dispatch_stage_task', revision = revision + 1, updated_at = ? WHERE run_id = ? AND revision = ? AND pending_action = 'activate_replacement'`, now, runID, expectedRevision)
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
