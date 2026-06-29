package bus

import (
	"encoding/json"
	"log/slog"
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
	dropped atomic.Uint64
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
			n := c.dropped.Add(1)
			slog.Warn("bus: slow consumer, event dropped", "total_dropped", n)
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
	var previous *state.AgentStateUpdate
	if update.AgentID != "" && !update.Removed {
		b.mu.RLock()
		if p, ok := b.snapshot[update.AgentID]; ok {
			cp := p
			previous = &cp
		}
		b.mu.RUnlock()
	}
	if update.AgentID != "" {
		b.SetSnapshot(update)
	}
	agentID := update.AgentID
	b.Publish("state_update", &agentID, update)
	if previous != nil && previous.State != update.State {
		switch update.State {
		case "done", "waiting_input":
			b.Publish("notification", &agentID, notificationPayload(update, update.State, nil))
		}
	}
}

func (b *Bus) PublishRuntimeEvent(ev runtime.Event) {
	agentID := ev.AgentID
	b.Publish("new_message", &agentID, ev)
	if ev.Type == runtime.EvPermissionRequest {
		var detail map[string]any
		_ = json.Unmarshal(ev.Data, &detail)
		var update state.AgentStateUpdate
		b.mu.RLock()
		update = b.snapshot[agentID]
		b.mu.RUnlock()
		if update.AgentID == "" {
			update.AgentID = agentID
			update.Name = agentID
		}
		b.Publish("notification", &agentID, notificationPayload(update, "permission_required", detail))
	}
}

func notificationPayload(update state.AgentStateUpdate, typ string, detail map[string]any) map[string]any {
	name := update.Name
	if name == "" {
		name = update.AgentID
	}
	title := name + " needs attention"
	switch typ {
	case "done":
		title = name + " finished"
	case "waiting_input":
		title = name + " needs input"
	case "permission_required":
		title = name + " requests permission"
	case "budget_exceeded":
		title = name + " hit its message budget"
	}
	body := update.Detail
	if body == "" && typ == "permission_required" && detail != nil {
		if reason, ok := detail["reason"].(string); ok {
			body = reason
		}
	}
	if detail == nil {
		detail = map[string]any{}
	}
	return map[string]any{
		"type":              "notification",
		"notification_type": typ,
		"agent_id":          update.AgentID,
		"agent_name":        name,
		"address":           update.Role + "@" + update.Project,
		"title":             title,
		"body":              body,
		"detail":            detail,
		"ts":                btime(),
	}
}

func btime() string {
	return time.Now().UTC().Format(time.RFC3339)
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
