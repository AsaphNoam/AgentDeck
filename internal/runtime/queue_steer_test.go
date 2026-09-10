package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// busyAgent starts an agent on the hold_turn scenario and leaves one turn open,
// which is the state every queue and steer requirement is about. It returns the
// handle, the live event channel, and a release func that ends the open turn.
func busyAgent(t *testing.T, env ...string) (*ChatRuntime, *Handle, <-chan Event, func(), string) {
	t.Helper()
	c, spec := newChatTest(t, "hold_turn")
	dir := t.TempDir()
	holdFile := filepath.Join(dir, "hold")
	promptLog := filepath.Join(dir, "prompts.log")
	spec.Env = append(spec.Env, "FAKEACP_HOLD_FILE="+holdFile, "FAKEACP_PROMPT_LOG="+promptLog)
	spec.Env = append(spec.Env, env...)
	ctx := context.Background()

	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	release := func() { _ = os.WriteFile(holdFile, []byte("go"), 0o644) }
	t.Cleanup(func() {
		release()
		_ = c.Stop(ctx, h.AgentID)
	})

	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(unsub)

	if err := c.SendPrompt(ctx, h.AgentID, "first"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	waitForChunk(t, ch, "working")
	return c, h, ch, release, promptLog
}

// waitForChunk blocks until the agent streams the given assistant delta. The ACP
// read loop is ordered, so observing it proves the turn is genuinely open.
func waitForChunk(t *testing.T, ch <-chan Event, want string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Type == EvAssistantText {
				var d AssistantTextData
				if err := json.Unmarshal(ev.Data, &d); err == nil && d.Delta == want {
					return
				}
			}
		case <-deadline:
			t.Fatalf("timed out waiting for the %q chunk", want)
		}
	}
}

// promptTexts returns the prompt text of every session/prompt the adapter
// actually received, which is the only evidence that distinguishes "held" from
// "sent" at the provider boundary.
func promptTexts(t *testing.T, logPath string) []string {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatalf("read prompt log: %v", err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var params struct {
			Prompt []struct {
				Text string `json:"text"`
			} `json:"prompt"`
		}
		if err := json.Unmarshal([]byte(line), &params); err != nil {
			t.Fatalf("decode prompt log line %q: %v", line, err)
		}
		if len(params.Prompt) > 0 {
			out = append(out, params.Prompt[0].Text)
		}
	}
	return out
}

