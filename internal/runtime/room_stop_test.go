package runtime

import (
	"errors"
	"testing"
)

// A private same-generation turn that wins the gate remains untouched; if
// room cleanup wins first, a delayed prompt cannot enter that runtime.
func TestRoomIdleStopClaimAndPrivatePromptAreExclusive(t *testing.T) {
	active := &agentState{turnActive: true}
	rt := &ChatRuntime{agents: map[string]*agentState{"a": active}}
	if rt.claimIdleStop("a") {
		t.Fatal("claimed stop over private turn")
	}
	active.mu.Lock()
	active.turnActive = false
	active.mu.Unlock()
	if !rt.claimIdleStop("a") {
		t.Fatal("idle runtime did not admit cleanup")
	}
	if _, _, err := active.claimTurnOrHold("private"); !errors.Is(err, ErrNoHandle) {
		t.Fatalf("private prompt entered stopped runtime: %v", err)
	}
}
