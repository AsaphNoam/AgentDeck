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

	if err := validateTaskArms(tx, task); err != nil {
		return Task{}, err
	}
	stamped, err := stampArmsFromResults(tx, task.Arms, now)
	if err != nil {
		return Task{}, err
	}
	task.Arms = stamped
	task.State = initialTaskState(task.Arms)
	if task.State == TaskReady {
		task.ReadyAt = &now
	}
	if task.State == TaskDependencyFailed {
		task.AttentionReason = parkedAttentionReason
	}

	if _, err := tx.Exec(`
INSERT INTO tasks(task_id, project, display_name, instruction, target_kind, target_agent_id,
  role, backend, model, state, attention_reason, created_by_kind, created_by_agent_id,
  created_by_generation, revision, ready_at, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.TaskID, task.Project, task.DisplayName, task.Instruction, task.TargetKind,
		task.TargetAgentID, task.Role, task.Backend, task.Model, task.State, task.AttentionReason,
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

// initialTaskState is armed while any arm is unsatisfied and ready when every
// one is satisfied; a task with no arms is ready the moment it is created
// (FS-16.R5). Parking wins over readiness, because an unsatisfiable arm can
// never be repaired in place (FS-16.R8).
func initialTaskState(arms []TaskArm) string {
	ready := true
	for _, arm := range arms {
		switch arm.State {
		case ArmUnsatisfiable:
			return TaskDependencyFailed
		case ArmSatisfied:
		default:
			ready = false
		}
	}
	if ready {
		return TaskReady
	}
	return TaskArmed
}

// parkedAttentionReason is the one wording for a task held by a prerequisite
// that can never be satisfied, wherever that is decided.
const parkedAttentionReason = "a prerequisite can no longer be satisfied"

// stampArmsFromResults decides each new arm's state from the results already
// registered. A task armed on work that has already finished must not wait for
// an event that has already happened: it is ready, or parked, the moment it is
// created or re-armed (FS-16.R5, R8, R23).
func stampArmsFromResults(tx *sql.Tx, arms []TaskArm, now time.Time) ([]TaskArm, error) {
	stamped := make([]TaskArm, len(arms))
	copy(stamped, arms)
	for i := range stamped {
		if stamped[i].Kind != ArmWorkResult {
			stamped[i].State = ArmUnsatisfied
			continue
		}
		var outcome string
		err := tx.QueryRow(`SELECT outcome FROM work_results WHERE source_kind = ? AND source_id = ?`,
			stamped[i].SourceKind, stamped[i].SourceID).Scan(&outcome)
		if errors.Is(err, sql.ErrNoRows) {
			stamped[i].State = ArmUnsatisfied
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("state: read registered result for arm: %w", err)
		}
		if containsString(stamped[i].SatisfyingOutcomes, outcome) {
			at := now
			stamped[i].State, stamped[i].SatisfiedAt = ArmSatisfied, &at
			continue
		}
		stamped[i].State = ArmUnsatisfiable
	}
	return stamped, nil
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

// EvaluateSource applies one source's registered result to every arm naming it,
// then advances the tasks those arms belong to. Evaluation is event-driven: a
// committed registration re-evaluates only the arms that name that source, never
// the whole graph (TS-10.R3). It reads the durable registration rather than
// taking an outcome from the caller, so it cannot disagree with the record and
// is safe to re-run — which is what the startup sweep does (TS-10.R15).
func (s *Store) EvaluateSource(sourceKind, sourceID string) ([]Task, error) {
	result, err := s.ReadWorkResult(sourceKind, sourceID)
	if errors.Is(err, ErrNotFound) {
		return []Task{}, nil
	}
	if err != nil {
		return nil, err
	}
	return s.resolveArms(func(tx *sql.Tx, now time.Time) error {
		// An arm is satisfied when the registered outcome is in its set and
		// unsatisfiable on any other terminal outcome — the two are decided in the
		// same pass so no arm is left waiting on a result that already arrived
		// (FS-16.R5, R8).
		if _, err := tx.Exec(`
UPDATE task_arms SET state = ?, satisfied_at = ?
WHERE kind = ? AND source_kind = ? AND source_id = ? AND state = ?
  AND instr(',' || satisfying_outcomes || ',', ',' || ? || ',') > 0`,
			ArmSatisfied, formatTime(now), ArmWorkResult, sourceKind, sourceID,
			ArmUnsatisfied, result.Outcome); err != nil {
			return fmt.Errorf("state: satisfy arms: %w", err)
		}
		if _, err := tx.Exec(`
UPDATE task_arms SET state = ?
WHERE kind = ? AND source_kind = ? AND source_id = ? AND state = ?`,
			ArmUnsatisfiable, ArmWorkResult, sourceKind, sourceID, ArmUnsatisfied); err != nil {
			return fmt.Errorf("state: park arms: %w", err)
		}
		return nil
	}, armsNamingSource(sourceKind, sourceID))
}

// MarkSourceUnsatisfiable parks every arm still waiting on a source that can no
// longer produce a result — a prerequisite task that was itself parked, or one
// that was deleted. An arm a result already satisfied is untouched, so deleting a
// finished prerequisite leaves its dependents completely unaffected (FS-16.R8,
// R18).
func (s *Store) MarkSourceUnsatisfiable(sourceKind, sourceID string) ([]Task, error) {
	return s.resolveArms(func(tx *sql.Tx, _ time.Time) error {
		if _, err := tx.Exec(`
UPDATE task_arms SET state = ?
WHERE kind = ? AND source_kind = ? AND source_id = ? AND state = ?`,
			ArmUnsatisfiable, ArmWorkResult, sourceKind, sourceID, ArmUnsatisfied); err != nil {
			return fmt.Errorf("state: park arms for source: %w", err)
		}
		return nil
	}, armsNamingSource(sourceKind, sourceID))
}

// FireSignal satisfies every signal arm in a project waiting on that name at this
// moment. A signal is not a stored object: firing a name no arm is waiting on
// succeeds and changes nothing, and an arm may name a signal never fired
// (FS-16.R9).
func (s *Store) FireSignal(project, name string) ([]Task, error) {
	owners := `
SELECT DISTINCT a.task_id FROM task_arms a JOIN tasks t ON t.task_id = a.task_id
WHERE a.kind = ? AND a.signal_name = ? AND t.project = ?`
	return s.resolveArms(func(tx *sql.Tx, now time.Time) error {
		if _, err := tx.Exec(`
UPDATE task_arms SET state = ?, satisfied_at = ?
WHERE kind = ? AND signal_name = ? AND state = ?
  AND task_id IN (SELECT task_id FROM tasks WHERE project = ?)`,
			ArmSatisfied, formatTime(now), ArmSignal, name, ArmUnsatisfied, project); err != nil {
			return fmt.Errorf("state: fire signal: %w", err)
		}
		return nil
	}, ownerQuery{sql: owners, args: []any{ArmSignal, name, project}})
}

type ownerQuery struct {
	sql  string
	args []any
}

func armsNamingSource(sourceKind, sourceID string) ownerQuery {
	return ownerQuery{
		sql: `SELECT DISTINCT task_id FROM task_arms
WHERE kind = ? AND source_kind = ? AND source_id = ?`,
		args: []any{ArmWorkResult, sourceKind, sourceID},
	}
}

// resolveArms runs one arm transition and the task advance it implies in a single
// transaction, so a task can never be observed with every arm satisfied while
// still armed. It returns the tasks whose state actually changed.
func (s *Store) resolveArms(transition func(*sql.Tx, time.Time) error, owners ownerQuery) ([]Task, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("state: begin arm evaluation: %w", err)
	}
	defer tx.Rollback()

	taskIDs, err := queryTaskIDs(tx, owners)
	if err != nil {
		return nil, err
	}
	now := timeNow()
	if err := transition(tx, now); err != nil {
		return nil, err
	}
	changed, err := advanceArmedTasks(tx, taskIDs, now)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("state: commit arm evaluation: %w", err)
	}

	tasks := []Task{}
	for _, id := range changed {
		task, err := s.ReadTask(id)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func queryTaskIDs(tx *sql.Tx, owners ownerQuery) ([]string, error) {
	rows, err := tx.Query(owners.sql, owners.args...)
	if err != nil {
		return nil, fmt.Errorf("state: list arm owners: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("state: scan arm owner: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate arm owners: %w", err)
	}
	return ids, nil
}

// advanceArmedTasks applies both promotions as single conditional statements, so
// availability and the transition are decided together and two evaluators racing
// on one task cannot both advance it (TS-10.R4, INV §5). Parking wins over
// readiness: an unsatisfiable arm can never be repaired in place, so a task
// carrying one is never ready however many others are satisfied (FS-16.R8).
func advanceArmedTasks(tx *sql.Tx, taskIDs []string, now time.Time) ([]string, error) {
	stamp := formatTime(now)
	var changed []string
	for _, id := range taskIDs {
		res, err := tx.Exec(`
UPDATE tasks SET state = ?, attention_reason = ?, revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state IN (?, ?)
  AND EXISTS (SELECT 1 FROM task_arms WHERE task_id = tasks.task_id AND state = ?)`,
			TaskDependencyFailed, parkedAttentionReason, stamp,
			id, TaskArmed, TaskReady, ArmUnsatisfiable)
		if err != nil {
			return nil, fmt.Errorf("state: park task: %w", err)
		}
		parked, err := res.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("state: park task rows: %w", err)
		}
		if parked > 0 {
			changed = append(changed, id)
			continue
		}
		res, err = tx.Exec(`
UPDATE tasks SET state = ?, ready_at = ?, revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state = ?
  AND NOT EXISTS (SELECT 1 FROM task_arms WHERE task_id = tasks.task_id AND state != ?)`,
			TaskReady, stamp, stamp, id, TaskArmed, ArmSatisfied)
		if err != nil {
			return nil, fmt.Errorf("state: ready task: %w", err)
		}
		ready, err := res.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("state: ready task rows: %w", err)
		}
		if ready > 0 {
			changed = append(changed, id)
		}
	}
	return changed, nil
}

// MaxTaskStartAttempts bounds how many times a task's start may genuinely fail
// before it parks. A deferral is not a failure and spends none of them
// (FS-16.R25).
const MaxTaskStartAttempts = 3

// TaskReservation is what a start attempt records durably before any launch,
// resume, or prompt happens. It identifies the attempt for recovery and holds
// the exclusive assignment claim; it authorizes nothing on its own, and only
// confirmation turns it into the assignee membership that authorizes attached
// context reads (TS-10.R4, FS-16 §3).
type TaskReservation struct {
	AttemptID  string
	AgentID    string
	Generation string
	// Claim is created, woke, or borrowed: whether this start will bring the
	// runtime up, wake a stopped one, or use one that is already up for someone
	// else's reasons. Only the first two consume a budget slot and only the first
	// two are stopped when the task finishes (FS-16.R4, R7).
	Claim string
}

// ReadyTasks lists tasks waiting for admission in the order they became ready,
// at most limit of them. The limit is the caller's: whatever one dispatch pass
// leaves is still ready for the next one.
func (s *Store) ReadyTasks(limit int) ([]Task, error) {
	rows, err := s.db.Query(taskSelect+`
WHERE state = ? ORDER BY ready_at, task_id LIMIT ?`, TaskReady, limit)
	if err != nil {
		return nil, fmt.Errorf("state: list ready tasks: %w", err)
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
		return nil, fmt.Errorf("state: iterate ready tasks: %w", err)
	}
	return tasks, nil
}

// AdmitReadyTask moves one ready task to starting, taking capacity and the
// exclusive assignment claim in the same statement that records the reservation
// (TS-10.R4, R17, R18, INV §5). Deciding availability and claiming separately
// would let two dispatch passes each see a free slot, or two tasks each see the
// same agent unassigned, and both commit.
//
// It reports false with no error when the task was not admitted: it is no longer
// ready, the budget is full, or another task already holds its target agent.
// None of those is a failed start attempt — the task is still ready
// with its allowance and its place in the admission order intact (FS-16.R25) —
// so they are one answer rather than distinct errors.
func (s *Store) AdmitReadyTask(taskID string, reservation TaskReservation, budget int) (Task, bool, error) {
	now := timeNow()
	stamp := formatTime(now)
	res, err := s.db.Exec(`
UPDATE tasks
SET state = ?, assigned_agent_id = ?, assigned_generation = ?, runtime_claim = ?,
  start_attempt_id = ?, start_claimed_at = ?, revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state = ?
  AND (? = ? OR (
    SELECT COUNT(*) FROM tasks AS live
    WHERE live.runtime_claim IN (?, ?) AND live.state IN (?, ?)) < ?)`,
		TaskStarting, reservation.AgentID, reservation.Generation, reservation.Claim,
		reservation.AttemptID, stamp, stamp,
		taskID, TaskReady,
		reservation.Claim, ClaimBorrowed,
		ClaimCreated, ClaimWoke, TaskStarting, TaskRunning, budget)
	if isUniqueViolation(err) {
		// The partial unique index over an active assignee refused the claim:
		// another task is already starting or running on this agent (TS-10.R18).
		// The loser stays ready and nothing was written.
		return Task{}, false, nil
	}
	if err != nil {
		return Task{}, false, fmt.Errorf("state: admit ready task: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Task{}, false, fmt.Errorf("state: admit ready task rows: %w", err)
	}
	if n == 0 {
		return Task{}, false, nil
	}
	task, err := s.ReadTask(taskID)
	return task, err == nil, err
}

// ConfirmTaskStart promotes a reservation into durable assignee membership once
// the assignment has crossed into a live runtime: only then is the task running
// (TS-10.R4, FS-16.R6). The attempt id is required, so a start abandoned and
// re-admitted cannot be confirmed by its predecessor's late success.
func (s *Store) ConfirmTaskStart(taskID, attemptID string) (Task, bool, error) {
	return s.settleTaskStart(taskID, attemptID, `
UPDATE tasks SET state = ?, started_at = ?, revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state = ? AND start_attempt_id = ?`,
		[]any{TaskRunning, formatTime(timeNow())}, "confirm task start")
}

// AbandonTaskStart returns a task whose start produced no confirmed runtime to
// ready, releasing its assignment claim and its slot and spending no attempt.
// This is the deferral path — a lost lifecycle claim, a start the caller gave up
// before any effect, or a recovery sweep resolving a reservation it reaped
// (FS-16.R25, TS-10.R15).
func (s *Store) AbandonTaskStart(taskID, attemptID string) (Task, bool, error) {
	return s.settleTaskStart(taskID, attemptID, `
UPDATE tasks SET state = ?, assigned_agent_id = NULL, assigned_generation = '',
  runtime_claim = '', start_attempt_id = '', start_claimed_at = NULL,
  revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state = ? AND start_attempt_id = ?`,
		[]any{TaskReady}, "abandon task start")
}

// FailTaskStart spends one attempt on a start that genuinely failed — a launch
// that did not start, a resume that did not complete, or a target that became
// ineligible — and returns the task to ready. When the last attempt is spent the
// task parks as dependency_failed recording that failure, so a start can never
// retry forever and never fails silently (FS-16.R8, R25, INV §8).
func (s *Store) FailTaskStart(taskID, attemptID, reason string) (Task, bool, error) {
	return s.settleTaskStart(taskID, attemptID, `
UPDATE tasks SET
  state = CASE WHEN start_attempt_count + 1 >= ? THEN ? ELSE ? END,
  attention_reason = CASE WHEN start_attempt_count + 1 >= ? THEN ? ELSE attention_reason END,
  start_attempt_count = start_attempt_count + 1,
  assigned_agent_id = NULL, assigned_generation = '', runtime_claim = '',
  start_attempt_id = '', start_claimed_at = NULL,
  revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state = ? AND start_attempt_id = ?`,
		[]any{MaxTaskStartAttempts, TaskDependencyFailed, TaskReady,
			MaxTaskStartAttempts, reason}, "fail task start")
}

// ParkTaskStart parks a task whose target can never become eligible — a deleted,
// archived, or terminal-interface agent. Retrying that is not a repair, so it
// spends no attempt and goes straight to dependency_failed with the reason
// (FS-16.R8, R19).
func (s *Store) ParkTaskStart(taskID, attemptID, reason string) (Task, bool, error) {
	return s.settleTaskStart(taskID, attemptID, `
UPDATE tasks SET state = ?, attention_reason = ?, assigned_agent_id = NULL,
  assigned_generation = '', runtime_claim = '', start_attempt_id = '',
  start_claimed_at = NULL, revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state = ? AND start_attempt_id = ?`,
		[]any{TaskDependencyFailed, reason}, "park task start")
}

// settleTaskStart runs one conditional statement that resolves a starting row,
// guarded by the attempt id that reserved it. A statement that matches nothing
// means the attempt is no longer the task's current one, which is an outcome
// rather than an error: a late worker must never overwrite a newer transition.
func (s *Store) settleTaskStart(taskID, attemptID, stmt string, leading []any, what string) (Task, bool, error) {
	args := append(append([]any{}, leading...), formatTime(timeNow()), taskID, TaskStarting, attemptID)
	res, err := s.db.Exec(stmt, args...)
	if err != nil {
		return Task{}, false, fmt.Errorf("state: %s: %w", what, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Task{}, false, fmt.Errorf("state: %s rows: %w", what, err)
	}
	if n == 0 {
		return Task{}, false, nil
	}
	task, err := s.ReadTask(taskID)
	return task, err == nil, err
}

// AssignedTask returns the one task this agent is executing — starting or
// running — or ErrNotFound. The partial unique index guarantees there is at most
// one, so this needs no ordering or tie-break (FS-16.R2, TS-10.R18).
func (s *Store) AssignedTask(agentID string) (Task, error) {
	task, err := scanTask(s.db.QueryRow(taskSelect+`
WHERE assigned_agent_id = ? AND state IN (?, ?)`, agentID, TaskStarting, TaskRunning))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	arms, err := s.readTaskArms(task.TaskID)
	if err != nil {
		return Task{}, err
	}
	task.Arms = arms
	return task, nil
}

// ListTaskAttachments returns a task's context attachments, oldest first. The
// attachment holds only the canonical reference id and its own presentation: it
// authorizes nothing and copies no content (FS-16.R10, TS-10.R12).
func (s *Store) ListTaskAttachments(taskID string) ([]TaskAttachment, error) {
	rows, err := s.db.Query(`
SELECT task_id, context_ref_id, label, description, created_at
FROM task_attachments WHERE task_id = ? ORDER BY created_at, context_ref_id`, taskID)
	if err != nil {
		return nil, fmt.Errorf("state: list task attachments: %w", err)
	}
	defer rows.Close()
	out := []TaskAttachment{}
	for rows.Next() {
		var a TaskAttachment
		var createdAt string
		if err := rows.Scan(&a.TaskID, &a.ContextRefID, &a.Label, &a.Description, &createdAt); err != nil {
			return nil, fmt.Errorf("state: scan task attachment: %w", err)
		}
		if a.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, wrapTimeErr("task attachment created_at", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate task attachments: %w", err)
	}
	return out, nil
}

// ErrTaskNotAssigned is returned when the caller does not hold the assignment it
// is reporting on — a different agent, or the same agent's earlier generation.
var ErrTaskNotAssigned = errors.New("state: caller does not hold this assignment")

// ErrTaskNotReportable is returned when a task is in no state to take a result.
var ErrTaskNotReportable = errors.New("state: task cannot take a result in this state")

// TaskResult is one recorded outcome for a task.
type TaskResult struct {
	Outcome string
	Summary string
	Details string
}

// RecordAgentTaskResult commits the assignee's result and a durable intent to
// release its runtime in one transaction, and registers the outcome in the
// shared result layer in the same commit (TS-10.R7, R19, FS-16.R3).
//
// The stop is deliberately not part of it: stopping a process cannot be
// transactional, and the reporting agent is still waiting on this call's own
// response. The claim and the budget slot stay held until the reporting turn
// ends, and because the intent is durable a crash in between cannot strand a
// live task-owned runtime (INV §15).
func (s *Store) RecordAgentTaskResult(taskID, agentID, generation string, result TaskResult) (Task, error) {
	if err := ValidateAgentReport(result.Outcome, result.Summary, result.Details, ""); err != nil {
		return Task{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Task{}, fmt.Errorf("state: begin record task result: %w", err)
	}
	defer tx.Rollback()

	var taskState, assignedAgent, assignedGeneration string
	err = tx.QueryRow(`
SELECT state, COALESCE(assigned_agent_id, ''), assigned_generation
FROM tasks WHERE task_id = ?`, taskID).Scan(&taskState, &assignedAgent, &assignedGeneration)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("state: read task for result: %w", err)
	}
	if !OwnsReportedWork(agentID, generation, assignedAgent, assignedGeneration) {
		return Task{}, ErrTaskNotAssigned
	}
	// A result is accepted from the assignment's own states only. A finished task
	// is immutable, and an interrupted one has no live assignee to report it
	// (FS-16.R3, R22).
	if taskState != TaskStarting && taskState != TaskRunning {
		return Task{}, ErrTaskNotReportable
	}
	now := timeNow()
	stamp := formatTime(now)
	if _, err := tx.Exec(`
UPDATE tasks SET state = ?, outcome = ?, outcome_source = ?, outcome_summary = ?,
  outcome_details = ?, attention_reason = '', pending_release = 1, finished_at = ?,
  revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state = ?`,
		TaskFinished, result.Outcome, "agent", result.Summary, result.Details,
		stamp, stamp, taskID, taskState); err != nil {
		return Task{}, fmt.Errorf("state: record task result: %w", err)
	}
	if err := RegisterWorkResultTx(tx, WorkResult{
		SourceKind: SourceTask, SourceID: taskID, Outcome: result.Outcome,
		Summary: result.Summary,
	}, now); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("state: commit task result: %w", err)
	}
	return s.ReadTask(taskID)
}

// PendingReleaseTask returns the task whose release this agent's generation owes,
// or ErrNotFound. The generation matters: a resumed agent is a new runtime, and
// the release belonged to the one that reported (TS-10.R19).
func (s *Store) PendingReleaseTask(agentID, generation string) (Task, error) {
	task, err := scanTask(s.db.QueryRow(taskSelect+`
WHERE assigned_agent_id = ? AND assigned_generation = ? AND pending_release = 1`,
		agentID, generation))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	return task, nil
}

// CompleteTaskRelease clears the runtime claim and the release intent once the
// stop has actually happened or the runtime has been reaped. Until it does, the
// intent stands and recovery finishes it, so no terminal state ever discards
// ownership of a live runtime (TS-10.R19, INV §15).
func (s *Store) CompleteTaskRelease(taskID string) error {
	if _, err := s.db.Exec(`
UPDATE tasks SET pending_release = 0, runtime_claim = '', revision = revision + 1,
  updated_at = ?
WHERE task_id = ? AND pending_release = 1`, formatTime(timeNow()), taskID); err != nil {
		return fmt.Errorf("state: complete task release: %w", err)
	}
	return nil
}

// InterruptTaskForAgent moves the task this agent generation was executing to
// interrupted, releasing its runtime claim and its budget slot. An agent that
// exits, crashes, is stopped, or has its runtime switched without recording an
// outcome leaves work interrupted, never successful and never failed: AgentDeck
// does not convert a process event into a result (FS-16.R16).
//
// The generation is required, so a resumed agent's exit cannot interrupt work
// assigned to the runtime that replaced it. A task that already recorded a
// result is untouched, which is what makes the stop that follows a report safe.
func (s *Store) InterruptTaskForAgent(agentID, generation, reason string) (Task, bool, error) {
	return s.interrupt(reason, `
WHERE assigned_agent_id = ? AND assigned_generation = ? AND state IN (?, ?)`,
		[]any{agentID, generation, TaskStarting, TaskRunning})
}

// InterruptTask interrupts one task by id, for a recovery sweep that already
// knows which rows it is resolving (TS-10.R15).
func (s *Store) InterruptTask(taskID, reason string) (Task, bool, error) {
	return s.interrupt(reason, `WHERE task_id = ? AND state IN (?, ?)`,
		[]any{taskID, TaskStarting, TaskRunning})
}

func (s *Store) interrupt(reason, where string, args []any) (Task, bool, error) {
	var taskID string
	err := s.db.QueryRow(`SELECT task_id FROM tasks `+where, args...).Scan(&taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, false, nil
	}
	if err != nil {
		return Task{}, false, fmt.Errorf("state: find task to interrupt: %w", err)
	}
	res, err := s.db.Exec(`
UPDATE tasks SET state = ?, attention_reason = ?, assigned_generation = '',
  runtime_claim = '', start_attempt_id = '', start_claimed_at = NULL,
  revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state IN (?, ?)`,
		TaskInterrupted, reason, formatTime(timeNow()), taskID, TaskStarting, TaskRunning)
	if err != nil {
		return Task{}, false, fmt.Errorf("state: interrupt task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Task{}, false, nil
	}
	task, err := s.ReadTask(taskID)
	return task, err == nil, err
}

// TasksInStates lists every task in one of the given states, oldest first. It is
// the startup sweep's bounded read over unfinished work (TS-10.R15).
func (s *Store) TasksInStates(states ...string) ([]Task, error) {
	if len(states) == 0 {
		return []Task{}, nil
	}
	args := make([]any, 0, len(states))
	for _, st := range states {
		args = append(args, st)
	}
	rows, err := s.db.Query(taskSelect+`
WHERE state IN (`+placeholders(len(states))+`) ORDER BY created_at, task_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("state: list tasks by state: %w", err)
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
		return nil, fmt.Errorf("state: iterate tasks by state: %w", err)
	}
	return tasks, nil
}

