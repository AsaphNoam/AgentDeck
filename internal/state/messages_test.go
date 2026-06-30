package state

import (
	"errors"
	"testing"
	"time"
)

// liveAgent writes an agent + running row so it shows up in LiveAgents.
func liveAgent(t *testing.T, st *Store, id, name, role, project string) {
	t.Helper()
	a := Agent{
		AgentID: id, Name: name, Role: role, Project: project,
		Backend: "claude", Model: "sonnet", Interface: "chat",
		CreatedAt: time.Now().UTC(),
	}
	if err := st.WriteAgent(a); err != nil {
		t.Fatalf("WriteAgent %s: %v", id, err)
	}
	if err := st.WriteRunning(RunningEntry{
		AgentID: id, PID: 100, SessionID: "s_" + id, Interface: "chat", StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteRunning %s: %v", id, err)
	}
}

func TestResolveRecipient(t *testing.T) {
	st, _ := newTestStore(t)
	liveAgent(t, st, "a_impl1", "Atlas", "implementer", "my-app")
	liveAgent(t, st, "a_rev1", "Nova", "reviewer", "my-app")

	cases := []struct {
		name, to, wantID string
	}{
		{"exact agent_id", "a_rev1", "a_rev1"},
		{"role@project", "reviewer@my-app", "a_rev1"},
		{"name case-insensitive", "nOvA", "a_rev1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, cands, err := st.ResolveRecipient(c.to)
			if err != nil {
				t.Fatalf("ResolveRecipient(%q) err = %v", c.to, err)
			}
			if id != c.wantID || cands != nil {
				t.Fatalf("ResolveRecipient(%q) = %q,%v want %q,nil", c.to, id, cands, c.wantID)
			}
		})
	}

	t.Run("not found", func(t *testing.T) {
		if _, _, err := st.ResolveRecipient("ghost@my-app"); !errors.Is(err, ErrRecipientNotFound) {
			t.Fatalf("err = %v, want ErrRecipientNotFound", err)
		}
	})

	t.Run("stopped recipient not addressable", func(t *testing.T) {
		// Removing the running row makes the agent non-live → not resolvable.
		if err := st.DeleteRunning("a_rev1"); err != nil {
			t.Fatalf("DeleteRunning: %v", err)
		}
		if _, _, err := st.ResolveRecipient("reviewer@my-app"); !errors.Is(err, ErrRecipientNotFound) {
			t.Fatalf("err = %v, want ErrRecipientNotFound for stopped agent", err)
		}
	})
}

func TestResolveRecipientAmbiguous(t *testing.T) {
	st, _ := newTestStore(t)
	liveAgent(t, st, "a_r1", "Echo", "reviewer", "my-app")
	liveAgent(t, st, "a_r2", "Echo", "reviewer", "my-app") // same role@project AND name

	// role@project ambiguity.
	_, cands, err := st.ResolveRecipient("reviewer@my-app")
	var ambErr *AmbiguousError
	if !errors.As(err, &ambErr) || !errors.Is(err, ErrAmbiguousRecipient) {
		t.Fatalf("err = %v, want AmbiguousError", err)
	}
	if len(ambErr.Candidates) != 2 || len(cands) != 0 {
		t.Fatalf("candidates from error = %d (return slice %d), want 2/0", len(ambErr.Candidates), len(cands))
	}

	// name ambiguity.
	if _, _, err := st.ResolveRecipient("Echo"); !errors.Is(err, ErrAmbiguousRecipient) {
		t.Fatalf("name ambiguity err = %v, want ErrAmbiguousRecipient", err)
	}
}

func TestListMessagesOrderingAndLimit(t *testing.T) {
	st, _ := newTestStore(t)
	base := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if _, err := st.InsertMessage(Message{
			FromAgent: "a_from", FromAddress: "x@y", FromName: "X",
			ToAgent: "a_to", Body: "m", CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("InsertMessage %d: %v", i, err)
		}
	}
	msgs, err := st.ListMessages("a_to", true, 2)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("limit not applied: got %d, want 2", len(msgs))
	}
	if !msgs[0].CreatedAt.Before(msgs[1].CreatedAt) {
		t.Fatalf("not ascending by created_at: %v then %v", msgs[0].CreatedAt, msgs[1].CreatedAt)
	}
}
