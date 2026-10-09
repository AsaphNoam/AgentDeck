package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AsaphNoam/Chuck/internal/pipeline"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// FS-14.A48 / FS-21.A31 against a fake provider: a room-only pipeline launches
// fresh same-project participants, then a fresh judge after discussion; the
// judge reads the stage context first; its published synthesis is accepted
// once as the exact named output and completes the run. No standing owner is
// prompted.
func TestPipelineThinkTankStageEndToEnd(t *testing.T) {
	srv, promptLog, _, hold := thinkTankTestServer(t)
	backends, err := srv.readBackendsOrDefault()
	if err != nil {
		t.Fatal(err)
	}
	target, effort, ae := resolveLaunchSpec(backends, "", "", "")
	if ae != nil {
		t.Fatal(ae)
	}
	slot := pipeline.RuntimeAssignment{Backend: target.BackendID, Model: target.ModelID, Effort: effort}
	template := pipeline.Template{
		Version: 2, Title: "Deliberate", OrchestratorRole: "impl", Inputs: []pipeline.ValueDecl{},
		Stages: []pipeline.Stage{{
			ID: "debate", Title: "Debate", Objective: "Pick a queue.", Coordination: pipeline.CoordinationThinkTank,
			ThinkTank: &pipeline.ThinkTankStage{JudgeRole: "impl", Participants: []pipeline.ThinkTankParticipant{
				{ID: "pro", Role: "impl", Limit: 1}, {ID: "con", Role: "impl", Limit: 1},
			}},
			Inputs: []pipeline.StageInput{}, Outputs: []pipeline.StageOutput{{Name: "decision", Value: "decision", Description: "Synthesis"}},
		}},
	}
	if record, err := srv.pipelineTemplates.Create("deliberate", template); err != nil || !record.Valid {
		t.Fatalf("create template = %+v err=%v", record, err)
	}
	before := promptCount(t, promptLog)
	ctx := context.Background()
	detail, _, err := srv.pipelineMgr.Start(ctx, pipeline.StartRequest{
		RequestID: "room-e2e", TemplateID: "deliberate", Project: "tmpproj", Goal: "Choose a queue",
		ThinkTankAssignments: map[string]pipeline.ThinkTankAssignment{"debate": {
			Participants: map[string]pipeline.RuntimeAssignment{"pro": slot, "con": slot}, Judge: slot,
		}},
	})
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	stage, err := srv.stateStore.LatestPipelineStageTask(detail.Run.RunID)
	if err != nil || stage.RoomID == "" {
		t.Fatalf("stage = %+v err=%v", stage, err)
	}
	room := stage.RoomID
	// FS-16.A32 — the stage task names Think Tank execution and its room, and
	// generic task controls cannot impersonate its authority.
	for _, path := range []string{"/cancel", "/retry", "/result", "/rearm"} {
		if rec := doJSON(t, srv.routes(), "POST", "/api/tasks/"+stage.TaskID+path, `{"outcome":"success","summary":"x","arms":[]}`); rec.Code != 409 {
			t.Fatalf("POST %s = %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, srv.routes(), "DELETE", "/api/tasks/"+stage.TaskID, ``); rec.Code != 409 {
		t.Fatalf("DELETE = %d", rec.Code)
	}
	if rec := doJSON(t, srv.routes(), "POST", "/api/tasks", `{"project":"tmpproj","display_name":"x","instruction":"x","target_kind":"think_tank"}`); rec.Code == 201 {
		t.Fatalf("public think_tank task create accepted: %s", rec.Body.String())
	}
	if rec := doJSON(t, srv.routes(), "GET", "/api/tasks/"+stage.TaskID, ``); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"room_id":"`+room+`"`) {
		t.Fatalf("task detail = %d %s", rec.Code, rec.Body.String())
	}

	srv.progressThinkTanks(ctx)
	d := waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Room.Phase == state.ThinkTankPhaseDiscussion })
	for _, m := range d.Members {
		if m.Project != "tmpproj" {
			t.Fatalf("participant project = %q", m.Project)
		}
		if _, err := srv.stateStore.ReadRunning(m.AgentID); err != nil {
			t.Fatalf("fresh participant %s not launched: %v", m.AgentID, err)
		}
	}
	for range d.Members {
		srv.progressThinkTanks(ctx)
		cur := waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Active != nil })
		actAsAgent(t, srv, *cur.Active, state.ThinkTankReply, "view of "+cur.Active.AgentID)
		releaseTurn(t, hold, srv, room, cur.Room.Revision)
	}
	waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool { return d.Room.JudgeStatus == state.ThinkTankJudgeReady })
	srv.progressThinkTanks(ctx)
	srv.progressThinkTanks(ctx)
	d = waitRoom(t, srv, room, func(d state.ThinkTankDetail) bool {
		return d.Active != nil && d.Active.Turn == state.ThinkTankTurnJudge
	})
	judge := *d.Active
	page, err := srv.stateStore.ReadThinkTankPage(state.ThinkTankReadRequest{CallerAgentID: judge.AgentID})
	if err != nil || len(page.Items) == 0 || page.Items[0].Entry.Kind != state.ThinkTankEntryStageContext {
		t.Fatalf("judge first read = %+v err=%v", page.Items, err)
	}
	synthesis := "Use the durable queue; con still prefers polling."
	actAsAgent(t, srv, judge, state.ThinkTankReply, synthesis)
	releaseTurn(t, hold, srv, room, d.Room.Revision)

	deadline := time.Now().Add(10 * time.Second)
	var run state.PipelineRunRecord
	for time.Now().Before(deadline) {
		if run, err = srv.stateStore.ReadPipelineRun(detail.Run.RunID); err == nil && run.State == "completed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if run.State != "completed" || run.FinalOutcome != state.OutcomeSuccess {
		t.Fatalf("run = %+v", run)
	}
	values, _ := srv.stateStore.ListPipelineValues(run.RunID)
	found := false
	for _, v := range values {
		found = found || (v.Name == "decision" && v.Value == synthesis)
	}
	if !found {
		t.Fatalf("values = %+v", values)
	}
	if got := promptCount(t, promptLog) - before; got != 3 {
		t.Fatalf("provider prompts = %d, want two participants and one judge", got)
	}
}

// TS-09.R54 / FS-21.R42 — a completed setup runtime may take private work
// under its original generation. Replayed Stop closes the room without
// stopping that turn; only a launch claimed across Stop is torn down.
func TestPipelineStopRoomPreservesPrivateTurnOnSameGeneration(t *testing.T) {
	srv, _, ids, _ := thinkTankTestServer(t)
	agents := strings.Split(ids, ",")
	agent := agents[0]
	room := createTestRoom(t, srv, agents, 1)
	generation := srv.registry.Generation(agent)
	if _, err := srv.stateStore.DB().Exec(`UPDATE think_tanks SET pipeline_run_id = 'pr_stop' WHERE room_id = ?`, room); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.stateStore.DB().Exec(`UPDATE think_tank_members SET launch_generation = ? WHERE room_id = ? AND agent_id = ?`, generation, room, agent); err != nil {
		t.Fatal(err)
	}
	before, err := srv.stateStore.ReadThinkTank(room)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.StopRoom(context.Background(), room); err != nil {
		t.Fatal(err)
	}
	after, err := srv.stateStore.ReadThinkTank(room)
	if err != nil || after.Room.Revision == before.Room.Revision {
		t.Fatalf("room did not close: %+v err=%v", after.Room, err)
	}
	if err := srv.StopRoom(context.Background(), room); err != nil {
		t.Fatal(err)
	}
	replayed, _ := srv.stateStore.ReadThinkTank(room)
	if replayed.Room.Revision != after.Room.Revision {
		t.Fatalf("idempotent close changed room revision: %d -> %d", after.Room.Revision, replayed.Room.Revision)
	}
	if _, err := srv.stateStore.ReadRunning(agent); err != nil {
		t.Fatalf("ready participant runtime stopped: %v", err)
	}
	if _, err := srv.registry.SendPromptOrHold(context.Background(), agent, "private follow-up"); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.stateStore.DB().Exec(`UPDATE think_tank_members SET stop_teardown = 1 WHERE room_id = ? AND agent_id = ?`, room, agent); err != nil {
		t.Fatal(err)
	}
	if err := srv.StopRoom(context.Background(), room); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.stateStore.ReadRunning(agent); err != nil {
		t.Fatalf("private same-generation turn stopped: %v", err)
	}
}