// TasksAwaitingRelease lists tasks whose recorded result committed a release its
// stop never completed, so recovery can finish exactly what was promised
// (TS-10.R19, FS-16.R17).
func (s *Store) TasksAwaitingRelease() ([]Task, error) {
	rows, err := s.db.Query(taskSelect + ` WHERE pending_release = 1 ORDER BY finished_at, task_id`)
	if err != nil {
		return nil, fmt.Errorf("state: list tasks awaiting release: %w", err)
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
		return nil, fmt.Errorf("state: iterate tasks awaiting release: %w", err)
	}
	return tasks, nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// AttachTaskContext binds canonical context references to a task. The attachment
// holds only the reference id and this task's own presentation: it copies no
// content, duplicates no reference, and creates no direct grant (FS-16.R10,
// TS-10.R12).
func (s *Store) AttachTaskContext(taskID string, attachments []TaskAttachment) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin attach task context: %w", err)
	}
	defer tx.Rollback()
	now := formatTime(timeNow())
	for _, a := range attachments {
		if _, err := tx.Exec(`
INSERT OR REPLACE INTO task_attachments(task_id, context_ref_id, label, description, created_at)
VALUES(?, ?, ?, ?, ?)`, taskID, a.ContextRefID, a.Label, a.Description, now); err != nil {
			return fmt.Errorf("state: attach task context: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit attach task context: %w", err)
	}
	return nil
}

