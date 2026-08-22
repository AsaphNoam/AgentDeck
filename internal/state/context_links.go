package state

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Context source kinds (FS-15.R2). The vocabulary is closed: each kind fixes
// which locator columns are meaningful, and a reference is keyed only by that
// locator — never by a target, label, creator, or read state (TS-02.R24).
const (
	ContextSourceTranscriptSpan = "transcript_span"
	ContextSourcePipelineReport = "pipeline_attempt_report"
)

// ErrInvalidContextSource reports a locator that does not match its kind's
// shape. Share-time validation rejects it before any row is written (FS-15.R15).
var ErrInvalidContextSource = errors.New("state: invalid context source locator")

// ContextSource is the immutable typed locator that gives a reference its
// identity. It carries no presentation, target, or copied source content.
type ContextSource struct {
	Kind              string `json:"kind"`
	AgentID           string `json:"agent_id,omitempty"`
	FirstSeq          int64  `json:"first_seq,omitempty"`
	LastSeq           int64  `json:"last_seq,omitempty"`
	PipelineAttemptID string `json:"pipeline_attempt_id,omitempty"`
}

// Validate enforces the kind-specific locator shape: a transcript span has one
// source agent and positive ordered sequence bounds and no attempt id; a
// pipeline report has one attempt id and no transcript locator.
func (s ContextSource) Validate() error {
	switch s.Kind {
	case ContextSourceTranscriptSpan:
		if s.AgentID == "" || s.FirstSeq <= 0 || s.LastSeq < s.FirstSeq || s.PipelineAttemptID != "" {
			return fmt.Errorf("%w: transcript span %+v", ErrInvalidContextSource, s)
		}
	case ContextSourcePipelineReport:
		if s.PipelineAttemptID == "" || s.AgentID != "" || s.FirstSeq != 0 || s.LastSeq != 0 {
			return fmt.Errorf("%w: pipeline report %+v", ErrInvalidContextSource, s)
		}
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidContextSource, s.Kind)
	}
	return nil
}

// ContextReference is the canonical, immutable reference row.
type ContextReference struct {
	ContextRefID string        `json:"context_ref_id"`
	Source       ContextSource `json:"source"`
	CreatedAt    time.Time     `json:"created_at"`
}

