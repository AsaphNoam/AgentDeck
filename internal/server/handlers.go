package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/state"
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
	state.Agent
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

// handleSessions joins running rows with agent identity rows. Only agents that have a
// running entry are returned. An empty store yields [] (never null).
func (s *Server) handleSessions(w http.ResponseWriter, _ *http.Request) {
	running, err := s.stateStore.ListRunning()
	if err != nil {
		s.log.Error("sessions: list running", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	agents, err := s.stateStore.ListAgents()
	if err != nil {
		s.log.Error("sessions: list agents", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	byID := make(map[string]state.Agent, len(agents))
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
	roles, err := s.configStore.ListRoles()
	if err != nil {
		s.log.Error("roles: list", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

// handleProjects returns the project map.
func (s *Server) handleProjects(w http.ResponseWriter, _ *http.Request) {
	projects, err := s.configStore.ListProjects()
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
	b, err := s.configStore.ReadBackends()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			s.log.Warn("backends: falling back to default", "err", err)
			writeJSON(w, http.StatusOK, config.DefaultBackends())
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
	l, err := s.configStore.ReadLayout()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, config.ErrCorrupt) {
			s.log.Warn("layout: falling back to default", "err", err)
			writeJSON(w, http.StatusOK, layoutFromConfig(config.DefaultLayout()))
			return
		}
		s.log.Error("layout: read", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, layoutFromConfig(l))
}

type layoutResponse struct {
	Order   []string      `json:"order"`
	Density layoutDensity `json:"density"`
}

type layoutDensity struct {
	PerRow int `json:"perRow"`
	Gap    int `json:"gap"`
}

func layoutFromConfig(l config.Layout) layoutResponse {
	return layoutResponse{
		Order:   append([]string(nil), l.Order...),
		Density: layoutDensity{PerRow: l.Density.CardsPerRow, Gap: l.Density.Gap},
	}
}

func (l layoutResponse) toConfig() config.Layout {
	return config.Layout{
		Order:   append([]string(nil), l.Order...),
		Density: config.Density{CardsPerRow: l.Density.PerRow, Gap: l.Density.Gap},
	}
}

func (s *Server) handlePutLayout(w http.ResponseWriter, r *http.Request) {
	var body layoutResponse
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeHookError(w, http.StatusBadRequest, "bad_request", "malformed JSON")
		return
	}
	if body.Density.PerRow < 1 || body.Density.PerRow > 8 || body.Density.Gap < 0 || body.Density.Gap > 48 {
		writeHookError(w, http.StatusBadRequest, "bad_request", "density out of range")
		return
	}
	for _, id := range body.Order {
		if id == "" {
			writeHookError(w, http.StatusBadRequest, "bad_request", "order contains empty id")
			return
		}
	}
	if err := s.configStore.WriteLayout(body.toConfig()); err != nil {
		s.log.Error("layout: write", "err", err)
		writeHookError(w, http.StatusInternalServerError, "internal", "write layout")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

// handleAPINotFound is the catch-all for unmatched /api/* paths → 404 JSON.
func (s *Server) handleAPINotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not found")
}