// ErrTaskHoldsRuntime is returned when deletion is refused because the task
// still owns a runtime claim or an unfinished release (FS-16.R18, TS-10.R16).
var ErrTaskHoldsRuntime = errors.New("state: task still owns a runtime or an unfinished release")

// ErrRetryRequiresRearm is returned for a retry on a task parked by an arm that
// can never be satisfied. A recorded result is immutable, so retrying the same
// arms would park it again at once: the repair is re-arming (FS-16.R23).
var ErrRetryRequiresRearm = errors.New("state: this task is parked by an unsatisfiable prerequisite; re-arm it instead")

// ErrTaskNotRetryable is returned for a retry on a task whose state has no
// execution to re-attempt.
var ErrTaskNotRetryable = errors.New("state: task cannot be retried in this state")

// ErrTaskNotRearmable is returned for a re-arm on a task whose arms have already
// been passed or are moot (FS-16.R23).
var ErrTaskNotRearmable = errors.New("state: task arms cannot be replaced in this state")

// CancelTask writes the one host-owned outcome. It is accepted in every
// non-terminal state, commits the terminal state together with a durable release
// intent while the claim is still held, and registers `cancelled` so a dependent
// armed on this task is resolved rather than left waiting (FS-16.R3, R8,
// TS-10.R19).
func (s *Store) CancelTask(taskID string) (Task, error) {
	return s.finishTask(taskID, TaskResult{Outcome: OutcomeCancelled, Summary: "cancelled"}, "",
		[]string{TaskArmed, TaskReady, TaskStarting, TaskRunning, TaskInterrupted, TaskDependencyFailed})
}

