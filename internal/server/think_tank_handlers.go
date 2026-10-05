package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/AsaphNoam/Chuck/internal/runtime"
	"github.com/AsaphNoam/Chuck/internal/state"
)

// /api/think-tanks (TS-14 §3). Local only: none of these routes enters the
// phone allowlist (TS-14.R11).

const (
	thinkTankRequestLimit = 256 << 10
	thinkTankListLimit    = 200
	thinkTankEntryPage    = 500
)

type thinkTankParticipantRequest struct {
	// AgentID names an existing chat agent; New launches a new normal agent
	// with the ordinary New Agent settings. Exactly one is set.
	AgentID  string         `json:"agent_id,omitempty"`
	New      *launchRequest `json:"new,omitempty"`
	Limit    int            `json:"limit"`
	MayLeave *bool          `json:"may_leave,omitempty"`
}

type thinkTankCreateRequest struct {
	CommandID     string                        `json:"command_id"`
	Goal          string                        `json:"goal"`
	OriginProject string                        `json:"origin_project"`
	Openings      bool                          `json:"openings"`
	Judge         *launchRequest                `json:"judge,omitempty"`
	Participants  []thinkTankParticipantRequest `json:"participants"`
}

func decodeThinkTankBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, thinkTankRequestLimit)).Decode(v); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return false
	}
	return true
}

func (s *Server) writeThinkTankError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, state.ErrNotFound):
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such think tank"))
	case errors.Is(err, state.ErrThinkTankInvalid):
		writeAPIError(w, apiError(runtime.CodeValidation, strings.TrimPrefix(err.Error(), state.ErrThinkTankInvalid.Error()+": ")))
	case errors.Is(err, state.ErrThinkTankConflict):
		writeAPIError(w, apiError(runtime.CodeConflict, strings.TrimPrefix(err.Error(), state.ErrThinkTankConflict.Error()+": ")))
	default:
		s.log.Warn("think tank request failed", "err", err)
		writeAPIError(w, apiError(runtime.CodeInternal, "the think tank could not be updated"))
	}
}

// requireLiveProject refuses a missing or archived project (FS-21.R22, R36).
func (s *Server) requireLiveProject(project, what string) *runtime.APIError {
	p, err := s.configStore.ReadProject(project)
	if err != nil {
		return apiError(runtime.CodeValidation, what+" project does not exist")
	}
	if p.Archived {
		return apiError(runtime.CodeProjectArchived, what+" project is archived")
	}
	return nil
}

func (s *Server) thinkTankLaunchConfig(req *launchRequest, what string) (string, *runtime.APIError) {
	if req.Project == "" {
		return "", apiError(runtime.CodeValidation, what+" needs a project")
	}
	if ae := s.requireLiveProject(req.Project, what+"'s"); ae != nil {
		return "", ae
	}
	req.Interface = "chat"
	raw, err := json.Marshal(req)
	if err != nil {
		return "", apiError(runtime.CodeInternal, err.Error())
	}
	return string(raw), nil
}

// handleCreateThinkTank validates setup and persists intent with reserved
// identities before any launch effect; the engine launches new participants
// (FS-21.R34, TS-14.R2).
func (s *Server) handleCreateThinkTank(w http.ResponseWriter, r *http.Request) {
	var req thinkTankCreateRequest
	if !decodeThinkTankBody(w, r, &req) {
		return
	}
	if ae := s.requireLiveProject(req.OriginProject, "origin"); ae != nil {
		writeAPIError(w, ae)
		return
	}
	create := state.ThinkTankCreate{CommandID: req.CommandID, Goal: req.Goal, OriginProject: req.OriginProject, Openings: req.Openings}
	if req.Judge != nil {
		config, ae := s.thinkTankLaunchConfig(req.Judge, "the judge")
		if ae != nil {
			writeAPIError(w, ae)
			return
		}
		create.JudgeConfig = config
	}
	for _, p := range req.Participants {
		m := state.ThinkTankMember{Cap: p.Limit, MayLeave: p.MayLeave == nil || *p.MayLeave}
		switch {
		case p.AgentID != "" && p.New == nil:
			agent, err := s.stateStore.ReadAgent(p.AgentID)
			if err != nil {
				writeAPIError(w, apiError(runtime.CodeValidation, "participant "+p.AgentID+" does not exist"))
				return
			}
			if agent.Archived || agent.Interface != "chat" {
				writeAPIError(w, apiError(runtime.CodeValidation, agent.Name+" must be a non-archived chat agent"))
				return
			}
			if ae := s.requireLiveProject(agent.Project, agent.Name+"'s"); ae != nil {
				writeAPIError(w, ae)
				return
			}
			m.AgentID, m.AgentName, m.Project = agent.AgentID, agent.Name, agent.Project
		case p.AgentID == "" && p.New != nil:
			config, ae := s.thinkTankLaunchConfig(p.New, "a new participant")
			if ae != nil {
				writeAPIError(w, ae)
				return
			}
			id, err := s.stateStore.NewAgentID()
			if err != nil {
				writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
				return
			}
			name := strings.TrimSpace(p.New.Name)
			if name == "" {
				name = "New agent"
			}
			m.AgentID, m.AgentName, m.Project, m.SetupConfig = id, name, p.New.Project, config
		default:
			writeAPIError(w, apiError(runtime.CodeValidation, "each participant names an existing agent or new agent settings"))
			return
		}
		create.Members = append(create.Members, m)
	}
	d, err := s.stateStore.CreateThinkTank(create)
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	s.publishThinkTankUpdate(d)
	s.kickThinkTanks()
	writeJSON(w, http.StatusCreated, s.thinkTankDetailWire(d))
}

