package terminal

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

// Finding 8: two subscribers of ONE hub must both receive the IDENTICAL byte
// stream — not a dup()-style disjoint split. Before the fix each WS got a dup()
// of the master and the two readers consumed different halves.
func TestPTYHubBroadcastsIdenticalBytesToAllSubscribers(t *testing.T) {
	// The "master" is the read end of a pipe; test writes to the write end.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	hub := newPTYHub(&pipeMaster{r: pr, w: io.Discard}, nil)
	defer hub.Close()

	snap1, ch1, unsub1 := hub.subscribe()
	snap2, ch2, unsub2 := hub.subscribe()
	defer unsub1()
	defer unsub2()
	if len(snap1) != 0 || len(snap2) != 0 {
		t.Fatalf("fresh subscribers should see empty scrollback, got %d/%d", len(snap1), len(snap2))
	}

	want := []byte("the quick brown fox jumps over the lazy dog\r\n")
	if _, err := pw.Write(want); err != nil {
		t.Fatalf("write to master: %v", err)
	}

	got1 := collect(t, ch1, len(want))
	got2 := collect(t, ch2, len(want))
	if !bytes.Equal(got1, want) {
		t.Fatalf("subscriber 1 got %q, want %q", got1, want)
	}
	if !bytes.Equal(got2, want) {
		t.Fatalf("subscriber 2 got %q, want %q (subscribers must mirror, not split)", got2, want)
	}
}

// collect reads chunks off a subscriber channel until it has n bytes.
func collect(t *testing.T, ch <-chan []byte, n int) []byte {
	t.Helper()
	var out bytes.Buffer
	deadline := time.After(3 * time.Second)
	for out.Len() < n {
		select {
		case chunk, ok := <-ch:
			if !ok {
				return out.Bytes()
			}
			out.Write(chunk)
		case <-deadline:
			t.Fatalf("timed out collecting; have %d/%d bytes", out.Len(), n)
		}
	}
	return out.Bytes()
}

// Finding 9: with NO subscriber attached, a writer pushing > 64 KiB into the
// master must NOT block — the always-on hub reader drains it. Then a late attach
// must replay bounded scrollback. Before the fix nothing drained the master with
// no WS attached, so the kernel tty buffer filled and the CLI stalled forever.
func TestPTYHubDrainsWithNoSubscriberThenReplaysScrollback(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	hub := newPTYHub(&pipeMaster{r: pr, w: io.Discard}, nil)
	defer hub.Close()

	// Write well beyond any single kernel pipe/tty buffer (~64 KiB) with nobody
	// subscribed. If the reader weren't always-on, this Write would block forever.
	const total = 256 * 1024
	payload := bytes.Repeat([]byte("x"), total)
	done := make(chan error, 1)
	go func() {
		_, werr := pw.Write(payload)
		done <- werr
	}()
	select {
	case werr := <-done:
		if werr != nil {
			t.Fatalf("write to undrained master: %v", werr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("writer blocked with no subscriber — the master is not being drained (Finding 9)")
	}

	// Give the reader a moment to finish draining into scrollback.
	waitFor(t, func() bool {
		snap, _, unsub := hub.subscribe()
		unsub()
		return len(snap) > 0
	})

	// A late attach replays scrollback, bounded to scrollbackBytes.
	snap, _, unsub := hub.subscribe()
	defer unsub()
	if len(snap) == 0 {
		t.Fatal("late attach saw no scrollback")
	}
	if len(snap) > scrollbackBytes {
		t.Fatalf("scrollback %d exceeds bound %d", len(snap), scrollbackBytes)
	}
	// Ring keeps the most recent bytes.
	if !bytes.Equal(snap, payload[total-len(snap):]) {
		t.Fatal("scrollback is not the most-recent slice of output")
	}
}

// Hub teardown: closing the master makes the always-on reader exit and every
// subscriber observe close — no goroutine leak, no panic.
func TestPTYHubCloseUnblocksReaderAndSubscribers(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_ = pw
	hub := newPTYHub(&pipeMaster{r: pr, w: io.Discard}, nil)

	_, ch, unsub := hub.subscribe()
	defer unsub()

	hub.Close()

	// The reader goroutine must exit.
	select {
	case <-hub.done:
	case <-time.After(3 * time.Second):
		t.Fatal("reader goroutine did not exit after Close (leak)")
	}
	// The subscriber channel must be closed.
	select {
	case _, ok := <-ch:
		if ok {
			// Drain any buffered chunk then expect close.
			for range ch {
			}
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber channel not closed after Close")
	}

	// Close is idempotent and a late subscribe returns an already-closed stream.
	hub.Close()
	_, ch2, unsub2 := hub.subscribe()
	defer unsub2()
	if _, ok := <-ch2; ok {
		t.Fatal("late subscribe after Close should yield a closed channel")
	}
}

// A slow subscriber that never drains its buffer is dropped-and-closed so the
// reader never blocks; other subscribers keep flowing. Driven via broadcast()
// directly so the per-chunk overflow is deterministic (an os.Pipe coalesces
// writes, hiding the chunk boundaries this policy keys off).
func TestPTYHubDropsSlowSubscriber(t *testing.T) {
	pr, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	hub := newPTYHub(&pipeMaster{r: pr, w: io.Discard}, nil)
	defer hub.Close()

	_, slow, unsubSlow := hub.subscribe() // never read → buffer fills
	defer unsubSlow()
	_, fast, unsubFast := hub.subscribe()
	defer unsubFast()

	// Fan out more distinct chunks than the per-subscriber buffer can hold. The
	// slow subscriber (never read) overflows and is dropped; the fast one is
	// drained concurrently and keeps up.
	go func() {
		for i := 0; i < ptySubBuffer*2; i++ {
			hub.broadcast([]byte("chunk\n"))
		}
	}()

	// The fast subscriber keeps receiving (drain it).
	got := collect(t, fast, len("chunk\n")*ptySubBuffer)
	if len(got) == 0 {
		t.Fatal("fast subscriber received nothing while slow one blocked the reader")
	}

	// The slow subscriber must eventually be closed (dropped), observable as a
	// closed channel once its buffer is drained.
	waitFor(t, func() bool {
		for {
			select {
			case _, ok := <-slow:
				if !ok {
					return true
				}
			default:
				return false
			}
		}
	})
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

// pipeMaster is a ptyReader backed by an os.Pipe read end (drivable master
// output) and a discard write end (keystrokes go nowhere in these unit tests).
type pipeMaster struct {
	r *os.File
	w io.Writer
}

func (m *pipeMaster) Read(b []byte) (int, error)  { return m.r.Read(b) }
func (m *pipeMaster) Write(b []byte) (int, error) { return m.w.Write(b) }
func (m *pipeMaster) Close() error                { return m.r.Close() }
