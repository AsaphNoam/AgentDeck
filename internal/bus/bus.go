package bus

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

const BufferSize = 256

type Event struct {
	Type    string  `json:"type"`
	Seq     uint64  `json:"seq"`
	TS      int64   `json:"ts"`
	AgentID *string `json:"agent_id"`
	Data    any     `json:"data"`
}

type client struct {
	ch      chan Event
	dropped uint64
}

type Bus struct {
	mu       sync.RWMutex
	nextID   int
	clients  map[int]*client
	snapshot map[string]state.AgentStateUpdate
	seq      atomic.Uint64
	now      func() time.Time
}

func New() *Bus {
	return &Bus{
		clients:  map[int]*client{},
		snapshot: map[string]state.AgentStateUpdate{},
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (b *Bus) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextID
	b.nextID++
	c := &client{ch: make(chan Event, BufferSize)}
	b.clients[id] = c
	return c.ch, func() { b.unsubscribe(id) }
}

func (b *Bus) unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if c, ok := b.clients[id]; ok {
		delete(b.clients, id)
		close(c.ch)
	}
}

func (b *Bus) Publish(typ string, agentID *string, data any) Event {
	ev := b.NewEvent(typ, agentID, data)
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, c := range b.clients {
		select {
		case c.ch <- ev:
		default:
			select {
			case <-c.ch:
			default:
			}
			select {
			case c.ch <- ev:
			default:
			}
			c.dropped++
		}
	}
	return ev
}

func (b *Bus) NewEvent(typ string, agentID *string, data any) Event {
	return Event{
		Type:    typ,
		Seq:     b.seq.Add(1),
		TS:      b.now().UnixMilli(),
		AgentID: agentID,
		Data:    data,
	}
}

func (b *Bus) PingEvent() Event {
	return b.NewEvent("ping", nil, map[string]any{})
}

func (b *Bus) PublishStateUpdate(update state.AgentStateUpdate) {
	if update.AgentID != "" {
		b.SetSnapshot(update)
	}
	agentID := update.AgentID
	b.Publish("state_update", &agentID, update)
}

func (b *Bus) PublishRuntimeEvent(ev runtime.Event) {
	agentID := ev.AgentID
	b.Publish("new_message", &agentID, ev)
}

func (b *Bus) SetSnapshot(update state.AgentStateUpdate) {
	if update.AgentID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if update.Removed {
		delete(b.snapshot, update.AgentID)
		return
	}
	b.snapshot[update.AgentID] = update
}

func (b *Bus) Snapshot() []state.AgentStateUpdate {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]state.AgentStateUpdate, 0, len(b.snapshot))
	for _, update := range b.snapshot {
		out = append(out, update)
	}
	return out
}

func (b *Bus) HydratedMarker() Event {
	agentID := "__hydrated__"
	return b.NewEvent("state_update", &agentID, map[string]bool{"hydrated": true})
}
