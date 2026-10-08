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
	revision := run.Revision
	if held {
		resumed, err := m.store.UpdatePipelineRunCAS(runID, run.Revision, state.PipelineRunUpdate{
			State: "running", PendingAction: "", CurrentStageID: current.StageID,
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
		m.clearRoomOutputFailures(runID)
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
	if !refused && m.noteRoomOutputFailure(runID) < maxRoomOutputAttempts {
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
	m.clearRoomOutputFailures(runID)
	m.publish(paused)
	m.notify(paused, "needs_attention")
	return nil
}

func (m *Manager) noteRoomOutputFailure(runID string) int {
	m.attentionMu.Lock()
	defer m.attentionMu.Unlock()
	if m.roomOutputFailures == nil {
		m.roomOutputFailures = map[string]int{}
	}
	m.roomOutputFailures[runID]++
	return m.roomOutputFailures[runID]
}

func (m *Manager) clearRoomOutputFailures(runID string) {
	m.attentionMu.Lock()
	defer m.attentionMu.Unlock()
	delete(m.roomOutputFailures, runID)
}
