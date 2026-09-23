package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func forkPrefix() []Event {
	raw := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	return []Event{
		{AgentID: "a_src", Seq: 5, Type: EvUserPrompt, Data: raw(UserPromptData{Text: "hello"})},
		{AgentID: "a_src", Seq: 6, Type: EvAssistantText, Data: raw(AssistantTextData{Delta: "hi"})},
		{AgentID: "a_src", Seq: 7, Type: EvTurnEnd, Data: raw(TurnEndData{StopReason: "end_turn"})},
	}
}

func forkSpec(t *testing.T, env ...string) (*ChatRuntime, LaunchSpec, string) {
	t.Helper()
	c, spec := newChatTest(t, "stream_text")
	dir := t.TempDir()
	spec.Env = append(spec.Env, env...)
	spec.Env = append(spec.Env, "FAKEACP_FORK_DUMP="+filepath.Join(dir, "fork.json"), "FAKEACP_DELETE_LOG="+filepath.Join(dir, "delete.json"), "FAKEACP_LOAD_HISTORY=2")
	spec.Fork = &ForkPlan{SourceAgentID: "a_src", SourceSessionID: "src-native", SourceSeq: 7, Prefix: forkPrefix()}
	return c, spec, dir
}

// A clone forks the source's native session in a fresh process, drops the
// provider's replayed history, and records AgentDeck's own copy of the source
// transcript plus a boundary marker under the clone's sequence (FS-01.A20).
func TestForkLaunchCopiesHistoryThroughTheBoundary(t *testing.T) {
	c, spec, dir := forkSpec(t, "FAKEACP_CAPS=1")
	ctx := context.Background()
	h, err := c.Start(ctx, spec)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = c.Stop(ctx, h.AgentID) })
	if h.SessionID != "fake-fork-1" {
		t.Fatalf("session id = %q, want the forked session", h.SessionID)
	}
	var params map[string]any
	raw, _ := os.ReadFile(filepath.Join(dir, "fork.json"))
	_ = json.Unmarshal(raw, &params)
	if params["sessionId"] != "src-native" || params["cwd"] != spec.Cwd || params["_meta"] != nil {
		t.Fatalf("fork params = %s", raw)
	}
	evs, err := c.Transcript(h.AgentID)
	if err != nil {
		t.Fatalf("Transcript: %v", err)
	}
	var got []string
	for _, ev := range evs {
		if ev.AgentID != h.AgentID {
			t.Fatalf("copied event kept the source identity: %+v", ev)
		}
		got = append(got, ev.Type+":"+string(ev.Data))
	}
	want := []string{
		`user_text:{"text":"hello"}`, `assistant_text:{"delta":"hi"}`, `turn_end:{"stop_reason":"end_turn","context_pct":0}`,
		`fork_boundary:{"forked_from_agent_id":"a_src","forked_from_seq":7}`,
	}
	if len(got) != len(want) {
		t.Fatalf("clone transcript = %q", got)
	}
	for i := range want {
		if got[i] != want[i] || evs[i].Seq != int64(i+1) {
			t.Fatalf("event %d = %q seq %d, want %q seq %d", i, got[i], evs[i].Seq, want[i], i+1)
		}
	}
	// The clone continues as its own conversation.
	ch, unsub, err := c.Subscribe(h.AgentID)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()
	if err := c.SendPrompt(ctx, h.AgentID, "next"); err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	if turn := drainTurn(t, ch); turn[len(turn)-1].Type != EvTurnEnd || turn[0].Seq != 5 {
		t.Fatalf("clone turn = %+v", turn)
	}
}

