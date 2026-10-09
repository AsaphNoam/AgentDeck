package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/AsaphNoam/Chuck/internal/state"
)

const (
	// pendingAcceptRoomOutput marks a published synthesis whose stage
	// acceptance exhausted its automatic retries or was refused; only the
	// run's Retry re-attempts it, reusing that exact entry (TS-09.R53).
	pendingAcceptRoomOutput = "accept_room_output"
	// maxRoomOutputAttempts bounds automatic acceptance retries before the
	// run pauses with Retry output acceptance (FS-14.R86).
	maxRoomOutputAttempts = 3
	roomOutputSweepLimit  = 64
)

// errRoomRecovery is the typed refusal of generic stage Continue/Retry/Replace
// for a room-backed stage: recovery belongs to the room (TS-09.R55).
var errRoomRecovery = controlError("room_recovery_required", "this Think Tank stage recovers from its room: open the room to resume, retry setup or retry the judge")

// AcceptRoomOutputs is the bounded durable recovery for a missed post-commit
// kick: every open room-backed stage whose judge completed is accepted once.
// SQLite CAS, not delivery of this call, prevents duplicate acceptance.
func (m *Manager) AcceptRoomOutputs(ctx context.Context) {
	stages, err := m.store.PendingThinkTankStageOutputs(roomOutputSweepLimit)
	if err != nil {
		slog.Warn("pipeline: list pending room outputs", "err", err)
		return
	}
	for _, stage := range stages {
		if err := m.acceptRoomOutput(ctx, stage.RunID, false); err != nil {
			slog.Warn("pipeline: accept room output", "run", stage.RunID, "err", err)
		}
	}
}

// AcceptRoomOutput is the post-commit kick after a pipeline room's judge
// publishes its synthesis.
func (m *Manager) AcceptRoomOutput(ctx context.Context, runID string) error {
	return m.acceptRoomOutput(ctx, runID, false)
}

// acceptRoomOutput accepts the current room stage's published synthesis. A
// paused exhausted acceptance waits for the explicit Retry unless explicit.
func (m *Manager) acceptRoomOutput(ctx context.Context, runID string, explicit bool) error {
	unlock := m.lockRun(runID)
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		unlock()
		return err
	}
	current, found, err := m.currentStageTask(runID)
	if err != nil || !found || current.ExecutionKind != state.StageExecutionThinkTank || current.State != "open" {
		unlock()
		return err
	}
	held := run.State == "paused" && run.PendingAction == pendingAcceptRoomOutput
	if (held && !explicit) || run.State == "stopping" || run.State == "stopped" || run.State == "completed" {
		unlock()
		return nil
	}
	if !explicit && current.AcceptanceAttempts >= maxRoomOutputAttempts {
		defer unlock()
		latest, readErr := m.store.ReadPipelineRun(runID)
		if readErr != nil {
			return readErr
		}
		paused, pauseErr := m.store.UpdatePipelineRunCAS(runID, latest.Revision, state.PipelineRunUpdate{
			State: "paused", PendingAction: pendingAcceptRoomOutput, CurrentStageID: current.StageID,
			AttentionReason: "output acceptance failed: automatic retries exhausted",
		})
		if pauseErr != nil {
			return pauseErr
		}
		m.publish(paused)
		m.notify(paused, "needs_attention")
		return nil
	}
	prepared, err := m.store.PrepareThinkTankStageOutputAcceptance(current.TaskID, !explicit)
	if err != nil {
		unlock()
		if errors.Is(err, state.ErrPipelineStageConflict) {
			return nil
		}
		return err
	}
	latest, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		unlock()
		return err
	}
	if latest.Revision != run.Revision {
		m.publish(latest)
	}
	revision := latest.Revision
	if held {
		resumed, err := m.store.UpdatePipelineRunCAS(runID, revision, state.PipelineRunUpdate{
			State: "running", PendingAction: pendingAcceptRoomOutput, CurrentStageID: current.StageID,
		})
		if err != nil {
			unlock()
			return err
		}
		revision = resumed.Revision
		m.publish(resumed)
	}
	accepted, err := m.store.AcceptThinkTankStageOutput(current.TaskID, revision)
	if err == nil {
		m.publish(accepted)
		unlock()
		return m.Reconcile(ctx, runID)
	}
	refused := errors.Is(err, state.ErrThinkTankOutputRefused)
	if !refused && errors.Is(err, state.ErrPipelineStageConflict) {
		// Not ready yet, or another caller won: nothing to record.
		unlock()
		return nil
	}
	defer unlock()
	if !refused && !explicit && prepared.AcceptanceAttempts < maxRoomOutputAttempts {
		return err
	}
	reason := "output acceptance failed: " + strings.TrimPrefix(err.Error(), state.ErrThinkTankOutputRefused.Error()+": ")
	latest, readErr := m.store.ReadPipelineRun(runID)
	if readErr != nil {
		return errors.Join(err, readErr)
	}
	paused, pauseErr := m.store.UpdatePipelineRunCAS(runID, latest.Revision, state.PipelineRunUpdate{
		State: "paused", PendingAction: pendingAcceptRoomOutput, CurrentStageID: current.StageID,
		AttentionReason: clipText(reason, MaxDescriptionRunes),
	})
	if pauseErr != nil {
		return errors.Join(err, pauseErr)
	}
	m.publish(paused)
	m.notify(paused, "needs_attention")
	return nil
}

// SyncRoomPhase projects an open room stage's intervention state onto its
// run: an explicit room pause, hold or judge failure pauses the run with the
// room's reason; room recovery returns it to running. Revision-guarded; never
// touches an output-acceptance hold, approval gate or stopping run (TS-09.R55).
func (m *Manager) SyncRoomPhase(_ context.Context, runID string) error {
	unlock := m.lockRun(runID)
	defer unlock()
	run, err := m.store.ReadPipelineRun(runID)
	if err != nil {
		return err
	}
	if (run.State != "running" && run.State != "paused") || run.PendingAction != "" {
		return nil
	}
	current, found, err := m.currentStageTask(runID)
	if err != nil || !found || current.RoomID == "" || current.State != "open" {
		return err
	}
	room, err := m.store.ReadThinkTank(current.RoomID)
	if err != nil {
		return err
	}
	reason := roomAttention(room.Room)
	want := "running"
	if reason != "" {
		want = "paused"
	}
	if run.State == want && run.AttentionReason == reason {
		return nil
	}
	updated, err := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
		State: want, PendingAction: "", CurrentStageID: current.StageID, AttentionReason: reason,
	})
	if errors.Is(err, state.ErrPipelineConflict) {
		return nil
	}
	if err != nil {
		return err
	}
	m.publish(updated)
	if want == "paused" {
		m.notify(updated, "needs_attention")
	}
	return nil
}

// roomAttention is the bounded run attention reason for a room needing a
// person, or "" while it can progress on its own.
func roomAttention(room state.ThinkTank) string {
	switch {
	case room.JudgeStatus == state.ThinkTankJudgeFailed:
		return clipText("Think Tank judge failed: "+room.JudgeError, MaxDescriptionRunes)
	case room.Hold != "":
		return clipText("Think Tank needs attention: "+room.Hold, MaxDescriptionRunes)
	case room.Control == state.ThinkTankPaused:
		return "Think Tank paused"
	}
	return ""
}
