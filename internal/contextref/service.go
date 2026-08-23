package contextref

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
	"github.com/agentdeck/agentdeck/internal/transcript"
)

// Friendly share selectors. Each is server-resolved to an exact FS-15.R2
// locator: no tool argument may name another agent's transcript or another
// generation's attempt (TS-05.R16).
const (
	SelectorCurrentTurn           = "current_turn"
	SelectorLatestCompletedTurn   = "latest_completed_turn"
	SelectorCurrentPipelineReport = "current_pipeline_report"
)

// Caller is the identity the MCP session token resolved to. It is never taken
// from a tool argument.
type Caller struct {
	AgentID    string
	Generation string
}

// Service is the one in-process context-plane service.
type Service struct {
	store *state.Store
	home  string
}

func New(store *state.Store, home string) *Service {
	return &Service{store: store, home: home}
}

// SourceDescriptor is the intrinsic, content-free description of a reference's
// source. It is safe to return from list and read results (TS-05.R16).
type SourceDescriptor struct {
	Kind              string `json:"kind"`
	AgentID           string `json:"agent_id,omitempty"`
	FirstSeq          int64  `json:"first_seq,omitempty"`
	LastSeq           int64  `json:"last_seq,omitempty"`
	PipelineRunID     string `json:"pipeline_run_id,omitempty"`
	PipelineStageID   string `json:"pipeline_stage_id,omitempty"`
	PipelineAttemptID string `json:"pipeline_attempt_id,omitempty"`
	AttemptNo         int    `json:"attempt_no,omitempty"`
}

func describe(source state.ContextSource) SourceDescriptor {
	return SourceDescriptor{
		Kind:              source.Kind,
		AgentID:           source.AgentID,
		FirstSeq:          source.FirstSeq,
		LastSeq:           source.LastSeq,
		PipelineAttemptID: source.PipelineAttemptID,
	}
}

// ShareResult is what a successful share returns: the canonical reference, the
// direct grant it created or refreshed, and the resolved source descriptor.
type ShareResult struct {
	ContextRefID string           `json:"context_ref_id"`
	GrantID      string           `json:"grant_id"`
	To           string           `json:"to"`
	ToAddress    string           `json:"to_address"`
	Source       SourceDescriptor `json:"source"`
}

// Share resolves a friendly selector to an exact locator, canonicalizes it, and
// grants one resolvable chat agent access. It creates no mail, activation,
// prompt, transcript event, or SSE payload (FS-15.R4–R5, R10).
func (s *Service) Share(caller Caller, selector, to, label, description string) (ShareResult, error) {
	if err := validatePresentation(label, description); err != nil {
		return ShareResult{}, err
	}
	recipients, err := s.store.ContextRecipients()
	if err != nil {
		return ShareResult{}, unavailable(err)
	}
	toID, candidates, err := state.ResolveRecipient(recipients, to)
	if err != nil {
		var amb *state.AmbiguousError
		switch {
		case errors.As(err, &amb):
			return ShareResult{}, &Error{Code: CodeAmbiguous,
				Message:    fmt.Sprintf("Multiple agents match %q; address by agent_id.", to),
				Candidates: amb.Candidates}
		case errors.Is(err, state.ErrRecipientNotFound):
			return ShareResult{}, &Error{Code: CodeRecipientNotFound,
				Message:    fmt.Sprintf("No durable chat agent matches %q.", to),
				Candidates: candidates}
		default:
			return ShareResult{}, unavailable(err)
		}
	}

	source, cerr := s.resolveSelector(caller, selector)
	if cerr != nil {
		return ShareResult{}, cerr
	}
	ref, grant, err := s.store.ShareContext(source, caller.AgentID, toID, label, description)
	if err != nil {
		if errors.Is(err, state.ErrInvalidContextSource) {
			return ShareResult{}, failf(CodeValidation, "Resolved source locator is invalid.")
		}
		return ShareResult{}, unavailable(err)
	}
	address := ""
	for _, r := range recipients {
		if r.AgentID == toID {
			address = r.Address
			break
		}
	}
	return ShareResult{
		ContextRefID: ref.ContextRefID,
		GrantID:      grant.GrantID,
		To:           toID,
		ToAddress:    address,
		Source:       describe(ref.Source),
	}, nil
}

