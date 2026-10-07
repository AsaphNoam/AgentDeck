package state

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ThinkTankBegin is the guarded admission of one room turn, committed inside
// the runtime's before(turnID) callback before the provider frame (TS-14.R3).
type ThinkTankBegin struct {
	RoomID     string
	Revision   int64
	AgentID    string
	Turn       string
	Generation string
	TurnID     string
}

// BeginThinkTankAttempt admits the room's next opportunity only if the room is
// still at the revision the caller selected it from and the opportunity is
// unchanged. It freezes the published head, records the actual generation and
// executing turn id, and resets the ordinary messaging budget atomically.
func (s *Store) BeginThinkTankAttempt(b ThinkTankBegin) (ThinkTankAttempt, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ThinkTankAttempt{}, fmt.Errorf("state: begin think tank attempt: %w", err)
	}
	defer tx.Rollback()
	d, err := readThinkTankDetail(tx, b.RoomID)
	if err != nil {
		return ThinkTankAttempt{}, err
	}
	if d.Room.Revision != b.Revision {
		return ThinkTankAttempt{}, thinkTankConflict("room changed since the turn was selected")
	}
	next, ok := NextThinkTankOpportunity(d)
	if !ok || next.AgentID != b.AgentID || next.Turn != b.Turn {
		return ThinkTankAttempt{}, thinkTankConflict("the selected turn is no longer next")
	}
	if b.Generation == "" || b.TurnID == "" {
		return ThinkTankAttempt{}, thinkTankInvalid("generation and turn id are required")
	}
	var assigned int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM tasks WHERE assigned_agent_id = ? AND state IN (?, ?, ?)`,
		b.AgentID, TaskStarting, TaskRunning, TaskWaiting).Scan(&assigned); err != nil {
		return ThinkTankAttempt{}, fmt.Errorf("state: check think tank task assignment: %w", err)
	}
	if assigned != 0 {
		return ThinkTankAttempt{}, thinkTankConflict("the participant has assigned work")
	}
	var checkpoint int64
	for _, m := range d.Members {
		if m.AgentID == b.AgentID {
			checkpoint = m.Checkpoint
		}
	}
	var head int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM think_tank_entries WHERE room_id = ?`, b.RoomID).Scan(&head); err != nil {
		return ThinkTankAttempt{}, fmt.Errorf("state: read think tank head: %w", err)
	}
	attemptID, err := newThinkTankID("tta_", 8)
	if err != nil {
		return ThinkTankAttempt{}, err
	}
	token, err := newThinkTankID("ttk_", 16)
	if err != nil {
		return ThinkTankAttempt{}, err
	}
	if _, err := tx.Exec(`
INSERT INTO think_tank_attempts(attempt_id, room_id, agent_id, turn, token, generation, turn_id, head,
  checkpoint, delivered_to, state, created_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attemptID, b.RoomID, b.AgentID, b.Turn, token, b.Generation, b.TurnID, head, checkpoint,
		checkpoint, ThinkTankAttemptRunning, formatTime(timeNow())); err != nil {
		return ThinkTankAttempt{}, fmt.Errorf("state: insert think tank attempt: %w", err)
	}
	if b.Turn == ThinkTankTurnJudge {
		if _, err := tx.Exec(`UPDATE think_tanks SET judge_status = ? WHERE room_id = ?`,
			ThinkTankJudgeRunning, b.RoomID); err != nil {
			return ThinkTankAttempt{}, fmt.Errorf("state: start think tank judge: %w", err)
		}
	}
	if err := resetTurnBudgetTx(tx, b.AgentID, b.TurnID); err != nil {
		return ThinkTankAttempt{}, err
	}
	if err := bumpThinkTankTx(tx, b.RoomID); err != nil {
		return ThinkTankAttempt{}, err
	}
	a, err := scanThinkTankAttempt(tx.QueryRow(`SELECT `+thinkTankAttemptColumns+` FROM think_tank_attempts WHERE attempt_id = ?`, attemptID))
	if err != nil {
		return ThinkTankAttempt{}, err
	}
	if err := tx.Commit(); err != nil {
		return ThinkTankAttempt{}, fmt.Errorf("state: commit think tank attempt: %w", err)
	}
	return a, nil
}

// ThinkTankReceipt is the opaque proof that an attempt's full new conversation
// view was delivered. It is derived, so it needs no storage.
func thinkTankReadReceipt(a ThinkTankAttempt) string {
	return "ttr_" + strings.TrimPrefix(a.AttemptID, "tta_") + "_" + strconv.FormatInt(a.Head, 10)
}

func thinkTankDelivered(a ThinkTankAttempt) bool {
	return a.DeliveredTo >= a.Head
}

// StageThinkTankTurn records one terminal intent for the caller's current
// attempt (TS-14.R5). Exact replay returns the same staged attempt; a
// conflicting, foreign, ended or stale submission changes nothing. Staging
// neither publishes nor charges: successful turn completion does (R6).
func (s *Store) StageThinkTankTurn(callerAgentID, token, disposition, message, receipt string) (ThinkTankAttempt, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ThinkTankAttempt{}, fmt.Errorf("state: begin stage think tank turn: %w", err)
	}
	defer tx.Rollback()
	a, err := scanThinkTankAttempt(tx.QueryRow(`SELECT `+thinkTankAttemptColumns+` FROM think_tank_attempts WHERE token = ?`, token))
	if errors.Is(err, ErrNotFound) {
		return ThinkTankAttempt{}, ErrThinkTankStale
	} else if err != nil {
		return ThinkTankAttempt{}, err
	}
	if a.AgentID != callerAgentID {
		return ThinkTankAttempt{}, ErrThinkTankNotMember
	}
	if a.State != ThinkTankAttemptRunning {
		return ThinkTankAttempt{}, ErrThinkTankStale
	}
	if a.Disposition != "" {
		if a.Disposition == disposition && a.Message == message {
			return a, nil
		}
		return ThinkTankAttempt{}, ErrThinkTankReplyConflict
	}
	if !thinkTankDelivered(a) || receipt != thinkTankReadReceipt(a) {
		return ThinkTankAttempt{}, ErrThinkTankReadIncomplete
	}
	member, err := scanThinkTankMember(tx.QueryRow(`SELECT `+thinkTankMemberColumns+` FROM think_tank_members WHERE room_id = ? AND agent_id = ?`, a.RoomID, a.AgentID))
	if err != nil {
		return ThinkTankAttempt{}, err
	}
	if len(message) > ThinkTankMaxTextBytes {
		return ThinkTankAttempt{}, thinkTankInvalid("message exceeds %d bytes", ThinkTankMaxTextBytes)
	}
	if !utf8.ValidString(message) {
		return ThinkTankAttempt{}, thinkTankInvalid("message is not valid UTF-8")
	}
	switch disposition {
	case ThinkTankReply:
		if strings.TrimSpace(message) == "" {
			return ThinkTankAttempt{}, thinkTankInvalid("a reply needs a message")
		}
	case ThinkTankLeave:
		if a.Turn != ThinkTankTurnDiscussion && a.Turn != ThinkTankTurnClosing {
			return ThinkTankAttempt{}, thinkTankInvalid("leaving is not available on this turn")
		}
		if !member.MayLeave {
			return ThinkTankAttempt{}, ErrThinkTankLeaveForbidden
		}
	case ThinkTankDeclineClosing:
		if a.Turn != ThinkTankTurnClosing {
			return ThinkTankAttempt{}, ErrThinkTankClosingOnly
		}
		if message != "" {
			return ThinkTankAttempt{}, thinkTankInvalid("decline_closing takes no message")
		}
	default:
		return ThinkTankAttempt{}, thinkTankInvalid("disposition must be reply, leave or decline_closing")
	}
	if _, err := tx.Exec(`UPDATE think_tank_attempts SET disposition = ?, message = ? WHERE attempt_id = ? AND state = ?`,
		disposition, message, a.AttemptID, ThinkTankAttemptRunning); err != nil {
		return ThinkTankAttempt{}, fmt.Errorf("state: stage think tank turn: %w", err)
	}
	a.Disposition, a.Message = disposition, message
	if err := tx.Commit(); err != nil {
		return ThinkTankAttempt{}, fmt.Errorf("state: commit stage think tank turn: %w", err)
	}
	return a, nil
}

// ThinkTankFinish is the outcome of one attempt's completion.
type ThinkTankFinish struct {
	Detail    ThinkTankDetail
	Attempt   ThinkTankAttempt
	Published []ThinkTankEntry
}

// FinalizeThinkTankAttempt seals the running attempt owned by exactly this
// agent generation and executing turn id after successful provider completion
// (TS-14.R6). One transaction publishes the staged contribution or departure,
// charges allowance, advances the read checkpoint to the frozen head, publishes
// queued input, applies pause/end and settles rotation. A turn that staged
// nothing fails instead: assistant output is never substituted. ErrNotFound
// means no attempt matches, so the completion belongs to no room.
func (s *Store) FinalizeThinkTankAttempt(agentID, generation, turnID string) (ThinkTankFinish, error) {
	return s.finishThinkTankAttempt(agentID, generation, turnID, "", 0)
}

// FinalizeThinkTankAttemptAt finalizes like FinalizeThinkTankAttempt and
// anchors a judge's retained synthesis result at the completing event's seq
// in the judge's own history (TS-14.R26).
func (s *Store) FinalizeThinkTankAttemptAt(agentID, generation, turnID string, eventSeq int64) (ThinkTankFinish, error) {
	return s.finishThinkTankAttempt(agentID, generation, turnID, "", eventSeq)
}

// FailThinkTankAttempt holds the room for explicit intervention after a
// cancelled, failed or lost turn, charging nothing (FS-21.R19, R21).
func (s *Store) FailThinkTankAttempt(agentID, generation, turnID, reason string) (ThinkTankFinish, error) {
	if reason == "" {
		reason = "The room turn did not complete."
	}
	return s.finishThinkTankAttempt(agentID, generation, turnID, reason, 0)
}

func (s *Store) RunningThinkTankAttempts(agentID, generation string) ([]ThinkTankAttempt, error) {
	rows, err := s.db.Query(`SELECT `+thinkTankAttemptColumns+` FROM think_tank_attempts WHERE agent_id = ? AND generation = ? AND state = ?`, agentID, generation, ThinkTankAttemptRunning)
	if err != nil {
		return nil, fmt.Errorf("state: read exiting room attempts: %w", err)
	}
	defer rows.Close()
	attempts := []ThinkTankAttempt{}
	for rows.Next() {
		a, err := scanThinkTankAttempt(rows)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

func (s *Store) finishThinkTankAttempt(agentID, generation, turnID, failure string, eventSeq int64) (ThinkTankFinish, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ThinkTankFinish{}, fmt.Errorf("state: begin finish think tank attempt: %w", err)
	}
	defer tx.Rollback()
	a, err := scanThinkTankAttempt(tx.QueryRow(`SELECT `+thinkTankAttemptColumns+` FROM think_tank_attempts
WHERE agent_id = ? AND generation = ? AND turn_id = ? AND state = ?`,
		agentID, generation, turnID, ThinkTankAttemptRunning))
	if err != nil {
		return ThinkTankFinish{}, err
	}
	if failure == "" && a.Disposition == "" {
		failure = "The turn ended without submitting a contribution through submit_think_tank_turn."
	}
	published, err := finishThinkTankAttemptTx(tx, a, failure)
	if err != nil {
		return ThinkTankFinish{}, err
	}
	if err := settleThinkTankTx(tx, a.RoomID); err != nil {
		return ThinkTankFinish{}, err
	}
	if err := bumpThinkTankTx(tx, a.RoomID); err != nil {
		return ThinkTankFinish{}, err
	}
	out := ThinkTankFinish{Published: published}
	if out.Detail, err = readThinkTankDetail(tx, a.RoomID); err != nil {
		return ThinkTankFinish{}, err
	}
	for _, e := range published {
		if e.Kind == ThinkTankEntrySynthesis {
			if err := insertThinkTankResultTx(tx, out.Detail.Room, a, e, eventSeq); err != nil {
				return ThinkTankFinish{}, err
			}
		}
	}
	if out.Attempt, err = scanThinkTankAttempt(tx.QueryRow(`SELECT `+thinkTankAttemptColumns+` FROM think_tank_attempts WHERE attempt_id = ?`, a.AttemptID)); err != nil {
		return ThinkTankFinish{}, err
	}
	if len(out.Published) > 0 {
		// Inputs and barrier publications follow the contribution in the same
		// transaction; report everything after the first new entry.
		if out.Published, err = listThinkTankEntries(tx, a.RoomID, out.Published[0].Seq-1, 1<<62, 1000); err != nil {
			return ThinkTankFinish{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ThinkTankFinish{}, fmt.Errorf("state: commit finish think tank attempt: %w", err)
	}
	return out, nil
}

func finishThinkTankAttemptTx(tx *sql.Tx, a ThinkTankAttempt, failure string) ([]ThinkTankEntry, error) {
	d, err := readThinkTankDetail(tx, a.RoomID)
	if err != nil {
		return nil, err
	}
	var member ThinkTankMember
	for _, m := range d.Members {
		if m.AgentID == a.AgentID {
			member = m
		}
	}
	now := formatTime(timeNow())
	published := []ThinkTankEntry{}
	if failure != "" {
		if _, err := tx.Exec(`UPDATE think_tank_attempts SET state = ?, failure = ?, finished_at = ? WHERE attempt_id = ?`,
			ThinkTankAttemptFailed, failure, now, a.AttemptID); err != nil {
			return nil, fmt.Errorf("state: fail think tank attempt: %w", err)
		}
		if a.Turn == ThinkTankTurnJudge {
			if _, err := tx.Exec(`UPDATE think_tanks SET judge_status = ?, judge_error = ? WHERE room_id = ?`,
				ThinkTankJudgeFailed, failure, a.RoomID); err != nil {
				return nil, fmt.Errorf("state: fail think tank judge: %w", err)
			}
			return published, nil
		}
		if err := setThinkTankHoldTx(tx, a.RoomID, member.AgentName+"'s turn failed: "+failure); err != nil {
			return nil, err
		}
		return published, applyThinkTankBoundaryTx(tx, a.RoomID)
	}

	state, entryKind, charge := ThinkTankAttemptFinalized, "", true
	switch {
	case a.Turn == ThinkTankTurnOpening:
		state = ThinkTankAttemptWithheld
	case a.Turn == ThinkTankTurnJudge:
		entryKind, charge = ThinkTankEntrySynthesis, false
	case a.Disposition == ThinkTankLeave:
		entryKind = ThinkTankEntryDeparture
	case a.Disposition == ThinkTankDeclineClosing:
		charge = false
	case a.Turn == ThinkTankTurnClosing:
		entryKind = ThinkTankEntryClosing
	default:
		entryKind = ThinkTankEntryReply
	}
	var seq int64
	if entryKind != "" {
		e := ThinkTankEntry{RoomID: a.RoomID, Kind: entryKind, AgentID: a.AgentID, AgentName: member.AgentName,
			Project: member.Project, Body: a.Message, AttemptID: a.AttemptID}
		if seq, err = insertThinkTankEntryTx(tx, e); err != nil {
			return nil, err
		}
		e.Seq = seq
		published = append(published, e)
	}
	if _, err := tx.Exec(`UPDATE think_tank_attempts SET state = ?, entry_seq = ?, finished_at = ? WHERE attempt_id = ?`,
		state, seq, now, a.AttemptID); err != nil {
		return nil, fmt.Errorf("state: finalize think tank attempt: %w", err)
	}
	if a.Turn == ThinkTankTurnJudge {
		if _, err := tx.Exec(`UPDATE think_tanks SET judge_status = ?, judge_error = '' WHERE room_id = ?`,
			ThinkTankJudgeCompleted, a.RoomID); err != nil {
			return nil, fmt.Errorf("state: complete think tank judge: %w", err)
		}
		return published, nil
	}
	if charge {
		memberState := member.State
		if a.Disposition == ThinkTankLeave {
			memberState = ThinkTankMemberDeparted
		} else if member.Completed+1 >= member.Cap {
			memberState = ThinkTankMemberExhausted
		}
		// The checkpoint advances to the frozen head the turn was given, never
		// to its own reply or the latest seq (TS-14.R7).
		if _, err := tx.Exec(`UPDATE think_tank_members SET completed = completed + 1, state = ?, checkpoint = ?
WHERE room_id = ? AND agent_id = ?`, memberState, a.Head, a.RoomID, a.AgentID); err != nil {
			return nil, fmt.Errorf("state: charge think tank member: %w", err)
		}
	}
	if a.Turn == ThinkTankTurnDiscussion {
		if _, err := tx.Exec(`UPDATE think_tanks SET rotation = ? WHERE room_id = ?`, member.Order+1, a.RoomID); err != nil {
			return nil, fmt.Errorf("state: rotate think tank: %w", err)
		}
	}
	return published, applyThinkTankBoundaryTx(tx, a.RoomID)
}

// applyThinkTankBoundaryTx runs at a finished room turn: queued input
// publishes (except during openings, where it waits for the barrier), then a
// pending pause or end takes effect (FS-21.R18, R32, R35).
func applyThinkTankBoundaryTx(tx *sql.Tx, roomID string) error {
	d, err := readThinkTankDetail(tx, roomID)
	if err != nil {
		return err
	}
	switch d.Room.Control {
	case ThinkTankEndRequested:
		return endThinkTankTx(tx, d, ThinkTankEndOperator)
	case ThinkTankPauseRequested:
		if err := setThinkTankControlTx(tx, roomID, ThinkTankPaused); err != nil {
			return err
		}
	}
	if d.Room.Phase != ThinkTankPhaseOpenings {
		return publishPendingInputsTx(tx, roomID)
	}
	return nil
}

// AddThinkTankInput records one user message or Room-target annotation. With
// no room turn active it publishes at once; during a turn or the openings it is
// held for the boundary (FS-21.R35). An exact CommandID replay returns the
// original input; ended discussion refuses Room delivery (R30).
func (s *Store) AddThinkTankInput(roomID, commandID, kind, body, context string) (ThinkTankInput, ThinkTankDetail, error) {
	return s.addThinkTankInput(roomID, commandID, kind, body, context, nil, false, nil)
}

// AddThinkTankMessage adds shared user input with optional selected
// mentions. Mentions snapshot their addressees into the input context; they
// grant nothing and change no speaker order (FS-21.R46, TS-14.R25).
func (s *Store) AddThinkTankMessage(roomID, commandID, body string, mentions []ThinkTankMention) (ThinkTankInput, ThinkTankDetail, error) {
	if mentions == nil {
		mentions = []ThinkTankMention{}
	}
	return s.addThinkTankInput(roomID, commandID, ThinkTankEntryUser, body, "", mentions, false, nil)
}

// AddThinkTankRecord records a room-source annotation delivered to a selected
// or new agent. It enters canonical history at once, in any phase, before that
// delivery happens; it is not shared input held for a turn boundary
// (FS-13.R26, FS-21.R30).
func (s *Store) AddThinkTankRecord(roomID, commandID, body, context string) (ThinkTankInput, ThinkTankDetail, error) {
	return s.addThinkTankInput(roomID, commandID, ThinkTankEntryAnnotation, body, context, nil, true, nil)
}

// AddThinkTankAnnotationMail commits the room record, recipient mail and wake
// receipt together. Exact command replay leaves all three unchanged.
func (s *Store) AddThinkTankAnnotationMail(roomID, commandID, body, context string, mail Message) (ThinkTankInput, ThinkTankDetail, error) {
	return s.addThinkTankInput(roomID, commandID, ThinkTankEntryAnnotation, body, context, nil, true, func(tx *sql.Tx) error {
		mail.Body, mail.Wake, mail.DeliveredVia = body, true, DeliveryPending
		if _, err := insertMessageTx(tx, mail); err != nil {
			return err
		}
		return EnsurePendingMailActivationTx(tx, mail.ToAgent)
	})
}

// addThinkTankInput stores one input. Non-nil mentions (user messages only)
// replace context with the validated addressee snapshot; replay compares the
// mention intent rather than the snapshot.
func (s *Store) addThinkTankInput(roomID, commandID, kind, body, context string, mentions []ThinkTankMention, record bool, effect func(*sql.Tx) error) (ThinkTankInput, ThinkTankDetail, error) {
	if kind != ThinkTankEntryUser && kind != ThinkTankEntryAnnotation {
		return ThinkTankInput{}, ThinkTankDetail{}, thinkTankInvalid("unknown input kind %q", kind)
	}
	if strings.TrimSpace(commandID) == "" {
		return ThinkTankInput{}, ThinkTankDetail{}, thinkTankInvalid("command id is required")
	}
	if strings.TrimSpace(body) == "" {
		return ThinkTankInput{}, ThinkTankDetail{}, thinkTankInvalid("message is empty")
	}
	if len(body)+len(context) > ThinkTankMaxTextBytes {
		return ThinkTankInput{}, ThinkTankDetail{}, thinkTankInvalid("message exceeds %d bytes", ThinkTankMaxTextBytes)
	}
	var input ThinkTankInput
	d, err := s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		existing, err := scanThinkTankInput(tx.QueryRow(`SELECT input_id, room_id, command_id, kind, body, context, entry_seq, created_at
FROM think_tank_inputs WHERE room_id = ? AND command_id = ?`, roomID, commandID))
		if err == nil {
			sameContext := existing.Context == context
			if mentions != nil {
				sameContext = sameThinkTankMentions(existing.Context, mentions)
			}
			if existing.Kind != kind || existing.Body != body || !sameContext {
				return thinkTankConflict("command id reused for a different message")
			}
			input = existing
			return errNoThinkTankChange
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if d.Room.Phase == ThinkTankPhaseEnded && !record {
			return thinkTankConflict("discussion has ended")
		}
		if len(mentions) > 0 {
			if context, err = thinkTankMentionContext(d, body, mentions); err != nil {
				return err
			}
		}
		id, err := newThinkTankID("tti_", 8)
		if err != nil {
			return err
		}
		input = ThinkTankInput{InputID: id, RoomID: roomID, CommandID: commandID, Kind: kind, Body: body,
			Context: context, CreatedAt: timeNow()}
		if _, err := tx.Exec(`INSERT INTO think_tank_inputs(input_id, room_id, command_id, kind, body, context, created_at)
VALUES(?, ?, ?, ?, ?, ?, ?)`, id, roomID, commandID, kind, body, context, formatTime(input.CreatedAt)); err != nil {
			return fmt.Errorf("state: insert think tank input: %w", err)
		}
		if record {
			seq, err := insertThinkTankEntryTx(tx, ThinkTankEntry{RoomID: roomID, Kind: kind, Body: body, InputID: id, Context: context})
			if err != nil {
				return err
			}
			input.EntrySeq = seq
			if _, err = tx.Exec(`UPDATE think_tank_inputs SET entry_seq = ? WHERE input_id = ?`, seq, id); err != nil {
				return err
			}
			if effect != nil {
				return effect(tx)
			}
			return nil
		}
		if d.Active == nil && d.Room.Phase != ThinkTankPhaseOpenings && d.Room.Phase != ThinkTankPhaseSetup {
			if err := publishPendingInputsTx(tx, roomID); err != nil {
				return err
			}
			return tx.QueryRow(`SELECT entry_seq FROM think_tank_inputs WHERE input_id = ?`, id).Scan(&input.EntrySeq)
		}
		return nil
	})
	if err != nil {
		return ThinkTankInput{}, ThinkTankDetail{}, err
	}
	return input, d, nil
}

// ReserveThinkTankJudge persists the fresh judge's reserved identity before
// its launch effects (TS-14.R2, R13).
func (s *Store) ReserveThinkTankJudge(roomID, agentID, agentName, project string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		if d.Room.Phase != ThinkTankPhaseEnded || d.Room.JudgeStatus != ThinkTankJudgeReady {
			return thinkTankConflict("the judge is not ready to launch")
		}
		if _, err := tx.Exec(`UPDATE think_tanks SET judge_status = ?, judge_agent_id = ?, judge_error = '' WHERE room_id = ?`,
			ThinkTankJudgeLaunching, agentID, roomID); err != nil {
			return fmt.Errorf("state: reserve think tank judge: %w", err)
		}
		if _, err := tx.Exec(`
INSERT INTO think_tank_members(room_id, agent_id, agent_name, project, role, ord, cap, may_leave, state, setup_state)
VALUES(?, ?, ?, ?, ?, (SELECT COUNT(*) FROM think_tank_members WHERE room_id = ?), 1, 0, ?, ?)`,
			roomID, agentID, agentName, project, ThinkTankRoleJudge, roomID, ThinkTankMemberActive,
			ThinkTankSetupPending); err != nil {
			return fmt.Errorf("state: insert think tank judge: %w", err)
		}
		return nil
	})
}

// MarkThinkTankJudgeLaunched records the reserved judge's launch outcome.
func (s *Store) MarkThinkTankJudgeLaunched(roomID, name, launchErr string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		if d.Room.JudgeStatus != ThinkTankJudgeLaunching {
			return thinkTankConflict("the judge is not launching")
		}
		status, setup := ThinkTankJudgeStarting, ThinkTankSetupReady
		if launchErr != "" {
			status, setup = ThinkTankJudgeFailed, ThinkTankSetupFailed
		}
		if _, err := tx.Exec(`UPDATE think_tanks SET judge_status = ?, judge_error = ? WHERE room_id = ?`,
			status, launchErr, roomID); err != nil {
			return fmt.Errorf("state: mark think tank judge: %w", err)
		}
		if _, err := tx.Exec(`UPDATE think_tank_members SET setup_state = ?, setup_error = ?,
  agent_name = CASE WHEN ? = '' THEN agent_name ELSE ? END
WHERE room_id = ? AND agent_id = ?`, setup, launchErr, name, name, roomID, d.Room.JudgeAgentID); err != nil {
			return fmt.Errorf("state: mark think tank judge member: %w", err)
		}
		return nil
	})
}

// RetryThinkTankJudge makes a failed final step ready again with optionally
// repaired launch settings. The retry launches a fresh judge; the failed one
// stays an ordinary agent. Discussion is not reopened (FS-21.R36).
func (s *Store) RetryThinkTankJudge(roomID, judgeConfig string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		if d.Room.JudgeStatus != ThinkTankJudgeFailed {
			return thinkTankConflict("the judge has not failed")
		}
		if judgeConfig == "" {
			judgeConfig = d.Room.JudgeConfig
		}
		_, err := tx.Exec(`UPDATE think_tanks SET judge_status = ?, judge_config = ?, judge_agent_id = '', judge_error = ''
WHERE room_id = ?`, ThinkTankJudgeReady, judgeConfig, roomID)
		return err
	})
}

// RetryThinkTankSetup returns failed new-participant slots to pending and
// clears the setup hold; ready slots are never relaunched (TS-14.R2).
func (s *Store) RetryThinkTankSetup(roomID string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		if d.Room.Phase != ThinkTankPhaseSetup {
			return thinkTankConflict("the room is not in setup")
		}
		if _, err := tx.Exec(`UPDATE think_tank_members SET setup_state = ?, setup_error = ''
WHERE room_id = ? AND setup_state = ?`, ThinkTankSetupPending, roomID, ThinkTankSetupFailed); err != nil {
			return fmt.Errorf("state: retry think tank setup: %w", err)
		}
		return setThinkTankHoldTx(tx, roomID, "")
	})
}

// RecoverThinkTanks fences every uncertain attempt at startup and holds
// unfinished rooms for explicit resume; nothing is replayed (TS-14.R14).
func (s *Store) RecoverThinkTanks() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin recover think tanks: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT `+thinkTankAttemptColumns+` FROM think_tank_attempts WHERE state = ?`, ThinkTankAttemptRunning)
	if err != nil {
		return fmt.Errorf("state: read running think tank attempts: %w", err)
	}
	running := []ThinkTankAttempt{}
	for rows.Next() {
		a, err := scanThinkTankAttempt(rows)
		if err != nil {
			rows.Close()
			return err
		}
		running = append(running, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("state: iterate running think tank attempts: %w", err)
	}
	rows.Close()
	if _, err := tx.Exec(`UPDATE think_tank_members SET setup_state = ?, setup_error = ? WHERE role = ? AND setup_state = ?`,
		ThinkTankSetupFailed, "Chuck restarted while this participant was starting.", ThinkTankRoleParticipant, ThinkTankSetupLaunching); err != nil {
		return fmt.Errorf("state: fence think tank setup launch: %w", err)
	}
	for _, a := range running {
		if _, err := finishThinkTankAttemptTx(tx, a, "Chuck restarted during this turn; it was not replayed."); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE think_tanks SET judge_status = ?, judge_error = ?
WHERE judge_status = ?`, ThinkTankJudgeFailed, "Chuck restarted while the judge was starting.", ThinkTankJudgeLaunching); err != nil {
		return fmt.Errorf("state: fence think tank judge launch: %w", err)
	}
	if _, err := tx.Exec(`UPDATE think_tanks SET control = ?,
  hold = CASE WHEN hold = '' THEN 'Chuck restarted; resume to continue.' ELSE hold END,
  revision = revision + 1
WHERE phase != ? AND control IN (?, ?)`, ThinkTankPaused, ThinkTankPhaseEnded, ThinkTankRunning, ThinkTankPauseRequested); err != nil {
		return fmt.Errorf("state: hold think tanks: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit recover think tanks: %w", err)
	}
	return nil
}

// Reading the room ---------------------------------------------------------

const (
	ThinkTankViewContext = "context"
	ThinkTankViewHistory = "history"
)

type ThinkTankReadRequest struct {
	CallerAgentID string
	RoomID        string
	View          string
	Cursor        string
}

type ThinkTankReadItem struct {
	Entry ThinkTankEntry
	// Offset is the byte offset of Entry.Body within the full entry; Continues
	// marks more of this entry on the next page.
	Offset    int
	Continues bool
	// Own marks the reader's own earlier contribution, omitted from the
	// default delta view to avoid duplicate input (TS-14.R7).
	Own bool
}

type ThinkTankReadPage struct {
	Room       ThinkTank
	Member     ThinkTankMember
	Attempt    *ThinkTankAttempt
	View       string
	First      bool
	Items      []ThinkTankReadItem
	NextCursor string
	Complete   bool
	// ReadReceipt and TurnToken are present only for the caller's current
	// attempt once its full new view has been delivered.
	ReadReceipt string
	TurnToken   string
}

type thinkTankCursor struct {
	room, view      string
	caller, attempt string
	head, seq       int64
	off             int
}

func (c thinkTankCursor) encode() string {
	raw := fmt.Sprintf("2|%s|%s|%d|%d|%d|%s|%s", c.room, c.view, c.head, c.seq, c.off, c.caller, c.attempt)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeThinkTankCursor(s string) (thinkTankCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return thinkTankCursor{}, ErrThinkTankCursor
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 8 || parts[0] != "2" {
		return thinkTankCursor{}, ErrThinkTankCursor
	}
	c := thinkTankCursor{room: parts[1], view: parts[2], caller: parts[6], attempt: parts[7]}
	var e1, e2, e3 error
	c.head, e1 = strconv.ParseInt(parts[3], 10, 64)
	c.seq, e2 = strconv.ParseInt(parts[4], 10, 64)
	c.off, e3 = strconv.Atoi(parts[5])
	if e1 != nil || e2 != nil || e3 != nil || c.off < 0 {
		return thinkTankCursor{}, ErrThinkTankCursor
	}
	return c, nil
}

// ReadThinkTank returns one bounded page of the room for a recorded member.
// Context pages run from the committed checkpoint to the current attempt's
// frozen head and record contiguous delivery; history pages revisit published
// entries without advancing delivery (TS-14.R7, §3).
func (s *Store) ReadThinkTankPage(req ThinkTankReadRequest) (ThinkTankReadPage, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ThinkTankReadPage{}, fmt.Errorf("state: begin read think tank: %w", err)
	}
	defer tx.Rollback()
	view := req.View
	if view == "" {
		view = ThinkTankViewContext
	}
	if view != ThinkTankViewContext && view != ThinkTankViewHistory {
		return ThinkTankReadPage{}, thinkTankInvalid("view must be context or history")
	}
	var current *ThinkTankAttempt
	roomID := req.RoomID
	a, err := scanThinkTankAttempt(tx.QueryRow(`SELECT `+thinkTankAttemptColumns+` FROM think_tank_attempts
WHERE agent_id = ? AND state = ? AND (? = '' OR room_id = ?) ORDER BY created_at DESC LIMIT 1`,
		req.CallerAgentID, ThinkTankAttemptRunning, roomID, roomID))
	switch {
	case err == nil:
		current = &a
		roomID = a.RoomID
	case !errors.Is(err, ErrNotFound):
		return ThinkTankReadPage{}, err
	case roomID == "":
		return ThinkTankReadPage{}, ErrThinkTankNoTurn
	}
	member, err := scanThinkTankMember(tx.QueryRow(`SELECT `+thinkTankMemberColumns+` FROM think_tank_members
WHERE room_id = ? AND agent_id = ? ORDER BY ord LIMIT 1`, roomID, req.CallerAgentID))
	if errors.Is(err, ErrNotFound) {
		return ThinkTankReadPage{}, ErrThinkTankNotMember
	} else if err != nil {
		return ThinkTankReadPage{}, err
	}
	room, err := scanThinkTank(tx.QueryRow(`SELECT `+thinkTankColumns+` FROM think_tanks WHERE room_id = ?`, roomID))
	if err != nil {
		return ThinkTankReadPage{}, err
	}
	var latest int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM think_tank_entries WHERE room_id = ?`, roomID).Scan(&latest); err != nil {
		return ThinkTankReadPage{}, fmt.Errorf("state: read think tank head: %w", err)
	}
	from, head := int64(0), latest
	if view == ThinkTankViewContext {
		from = member.Checkpoint
		if current != nil {
			from, head = current.Checkpoint, current.Head
		}
	}
	pos := thinkTankCursor{room: roomID, view: view, head: head, seq: from + 1, caller: req.CallerAgentID}
	if current != nil {
		pos.attempt = current.AttemptID
	}
	first := req.Cursor == ""
	if !first {
		c, err := decodeThinkTankCursor(req.Cursor)
		if err != nil {
			return ThinkTankReadPage{}, err
		}
		if c.room != roomID || c.view != view || c.head != head || c.seq <= from || c.seq > head ||
			c.caller != pos.caller || c.attempt != pos.attempt {
			return ThinkTankReadPage{}, ErrThinkTankCursor
		}
		var body string
		if err := tx.QueryRow(`SELECT body FROM think_tank_entries WHERE room_id = ? AND seq = ?`, roomID, c.seq).Scan(&body); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ThinkTankReadPage{}, ErrThinkTankCursor
			}
			return ThinkTankReadPage{}, err
		}
		if c.off > len(body) || (c.off > 0 && (c.off == len(body) || !utf8.RuneStart(body[c.off]))) {
			return ThinkTankReadPage{}, ErrThinkTankCursor
		}
		pos = c
	}
	page := ThinkTankReadPage{Room: room, Member: member, Attempt: current, View: view, First: first, Items: []ThinkTankReadItem{}}
	budget := ThinkTankPageBytes
	seq, off := pos.seq, pos.off
	for budget > 0 && seq <= head {
		entries, err := listThinkTankEntries(tx, roomID, seq-1, head, 64)
		if err != nil {
			return ThinkTankReadPage{}, err
		}
		if len(entries) == 0 {
			seq = head + 1
			break
		}
		for _, e := range entries {
			if budget <= 0 {
				break
			}
			if e.Seq > seq {
				seq, off = e.Seq, 0
			}
			item := ThinkTankReadItem{Entry: e, Offset: off}
			own := view == ThinkTankViewContext && e.AgentID == req.CallerAgentID && e.InputID == ""
			if own {
				item.Own, item.Entry.Body = true, ""
				page.Items = append(page.Items, item)
				budget -= 64
				seq, off = e.Seq+1, 0
				continue
			}
			body := e.Body[off:]
			if len(body) > budget {
				cut := budget
				for cut > 0 && !utf8.RuneStart(body[cut]) {
					cut--
				}
				if cut == 0 && len(page.Items) > 0 {
					budget = 0
					break
				}
				if cut == 0 {
					_, size := utf8.DecodeRuneInString(body)
					cut = size
				}
				item.Entry.Body, item.Continues = body[:cut], true
				page.Items = append(page.Items, item)
				off += cut
				budget = 0
				break
			}
			item.Entry.Body = body
			page.Items = append(page.Items, item)
			budget -= len(body) + 64
			seq, off = e.Seq+1, 0
		}
	}
	page.Complete = seq > head
	if !page.Complete {
		next := pos
		next.seq, next.off = seq, off
		page.NextCursor = next.encode()
	}
	if current != nil && view == ThinkTankViewContext {
		// Delivery is contiguous only when this page began at or before the
		// already delivered position.
		startsWithin := pos.seq < current.DeliveredTo+1 ||
			(pos.seq == current.DeliveredTo+1 && pos.off <= current.DeliveredAt)
		if startsWithin && (seq-1 > current.DeliveredTo || (seq-1 == current.DeliveredTo && off > current.DeliveredAt)) {
			if _, err := tx.Exec(`UPDATE think_tank_attempts SET delivered_to = ?, delivered_at = ? WHERE attempt_id = ? AND state = ?`,
				seq-1, off, current.AttemptID, ThinkTankAttemptRunning); err != nil {
				return ThinkTankReadPage{}, fmt.Errorf("state: record think tank delivery: %w", err)
			}
			current.DeliveredTo, current.DeliveredAt = seq-1, off
		}
	}
	if current != nil {
		page.TurnToken = current.Token
		if thinkTankDelivered(*current) {
			page.ReadReceipt = thinkTankReadReceipt(*current)
		}
	}
	if err := tx.Commit(); err != nil {
		return ThinkTankReadPage{}, fmt.Errorf("state: commit read think tank: %w", err)
	}
	return page, nil
}
