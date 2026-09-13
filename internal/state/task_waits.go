package state

import (
	"database/sql"
	"errors"
	"fmt"
)

const MaxTaskWaitSources = 64

var (
	ErrTaskWaitConflict = errors.New("state: task wait conflict")
	ErrTaskWaitScope    = errors.New("state: task wait source unavailable")
)

type TaskWaitObservation struct {
	TaskID        string `json:"task_id"`
	AfterRevision int    `json:"after_revision"`
}

type TaskWaitChange struct {
	TaskID          string `json:"task_id"`
	Revision        int    `json:"revision"`
	State           string `json:"state"`
	Outcome         string `json:"outcome,omitempty"`
	AttentionReason string `json:"attention_reason,omitempty"`
}

// WaitForTasks atomically observes readable child work or records a durable
// watch and yield intent. Result registration and subscription therefore cannot
// cross without one side seeing the other's revision.
func (s *Store) WaitForTasks(agentID, generation, executionHandle string, observations []TaskWaitObservation) ([]TaskWaitChange, Task, error) {
	if len(observations) == 0 || len(observations) > MaxTaskWaitSources {
		return nil, Task{}, ErrTaskWaitConflict
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, Task{}, fmt.Errorf("state: begin task wait: %w", err)
	}
	defer tx.Rollback()
	// Registering a wait from a stage that has already closed would leave a
	// durable watch nothing may act on, so the same fence applies here as to the
	// wake it would later cause (TS-09.R42, TS-10.R29).
	waiting, err := scanTask(tx.QueryRow(taskSelect+` WHERE assigned_agent_id = ? AND assigned_generation = ? AND state = ? AND execution_handle = ?`+taskRunOpenClause, agentID, generation, TaskRunning, executionHandle))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, Task{}, ErrTaskWaitConflict
	}
	if err != nil {
		return nil, Task{}, err
	}
	changes := make([]TaskWaitChange, 0)
	seen := map[string]struct{}{}
	for _, observation := range observations {
		if observation.TaskID == "" || observation.AfterRevision < 0 || observation.TaskID == waiting.TaskID {
			return nil, Task{}, ErrTaskWaitConflict
		}
		if _, duplicate := seen[observation.TaskID]; duplicate {
			return nil, Task{}, ErrTaskWaitConflict
		}
		seen[observation.TaskID] = struct{}{}
		var revision int
		var taskState, outcome, attention, project, creator, targetAgent, assignedAgent string
		err = tx.QueryRow(`SELECT revision, state, outcome, attention_reason, project, created_by_agent_id,
  target_agent_id, COALESCE(assigned_agent_id, '') FROM tasks WHERE task_id = ?`, observation.TaskID).Scan(
			&revision, &taskState, &outcome, &attention, &project, &creator, &targetAgent, &assignedAgent)
		if errors.Is(err, sql.ErrNoRows) || project != waiting.Project {
			return nil, Task{}, ErrTaskWaitScope
		}
		if err != nil {
			return nil, Task{}, err
		}
		if creator != agentID {
			// A standing stage owner watches work it did not create: its managed
			// coordinator, and retained predecessor work after a replacement. That
			// authority comes from the live stage binding, not creation history
			// (TS-09.R39/R41/R49).
			managed, err := agentManagesPipelineWorkTx(tx, agentID, observation.TaskID)
			if err != nil {
				return nil, Task{}, err
			}
			if !managed {
				return nil, Task{}, ErrTaskWaitScope
			}
		}
		if targetAgent == agentID || assignedAgent == agentID {
			return nil, Task{}, ErrTaskWaitConflict
		}
		if revision > observation.AfterRevision || taskState == TaskFinished || taskState == TaskInterrupted || taskState == TaskDependencyFailed {
			changes = append(changes, TaskWaitChange{TaskID: observation.TaskID, Revision: revision, State: taskState, Outcome: outcome, AttentionReason: attention})
		}
	}
	if len(changes) > 0 {
		if err := tx.Commit(); err != nil {
			return nil, Task{}, err
		}
		return changes, waiting, nil
	}
	version := waiting.WaitVersion + 1
	for _, observation := range observations {
		if _, err := tx.Exec(`INSERT INTO task_waits(waiting_task_id, wait_version, source_task_id, after_revision) VALUES (?, ?, ?, ?)`, waiting.TaskID, version, observation.TaskID, observation.AfterRevision); err != nil {
			return nil, Task{}, err
		}
	}
	if _, err := tx.Exec(`UPDATE tasks SET pending_yield = 1, wait_version = ?, revision = revision + 1, updated_at = ? WHERE task_id = ? AND state = ? AND execution_handle = ?`, version, formatTime(timeNow()), waiting.TaskID, TaskRunning, executionHandle); err != nil {
		return nil, Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return nil, Task{}, err
	}
	updated, err := s.ReadTask(waiting.TaskID)
	return []TaskWaitChange{}, updated, err
}

