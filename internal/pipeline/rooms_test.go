package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/AsaphNoam/Chuck/internal/state"
)

// mixedRoomFixture saves ordinary → think_tank → ordinary and starts it.
func mixedRoomFixture(t *testing.T, approval bool) (*Manager, *fakeLifecycle, RunDetail) {
	t.Helper()
	manager, lifecycle, _ := pipelineManagerFixture(t)
	template := Template{
		Version: 2, Title: "Mixed", OrchestratorRole: "orchestrator",
		Inputs: []ValueDecl{{Name: "spec", Description: "Specification", Required: true}},
		Stages: []Stage{
			{ID: "draft", Title: "Draft", Objective: "Draft options.",
				Inputs:  []StageInput{{Name: "spec", Value: "spec", Required: true}},
				Outputs: []StageOutput{{Name: "options", Value: "options", Description: "Options"}}},
			{ID: "debate", Title: "Debate", Objective: "Choose an option.", Coordination: CoordinationThinkTank, ApprovalAfterSuccess: approval,
				ThinkTank: &ThinkTankStage{JudgeRole: "reviewer", Participants: []ThinkTankParticipant{
					{ID: "pro", Role: "implementer", Limit: 2}, {ID: "con", Role: "implementer", Limit: 2},
				}},
				Inputs:  []StageInput{{Name: "options", Value: "options", Required: true}},
				Outputs: []StageOutput{{Name: "decision", Value: "decision", Description: "Judge synthesis"}}},
			{ID: "build", Title: "Build", Objective: "Build the decision.",
				Inputs: []StageInput{{Name: "decision", Value: "decision", Required: true}}, Outputs: []StageOutput{}},
		},
	}
	if record, err := manager.templates.Create("mixed", template); err != nil || !record.Valid {
		t.Fatalf("create = %+v err=%v", record, err)
	}
	slot := RuntimeAssignment{Backend: "claude", Model: "sonnet"}
	detail, _, err := manager.Start(context.Background(), StartRequest{
		RequestID: "mixed", TemplateID: "mixed", Project: "app", Goal: "Pick and build",
		Inputs: map[string]string{"spec": "the spec"}, Orchestrator: slot,
		ThinkTankAssignments: map[string]ThinkTankAssignment{"debate": {
			Participants: map[string]RuntimeAssignment{"pro": slot, "con": {Backend: "codex", Model: "gpt"}}, Judge: slot,
		}},
	})
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	return manager, lifecycle, detail
}

// finishOrdinaryStage reports the current ordinary stage through the agent
// branch and lets its release converge.
func finishOrdinaryStage(t *testing.T, manager *Manager, runID, agentID string, outputs map[string]string) {
	t.Helper()
	stage, found, err := manager.currentStageTask(runID)
	if err != nil || !found {
		t.Fatalf("current stage: %v", err)
	}
	if _, err := manager.store.DB().Exec(`UPDATE tasks SET state = ?, assigned_agent_id = ?, assigned_generation = 'gen', execution_handle = 'ta_x' WHERE task_id = ?`,
		state.TaskRunning, agentID, stage.TaskID); err != nil {
		t.Fatal(err)
	}
	if err := manager.store.BindPipelineStageTaskStandingAgent(stage.TaskID, agentID); err != nil {
		t.Fatal(err)
	}
	run, _ := manager.store.ReadPipelineRun(runID)
	if _, err := manager.store.AcceptPipelineStageTaskResult(stage.TaskID, agentID, "gen", "ta_x", run.Revision,
		state.TaskResult{Outcome: state.OutcomeSuccess, Summary: "done", Outputs: outputs}); err != nil {
		t.Fatal(err)
	}
	if err := manager.store.CompleteTaskRelease(stage.TaskID); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reconcile(context.Background(), runID); err != nil {
		t.Fatal(err)
	}
}

// publishSynthesis stands in for the room engine's judge finalization.
func publishSynthesis(t *testing.T, manager *Manager, roomID, body string) {
	t.Helper()
	if _, err := manager.store.DB().Exec(`INSERT INTO think_tank_entries(room_id, seq, kind, agent_id, body, created_at) VALUES (?, (SELECT COALESCE(MAX(seq), 0) + 1 FROM think_tank_entries WHERE room_id = ?), 'synthesis', 'a_judge', ?, '2026-10-08T00:00:00Z')`, roomID, roomID, body); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.store.DB().Exec(`UPDATE think_tanks SET judge_status = 'completed', phase = 'ended' WHERE room_id = ?`, roomID); err != nil {
		t.Fatal(err)
	}
}

