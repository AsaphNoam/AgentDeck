package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

// cloneResponse is the ordinary new-session envelope plus the fork lineage
// (TS-03.R43).
type cloneResponse struct {
	sessionResponse
	HistoryHandoff    string `json:"history_handoff"`
	ForkedFromAgentID string `json:"forked_from_agent_id"`
	ForkedFromSeq     int64  `json:"forked_from_seq"`
}

// handleClone implements POST /api/sessions/{id}/clone: a new agent whose
// provider conversation is a native fork of the source's at its latest
// completed turn (FS-01.R36). The request carries no provider or session id;
// every setting and the native source id come from the stored source.
func (s *Server) handleClone(w http.ResponseWriter, r *http.Request) {
	resp, ae := s.cloneAgent(r.Context(), r.PathValue("id"))
	if ae != nil {
		writeAPIError(w, ae)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) cloneAgent(ctx context.Context, sourceID string) (cloneResponse, *runtime.APIError) {
	source, err := s.stateStore.ReadAgent(sourceID)
	if err != nil {
		return cloneResponse{}, apiError(runtime.CodeNotFound, "no such agent: "+sourceID)
	}
	// One exclusive lifecycle claim over the source: resume, switch, stop and
	// a second clone cannot move it while its conversation point is copied.
	if !s.claimLifecycle(sourceID) {
		return cloneResponse{}, apiError(runtime.CodeTransitionInProgress, "another lifecycle change is in progress for this agent")
	}
	defer s.releaseLifecycle(sourceID)

	snap, err := s.stateStore.ReadSession(sourceID)
	if err != nil && !errors.Is(err, state.ErrNotFound) {
		return cloneResponse{}, apiError(runtime.CodeInternal, "read source session")
	}
	busy := false
	if status, err := s.stateStore.ReadStatus(sourceID); err == nil {
		busy = state.CloneBusyState(status.State)
	}
	if busy && source.Interface == "chat" && !source.Archived {
		return cloneResponse{}, apiError(runtime.CodeAgentBusy, state.CloneReasonBusy)
	}
	if aff := state.CloneAvailability(source.Interface, source.Archived, snap.RuntimeCapabilities, snap.LastSessionID, busy); !aff.Available {
		return cloneResponse{}, cloneUnavailable(aff.Reason)
	}

	// The fork boundary is the source's latest completed turn; the clone gets a
	// durable copy of the visible transcript through it.
	prefix, boundary, err := transcript.CloneCompletedPrefix(s.configStore.Home(), sourceID)
	if err != nil {
		return cloneResponse{}, apiError(runtime.CodeInternal, "read source transcript")
	}
	if boundary == 0 {
		return cloneResponse{}, cloneUnavailable(state.CloneReasonNoSession)
	}

	resp, ae := s.launchAgent(ctx, launchRequest{
		Role: source.Role, Project: source.Project, Backend: source.Backend, Model: source.Model,
		Effort: source.Effort, Fast: source.Fast, Interface: source.Interface, Group: source.Group,
	}, launchOptions{Fork: &runtime.ForkPlan{
		SourceAgentID: sourceID, SourceSessionID: snap.LastSessionID, SourceSeq: boundary, Prefix: prefix,
	}})
	if ae != nil {
		return cloneResponse{}, ae
	}
	return cloneResponse{sessionResponse: resp, HistoryHandoff: "native_fork", ForkedFromAgentID: sourceID, ForkedFromSeq: boundary}, nil
}

func cloneUnavailable(reason string) *runtime.APIError {
	ae := apiError(runtime.CodeCloneUnavailable, reason)
	ae.Details = map[string]any{"reason": reason}
	return ae
}
