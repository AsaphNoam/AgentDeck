package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

const (
	maxAnnotationCount      = 20
	maxAnnotationChars      = 2000
	maxAnnotationMailBytes  = 8000
	annotationClippedMarker = "\n… [excerpt clipped]"
)

// handlePrompt implements POST /api/sessions/{id}/prompt (techspec §7.3).
func (s *Server) handlePrompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeAPIError(w, apiError(runtime.CodeValidation, "text is required"))
		return
	}
	err := s.registry.SendPrompt(r.Context(), id, body.Text)
	// A prompt to a stopped chat agent wakes it and is then delivered in the woken
	// session (FS-01.R33, TS-03.R25). The wake runs inside this request, so the
	// caller simply waits out the resume; ACP's per-stage deadlines bound it
	// (TS-04.R22). An agent the wake gates exclude keeps today's 404.
	if errors.Is(err, runtime.ErrNoHandle) {
		woken, ae := s.wakeAgent(r.Context(), id)
		if ae != nil {
			writeAPIError(w, ae)
			return
		}
		if woken {
			err = s.registry.SendPrompt(r.Context(), id, body.Text)
		}
	}
	if err != nil {
		writeAPIError(w, sessionOpError(err))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "agent_id": id})
}

// handleAnnotations accepts one point-in-time review batch. New-task delivery
// composes the existing launch endpoint with this same agent target, keeping one
// delivery path for mail and one event shape for replay/search.
func (s *Server) handleAnnotations(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("id")
	var body runtime.AnnotationData
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	if err := validateAnnotations(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, err.Error()))
		return
	}
	source, err := s.stateStore.ReadAgent(sourceID)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+sourceID))
		return
	}
	if source.Interface != "chat" {
		writeAPIError(w, apiError(runtime.CodeValidation, "terminal agents cannot be annotation sources"))
		return
	}

	// Every target is validated and its delivery payload composed before anything
	// is released, so the durable source event can be written first (invariant
	// §15). Delivery is deferred into this closure rather than run inline: the
	// tray is deliberately preserved on failure (FS-13.R3), so a 500 returned
	// after mail was inserted or a turn was started makes the natural retry
	// duplicate that irreversible effect — and FS-13.R5's durable record would be
	// missing for work an agent is already acting on.
	var targetID string
	var deliver func() *runtime.APIError
	switch body.Target.Kind {
	case "self":
		if _, err := s.stateStore.ReadRunning(sourceID); err != nil {
			writeAPIError(w, apiError(runtime.CodeConflict, "the source agent is not running"))
			return
		}
		status, err := s.stateStore.ReadStatus(sourceID)
		if err != nil || status.State != "idle" {
			writeAPIError(w, apiError(runtime.CodeConflict, "the source agent must be idle"))
			return
		}
		block := runtime.FormatAnnotationBlock(body)
		targetID = sourceID
		deliver = func() *runtime.APIError {
			if err := s.registry.SendPrompt(r.Context(), sourceID, block); err != nil {
				return sessionOpError(err)
			}
			return nil
		}
	case "agent":
		targetID = body.Target.AgentID
		if targetID == sourceID {
			writeAPIError(w, apiError(runtime.CodeValidation, "use target self for the source agent"))
			return
		}
		target, err := s.stateStore.ReadAgent(targetID)
		if err != nil || target.Interface != "chat" {
			writeAPIError(w, apiError(runtime.CodeValidation, "target must be a running chat agent"))
			return
		}
		if _, err := s.stateStore.ReadRunning(targetID); err != nil {
			writeAPIError(w, apiError(runtime.CodeConflict, "target agent is not running"))
			return
		}
		mailBody, err := annotationMailBody(body)
		if err != nil {
			writeAPIError(w, apiError(runtime.CodeValidation, err.Error()))
			return
		}
		deliver = func() *runtime.APIError {
			if _, err := s.stateStore.InsertMessage(state.Message{
				FromAgent: "user", FromAddress: "user@dashboard", FromName: "Dashboard user",
				ToAgent: targetID, Subject: "Annotations", Body: mailBody,
			}); err != nil {
				return apiError(runtime.CodeInternal, err.Error())
			}
			select {
			case s.nudgeCh <- targetID:
			default:
			}
			if _, err := s.stateMgr.Touch(targetID); err != nil {
				s.log.Debug("touch annotation recipient failed", "agent", targetID, "err", err)
			}
			return nil
		}
	default:
		writeAPIError(w, apiError(runtime.CodeValidation, "target kind must be self or agent"))
		return
	}
	body.Target.AgentID = targetID
	ev, err := s.appendAnnotation(sourceID, body)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if apiErr := deliver(); apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "seq": ev.Seq})
}

