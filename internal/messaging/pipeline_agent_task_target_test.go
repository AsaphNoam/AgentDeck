package messaging

import (
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// stoppedPipelineAgent writes the exact shape of the field report's earlier
// stage coordinator: a real, non-archived chat agent with the frozen session
// snapshot a resume needs, no running row, and the pipeline attempt its stage
// left behind. That attempt row lives as long as the run record does
// (`ON DELETE CASCADE` from `pipeline_runs`, internal/state/schema.go:237), so
// the association outlives the stage, the run, and the whole pipeline.
func stoppedPipelineAgent(t *testing.T, st *state.Store, id, name, role, project string) {
	t.Helper()
	if err := st.WriteAgent(state.Agent{
		AgentID: id, Name: name, Role: role, Project: project,
		Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteAgent %s: %v", id, err)
	}
	if _, err := st.DB().Exec(`
INSERT INTO sessions(agent_id, name, role, project, backend, model, interface, cwd, system_prompt, created_at, updated_at)
VALUES (?,?,?,?,'claude','sonnet','chat','/tmp','prompt','2026-09-01T10:00:00Z','2026-09-01T10:01:00Z')`,
		id, name, role, project); err != nil {
		t.Fatalf("insert session %s: %v", id, err)
	}
	if _, err := st.DB().Exec(`
INSERT INTO pipeline_runs(run_id, template_id, display_name, project, goal, state, created_at, updated_at)
VALUES ('pr_1','t_1','Ship','` + project + `','ship','running','2026-09-01T10:00:00Z','2026-09-01T10:00:00Z')`); err != nil {
		t.Fatalf("insert pipeline run: %v", err)
	}
	if err := st.InsertPipelineAttempt(state.PipelineAttemptRecord{
		AttemptID: "pa_1", RunID: "pr_1", StageID: "implement", AttemptNo: 1, VisitNo: 1,
		AgentID: id, AgentGeneration: "gen-" + id, Backend: "claude", Model: "sonnet", State: "done",
	}); err != nil {
		t.Fatalf("InsertPipelineAttempt: %v", err)
	}
}

// Regression for the 2026-09-02 bug investigation finding "a task aimed at a
// stopped pipeline agent is refused as if that agent did not exist".
//
// A later pipeline stage delegated work back to the coordinator an earlier
// stage had used. That agent still existed and was still resumable, but it was
// stopped, and FS-06.R22's wake gates exclude every pipeline-associated agent
// (`stoppedWakeGates`, internal/state/messages.go:40). Excluding it is
// specified. Saying "No agent matches" is not: the same identity resolves fine
// for `share_context`, which FS-15.R17 deliberately runs over a looser set, so
// the operator saw one plane accept the agent and another deny its existence.
//
// `recipient_not_found` is classified `after_change` (FS-17 §3), which is the
// right class — resuming the agent does make the call succeed — but the message
// names no change, so neither the agent nor the person can find the one action
// that unblocks the stage. INV §8 requires a user-facing refusal to be
// human-meaningful and in vocabulary; this is the same defect class as the
// 2026-08-28 `stale_assignment` report.
//
// The fix session owns the final wording, and may instead decide the recipient
// set is wrong; adjust this expectation with whichever it picks.
func TestTaskAimedAtAStoppedPipelineAgentNamesTheRealCondition(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_coord", "Atlas", "agentdecker", "my-app")
	f.srv.RegisterSession("tok-coord", "a_coord", "gen-a_coord")
	f.srv.SetAddressableAgents(func() ([]state.LiveAgent, error) {
		return f.store.AddressableAgents()
	})
	f.srv.SetTaskControl(&stubTaskControl{})
	f.transcriptEvents(t, "a_coord",
		contextEvent(t, runtime.EvUserPrompt, runtime.UserPromptData{Text: "validate the change"}),
		contextEvent(t, runtime.EvAssistantText, runtime.AssistantTextData{Delta: "the checks to run"}),
	)

	// The earlier stage's coordinator, exactly as the pipeline left it.
	stoppedPipelineAgent(t, f.store, "a_stage", "Nova", "implementer", "my-app")

	coord := connect(t, f.srv, "tok-coord")

	// The context plane resolves the same agent by the same id and shares with
	// it, which is what made the task refusal read as a product fault.
	res, isErr := call(t, coord, "share_context", map[string]any{
		"to": "a_stage", "source": "current_turn", "label": "validation package",
	})
	if isErr || res["ok"] != true {
		t.Fatalf("share_context to the stopped stage agent = %v (isErr=%v); the fixture no longer matches the report", res, isErr)
	}

	res, isErr = call(t, coord, "create_task", map[string]any{
		"display_name": "validate the change", "instruction": "run the checks", "to": "a_stage",
	})
	if !isErr {
		t.Fatalf("create_task accepted a stopped pipeline agent = %v", res)
	}
	message, _ := res["message"].(string)
	if strings.Contains(message, "No agent matches") {
		t.Fatalf("refusal denies the existence of an agent this same call just shared context with: %q", message)
	}
	if !strings.Contains(message, "stopped") {
		t.Fatalf("refusal does not name the condition that excluded the agent: %q", message)
	}
	if !strings.Contains(message, "resum") {
		t.Fatalf("refusal does not name the change that would let the call succeed: %q", message)
	}

	res, isErr = call(t, coord, "send_message", map[string]any{
		"to": "a_stage", "subject": "status", "body": "report when ready",
	})
	if !isErr {
		t.Fatalf("send_message accepted a stopped pipeline agent = %v", res)
	}
	message, _ = res["message"].(string)
	if !strings.Contains(message, "stopped") || !strings.Contains(message, "resum") {
		t.Fatalf("send_message refusal does not explain the pipeline exclusion: %q", message)
	}
}
