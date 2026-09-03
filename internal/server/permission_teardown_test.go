package server

import (
	"encoding/json"
	"testing"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

// TestAgentExitClearsPendingPermissionTools pins the teardown side of the tool
// names the server holds for FS-03.R44's withheld-tool log. They are written
// when a request is raised and deleted when it resolves, but a crashed process
// abandons its pending requests without resolving them (internal/runtime's
// onTransportClosed), so without an exit-path sweep every crash-with-approval
// leaks one entry for the life of the process (INV §4). The sweep is
// generation-scoped, so a late crash of one launch must not clear another's.
func TestAgentExitClearsPendingPermissionTools(t *testing.T) {
	srv := testServer(t, false)
	raise := func(agentID, generation, toolCallID, name string) {
		t.Helper()
		data, err := json.Marshal(runtime.PermissionRequestData{ToolCallID: toolCallID, Name: name})
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		srv.handlePermissionEvent(runtime.Event{
			Type:       runtime.EvPermissionRequest,
			AgentID:    agentID,
			Generation: generation,
			Data:       data,
		})
	}
	raise("agent-a", "gen-1", "call-1", "Bash")
	raise("agent-a", "gen-2", "call-2", "Edit")
	raise("agent-b", "gen-1", "call-3", "Write")
	if got := len(srv.permissionTools); got != 3 {
		t.Fatalf("precondition: %d pending tool names, want 3", got)
	}

	// The unsolicited crash path: the registry's exit hook, with no resolution
	// event, because the process that owed one is gone.
	srv.handleAgentExit("agent-a", "gen-1", "crash")

	if _, ok := srv.permissionTools[permissionToolKey("agent-a", "gen-1", "call-1")]; ok {
		t.Fatal("crashed generation's pending tool name survived its exit")
	}
	if got := srv.permissionTools[permissionToolKey("agent-a", "gen-2", "call-2")]; got != "Edit" {
		t.Fatalf("another generation of the same agent was cleared: %q", got)
	}
	if got := srv.permissionTools[permissionToolKey("agent-b", "gen-1", "call-3")]; got != "Write" {
		t.Fatalf("another agent's pending tool name was cleared: %q", got)
	}
}