func validateAnnotations(data *runtime.AnnotationData) error {
	if len(data.Annotations) == 0 || len(data.Annotations) > maxAnnotationCount {
		return fmt.Errorf("annotations must contain 1..%d entries", maxAnnotationCount)
	}
	if runeCount(data.OverallInstruction) > maxAnnotationChars {
		return fmt.Errorf("overall instruction must be at most %d characters", maxAnnotationChars)
	}
	for i := range data.Annotations {
		a := &data.Annotations[i]
		if a.Seq < 1 || strings.TrimSpace(a.Excerpt) == "" || strings.TrimSpace(a.Instruction) == "" {
			return fmt.Errorf("annotation %d needs an event, excerpt, and instruction", i+1)
		}
		if runeCount(a.Instruction) > maxAnnotationChars {
			return fmt.Errorf("annotation %d instruction must be at most %d characters", i+1, maxAnnotationChars)
		}
		if (a.StartLine > 0 || a.EndLine > 0 || a.Side != "") &&
			(a.Path == "" || (a.Side != "old" && a.Side != "new") || a.StartLine < 1 || a.EndLine < a.StartLine) {
			return fmt.Errorf("annotation %d has an invalid diff anchor", i+1)
		}
		a.Excerpt = clipAnnotationExcerpt(a.Excerpt, maxAnnotationChars)
		a.Instruction = strings.TrimSpace(a.Instruction)
	}
	data.OverallInstruction = strings.TrimSpace(data.OverallInstruction)
	return nil
}

func runeCount(s string) int { return len([]rune(s)) }

func clipAnnotationExcerpt(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	marker := []rune(annotationClippedMarker)
	if max <= len(marker) {
		return string(marker[:max])
	}
	return string(r[:max-len(marker)]) + annotationClippedMarker
}

func annotationMailBody(data runtime.AnnotationData) (string, error) {
	if len(runtime.FormatAnnotationBlock(data)) <= maxAnnotationMailBytes {
		return runtime.FormatAnnotationBlock(data), nil
	}
	trimmed := data
	trimmed.Annotations = append([]runtime.Annotation{}, data.Annotations...)
	for i := range trimmed.Annotations {
		trimmed.Annotations[i].Excerpt = ""
	}
	if len(runtime.FormatAnnotationBlock(trimmed)) > maxAnnotationMailBytes {
		return "", fmt.Errorf("annotation anchors and instructions exceed the %d-byte mail limit", maxAnnotationMailBytes)
	}
	for i := range trimmed.Annotations {
		original := data.Annotations[i].Excerpt
		lo, hi, best := 0, len([]rune(original)), ""
		for lo <= hi {
			mid := (lo + hi) / 2
			candidate := clipAnnotationExcerpt(original, mid)
			trimmed.Annotations[i].Excerpt = candidate
			if len(runtime.FormatAnnotationBlock(trimmed)) <= maxAnnotationMailBytes {
				best, lo = candidate, mid+1
			} else {
				hi = mid - 1
			}
		}
		if best == "" && original != "" {
			best = clipAnnotationExcerpt(original, 1)
		}
		trimmed.Annotations[i].Excerpt = best
	}
	body := runtime.FormatAnnotationBlock(trimmed)
	if len(body) > maxAnnotationMailBytes {
		return "", fmt.Errorf("annotation batch exceeds the %d-byte mail limit", maxAnnotationMailBytes)
	}
	return body, nil
}