// RecordPersonTaskResult is the only non-cancelling way to resolve work whose
// agent went away. It is the counterpart to the agent's report tool rather than
// an override of it, so it is accepted only where no agent will report: on a
// running or interrupted task (FS-16.R22).
func (s *Store) RecordPersonTaskResult(taskID string, result TaskResult) (Task, error) {
	if err := ValidateAgentReport(result.Outcome, result.Summary, result.Details, ""); err != nil {
		return Task{}, err
	}
	return s.finishTask(taskID, result, "person", []string{TaskRunning, TaskInterrupted})
}

// finishTask commits one terminal transition, its release intent, and its result
// registration together. The release intent is set only when the task still owns
// a runtime, so a task that never started leaves nothing to release.
func (s *Store) finishTask(taskID string, result TaskResult, source string, from []string) (Task, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Task{}, fmt.Errorf("state: begin finish task: %w", err)
	}
	defer tx.Rollback()

	var taskState, claim string
	err = tx.QueryRow(`SELECT state, runtime_claim FROM tasks WHERE task_id = ?`, taskID).
		Scan(&taskState, &claim)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("state: read task to finish: %w", err)
	}
	if !containsString(from, taskState) {
		return Task{}, ErrTaskNotReportable
	}
	now := timeNow()
	stamp := formatTime(now)
	release := 0
	if claim != "" {
		release = 1
	}
	if _, err := tx.Exec(`
UPDATE tasks SET state = ?, outcome = ?, outcome_source = ?, outcome_summary = ?,
  outcome_details = ?, attention_reason = '', pending_release = ?, finished_at = ?,
  revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state = ?`,
		TaskFinished, result.Outcome, source, result.Summary, result.Details,
		release, stamp, stamp, taskID, taskState); err != nil {
		return Task{}, fmt.Errorf("state: finish task: %w", err)
	}
	if err := RegisterWorkResultTx(tx, WorkResult{
		SourceKind: SourceTask, SourceID: taskID, Outcome: result.Outcome, Summary: result.Summary,
	}, now); err != nil && !errors.Is(err, ErrWorkResultRecorded) {
		return Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("state: commit finish task: %w", err)
	}
	return s.ReadTask(taskID)
}

