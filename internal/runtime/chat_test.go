package runtime

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

var (
	fakeOnce sync.Once
	fakePath string
	fakeErr  error
)

// buildFakeACP compiles the standalone fake ACP CLI once and returns its path.
func buildFakeACP(t *testing.T) string {
	t.Helper()
	fakeOnce.Do(func() {
		dir := t.TempDir()
		out := filepath.Join(dir, "fakeacp")
		cmd := exec.Command("go", "build", "-o", out, "./testdata/fakeacp")
		if b, err := cmd.CombinedOutput(); err != nil {
			fakeErr = err
			t.Logf("build fakeacp: %s", b)
			return
		}
		fakePath = out
	})
	if fakeErr != nil {
		t.Fatalf("build fakeacp: %v", fakeErr)
	}
	// The binary lives under the first builder's TempDir, which is removed at
	// that test's end. Rebuild per top-level test by resetting if missing.
	if _, err := os.Stat(fakePath); err != nil {
		out := filepath.Join(t.TempDir(), "fakeacp")
		cmd := exec.Command("go", "build", "-o", out, "./testdata/fakeacp")
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("rebuild fakeacp: %v\n%s", err, b)
		}
		fakePath = out
	}
	return fakePath
}

// newChatTest builds a ChatRuntime wired to the fake CLI plus a temp state store
// pre-seeded with an agent identity row (FK target for running/status).
func newChatTest(t *testing.T, scenario string) (*ChatRuntime, LaunchSpec) {
	t.Helper()
	bin := buildFakeACP(t)

	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	agent := state.Agent{
		AgentID: "a_test01", Name: "Atlas", Role: "implementer",
		Project: "my-app", Backend: "claude", Model: "sonnet-4-6",
		Interface: "chat", CreatedAt: time.Now().UTC(),
	}
	if err := st.WriteAgent(agent); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}

	c := NewChatRuntime(st)
	c.command = bin

	spec := LaunchSpec{
		Agent:       agent,
		Cwd:         t.TempDir(),
		BackendType: "claude-acp",
		ModelID:     "claude-sonnet-4-6",
		Env:         []string{"FAKEACP_SCENARIO=" + scenario, "HOME=" + os.Getenv("HOME")},
	}
	return c, spec
}

// drainTurn collects events from ch until a turn_end (or timeout).
func drainTurn(t *testing.T, ch <-chan Event) []Event {
	t.Helper()
	var got []Event
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, ev)
			if ev.Type == EvTurnEnd {
				return got
			}
		case <-deadline:
			t.Fatalf("timed out; collected %d events", len(got))
		}
	}
}

func TestChatStreamText(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	ctx := context.Background()

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { c.Stop(ctx, h.AgentID) })

	if h.SessionID != "fake-sess-1" {
		t.Fatalf("sessionID = %q, want fake-sess-1", h.SessionID)
	}
	// After Start: running row + idle status row.
	if st, err := c.store.ReadStatus(h.AgentID); err != nil || st.State != "idle" {
		t.Fatalf("post-start status = %+v err=%v, want idle", st, err)
	}
	if _, err := c.store.ReadRunning(h.AgentID); err != nil {
		t.Fatalf("running row missing: %v", err)
	}

	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()

	if err := c.SendPrompt(ctx, h.AgentID, "hello"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	// SendPrompt writes busy synchronously before returning.
	if st, _ := c.store.ReadStatus(h.AgentID); st.State != "busy" {
		t.Fatalf("mid-turn status = %q, want busy", st.State)
	}

	evs := drainTurn(t, ch)
	var texts int
	var seqs []int64
	for _, e := range evs {
		seqs = append(seqs, e.Seq)
		if e.Type == EvAssistantText {
			texts++
		}
	}
	if texts < 2 {
		t.Fatalf("want >=2 assistant_text deltas (incremental), got %d", texts)
	}
	if evs[len(evs)-1].Type != EvTurnEnd {
		t.Fatalf("last event = %q, want turn_end", evs[len(evs)-1].Type)
	}
	// Seq is monotonic starting at 1.
	for i, s := range seqs {
		if s != int64(i+1) {
			t.Fatalf("seq[%d] = %d, want %d (monotonic from 1)", i, s, i+1)
		}
	}

	// turn_end payload carries context_pct = 4200/200000.
	var td TurnEndData
	json.Unmarshal(evs[len(evs)-1].Data, &td)
	if td.ContextPct < 0.02 || td.ContextPct > 0.022 {
		t.Fatalf("context_pct = %v, want ~0.021", td.ContextPct)
	}

	// After the turn: idle, busy_since cleared, context_pct written.
	final, _ := c.store.ReadStatus(h.AgentID)
	if final.State != "idle" || final.BusySince != nil {
		t.Fatalf("post-turn status = %+v, want idle + nil busy_since", final)
	}
	if final.ContextPct < 0.02 || final.ContextPct > 0.022 {
		t.Fatalf("post-turn context_pct = %v, want ~0.021", final.ContextPct)
	}
	if final.LastTrace != "Stop" {
		t.Fatalf("post-turn last_trace = %q, want Stop", final.LastTrace)
	}
}

func TestChatToolFlow(t *testing.T) {
	c, spec := newChatTest(t, "tool_flow")
	ctx := context.Background()

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { c.Stop(ctx, h.AgentID) })

	ch, unsub, _ := c.Subscribe(h.AgentID)
	defer unsub()

	if err := c.SendPrompt(ctx, h.AgentID, "edit the file"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	evs := drainTurn(t, ch)

	var call *ToolCallData
	var result *ToolResultData
	var diff *DiffData
	for _, e := range evs {
		switch e.Type {
		case EvToolCall:
			var d ToolCallData
			json.Unmarshal(e.Data, &d)
			call = &d
		case EvToolResult:
			var d ToolResultData
			json.Unmarshal(e.Data, &d)
			result = &d
		case EvDiff:
			var d DiffData
			json.Unmarshal(e.Data, &d)
			diff = &d
		}
	}
	if call == nil || result == nil || diff == nil {
		t.Fatalf("missing events: call=%v result=%v diff=%v", call, result, diff)
	}
	// All three correlate by tool_call_id.
	if call.ToolCallID != "tc_1" || result.ToolCallID != "tc_1" || diff.ToolCallID != "tc_1" {
		t.Fatalf("tool_call_id mismatch: %q %q %q", call.ToolCallID, result.ToolCallID, diff.ToolCallID)
	}
	if call.Name != "edit" || call.Title != "Edit main.go" {
		t.Fatalf("tool_call name/title = %q/%q", call.Name, call.Title)
	}
	if result.Status != "completed" {
		t.Fatalf("tool_result status = %q, want completed", result.Status)
	}
	if diff.Path != "main.go" || diff.NewText != "b" {
		t.Fatalf("diff = %+v", diff)
	}
}

func TestChatBackendGate(t *testing.T) {
	c := NewChatRuntime(nil)
	if _, err := c.Start(context.Background(), LaunchSpec{BackendType: "codex-acp"}); err == nil {
		t.Fatal("codex-acp Start should error")
	}
}