func validatePresentation(label, description string) *Error {
	if utf8.RuneCountInString(label) > MaxLabelRunes {
		return failf(CodeValidation, "label must be at most %d characters.", MaxLabelRunes)
	}
	if utf8.RuneCountInString(description) > MaxDescriptionRunes {
		return failf(CodeValidation, "description must be at most %d characters.", MaxDescriptionRunes)
	}
	return nil
}

func (s *Service) resolveSelector(caller Caller, selector string) (state.ContextSource, *Error) {
	switch selector {
	case SelectorCurrentTurn, SelectorLatestCompletedTurn:
		return s.resolveTranscriptSpan(caller.AgentID, selector)
	case SelectorCurrentPipelineReport:
		return s.resolvePipelineReport(caller)
	default:
		return state.ContextSource{}, failf(CodeValidation,
			"source must be one of %q, %q, or %q.",
			SelectorCurrentTurn, SelectorLatestCompletedTurn, SelectorCurrentPipelineReport)
	}
}

// turnScan is one streaming pass over the caller's own durable transcript. It
// tracks the turn in progress and the most recently completed turn without
// retaining the session in memory.
type turnScan struct {
	openFirst, openLast           int64
	openHas                       bool
	completedFirst, completedLast int64
	completedHas                  bool
}

func (s *Service) resolveTranscriptSpan(agentID, selector string) (state.ContextSource, *Error) {
	var scan turnScan
	// IncludeMeta is true only so the scan can see session and backend-switch
	// boundaries; their content never enters a span. A boundary closes whatever
	// turn was left open on the other side of it, so a stop or crash without a
	// turn_end cannot make the turn after the resume start inside the abandoned
	// one (TS-04.R28, FS-15.R4/R15, INV §1).
	err := transcript.ForEachFile(s.home, agentID, transcript.ReadOptions{IncludeMeta: true}, func(ev runtime.Event) error {
		switch ev.Type {
		case runtime.EvSessionMeta, runtime.EvBackendSwitch:
			scan.openHas = false
			return nil
		case runtime.EvTurnEnd:
			if scan.openHas {
				scan.completedFirst = scan.openFirst
			} else {
				scan.completedFirst = ev.Seq
			}
			scan.completedLast = ev.Seq
			scan.completedHas = true
			scan.openHas = false
			return nil
		}
		if ev.Seq == 0 {
			return nil
		}
		if !scan.openHas {
			scan.openFirst = ev.Seq
			scan.openHas = true
		}
		scan.openLast = ev.Seq
		return nil
	})
	if err != nil {
		return state.ContextSource{}, unavailable(err)
	}

	span := state.ContextSource{Kind: state.ContextSourceTranscriptSpan, AgentID: agentID}
	switch selector {
	case SelectorCurrentTurn:
		if !scan.openHas {
			return state.ContextSource{}, failf(CodeSourceUnavailable,
				"The current turn has no persisted transcript content yet.")
		}
		span.FirstSeq, span.LastSeq = scan.openFirst, scan.openLast
	default:
		if !scan.completedHas {
			return state.ContextSource{}, failf(CodeSourceUnavailable,
				"This agent has no completed transcript turn yet.")
		}
		// The completed turn is available only from inside a later turn started
		// for some independent reason: context sharing does not create that turn,
		// and an idle session token must not be able to hand out finished work
		// (TS-04.R28).
		if !scan.openHas {
			return state.ContextSource{}, failf(CodeSourceUnavailable,
				"The latest completed turn is shareable only from inside a later turn.")
		}
		span.FirstSeq, span.LastSeq = scan.completedFirst, scan.completedLast
	}
	if err := span.Validate(); err != nil {
		return state.ContextSource{}, failf(CodeSourceUnavailable, "The selected turn is not a valid transcript span.")
	}
	return span, nil
}

