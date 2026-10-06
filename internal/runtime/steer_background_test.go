package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// FS-03.A48, TS-04.R81: a steer that moves the running command to the
// background keeps the tool call, opens its background task through the usual
// task events, and reports the steer as injected. Chuck sends no cancel, and
// targeted Stop reaches a second backgrounded command.
func TestSteerThatBackgroundsTheRunningTool(t *testing.T) {
	cancelLog := filepath.Join(t.TempDir(), "cancel.log")
	c, spec := newChatTest(t, "steer_backgrounds_tool")
	spec.Env = append(spec.Env, "FAKEACP_CAPS=1", "FAKEACP_STEERING=1", "FAKEACP_CANCEL_LOG="+cancelLog)
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
	defer unsub()
	if err := c.SendPrompt(ctx, h.AgentID, "build it"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	for started := false; !started; {
		select {
		case ev := <-ch:
			started = ev.Type == EvToolCall
		case <-time.After(3 * time.Second):
			t.Fatal("tool call never started")
		}
	}
	outcome, err := c.Steer(ctx, h.AgentID, "also check the other file")
	if err != nil || outcome != SteerInjected {
		t.Fatalf("Steer = %q, %v; want injected", outcome, err)
	}

	var got []string
	for _, ev := range drainTurn(t, ch) {
		switch ev.Type {
		case EvToolCall:
			var d ToolCallData
			_ = json.Unmarshal(ev.Data, &d)
			got = append(got, "call|"+d.ToolCallID)
		case EvToolResult:
			var d ToolResultData
			_ = json.Unmarshal(ev.Data, &d)
			got = append(got, "result|"+d.ToolCallID+"|"+d.Status)
		case EvAssistantText:
			got = append(got, "text")
		}
	}
	want := []string{"result|tc_bg|completed", "text", "call|tc_bg2", "result|tc_bg2|completed"}
	if len(got) != len(want) {
		t.Fatalf("events = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d = %q, want %q", i, got[i], want[i])
		}
	}
	transcript, err := c.Transcript(h.AgentID)
	if err != nil {
		t.Fatalf("Transcript: %v", err)
	}
	tasks := taskStates(transcript)
	wantTasks := []string{"|task_1|tc_bg|running|npm run build", "|task_1|tc_bg|completed|", "|task_2|tc_bg2|running|npm run dev"}
	if len(tasks) != len(wantTasks) {
		t.Fatalf("tasks = %q, want %q", tasks, wantTasks)
	}
	for i := range wantTasks {
		if tasks[i] != wantTasks[i] {
			t.Fatalf("task %d = %q, want %q", i, tasks[i], wantTasks[i])
		}
	}

	if err := c.StopBackgroundTask(ctx, h.AgentID, "task_2"); err != nil {
		t.Fatalf("StopBackgroundTask: %v", err)
	}
	select {
	case ev := <-ch:
		if got := taskStates([]Event{ev}); len(got) != 1 || got[0] != "|task_2|tc_bg2|stopped|" {
			t.Fatalf("after stop = %q (%s)", got, ev.Type)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no terminal task update")
	}
	if _, err := os.Stat(cancelLog); !os.IsNotExist(err) {
		t.Fatalf("Chuck sent session/cancel (stat err %v)", err)
	}
}
