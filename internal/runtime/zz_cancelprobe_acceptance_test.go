//go:build acceptance

// Temporary probe (not committed): live codex-acp launch/turn/stop/resume plus
// CODEX_CONFIG developer_instructions delivery (FS-09.A7/A11, TS-04).
package runtime

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

func jsonUnmarshalProbe(b []byte, v any) error { return json.Unmarshal(b, v) }

const codexSecret = "XYZZY-7788"

func startCodexAgent(t *testing.T, ctx context.Context, id, cwd, sysPrompt, resumeID string) (*ChatRuntime, *Handle, <-chan Event) {
	t.Helper()
	bin := os.Getenv("CODEX_ACP_CMD")
	if bin == "" {
		bin = "codex-acp"
	}
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("codex adapter %q not on PATH; skipping", bin)
	}

	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	agent := state.Agent{
		AgentID: id, Name: "Codexy", Role: "implementer", Project: "acc",
		Backend: "codex", Model: "gpt-5.6-sol", Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	if err := st.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}

	c := NewChatRuntime(st)
	c.SetCommand(bin)
	spec := LaunchSpec{
		Agent: agent, Cwd: cwd, BackendType: "codex-acp",
		ModelID: "gpt-5.6-sol", SkipPerms: true, Env: os.Environ(),
		SystemPrompt: sysPrompt, LastSessionID: resumeID,
	}

	var h *Handle
	if resumeID == "" {
		h, err = c.Start(ctx, spec)
	} else {
		h, err = c.Resume(ctx, spec, resumeID)
	}
	if err != nil {
		t.Fatalf("start/resume codex adapter: %v", err)
	}
	t.Cleanup(func() { c.Stop(context.Background(), h.AgentID) })

	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(unsub)
	return c, h, ch
}

// codexTurn sends a prompt and collects the assistant text until turn_end.
func codexTurn(t *testing.T, ctx context.Context, c *ChatRuntime, h *Handle, ch <-chan Event, prompt string) string {
	t.Helper()
	if err := c.SendPrompt(ctx, h.AgentID, prompt); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	var sb strings.Builder
	deadline := time.After(3 * time.Minute)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("event channel closed mid-turn")
			}
			switch ev.Type {
			case EvAssistantText:
				var d AssistantTextData
				if err := jsonUnmarshalProbe(ev.Data, &d); err == nil {
					sb.WriteString(d.Delta)
				}
			case EvError:
				t.Logf("error event: %s", string(ev.Data))
			case EvTurnEnd:
				t.Logf("turn_end: %s", string(ev.Data))
				return sb.String()
			}
		case <-deadline:
			t.Fatalf("no turn_end within 3m; text so far: %q", sb.String())
		}
	}
}

// TestProbeCodexTurnAndPrompt: launch, one streamed turn, developer_instructions
// delivery via CODEX_CONFIG, then Stop.
func TestProbeCodexTurnAndPrompt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	sys := "You are a test agent. The secret pass phrase is " + codexSecret +
		". If asked for the secret pass phrase, reply with exactly that phrase and nothing else."
	c, h, ch := startCodexAgent(t, ctx, "a_codex01", t.TempDir(), sys, "")
	t.Logf("codex session id = %q pid=%d", h.SessionID, h.Pid)

	out := codexTurn(t, ctx, c, h, ch, "What is the secret pass phrase from your instructions? Reply with only the phrase.")
	t.Logf("codex reply = %q", out)
	if !strings.Contains(out, codexSecret) {
		t.Errorf("developer_instructions not honored: reply %q lacks %q", out, codexSecret)
	}
	if st, err := c.store.ReadStatus(h.AgentID); err != nil || st.State != "idle" {
		t.Errorf("post-turn status = %+v err=%v, want idle", st, err)
	}

	if err := c.Stop(ctx, h.AgentID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	if pidAlive(h.Pid) {
		t.Error("codex process group still alive after Stop")
	}
	if _, err := c.store.ReadRunning(h.AgentID); err == nil {
		t.Error("running row still present after Stop")
	}
	if st, _ := c.store.ReadStatus(h.AgentID); st.State != "done" {
		t.Errorf("post-stop status = %q, want done", st.State)
	}
}

// TestProbeCodexResume: native session/load carries prior turn history.
func TestProbeCodexResume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cwd := t.TempDir()
	c, h, ch := startCodexAgent(t, ctx, "a_codex02", cwd, "You are a terse test agent.", "")
	first := codexTurn(t, ctx, c, h, ch, "Remember the number 4242 for later. Reply with just: OK")
	t.Logf("first reply = %q; session=%q", first, h.SessionID)
	sid := h.SessionID
	if sid == "" {
		t.Fatal("no native session id captured — cannot exercise resume")
	}
	if err := c.Stop(ctx, h.AgentID); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	c2, h2, ch2 := startCodexAgent(t, ctx, "a_codex02", cwd, "You are a terse test agent.", sid)
	t.Logf("resumed session id = %q (was %q)", h2.SessionID, sid)
	out := codexTurn(t, ctx, c2, h2, ch2, "What number did I ask you to remember? Reply with just the number.")
	t.Logf("resumed reply = %q", out)
	if !strings.Contains(out, "4242") {
		t.Errorf("native resume lost history: reply %q lacks 4242", out)
	}
}