// resolvePipelineReport joins the caller plus its token generation to the one
// current attempt and requires an accepted report that has not yet reached
// quiescence — the exact window between report_pipeline_stage_result succeeding
// and the reporting turn ending (TS-04.R28, FS-15.R4).
func (s *Service) resolvePipelineReport(caller Caller) (state.ContextSource, *Error) {
	_, attempt, err := s.store.CurrentPipelineAttemptForAgent(caller.AgentID)
	if errors.Is(err, state.ErrNotFound) {
		return state.ContextSource{}, failf(CodeSourceUnavailable,
			"You have no current pipeline attempt with an accepted report.")
	}
	if err != nil {
		return state.ContextSource{}, unavailable(err)
	}
	if caller.Generation != "" && attempt.AgentGeneration != "" && attempt.AgentGeneration != caller.Generation {
		return state.ContextSource{}, failf(CodeSourceUnavailable,
			"Your session does not own the current pipeline attempt.")
	}
	if attempt.ReportedAt == nil || attempt.QuiescentAt != nil {
		return state.ContextSource{}, failf(CodeSourceUnavailable,
			"A pipeline report is shareable only after it is accepted and before the reporting turn ends.")
	}
	return state.ContextSource{Kind: state.ContextSourcePipelineReport, PipelineAttemptID: attempt.AttemptID}, nil
}

// GrantSummary is one bounded ad-hoc list entry: grant presentation and
// provenance plus intrinsic source metadata and personal hidden state. It
// carries no source body and no future work attachment (FS-15.R6).
type GrantSummary struct {
	GrantID      string           `json:"grant_id"`
	ContextRefID string           `json:"context_ref_id"`
	GrantedBy    string           `json:"granted_by_agent_id"`
	Label        string           `json:"label"`
	Description  string           `json:"description"`
	SharedAt     string           `json:"shared_at"`
	Hidden       bool             `json:"hidden"`
	Source       SourceDescriptor `json:"source"`
}

