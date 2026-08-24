package state

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Task states (FS-16 §3). Only these values are ever written.
const (
	TaskArmed            = "armed"
	TaskReady            = "ready"
	TaskStarting         = "starting"
	TaskRunning          = "running"
	TaskInterrupted      = "interrupted"
	TaskFinished         = "finished"
	TaskDependencyFailed = "dependency_failed"
)

// Task outcomes (FS-16.R3). An agent or a person may record the first three;
// cancelled is host-written only.
const (
	OutcomeSuccess   = "success"
	OutcomeFailure   = "failure"
	OutcomeBlocked   = "blocked"
	OutcomeCancelled = "cancelled"
)

// Arm kinds and states (FS-16.R5, §3).
const (
	ArmWorkResult = "work_result"
	ArmSignal     = "signal"

	ArmUnsatisfied   = "unsatisfied"
	ArmSatisfied     = "satisfied"
	ArmUnsatisfiable = "unsatisfiable"
)

// Result and target source kinds. A work result is keyed by these two together
// (TS-10.R8), and an arm names the same pair.
const (
	SourceTask        = "task"
	SourcePipelineRun = "pipeline_run"

	TargetAgent  = "agent"
	TargetLaunch = "launch"
)

// Runtime claim kinds (FS-16.R4). Only created and woke are stopped when the
// task finishes; a borrowed runtime belongs to someone else.
const (
	ClaimCreated  = "created"
	ClaimWoke     = "woke"
	ClaimBorrowed = "borrowed"
)

// ErrTaskConflict is returned when a compare-and-set loses to a newer revision.
var ErrTaskConflict = errors.New("state: task revision conflict")

// ErrTaskCycle is returned when an arm set would make the graph cyclic, name its
// own task, or name an unknown or cross-project prerequisite (FS-16.R15).
var ErrTaskCycle = errors.New("state: task arms would create a cycle")

// ErrTaskArmSource is returned when an arm names a prerequisite that does not
// exist or belongs to another project.
var ErrTaskArmSource = errors.New("state: task arm names an unusable source")

// ErrWorkResultRecorded is returned when a source already registered a result.
// Registrations are immutable (TS-10.R8).
var ErrWorkResultRecorded = errors.New("state: work result already registered")

// Task is one durable unit of dependent work (FS-16.R1).
type Task struct {
	TaskID      string `json:"task_id"`
	Project     string `json:"project"`
	DisplayName string `json:"display_name"`
	Instruction string `json:"instruction"`

	TargetKind    string `json:"target_kind"`
	TargetAgentID string `json:"target_agent_id,omitempty"`
	Role          string `json:"role,omitempty"`
	Backend       string `json:"backend,omitempty"`
	Model         string `json:"model,omitempty"`

	State           string `json:"state"`
	Outcome         string `json:"outcome,omitempty"`
	OutcomeSource   string `json:"outcome_source,omitempty"`
	OutcomeSummary  string `json:"outcome_summary,omitempty"`
	OutcomeDetails  string `json:"outcome_details,omitempty"`
	AttentionReason string `json:"attention_reason,omitempty"`

	CreatedByKind       string `json:"created_by_kind"`
	CreatedByAgentID    string `json:"created_by_agent_id,omitempty"`
	CreatedByGeneration string `json:"created_by_generation,omitempty"`

	AssignedAgentID    string `json:"assigned_agent_id,omitempty"`
	AssignedGeneration string `json:"assigned_generation,omitempty"`
	RuntimeClaim       string `json:"runtime_claim,omitempty"`
	PendingRelease     bool   `json:"pending_release"`
	StartAttemptID     string `json:"start_attempt_id,omitempty"`
	StartAttemptCount  int    `json:"start_attempt_count"`

	Revision int `json:"revision"`

	ReadyAt        *time.Time `json:"ready_at,omitempty"`
	StartClaimedAt *time.Time `json:"start_claimed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`

	Arms []TaskArm `json:"arms"`
}

