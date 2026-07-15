package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/backend"
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

// TestChatCodexBackendEndToEnd exercises the codex-acp backend through the chat
// runtime end-to-end: launch → prompt → stream → stop → native resume. The
// per-backend adapter (binary/env/resume) is the only difference from claude;
// here the fakeacp command override stands in for the real codex-acp CLI (the
// credentialed live Codex run is gated, like the Phase 1 real-CLI acceptance).
func TestChatCodexBackendEndToEnd(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	ctx := context.Background()
	spec.BackendType = "codex-acp"
	spec.Agent.Backend = "codex"
	spec.ModelID = "gpt-5.5"

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("codex Start: %v", err)
	}
	if st, err := c.store.ReadStatus(h.AgentID); err != nil || st.State != "idle" {
		t.Fatalf("post-start status = %+v err=%v, want idle", st, err)
	}

	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := c.SendPrompt(ctx, h.AgentID, "hello codex"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	evs := drainTurn(t, ch)
	if evs[len(evs)-1].Type != EvTurnEnd {
		t.Fatalf("codex turn last event = %q, want turn_end", evs[len(evs)-1].Type)
	}
	var texts int
	for _, e := range evs {
		if e.Type == EvAssistantText {
			texts++
		}
	}
	if texts < 1 {
		t.Fatalf("codex turn produced no assistant text")
	}
	unsub()

	if err := c.Stop(ctx, h.AgentID); err != nil {
		t.Fatalf("codex Stop: %v", err)
	}
	if _, err := c.store.ReadRunning(h.AgentID); err == nil {
		t.Fatalf("running row should be gone after Stop")
	}

	// Native resume: same agent_id, fresh running row.
	spec.LastSessionID = h.SessionID
	rh, err := c.Resume(ctx, spec, h.SessionID)
	if err != nil {
		t.Fatalf("codex Resume: %v", err)
	}
	t.Cleanup(func() { c.Stop(ctx, rh.AgentID) })
	if rh.AgentID != h.AgentID {
		t.Fatalf("resume agent_id = %q, want %q (stable)", rh.AgentID, h.AgentID)
	}
	if _, err := c.store.ReadRunning(rh.AgentID); err != nil {
		t.Fatalf("running row missing after resume: %v", err)
	}
}

// TestOpenCodeChatE2E and TestOpenHandsChatE2E exercise the Phase 7 backends
// through the chat runtime end-to-end (launch → prompt → stream → stop → native
// resume). Like the codex e2e, fakeacp stands in for the real CLI (the
// credentialed live run is gated, §7.4); the only difference from claude is the
// per-backend adapter, so a green run proves the adapters ride the shared
// runtime with no runtime branch.
func TestOpenCodeChatE2E(t *testing.T) {
	runNewBackendChatE2E(t, "opencode-acp", "opencode", "anthropic/claude-sonnet-4-5")
}
func TestOpenHandsChatE2E(t *testing.T) {
	runNewBackendChatE2E(t, "openhands-acp", "openhands", "anthropic/claude-sonnet-4-5")
}