// ListResult is one bounded page of the caller's direct grants.
type ListResult struct {
	Links      []GrantSummary `json:"links"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// List returns the caller's active direct grants, newest first.
func (s *Service) List(caller Caller, includeHidden bool, limit int, cursor string) (ListResult, error) {
	if limit == 0 {
		limit = DefaultListLimit
	}
	if limit < MinListLimit || limit > MaxListLimit {
		return ListResult{}, failf(CodeValidation, "limit must be between %d and %d.", MinListLimit, MaxListLimit)
	}
	afterAt, afterID, cerr := decodeListCursor(caller.AgentID, cursor)
	if cerr != nil {
		return ListResult{}, cerr
	}
	grants, err := s.store.ListContextGrantsForRecipient(caller.AgentID, includeHidden, limit, afterAt, afterID)
	if err != nil {
		return ListResult{}, unavailable(err)
	}
	out := ListResult{Links: make([]GrantSummary, 0, len(grants))}
	for _, g := range grants {
		out.Links = append(out.Links, GrantSummary{
			GrantID:      g.GrantID,
			ContextRefID: g.RefID,
			GrantedBy:    g.GrantedBy,
			Label:        g.Label,
			Description:  g.Description,
			SharedAt:     g.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Hidden:       g.Hidden,
			Source:       describe(g.Source),
		})
	}
	if len(grants) == limit {
		last := grants[len(grants)-1]
		out.NextCursor = encodeListCursor(caller.AgentID, last.UpdatedAt, last.GrantID)
	}
	return out, nil
}

// ReadResult is one bounded content page of a reference's source.
type ReadResult struct {
	ContextRefID string           `json:"context_ref_id"`
	Source       SourceDescriptor `json:"source"`
	Text         string           `json:"text"`
	Complete     bool             `json:"complete"`
	NextCursor   string           `json:"next_cursor,omitempty"`
}

// Read returns one deterministic bounded page. Authorization is re-evaluated on
// every page, and a missing or unauthorized id shares one safe outcome
// (FS-15.R9, TS-05.R16). Reading has no personal-state side effect.
func (s *Service) Read(caller Caller, refID, cursor string) (ReadResult, error) {
	ref, err := s.store.ReadContextReference(refID)
	if errors.Is(err, state.ErrNotFound) {
		return ReadResult{}, notFound()
	}
	if err != nil {
		return ReadResult{}, unavailable(err)
	}
	authorized, err := s.store.ContextReadAuthorized(refID, caller.AgentID)
	if err != nil {
		return ReadResult{}, unavailable(err)
	}
	if !authorized {
		return ReadResult{}, notFound()
	}
	offset, cerr := decodeCursor(refID, cursor)
	if cerr != nil {
		return ReadResult{}, cerr
	}

	out := newPageWriter(offset, MaxPageBytes)
	switch ref.Source.Kind {
	case state.ContextSourceTranscriptSpan:
		if cerr := s.renderSpan(ref.Source, out); cerr != nil {
			return ReadResult{}, cerr
		}
	case state.ContextSourcePipelineReport:
		attempt, err := s.store.ReadPipelineAttempt(ref.Source.PipelineAttemptID)
		if errors.Is(err, state.ErrNotFound) {
			return ReadResult{}, sourceGone()
		}
		if err != nil {
			return ReadResult{}, unavailable(err)
		}
		if attempt.ReportedAt == nil {
			return ReadResult{}, sourceGone()
		}
		renderPipelineReport(attempt, out)
	default:
		return ReadResult{}, sourceGone()
	}

	if !out.validOffset() {
		return ReadResult{}, failf(CodeInvalidCursor, "Cursor does not name a position in this source.")
	}
	text, nextOffset, complete := out.page()
	result := ReadResult{
		ContextRefID: refID,
		Source:       describe(ref.Source),
		Text:         text,
		Complete:     complete,
	}
	if !complete {
		result.NextCursor = encodeCursor(refID, nextOffset)
	}
	return result, nil
}

func (s *Service) renderSpan(source state.ContextSource, out *pageWriter) *Error {
	// The durable agent identity is the deletion test: archive is not deletion,
	// so an archived agent's transcript still reads (FS-15.R12–R13).
	if _, err := s.store.ReadAgent(source.AgentID); errors.Is(err, state.ErrNotFound) {
		return sourceGone()
	} else if err != nil {
		return unavailable(err)
	}
	renderer := newTranscriptRenderer(source, out)
	// The renderer range-filters the span itself and needs the records before it
	// as stream positions, so this read neither seeks past them nor lets the
	// reader hide a session boundary from the oversized-record edge rules.
	opts := transcript.ReadOptions{IncludeMeta: true, OnOversized: renderer.oversized}
	if err := transcript.ForEachFile(s.home, source.AgentID, opts, renderer.event); err != nil {
		return unavailable(err)
	}
	renderer.done()
	if renderer.empty {
		return sourceGone()
	}
	return nil
}

func sourceGone() *Error {
	return &Error{Code: CodeSourceGone,
		Message: "The source this reference names is no longer available."}
}

// SetHidden changes only the caller's personal list projection (FS-15.R7).
func (s *Service) SetHidden(caller Caller, grantID string, hidden bool) error {
	ok, err := s.store.SetContextGrantHidden(grantID, caller.AgentID, hidden)
	if err != nil {
		return unavailable(err)
	}
	if !ok {
		return notFound()
	}
	return nil
}

// Revoke withdraws the caller's own direct grant (FS-15.R8).
func (s *Service) Revoke(caller Caller, grantID string) error {
	ok, err := s.store.RevokeContextGrant(grantID, caller.AgentID)
	if err != nil {
		return unavailable(err)
	}
	if !ok {
		return notFound()
	}
	return nil
}

// The list cursor is a keyset position, bound to its caller so it cannot be
// replayed against another agent's list. Like the read cursor it confers no
// authority: the caller identity is still taken from the session token.
func encodeListCursor(caller string, updatedAt time.Time, grantID string) string {
	return encodeOpaque(caller + "\x00" + updatedAt.UTC().Format(time.RFC3339) + "\x00" + grantID)
}

func decodeListCursor(caller, cursor string) (updatedAt time.Time, grantID string, cerr *Error) {
	if cursor == "" {
		return time.Time{}, "", nil
	}
	raw, ok := decodeOpaque(cursor)
	if !ok {
		return time.Time{}, "", failf(CodeInvalidCursor, "Cursor is malformed.")
	}
	parts := strings.SplitN(raw, "\x00", 3)
	if len(parts) != 3 || parts[0] != caller {
		return time.Time{}, "", failf(CodeInvalidCursor, "Cursor does not belong to this list.")
	}
	at, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return time.Time{}, "", failf(CodeInvalidCursor, "Cursor is malformed.")
	}
	return at, parts[2], nil
}