// RetryTask re-attempts execution without touching arms. It restores the full
// start-attempt allowance and returns the task to ready, keeping the assignee it
// already confirmed so its transcript and its attached-context membership stay
// continuous (FS-16.R23, R25).
func (s *Store) RetryTask(taskID string) (Task, error) {
	task, err := s.ReadTask(taskID)
	if err != nil {
		return Task{}, err
	}
	switch task.State {
	case TaskInterrupted:
	case TaskDependencyFailed:
		// The two parked reasons need different repairs, and the arms say which
		// one this is without a second column to keep honest.
		for _, arm := range task.Arms {
			if arm.State == ArmUnsatisfiable {
				return Task{}, ErrRetryRequiresRearm
			}
		}
	default:
		return Task{}, ErrTaskNotRetryable
	}
	now := timeNow()
	stamp := formatTime(now)
	res, err := s.db.Exec(`
UPDATE tasks SET state = ?, start_attempt_count = 0, attention_reason = '',
  ready_at = ?, revision = revision + 1, updated_at = ?
WHERE task_id = ? AND state = ?`, TaskReady, stamp, stamp, taskID, task.State)
	if err != nil {
		return Task{}, fmt.Errorf("state: retry task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Task{}, ErrTaskConflict
	}
	return s.ReadTask(taskID)
}

// RearmTask replaces a task's whole arm set atomically, revalidating the graph
// inside the same transaction. An unsatisfiable arm is never repaired in place,
// so replacing the set is the repair (FS-16.R15, R23).
func (s *Store) RearmTask(taskID string, arms []TaskArm) (Task, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Task{}, fmt.Errorf("state: begin rearm task: %w", err)
	}
	defer tx.Rollback()

	var taskState, project string
	err = tx.QueryRow(`SELECT state, project FROM tasks WHERE task_id = ?`, taskID).Scan(&taskState, &project)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("state: read task to rearm: %w", err)
	}
	if taskState != TaskArmed && taskState != TaskReady && taskState != TaskDependencyFailed {
		return Task{}, ErrTaskNotRearmable
	}
	if _, err := tx.Exec(`DELETE FROM task_arms WHERE task_id = ?`, taskID); err != nil {
		return Task{}, fmt.Errorf("state: clear task arms: %w", err)
	}
	candidate := Task{TaskID: taskID, Project: project, Arms: arms}
	if err := validateTaskArms(tx, candidate); err != nil {
		return Task{}, err
	}
	now := timeNow()
	stamped, err := stampArmsFromResults(tx, arms, now)
	if err != nil {
		return Task{}, err
	}
	if err := insertTaskArms(tx, taskID, stamped); err != nil {
		return Task{}, err
	}
	stamp := formatTime(now)
	next := initialTaskState(stamped)
	readyAt := any(nil)
	if next == TaskReady {
		readyAt = stamp
	}
	reason := ""
	if next == TaskDependencyFailed {
		reason = parkedAttentionReason
	}
	if _, err := tx.Exec(`
UPDATE tasks SET state = ?, attention_reason = ?, ready_at = ?, revision = revision + 1,
  updated_at = ?
WHERE task_id = ? AND state = ?`, next, reason, readyAt, stamp, taskID, taskState); err != nil {
		return Task{}, fmt.Errorf("state: rearm task: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("state: commit rearm task: %w", err)
	}
	return s.ReadTask(taskID)
}

