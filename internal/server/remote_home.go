package server

import (
	"maps"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/strutil"
)

// attentionItem is one row of the phone's Home (FS-20.R33). Reason is a short,
// in-vocabulary line; message, command, and diff text never appear here.
type attentionItem struct {
	Kind        string    `json:"kind"` // agent | run
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Project     string    `json:"project"`
	State       string    `json:"state"`
	Reason      string    `json:"reason"`
	AgentID     string    `json:"agent_id,omitempty"` // the conversation to open
	StageNumber int       `json:"stage_number,omitempty"`
	StageCount  int       `json:"stage_count,omitempty"`
	Outcome     string    `json:"outcome,omitempty"`
	Since       time.Time `json:"since"`
}

// Attention reasons. Push notifications reuse them, so Home and push cannot
// disagree about what needs the person (TS-13.R10, INV §2).
const (
	reasonPermission = "needs permission"
	reasonQuestion   = "is waiting for your reply"
	reasonError      = "hit an error"
	reasonRunPaused  = "is paused"
)

type attentionLists struct {
	NeedsYou   []attentionItem `json:"needs_you"`
	ActiveRuns []attentionItem `json:"active_runs"`
}

const (
	attentionListLimit = 100
)

// agentHasPendingPermission reports whether any live generation of the agent
// holds an unanswered permission request.
func (s *Server) agentHasPendingPermission(agentID string) bool {
	prefix := agentID + "\x00"
	s.permissionMu.Lock()
	defer s.permissionMu.Unlock()
	for key := range s.permissionTools {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// attention is the one classification of agent and pipeline-run attention
// (TS-13.R20). It deliberately excludes tasks, moving agents, and terminal
// history: those are no longer phone surfaces.
func (s *Server) attention() (attentionLists, error) {
	out := attentionLists{NeedsYou: []attentionItem{}, ActiveRuns: []attentionItem{}}

	for _, a := range s.eventBus.Snapshot() {
		if a.Removed || a.Archived {
			continue
		}
		at := time.UnixMilli(a.UpdatedAt).UTC()
		item := attentionItem{Kind: "agent", ID: a.AgentID, AgentID: a.AgentID, Title: agentTitle(a.AgentState), Project: a.Project, State: a.State, Since: at}
		switch a.State {
		case "waiting_input":
			item.Reason = reasonQuestion
			if s.agentHasPendingPermission(a.AgentID) {
				item.Reason = reasonPermission
			}
			out.NeedsYou = append(out.NeedsYou, item)
		case "error":
			item.Reason = reasonError
			out.NeedsYou = append(out.NeedsYou, item)
		}
	}

	if s.pipelineMgr != nil {
		runs, err := s.pipelineMgr.ListAttention(time.Time{}, attentionListLimit)
		if err != nil {
			return out, err
		}
		for _, r := range runs {
			updated, _ := time.Parse(time.RFC3339Nano, r.UpdatedAt)
			item := attentionItem{Kind: "run", ID: r.RunID, Title: r.DisplayName, Project: r.Project, State: r.State, AgentID: r.CurrentAgentID, Since: updated.UTC()}
			item.StageNumber, item.StageCount = r.StageNumber, r.StageCount
			switch {
			case r.State == "completed" || r.State == "stopped":
			case r.State == "paused" || r.AttentionReason != "":
				item.Reason = reasonRunPaused
				if r.AttentionReason != "" {
					item.Reason = strutil.ClipRunes(r.AttentionReason, 120)
				}
				out.NeedsYou = append(out.NeedsYou, item)
			default:
				item.Reason = "running"
				out.ActiveRuns = append(out.ActiveRuns, item)
			}
		}
	}

	// Needs you is oldest first; active runs are grouped by project and newest
	// first within each project.
	sort.SliceStable(out.NeedsYou, func(i, j int) bool { return out.NeedsYou[i].Since.Before(out.NeedsYou[j].Since) })
	sort.SliceStable(out.ActiveRuns, func(i, j int) bool {
		if out.ActiveRuns[i].Project != out.ActiveRuns[j].Project {
			return out.ActiveRuns[i].Project < out.ActiveRuns[j].Project
		}
		return out.ActiveRuns[i].Since.After(out.ActiveRuns[j].Since)
	})
	out.NeedsYou = boundItems(out.NeedsYou)
	out.ActiveRuns = boundItems(out.ActiveRuns)
	return out, nil
}

func boundItems(items []attentionItem) []attentionItem {
	if len(items) > attentionListLimit {
		return items[:attentionListLimit]
	}
	return items
}

// agentTitle is the role@project address the desktop shows, or the display
// name when one was set.
func agentTitle(a state.AgentState) string {
	if a.Name != "" {
		return a.Name
	}
	return a.Role + "@" + a.Project
}

// handleRemoteHome implements tailnet-only GET /api/remote/home.
func (s *Server) handleRemoteHome(w http.ResponseWriter, r *http.Request) {
	lists, err := s.attention()
	if err != nil {
		s.log.Error("remote: home", "err", err)
		writeRemoteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, lists)
}

// remoteRuntimeModel and remoteRuntimeBackend are the phone's secret-free view
// of the backend catalog (TS-13.R15): ids, names, and each model's allowed
// effort and fast values. Backend type, env, credentials, and executable paths
// never leave the Mac.
type remoteRuntimeModel struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Efforts       []string `json:"efforts"`
	DefaultEffort string   `json:"default_effort,omitempty"`
	Fast          bool     `json:"fast"`
}

type remoteRuntimeBackend struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Default      bool                 `json:"default"`
	DefaultModel string               `json:"default_model"`
	Models       []remoteRuntimeModel `json:"models"`
}

// handleRemoteRuntimeOptions implements tailnet-only GET
// /api/remote/runtime-options for Replace orchestrator (FS-20.R31). Fast is
// offered only where the shared launch validator would accept it.
func (s *Server) handleRemoteRuntimeOptions(w http.ResponseWriter, _ *http.Request) {
	backends, err := s.readBackendsOrDefault()
	if err != nil {
		s.log.Error("remote: runtime options", "err", err)
		writeRemoteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]remoteRuntimeBackend, 0, len(backends.Backends))
	for _, id := range slices.Sorted(maps.Keys(backends.Backends)) {
		backend := backends.Backends[id]
		entry := remoteRuntimeBackend{ID: id, Name: backend.Name, Default: backend.Default, DefaultModel: backend.DefaultModel, Models: []remoteRuntimeModel{}}
		for _, modelID := range slices.Sorted(maps.Keys(backend.Models)) {
			model := backend.Models[modelID]
			entry.Models = append(entry.Models, remoteRuntimeModel{
				ID: modelID, Name: model.Name, Efforts: append([]string{}, model.Efforts...), DefaultEffort: model.DefaultEffort,
				Fast: config.ValidateModelFast(backend, model, true) == nil,
			})
		}
		out = append(out, entry)
	}
	writeJSON(w, http.StatusOK, map[string]any{"backends": out})
}