// TaskArm is one prerequisite. Every arm of a task must be satisfied before it
// becomes ready — a conjunction, with no OR, quorum, or negation (FS-16.R5).
type TaskArm struct {
	ArmID  string `json:"arm_id"`
	TaskID string `json:"task_id"`
	Kind   string `json:"kind"`

	SourceKind string `json:"source_kind,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	SignalName string `json:"signal_name,omitempty"`

	// SatisfyingOutcomes is the non-empty outcome set that satisfies a
	// work_result arm. It is stored sorted and comma-separated so the closed
	// vocabulary stays greppable in the database.
	SatisfyingOutcomes []string `json:"satisfying_outcomes,omitempty"`

	State       string     `json:"state"`
	SatisfiedAt *time.Time `json:"satisfied_at,omitempty"`
}

// TaskAttachment binds a canonical context reference to a task with its own
// bounded presentation. It holds no authorization of its own (TS-10.R12).
type TaskAttachment struct {
	TaskID       string    `json:"task_id"`
	ContextRefID string    `json:"context_ref_id"`
	Label        string    `json:"label,omitempty"`
	Description  string    `json:"description,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// WorkResult is the normalized terminal outcome of one source, and the only
// thing arm evaluation reads (TS-10.R8).
type WorkResult struct {
	SourceKind string    `json:"source_kind"`
	SourceID   string    `json:"source_id"`
	Outcome    string    `json:"outcome"`
	RawLabel   string    `json:"raw_label,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}

// migrateTasks creates the dependent-work rows (TS-10.R16). No backfill: nothing
// existed before this table. Agent, pipeline-run, and context-reference ids are
// logical references without cascades, so deleting an agent, a run, or a
// reference never deletes task history and deleting a task never reaches into
// any of them.
func migrateTasks(tx *sql.Tx) error {
	if _, err := tx.Exec(`
CREATE TABLE tasks (
  task_id               TEXT PRIMARY KEY,
  project               TEXT NOT NULL,
  display_name          TEXT NOT NULL,
  instruction           TEXT NOT NULL,
  target_kind           TEXT NOT NULL,
  target_agent_id       TEXT NOT NULL DEFAULT '',
  role                  TEXT NOT NULL DEFAULT '',
  backend               TEXT NOT NULL DEFAULT '',
  model                 TEXT NOT NULL DEFAULT '',
  state                 TEXT NOT NULL,
  outcome               TEXT NOT NULL DEFAULT '',
  outcome_source        TEXT NOT NULL DEFAULT '',
  outcome_summary       TEXT NOT NULL DEFAULT '',
  outcome_details       TEXT NOT NULL DEFAULT '',
  attention_reason      TEXT NOT NULL DEFAULT '',
  created_by_kind       TEXT NOT NULL,
  created_by_agent_id   TEXT NOT NULL DEFAULT '',
  created_by_generation TEXT NOT NULL DEFAULT '',
  assigned_agent_id     TEXT,
  assigned_generation   TEXT NOT NULL DEFAULT '',
  runtime_claim         TEXT NOT NULL DEFAULT '',
  pending_release       INTEGER NOT NULL DEFAULT 0,
  start_attempt_id      TEXT NOT NULL DEFAULT '',
  start_attempt_count   INTEGER NOT NULL DEFAULT 0,
  revision              INTEGER NOT NULL DEFAULT 1,
  ready_at              TEXT,
  start_claimed_at      TEXT,
  created_at            TEXT NOT NULL,
  updated_at            TEXT NOT NULL,
  started_at            TEXT,
  finished_at           TEXT
);
-- FS-16.R2's "one active task per agent" is a database guarantee, not a
-- scheduling accident: the exclusive lifecycle claim is released after each
-- transition and cannot hold it (TS-10.R18).
CREATE UNIQUE INDEX idx_tasks_active_assignee
  ON tasks(assigned_agent_id)
  WHERE assigned_agent_id IS NOT NULL AND state IN ('starting', 'running');
-- The dispatcher admits ready work in the order it became ready (FS-16.R7).
CREATE INDEX idx_tasks_ready ON tasks(state, ready_at, task_id);
CREATE INDEX idx_tasks_project ON tasks(project, created_at DESC, task_id);

CREATE TABLE task_arms (
  arm_id              TEXT PRIMARY KEY,
  task_id             TEXT NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
  kind                TEXT NOT NULL,
  source_kind         TEXT NOT NULL DEFAULT '',
  source_id           TEXT NOT NULL DEFAULT '',
  satisfying_outcomes TEXT NOT NULL DEFAULT '',
  signal_name         TEXT NOT NULL DEFAULT '',
  state               TEXT NOT NULL,
  satisfied_at        TEXT
);
-- Evaluation is event-driven: a committed registration or fired signal looks up
-- only the arms that name that source (TS-10.R3).
CREATE INDEX idx_task_arms_source ON task_arms(source_kind, source_id, state);
CREATE INDEX idx_task_arms_signal ON task_arms(signal_name, state);
CREATE INDEX idx_task_arms_task ON task_arms(task_id);

CREATE TABLE task_attachments (
  task_id        TEXT NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
  context_ref_id TEXT NOT NULL,
  label          TEXT NOT NULL DEFAULT '',
  description    TEXT NOT NULL DEFAULT '',
  created_at     TEXT NOT NULL,
  PRIMARY KEY (task_id, context_ref_id)
);

CREATE TABLE work_results (
  source_kind TEXT NOT NULL,
  source_id   TEXT NOT NULL,
  outcome     TEXT NOT NULL,
  raw_label   TEXT NOT NULL DEFAULT '',
  summary     TEXT NOT NULL DEFAULT '',
  recorded_at TEXT NOT NULL,
  PRIMARY KEY (source_kind, source_id)
);

-- The dependency activation kind keys one pending row per (agent_id, source_id)
-- rather than mail's one per agent (TS-10.R5).
ALTER TABLE activations ADD COLUMN source_id TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX idx_activations_pending_dependency
  ON activations(agent_id, source_id)
  WHERE state = 'pending' AND kind = 'dependency';
`); err != nil {
		return fmt.Errorf("state: create tasks: %w", err)
	}
	return nil
}

// NewTaskID mints a durable opaque task id. A display name never substitutes.
func (s *Store) NewTaskID() (string, error) {
	for i := 0; i < 10; i++ {
		var bytes [8]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			return "", fmt.Errorf("state: mint task id: %w", err)
		}
		id := "tk_" + hex.EncodeToString(bytes[:])
		var exists int
		err := s.db.QueryRow(`SELECT 1 FROM tasks WHERE task_id = ?`, id).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return id, nil
		}
		if err != nil {
			return "", fmt.Errorf("state: check task id collision: %w", err)
		}
	}
	return "", errors.New("state: could not mint unique task id")
}

// CreateTask inserts a task and its arms in one transaction, rejecting a cycle,
// a self-reference, and an unknown or cross-project prerequisite inside it so
// two concurrent writers cannot interleave into a cycle (TS-10.R9, FS-16.R15).
// A rejected create mutates nothing.
func (s *Store) CreateTask(task Task) (Task, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Task{}, fmt.Errorf("state: begin create task: %w", err)
	}
	defer tx.Rollback()

	now := timeNow()
	task.CreatedAt, task.UpdatedAt = now, now
	task.Revision = 1
	task.State = initialTaskState(task.Arms)
	if task.State == TaskReady {
		task.ReadyAt = &now
	}

	if err := validateTaskArms(tx, task); err != nil {
		return Task{}, err
	}

	if _, err := tx.Exec(`
INSERT INTO tasks(task_id, project, display_name, instruction, target_kind, target_agent_id,
  role, backend, model, state, created_by_kind, created_by_agent_id, created_by_generation,
  revision, ready_at, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.TaskID, task.Project, task.DisplayName, task.Instruction, task.TargetKind,
		task.TargetAgentID, task.Role, task.Backend, task.Model, task.State,
		task.CreatedByKind, task.CreatedByAgentID, task.CreatedByGeneration,
		task.Revision, formatOptionalTime(task.ReadyAt), formatTime(task.CreatedAt),
		formatTime(task.UpdatedAt)); err != nil {
		return Task{}, fmt.Errorf("state: insert task: %w", err)
	}
	if err := insertTaskArms(tx, task.TaskID, task.Arms); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("state: commit create task: %w", err)
	}
	return s.ReadTask(task.TaskID)
}

