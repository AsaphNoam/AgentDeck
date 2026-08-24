package state

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	ActivationKindMail = "mail"

	ActivationPending   = "pending"
	ActivationClaimed   = "claimed"
	ActivationAttempted = "attempted"
)

// Activation is operational control state. It intentionally has no mail or
// conversation payload: the mail reader remains the check_messages MCP tool.
type Activation struct {
	ActivationID string
	AgentID      string
	Kind         string
	State        string
	ClaimToken   string
	CreatedAt    time.Time
	ClaimedAt    *time.Time
	AttemptedAt  *time.Time
}

func migrateActivations(tx *sql.Tx) error {
	if _, err := tx.Exec(`
CREATE TABLE activations (
  activation_id TEXT PRIMARY KEY,
  agent_id      TEXT NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
  kind          TEXT NOT NULL,
  state         TEXT NOT NULL,
  claim_token   TEXT NOT NULL DEFAULT '',
  created_at    TEXT NOT NULL,
  claimed_at    TEXT,
  attempted_at  TEXT
);
CREATE UNIQUE INDEX idx_activations_one_pending_mail
  ON activations(agent_id) WHERE state = 'pending' AND kind = 'mail';
CREATE INDEX idx_activations_pending
  ON activations(state, created_at);
`); err != nil {
		return fmt.Errorf("state: create activations: %w", err)
	}
	// Old nudge/wake markers have already crossed the former attempt boundary.
	// Only untouched pending mail is safe to backfill.
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO activations(activation_id, agent_id, kind, state, created_at)
SELECT 'a_' || lower(hex(randomblob(12))), m.to_agent, 'mail', 'pending', ?
FROM messages m
JOIN agents a ON a.agent_id = m.to_agent
WHERE m.read = 0 AND m.delivered_via = 'pending'
GROUP BY m.to_agent`, formatTime(time.Now().UTC())); err != nil {
		return fmt.Errorf("state: backfill mail activations: %w", err)
	}
	return nil
}

// EnsurePendingMailActivationTx coalesces an unread mail batch into one pending
// mail opportunity. Callers that insert mail must use this in their transaction.
func EnsurePendingMailActivationTx(tx *sql.Tx, agentID string) error {
	var b [12]byte
	if _, err := randRead(b[:]); err != nil {
		return fmt.Errorf("state: read activation random: %w", err)
	}
	_, err := tx.Exec(`
INSERT OR IGNORE INTO activations(activation_id, agent_id, kind, state, created_at)
SELECT ?, agent_id, ?, ?, ? FROM agents WHERE agent_id = ?`,
		"a_"+hex.EncodeToString(b[:]), ActivationKindMail, ActivationPending,
		formatTime(time.Now().UTC()), agentID)
	if err != nil {
		return fmt.Errorf("state: ensure pending mail activation: %w", err)
	}
	return nil
}

// PendingMailActivations lists at most limit rows of durable mail work waiting
// for the executor, oldest first. The limit is required: the caller decides how
// much of a backlog one sweep admits, and whatever it leaves stays pending for
// the next one (TS-01.R20, INV §9).
func (s *Store) PendingMailActivations(agentID string, limit int) ([]Activation, error) {
	rows, err := s.db.Query(`
SELECT activation_id, agent_id, kind, state, claim_token, created_at, claimed_at, attempted_at
FROM activations
WHERE kind = ? AND state = ? AND (? = '' OR agent_id = ?)
ORDER BY created_at, activation_id
LIMIT ?`, ActivationKindMail, ActivationPending, agentID, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("state: list pending mail activations: %w", err)
	}
	defer rows.Close()
	out := []Activation{}
	for rows.Next() {
		a, err := scanActivation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate pending mail activations: %w", err)
	}
	return out, nil
}

func scanActivation(rows *sql.Rows) (Activation, error) {
	var a Activation
	var createdAt string
	var claimedAt, attemptedAt sql.NullString
	if err := rows.Scan(&a.ActivationID, &a.AgentID, &a.Kind, &a.State, &a.ClaimToken,
		&createdAt, &claimedAt, &attemptedAt); err != nil {
		return Activation{}, fmt.Errorf("state: scan activation: %w", err)
	}
	var err error
	if a.CreatedAt, err = parseTime(createdAt); err != nil {
		return Activation{}, wrapTimeErr("activation.created_at", err)
	}
	if a.ClaimedAt, err = parseOptionalTime(claimedAt); err != nil {
		return Activation{}, wrapTimeErr("activation.claimed_at", err)
	}
	if a.AttemptedAt, err = parseOptionalTime(attemptedAt); err != nil {
		return Activation{}, wrapTimeErr("activation.attempted_at", err)
	}
	return a, nil
}

// ClaimMailActivation atomically decides one pending activation's fate: it
// reserves the opportunity only if the recipient still has unread mail, and
// otherwise retires it (FS-06.R25). Testing availability and claiming in two
// statements let a concurrent check_messages or dashboard mailbox read drain the
// last unread row in between, after which the claim crossed the non-replayable
// attempt boundary and sent an empty provider turn (INV §5/§15). Each statement
// re-tests both facts, so a claim implies mail existed at claim time and a retire
// implies the mailbox was empty at retire time. A false result with no error
// means another executor won the claim or the opportunity was retired.
func (s *Store) ClaimMailActivation(activationID string) (string, bool, error) {
	var b [16]byte
	if _, err := randRead(b[:]); err != nil {
		return "", false, fmt.Errorf("state: read activation claim random: %w", err)
	}
	token := hex.EncodeToString(b[:])
	res, err := s.db.Exec(`
UPDATE activations
SET state = ?, claim_token = ?, claimed_at = ?
WHERE activation_id = ? AND kind = ? AND state = ?
  AND EXISTS (SELECT 1 FROM messages WHERE to_agent = activations.agent_id AND read = 0)`,
		ActivationClaimed, token, formatTime(time.Now().UTC()),
		activationID, ActivationKindMail, ActivationPending)
	if err != nil {
		return "", false, fmt.Errorf("state: claim mail activation: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return token, true, nil
	}
	// Either another executor won the claim (state is no longer pending, so the
	// delete matches nothing) or the mailbox drained first. Mail inserted between
	// the two statements fails NOT EXISTS and correctly leaves the row pending.
	if _, err := s.db.Exec(`
DELETE FROM activations
WHERE activation_id = ? AND kind = ? AND state = ?
  AND NOT EXISTS (SELECT 1 FROM messages WHERE to_agent = activations.agent_id AND read = 0)`,
		activationID, ActivationKindMail, ActivationPending); err != nil {
		return "", false, fmt.Errorf("state: retire empty mail activation: %w", err)
	}
	return "", false, nil
}

// ReleaseMailActivation makes a pre-attempt claim actionable again. The token
// prevents a delayed worker from releasing a later claim.
func (s *Store) ReleaseMailActivation(activationID, token string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin release mail activation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
DELETE FROM activations AS claimed
WHERE activation_id = ? AND kind = ? AND state = ? AND claim_token = ?
  AND EXISTS (
    SELECT 1 FROM activations AS pending
    WHERE pending.agent_id = claimed.agent_id AND pending.kind = claimed.kind AND pending.state = ?
  )`, activationID, ActivationKindMail, ActivationClaimed, token, ActivationPending); err != nil {
		return fmt.Errorf("state: coalesce released mail activation: %w", err)
	}
	if _, err := tx.Exec(`
UPDATE activations
SET state = ?, claim_token = '', claimed_at = NULL
WHERE activation_id = ? AND kind = ? AND state = ? AND claim_token = ?`,
		ActivationPending, activationID, ActivationKindMail, ActivationClaimed, token); err != nil {
		return fmt.Errorf("state: release mail activation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit release mail activation: %w", err)
	}
	return nil
}

