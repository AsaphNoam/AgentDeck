package bus

import (
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/state"
)

func TestPublishDeliversInOrderToSubscribers(t *testing.T) {
	b := New()
	b.now = func() time.Time { return time.UnixMilli(1000).UTC() }
	ch1, unsub1 := b.Subscribe()
	defer unsub1()
	ch2, unsub2 := b.Subscribe()
	defer unsub2()

	id := "a_8f3c12"
	b.Publish("state_update", &id, map[string]string{"state": "idle"})
	b.Publish("state_update", &id, map[string]string{"state": "busy"})

	for i, ch := range []<-chan Event{ch1, ch2} {
		first := <-ch
		second := <-ch
		if first.Seq != 1 || second.Seq != 2 {
			t.Fatalf("subscriber %d seqs = %d,%d want 1,2", i, first.Seq, second.Seq)
		}
		if first.TS != 1000 || second.TS != 1000 {
			t.Fatalf("subscriber %d timestamps = %d,%d want 1000", i, first.TS, second.TS)
		}
	}
}

func TestPublishDropsOldestForSlowSubscriber(t *testing.T) {
	b := New()
	ch, unsub := b.Subscribe()
	defer unsub()
	id := "a_8f3c12"

	for i := 0; i < BufferSize+5; i++ {
		b.Publish("new_message", &id, i)
	}

	got := []uint64{}
	for i := 0; i < BufferSize; i++ {
		got = append(got, (<-ch).Seq)
	}
	if got[0] != 6 {
		t.Fatalf("first retained seq = %d, want 6 after dropping oldest 5", got[0])
	}
	if got[len(got)-1] != BufferSize+5 {
		t.Fatalf("last retained seq = %d, want %d", got[len(got)-1], BufferSize+5)
	}
}

func TestStateUpdateMaintainsSnapshot(t *testing.T) {
	b := New()
	b.PublishStateUpdate(state.AgentStateUpdate{AgentState: state.AgentState{AgentID: "a_1", State: "idle"}})
	b.PublishStateUpdate(state.AgentStateUpdate{AgentState: state.AgentState{AgentID: "a_2", State: "busy"}})
	if got := len(b.Snapshot()); got != 2 {
		t.Fatalf("snapshot len = %d, want 2", got)
	}
	b.PublishStateUpdate(state.AgentStateUpdate{AgentState: state.AgentState{AgentID: "a_1"}, Removed: true})
	snap := b.Snapshot()
	if len(snap) != 1 || snap[0].AgentID != "a_2" {
		t.Fatalf("snapshot after removal = %+v, want only a_2", snap)
	}
}