// A peer that does not advertise fork gets no weaker path, and a refused fork
// deletes nothing because nothing was created (FS-01.A20).
func TestForkLaunchRefusalsCreateNothing(t *testing.T) {
	ctx := context.Background()
	c, spec, _ := forkSpec(t)
	if _, err := c.Start(ctx, spec); !errors.Is(err, ErrForkUnavailable) {
		t.Fatalf("unadvertised fork = %v", err)
	}
	if _, err := c.store.ReadRunning(spec.Agent.AgentID); err == nil {
		t.Fatal("unadvertised fork left a running row")
	}

	c, spec, dir := forkSpec(t, "FAKEACP_CAPS=1", "FAKEACP_FORK_FAIL=1")
	if _, err := c.Start(ctx, spec); err == nil {
		t.Fatal("refused fork started an agent")
	}
	if _, err := os.Stat(filepath.Join(dir, "delete.json")); err == nil {
		t.Fatal("a refused fork must not delete anything")
	}
}

type failingForkWriter struct {
	appended  int
	failAfter int
	discarded bool
}

func (w *failingForkWriter) Append(Event) error {
	if w.appended == w.failAfter {
		return errors.New("disk full")
	}
	w.appended++
	return nil
}
func (w *failingForkWriter) Sync() error    { return nil }
func (w *failingForkWriter) Close() error   { return nil }
func (w *failingForkWriter) NextSeq() int64 { return 1 }
func (w *failingForkWriter) Discard() error { w.discarded = true; return nil }

type removalIndexer struct{ removed []string }

func (ix *removalIndexer) UpsertSessionMeta(string, SessionMetaData) error           { return nil }
func (ix *removalIndexer) OnEvent(string, Event) error                               { return nil }
func (ix *removalIndexer) OnTurnEnd(string, TurnRollup) error                        { return nil }
func (ix *removalIndexer) OnEventAndTurnEnd(string, Event, TurnRollup) error         { return nil }
func (ix *removalIndexer) OnEventAndFlushContent(string, Event, int64, string) error { return nil }
func (ix *removalIndexer) RemoveAgent(id string) error {
	ix.removed = append(ix.removed, id)
	return nil
}

// A clone whose history copy fails part-way discards its partial transcript and
// index rows and deletes the forked provider session: no half-copied clone
// survives (TS-02.R35, INV §15).
func TestForkLaunchRollsBackAPartialHistoryCopy(t *testing.T) {
	c, spec, dir := forkSpec(t, "FAKEACP_CAPS=1")
	w := &failingForkWriter{failAfter: 1}
	ix := &removalIndexer{}
	c.SetPersistence(t.TempDir(), func(string, string, *SessionMetaData) (TranscriptWriter, error) { return w, nil }, ix)
	if _, err := c.Start(context.Background(), spec); err == nil {
		t.Fatal("a failed history copy started the clone")
	}
	if w.appended != 1 || !w.discarded {
		t.Fatalf("writer appended %d, discarded %v", w.appended, w.discarded)
	}
	if len(ix.removed) != 1 || ix.removed[0] != spec.Agent.AgentID {
		t.Fatalf("index removals = %v", ix.removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "delete.json")); err != nil {
		t.Fatalf("forked session not deleted: %v", err)
	}
	if _, err := c.store.ReadRunning(spec.Agent.AgentID); err == nil {
		t.Fatal("failed clone left a running row")
	}
}

// A fork whose local commit fails deletes the forked provider session before
// teardown (TS-04.R65, INV §15).
func TestForkLaunchDeletesTheForkWhenLocalCommitFails(t *testing.T) {
	c, spec, dir := forkSpec(t, "FAKEACP_CAPS=1", "FAKEACP_EFFORT_FAIL=1")
	spec.Effort = "high"
	if _, err := c.Start(context.Background(), spec); err == nil {
		t.Fatal("expected the post-fork config failure")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "delete.json"))
	if err != nil {
		t.Fatalf("no session/delete: %v", err)
	}
	var params map[string]string
	_ = json.Unmarshal(raw, &params)
	if params["sessionId"] != "fake-fork-1" {
		t.Fatalf("delete params = %s", raw)
	}
	if _, err := c.store.ReadRunning(spec.Agent.AgentID); err == nil {
		t.Fatal("failed fork left a running row")
	}
}
