package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agentdeck/agentdeck/internal/strutil"
)

// Background terminals are normalized async tasks, never AgentDeck durable
// tasks (FS-03.R59, TS-04.R64). The adapter announces a command that continues
// after its tool call and later reports one terminal state; its output stays
// with the related tool call. Runtime alone keeps the provider's session and
// task ids and resolves a targeted stop through them.

// EvBackgroundTaskState is one durable lifecycle record for a background task.
const EvBackgroundTaskState = "background_task_state"

const (
	asyncTaskStopMethod = "_session/async_task/stop"
	maxOpenTasks        = 128
	maxTerminalTasks    = 32
	maxTaskName         = 200
	taskStopTimeout     = 10 * time.Second
)

var (
	// ErrBackgroundTaskControlUnavailable: this session negotiated no targeted
	// background-task stop, or its runtime has none (TS-03.R44).
	ErrBackgroundTaskControlUnavailable = errors.New("runtime: background task control is not available for this agent")
	// ErrUnknownBackgroundTask: no running task of that id in this generation.
	ErrUnknownBackgroundTask = errors.New("runtime: no running background task with that id")
	// ErrBackgroundTaskStopRefused: the runtime declined or failed the stop; the
	// task keeps its last state and the request may be retried.
	ErrBackgroundTaskStopRefused = errors.New("runtime: the agent could not stop that background task")
)

// BackgroundTaskData is a background task's lifecycle payload. TaskID and
// ToolCallID are AgentDeck-normalized (activity-scoped for a child).
type BackgroundTaskData struct {
	TaskID     string `json:"task_id"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
	State      string `json:"state"` // running | completed | failed | stopped
	CanStop    bool   `json:"can_stop,omitempty"`
}

type taskRef struct {
	sessionID string // provider session owning the task (root or child)
	rawID     string
	scope     activityScope
	running   bool
}

// onAsyncTaskUpdate handles async-task lifecycle updates, reporting whether
// params was one. A duplicate (replayed) announcement or terminal update is
// dropped so one provider task yields one row per state (INV §16, FS-05.R38).
func (c *ChatRuntime) onAsyncTaskUpdate(as *agentState, scope activityScope, params json.RawMessage) bool {
	var su struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			SessionUpdate string `json:"sessionUpdate"`
			AsyncTaskID   string `json:"asyncTaskId"`
			ToolCallID    string `json:"toolCallId"`
			Name          string `json:"name"`
			State         string `json:"state"`
			CanStop       bool   `json:"canStop"`
		} `json:"update"`
	}
	if json.Unmarshal(params, &su) != nil {
		return false
	}
	u := su.Update
	if u.SessionUpdate != "async_task_spawned" && u.SessionUpdate != "async_task_state_update" {
		return false
	}
	if u.AsyncTaskID == "" {
		return true
	}
	id := scope.toolCallID(u.AsyncTaskID)
	data := BackgroundTaskData{TaskID: id, ToolCallID: scope.toolCallID(u.ToolCallID)}

	as.mu.Lock()
	ref, known := as.tasks[id]
	switch u.SessionUpdate {
	case "async_task_spawned":
		if known || as.openTasks >= maxOpenTasks {
			as.mu.Unlock()
			return true
		}
		if as.tasks == nil {
			as.tasks = map[string]*taskRef{}
		}
		as.tasks[id] = &taskRef{sessionID: su.SessionID, rawID: u.AsyncTaskID, scope: scope, running: true}
		as.openTasks++
		data.State, data.Name, data.CanStop = "running", strutil.ClipRunes(u.Name, maxTaskName), u.CanStop
	default:
		state, terminal := taskStateFor(u.State)
		if !known || !ref.running || !terminal {
			as.mu.Unlock()
			return true
		}
		ref.running = false
		as.openTasks--
		as.finishedTasks = append(as.finishedTasks, id)
		if len(as.finishedTasks) > maxTerminalTasks {
			delete(as.tasks, as.finishedTasks[0])
			as.finishedTasks = as.finishedTasks[1:]
		}
		data.State = state
	}
	as.mu.Unlock()
	c.emitIn(as, scope, EvBackgroundTaskState, data)
	return true
}

func taskStateFor(state string) (string, bool) {
	switch state {
	case "completed", "failed", "stopped":
		return state, true
	}
	return "", false
}

// StopBackgroundTask asks the runtime to stop one running task. Acceptance
// emits nothing: the runtime's own terminal update is the truth (TS-04.R64).
func (c *ChatRuntime) StopBackgroundTask(ctx context.Context, agentID, taskID string) error {
	as, err := c.lookup(agentID)
	if err != nil {
		return err
	}
	if !as.capabilities().BackgroundTaskStop {
		return ErrBackgroundTaskControlUnavailable
	}
	as.mu.Lock()
	ref, ok := as.tasks[taskID]
	var sessionID, rawID string
	if ok && ref.running {
		sessionID, rawID = ref.sessionID, ref.rawID
	}
	as.mu.Unlock()
	if rawID == "" {
		return ErrUnknownBackgroundTask
	}
	callCtx, cancel := context.WithTimeout(ctx, taskStopTimeout)
	defer cancel()
	res, err := as.transport.Call(callCtx, asyncTaskStopMethod, map[string]any{"sessionId": sessionID, "asyncTaskId": rawID})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBackgroundTaskStopRefused, err)
	}
	var body struct {
		Stopped bool `json:"stopped"`
	}
	if json.Unmarshal(res, &body) != nil || !body.Stopped {
		return ErrBackgroundTaskStopRefused
	}
	return nil
}