func (s *Server) appendAnnotation(sourceID string, data runtime.AnnotationData) (runtime.Event, error) {
	if _, err := s.stateStore.ReadRunning(sourceID); err == nil {
		// For live agents: validate and append durably to the runtime transcript,
		// then flush the index, and only then deliver through the event bus.
		// This ordering ensures a failed append returns an error before any
		// delivery happens, and a delivery failure after the append does not drop
		// the annotation from the local source (FS-13.R5, TS-03.R14, INV §15).
		if err := validateAnnotations(&data); err != nil {
			return runtime.Event{}, err
		}
		// Verify the mail body will fit before persisting.
		_, err := annotationMailBody(data)
		if err != nil {
			return runtime.Event{}, err
		}

		// AppendAnnotationAndSync appends to the runtime transcript and returns
		// the event WITHOUT publishing. The caller is responsible for publishing
		// after durability is confirmed.
		ev, err := s.registry.AppendAnnotationAndSync(sourceID, data)
		if err != nil {
			return runtime.Event{}, err
		}

		// Only after durability is confirmed, publish through the event bus.
		s.eventBus.PublishRuntimeEvent(ev)

		return ev, nil
	} else if !errors.Is(err, state.ErrNotFound) {
		return runtime.Event{}, err
	}

	// For archived/inactive agents: append durably to the frozen transcript.
	if err := validateAnnotations(&data); err != nil {
		return runtime.Event{}, err
	}
	_, err := annotationMailBody(data)
	if err != nil {
		return runtime.Event{}, err
	}

	w, err := transcript.Open(s.configStore.Home(), sourceID, nil)
	if err != nil {
		return runtime.Event{}, err
	}
	defer w.Close()
	raw, err := json.Marshal(data)
	if err != nil {
		return runtime.Event{}, err
	}
	ev := runtime.Event{AgentID: sourceID, Seq: w.NextSeq(), Type: runtime.EvAnnotation, Data: raw, Ts: time.Now().UTC().Format(time.RFC3339)}
	if err := w.Append(ev); err != nil {
		return runtime.Event{}, err
	}
	if err := w.Sync(); err != nil {
		return runtime.Event{}, err
	}
	if err := s.indexer.OnEventAndFlushContent(sourceID, ev, ev.Seq, ev.Ts); err != nil {
		return runtime.Event{}, err
	}
	s.eventBus.PublishRuntimeEvent(ev)
	return ev, nil
}

func (s *Server) handleTranscript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.stateStore.ReadAgent(id); err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	sinceSeq, err := parseInt64Query(r, "since_seq")
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "since_seq must be an integer"))
		return
	}
	events, err := transcript.ReadFile(s.configStore.Home(), id, transcript.ReadOptions{
		SinceSeq:    sinceSeq,
		IncludeMeta: r.URL.Query().Get("include_meta") == "true",
	})
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": id, "events": events})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.stateStore.ReadAgent(id); err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	unreadOnly := r.URL.Query().Get("unread_only") == "true"
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeAPIError(w, apiError(runtime.CodeValidation, "limit must be a positive integer"))
			return
		}
		limit = n
	}
	if limit > 200 {
		limit = 200
	}
	// ListMessages returns the newest N (created_at DESC); the inbox presents
	// newest-first directly, so no reversal is needed. (The prior ASC query +
	// reversal returned the oldest N and dropped new mail past the limit.)
	msgs, err := s.stateStore.ListMessages(id, unreadOnly, limit)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	unread, err := s.stateStore.UnreadCount(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":     id,
		"unread_count": unread,
		"messages":     msgs,
	})
}

