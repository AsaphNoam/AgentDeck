package state

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
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
	// Room makes this a room-backed think_tank stage: the room, its reserved
	// identities and the binding commit with the stage task, before any launch
	// (TS-09.R52). The pipeline origin fields are filled here, not by callers.
	Room *ThinkTankCreate
}

// insertPipelineStageRowsTx is the one stage-task construction shared by run
// start and later stage creation (INV §2): task, lineage, optional managed
// coordinator or room, and the stage association. It returns the bound room id.
func insertPipelineStageRowsTx(tx *sql.Tx, p CreatePipelineStageTaskParams, now time.Time) (string, error) {
	p.Task.CreatedAt, p.Task.UpdatedAt, p.Task.Revision = now, now, 1
	p.Task.State, p.Task.Arms = TaskReady, nil
	p.Task.AttentionReason, p.Task.ReadyAt = "", &now
	kind := StageExecutionAgent
	if p.Room != nil {
		// A room-backed task is never admitted by the task dispatcher: it has no
		// assignee, handle or slot, and its phase comes from the room (TS-10.R38).
		kind = StageExecutionThinkTank
		p.Task.TargetKind, p.Task.State, p.Task.ReadyAt = TargetThinkTank, TaskRunning, nil
		p.Task.Role, p.Task.Backend, p.Task.Model, p.Task.Effort, p.Task.Fast = "", "", "", "", false
		if p.Coordinator != nil {
			return "", fmt.Errorf("state: a room-backed stage has no coordinator")
		}
	}
	if err := insertTaskRowTx(tx, p.Task); err != nil {
		return "", fmt.Errorf("state: insert pipeline stage task: %w", err)
	}
	attempt := fmt.Sprintf("%d", p.AttemptNumber)
	if _, err := tx.Exec(`INSERT INTO task_lineage(task_id, parent_task_id, pipeline_run_id, pipeline_stage_id, creation_attempt_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		p.Task.TaskID, p.ParentTaskID, p.RunID, p.StageID, attempt, formatTime(now)); err != nil {
		return "", fmt.Errorf("state: insert pipeline task lineage: %w", err)
	}
	coordinatorID := ""
	if p.Coordinator != nil {
		c := *p.Coordinator
		c.State, c.Revision, c.CreatedAt, c.UpdatedAt = TaskArmed, 1, now, now
		// A managed coordinator is armed until its standing owner is confirmed,
		// so it deliberately carries no ready time (TS-09.R49).
		c.AttentionReason, c.ReadyAt = "", nil
		if err := insertTaskRowTx(tx, c); err != nil {
			return "", err
		}
		if _, err := tx.Exec(`INSERT INTO task_lineage(task_id, parent_task_id, pipeline_run_id, pipeline_stage_id, creation_attempt_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`, c.TaskID, p.Task.TaskID, p.RunID, p.StageID, attempt, formatTime(now)); err != nil {
			return "", err
		}
		coordinatorID = c.TaskID
	}
	roomID := ""
	if p.Room != nil {
		c := *p.Room
		c.PipelineRunID, c.PipelineStageID, c.PipelineTaskID = p.RunID, p.StageID, p.Task.TaskID
		if c.StageContext == "" {
			return "", thinkTankInvalid("a pipeline room requires stage context")
		}
		d, err := createThinkTankTx(tx, c)
		if err != nil {
			return "", err
		}
		if d.Room.PipelineTaskID != p.Task.TaskID {
			return "", ErrPipelineStageConflict
		}
		roomID = d.Room.RoomID
	}
	outputJSON, err := json.Marshal(p.OutputValues)
	if err != nil {
		return "", fmt.Errorf("state: encode stage output contract: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO pipeline_stage_tasks(run_id, stage_index, attempt_number, stage_id, task_id, coordinator_task_id, assignment_digest, output_values_json, created_at, execution_kind, room_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.RunID, p.StageIndex, p.AttemptNumber, p.StageID, p.Task.TaskID, coordinatorID, p.AssignmentDigest, string(outputJSON), formatTime(now), kind, roomID); err != nil {
		return "", fmt.Errorf("state: insert pipeline stage association: %w", err)
	}
	return roomID, nil
}

const pipelineStageTaskColumns = `p.run_id, p.stage_index, p.attempt_number, p.stage_id, p.task_id, p.standing_agent_id, p.coordinator_task_id, p.assignment_digest, p.state, p.closure_revision, p.created_at, COALESCE(p.closed_at, ''), p.execution_kind, p.room_id, p.source_entry_seq, p.acceptance_entry_seq, p.acceptance_attempts`

const maxThinkTankAcceptanceAttempts = 3

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
SELECT `+pipelineStageTaskColumns+`
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
	var templateJSON string
	var stageIndex int
	var revision int64
	var pendingYield int
	err = tx.QueryRow(`
		SELECT p.run_id, r.state, p.state, r.revision, t.state, COALESCE(t.assigned_agent_id, ''), t.assigned_generation, t.execution_handle, t.pending_yield, p.output_values_json, p.stage_index, r.template_snapshot_json
FROM pipeline_stage_tasks p JOIN pipeline_runs r ON r.run_id = p.run_id JOIN tasks t ON t.task_id = p.task_id
		WHERE p.task_id = ?`, taskID).Scan(&runID, &runState, &stageState, &revision, &taskState, &assignedID, &assignedGeneration, &storedHandle, &pendingYield, &outputJSON, &stageIndex, &templateJSON)
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
	if result.Outcome == OutcomeSuccess {
		if err := validateProspectiveRoomContextsTx(tx, runID, stageIndex, templateJSON, declared, result.Outputs); err != nil {
			return PipelineRunRecord{}, err
		}
	}
	if err := commitStageResultTx(tx, stageResultCommit{
		RunID: runID, TaskID: taskID, TaskState: taskState, Revision: revision, Source: "agent",
		PendingRelease: true, Result: result, Declared: declared,
	}); err != nil {
		return PipelineRunRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: commit pipeline task result: %w", err)
	}
	return s.ReadPipelineRun(runID)
}

// stageResultCommit is one already-authorized stage result. Authority is
// validated by the caller's own branch (ordinary handle or trusted room
// synthesis); these writes are shared (TS-09.R53, INV §2).
type stageResultCommit struct {
	RunID, TaskID, TaskState, Source string
	Revision                         int64
	PendingRelease                   bool
	SourceEntrySeq                   int64
	Result                           TaskResult
	Declared                         map[string]string
}

// commitStageResultTx writes the immutable task result, named outputs, value
// projection, closure fence and run projection once.
func commitStageResultTx(tx *sql.Tx, c stageResultCommit) error {
	result, taskID, runID, revision, declared := c.Result, c.TaskID, c.RunID, c.Revision, c.Declared
	now := timeNow()
	stamp := formatTime(now)
	res, err := tx.Exec(`UPDATE tasks SET state = ?, outcome = ?, outcome_source = ?, outcome_summary = ?, outcome_details = ?, attention_reason = '', pending_release = ?, finished_at = ?, revision = revision + 1, updated_at = ? WHERE task_id = ? AND state = ?`, TaskFinished, result.Outcome, c.Source, result.Summary, result.Details, c.PendingRelease, stamp, stamp, taskID, c.TaskState)
	if err != nil {
		return fmt.Errorf("state: finish stage task: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return ErrPipelineStageConflict
	}
	if err := RegisterWorkResultTx(tx, WorkResult{SourceKind: SourceTask, SourceID: taskID, Outcome: result.Outcome, Summary: result.Summary}, now); err != nil {
		return err
	}
	if err := insertTaskResultOutputs(tx, taskID, result.Outputs); err != nil {
		return err
	}
	for name, valueKey := range declared {
		if value, ok := result.Outputs[name]; ok {
			if _, err := tx.Exec(`INSERT INTO pipeline_values(run_id, name, value, source_kind, source_attempt_id, updated_at) VALUES (?, ?, ?, 'stage_task', ?, ?) ON CONFLICT(run_id, name) DO UPDATE SET value = excluded.value, source_kind = excluded.source_kind, source_attempt_id = excluded.source_attempt_id, updated_at = excluded.updated_at`, runID, valueKey, value, taskID, stamp); err != nil {
				return fmt.Errorf("state: store stage output: %w", err)
			}
		}
	}
	res, err = tx.Exec(`UPDATE pipeline_stage_tasks SET state = 'closing', closure_revision = ?, closed_at = ?, source_entry_seq = ? WHERE task_id = ? AND state = 'open'`, revision+1, stamp, c.SourceEntrySeq, taskID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return ErrPipelineStageConflict
	}
	attention := ""
	if result.Outcome == OutcomeFailure || result.Outcome == OutcomeBlocked {
		attention = result.Outcome
	}
	// The same fence again as the write's own condition, so a Stop that commits
	// between the caller's read and this statement loses nothing.
	res, err = tx.Exec(`UPDATE pipeline_runs SET state = 'finishing', pending_action = 'release_stage_task', attention_reason = ?, revision = revision + 1, updated_at = ? WHERE run_id = ? AND revision = ? AND state NOT IN ('stopping', 'stopped', 'completed')`, attention, stamp, runID, revision)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return ErrPipelineStageConflict
	}
	return nil
}

// ErrThinkTankOutputRefused is a non-recoverable synthesis acceptance refusal:
// the published text cannot become the named output as it stands (TS-09.R53).
var ErrThinkTankOutputRefused = errors.New("state: think tank output refused")

// validateProspectiveRoomContextsTx checks the values that would be visible to
// every later room before the producing task is accepted. It deliberately runs
// in the result transaction: a rejected output must leave the producer and its
// run cursor available for correction (TS-09.R56, FS-14.A52).
func validateProspectiveRoomContextsTx(tx *sql.Tx, runID string, stageIndex int, templateJSON string, declared, outputs map[string]string) error {
	type stageInput struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	type stageOutput struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	type stage struct {
		ID           string        `json:"id"`
		Title        string        `json:"title"`
		Objective    string        `json:"objective"`
		Coordination string        `json:"coordination"`
		Inputs       []stageInput  `json:"inputs"`
		Outputs      []stageOutput `json:"outputs"`
	}
	var snapshot struct {
		Stages []stage `json:"stages"`
	}
	if err := json.Unmarshal([]byte(templateJSON), &snapshot); err != nil {
		return fmt.Errorf("state: decode pipeline template for prospective room context: %w", err)
	}
	var runName, goal string
	if err := tx.QueryRow(`SELECT display_name, goal FROM pipeline_runs WHERE run_id = ?`, runID).Scan(&runName, &goal); err != nil {
		return fmt.Errorf("state: read pipeline context identity: %w", err)
	}
	values := map[string]string{}
	rows, err := tx.Query(`SELECT name, value FROM pipeline_values WHERE run_id = ?`, runID)
	if err != nil {
		return fmt.Errorf("state: read prospective pipeline values: %w", err)
	}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			rows.Close()
			return err
		}
		values[name] = value
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("state: iterate prospective pipeline values: %w", err)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for name, valueKey := range declared {
		if value, ok := outputs[name]; ok {
			values[valueKey] = value
		}
	}
	outputNames := []string{}
	for name := range outputs {
		if _, declaredOutput := declared[name]; declaredOutput {
			outputNames = append(outputNames, name)
		}
	}
	sort.Strings(outputNames)
	for i := stageIndex + 1; i < len(snapshot.Stages); i++ {
		next := snapshot.Stages[i]
		if next.Coordination != "think_tank" || len(next.Outputs) != 1 {
			continue
		}
		context := ThinkTankStageContext{
			RunID: runID, RunName: runName, StageID: next.ID, StageTitle: next.Title,
			Goal: goal, Objective: next.Objective, Inputs: []ThinkTankStageInput{},
			Output: ThinkTankStageOutput{Name: next.Outputs[0].Name, Description: next.Outputs[0].Description, MaxRunes: MaxStageValueRunes},
		}
		for _, input := range next.Inputs {
			context.Inputs = append(context.Inputs, ThinkTankStageInput{Name: input.Name, Value: values[input.Value]})
		}
		if _, err := EncodeThinkTankStageContext(context); err != nil {
			return fmt.Errorf("%w: output %s: later room %s context exceeds its size limit", ErrThinkTankOutputRefused, strings.Join(outputNames, ", "), next.ID)
		}
	}
	return nil
}

// MaxStageValueRunes mirrors the pipeline named-value limit for room output.
const MaxStageValueRunes = 64000

// AcceptThinkTankStageOutput is the room branch of stage result acceptance. Host
// authority is the bound room's finalized judge synthesis, checked inside this
// transaction with the open run/stage and expected revision; no HTTP or MCP
// caller supplies it. The exact synthesis becomes the single named output, with
// the room and entry recorded as its source (TS-09.R53, TS-14.R21).
func (s *Store) AcceptThinkTankStageOutput(taskID string, expectedRunRevision int64) (PipelineRunRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: begin accept think tank output: %w", err)
	}
	defer tx.Rollback()
	var runID, runState, stageState, kind, roomID, taskState, outputJSON, judgeStatus string
	var templateJSON string
	var stageIndex int
	var revision, acceptanceSeq int64
	err = tx.QueryRow(`
		SELECT p.run_id, r.state, p.state, r.revision, p.execution_kind, p.room_id, t.state, p.output_values_json, tt.judge_status, p.stage_index, r.template_snapshot_json, p.acceptance_entry_seq
FROM pipeline_stage_tasks p JOIN pipeline_runs r ON r.run_id = p.run_id JOIN tasks t ON t.task_id = p.task_id
JOIN think_tanks tt ON tt.room_id = p.room_id AND tt.pipeline_task_id = p.task_id
		WHERE p.task_id = ?`, taskID).Scan(&runID, &runState, &stageState, &revision, &kind, &roomID, &taskState, &outputJSON, &judgeStatus, &stageIndex, &templateJSON, &acceptanceSeq)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineRunRecord{}, ErrNotFound
	}
	if err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: read room output authority: %w", err)
	}
	if kind != StageExecutionThinkTank || revision != expectedRunRevision || stageState != "open" || taskState == TaskFinished ||
		runState == "stopping" || runState == "stopped" || runState == "completed" || judgeStatus != ThinkTankJudgeCompleted {
		return PipelineRunRecord{}, ErrPipelineStageConflict
	}
	var seq int64
	var body string
	if err := tx.QueryRow(`SELECT seq, body FROM think_tank_entries WHERE room_id = ? AND kind = ? ORDER BY seq DESC LIMIT 1`, roomID, ThinkTankEntrySynthesis).Scan(&seq, &body); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PipelineRunRecord{}, ErrPipelineStageConflict
		}
		return PipelineRunRecord{}, fmt.Errorf("state: read room synthesis: %w", err)
	}
	if acceptanceSeq == 0 || acceptanceSeq != seq {
		return PipelineRunRecord{}, ErrPipelineStageConflict
	}
	declared := map[string]string{}
	if err := json.Unmarshal([]byte(outputJSON), &declared); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: decode stage output contract: %w", err)
	}
	if len(declared) != 1 {
		return PipelineRunRecord{}, fmt.Errorf("%w: the stage must declare exactly one output", ErrThinkTankOutputRefused)
	}
	name := ""
	for n := range declared {
		name = n
	}
	if strings.TrimSpace(body) == "" {
		return PipelineRunRecord{}, fmt.Errorf("%w: output %s: the synthesis is empty", ErrThinkTankOutputRefused, name)
	}
	if utf8.RuneCountInString(body) > MaxStageValueRunes {
		return PipelineRunRecord{}, fmt.Errorf("%w: output %s: the synthesis exceeds %d characters", ErrThinkTankOutputRefused, name, MaxStageValueRunes)
	}
	if err := validateProspectiveRoomContextsTx(tx, runID, stageIndex, templateJSON, declared, map[string]string{name: body}); err != nil {
		return PipelineRunRecord{}, err
	}
	summary := fmt.Sprintf("Think Tank judge synthesis accepted from room %s entry %d.", roomID, seq)
	if err := commitStageResultTx(tx, stageResultCommit{
		RunID: runID, TaskID: taskID, TaskState: taskState, Revision: revision, Source: StageExecutionThinkTank,
		SourceEntrySeq: seq, Result: TaskResult{Outcome: OutcomeSuccess, Summary: summary, Outputs: map[string]string{name: body}}, Declared: declared,
	}); err != nil {
		return PipelineRunRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return PipelineRunRecord{}, fmt.Errorf("state: commit think tank output: %w", err)
	}
	return s.ReadPipelineRun(runID)
}

// PrepareThinkTankStageOutputAcceptance records the exact published synthesis
// and, for automatic recovery, consumes one bounded attempt. The update is
// conditional on the open stage authority so a restart or a duplicate sweep
// cannot reset or replace the source entry (TS-09.R53, INV §5/§9).
func (s *Store) PrepareThinkTankStageOutputAcceptance(taskID string, automatic bool) (PipelineStageTask, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PipelineStageTask{}, fmt.Errorf("state: begin prepare think tank output acceptance: %w", err)
	}
	defer tx.Rollback()
	var runState, stageState, kind, roomID, judgeStatus string
	var sourceSeq, attempts int64
	err = tx.QueryRow(`
SELECT r.state, p.state, p.execution_kind, p.room_id, tt.judge_status,
       p.acceptance_entry_seq, p.acceptance_attempts
FROM pipeline_stage_tasks p JOIN pipeline_runs r ON r.run_id = p.run_id
JOIN think_tanks tt ON tt.room_id = p.room_id AND tt.pipeline_task_id = p.task_id
WHERE p.task_id = ?`, taskID).Scan(&runState, &stageState, &kind, &roomID, &judgeStatus, &sourceSeq, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineStageTask{}, ErrNotFound
	}
	if err != nil {
		return PipelineStageTask{}, fmt.Errorf("state: read think tank output acceptance: %w", err)
	}
	if kind != StageExecutionThinkTank || stageState != "open" || runState == "stopping" || runState == "stopped" || runState == "completed" || judgeStatus != ThinkTankJudgeCompleted {
		return PipelineStageTask{}, ErrPipelineStageConflict
	}
	var publishedSeq int64
	if err := tx.QueryRow(`SELECT seq FROM think_tank_entries WHERE room_id = ? AND kind = ? ORDER BY seq DESC LIMIT 1`, roomID, ThinkTankEntrySynthesis).Scan(&publishedSeq); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PipelineStageTask{}, ErrPipelineStageConflict
		}
		return PipelineStageTask{}, fmt.Errorf("state: read think tank synthesis: %w", err)
	}
	if sourceSeq != 0 && sourceSeq != publishedSeq {
		return PipelineStageTask{}, ErrPipelineStageConflict
	}
	if sourceSeq == 0 {
		sourceSeq = publishedSeq
	}
	if automatic && attempts < maxThinkTankAcceptanceAttempts {
		attempts++
	}
	if _, err := tx.Exec(`UPDATE pipeline_stage_tasks SET acceptance_entry_seq = ?, acceptance_attempts = ? WHERE task_id = ? AND state = 'open'`, sourceSeq, attempts, taskID); err != nil {
		return PipelineStageTask{}, fmt.Errorf("state: record think tank output acceptance: %w", err)
	}
	if runState == "running" {
		if _, err := tx.Exec(`UPDATE pipeline_runs SET pending_action = 'accept_room_output', revision = revision + 1, updated_at = ? WHERE run_id = (SELECT run_id FROM pipeline_stage_tasks WHERE task_id = ?) AND state = 'running' AND pending_action = ''`, formatTime(timeNow()), taskID); err != nil {
			return PipelineStageTask{}, fmt.Errorf("state: record room output intent: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return PipelineStageTask{}, fmt.Errorf("state: commit think tank output acceptance: %w", err)
	}
	return scanPipelineStageTask(s.db.QueryRow(`SELECT `+pipelineStageTaskColumns+` FROM pipeline_stage_tasks p WHERE p.task_id = ?`, taskID))
}

// PendingThinkTankStageOutputs lists open room-backed stages whose judge has
// completed, for the bounded post-commit recovery sweep (TS-09.R53).
func (s *Store) PendingThinkTankStageOutputs(limit int) ([]PipelineStageTask, error) {
	rows, err := s.db.Query(`SELECT `+pipelineStageTaskColumns+` FROM pipeline_stage_tasks p
JOIN think_tanks tt ON tt.room_id = p.room_id
JOIN pipeline_runs r ON r.run_id = p.run_id
WHERE p.execution_kind = ? AND p.state = 'open' AND tt.judge_status = ?
  AND r.state = 'running' AND r.pending_action IN ('', 'accept_room_output')
ORDER BY p.created_at, p.run_id, p.stage_index, p.attempt_number LIMIT ?`, StageExecutionThinkTank, ThinkTankJudgeCompleted, limit)
	if err != nil {
		return nil, fmt.Errorf("state: list pending room outputs: %w", err)
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
	return out, rows.Err()
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
	if _, err := insertPipelineStageRowsTx(tx, p, now); err != nil {
		return PipelineStageTask{}, Task{}, false, err
	}
	// A room-backed stage needs no task dispatch: the room engine owns it, so
	// the run is running from creation (TS-09.R55).
	runState, pending := "queued", "dispatch_stage_task"
	if p.Room != nil {
		runState, pending = "running", ""
	}
	if _, err := tx.Exec(`UPDATE pipeline_runs SET state = ?, pending_action = ?, current_stage_id = ?, revision = revision + 1, updated_at = ? WHERE run_id = ? AND revision = ?`, runState, pending, p.StageID, formatTime(now), p.RunID, revision); err != nil {
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
	rows, err := s.db.Query(`SELECT `+pipelineStageTaskColumns+` FROM pipeline_stage_tasks p WHERE p.run_id = ? ORDER BY p.stage_index, p.attempt_number`, runID)
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
	return scanPipelineStageTask(tx.QueryRow(`SELECT `+pipelineStageTaskColumns+` FROM pipeline_stage_tasks p WHERE p.run_id = ? AND p.stage_index = ? AND p.attempt_number = ?`, runID, stageIndex, attempt))
}

func scanPipelineStageTask(row interface{ Scan(...any) error }) (PipelineStageTask, error) {
	var v PipelineStageTask
	var created, closed string
	if err := row.Scan(&v.RunID, &v.StageIndex, &v.AttemptNumber, &v.StageID, &v.TaskID, &v.StandingAgentID, &v.CoordinatorTaskID, &v.AssignmentDigest, &v.State, &v.ClosureRevision, &created, &closed, &v.ExecutionKind, &v.RoomID, &v.SourceEntrySeq, &v.AcceptanceEntrySeq, &v.AcceptanceAttempts); err != nil {
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

// PipelineOrchestratorTemplateSnapshot returns the frozen template of the run
// whose agent-executed stage task or dedicated coordinator binding is taskID
// (TS-09.R58). Lineage only locates the coordinator's binding row by key; an
// ordinary descendant and a room-backed stage return ErrNotFound.
func (s *Store) PipelineOrchestratorTemplateSnapshot(taskID string) (json.RawMessage, error) {
	if taskID == "" {
		return nil, ErrNotFound
	}
	var snapshot string
	err := s.db.QueryRow(`SELECT r.template_snapshot_json FROM pipeline_stage_tasks p JOIN pipeline_runs r ON r.run_id = p.run_id
		WHERE p.task_id = ? AND p.execution_kind = ?
		UNION ALL
		SELECT r.template_snapshot_json FROM task_lineage l
		JOIN pipeline_stage_tasks p ON p.task_id = l.parent_task_id AND p.coordinator_task_id = l.task_id
		JOIN pipeline_runs r ON r.run_id = p.run_id
		WHERE l.task_id = ?
		LIMIT 1`, taskID, StageExecutionAgent, taskID).Scan(&snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("state: read pipeline orchestrator template: %w", err)
	}
	return json.RawMessage(snapshot), nil
}

// BindPipelineStageTaskStandingAgent records the dispatcher-confirmed identity.
// A non-empty different identity is never silently substituted.
func (s *Store) BindPipelineStageTaskStandingAgent(taskID, agentID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var coordinator, runID string
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
	if err := tx.QueryRow(`SELECT run_id, coordinator_task_id FROM pipeline_stage_tasks WHERE task_id = ?`, taskID).Scan(&runID, &coordinator); err != nil {
		return err
	}
	// The confirmed start is the run's queued → running boundary (FS-14 §3,
	// R37). Only a dispatch-pending run moves, so a Stop that already
	// committed keeps its state.
	if _, err := tx.Exec(`UPDATE pipeline_runs SET state = 'running', pending_action = '', revision = revision + 1, updated_at = ? WHERE run_id = ? AND state = 'queued' AND pending_action = 'dispatch_stage_task'`, formatTime(timeNow()), runID); err != nil {
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