func runNewBackendChatE2E(t *testing.T, backendType, backendID, modelID string) {
	t.Helper()
	c, spec := newChatTest(t, "stream_text")
	ctx := context.Background()
	spec.BackendType = backendType
	spec.Agent.Backend = backendID
	spec.ModelID = modelID

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("%s Start: %v", backendType, err)
	}
	if st, err := c.store.ReadStatus(h.AgentID); err != nil || st.State != "idle" {
		t.Fatalf("%s post-start status = %+v err=%v, want idle", backendType, st, err)
	}

	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := c.SendPrompt(ctx, h.AgentID, "hello"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	evs := drainTurn(t, ch)
	if evs[len(evs)-1].Type != EvTurnEnd {
		t.Fatalf("%s turn last event = %q, want turn_end", backendType, evs[len(evs)-1].Type)
	}
	var texts int
	for _, e := range evs {
		if e.Type == EvAssistantText {
			texts++
		}
	}
	if texts < 1 {
		t.Fatalf("%s turn produced no assistant text", backendType)
	}
	unsub()

	if err := c.Stop(ctx, h.AgentID); err != nil {
		t.Fatalf("%s Stop: %v", backendType, err)
	}
	if _, err := c.store.ReadRunning(h.AgentID); err == nil {
		t.Fatalf("running row should be gone after Stop")
	}

	// Native resume: same agent_id, fresh running row.
	spec.LastSessionID = h.SessionID
	rh, err := c.Resume(ctx, spec, h.SessionID)
	if err != nil {
		t.Fatalf("%s Resume: %v", backendType, err)
	}
	t.Cleanup(func() { c.Stop(ctx, rh.AgentID) })
	if rh.AgentID != h.AgentID {
		t.Fatalf("%s resume agent_id = %q, want %q (stable)", backendType, rh.AgentID, h.AgentID)
	}
	if _, err := c.store.ReadRunning(rh.AgentID); err != nil {
		t.Fatalf("%s running row missing after resume: %v", backendType, err)
	}
}

