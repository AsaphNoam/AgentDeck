package terminal

import (
	"io"
	"sync"
)

// scrollbackBytes bounds the per-agent PTY scrollback ring buffer. A late
// attach (or a reconnect) replays at most this many bytes of recent output
// before the live stream. 256 KiB comfortably covers a screenful of dense CLI
// output plus context while staying small per agent (§3.4).
const scrollbackBytes = 256 * 1024

// ptySubBuffer is the per-subscriber channel capacity (number of read chunks
// buffered before a subscriber is considered too slow). Each chunk is at most
// one master read (≤ ptyReadChunk), so this bounds per-subscriber memory. On
// overflow the hub drops-and-closes that ONE slow subscriber rather than
// blocking the always-on reader — the reader must keep draining the master so
// the CLI never stalls (Finding 9).
const ptySubBuffer = 256

// ptyReadChunk is the master read buffer size (matches the WS pump's frame size).
const ptyReadChunk = 32 * 1024

// ptyReader is the subset of the PTY master the hub reads/writes/resizes. The
// real master is an *os.File; tests supply an os.Pipe-backed fake.
type ptyReader interface {
	io.ReadWriteCloser
}

// ptyHub is a per-agent broadcast hub over one PTY master (§3.4, Findings 8+9).
//
// A single always-on reader goroutine drains the master from the moment the
// agent starts — independent of whether any WebSocket is attached — so a
// long-running CLI never blocks on a full kernel tty buffer (Finding 9). Every
// read is (a) appended to a bounded scrollback ring and (b) fanned out to all
// current subscribers via non-blocking buffered sends; a subscriber that can't
// keep up is dropped-and-closed rather than stalling the reader. All
// subscribers therefore observe identical bytes (fixes Finding 8's dup() split).
//
// Writes (keystrokes) and resizes go to the single shared master, serialized by
// writeMu. Only Close (driven by Stop/CloseTab) closes the master; a subscriber
// leaving never does.
type ptyHub struct {
	master  ptyReader
	setSize func(rows, cols uint16) error

	writeMu sync.Mutex // serializes concurrent WS writes/resizes to the one master

	mu         sync.Mutex
	nextID     int
	subs       map[int]*ptySub
	scrollback []byte // bounded ring; last <= scrollbackBytes of output
	closed     bool
	done       chan struct{} // closed when the reader goroutine has exited
}

// ptySub is one subscriber's buffered channel plus a one-shot closed flag.
type ptySub struct {
	ch     chan []byte
	closed bool
}

// newPTYHub builds a hub over master and starts its always-on reader goroutine.
// setSize handles resize (pty.Setsize on the real master); tests may pass nil.
func newPTYHub(master ptyReader, setSize func(rows, cols uint16) error) *ptyHub {
	h := &ptyHub{
		master:  master,
		setSize: setSize,
		subs:    map[int]*ptySub{},
		done:    make(chan struct{}),
	}
	go h.readLoop()
	return h
}

// readLoop drains the master forever, appending to scrollback and fanning out to
// subscribers, until a read error/EOF (which Close/CloseTab induces by closing
// the master). It then closes the hub so all subscribers observe the end.
func (h *ptyHub) readLoop() {
	defer close(h.done)
	buf := make([]byte, ptyReadChunk)
	for {
		n, err := h.master.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			h.broadcast(chunk)
		}
		if err != nil {
			h.Close()
			return
		}
	}
}

// broadcast appends chunk to the ring and delivers it to every subscriber with a
// non-blocking send. A subscriber whose buffer is full is dropped-and-closed so
// the reader never blocks (Finding 9 is load-bearing on this never blocking).
func (h *ptyHub) broadcast(chunk []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.appendScrollback(chunk)
	for id, s := range h.subs {
		select {
		case s.ch <- chunk:
		default:
			// Slow subscriber: drop it entirely rather than stall the reader or
			// silently lose bytes mid-stream. The WS pump sees the closed channel
			// as EOF and the browser can reconnect for a fresh scrollback replay.
			s.closed = true
			close(s.ch)
			delete(h.subs, id)
		}
	}
}