// FS-14.A48 — ordinary → Think Tank → ordinary: the room receives the produced
// input as stage context, nothing advances before the judge's synthesis is
// published, the exact synthesis becomes the named output once, and the next
// ordinary stage reuses the standing owner without any standing room turn.
func TestMixedRoomStageAcceptsExactSynthesisOnce(t *testing.T) {
	manager, lifecycle, detail := mixedRoomFixture(t, false)
	runID := detail.Run.RunID
	finishOrdinaryStage(t, manager, runID, "a_owner", map[string]string{"options": "A or B"})

	room, found, err := manager.currentStageTask(runID)
	if err != nil || !found || room.ExecutionKind != state.StageExecutionThinkTank || room.StageID != "debate" {
		t.Fatalf("room stage = %+v err=%v", room, err)
	}
	entries, err := manager.store.ListThinkTankEntries(room.RoomID, 0, 10)
	if err != nil || len(entries) == 0 || entries[0].Kind != state.ThinkTankEntryStageContext || !strings.Contains(entries[0].Body, "A or B") {
		t.Fatalf("stage context entry = %+v err=%v", entries, err)
	}

	// Idle, discussion and an unfinalized judge never advance.
	if _, err := manager.store.DB().Exec(`UPDATE think_tanks SET judge_status = 'running', phase = 'ended' WHERE room_id = ?`, room.RoomID); err != nil {
		t.Fatal(err)
	}
	manager.AcceptRoomOutputs(context.Background())
	if run, _ := manager.store.ReadPipelineRun(runID); run.State != "running" || run.CurrentStageID != "debate" {
		t.Fatalf("run advanced before publication: %+v", run)
	}

	synthesis := "Choose B.\n\nDissent: pro preferred A for speed."
	publishSynthesis(t, manager, room.RoomID, synthesis)
	if err := manager.AcceptRoomOutput(context.Background(), runID); err != nil {
		t.Fatal(err)
	}
	manager.AcceptRoomOutputs(context.Background()) // a replayed kick is a no-op

	task, err := manager.store.ReadTask(room.TaskID)
	if err != nil || task.State != state.TaskFinished || task.Outcome != state.OutcomeSuccess || task.OutcomeSource != state.StageExecutionThinkTank || task.Outputs["decision"] != synthesis {
		t.Fatalf("room task = %+v err=%v", task, err)
	}
	values, _ := manager.store.ListPipelineValues(runID)
	count := 0
	for _, v := range values {
		if v.Name == "decision" {
			count++
			if v.Value != synthesis {
				t.Fatalf("decision = %q", v.Value)
			}
		}
	}
	if count != 1 {
		t.Fatalf("decision values = %d", count)
	}
	stages, _ := manager.store.ListPipelineStageTasks(runID)
	if len(stages) != 3 || stages[2].StageID != "build" {
		t.Fatalf("stages = %+v", stages)
	}
	build, _ := manager.store.ReadTask(stages[2].TaskID)
	if build.TargetKind != state.TargetAgent || build.TargetAgentID != "a_owner" {
		t.Fatalf("build stage did not reuse the standing owner: %+v", build)
	}
	if len(lifecycle.launches) != 0 {
		t.Fatalf("pipeline launched directly: %+v", lifecycle.launches)
	}
}

// FS-14.A48 — a human gate still waits after accepted synthesis.
func TestRoomStageApprovalWaitsAfterAcceptance(t *testing.T) {
	manager, _, detail := mixedRoomFixture(t, true)
	runID := detail.Run.RunID
	finishOrdinaryStage(t, manager, runID, "a_owner", map[string]string{"options": "A or B"})
	room, _, _ := manager.currentStageTask(runID)
	publishSynthesis(t, manager, room.RoomID, "Choose B.")
	if err := manager.AcceptRoomOutput(context.Background(), runID); err != nil {
		t.Fatal(err)
	}
	run, _ := manager.store.ReadPipelineRun(runID)
	if run.State != "paused" || run.PendingAction != "await_approval" {
		t.Fatalf("run after accepted synthesis = %+v", run)
	}
}

// FS-14.A52 — refused output pauses with Retry output acceptance; retry reuses
// the same entry and accepts no partial result; a stopped run accepts nothing.
func TestRoomOutputRefusalAndStopFence(t *testing.T) {
	manager, _, detail := mixedRoomFixture(t, false)
	runID := detail.Run.RunID
	finishOrdinaryStage(t, manager, runID, "a_owner", map[string]string{"options": "A or B"})
	room, _, _ := manager.currentStageTask(runID)
	publishSynthesis(t, manager, room.RoomID, strings.Repeat("x", MaxValueRunes+1))
	if err := manager.AcceptRoomOutput(context.Background(), runID); err != nil {
		t.Fatal(err)
	}
	run, _ := manager.store.ReadPipelineRun(runID)
	if run.State != "paused" || run.PendingAction != pendingAcceptRoomOutput || !strings.Contains(run.AttentionReason, "decision") {
		t.Fatalf("refused run = %+v", run)
	}
	// The automatic sweep leaves the held acceptance to the explicit retry.
	manager.AcceptRoomOutputs(context.Background())
	if task, _ := manager.store.ReadTask(room.TaskID); task.State == state.TaskFinished {
		t.Fatalf("held acceptance finished the task: %+v", task)
	}
	// Retry still refuses the same oversized entry without partial output.
	if _, err := manager.Retry(context.Background(), runID, run.Revision); err != nil {
		t.Fatal(err)
	}
	if values, _ := manager.store.ListPipelineValues(runID); hasValue(values, "decision") {
		t.Fatalf("partial output installed: %+v", values)
	}

	// A stopped run cannot accept even a previously published synthesis.
	if _, err := manager.store.DB().Exec(`UPDATE pipeline_runs SET state = 'stopping', pending_action = 'cleanup_run' WHERE run_id = ?`, runID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.store.DB().Exec(`UPDATE think_tank_entries SET body = 'short' WHERE room_id = ? AND kind = 'synthesis'`, room.RoomID); err != nil {
		t.Fatal(err)
	}
	manager.AcceptRoomOutputs(context.Background())
	if values, _ := manager.store.ListPipelineValues(runID); hasValue(values, "decision") {
		t.Fatalf("stopped run accepted output: %+v", values)
	}
}

func hasValue(values []state.PipelineValueRecord, name string) bool {
	for _, v := range values {
		if v.Name == name {
			return true
		}
	}
	return false
}
