package server

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/messaging"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

func TestRegisterMessagingMCPWritesHTTPConfigAndCleanup(t *testing.T) {
	srv := testServer(t, true)
	agent := state.Agent{
		AgentID: "a_msgreg", Name: "Atlas", Role: "implementer", Project: "my-app",
		Backend: "claude", Model: "sonnet", Interface: "chat",
	}
	spec, err := srv.registerMessagingMCP(agent)
	if err != nil {
		t.Fatalf("registerMessagingMCP: %v", err)
	}
	if spec.Name != messagingMCPName || spec.Type != "http" || spec.URL != "http://127.0.0.1:4317/mcp" {
		t.Fatalf("spec = %+v, want HTTP messaging spec on configured port", spec)
	}
	token := spec.Headers[messaging.TokenHeader]
	if token == "" {
		t.Fatalf("spec headers = %+v, want token header", spec.Headers)
	}
	if got, ok := srv.messaging.Lookup(token); !ok || got != agent.AgentID {
		t.Fatalf("Lookup(token) = %q,%v want %s,true", got, ok, agent.AgentID)
	}

	path := filepath.Join(srv.configStore.Home(), "mcp", agent.AgentID+".mcp.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mcp config: %v", err)
	}
	var body struct {
		MCPServers map[string]struct {
			Type    string            `json:"type"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal mcp config: %v\n%s", err, raw)
	}
	entry, ok := body.MCPServers[messagingMCPName]
	if !ok || entry.Type != "http" || entry.URL != spec.URL || entry.Headers[messaging.TokenHeader] != token {
		t.Fatalf("config entry = %+v ok=%v, want matching HTTP entry", entry, ok)
	}

	srv.cleanupMessagingMCP(agent.AgentID)
	if _, ok := srv.messaging.Lookup(token); ok {
		t.Fatal("token still registered after cleanup")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config stat after cleanup err = %v, want not exist", err)
	}
}

func TestNudgeOnceWakesIdleAgentAndMarksDelivered(t *testing.T) {
	srv := testServer(t, true)
	fake := buildServerFakeACP(t)
	promptDump := filepath.Join(t.TempDir(), "prompt.json")
	srv.registry.Chat().SetCommand(fake)

	agent := state.Agent{
		AgentID: "a_nudged", Name: "Nova", Role: "reviewer", Project: "my-app",
		Backend: "claude", Model: "sonnet", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	if err := srv.stateStore.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	spec := runtime.LaunchSpec{
		Agent:       agent,
		Cwd:         t.TempDir(),
		BackendType: "claude-acp",
		ModelID:     "claude-sonnet-4-6",
		Env:         []string{"FAKEACP_SCENARIO=stream_text", "FAKEACP_PROMPT_DUMP=" + promptDump, "HOME=" + os.Getenv("HOME")},
	}
	h, err := srv.registry.Launch(context.Background(), spec)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	t.Cleanup(func() { _ = srv.registry.Stop(context.Background(), h.AgentID) })

	msgID, err := srv.stateStore.InsertMessage(state.Message{
		FromAgent: "a_sender", FromAddress: "implementer@my-app", FromName: "Atlas",
		ToAgent: agent.AgentID, Body: "please review",
	})
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	srv.nudgeOnce(context.Background(), agent.AgentID, map[string]nudgeState{})

	deadline := time.Now().Add(3 * time.Second)
	for {
		if raw, err := os.ReadFile(promptDump); err == nil && strings.Contains(string(raw), "check_messages") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for nudge prompt dump")
		}
		time.Sleep(10 * time.Millisecond)
	}
	msgs, err := srv.stateStore.ListMessages(agent.AgentID, false, 0)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].MessageID != msgID || msgs[0].DeliveredVia != "nudge" {
		t.Fatalf("message after nudge = %+v, want delivered_via=nudge", msgs)
	}
}

func buildServerFakeACP(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "fakeacp")
	cmd := exec.Command("go", "build", "-o", out, "../runtime/testdata/fakeacp")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fakeacp: %v\n%s", err, b)
	}
	return out
}
