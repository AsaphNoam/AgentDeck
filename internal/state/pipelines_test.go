package state

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// FS-14.A5: run snapshots and idempotency identity survive process restart.
func TestPipelineRunPersistenceIdempotencyAndRestart(t *testing.T) {
	store, home := newTestStore(t)
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	runID, err := store.NewPipelineRunID()
	if err != nil {
		t.Fatal(err)
	}
	params := CreatePipelineRunParams{
		Run: PipelineRunRecord{
			RunID: runID, TemplateID: "quality", TemplateSnapshot: json.RawMessage(`{"version":1,"inputs":[],"stages":[]}`),
			DisplayName: "Quality run", Project: "app", Goal: "Ship it", Inputs: json.RawMessage(`{"spec":"Do it"}`),
			Assignments: json.RawMessage(`{"work":{"backend":"codex","model":"gpt"}}`), State: "queued",
			Revision: 1, PendingAction: "launch_stage", CurrentStageID: "work", CreatedAt: now, UpdatedAt: now,
		},
		RequestID: "request-1", RequestHash: "hash-1",
		Values: []PipelineValueRecord{{RunID: runID, Name: "spec", Value: "Do it", SourceKind: "run_input", UpdatedAt: now}},
	}
	created, replay, err := store.CreatePipelineRun(params)
	if err != nil || replay || created.RunID != runID {
		t.Fatalf("CreatePipelineRun = %+v replay=%v err=%v", created, replay, err)
	}
	replayed, replay, err := store.CreatePipelineRun(params)
	if err != nil || !replay || replayed.RunID != runID {
		t.Fatalf("replay = %+v replay=%v err=%v", replayed, replay, err)
	}
	conflict := params
	conflict.RequestHash = "different"
	if _, _, err := store.CreatePipelineRun(conflict); !errors.Is(err, ErrPipelineRequestConflict) {
		t.Fatalf("mismatched replay err = %v, want ErrPipelineRequestConflict", err)
	}
	values, err := store.ListPipelineValues(runID)
	if err != nil || len(values) != 1 || values[0].Value != "Do it" {
		t.Fatalf("values = %+v err=%v", values, err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.ReadPipelineRun(runID)
	if err != nil || got.PendingAction != "launch_stage" || got.Revision != 1 {
		t.Fatalf("restarted run = %+v err=%v", got, err)
	}
}

// FS-14.A5/A8: revisions and lineage are monotonic, while deletion leaves the
// ordinary agent identity intact.
func TestPipelineAttemptLineageCASAndDeletionKeepsAgent(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	agent := testAgent("a_pipeline", now)
	if err := store.WriteAgent(agent); err != nil {
		t.Fatal(err)
	}
	runID, _ := store.NewPipelineRunID()
	_, _, err := store.CreatePipelineRun(CreatePipelineRunParams{
		Run:       PipelineRunRecord{RunID: runID, TemplateID: "quality", DisplayName: "Run", Project: "app", Goal: "goal", State: "queued", CurrentStageID: "work", CreatedAt: now, UpdatedAt: now},
		RequestID: "request-2", RequestHash: "hash-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	attemptID, _ := store.NewPipelineAttemptID()
	if err := store.InsertPipelineAttempt(PipelineAttemptRecord{
		AttemptID: attemptID, RunID: runID, StageID: "work", AttemptNo: 1, VisitNo: 1,
		AgentID: agent.AgentID, Backend: "codex", Model: "gpt", Effort: "high", State: "queued", ReportOutputs: nil,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	attempts, err := store.ListPipelineAttempts(runID)
	if err != nil || len(attempts) != 1 || attempts[0].Effort != "high" || string(attempts[0].ReportOutputs) != "{}" {
		t.Fatalf("attempts = %+v err=%v", attempts, err)
	}
	updated, err := store.UpdatePipelineRunCAS(runID, 1, PipelineRunUpdate{
		State: "running", PendingAction: "await_result", CurrentStageID: "work", CurrentAttemptID: attemptID,
		CurrentAgentID: agent.AgentID, UpdatedAt: now.Add(time.Second),
	})
	if err != nil || updated.Revision != 2 {
		t.Fatalf("UpdatePipelineRunCAS = %+v err=%v", updated, err)
	}
	if _, err := store.UpdatePipelineRunCAS(runID, 1, PipelineRunUpdate{State: "stopped"}); !errors.Is(err, ErrPipelineConflict) {
		t.Fatalf("stale CAS err = %v, want ErrPipelineConflict", err)
	}
	if err := store.DeletePipelineRun(runID); !errors.Is(err, ErrPipelineActive) {
		t.Fatalf("active delete err = %v, want ErrPipelineActive", err)
	}
	updated, err = store.UpdatePipelineRunCAS(runID, 2, PipelineRunUpdate{State: "stopped", FinalOutcome: "stopped", UpdatedAt: now.Add(2 * time.Second)})
	if err != nil || updated.State != "stopped" {
		t.Fatalf("stop update = %+v err=%v", updated, err)
	}
	if err := store.DeletePipelineRun(runID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAgent(agent.AgentID); err != nil {
		t.Fatalf("pipeline deletion removed agent: %v", err)
	}
	if attempts, err := store.ListPipelineAttempts(runID); err != nil || attempts == nil || len(attempts) != 0 {
		t.Fatalf("attempts after delete = %+v err=%v", attempts, err)
	}
}

func TestListActivePipelineRunsExcludesTerminalHistory(t *testing.T) {
	store, _ := newTestStore(t)
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	for i, stateName := range []string{"running", "paused", "completed", "stopped"} {
		runID, err := store.NewPipelineRunID()
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = store.CreatePipelineRun(CreatePipelineRunParams{
			Run:       PipelineRunRecord{RunID: runID, TemplateID: "quality", DisplayName: stateName, Project: "app", Goal: "goal", State: stateName, CreatedAt: now.Add(time.Duration(i) * time.Second), UpdatedAt: now},
			RequestID: "active-list-" + stateName, RequestHash: stateName,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	runs, err := store.ListActivePipelineRuns()
	if err != nil || len(runs) != 2 || runs[0].State != "running" || runs[1].State != "paused" {
		t.Fatalf("active runs = %+v err=%v", runs, err)
	}
}

// FS-14.R34 / FS-16.R13 / TS-09.R27, TS-10.R21 — a run registers its terminal
// outcome in the same commit that makes it terminal, mapped onto the shared
// vocabulary with its template-defined label kept as a display detail. That
// registration is what makes a run usable as a prerequisite, and because it is
// keyed to the source and holds no foreign key into the run, deleting the run
// afterwards leaves every arm that waited on it already resolved.
func TestTerminalPipelineRunsRegisterTheirOutcome(t *testing.T) {
	cases := []struct {
		name         string
		runState     string
		finalOutcome string
		want         string
	}{
		{"completed with success", "completed", "success", OutcomeSuccess},
		{"completed with another final label", "completed", "needs-rework", OutcomeFailure},
		{"stopped", "stopped", "stopped", OutcomeCancelled},
		{"still running", "running", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := newTestStore(t)
			now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
			runID, err := store.NewPipelineRunID()
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := store.CreatePipelineRun(CreatePipelineRunParams{
				Run: PipelineRunRecord{
					RunID: runID, TemplateID: "quality",
					TemplateSnapshot: json.RawMessage(`{"version":1,"inputs":[],"stages":[]}`),
					DisplayName:      "Quality run", Project: "app", Goal: "Ship it",
					Inputs: json.RawMessage(`{}`), Assignments: json.RawMessage(`{}`),
					State: "queued", Revision: 1, PendingAction: "launch_stage",
					CurrentStageID: "work", CreatedAt: now, UpdatedAt: now,
				},
				RequestID: "request-" + tc.name, RequestHash: "hash",
			}); err != nil {
				t.Fatalf("CreatePipelineRun: %v", err)
			}
			if _, err := store.UpdatePipelineRunCAS(runID, 1, PipelineRunUpdate{
				State: tc.runState, FinalOutcome: tc.finalOutcome, CurrentStageID: "work",
			}); err != nil {
				t.Fatalf("UpdatePipelineRunCAS: %v", err)
			}
			result, err := store.ReadWorkResult(SourcePipelineRun, runID)
			if tc.want == "" {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("a non-terminal run registered %+v, %v", result, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadWorkResult: %v", err)
			}
			if result.Outcome != tc.want {
				t.Fatalf("registered outcome = %q, want %q", result.Outcome, tc.want)
			}
			if result.RawLabel != tc.finalOutcome {
				t.Fatalf("raw label = %q, want the template's own %q", result.RawLabel, tc.finalOutcome)
			}

			// Deleting the run leaves the registration in place, which is why an
			// arm can never be left waiting on a run that no longer exists.
			if err := store.DeletePipelineRun(runID); err != nil {
				t.Fatalf("DeletePipelineRun: %v", err)
			}
			if _, err := store.ReadWorkResult(SourcePipelineRun, runID); err != nil {
				t.Fatalf("the registration was removed with its run: %v", err)
			}
		})
	}
}