func parseInt64Query(r *http.Request, key string) (int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

// handleCancel implements POST /api/sessions/{id}/cancel (techspec §7.4).
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cancelled, err := s.registry.Cancel(r.Context(), id)
	if err != nil {
		if errors.Is(err, runtime.ErrNoHandle) {
			writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
			return
		}
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	// false = the agent was idle, so the cancel was a no-op (techspec §7.4).
	writeJSON(w, http.StatusAccepted, map[string]any{"cancelled": cancelled})
}

// handleStop implements POST /api/sessions/{id}/stop (techspec §7.5).
//
// Stop takes the same exclusive per-agent lifecycle claim every resume and wake
// takes (TS-01.R16). Registry.Stop cannot distinguish an in-progress resume's nil
// sentinel from "no handle", so an unclaimed Stop arriving mid-wake fell into the
// idempotent branch below and tore down the wake's freshly minted hook token, MCP
// session, and hook-settings file while the resume ran on to report success
// (INV §4). Losing the claim returns the same conflict a losing resume returns;
// the client may retry once the transition settles.
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.claimLifecycle(id) {
		writeAPIError(w, apiError(runtime.CodeConflict, "a resume is already in progress"))
		return
	}
	defer s.releaseLifecycle(id)
	if err := s.registry.Stop(r.Context(), id); err != nil {
		if errors.Is(err, runtime.ErrNoHandle) {
			// Not currently running. If the identity still exists, the agent is
			// already stopped — make stop idempotent so a double-click or a
			// lost-response retry reads as success, not a phantom "unknown agent".
			// Check for orphan runtimes left by a prior crash (invariant §4).
			if _, rerr := s.stateStore.ReadAgent(id); rerr == nil {
				s.reapOrphanRuntime(id)
				s.teardownAgentRegistration(id)
				writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
				return
			}
			writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
			return
		}
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	s.teardownAgentRegistration(id)
	writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
}

// handlePermission implements POST /api/sessions/{id}/permission (techspec §7.6).
func (s *Server) handlePermission(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		ToolCallID string `json:"tool_call_id"`
		Decision   string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	if body.Decision != "approve" && body.Decision != "deny" {
		writeAPIError(w, apiError(runtime.CodeValidation, "decision must be approve or deny"))
		return
	}
	if err := s.registry.Permission(r.Context(), id, body.ToolCallID, body.Decision); err != nil {
		writeAPIError(w, permissionError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"resolved": true, "tool_call_id": body.ToolCallID, "decision": body.Decision,
	})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	name, ae := normalizeAgentName(body.Name, false)
	if ae != nil {
		writeAPIError(w, ae)
		return
	}
	agent, err := s.stateStore.ReadAgent(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	agent.Name = name
	if err := s.stateStore.WriteAgent(agent); err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if _, err := s.stateMgr.Touch(id); err != nil {
		s.log.Debug("rename state touch failed", "agent", id, "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": id, "name": agent.Name})
}

func (s *Server) handleIdentity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Name  *string `json:"name"`
		Group *string `json:"group"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeValidation, "invalid JSON body"))
		return
	}
	agent, err := s.stateStore.ReadAgent(id)
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeNotFound, "no such agent: "+id))
		return
	}
	if body.Name != nil {
		name, ae := normalizeAgentName(*body.Name, false)
		if ae != nil {
			writeAPIError(w, ae)
			return
		}
		agent.Name = name
	}
	if body.Group != nil {
		group := strings.TrimSpace(*body.Group)
		if group == "_ungrouped" {
			writeAPIError(w, apiError(runtime.CodeInvalidGroupName, "_ungrouped is reserved"))
			return
		}
		agent.Group = group
	}
	if err := s.stateStore.WriteAgent(agent); err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	if _, err := s.stateMgr.Touch(id); err != nil {
		s.log.Debug("identity state touch failed", "agent", id, "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":  agent.AgentID,
		"group":     agent.Group,
		"name":      agent.Name,
		"role":      agent.Role,
		"project":   agent.Project,
		"backend":   agent.Backend,
		"model":     agent.Model,
		"interface": agent.Interface,
	})
}

// sessionOpError maps a prompt/control error to an APIError (techspec §7.3).
func sessionOpError(err error) *runtime.APIError {
	switch {
	case errors.Is(err, runtime.ErrNoHandle):
		return apiError(runtime.CodeNotFound, "agent not started")
	case errors.Is(err, runtime.ErrTurnInFlight):
		return apiError(runtime.CodeConflict, "a turn is already in flight")
	default:
		return apiError(runtime.CodeInternal, err.Error())
	}
}

// permissionError maps a Permission relay error to an APIError (techspec §7.6).
func permissionError(err error) *runtime.APIError {
	switch {
	case errors.Is(err, runtime.ErrNoHandle):
		return apiError(runtime.CodeNotFound, "agent not started")
	case errors.Is(err, runtime.ErrNoPendingPermission):
		return apiError(runtime.CodeConflict, "no pending permission for that tool_call_id")
	case errors.Is(err, runtime.ErrPermissionAlreadyResolved):
		return apiError(runtime.CodeConflict, "permission already resolved for that tool_call_id")
	case errors.Is(err, runtime.ErrInvalidDecision):
		return apiError(runtime.CodeValidation, "decision must be approve or deny")
	default:
		return apiError(runtime.CodeInternal, err.Error())
	}
}
