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
	cmd, err := c.spawnCmd(ocAd, base)
	if err != nil {
		t.Fatalf("spawn openCode: %v", err)
	}
	if _, ok := hasEnv(cmd.Env, "OPENCODE_CONFIG_CONTENT"); ok {
		t.Fatal("opencode skip=false must not set OPENCODE_CONFIG_CONTENT")
	}
	// skip=true → yolo config injected.
	yolo := base
	yolo.SkipPerms = true
	cmd, err = c.spawnCmd(ocAd, yolo)
	if err != nil {
		t.Fatalf("spawn openCode skip=true: %v", err)
	}
	v, ok := hasEnv(cmd.Env, "OPENCODE_CONFIG_CONTENT")
	if !ok || !strings.Contains(v, `"permission"`) {
		t.Fatalf("opencode skip=true OPENCODE_CONFIG_CONTENT = %q, want a permission config", v)
	}

	// OpenHands: LLM_MODEL carries the model regardless of skip; a shell LLM_MODEL
	// is stripped so the adapter value is authoritative.
	ohAd, _ := backend.For("openhands-acp")
	ohSpec := LaunchSpec{Cwd: t.TempDir(), ModelID: "anthropic/claude-sonnet-4-5", Env: []string{"HOME=/x", "LLM_MODEL=shell-leak"}}
	cmd, err = c.spawnCmd(ohAd, ohSpec)
	if err != nil {
		t.Fatalf("spawn openHands: %v", err)
	}
	got, ok := hasEnv(cmd.Env, "LLM_MODEL")
	if !ok || got != "anthropic/claude-sonnet-4-5" {
		t.Fatalf("openhands LLM_MODEL = %q (ok=%v), want the adapter model, not the shell leak", got, ok)
	}
}

// FS-09.A11 / TS-04.R14: codex-acp does not accept ACP systemPrompt. Its
// documented per-process CODEX_CONFIG overlay must preserve user config while
// carrying the frozen AgentDeck role/project prompt for both Start and Resume.
func TestCodexDeveloperInstructionsEnv(t *testing.T) {
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	c := NewChatRuntime(st)
	ad, ok := backend.For("codex-acp")
	if !ok {
		t.Fatal("codex adapter missing")
	}

	base := LaunchSpec{
		Cwd:          t.TempDir(),
		BackendType:  "codex-acp",
		SystemPrompt: "Project context\n\nAct as reviewer.",
		Env: []string{
			"HOME=/x",
			`CODEX_CONFIG={"model":"gpt-5.5","developer_instructions":"Keep commits small."}`,
		},
	}
	hasEnv := func(env []string, key string) (string, bool) {
		for _, kv := range env {
			if strings.HasPrefix(kv, key+"=") {
				return strings.TrimPrefix(kv, key+"="), true
			}
		}
		return "", false
	}
	assertPrompt := func(spec LaunchSpec) {
		t.Helper()
		cmd, err := c.spawnCmd(ad, spec)
		if err != nil {
			t.Fatalf("spawn Codex: %v", err)
		}
		raw, ok := hasEnv(cmd.Env, "CODEX_CONFIG")
		if !ok {
			t.Fatal("CODEX_CONFIG missing")
		}
		var config map[string]any
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			t.Fatalf("CODEX_CONFIG is not JSON: %v", err)
		}
		if config["model"] != "gpt-5.5" {
			t.Fatalf("model = %#v, want preserved value", config["model"])
		}
		wantPrompt := "Keep commits small.\n\n" + spec.StartSystemPrompt()
		if got := config["developer_instructions"]; got != wantPrompt {
			t.Fatalf("developer_instructions = %#v", got)
		}
	}

	assertPrompt(base)
	resume := base
	resume.RuntimeSystemPrompt = "Project context\n\nAct as reviewer.\n\nPrior-turn primer."
	assertPrompt(resume)
}