// initialTaskState is armed while any arm is unsatisfied. A task with no arms is
// ready the moment it is created (FS-16.R5).
func initialTaskState(arms []TaskArm) string {
	for _, arm := range arms {
		if arm.State != ArmSatisfied {
			return TaskArmed
		}
	}
	return TaskReady
}

func insertTaskArms(tx *sql.Tx, taskID string, arms []TaskArm) error {
	for i, arm := range arms {
		if arm.ArmID == "" {
			arm.ArmID = fmt.Sprintf("%s_arm%02d", taskID, i)
		}
		if arm.State == "" {
			arm.State = ArmUnsatisfied
		}
		if _, err := tx.Exec(`
INSERT INTO task_arms(arm_id, task_id, kind, source_kind, source_id, satisfying_outcomes,
  signal_name, state, satisfied_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			arm.ArmID, taskID, arm.Kind, arm.SourceKind, arm.SourceID,
			encodeOutcomeSet(arm.SatisfyingOutcomes), arm.SignalName, arm.State,
			formatOptionalTime(arm.SatisfiedAt)); err != nil {
			return fmt.Errorf("state: insert task arm: %w", err)
		}
	}
	return nil
}

// validateTaskArms runs every graph check inside the caller's transaction so a
// concurrent writer cannot slip a back edge in between the check and the write.
func validateTaskArms(tx *sql.Tx, task Task) error {
	for _, arm := range task.Arms {
		if arm.Kind == ArmSignal {
			if arm.SignalName == "" {
				return fmt.Errorf("%w: signal arm has no name", ErrTaskArmSource)
			}
			continue
		}
		if len(arm.SatisfyingOutcomes) == 0 {
			return fmt.Errorf("%w: work_result arm has no satisfying outcome", ErrTaskArmSource)
		}
		if arm.SourceID == task.TaskID {
			return fmt.Errorf("%w: task names itself", ErrTaskCycle)
		}
		switch arm.SourceKind {
		case SourceTask:
			var project string
			err := tx.QueryRow(`SELECT project FROM tasks WHERE task_id = ?`, arm.SourceID).Scan(&project)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: no such prerequisite task %q", ErrTaskArmSource, arm.SourceID)
			}
			if err != nil {
				return fmt.Errorf("state: read prerequisite task: %w", err)
			}
			if project != task.Project {
				return fmt.Errorf("%w: prerequisite task is in another project", ErrTaskArmSource)
			}
		case SourcePipelineRun:
			var project string
			err := tx.QueryRow(`SELECT project FROM pipeline_runs WHERE run_id = ?`, arm.SourceID).Scan(&project)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: no such pipeline run %q", ErrTaskArmSource, arm.SourceID)
			}
			if err != nil {
				return fmt.Errorf("state: read prerequisite run: %w", err)
			}
			if project != task.Project {
				return fmt.Errorf("%w: pipeline run is in another project", ErrTaskArmSource)
			}
		default:
			return fmt.Errorf("%w: unknown arm source kind %q", ErrTaskArmSource, arm.SourceKind)
		}
	}
	return reachabilityAllows(tx, task)
}

// reachabilityAllows walks forward from every task prerequisite: if this task is
// already reachable from one of them, adding the arm closes a loop. Only task
// arms can form one — a pipeline run and a signal are never dependents.
func reachabilityAllows(tx *sql.Tx, task Task) error {
	seen := map[string]bool{}
	var frontier []string
	for _, arm := range task.Arms {
		if arm.Kind == ArmWorkResult && arm.SourceKind == SourceTask {
			frontier = append(frontier, arm.SourceID)
		}
	}
	for len(frontier) > 0 {
		id := frontier[0]
		frontier = frontier[1:]
		if id == task.TaskID {
			return ErrTaskCycle
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		rows, err := tx.Query(`
SELECT source_id FROM task_arms
WHERE task_id = ? AND kind = ? AND source_kind = ?`, id, ArmWorkResult, SourceTask)
		if err != nil {
			return fmt.Errorf("state: walk task arms: %w", err)
		}
		var next []string
		for rows.Next() {
			var sourceID string
			if err := rows.Scan(&sourceID); err != nil {
				rows.Close()
				return fmt.Errorf("state: scan task arm source: %w", err)
			}
			next = append(next, sourceID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("state: iterate task arms: %w", err)
		}
		rows.Close()
		frontier = append(frontier, next...)
	}
	return nil
}

// ReadTask returns one task with its arms, oldest arm first.
func (s *Store) ReadTask(taskID string) (Task, error) {
	task, err := scanTask(s.db.QueryRow(taskSelect+` WHERE task_id = ?`, taskID))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	arms, err := s.readTaskArms(taskID)
	if err != nil {
		return Task{}, err
	}
	task.Arms = arms
	return task, nil
}

// ListTasks returns a project's tasks newest first, each with its arms.
func (s *Store) ListTasks(project string) ([]Task, error) {
	rows, err := s.db.Query(taskSelect+` WHERE project = ? ORDER BY created_at DESC, task_id`, project)
	if err != nil {
		return nil, fmt.Errorf("state: list tasks: %w", err)
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate tasks: %w", err)
	}
	for i := range tasks {
		arms, err := s.readTaskArms(tasks[i].TaskID)
		if err != nil {
			return nil, err
		}
		tasks[i].Arms = arms
	}
	return tasks, nil
}

func (s *Store) readTaskArms(taskID string) ([]TaskArm, error) {
	rows, err := s.db.Query(`
SELECT arm_id, task_id, kind, source_kind, source_id, satisfying_outcomes, signal_name,
  state, satisfied_at
FROM task_arms WHERE task_id = ? ORDER BY arm_id`, taskID)
	if err != nil {
		return nil, fmt.Errorf("state: list task arms: %w", err)
	}
	defer rows.Close()
	arms := []TaskArm{}
	for rows.Next() {
		var arm TaskArm
		var outcomes string
		var satisfiedAt sql.NullString
		if err := rows.Scan(&arm.ArmID, &arm.TaskID, &arm.Kind, &arm.SourceKind, &arm.SourceID,
			&outcomes, &arm.SignalName, &arm.State, &satisfiedAt); err != nil {
			return nil, fmt.Errorf("state: scan task arm: %w", err)
		}
		arm.SatisfyingOutcomes = decodeOutcomeSet(outcomes)
		t, err := parseOptionalTime(satisfiedAt)
		if err != nil {
			return nil, wrapTimeErr("task arm satisfied_at", err)
		}
		arm.SatisfiedAt = t
		arms = append(arms, arm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate task arms: %w", err)
	}
	return arms, nil
}

const taskSelect = `
SELECT task_id, project, display_name, instruction, target_kind, target_agent_id, role, backend,
  model, state, outcome, outcome_source, outcome_summary, outcome_details, attention_reason,
  created_by_kind, created_by_agent_id, created_by_generation, assigned_agent_id,
  assigned_generation, runtime_claim, pending_release, start_attempt_id, start_attempt_count,
  revision, ready_at, start_claimed_at, created_at, updated_at, started_at, finished_at
FROM tasks`

func scanTask(row rowScanner) (Task, error) {
	var t Task
	var assignedAgentID sql.NullString
	var pendingRelease int
	var readyAt, startClaimedAt, startedAt, finishedAt sql.NullString
	var createdAt, updatedAt string
	if err := row.Scan(&t.TaskID, &t.Project, &t.DisplayName, &t.Instruction, &t.TargetKind,
		&t.TargetAgentID, &t.Role, &t.Backend, &t.Model, &t.State, &t.Outcome, &t.OutcomeSource,
		&t.OutcomeSummary, &t.OutcomeDetails, &t.AttentionReason, &t.CreatedByKind,
		&t.CreatedByAgentID, &t.CreatedByGeneration, &assignedAgentID, &t.AssignedGeneration,
		&t.RuntimeClaim, &pendingRelease, &t.StartAttemptID, &t.StartAttemptCount, &t.Revision,
		&readyAt, &startClaimedAt, &createdAt, &updatedAt, &startedAt, &finishedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, err
		}
		return Task{}, fmt.Errorf("state: scan task: %w", err)
	}
	t.AssignedAgentID = assignedAgentID.String
	t.PendingRelease = pendingRelease != 0

	var err error
	if t.CreatedAt, err = parseTime(createdAt); err != nil {
		return Task{}, wrapTimeErr("task created_at", err)
	}
	if t.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return Task{}, wrapTimeErr("task updated_at", err)
	}
	for _, opt := range []struct {
		name string
		raw  sql.NullString
		dst  **time.Time
	}{
		{"task ready_at", readyAt, &t.ReadyAt},
		{"task start_claimed_at", startClaimedAt, &t.StartClaimedAt},
		{"task started_at", startedAt, &t.StartedAt},
		{"task finished_at", finishedAt, &t.FinishedAt},
	} {
		v, err := parseOptionalTime(opt.raw)
		if err != nil {
			return Task{}, wrapTimeErr(opt.name, err)
		}
		*opt.dst = v
	}
	t.Arms = []TaskArm{}
	return t, nil
}

// RegisterWorkResult records one source's normalized terminal outcome. It is
// unique per source and immutable once written, so a second registration for the
// same source is a conflict rather than an overwrite (TS-10.R8).
func (s *Store) RegisterWorkResult(result WorkResult) error {
	return RegisterWorkResultTx(s.db, result, timeNow())
}

// execer is satisfied by both *sql.DB and *sql.Tx, so each domain registers its
// result inside its own terminal-state transaction (TS-10.R7).
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// RegisterWorkResultTx is the registration itself, callable from whichever
// transaction commits the domain's terminal state.
func RegisterWorkResultTx(db execer, result WorkResult, now time.Time) error {
	res, err := db.Exec(`
INSERT OR IGNORE INTO work_results(source_kind, source_id, outcome, raw_label, summary, recorded_at)
VALUES(?, ?, ?, ?, ?, ?)`,
		result.SourceKind, result.SourceID, result.Outcome, result.RawLabel, result.Summary,
		formatTime(now))
	if err != nil {
		return fmt.Errorf("state: register work result: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("state: register work result rows: %w", err)
	}
	if n == 0 {
		return ErrWorkResultRecorded
	}
	return nil
}

// ReadWorkResult returns one source's registered outcome.
func (s *Store) ReadWorkResult(sourceKind, sourceID string) (WorkResult, error) {
	var r WorkResult
	var recordedAt string
	err := s.db.QueryRow(`
SELECT source_kind, source_id, outcome, raw_label, summary, recorded_at
FROM work_results WHERE source_kind = ? AND source_id = ?`, sourceKind, sourceID).
		Scan(&r.SourceKind, &r.SourceID, &r.Outcome, &r.RawLabel, &r.Summary, &recordedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkResult{}, ErrNotFound
	}
	if err != nil {
		return WorkResult{}, fmt.Errorf("state: read work result: %w", err)
	}
	if r.RecordedAt, err = parseTime(recordedAt); err != nil {
		return WorkResult{}, wrapTimeErr("work result recorded_at", err)
	}
	return r, nil
}

// TaskUpdate carries the fields one compare-and-set may change. A nil pointer
// leaves that column alone.
type TaskUpdate struct {
	State           string
	Outcome         *string
	OutcomeSource   *string
	OutcomeSummary  *string
	OutcomeDetails  *string
	AttentionReason *string
	ReadyAt         *time.Time
	FinishedAt      *time.Time
}

// UpdateTaskCAS advances a task against the revision the caller observed. State
// advances monotonically under compare-and-set, so a stale writer loses rather
// than overwriting a newer transition (TS-10.R10). A finished task is terminal
// and never reopened.
func (s *Store) UpdateTaskCAS(taskID string, revision int, update TaskUpdate) (Task, error) {
	now := timeNow()
	sets := []string{"state = ?", "revision = revision + 1", "updated_at = ?"}
	args := []any{update.State, formatTime(now)}
	for _, col := range []struct {
		name  string
		value *string
	}{
		{"outcome", update.Outcome},
		{"outcome_source", update.OutcomeSource},
		{"outcome_summary", update.OutcomeSummary},
		{"outcome_details", update.OutcomeDetails},
		{"attention_reason", update.AttentionReason},
	} {
		if col.value != nil {
			sets = append(sets, col.name+" = ?")
			args = append(args, *col.value)
		}
	}
	if update.ReadyAt != nil {
		sets = append(sets, "ready_at = ?")
		args = append(args, formatTime(*update.ReadyAt))
	}
	if update.FinishedAt != nil {
		sets = append(sets, "finished_at = ?")
		args = append(args, formatTime(*update.FinishedAt))
	}
	args = append(args, taskID, revision)

	res, err := s.db.Exec(`UPDATE tasks SET `+strings.Join(sets, ", ")+`
WHERE task_id = ? AND revision = ? AND state != '`+TaskFinished+`'`, args...)
	if err != nil {
		return Task{}, fmt.Errorf("state: update task: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Task{}, fmt.Errorf("state: update task rows: %w", err)
	}
	if n == 0 {
		if _, readErr := s.ReadTask(taskID); errors.Is(readErr, ErrNotFound) {
			return Task{}, ErrNotFound
		}
		return Task{}, ErrTaskConflict
	}
	return s.ReadTask(taskID)
}

func encodeOutcomeSet(outcomes []string) string {
	if len(outcomes) == 0 {
		return ""
	}
	sorted := append([]string{}, outcomes...)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

func decodeOutcomeSet(raw string) []string {
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}
