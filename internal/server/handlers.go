package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/agentdeck/agentdeck/internal/store"
	"github.com/agentdeck/agentdeck/internal/version"
)

// HTTP handlers. All are GET-only; the 1.22 mux enforces the method, returning
// 405 for non-GET requests to a registered GET route.

// healthResponse is the GET /api/health body.
type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Time    string `json:"time"`
}

// handleHealth always returns 200 if the process is up. It performs no store reads.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Version: version.Version,
		Time:    time.Now().UTC().Format(time.RFC3339),
	})
}

// sessionEntry is one element of GET /api/sessions: an agent identity with its
// matched running entry (omitted if the agent has none).
type sessionEntry struct {
	store.Agent
	Running *runningView `json:"running,omitempty"`
}

// runningView is the running sub-object embedded in a session entry. AgentID is
// dropped (already present on the parent agent) to match the documented shape.
type runningView struct {
	PID       int       `json:"pid"`
	SessionID string    `json:"session_id"`
	Interface string    `json:"interface"`
	TTY       string    `json:"tty,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

// handleSessions joins running/*.json with agents/*.json. Only agents that have a
// running entry are returned. An empty store yields [] (never null).
func (s *Server) handleSessions(w http.ResponseWriter, _ *http.Request) {
	running, err := s.store.ListRunning()
	if err != nil {
		s.log.Error("sessions: list running", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	agents, err := s.store.ListAgents()
	if err != nil {
		s.log.Error("sessions: list agents", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	byID := make(map[string]store.Agent, len(agents))
	for _, a := range agents {
		byID[a.AgentID] = a
	}

	out := []sessionEntry{}
	for _, r := range running {
		a, ok := byID[r.AgentID]
		if !ok {
			// running entry with no identity: skip (orphan).
			continue
		}
		out = append(out, sessionEntry{
			Agent: a,
			Running: &runningView{
				PID:       r.PID,
				SessionID: r.SessionID,
				Interface: r.Interface,
				TTY:       r.TTY,
				StartedAt: r.StartedAt,
			},
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleRoles returns the role map. Corrupt entries are already skipped by the store.
func (s *Server) handleRoles(w http.ResponseWriter, _ *http.Request) {
	roles, err := s.store.ListRoles()
	if err != nil {
		s.log.Error("roles: list", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

// handleProjects returns the project map.
func (s *Server) handleProjects(w http.ResponseWriter, _ *http.Request) {
	projects, err := s.store.ListProjects()
	if err != nil {
		s.log.Error("projects: list", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

// handleBackends returns backends.json, falling back to the in-memory default on
// missing/corrupt (still 200), per §6/§7.
func (s *Server) handleBackends(w http.ResponseWriter, _ *http.Request) {
	b, err := s.store.ReadBackends()
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrCorrupt) {
			s.log.Warn("backends: falling back to default", "err", err)
			writeJSON(w, http.StatusOK, store.DefaultBackends())
			return
		}
		s.log.Error("backends: read", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// handleLayout returns layout.json, falling back to the default on missing/corrupt.
func (s *Server) handleLayout(w http.ResponseWriter, _ *http.Request) {
	l, err := s.store.ReadLayout()
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrCorrupt) {
			s.log.Warn("layout: falling back to default", "err", err)
			writeJSON(w, http.StatusOK, store.DefaultLayout())
			return
		}
		s.log.Error("layout: read", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

// handleAPINotFound is the catch-all for unmatched /api/* paths → 404 JSON.
func (s *Server) handleAPINotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not found")
}