// DiscardClaimedMailActivation removes a pre-attempt opportunity whose recipient
// is durably ineligible to receive mail activation. The token prevents a delayed
// worker from discarding a later claim.
func (s *Store) DiscardClaimedMailActivation(activationID, token string) error {
	_, err := s.db.Exec(`
DELETE FROM activations
WHERE activation_id = ? AND kind = ? AND state = ? AND claim_token = ?`,
		activationID, ActivationKindMail, ActivationClaimed, token)
	if err != nil {
		return fmt.Errorf("state: discard claimed mail activation: %w", err)
	}
	return nil
}

// AttemptMailActivation crosses mail's non-replayable boundary before a wake or
// provider side effect. A running activation supplies turnID so its budget reset
// commits atomically with this boundary; a stopped activation supplies "" and
// opens its freshly resumed turn through StartAttemptedMailTurn.
func (s *Store) AttemptMailActivation(activationID, token, turnID string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("state: begin mail activation attempt: %w", err)
	}
	defer tx.Rollback()
	var agentID string
	if err := tx.QueryRow(`
SELECT agent_id FROM activations
WHERE activation_id = ? AND kind = ? AND state = ? AND claim_token = ?`,
		activationID, ActivationKindMail, ActivationClaimed, token).Scan(&agentID); err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("state: read claimed mail activation: %w", err)
	}
	res, err := tx.Exec(`
UPDATE activations
SET state = ?, attempted_at = ?
WHERE activation_id = ? AND kind = ? AND state = ? AND claim_token = ?`,
		ActivationAttempted, formatTime(time.Now().UTC()), activationID, ActivationKindMail,
		ActivationClaimed, token)
	if err != nil {
		return false, fmt.Errorf("state: mark mail activation attempted: %w", err)
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return false, nil
	}
	if turnID != "" {
		if err := resetTurnBudgetTx(tx, agentID, turnID); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("state: commit mail activation attempt: %w", err)
	}
	return true, nil
}

