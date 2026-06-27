package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
)

// sseKeepalive is the keepalive interval (techspec §7.8, matches Phase 2 §4.3).
const sseKeepalive = 10 * time.Second

// handleEvents implements GET /api/sessions/{id}/events — the interim per-agent
// SSE stream (techspec §7.8). On connect it subscribes to the agent's hub and
// streams every Event as `event: message`. A synthetic status replay gives a
// late-joining client context; keepalives are `event: ping`. The `data:` object
// is byte-identical to the runtime.Event struct (Phase 2 forward-compat).
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	ch, unsub, err := s.registry.Subscribe(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	defer unsub()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, apiError(runtime.CodeInternal, "streaming unsupported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Replay the current status row as a synthetic state_update event so a
	// late-joining client has context (full transcript replay is Phase 4).
	if st, err := s.stateStore.ReadStatus(id); err == nil {
		if payload, err := json.Marshal(st); err == nil {
			writeSSE(w, "", "state_update", payload)
			flusher.Flush()
		}
	}

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
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			writeSSE(w, fmt.Sprintf("%d", ev.Seq), "message", payload)
			flusher.Flush()
		case <-ticker.C:
			ping, _ := json.Marshal(map[string]string{"ts": time.Now().UTC().Format(time.RFC3339)})
			writeSSE(w, "", "ping", ping)
			flusher.Flush()
		}
	}
}

// writeSSE writes one SSE frame: optional id:, event:, then data: and a blank line.
func writeSSE(w http.ResponseWriter, id, event string, data []byte) {
	if id != "" {
		fmt.Fprintf(w, "id: %s\n", id)
	}
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", data)
}