func (s *Server) handleThinkTanks(w http.ResponseWriter, r *http.Request) {
	rooms, err := s.stateStore.ListThinkTanks(r.URL.Query().Get("project"), thinkTankListLimit)
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	out := make([]thinkTankSummaryWire, 0, len(rooms))
	for _, room := range rooms {
		d, err := s.stateStore.ReadThinkTank(room.RoomID)
		if err != nil {
			continue
		}
		out = append(out, thinkTankSummaryFor(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": thinkTankWireVersion, "rooms": out})
}

func (s *Server) handleThinkTankDetail(w http.ResponseWriter, r *http.Request) {
	d, err := s.stateStore.ReadThinkTank(r.PathValue("id"))
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.thinkTankDetailWire(d))
}

// handleThinkTankEntries pages published entries after a room sequence.
func (s *Server) handleThinkTankEntries(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if _, err := s.stateStore.ReadThinkTank(roomID); err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	entries, err := s.stateStore.ListThinkTankEntries(roomID, max(after, 0), thinkTankEntryPage+1)
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	complete := len(entries) <= thinkTankEntryPage
	if !complete {
		entries = entries[:thinkTankEntryPage]
	}
	out := make([]thinkTankEntryWire, 0, len(entries))
	for _, e := range entries {
		out = append(out, thinkTankEntryFor(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": thinkTankWireVersion, "entries": out, "complete": complete})
}

type thinkTankMessageRequest struct {
	CommandID string `json:"command_id"`
	Body      string `json:"body"`
}

// handleThinkTankMessage adds durable shared user input: published between
// turns, or held for the active turn's boundary (FS-21.R15, R35).
func (s *Server) handleThinkTankMessage(w http.ResponseWriter, r *http.Request) {
	var req thinkTankMessageRequest
	if !decodeThinkTankBody(w, r, &req) {
		return
	}
	input, d, err := s.stateStore.AddThinkTankInput(r.PathValue("id"), req.CommandID, state.ThinkTankEntryUser, req.Body, "")
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	s.publishThinkTankUpdate(d)
	s.kickThinkTanks()
	writeJSON(w, http.StatusOK, map[string]any{
		"version": thinkTankWireVersion, "input_id": input.InputID, "published_seq": input.EntrySeq,
	})
}

func (s *Server) handleThinkTankControl(action func(string) (state.ThinkTankDetail, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d, err := action(r.PathValue("id"))
		if err != nil {
			s.writeThinkTankError(w, err)
			return
		}
		s.publishThinkTankUpdate(d)
		s.kickThinkTanks()
		writeJSON(w, http.StatusOK, s.thinkTankDetailWire(d))
	}
}

type thinkTankRetryRequest struct {
	// Target is setup (failed new-participant slots), turn (a failed or
	// held participant turn) or judge (the failed final step).
	Target string         `json:"target"`
	Judge  *launchRequest `json:"judge,omitempty"`
}

// handleRetryThinkTank retries an identified failure. Only the judge's
// launch settings may be repaired; no retry edits goal or membership or
// replays a completed effect (TS-14 §3).
func (s *Server) handleRetryThinkTank(w http.ResponseWriter, r *http.Request) {
	var req thinkTankRetryRequest
	if !decodeThinkTankBody(w, r, &req) {
		return
	}
	roomID := r.PathValue("id")
	var d state.ThinkTankDetail
	var err error
	switch req.Target {
	case "setup":
		d, err = s.stateStore.RetryThinkTankSetup(roomID)
	case "turn":
		d, err = s.stateStore.ResumeThinkTank(roomID)
	case "judge":
		config := ""
		if req.Judge != nil {
			var ae *runtime.APIError
			if config, ae = s.thinkTankLaunchConfig(req.Judge, "the judge"); ae != nil {
				writeAPIError(w, ae)
				return
			}
		}
		d, err = s.stateStore.RetryThinkTankJudge(roomID, config)
	default:
		writeAPIError(w, apiError(runtime.CodeValidation, "target must be setup, turn or judge"))
		return
	}
	if err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	s.publishThinkTankUpdate(d)
	s.kickThinkTanks()
	writeJSON(w, http.StatusOK, s.thinkTankDetailWire(d))
}

// handleDeleteThinkTank removes room-owned data only; participant agents and
// their histories are untouched (FS-21.R40).
func (s *Server) handleDeleteThinkTank(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if err := s.stateStore.DeleteThinkTank(roomID); err != nil {
		s.writeThinkTankError(w, err)
		return
	}
	s.eventBus.Publish("think_tank_update", nil, thinkTankUpdate{Version: thinkTankWireVersion, RoomID: roomID, Deleted: true})
	w.WriteHeader(http.StatusNoContent)
}
