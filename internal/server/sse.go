package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/agentdeck/agentdeck/internal/bus"
)

const sseKeepalive = 10 * time.Second

// handleEvents implements GET /api/events — the multiplexed Phase 2 SSE stream.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeHookError(w, http.StatusInternalServerError, "internal", "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 2000\n\n")

	// Atomically snapshot + subscribe so a state_update published in between is
	// not lost (it would otherwise be in neither the snapshot nor this client).
	// The channel is buffered, so events published while we write the snapshot
	// below queue until the select loop drains them.
	snapshot, ch, unsub := s.eventBus.SubscribeWithSnapshot()
	defer unsub()

	for _, update := range snapshot {
		agentID := update.AgentID
		writeBusSSE(w, s.eventBus.NewEvent("state_update", &agentID, update))
	}
	writeBusSSE(w, s.eventBus.HydratedMarker())
	flusher.Flush()

	ticker := time.NewTicker(sseKeepalive)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			writeBusSSE(w, ev)
			flusher.Flush()
		case <-ticker.C:
			writeBusSSE(w, s.eventBus.PingEvent())
			flusher.Flush()
		}
	}
}

func writeBusSSE(w http.ResponseWriter, ev bus.Event) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	writeSSE(w, fmt.Sprintf("%d", ev.Seq), ev.Type, payload)
}

func writeSSE(w http.ResponseWriter, id, event string, data []byte) {
	if id != "" {
		fmt.Fprintf(w, "id: %s\n", id)
	}
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", data)
}
