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

// A historical pipeline association is no longer a wake veto. Durable task
// ownership provides the execution fence, so an otherwise resumable stopped
// agent remains an ordinary task and mail target.
func TestStoppedPipelineAgentRemainsAddressable(t *testing.T) {
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
	if isErr || res["ok"] != true {
		t.Fatalf("create_task to stopped pipeline agent = %v (isErr=%v)", res, isErr)
	}

	res, isErr = call(t, coord, "send_message", map[string]any{
		"to": "a_stage", "subject": "status", "body": "report when ready",
	})
	if isErr || res["ok"] != true {
		t.Fatalf("send_message to stopped pipeline agent = %v (isErr=%v)", res, isErr)
	}
}

// FS-06.A19: do not promise Resume when the configuration-owned project gate
// means Resume cannot succeed until the project is restored.
func TestArchivedProjectDoesNotOfferPipelineResume(t *testing.T) {
	f := newContextFixture(t)
	liveAgent(t, f.store, "a_coord", "Atlas", "agentdecker", "my-app")
	f.srv.RegisterSession("tok-coord", "a_coord", "gen-a_coord")
	f.srv.SetAddressableAgents(func() ([]state.LiveAgent, error) { return f.store.LiveAgents() })
	f.srv.SetProjectAvailable(func(project string) (bool, error) { return false, nil })
	f.srv.SetTaskControl(&stubTaskControl{})
	stoppedPipelineAgent(t, f.store, "a_stage", "Nova", "implementer", "archived")
	coord := connect(t, f.srv, "tok-coord")
	result, isErr := call(t, coord, "create_task", map[string]any{
		"display_name": "validate", "instruction": "run checks", "to": "a_stage",
	})
	message, _ := result["message"].(string)
	if !isErr || strings.Contains(message, "resume") {
		t.Fatalf("archived-project refusal = %v isErr=%v", result, isErr)
	}
}