// StartAttemptedMailTurn initializes the runtime turn budget for the claimed
// activation that already crossed the wake boundary.
func (s *Store) StartAttemptedMailTurn(activationID, token, turnID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin attempted mail turn: %w", err)
	}
	defer tx.Rollback()
	var agentID string
	if err := tx.QueryRow(`
SELECT agent_id FROM activations
WHERE activation_id = ? AND kind = ? AND state = ? AND claim_token = ?`,
		activationID, ActivationKindMail, ActivationAttempted, token).Scan(&agentID); err != nil {
		return fmt.Errorf("state: read attempted mail activation: %w", err)
	}
	if err := resetTurnBudgetTx(tx, agentID, turnID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit attempted mail turn: %w", err)
	}
	return nil
}

// RetireMailActivation removes an attempted record after its one bounded
// handoff. Attempted rows are not history and are never replayed.
func (s *Store) RetireMailActivation(activationID, token string) error {
	_, err := s.db.Exec(`
DELETE FROM activations
WHERE activation_id = ? AND kind = ? AND state = ? AND claim_token = ?`,
		activationID, ActivationKindMail, ActivationAttempted, token)
	if err != nil {
		return fmt.Errorf("state: retire mail activation: %w", err)
	}
	return nil
}

// RecoverMailActivations repairs only pre-side-effect claims and discards
// already-attempted records. Mail's at-most-once cleanup is deliberately not a
// generic activation policy.
func (s *Store) RecoverMailActivations() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin recover mail activations: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM activations WHERE kind = ? AND state = ?`, ActivationKindMail, ActivationAttempted); err != nil {
		return fmt.Errorf("state: delete attempted mail activations: %w", err)
	}
	if _, err := tx.Exec(`
DELETE FROM activations AS claimed
WHERE kind = ? AND state = ?
  AND EXISTS (
    SELECT 1 FROM activations AS pending
    WHERE pending.agent_id = claimed.agent_id AND pending.kind = claimed.kind AND pending.state = ?
  )`, ActivationKindMail, ActivationClaimed, ActivationPending); err != nil {
		return fmt.Errorf("state: coalesce claimed mail activations: %w", err)
	}
	if _, err := tx.Exec(`
UPDATE activations SET state = ?, claim_token = '', claimed_at = NULL
WHERE kind = ? AND state = ?`, ActivationPending, ActivationKindMail, ActivationClaimed); err != nil {
		return fmt.Errorf("state: release claimed mail activations: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit recover mail activations: %w", err)
	}
	return nil
}
