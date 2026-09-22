package runtime

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func startSubagentAgent(t *testing.T, env ...string) (*ChatRuntime, *Handle, <-chan Event, *[]ActivityNotice, *sync.Mutex) {
	t.Helper()
	c, spec := newChatTest(t, "subagent_flow")
	spec.Env = append(spec.Env, env...)
	var mu sync.Mutex
	var notices []ActivityNotice
	c.SetActivitySink(func(n ActivityNotice) {
		mu.Lock()
		notices = append(notices, n)
		mu.Unlock()
	})
	ctx := context.Background()
	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = c.Stop(ctx, h.AgentID) })
	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(unsub)
	if err := c.SendPrompt(ctx, h.AgentID, "delegate"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	return c, h, ch, &notices, &mu
}

// A negotiated child announces before its output, its events keep the root
// payloads under activity scope, a grandchild names its immediate parent, and
// an unannounced child session produces nothing (FS-03.A40, TS-04.R63).
func TestNativeChildSessionsAreScopedUnderTheirParent(t *testing.T) {
	_, _, ch, notices, mu := startSubagentAgent(t, "FAKEACP_CAPS=1")
	evs := drainTurn(t, ch)

	child := activityIDFor("th_child_1")
	grand := activityIDFor("th_grand_1")
	type row struct{ typ, activity, parent, data string }
	var got []row
	for _, ev := range evs {
		if ev.Type == EvUserPrompt || ev.Type == EvTurnEnd {
			continue
		}
		got = append(got, row{ev.Type, ev.ActivityID, ev.ParentActivityID, string(ev.Data)})
	}
	want := []row{
		{EvToolCall, "", "", `{"tool_call_id":"tc_1","name":"execute","title":"Root ls","args":{"command":"ls"},"status":"in_progress"}`},
		{EvActivityStarted, child, "", `{"name":"researcher","task":"Find the bug"}`},
		{EvAssistantText, child, "", `{"delta":"child says"}`},
		{EvToolCall, child, "", `{"tool_call_id":"` + child + `/tc_1","name":"exec_command","title":"Child grep","args":{"command":"grep x"},"status":"in_progress"}`},
		{EvToolResult, child, "", `{"tool_call_id":"` + child + `/tc_1","status":"completed","content":[{"newText":"b","oldText":"a","path":"child.go","type":"diff"}]}`},
		{EvDiff, child, "", `{"tool_call_id":"` + child + `/tc_1","path":"child.go","old_text":"a","new_text":"b","patch":""}`},
		{EvActivityStarted, grand, child, `{"name":"helper"}`},
		{EvAssistantText, grand, child, `{"delta":"grandchild says"}`},
		{EvActivityState, grand, child, `{"state":"stopped"}`},
		{EvActivityState, child, "", `{"state":"completed"}`},
		{EvAssistantText, "", "", `{"delta":"root done"}`},
	}
	if len(got) != len(want) {
		t.Fatalf("events =\n%+v\nwant\n%+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d = %+v\nwant %+v", i, got[i], want[i])
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*notices) != 1 || (*notices)[0].ActivityID != child || (*notices)[0].Delta != "child thinks" {
		t.Fatalf("child reasoning = %+v", *notices)
	}
}

// A child permission rides the ordinary single-winner gate under its composite
// id and resolves with the ordinary decision call (FS-03.A40, TS-03.R44).
func TestChildPermissionResolvesThroughTheParentGate(t *testing.T) {
	c, h, ch, _, _ := startSubagentAgent(t, "FAKEACP_CAPS=1", "FAKEACP_CHILD_PERMISSION=1")
	child := activityIDFor("th_child_1")
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Type != EvPermissionRequest {
				continue
			}
			var d PermissionRequestData
			_ = json.Unmarshal(ev.Data, &d)
			if ev.ActivityID != child || d.ToolCallID != child+"/tc_1" {
				t.Fatalf("child permission = scope %q id %q", ev.ActivityID, d.ToolCallID)
			}
			if err := c.Permission(context.Background(), h.AgentID, "tc_1", "approve"); err == nil {
				t.Fatal("the raw provider id must not resolve a child permission")
			}
			if err := c.Permission(context.Background(), h.AgentID, d.ToolCallID, "approve"); err != nil {
				t.Fatalf("Permission: %v", err)
			}
			rest := drainTurn(t, ch)
			if rest[len(rest)-1].Type != EvTurnEnd {
				t.Fatalf("turn did not finish after the child permission: %+v", rest)
			}
			return
		case <-deadline:
			t.Fatal("no child permission request")
		}
	}
}

// Without negotiated subagents no lifecycle is invented: the frames the
// adapter would never send stay ordinary root activity (FS-03.A40).
func TestChildLifecycleNeedsNegotiation(t *testing.T) {
	_, _, ch, _, _ := startSubagentAgent(t)
	for _, ev := range drainTurn(t, ch) {
		if ev.ActivityID != "" || ev.Type == EvActivityStarted || ev.Type == EvActivityState {
			t.Fatalf("unnegotiated child produced scoped event %+v", ev)
		}
	}
}