// NotifyTaskWaiters coalesces a watched change. A task whose old runtime still
// owns capacity records resume_needed; a settled waiter becomes ready once.
func (s *Store) NotifyTaskWaiters(sourceTaskID string) ([]Task, error) {
	now := formatTime(timeNow())
	rows, err := s.db.Query(`SELECT DISTINCT waiting_task_id FROM task_waits WHERE source_task_id = ?`, sourceTaskID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	// A mid-scan failure otherwise looks exactly like the end of the list, and
	// the waiters it dropped would never be woken (INV §7).
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("state: iterate task waiters: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var changed []Task
	for _, id := range ids {
		// A watched change can fire long after the stage that authorized the wait
		// completed or its run was stopped. Waking the waiter then is the same
		// escape a ready child is, so it takes the same inherited-closure fence
		// (TS-09.R42, TS-10.R29). The durable watch row is untouched.
		res, err := s.db.Exec(`UPDATE tasks SET resume_needed = 1, state = CASE WHEN state = ? AND pending_yield = 0 THEN ? ELSE state END, continuation_pending = CASE WHEN state = ? AND pending_yield = 0 THEN 1 ELSE continuation_pending END, ready_at = CASE WHEN state = ? AND pending_yield = 0 THEN ? ELSE ready_at END, revision = revision + 1, updated_at = ? WHERE task_id = ? AND state IN (?, ?) AND resume_needed = 0`+taskRunOpenClause, TaskWaiting, TaskReady, TaskWaiting, TaskWaiting, now, now, id, TaskRunning, TaskWaiting)
		if err != nil {
			return nil, err
		}
		n, rowsErr := res.RowsAffected()
		if rowsErr != nil {
			return nil, rowsErr
		}
		if n == 1 {
			task, err := s.ReadTask(id)
			if err != nil {
				return nil, err
			}
			changed = append(changed, task)
		}
	}
	return changed, nil
}

// CompleteTaskYield drops runtime ownership after the reporting turn safely
// ends while preserving the assignment identity and durable watch.
func (s *Store) CompleteTaskYield(taskID string) (Task, error) {
	now := formatTime(timeNow())
	res, err := s.db.Exec(`UPDATE tasks SET state = CASE WHEN resume_needed = 1 THEN ? ELSE ? END, continuation_pending = resume_needed, pending_yield = 0, runtime_claim = '', assigned_generation = '', execution_handle = '', execution_turn = '', start_attempt_id = '', start_claimed_at = NULL, cleanup_phase = '', cleanup_effect_key = '', cleanup_failure_count = 0, cleanup_first_failure_at = NULL, cleanup_next_retry_at = NULL, cleanup_last_error = '', cleanup_unsafe = 0, attention_reason = CASE WHEN cleanup_phase <> '' THEN '' ELSE attention_reason END, ready_at = CASE WHEN resume_needed = 1 THEN ? ELSE ready_at END, revision = revision + 1, updated_at = ? WHERE task_id = ? AND pending_yield = 1`, TaskReady, TaskWaiting, now, now, taskID)
	if err != nil {
		return Task{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Task{}, ErrTaskWaitConflict
	}
	return s.ReadTask(taskID)
}

func (s *Store) PendingYieldTask(agentID, generation string) (Task, error) {
	task, err := scanTask(s.db.QueryRow(taskSelect+` WHERE assigned_agent_id = ? AND assigned_generation = ? AND pending_yield = 1`, agentID, generation))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}
