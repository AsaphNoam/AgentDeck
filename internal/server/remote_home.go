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

// attentionItem is one row of the phone's Home (FS-20.R11). Reason is a short,
// in-vocabulary line; message, command, and diff text never appear here.
type attentionItem struct {
	Kind        string    `json:"kind"` // agent | task | run
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
	reasonTaskStuck  = "was interrupted"
	reasonDepFailed  = "has a failed prerequisite"
	reasonRunPaused  = "is paused"
)

type attentionLists struct {
	NeedsYou  []attentionItem `json:"needs_you"`
	Moving    []attentionItem `json:"moving"`
	SinceLast []attentionItem `json:"since_last"`
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

// attention is the one classification of what needs the person, what is
// moving, and what finished after since (TS-13.R10).
func (s *Server) attention(since time.Time) (attentionLists, error) {
	out := attentionLists{NeedsYou: []attentionItem{}, Moving: []attentionItem{}, SinceLast: []attentionItem{}}

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
		case "busy":
			item.Reason = "working"
			out.Moving = append(out.Moving, item)
		case "done":
			if at.After(since) {
				item.Reason = "finished"
				out.SinceLast = append(out.SinceLast, item)
			}
		}
	}

	tasks, err := s.stateStore.ListAttentionTasks(since, attentionListLimit)
	if err != nil {
		return out, err
	}
	for _, t := range tasks {
		item := attentionItem{Kind: "task", ID: t.TaskID, Title: t.DisplayName, Project: t.Project, State: t.State, AgentID: t.AssignedAgentID, Since: t.UpdatedAt}
		switch t.State {
		case state.TaskInterrupted:
			item.Reason = reasonTaskStuck
			out.NeedsYou = append(out.NeedsYou, item)
		case state.TaskDependencyFailed:
			item.Reason = reasonDepFailed
			out.NeedsYou = append(out.NeedsYou, item)
		case state.TaskFinished:
			item.Reason, item.Outcome = "finished", t.Outcome
			out.SinceLast = append(out.SinceLast, item)
		default:
			item.Reason = "running"
			out.Moving = append(out.Moving, item)
		}
	}

	if s.pipelineMgr != nil {
		runs, err := s.pipelineMgr.ListAttention(since, attentionListLimit)
		if err != nil {
			return out, err
		}
		for _, r := range runs {
			updated, _ := time.Parse(time.RFC3339Nano, r.UpdatedAt)
			item := attentionItem{Kind: "run", ID: r.RunID, Title: r.DisplayName, Project: r.Project, State: r.State, AgentID: r.CurrentAgentID, Since: updated.UTC()}
			item.StageNumber, item.StageCount = r.StageNumber, r.StageCount
			switch {
			case r.State == "completed" || r.State == "stopped":
				if updated.After(since) {
					item.Reason, item.Outcome = r.State, r.FinalOutcome
					out.SinceLast = append(out.SinceLast, item)
				}
			case r.State == "paused" || r.AttentionReason != "":
				item.Reason = reasonRunPaused
				if r.AttentionReason != "" {
					item.Reason = strutil.ClipRunes(r.AttentionReason, 120)
				}
				out.NeedsYou = append(out.NeedsYou, item)
			default:
				item.Reason = "running"
				out.Moving = append(out.Moving, item)
			}
		}
	}

	// A finished agent whose task or run needs the person is that item's
	// conversation, not separate news: listing it as "finished" beside
	// "was interrupted" contradicts the entry that needs action (FS-20.R11).
	owned := map[string]bool{}
	for _, item := range out.NeedsYou {
		if item.Kind != "agent" && item.AgentID != "" {
			owned[item.AgentID] = true
		}
	}
	out.SinceLast = slices.DeleteFunc(out.SinceLast, func(item attentionItem) bool {
		return item.Kind == "agent" && owned[item.AgentID]
	})

	// Needs you is oldest first; the other lists show the newest first.
	sort.SliceStable(out.NeedsYou, func(i, j int) bool { return out.NeedsYou[i].Since.Before(out.NeedsYou[j].Since) })
	sort.SliceStable(out.Moving, func(i, j int) bool {
		if out.Moving[i].Project != out.Moving[j].Project {
			return out.Moving[i].Project < out.Moving[j].Project
		}
		return out.Moving[i].Since.After(out.Moving[j].Since)
	})
	sort.SliceStable(out.SinceLast, func(i, j int) bool { return out.SinceLast[i].Since.After(out.SinceLast[j].Since) })
	out.NeedsYou = boundItems(out.NeedsYou)
	out.Moving = boundItems(out.Moving)
	out.SinceLast = boundItems(out.SinceLast)
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

// handleRemoteHome implements tailnet-only GET /api/remote/home?since=<time>.
func (s *Server) handleRemoteHome(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-24 * time.Hour)
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = t
		}
	}
	lists, err := s.attention(since)
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
	ID     string               `json:"id"`
	Name   string               `json:"name"`
	Models []remoteRuntimeModel `json:"models"`
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
		entry := remoteRuntimeBackend{ID: id, Name: backend.Name, Models: []remoteRuntimeModel{}}
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