// appendScrollback appends chunk to the ring, trimming the oldest bytes to keep
// the buffer within scrollbackBytes. Caller holds h.mu.
func (h *ptyHub) appendScrollback(chunk []byte) {
	h.scrollback = append(h.scrollback, chunk...)
	if len(h.scrollback) > scrollbackBytes {
		drop := len(h.scrollback) - scrollbackBytes
		// Re-slice into a fresh backing array so the old head can be GC'd and the
		// buffer never grows unbounded.
		trimmed := make([]byte, scrollbackBytes)
		copy(trimmed, h.scrollback[drop:])
		h.scrollback = trimmed
	}
}

// subscribe registers a new subscriber and returns its channel, the current
// scrollback snapshot to replay first, and an unsubscribe func. After Close it
// returns an already-closed channel and the final scrollback (no live stream).
func (h *ptyHub) subscribe() (snapshot []byte, ch <-chan []byte, unsub func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	snap := make([]byte, len(h.scrollback))
	copy(snap, h.scrollback)
	if h.closed {
		c := make(chan []byte)
		close(c)
		return snap, c, func() {}
	}
	id := h.nextID
	h.nextID++
	s := &ptySub{ch: make(chan []byte, ptySubBuffer)}
	h.subs[id] = s
	return snap, s.ch, func() { h.unsubscribe(id) }
}

// unsubscribe drops a subscriber (WS closed) WITHOUT touching the master — the
// master lives until Stop/CloseTab.
func (h *ptyHub) unsubscribe(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.subs[id]; ok {
		delete(h.subs, id)
		if !s.closed {
			s.closed = true
			close(s.ch)
		}
	}
}

// write forwards keystrokes to the single shared master, serialized so
// concurrent WS writers don't interleave partial writes.
func (h *ptyHub) write(b []byte) (int, error) {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	return h.master.Write(b)
}

// resize applies a window size to the master, serialized with writes.
func (h *ptyHub) resize(rows, cols uint16) error {
	if h.setSize == nil {
		return nil
	}
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	return h.setSize(rows, cols)
}

// Close marks the hub closed and closes every subscriber channel, then closes
// the master to unblock the always-on reader. Safe to call multiple times and
// safe to race with subscribe. readLoop also calls Close on read error/EOF; the
// closed guard makes the second call a no-op, so the master is closed once.
//
// The driver's CloseTab (Stop/crash teardown) ALSO closes the same master fd;
// closing an *os.File twice is harmless (the second returns an error we ignore),
// and only Close/CloseTab ever close it — a subscriber leaving never does.
func (h *ptyHub) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	for id, s := range h.subs {
		if !s.closed {
			s.closed = true
			close(s.ch)
		}
		delete(h.subs, id)
	}
	master := h.master
	h.mu.Unlock()
	// Closing the master unblocks a pending readLoop Read so the goroutine exits.
	_ = master.Close()
}

// wait blocks until the always-on reader goroutine has exited. Used by teardown
// paths (and tests) to guarantee no reader leak after Close/CloseTab.
func (h *ptyHub) wait() { <-h.done }

// hubConn is the PTYConn a WebSocket bridges to: a hub SUBSCRIBER. Read yields
// the scrollback snapshot first (replayed to the fresh xterm) then the live
// stream; Write/Resize go to the shared master via the hub; Close unsubscribes
// (never closes the master). This replaces the per-WS dup() model, so every
// viewer sees identical bytes (Finding 8) and the pumps in bridge.go are
// unchanged.
type hubConn struct {
	hub   *ptyHub
	ch    <-chan []byte
	unsub func()

	pending []byte // scrollback snapshot + any live chunk not yet fully read
	eof     bool
}

func (c *hubConn) Read(b []byte) (int, error) {
	if len(c.pending) > 0 {
		n := copy(b, c.pending)
		c.pending = c.pending[n:]
		return n, nil
	}
	if c.eof {
		return 0, io.EOF
	}
	chunk, ok := <-c.ch
	if !ok {
		c.eof = true
		return 0, io.EOF
	}
	n := copy(b, chunk)
	if n < len(chunk) {
		c.pending = chunk[n:]
	}
	return n, nil
}

func (c *hubConn) Write(b []byte) (int, error) { return c.hub.write(b) }

func (c *hubConn) Resize(rows, cols uint16) error { return c.hub.resize(rows, cols) }

func (c *hubConn) Close() error {
	c.unsub()
	return nil
}

var _ PTYConn = (*hubConn)(nil)
