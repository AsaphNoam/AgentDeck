package runtime

import "sync"

// subBuffer is the per-subscriber channel capacity. On overflow the hub drops
// the oldest event so a slow subscriber never blocks the agent's read loop
// (techspec §2, §7.8 backpressure). A seq gap then tells the client it lost data.
const subBuffer = 256

// Hub fans normalized events out to per-agent subscribers via bounded buffered
// channels with drop-oldest semantics. One Hub per agent; its semantics match
// Phase 2's multiplexed bus so the code generalizes (techspec §2).
type Hub struct {
	mu     sync.Mutex
	nextID int
	subs   map[int]chan Event
	closed bool
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{subs: map[int]chan Event{}}
}

// Subscribe returns a buffered event channel and an unsubscribe func. After a
// closed hub, Subscribe returns an already-closed channel and a no-op cancel.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		ch := make(chan Event)
		close(ch)
		return ch, func() {}
	}
	id := h.nextID
	h.nextID++
	ch := make(chan Event, subBuffer)
	h.subs[id] = ch
	return ch, func() { h.unsubscribe(id) }
}

func (h *Hub) unsubscribe(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.subs[id]; ok {
		delete(h.subs, id)
		close(ch)
	}
}

// Publish delivers ev to every subscriber, dropping the oldest buffered event
// for any subscriber whose buffer is full (never blocks).
func (h *Hub) Publish(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs {
		select {
		case ch <- ev:
		default:
			// Buffer full: drop the oldest, then enqueue the newest.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- ev:
			default:
			}
		}
	}
}

// Close closes every subscriber channel and marks the hub closed.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for id, ch := range h.subs {
		delete(h.subs, id)
		close(ch)
	}
}