// TestSkipPermissionsEnvOpenCode proves the yolo mapping reaches the spawned
// process env: OpenCode gets OPENCODE_CONFIG_CONTENT only when skip=true, and
// OpenHands always carries LLM_MODEL (model-via-env). Asserted on the real
// exec.Cmd spawnCmd builds, so the adapter→runtime wiring is covered end to end.
func TestSkipPermissionsEnvOpenCode(t *testing.T) {
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	c := NewChatRuntime(st)

	hasEnv := func(env []string, key string) (string, bool) {
		for _, kv := range env {
			if strings.HasPrefix(kv, key+"=") {
				return strings.TrimPrefix(kv, key+"="), true
			}
		}
		return "", false
	}

	ocAd, _ := backend.For("opencode-acp")
	base := LaunchSpec{Cwd: t.TempDir(), ModelID: "anthropic/claude-sonnet-4-5", Env: []string{"HOME=/x"}}

	// skip=false → no OPENCODE_CONFIG_CONTENT.
	if _, ok := hasEnv(c.spawnCmd(ocAd, base).Env, "OPENCODE_CONFIG_CONTENT"); ok {
		t.Fatal("opencode skip=false must not set OPENCODE_CONFIG_CONTENT")
	}
	// skip=true → yolo config injected.
	yolo := base
	yolo.SkipPerms = true
	v, ok := hasEnv(c.spawnCmd(ocAd, yolo).Env, "OPENCODE_CONFIG_CONTENT")
	if !ok || !strings.Contains(v, `"permission"`) {
		t.Fatalf("opencode skip=true OPENCODE_CONFIG_CONTENT = %q, want a permission config", v)
	}

	// OpenHands: LLM_MODEL carries the model regardless of skip; a shell LLM_MODEL
	// is stripped so the adapter value is authoritative.
	ohAd, _ := backend.For("openhands-acp")
	ohSpec := LaunchSpec{Cwd: t.TempDir(), ModelID: "anthropic/claude-sonnet-4-5", Env: []string{"HOME=/x", "LLM_MODEL=shell-leak"}}
	got, ok := hasEnv(c.spawnCmd(ohAd, ohSpec).Env, "LLM_MODEL")
	if !ok || got != "anthropic/claude-sonnet-4-5" {
		t.Fatalf("openhands LLM_MODEL = %q (ok=%v), want the adapter model, not the shell leak", got, ok)
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
	if budget, err := c.store.CurrentTurnBudget(h.AgentID, 15); err != nil || budget.TurnID != "t_000000000001" || budget.Remaining != 15 {
		t.Fatalf("turn budget after SendPrompt = %+v err=%v, want fresh t_000000000001", budget, err)
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
	// Keep this deterministic on developer machines that have codex-acp installed:
	// the test needs a spawn failure before the nil store is ever reached.
	c.SetCommand(filepath.Join(t.TempDir(), "missing-acp"))
	if _, err := c.Start(context.Background(), LaunchSpec{BackendType: "codex-acp"}); err == nil {
		t.Fatal("codex-acp Start should error")
	}
}

// TestStartProtocolVersionMismatch verifies Start fails (not just warns) when
// the adapter negotiates an ACP protocol version outside the pinned range.
func TestStartProtocolVersionMismatch(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	spec.Env = append(spec.Env, "FAKEACP_PROTO_VERSION=99")

	_, err := c.Start(context.Background(), spec)
	if err == nil {
		t.Fatal("Start should fail on incompatible protocol version")
	}
	if !errors.Is(err, ErrProtocolVersion) {
		t.Fatalf("err = %v, want ErrProtocolVersion", err)
	}
}

// TestResumeSessionLoadAppliesMCP guards the BLOCKING finding that a successful
// session/load resume must still carry the freshly-minted MCP registration that
// Phase 5 messaging depends on — not only the session/new fallback path.
func TestResumeSessionLoadAppliesMCP(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	ctx := context.Background()

	dump := filepath.Join(t.TempDir(), "load_params.json")
	spec.Env = append(spec.Env, "FAKEACP_LOAD_DUMP="+dump)
	spec.HookToken = "tok-123"
	spec.MCPServers = []MCPServerSpec{{
		Name:    "agentdeck-messaging",
		Command: "/usr/bin/agentdeck",
		Args:    []string{"mcp-stdio", "--agent", spec.Agent.AgentID, "--token", "tok-123"},
		Env:     []string{"X=1"},
	}}

	h, err := c.Resume(ctx, spec, "prior-session-id")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	t.Cleanup(func() { c.Stop(ctx, h.AgentID) })

	// fakeacp's session/load succeeds → resumed via the load path.
	if h.SessionID != "fake-sess-loaded" {
		t.Fatalf("sessionID = %q, want fake-sess-loaded (load path)", h.SessionID)
	}

	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("read load-params dump (session/load not invoked?): %v", err)
	}
	var params struct {
		SessionID  string `json:"sessionId"`
		Cwd        string `json:"cwd"`
		MCPServers []struct {
			Name string `json:"name"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatalf("unmarshal load params: %v\n%s", err, raw)
	}
	if params.SessionID != "prior-session-id" {
		t.Fatalf("load sessionId = %q, want prior-session-id", params.SessionID)
	}
	if len(params.MCPServers) != 1 || params.MCPServers[0].Name != "agentdeck-messaging" {
		t.Fatalf("load mcpServers = %+v, want the fresh messaging server", params.MCPServers)
	}
}

func TestCheckMessagesInjectsNudgeTurn(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	dump := filepath.Join(t.TempDir(), "prompt.json")
	spec.Env = append(spec.Env, "FAKEACP_PROMPT_DUMP="+dump)
	ctx := context.Background()

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { c.Stop(ctx, h.AgentID) })
	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()
	run, err := c.store.ReadRunning(h.AgentID)
	if err != nil {
		t.Fatalf("ReadRunning: %v", err)
	}
	if err := c.CheckMessages(ctx, run.PID); err != nil {
		t.Fatalf("CheckMessages: %v", err)
	}
	if budget, err := c.store.CurrentTurnBudget(h.AgentID, 15); err != nil || budget.TurnID != "t_000000000001" || budget.Remaining != 15 {
		t.Fatalf("turn budget after nudge = %+v err=%v, want fresh t_000000000001", budget, err)
	}
	_ = drainTurn(t, ch)

	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("read prompt dump: %v", err)
	}
	if !strings.Contains(string(raw), "check_messages") {
		t.Fatalf("nudge prompt = %s, want check_messages instruction", raw)
	}
	final, err := c.store.ReadStatus(h.AgentID)
	if err != nil {
		t.Fatalf("ReadStatus final: %v", err)
	}
	if final.State != "idle" {
		t.Fatalf("final status = %+v, want idle", final)
	}
}