// DeleteTask removes a task and, in the same transaction, resolves the arms that
// were still waiting on it. Deletion is refused in the statement that checks it
// while the task owns a runtime claim or an unfinished release, so the row — the
// only record of that claim — outlives anything depending on it (FS-16.R18,
// TS-10.R16, INV §4/§15).
//
// A result the task already registered is deliberately not removed: it is keyed
// to its source and outlives the task, so a dependent whose arm it already
// satisfied is completely unaffected, whatever state that dependent is in.
func (s *Store) DeleteTask(taskID string) ([]Task, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("state: begin delete task: %w", err)
	}
	defer tx.Rollback()

	dependents, err := queryTaskIDs(tx, armsNamingSource(SourceTask, taskID))
	if err != nil {
		return nil, err
	}
	res, err := tx.Exec(`
DELETE FROM tasks
WHERE task_id = ? AND state NOT IN (?, ?) AND pending_release = 0`,
		taskID, TaskStarting, TaskRunning)
	if err != nil {
		return nil, fmt.Errorf("state: delete task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// The check has to run on this transaction's own connection: the store
		// keeps a single connection, so reading through it while this transaction
		// is open would wait for a connection this goroutine is holding.
		var exists int
		switch err := tx.QueryRow(`SELECT 1 FROM tasks WHERE task_id = ?`, taskID).Scan(&exists); {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		case err != nil:
			return nil, fmt.Errorf("state: read task after refused delete: %w", err)
		}
		return nil, ErrTaskHoldsRuntime
	}
	// Only an arm still waiting on it becomes unsatisfiable; one its result
	// already satisfied stays satisfied (FS-16.R18).
	if _, err := tx.Exec(`
UPDATE task_arms SET state = ?
WHERE kind = ? AND source_kind = ? AND source_id = ? AND state = ?`,
		ArmUnsatisfiable, ArmWorkResult, SourceTask, taskID, ArmUnsatisfied); err != nil {
		return nil, fmt.Errorf("state: park arms for deleted task: %w", err)
	}
	changed, err := advanceArmedTasks(tx, dependents, timeNow())
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("state: commit delete task: %w", err)
	}
	parked := []Task{}
	for _, id := range changed {
		task, err := s.ReadTask(id)
		if err != nil {
			return nil, err
		}
		parked = append(parked, task)
	}
	return parked, nil
}

func containsString(all []string, want string) bool {
	for _, v := range all {
		if v == want {
			return true
		}
	}
	return false
}