// FS-09.R43/R44, TS-04.R20/R21: a codex-acp start refreshes its dedicated
// CODEX_HOME profile from the personal Codex setup immediately before its
// process starts.
func TestCodexSpawnRefreshesProfile(t *testing.T) {
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	c := NewChatRuntime(st)
	c.command = "/bin/sh"
	c.cmdArgs = []string{"-c", "exit 0"}
	ad, _ := backend.For("codex-acp")

	personal := filepath.Join(t.TempDir(), "codex")
	if err := os.MkdirAll(personal, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(personal, "config.toml"), []byte("model = \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", personal)

	profile := filepath.Join(t.TempDir(), "codex")
	spec := LaunchSpec{
		Cwd:         t.TempDir(),
		BackendType: "codex-acp",
		Env:         []string{"HOME=/x", "CODEX_HOME=" + profile},
	}
	cmd, err := c.spawnCmd(ad, spec)
	if err != nil {
		t.Fatalf("spawnCmd: %v", err)
	}
	if got := envValue(cmd.Env, "CODEX_HOME"); got != profile {
		t.Fatalf("child CODEX_HOME = %q, want %q", got, profile)
	}
	if err := c.startCmd(cmd, spec); err != nil {
		t.Fatalf("startCmd: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profile, "config.toml")); err != nil {
		t.Fatalf("profile setup not refreshed before process start: %v", err)
	}
}

func TestCodexDeveloperInstructionsRejectInvalidConfig(t *testing.T) {
	c := NewChatRuntime(nil)
	ad, _ := backend.For("codex-acp")
	for name, config := range map[string]string{
		"malformed JSON":                "{not-json",
		"non-object":                    "[]",
		"non-string instructions value": `{"developer_instructions":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := c.spawnCmd(ad, LaunchSpec{
				Cwd:          t.TempDir(),
				BackendType:  "codex-acp",
				SystemPrompt: "Act as reviewer.",
				Env:          []string{"CODEX_CONFIG=" + config},
			})
			if err == nil || strings.Contains(err.Error(), config) {
				t.Fatalf("spawn error = %v, want bounded invalid CODEX_CONFIG error", err)
			}
		})
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
	var userPrompts int
	var seqs []int64
	for _, e := range evs {
		seqs = append(seqs, e.Seq)
		if e.Type == EvAssistantText {
			texts++
		}
		if e.Type == EvUserPrompt {
			var prompt UserPromptData
			if err := json.Unmarshal(e.Data, &prompt); err != nil || prompt.Text != "hello" {
				t.Fatalf("user prompt event = %+v err=%v, want accepted prompt", prompt, err)
			}
			userPrompts++
		}
	}
	if userPrompts != 1 {
		t.Fatalf("want one durable user_text event, got %d", userPrompts)
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

	// turn_end payload carries context_pct = 44000/200000 from the fake
	// adapter's real-shaped usage_update notification.
	var td TurnEndData
	json.Unmarshal(evs[len(evs)-1].Data, &td)
	if td.ContextPct < 0.219 || td.ContextPct > 0.221 {
		t.Fatalf("context_pct = %v, want ~0.22", td.ContextPct)
	}

	// After the turn: idle, busy_since cleared, context_pct written.
	final, _ := c.store.ReadStatus(h.AgentID)
	if final.State != "idle" || final.BusySince != nil {
		t.Fatalf("post-turn status = %+v, want idle + nil busy_since", final)
	}
	if final.ContextPct < 0.219 || final.ContextPct > 0.221 {
		t.Fatalf("post-turn context_pct = %v, want ~0.22", final.ContextPct)
	}
	if final.LastTrace != "Stop" {
		t.Fatalf("post-turn last_trace = %q, want Stop", final.LastTrace)
	}
}

// TestUsageUpdateRepublishesContextPctMidTurn proves a mid-turn usage_update
// republishes context_pct through the durable status row immediately, rather
// than waiting for the next tool/status event or turn_end (TS-04.R25, INV
// §1). The fake adapter emits usage_update, then a marker chunk, then blocks
// on a hold file so the turn stays open while the test observes the status
// row.
func TestUsageUpdateRepublishesContextPctMidTurn(t *testing.T) {
	c, spec := newChatTest(t, "context_mid_turn")
	holdFile := filepath.Join(t.TempDir(), "hold")
	spec.Env = append(spec.Env, "FAKEACP_HOLD_FILE="+holdFile)
	ctx := context.Background()

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(holdFile, []byte("go"), 0o644)
		c.Stop(ctx, h.AgentID)
	})

	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()

	if err := c.SendPrompt(ctx, h.AgentID, "hello"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}

	// Wait for the marker chunk fakeacp emits right after usage_update. ACP
	// notifications are processed in order on one read loop, so observing
	// this chunk proves the usage_update was already applied.
	deadline := time.After(5 * time.Second)
	found := false
	for !found {
		select {
		case ev := <-ch:
			if ev.Type == EvAssistantText {
				var d AssistantTextData
				if err := json.Unmarshal(ev.Data, &d); err == nil && d.Delta == "usage applied" {
					found = true
				}
			}
		case <-deadline:
			t.Fatal("timed out waiting for post-usage_update marker chunk")
		}
	}

	// The scenario is blocked on the hold file, so the turn is still open.
	// The status row must already carry the fresh context_pct.
	st, err := c.store.ReadStatus(h.AgentID)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if st.State != "busy" {
		t.Fatalf("mid-turn state = %q, want busy (turn should still be open)", st.State)
	}
	if st.ContextPct < 0.74 || st.ContextPct > 0.76 {
		t.Fatalf("mid-turn context_pct = %v, want ~0.75 (republished before turn_end)", st.ContextPct)
	}

	// Release the hold so the turn can finish cleanly.
	if err := os.WriteFile(holdFile, []byte("go"), 0o644); err != nil {
		t.Fatalf("write hold file: %v", err)
	}
	drainTurn(t, ch)
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

func TestStartClaudeInitializeExitReturnsSafeGuidance(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	spec.Env = append(spec.Env, "FAKEACP_INIT_STDERR=EMFILE: too many open files at /private/account/token-secret")

	_, err := c.Start(context.Background(), spec)
	if err == nil {
		t.Fatal("Start should fail when the adapter exits during initialize")
	}
	message := err.Error()
	for _, want := range []string{"Claude initialize failed", "could not open required files"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error = %q, want %q", message, want)
		}
	}
	for _, leaked := range []string{"transport closed", "token-secret", "/private/account"} {
		if strings.Contains(message, leaked) {
			t.Fatalf("error = %q, leaked %q", message, leaked)
		}
	}
	if _, lookupErr := c.lookup(spec.Agent.AgentID); !errors.Is(lookupErr, ErrNoHandle) {
		t.Fatalf("lookup after failed startup = %v, want ErrNoHandle", lookupErr)
	}
}

func TestResumeClaudeInitializeTimeoutIsBounded(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	c.startupCallTimeout = 50 * time.Millisecond
	spec.Env = append(spec.Env, "FAKEACP_INIT_HANG=1")

	started := time.Now()
	_, err := c.Resume(context.Background(), spec, "prior-session-id")
	if err == nil {
		t.Fatal("Resume should fail when initialize never answers")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("Resume returned after %s, want bounded startup", elapsed)
	}
	message := err.Error()
	if !strings.Contains(message, "Claude initialize timed out") {
		t.Fatalf("error = %q, want Claude initialize timeout guidance", message)
	}
	if _, lookupErr := c.lookup(spec.Agent.AgentID); !errors.Is(lookupErr, ErrNoHandle) {
		t.Fatalf("lookup after failed resume = %v, want ErrNoHandle", lookupErr)
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

// Regression for the confirmed resume-history finding: ACP session/load does
// not return a new sessionId. The requested id remains authoritative, so an
// empty success result must be treated as ownership of that id — never mistaken
// for load failure and fallen through to session/new, which would abandon the
// provider conversation history (FS-01.R10, FS-03.R13, TS-04.R22, INV §11).
func TestResumeSuccessfulLoadWithoutSessionIDKeepsPriorSession(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	ctx := context.Background()
	newDump := filepath.Join(t.TempDir(), "new_params.json")
	spec.Env = append(spec.Env, "FAKEACP_LOAD_EMPTY=1", "FAKEACP_NEW_DUMP="+newDump)

	h, err := c.Resume(ctx, spec, "prior-session-id")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	t.Cleanup(func() { c.Stop(ctx, h.AgentID) })

	if h.SessionID != "prior-session-id" {
		t.Fatalf("sessionID = %q, want loaded prior-session-id", h.SessionID)
	}
	if _, statErr := os.Stat(newDump); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("session/new was invoked after a successful load (stat err = %v); resume must keep the prior session", statErr)
	}
}

// TestChatEffortPostSessionApplied guards FS-09.A15/R40: for a claude-acp chat
// agent the resolved effort is delivered by a post-session
// `session/set_config_option` call carrying the adapter's option id and value.
func TestChatEffortPostSessionApplied(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	ctx := context.Background()

	dump := filepath.Join(t.TempDir(), "effort_params.json")
	spec.Effort = "high"
	spec.Env = append(spec.Env, "FAKEACP_EFFORT_DUMP="+dump)

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { c.Stop(ctx, h.AgentID) })

	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("read effort dump (set_config_option not invoked?): %v", err)
	}
	var params struct {
		SessionID string `json:"sessionId"`
		ConfigID  string `json:"configId"`
		Value     string `json:"value"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatalf("unmarshal effort params: %v\n%s", err, raw)
	}
	if params.SessionID != h.SessionID {
		t.Fatalf("effort sessionId = %q, want %q", params.SessionID, h.SessionID)
	}
	if params.ConfigID != "effort" || params.Value != "high" {
		t.Fatalf("effort params = %+v, want configId=effort value=high", params)
	}
}

// TestChatEffortPostSessionFailureLeavesNoAgent guards FS-09.A15's teardown
// clause: a rejected post-session effort option fails the launch and registers
// no running agent (the option precedes WriteRunning, so nothing survives).
func TestChatEffortPostSessionFailureLeavesNoAgent(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	ctx := context.Background()

	spec.Effort = "high"
	spec.Env = append(spec.Env, "FAKEACP_EFFORT_FAIL=1")

	if _, err := c.Start(ctx, spec); err == nil {
		t.Fatal("Start should fail when the effort option is rejected")
	}
	if _, err := c.store.ReadRunning(spec.Agent.AgentID); !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("running row after failed effort = %v, want ErrNotFound (no agent registered)", err)
	}
}

func TestStartActivationInjectsPayloadFreeMailTurn(t *testing.T) {
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
	started, err := c.StartActivation(ctx, h.AgentID, "mail", func(turnID string) error {
		return c.store.ResetTurnBudget(h.AgentID, turnID)
	})
	if err != nil || !started {
		t.Fatalf("StartActivation = %v, %v; want started", started, err)
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

// TS-01.R21 / INV §15 — the ordinary busy turn state commits before the provider
// frame. StartActivation used to discard the status write's error and prompt the
// model anyway, so a store failure left durable and UI state `idle` while the
// mail turn was running. The failure must surface, send nothing, and release the
// in-memory turn gate — the caller's already-committed attempted boundary is what
// keeps mail from being replayed.
func TestStartActivationStatusWriteFailureSendsNoPrompt(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	log := filepath.Join(t.TempDir(), "prompts.log")
	spec.Env = append(spec.Env, "FAKEACP_PROMPT_LOG="+log)
	ctx := context.Background()

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { c.Stop(ctx, h.AgentID) })

	// Fail status writes only; reads keep working, so the activation reaches the
	// write instead of being turned away by the idle gate above it.
	if _, err := c.store.DB().Exec(`
CREATE TRIGGER inject_status_write_failure BEFORE INSERT ON status
BEGIN SELECT RAISE(ABORT, 'injected status write failure'); END`); err != nil {
		t.Fatalf("install status trigger: %v", err)
	}

	before := 0
	started, err := c.StartActivation(ctx, h.AgentID, "mail", func(turnID string) error {
		before++
		return nil
	})
	if started || err == nil {
		t.Fatalf("StartActivation = %v, %v; want not started with the status error surfaced", started, err)
	}
	if before != 1 {
		t.Fatalf("attempt boundary ran %d times, want exactly once", before)
	}

	// No provider frame may have been written for the failed activation.
	if raw, rerr := os.ReadFile(log); rerr == nil && len(raw) > 0 {
		t.Fatalf("provider prompt sent despite status write failure: %s", raw)
	}

	// The in-memory turn gate must be free again, otherwise the agent is wedged
	// for the rest of its life over a transient store failure.
	if _, err := c.store.DB().Exec(`DROP TRIGGER inject_status_write_failure`); err != nil {
		t.Fatalf("drop status trigger: %v", err)
	}
	started, err = c.StartActivation(ctx, h.AgentID, "mail", func(turnID string) error {
		return c.store.ResetTurnBudget(h.AgentID, turnID)
	})
	if err != nil || !started {
		t.Fatalf("StartActivation after recovery = %v, %v; want started (turn gate leaked)", started, err)
	}
}

// TS-10.R5 / FS-16.R26 (INV §2) — the instruction and status a host-owned turn
// carries come from the kind's row in the code-owned registry, not from a literal
// at the call site and never from the caller. A kind with no row cannot start, so
// an unregistered kind fails loudly instead of prompting the model with mail's
// instruction, and it leaves the turn gate free for the real turn behind it.
func TestStartActivationRefusesAKindWithNoRegisteredContract(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	dump := filepath.Join(t.TempDir(), "prompt.json")
	spec.Env = append(spec.Env, "FAKEACP_PROMPT_DUMP="+dump)
	ctx := context.Background()

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { c.Stop(ctx, h.AgentID) })

	called := false
	started, err := c.StartActivation(ctx, h.AgentID, "not-a-registered-kind", func(string) error {
		called = true
		return nil
	})
	if started || err == nil {
		t.Fatalf("StartActivation for an unregistered kind = %v, %v; want a refusal", started, err)
	}
	if called {
		t.Fatal("an unregistered kind ran its pre-side-effect callback")
	}
	if _, statErr := os.Stat(dump); statErr == nil {
		t.Fatal("an unregistered kind sent a provider prompt")
	}

	// The gate is untouched, so mail still activates behind the refusal and gets
	// its own registered instruction.
	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()
	started, err = c.StartActivation(ctx, h.AgentID, "mail", func(turnID string) error {
		return c.store.ResetTurnBudget(h.AgentID, turnID)
	})
	if err != nil || !started {
		t.Fatalf("StartActivation after the refusal = %v, %v; want started (turn gate leaked)", started, err)
	}
	_ = drainTurn(t, ch)

	mail, ok := LookupActivationKind("mail")
	if !ok {
		t.Fatal("mail has no registered activation contract")
	}
	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("read prompt dump: %v", err)
	}
	if !strings.Contains(string(raw), mail.Instruction) {
		t.Fatalf("prompt = %s, want the registered instruction %q", raw, mail.Instruction)
	}

	// FS-16.R26 — dependent work has its own row. An agent told to check its
	// messages would do exactly that and never find its task, so the two kinds
	// must not share an instruction or a status.
	dependency, ok := LookupActivationKind("dependency")
	if !ok {
		t.Fatal("dependency has no registered activation contract")
	}
	if dependency.Instruction == mail.Instruction || dependency.StatusDetail == mail.StatusDetail {
		t.Fatalf("dependency inherited mail's contract: %+v", dependency)
	}
	if !strings.Contains(dependency.Instruction, "get_assigned_task") {
		t.Fatalf("dependency instruction = %q, want it to name the tool that reads the assignment",
			dependency.Instruction)
	}
}