// waitForPrompts polls the adapter's prompt log until it holds want entries, so
// a test can assert on delivery without sleeping for a fixed interval.
func waitForPrompts(t *testing.T, logPath string, want int) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := promptTexts(t, logPath)
		if len(got) >= want {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d provider prompts; got %v", want, got)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestSendPromptOrHoldQueuesOneFollowUpAndDeliversItAsTheNextTurn is FS-03.A31's
// core: the person's send to a busy agent is accepted rather than refused, holds
// exactly one message, a second send replaces rather than stacks, the held
// message reaches neither the provider nor the durable transcript before it is
// sent, and the agent-facing entry point still fails closed throughout.
func TestSendPromptOrHoldQueuesOneFollowUpAndDeliversItAsTheNextTurn(t *testing.T) {
	c, h, _, release, promptLog := busyAgent(t)
	ctx := context.Background()

	held, err := c.SendPromptOrHold(ctx, h.AgentID, "second")
	if err != nil || !held {
		t.Fatalf("SendPromptOrHold while busy = held %v err %v, want held", held, err)
	}
	held, err = c.SendPromptOrHold(ctx, h.AgentID, "third")
	if err != nil || !held {
		t.Fatalf("second SendPromptOrHold = held %v err %v, want held", held, err)
	}

	// TS-01.R29: the shared entry point three agent-facing callers arbitrate on
	// keeps its exact fail-closed contract.
	if err := c.SendPrompt(ctx, h.AgentID, "agent-initiated"); !errors.Is(err, ErrTurnInFlight) {
		t.Fatalf("SendPrompt while busy = %v, want ErrTurnInFlight", err)
	}

	// Nothing held has crossed to the provider or into the durable transcript.
	if got := promptTexts(t, promptLog); len(got) != 1 || got[0] != "first" {
		t.Fatalf("provider prompts while holding = %v, want only [first]", got)
	}
	events, err := c.Transcript(h.AgentID)
	if err != nil {
		t.Fatalf("Transcript: %v", err)
	}
	for _, ev := range events {
		if ev.Type == EvUserPrompt && strings.Contains(string(ev.Data), "third") {
			t.Fatal("held message entered the transcript before it was sent")
		}
	}

	release()
	got := waitForPrompts(t, promptLog, 2)
	if len(got) != 2 {
		t.Fatalf("provider prompts = %v, want exactly two turns", got)
	}
	if got[1] != "third" {
		t.Fatalf("delivered follow-up = %q, want the replacing message %q", got[1], "third")
	}
}

// TestWithdrawnFollowUpIsNeverSent covers the withdraw half of FS-03.A31 and
// TS-03.R38's idempotence: a withdrawn message runs no turn, and withdrawing
// again is success rather than an error.
func TestWithdrawnFollowUpIsNeverSent(t *testing.T) {
	c, h, _, release, promptLog := busyAgent(t)
	ctx := context.Background()

	if _, err := c.SendPromptOrHold(ctx, h.AgentID, "never mind"); err != nil {
		t.Fatalf("SendPromptOrHold: %v", err)
	}
	if err := c.WithdrawHeld(h.AgentID); err != nil {
		t.Fatalf("WithdrawHeld: %v", err)
	}
	if err := c.WithdrawHeld(h.AgentID); err != nil {
		t.Fatalf("second WithdrawHeld = %v, want a no-op success", err)
	}

	release()
	waitForPrompts(t, promptLog, 1)
	// Give a mistaken release a chance to appear before asserting it did not.
	time.Sleep(150 * time.Millisecond)
	if got := promptTexts(t, promptLog); len(got) != 1 {
		t.Fatalf("provider prompts = %v, want only the original turn", got)
	}
}

// TestCancelDeliversTheHeldFollowUp is FS-03.A32's runtime half: cancelling is
// "stop that, do this instead", so the already-submitted correction runs as the
// next turn rather than being discarded with the turn it replaced.
func TestCancelDeliversTheHeldFollowUp(t *testing.T) {
	c, h, _, _, promptLog := busyAgent(t)
	ctx := context.Background()

	if _, err := c.SendPromptOrHold(ctx, h.AgentID, "do this instead"); err != nil {
		t.Fatalf("SendPromptOrHold: %v", err)
	}
	cancelled, err := c.Cancel(ctx, h.AgentID)
	if err != nil || !cancelled {
		t.Fatalf("Cancel = %v err %v, want an interrupted turn", cancelled, err)
	}

	got := waitForPrompts(t, promptLog, 2)
	if got[1] != "do this instead" {
		t.Fatalf("post-cancel prompts = %v, want the held message as the next turn", got)
	}
}

// TestSteerInjectsIntoTheRunningTurn is FS-03.A33's advertised-runtime case: the
// message reaches the adapter's steering extension, not a second prompt, and the
// runtime reports the adapter's own outcome.
func TestSteerInjectsIntoTheRunningTurn(t *testing.T) {
	steerLog := filepath.Join(t.TempDir(), "steer.log")
	c, h, _, _, promptLog := busyAgent(t, "FAKEACP_STEERING=1", "FAKEACP_STEER_LOG="+steerLog)
	ctx := context.Background()

	if r, err := c.store.ReadRunning(h.AgentID); err != nil || !r.SteeringAvailable {
		t.Fatalf("running row steering_available = %v err %v, want true", r.SteeringAvailable, err)
	}

	outcome, err := c.Steer(ctx, h.AgentID, "actually, use the other file")
	if err != nil {
		t.Fatalf("Steer: %v", err)
	}
	if outcome != SteerInjected {
		t.Fatalf("outcome = %q, want %q", outcome, SteerInjected)
	}
	if got := promptTexts(t, steerLog); len(got) != 1 || got[0] != "actually, use the other file" {
		t.Fatalf("steered prompts = %v, want the message once", got)
	}
	// The turn is still the one that was already running: steering starts no
	// second provider turn.
	if got := promptTexts(t, promptLog); len(got) != 1 {
		t.Fatalf("provider prompts = %v, want the steer to add none", got)
	}
}

// TestSteerReportsTheAdapterStartedANewTurn covers the turn that ends between the
// click and the delivery: the adapter starts a fresh turn from the same message
// and the runtime reports which of the two happened rather than inferring it.
func TestSteerReportsTheAdapterStartedANewTurn(t *testing.T) {
	c, h, _, _, _ := busyAgent(t, "FAKEACP_STEERING=1", "FAKEACP_STEER_OUTCOME=startedNewTurn")
	outcome, err := c.Steer(context.Background(), h.AgentID, "late")
	if err != nil {
		t.Fatalf("Steer: %v", err)
	}
	if outcome != SteerNewTurn {
		t.Fatalf("outcome = %q, want %q", outcome, SteerNewTurn)
	}
}

// TestSteerIsUnavailableWithoutTheAdvertisement is the other half of FS-03.A33's
// capability rule: an adapter that does not advertise steering exposes no Steer
// control, and detection comes from the advertisement alone (TS-04.R49).
func TestSteerIsUnavailableWithoutTheAdvertisement(t *testing.T) {
	c, h, _, release, promptLog := busyAgent(t)

	if r, err := c.store.ReadRunning(h.AgentID); err != nil || r.SteeringAvailable {
		t.Fatalf("running row steering_available = %v err %v, want false", r.SteeringAvailable, err)
	}
	if _, err := c.Steer(context.Background(), h.AgentID, "nope"); !errors.Is(err, ErrSteeringUnsupported) {
		t.Fatalf("Steer without the advertisement = %v, want ErrSteeringUnsupported", err)
	}
	// Send is unaffected where Steer is unavailable.
	if held, err := c.SendPromptOrHold(context.Background(), h.AgentID, "queued"); err != nil || !held {
		t.Fatalf("SendPromptOrHold = held %v err %v, want held", held, err)
	}
	release()
	if got := waitForPrompts(t, promptLog, 2); got[1] != "queued" {
		t.Fatalf("prompts = %v, want the queued message delivered", got)
	}
}

// TestSteerPromotesTheHeldFollowUp is FS-03.A33's promotion case: an empty steer
// delivers the held message immediately and clears the hold, so it runs once, in
// the running turn, and not again as its own turn.
func TestSteerPromotesTheHeldFollowUp(t *testing.T) {
	steerLog := filepath.Join(t.TempDir(), "steer.log")
	c, h, _, release, promptLog := busyAgent(t, "FAKEACP_STEERING=1", "FAKEACP_STEER_LOG="+steerLog)
	ctx := context.Background()

	if _, err := c.SendPromptOrHold(ctx, h.AgentID, "changed my mind"); err != nil {
		t.Fatalf("SendPromptOrHold: %v", err)
	}
	outcome, err := c.Steer(ctx, h.AgentID, "")
	if err != nil {
		t.Fatalf("Steer promoting the held message: %v", err)
	}
	if outcome != SteerInjected {
		t.Fatalf("outcome = %q, want %q", outcome, SteerInjected)
	}
	if got := promptTexts(t, steerLog); len(got) != 1 || got[0] != "changed my mind" {
		t.Fatalf("steered prompts = %v, want the held message", got)
	}

	release()
	waitForPrompts(t, promptLog, 1)
	time.Sleep(150 * time.Millisecond)
	if got := promptTexts(t, promptLog); len(got) != 1 {
		t.Fatalf("provider prompts = %v, want the promoted message not to run twice", got)
	}
	if _, err := c.Steer(ctx, h.AgentID, ""); !errors.Is(err, ErrNothingHeld) {
		t.Fatalf("empty steer with nothing held = %v, want ErrNothingHeld", err)
	}
}

// TestRefusedSteerRestoresThePromotedFollowUp guards the one path where a refusal
// could destroy the person's message: a promoted hold exists only in the runtime,
// so a refused steer must put it back rather than swallow it (FS-03.R50).
func TestRefusedSteerRestoresThePromotedFollowUp(t *testing.T) {
	c, h, _, release, promptLog := busyAgent(t, "FAKEACP_STEERING=1", "FAKEACP_STEER_OUTCOME=reject")
	ctx := context.Background()

	if _, err := c.SendPromptOrHold(ctx, h.AgentID, "still wanted"); err != nil {
		t.Fatalf("SendPromptOrHold: %v", err)
	}
	if _, err := c.Steer(ctx, h.AgentID, ""); err == nil {
		t.Fatal("Steer against a refusing adapter = nil, want the adapter's reason")
	}

	release()
	if got := waitForPrompts(t, promptLog, 2); got[1] != "still wanted" {
		t.Fatalf("prompts = %v, want the restored message delivered as the next turn", got)
	}
}

// TestDecodeSteeringSupportReadsTheAdvertisementAlone pins INV §12's defensive
// read: anything other than an explicit supported advertisement is unsupported,
// so an unknown adapter degrades to Send-only instead of exposing a control it
// would reject.
func TestDecodeSteeringSupportReadsTheAdvertisementAlone(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"advertised", `{"_meta":{"steering":{"supported":true}}}`, true},
		{"explicitly unsupported", `{"_meta":{"steering":{"supported":false}}}`, false},
		{"no meta", `{"protocolVersion":1}`, false},
		{"other extensions only", `{"_meta":{"goal":{"version":1}}}`, false},
		{"wrong type", `{"_meta":{"steering":"yes"}}`, false},
		{"unreadable", `not json`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decodeSteeringSupport(json.RawMessage(tc.body)); got != tc.want {
				t.Fatalf("decodeSteeringSupport(%s) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

// TestMapSteerResultRejectsAnythingButTheTwoOutcomes keeps an adapter's "failed"
// or its promptRequired fallback from being read as success and silently
// downgraded to a queue (FS-03.R50, INV §12).
func TestMapSteerResultRejectsAnythingButTheTwoOutcomes(t *testing.T) {
	if got, err := mapSteerResult(json.RawMessage(`{"outcome":"injected"}`), nil); err != nil || got != SteerInjected {
		t.Fatalf("injected = %q err %v", got, err)
	}
	if got, err := mapSteerResult(json.RawMessage(`{"outcome":"startedNewTurn"}`), nil); err != nil || got != SteerNewTurn {
		t.Fatalf("startedNewTurn = %q err %v", got, err)
	}
	for _, body := range []string{`{"outcome":"failed"}`, `{"outcome":"promptRequired"}`, `{}`, `nope`} {
		if got, err := mapSteerResult(json.RawMessage(body), nil); err == nil {
			t.Fatalf("mapSteerResult(%s) = %q, want an error", body, got)
		}
	}
	sentinel := errors.New("transport gone")
	if _, err := mapSteerResult(nil, sentinel); !errors.Is(err, sentinel) {
		t.Fatalf("call error = %v, want it surfaced verbatim", err)
	}
}
