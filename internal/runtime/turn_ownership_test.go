package runtime

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// TT-02: pause completion after its idle write but before terminal emission.
// The real Send path must hold B until A's terminal callback has finished.
func TestCompletionRetainsGateAndTurnOwner(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	h, err := c.Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Stop(context.Background(), h.AgentID) })
	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	c.SetStateTouch(func(id string) {
		status, err := c.store.ReadStatus(id)
		if err == nil && status.State == "idle" {
			once.Do(func() { close(entered); <-release })
		}
	})
	if err := c.SendPrompt(context.Background(), h.AgentID, "A"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("completion did not reach idle write")
	}
	held, err := c.SendPromptOrHold(context.Background(), h.AgentID, "B")
	close(release)
	if err != nil || !held {
		t.Fatalf("racing B held = %v, error = %v", held, err)
	}
	first := drainTurn(t, ch)
	second := drainTurn(t, ch)
	ownerA, ownerB := first[0].TurnID, second[0].TurnID
	if ownerA == "" || ownerB == "" || ownerA == ownerB {
		t.Fatalf("owners A=%q B=%q", ownerA, ownerB)
	}
	for _, group := range []struct {
		events []Event
		owner  string
	}{{first, ownerA}, {second, ownerB}} {
		for _, ev := range group.events {
			if ev.TurnID != group.owner {
				t.Fatalf("%s owner = %q, want %q", ev.Type, ev.TurnID, group.owner)
			}
		}
	}
}

// TT-03: descendants announced during a newer root turn still inherit their
// parent's original ownership; an unowned child must stay unowned too.
func TestLateChildActivityRetainsOriginTurn(t *testing.T) {
	c, spec := newChatTest(t, "stream_text")
	h, err := c.Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Stop(context.Background(), h.AgentID) })
	as, _ := c.lookup(h.AgentID)
	spawn := func(parent, child string) {
		t.Helper()
		params, _ := json.Marshal(map[string]any{"sessionId": parent, "update": map[string]any{"sessionUpdate": "subagent_spawned", "subagentSessionId": child}})
		if !c.onSubagentUpdate(as, params) {
			t.Fatal("spawn not handled")
		}
	}
	as.setExecTurn("A")
	spawn(as.sessionID, "child-a")
	as.setExecTurn("")
	spawn(as.sessionID, "unowned")
	as.setExecTurn("B")
	spawn("child-a", "grandchild-a")
	for _, child := range []struct{ id, owner string }{{"child-a", "A"}, {"grandchild-a", "A"}, {"unowned", ""}} {
		scope, ok := as.scopeFor(child.id)
		if !ok {
			t.Fatal("missing child scope")
		}
		for _, typ := range []string{EvToolCall, EvDiff, EvPermissionRequest, EvActivityState} {
			ev := c.emitIn(as, scope, typ, map[string]string{"test": "late"})
			if ev.TurnID != child.owner {
				t.Fatalf("%s %s owner = %q, want %q", child.id, typ, ev.TurnID, child.owner)
			}
		}
	}
	if ev := c.emit(as, EvAssistantText, AssistantTextData{Delta: "B"}); ev.TurnID != "B" {
		t.Fatalf("root owner = %q", ev.TurnID)
	}
}
