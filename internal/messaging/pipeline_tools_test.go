package messaging

import (
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/pipeline"
	"github.com/agentdeck/agentdeck/internal/state"
)

func pipelineProposalFixture(t *testing.T) (*Server, *state.Store, *pipeline.TemplateStore) {
	t.Helper()
	home := t.TempDir()
	configStore := config.NewWithHome(home)
	if err := configStore.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	if err := configStore.WriteRole("implementer", config.Role{Title: "Implementer"}); err != nil {
		t.Fatal(err)
	}
	stateStore, err := state.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stateStore.Close() })
	agent := state.Agent{AgentID: "a_builder", Name: "Builder", Role: "agentdecker", Project: "app", Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC()}
	if err := stateStore.WriteAgent(agent); err != nil {
		t.Fatal(err)
	}
	ordinary := agent
	ordinary.AgentID = "a_other"
	ordinary.Role = "implementer"
	if err := stateStore.WriteAgent(ordinary); err != nil {
		t.Fatal(err)
	}
	templates := pipeline.NewTemplateStore(configStore)
	manager := pipeline.NewManager(stateStore, templates, nil, nil)
	server := New(stateStore, nil)
	server.SetPipelineManager(manager)
	server.RegisterSession("builder-token", agent.AgentID, "builder-generation")
	server.RegisterSession("other-token", ordinary.AgentID, "other-generation")
	return server, stateStore, templates
}

func proposalTemplateArgs() map[string]any {
	return map[string]any{
		"id": "quality",
		"template": map[string]any{
			"version": 1, "title": "Quality", "inputs": []any{},
			"stages": []any{map[string]any{
				"id": "work", "title": "Work", "role": "implementer", "instruction": "Do the work.",
				"inputs": []any{}, "outputs": []any{},
				"transitions": map[string]any{
					"success": map[string]any{"final": "success", "approval": "automatic"},
					"failure": map[string]any{"final": "failure", "approval": "required"},
				},
			}},
		},
	}
}

// FS-14.A10 / TS-04.R17: proposal tools are AgentDecker-only, return data and
// a digest, cannot mutate the template store themselves, and publish exactly
// one durable review record before MCP reports success.
func TestPipelineProposalToolIsScopedAndNonMutating(t *testing.T) {
	server, stateStore, templates := pipelineProposalFixture(t)
	ordinary := connect(t, server, "other-token")
	result, isErr := call(t, ordinary, "propose_pipeline_template", proposalTemplateArgs())
	if !isErr || result["error"] != "proposal_forbidden" {
		t.Fatalf("ordinary proposal = %v isErr=%v", result, isErr)
	}
	builder := connect(t, server, "builder-token")
	result, isErr = call(t, builder, "propose_pipeline_template", proposalTemplateArgs())
	if isErr || result["ok"] != true {
		t.Fatalf("builder proposal = %v isErr=%v", result, isErr)
	}
	proposal := result["proposal"].(map[string]any)
	if proposal["kind"] != "save_template" || proposal["digest"] == "" || proposal["proposal_id"] == "" {
		t.Fatalf("proposal = %v", proposal)
	}
	if _, err := templates.Read("quality"); err != pipeline.ErrTemplateNotFound {
		t.Fatalf("proposal mutated template store: %v", err)
	}

	// A real ACP adapter can persist its terminal tool_result with content:null.
	// Discard the caller-only result here and prove the token-bound MCP handler
	// already committed the canonical proposal to the server-owned authority.
	if _, isErr = call(t, builder, "propose_pipeline_template", proposalTemplateArgs()); isErr {
		t.Fatalf("repeat builder proposal failed")
	}
	proposals, err := stateStore.ListPipelineProposals(10)
	if err != nil || len(proposals) != 1 {
		t.Fatalf("durable proposals = %+v err=%v", proposals, err)
	}
	if proposals[0].ProposalID != proposal["proposal_id"] || proposals[0].Kind != "save_template" {
		t.Fatalf("durable proposal = %+v", proposals[0])
	}
}

func TestPipelineProposalToolFailsCleanlyWithoutManager(t *testing.T) {
	server, _, _ := pipelineProposalFixture(t)
	server.SetPipelineManager(nil)
	builder := connect(t, server, "builder-token")
	result, isErr := call(t, builder, "propose_pipeline_template", proposalTemplateArgs())
	if !isErr || result["error"] != "pipeline_unavailable" {
		t.Fatalf("proposal without manager = %v isErr=%v", result, isErr)
	}
}