// ContextGrant authorizes one recipient to read one reference and owns its own
// presentation and lifecycle (FS-15.R5).
type ContextGrant struct {
	GrantID     string     `json:"grant_id"`
	RefID       string     `json:"context_ref_id"`
	GrantedBy   string     `json:"granted_by_agent_id"`
	GrantedTo   string     `json:"granted_to_agent_id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	Hidden      bool       `json:"hidden"`
	Source      ContextSource
}

func migrateContextLinks(tx *sql.Tx) error {
	if _, err := tx.Exec(`
CREATE TABLE context_references (
  context_ref_id      TEXT PRIMARY KEY,
  source_kind         TEXT NOT NULL,
  source_agent_id     TEXT NOT NULL DEFAULT '',
  first_seq           INTEGER,
  last_seq            INTEGER,
  pipeline_attempt_id TEXT NOT NULL DEFAULT '',
  created_at          TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_context_ref_transcript
  ON context_references(source_agent_id, first_seq, last_seq)
  WHERE source_kind = 'transcript_span';
CREATE UNIQUE INDEX idx_context_ref_pipeline_report
  ON context_references(pipeline_attempt_id)
  WHERE source_kind = 'pipeline_attempt_report';

CREATE TABLE context_grants (
  grant_id             TEXT PRIMARY KEY,
  context_ref_id       TEXT NOT NULL REFERENCES context_references(context_ref_id),
  granted_by_agent_id  TEXT NOT NULL,
  granted_to_agent_id  TEXT NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
  label                TEXT NOT NULL DEFAULT '',
  description          TEXT NOT NULL DEFAULT '',
  created_at           TEXT NOT NULL,
  updated_at           TEXT NOT NULL,
  revoked_at           TEXT,
  UNIQUE(context_ref_id, granted_by_agent_id, granted_to_agent_id)
);
CREATE INDEX idx_context_grants_recipient
  ON context_grants(granted_to_agent_id, revoked_at, updated_at DESC, grant_id);

CREATE TABLE context_grant_preferences (
  grant_id    TEXT PRIMARY KEY REFERENCES context_grants(grant_id) ON DELETE CASCADE,
  hidden_at   TEXT
);
`); err != nil {
		return fmt.Errorf("state: create context links: %w", err)
	}
	return nil
}

// CanonicalizeContextReferenceTx returns the one reference id for a locator,
// creating it only when the locator has never been referenced. The partial
// unique indexes are the concurrency guard: a lost INSERT race re-reads the
// winner's row instead of minting a second id for the same source (INV §5).
func CanonicalizeContextReferenceTx(tx *sql.Tx, source ContextSource, now time.Time) (ContextReference, error) {
	if err := source.Validate(); err != nil {
		return ContextReference{}, err
	}
	id, err := newContextID("cr")
	if err != nil {
		return ContextReference{}, err
	}
	var firstSeq, lastSeq any
	if source.Kind == ContextSourceTranscriptSpan {
		firstSeq, lastSeq = source.FirstSeq, source.LastSeq
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO context_references(
  context_ref_id, source_kind, source_agent_id, first_seq, last_seq, pipeline_attempt_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, source.Kind, source.AgentID, firstSeq, lastSeq, source.PipelineAttemptID,
		formatTime(now)); err != nil {
		return ContextReference{}, fmt.Errorf("state: canonicalize context reference: %w", err)
	}
	return readContextReferenceBySourceTx(tx, source)
}

func readContextReferenceBySourceTx(tx *sql.Tx, source ContextSource) (ContextReference, error) {
	var row *sql.Row
	switch source.Kind {
	case ContextSourceTranscriptSpan:
		row = tx.QueryRow(`
SELECT `+contextRefColumns+` FROM context_references
WHERE source_kind = ? AND source_agent_id = ? AND first_seq = ? AND last_seq = ?`,
			source.Kind, source.AgentID, source.FirstSeq, source.LastSeq)
	default:
		row = tx.QueryRow(`
SELECT `+contextRefColumns+` FROM context_references
WHERE source_kind = ? AND pipeline_attempt_id = ?`, source.Kind, source.PipelineAttemptID)
	}
	return scanContextReference(row)
}

const contextRefColumns = `context_ref_id, source_kind, source_agent_id,
       COALESCE(first_seq, 0), COALESCE(last_seq, 0), pipeline_attempt_id, created_at`

func scanContextReference(row rowScanner) (ContextReference, error) {
	var ref ContextReference
	var createdAt string
	err := row.Scan(&ref.ContextRefID, &ref.Source.Kind, &ref.Source.AgentID,
		&ref.Source.FirstSeq, &ref.Source.LastSeq, &ref.Source.PipelineAttemptID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ContextReference{}, ErrNotFound
	}
	if err != nil {
		return ContextReference{}, fmt.Errorf("state: scan context reference: %w", err)
	}
	if ref.CreatedAt, err = parseTime(createdAt); err != nil {
		return ContextReference{}, wrapTimeErr("context_reference.created_at", err)
	}
	return ref, nil
}

// ReadContextReference returns one canonical reference by id.
func (s *Store) ReadContextReference(refID string) (ContextReference, error) {
	return scanContextReference(s.db.QueryRow(
		`SELECT `+contextRefColumns+` FROM context_references WHERE context_ref_id = ?`, refID))
}

// ShareContext canonicalizes a locator and creates or refreshes the one grant
// for its reference/grantor/recipient triple, in a single transaction so a
// partially applied share can never leave a reference without its grant
// (INV §5/§15). Re-sharing clears revocation and the recipient's hidden
// preference and replaces that grant's presentation (FS-15.R8, TS-02.R24).
func (s *Store) ShareContext(source ContextSource, grantedBy, grantedTo, label, description string) (ContextReference, ContextGrant, error) {
	if err := source.Validate(); err != nil {
		return ContextReference{}, ContextGrant{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return ContextReference{}, ContextGrant{}, fmt.Errorf("state: begin share context: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	ref, err := CanonicalizeContextReferenceTx(tx, source, now)
	if err != nil {
		return ContextReference{}, ContextGrant{}, err
	}
	grantID, err := newContextID("cg")
	if err != nil {
		return ContextReference{}, ContextGrant{}, err
	}
	if _, err := tx.Exec(`
INSERT INTO context_grants(grant_id, context_ref_id, granted_by_agent_id, granted_to_agent_id,
                           label, description, created_at, updated_at, revoked_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)
ON CONFLICT(context_ref_id, granted_by_agent_id, granted_to_agent_id) DO UPDATE SET
  label = excluded.label,
  description = excluded.description,
  updated_at = excluded.updated_at,
  revoked_at = NULL`,
		grantID, ref.ContextRefID, grantedBy, grantedTo, label, description,
		formatTime(now), formatTime(now)); err != nil {
		return ContextReference{}, ContextGrant{}, fmt.Errorf("state: share context: %w", err)
	}
	grant, err := readContextGrantTx(tx, ref.ContextRefID, grantedBy, grantedTo)
	if err != nil {
		return ContextReference{}, ContextGrant{}, err
	}
	// A re-share is an intentional new offer, so it also returns the grant to the
	// recipient's normal list; hiding is personal list state, not a standing
	// refusal of future shares (FS-15.R7).
	if _, err := tx.Exec(`DELETE FROM context_grant_preferences WHERE grant_id = ?`, grant.GrantID); err != nil {
		return ContextReference{}, ContextGrant{}, fmt.Errorf("state: clear context grant preference: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ContextReference{}, ContextGrant{}, fmt.Errorf("state: commit share context: %w", err)
	}
	grant.Hidden = false
	grant.Source = ref.Source
	return ref, grant, nil
}

const contextGrantColumns = `g.grant_id, g.context_ref_id, g.granted_by_agent_id, g.granted_to_agent_id,
       g.label, g.description, g.created_at, g.updated_at, g.revoked_at,
       CASE WHEN p.hidden_at IS NOT NULL THEN 1 ELSE 0 END,
       r.source_kind, r.source_agent_id, COALESCE(r.first_seq, 0), COALESCE(r.last_seq, 0),
       r.pipeline_attempt_id`

const contextGrantJoin = `FROM context_grants g
JOIN context_references r ON r.context_ref_id = g.context_ref_id
LEFT JOIN context_grant_preferences p ON p.grant_id = g.grant_id`

func readContextGrantTx(tx *sql.Tx, refID, grantedBy, grantedTo string) (ContextGrant, error) {
	return scanContextGrant(tx.QueryRow(`SELECT `+contextGrantColumns+` `+contextGrantJoin+`
WHERE g.context_ref_id = ? AND g.granted_by_agent_id = ? AND g.granted_to_agent_id = ?`,
		refID, grantedBy, grantedTo))
}

// ReadContextGrant returns one grant by id, active or revoked.
func (s *Store) ReadContextGrant(grantID string) (ContextGrant, error) {
	return scanContextGrant(s.db.QueryRow(
		`SELECT `+contextGrantColumns+` `+contextGrantJoin+` WHERE g.grant_id = ?`, grantID))
}

func scanContextGrant(row rowScanner) (ContextGrant, error) {
	var g ContextGrant
	var createdAt, updatedAt string
	var revokedAt sql.NullString
	var hidden int
	err := row.Scan(&g.GrantID, &g.RefID, &g.GrantedBy, &g.GrantedTo, &g.Label, &g.Description,
		&createdAt, &updatedAt, &revokedAt, &hidden,
		&g.Source.Kind, &g.Source.AgentID, &g.Source.FirstSeq, &g.Source.LastSeq, &g.Source.PipelineAttemptID)
	if errors.Is(err, sql.ErrNoRows) {
		return ContextGrant{}, ErrNotFound
	}
	if err != nil {
		return ContextGrant{}, fmt.Errorf("state: scan context grant: %w", err)
	}
	if g.CreatedAt, err = parseTime(createdAt); err != nil {
		return ContextGrant{}, wrapTimeErr("context_grant.created_at", err)
	}
	if g.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return ContextGrant{}, wrapTimeErr("context_grant.updated_at", err)
	}
	if g.RevokedAt, err = parseOptionalTime(revokedAt); err != nil {
		return ContextGrant{}, wrapTimeErr("context_grant.revoked_at", err)
	}
	g.Hidden = hidden == 1
	return g, nil
}

// ContextGrantsPage is one bounded page of a recipient's ad-hoc direct shares.
type ContextGrantsPage struct {
	Grants     []ContextGrant
	NextCursor string
}

// ListContextGrantsForRecipient returns the recipient's active grants, newest
// first, bounded by limit. after is the opaque keyset position produced by the
// previous page (updated_at + grant_id), so traversal stays stable while newer
// grants arrive. A zero after time starts at the newest grant.
func (s *Store) ListContextGrantsForRecipient(recipient string, includeHidden bool, limit int, after time.Time, afterGrantID string) ([]ContextGrant, error) {
	afterUpdatedAt := ""
	if !after.IsZero() {
		afterUpdatedAt = formatTime(after)
	}
	rows, err := s.db.Query(`
SELECT `+contextGrantColumns+` `+contextGrantJoin+`
WHERE g.granted_to_agent_id = ?
  AND g.revoked_at IS NULL
  AND (? = 1 OR p.hidden_at IS NULL)
  AND (? = '' OR g.updated_at < ? OR (g.updated_at = ? AND g.grant_id < ?))
ORDER BY g.updated_at DESC, g.grant_id DESC
LIMIT ?`, recipient, boolInt(includeHidden), afterUpdatedAt, afterUpdatedAt, afterUpdatedAt, afterGrantID, limit)
	if err != nil {
		return nil, fmt.Errorf("state: list context grants: %w", err)
	}
	defer rows.Close()
	out := []ContextGrant{}
	for rows.Next() {
		g, err := scanContextGrant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate context grants: %w", err)
	}
	return out, nil
}

// ContextRecipients is the context-specific durable chat-agent directory
// (TS-01.R22, FS-15.R17). It deliberately is not AddressableAgents: a grant
// starts no process, so a stopped agent needs no wake gate, a session snapshot
// is irrelevant, and pipeline association does not exclude anyone. What it does
// share is one state snapshot per call and the same LiveAgent projection, so
// state.ResolveRecipient's address matching stays the single spelling (INV §2).
// Archived identities are excluded; ResolveRecipient drops terminal agents.
func (s *Store) ContextRecipients() ([]LiveAgent, error) {
	rows, err := s.db.Query(`
SELECT ` + agentColumns + `,
       CASE WHEN r.agent_id IS NOT NULL THEN '` + AvailabilityRunning + `'
            ELSE '` + AvailabilityStopped + `' END
FROM agents a
LEFT JOIN running r ON r.agent_id = a.agent_id
LEFT JOIN status st ON st.agent_id = a.agent_id
WHERE a.archived = 0 AND a.interface = 'chat'
ORDER BY a.name`)
	if err != nil {
		return nil, fmt.Errorf("state: list context recipients: %w", err)
	}
	defer rows.Close()
	out := []LiveAgent{}
	for rows.Next() {
		var la LiveAgent
		if err := rows.Scan(&la.AgentID, &la.Name, &la.Role, &la.Project, &la.Interface,
			&la.State, &la.Detail, &la.ContextPct, &la.Availability); err != nil {
			return nil, fmt.Errorf("state: scan context recipient: %w", err)
		}
		la.Address = address(la.Role, la.Project)
		out = append(out, la)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate context recipients: %w", err)
	}
	return out, nil
}

// ContextReadAuthorized reports whether reader has an effective authorization
// path to refID. Today the only path is an active direct grant; a future work
// domain adds its own participant path without changing reference identity
// (TS-01.R23). It is re-evaluated for every page (TS-05.R16).
func (s *Store) ContextReadAuthorized(refID, reader string) (bool, error) {
	var one int
	err := s.db.QueryRow(`
SELECT 1 FROM context_grants
WHERE context_ref_id = ? AND granted_to_agent_id = ? AND revoked_at IS NULL
LIMIT 1`, refID, reader).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("state: check context authorization: %w", err)
	}
	return true, nil
}

// SetContextGrantHidden changes only the recipient's personal list projection.
// It is recipient-scoped: a caller that is not the recipient changes nothing and
// reports false, which the tool surfaces as the shared not-found outcome.
func (s *Store) SetContextGrantHidden(grantID, recipient string, hidden bool) (bool, error) {
	var owned int
	err := s.db.QueryRow(`
SELECT 1 FROM context_grants
WHERE grant_id = ? AND granted_to_agent_id = ? AND revoked_at IS NULL`, grantID, recipient).Scan(&owned)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("state: read context grant for visibility: %w", err)
	}
	if hidden {
		if _, err := s.db.Exec(`
INSERT INTO context_grant_preferences(grant_id, hidden_at) VALUES (?, ?)
ON CONFLICT(grant_id) DO UPDATE SET hidden_at = excluded.hidden_at`,
			grantID, formatTime(time.Now().UTC())); err != nil {
			return false, fmt.Errorf("state: hide context grant: %w", err)
		}
		return true, nil
	}
	if _, err := s.db.Exec(`DELETE FROM context_grant_preferences WHERE grant_id = ?`, grantID); err != nil {
		return false, fmt.Errorf("state: unhide context grant: %w", err)
	}
	return true, nil
}

// RevokeContextGrant withdraws authorization. It is grantor-scoped and changes
// only that grant row: the canonical reference, its source, and every other
// grant are untouched (FS-15.R8/R14).
func (s *Store) RevokeContextGrant(grantID, grantor string) (bool, error) {
	res, err := s.db.Exec(`
UPDATE context_grants SET revoked_at = ?, updated_at = ?
WHERE grant_id = ? AND granted_by_agent_id = ? AND revoked_at IS NULL`,
		formatTime(time.Now().UTC()), formatTime(time.Now().UTC()), grantID, grantor)
	if err != nil {
		return false, fmt.Errorf("state: revoke context grant: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func newContextID(prefix string) (string, error) {
	var b [12]byte
	if _, err := randRead(b[:]); err != nil {
		return "", fmt.Errorf("state: read context id random: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(b[:]), nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
