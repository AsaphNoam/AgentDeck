package state

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Think Tank room state (FS-21, TS-14). The store is the sole authority for
// rooms, membership, attempts, published entries, queued input and retained
// activity. Room history deliberately has no agent/project foreign keys: it
// outlives both, and deleting a room removes only room-owned rows (TS-14.R12).

const (
	ThinkTankPhaseSetup      = "setup"
	ThinkTankPhaseOpenings   = "openings"
	ThinkTankPhaseDiscussion = "discussion"
	ThinkTankPhaseClosing    = "closing"
	ThinkTankPhaseEnded      = "ended"

	ThinkTankRunning        = "running"
	ThinkTankPauseRequested = "pause_requested"
	ThinkTankPaused         = "paused"
	ThinkTankEndRequested   = "end_requested"

	ThinkTankRoleParticipant = "participant"
	ThinkTankRoleJudge       = "judge"

	ThinkTankMemberActive    = "active"
	ThinkTankMemberDeparted  = "departed"
	ThinkTankMemberExhausted = "exhausted"

	ThinkTankSetupReady     = "ready"
	ThinkTankSetupPending   = "pending"
	ThinkTankSetupLaunching = "launching"
	ThinkTankSetupAbandoned = "abandoned"
	ThinkTankSetupFailed    = "failed"

	ThinkTankTurnOpening    = "opening"
	ThinkTankTurnDiscussion = "discussion"
	ThinkTankTurnClosing    = "closing"
	ThinkTankTurnJudge      = "judge"

	ThinkTankAttemptRunning   = "running"
	ThinkTankAttemptWithheld  = "withheld"
	ThinkTankAttemptFinalized = "finalized"
	ThinkTankAttemptFailed    = "failed"
	// ThinkTankAttemptRetried marks a failed opening the operator explicitly
	// retried; until then the member is not readmitted (TS-14.R23).
	ThinkTankAttemptRetried = "retried"

	ThinkTankReply          = "reply"
	ThinkTankLeave          = "leave"
	ThinkTankDeclineClosing = "decline_closing"

	ThinkTankEntryOpening        = "opening"
	ThinkTankEntryReply          = "reply"
	ThinkTankEntryDeparture      = "departure"
	ThinkTankEntryClosing        = "closing"
	ThinkTankEntryUser           = "user"
	ThinkTankEntryAnnotation     = "annotation"
	ThinkTankEntrySynthesis      = "synthesis"
	ThinkTankEntryMissingOpening = "missing_opening"
	// ThinkTankEntryStageContext is a pipeline room's attributed stage
	// context, published first so every participant and the judge read it
	// through the ordinary paged context read (TS-14.R19).
	ThinkTankEntryStageContext = "stage_context"

	ThinkTankEndOperator           = "operator"
	ThinkTankEndParticipantsLeft   = "participants_left"
	ThinkTankEndAllowanceExhausted = "allowance_exhausted"
	ThinkTankEndLeftAndExhausted   = "left_and_exhausted"

	ThinkTankJudgeNone      = ""
	ThinkTankJudgeWaiting   = "waiting"
	ThinkTankJudgeReady     = "ready"
	ThinkTankJudgeLaunching = "launching"
	ThinkTankJudgeStarting  = "starting"
	ThinkTankJudgeRunning   = "running"
	ThinkTankJudgeFailed    = "failed"
	ThinkTankJudgeCompleted = "completed"
)

// Initial bounds (TS-14.R17).
const (
	ThinkTankMaxGoalRunes     = 8000
	ThinkTankMaxTitleRunes    = 120
	ThinkTankMinParticipants  = 2
	ThinkTankMaxParticipants  = 32
	ThinkTankMaxTurnLimit     = 1000
	ThinkTankMaxTextBytes     = 64 << 10
	ThinkTankPageBytes        = 32 << 10
	ThinkTankMaxActivityBytes = 8 << 20
	ThinkTankMaxAttemptBytes  = 64 << 20
)

var (
	// ErrThinkTankInvalid is a refused request shape; the wrapped text names why.
	ErrThinkTankInvalid = errors.New("state: invalid think tank request")
	// ErrThinkTankConflict is a state-dependent refusal: stale revision, active
	// attempt, wrong phase, or a conflicting replay.
	ErrThinkTankConflict = errors.New("state: think tank conflict")
	// ErrThinkTankStale is an attempt token, generation or turn that no longer
	// owns the room's current attempt.
	ErrThinkTankStale = errors.New("state: think tank attempt is not current")
	// ErrThinkTankNotMember refuses an agent room action from a nonmember.
	ErrThinkTankNotMember = errors.New("state: not a think tank member")

	// Specific refusals the agent tools classify (FS-17.R21). Each also
	// matches its general class above.
	ErrThinkTankReadIncomplete = fmt.Errorf("%w: read the full new conversation with read_think_tank and pass its read_receipt", ErrThinkTankConflict)
	ErrThinkTankReplyConflict  = fmt.Errorf("%w: this turn already staged a different submission", ErrThinkTankConflict)
	ErrThinkTankLeaveForbidden = fmt.Errorf("%w: you are not permitted to leave this room", ErrThinkTankConflict)
	ErrThinkTankClosingOnly    = fmt.Errorf("%w: decline_closing is only available on the closing turn", ErrThinkTankInvalid)
	ErrThinkTankCursor         = fmt.Errorf("%w: cursor is not valid for this read", ErrThinkTankInvalid)
	ErrThinkTankNoTurn         = fmt.Errorf("%w: you have no active Think Tank turn; pass room_id to read a room you belong to", ErrThinkTankConflict)
)

func thinkTankInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrThinkTankInvalid, fmt.Sprintf(format, args...))
}

func thinkTankConflict(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrThinkTankConflict, fmt.Sprintf(format, args...))
}

type ThinkTank struct {
	RoomID    string
	CommandID string
	// Title is the short fixed room name; legacy rows carry a goal-derived
	// fallback (FS-21.R43, TS-14.R22).
	Title         string
	Goal          string
	OriginProject string
	Openings      bool
	Phase         string
	Control       string
	// Hold is a durable intervention reason; non-empty stops progression until
	// an explicit resume or retry (FS-21.R19, R37).
	Hold      string
	EndReason string
	Rotation  int
	Revision  int64
	// JudgeConfig is the opaque ordinary launch request for the optional fresh
	// judge; empty disables synthesis (FS-21.R31, R36).
	JudgeConfig  string
	JudgeStatus  string
	JudgeAgentID string
	JudgeError   string
	// Pipeline origin is trusted host data set only by the stage transaction;
	// standalone rooms leave it empty (TS-14.R19).
	PipelineRunID   string
	PipelineStageID string
	PipelineTaskID  string
	// StageContext is the room-owned encoded ThinkTankStageContext.
	StageContext string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	EndedAt      *time.Time
}

// ThinkTankStageContext is the attributed stage data every participant and the
// judge read before contributing. It is durable room data, so run deletion does
// not erase what participants received (TS-14.R19).
type ThinkTankStageContext struct {
	RunID      string                `json:"run_id"`
	RunName    string                `json:"run_name"`
	StageID    string                `json:"stage_id"`
	StageTitle string                `json:"stage_title"`
	Goal       string                `json:"goal"`
	Objective  string                `json:"objective"`
	Inputs     []ThinkTankStageInput `json:"inputs"`
	Output     ThinkTankStageOutput  `json:"output"`
}

type ThinkTankStageInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ThinkTankStageOutput is the judge's synthesis contract: the named output and
// its limit, enforced before staging (TS-14.R21).
type ThinkTankStageOutput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MaxRunes    int    `json:"max_runes"`
}

// ThinkTankMaxStageContextBytes bounds encoded stage context (TS-09.R56).
const ThinkTankMaxStageContextBytes = 256 << 10

// EncodeThinkTankStageContext refuses oversized context instead of truncating
// required data (TS-09.R56).
func EncodeThinkTankStageContext(c ThinkTankStageContext) (string, error) {
	if c.Inputs == nil {
		c.Inputs = []ThinkTankStageInput{}
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	if len(raw) > ThinkTankMaxStageContextBytes {
		return "", thinkTankInvalid("stage context exceeds %d bytes", ThinkTankMaxStageContextBytes)
	}
	return string(raw), nil
}

type ThinkTankMember struct {
	RoomID     string
	AgentID    string
	AgentName  string
	Project    string
	Role       string
	Order      int
	Cap        int
	Completed  int
	MayLeave   bool
	State      string
	Checkpoint int64
	// Setup records a reserved new agent's launch intent before its effects
	// (TS-14.R2). Existing agents join ready.
	SetupState  string
	SetupConfig string
	SetupError  string
}

type ThinkTankAttempt struct {
	AttemptID   string
	RoomID      string
	AgentID     string
	Turn        string
	Token       string
	Generation  string
	TurnID      string
	Head        int64
	Checkpoint  int64
	DeliveredTo int64
	DeliveredAt int
	State       string
	Disposition string
	Message     string
	EntrySeq    int64
	Failure     string
	CreatedAt   time.Time
	FinishedAt  *time.Time
}

type ThinkTankEntry struct {
	RoomID      string
	Seq         int64
	Kind        string
	AgentID     string
	AgentName   string
	Project     string
	Body        string
	AttemptID   string
	InputID     string
	Context     string
	Undiscussed bool
	CreatedAt   time.Time
}

type ThinkTankInput struct {
	InputID   string
	RoomID    string
	CommandID string
	Kind      string
	Body      string
	Context   string
	EntrySeq  int64
	CreatedAt time.Time
}

// ThinkTankDetail is one consistent read of a room's control state.
type ThinkTankDetail struct {
	Room    ThinkTank
	Members []ThinkTankMember
	// Active is a running attempt (non-nil whenever any runs); Running lists
	// every one, several only during independent openings (TS-14.R23).
	Active   *ThinkTankAttempt
	Running  []ThinkTankAttempt
	Pending  []ThinkTankInput
	Attempts []ThinkTankAttempt
}

// ThinkTankOpportunity is the one turn a room may admit next.
type ThinkTankOpportunity struct {
	AgentID string
	Turn    string
}

func migrateThinkTanks(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE think_tanks (
  room_id        TEXT PRIMARY KEY,
  command_id     TEXT NOT NULL UNIQUE,
  goal           TEXT NOT NULL,
  origin_project TEXT NOT NULL,
  openings       INTEGER NOT NULL,
  phase          TEXT NOT NULL,
  control        TEXT NOT NULL,
  hold           TEXT NOT NULL DEFAULT '',
  end_reason     TEXT NOT NULL DEFAULT '',
  rotation       INTEGER NOT NULL DEFAULT 0,
  revision       INTEGER NOT NULL DEFAULT 1,
  judge_config   TEXT NOT NULL DEFAULT '',
  judge_status   TEXT NOT NULL DEFAULT '',
  judge_agent_id TEXT NOT NULL DEFAULT '',
  judge_error    TEXT NOT NULL DEFAULT '',
  created_at     TEXT NOT NULL,
  updated_at     TEXT NOT NULL,
  ended_at       TEXT
);
CREATE INDEX idx_think_tanks_project ON think_tanks(origin_project, created_at);
CREATE TABLE think_tank_members (
  room_id      TEXT NOT NULL REFERENCES think_tanks(room_id) ON DELETE CASCADE,
  agent_id     TEXT NOT NULL,
  agent_name   TEXT NOT NULL,
  project      TEXT NOT NULL,
  role         TEXT NOT NULL,
  ord          INTEGER NOT NULL,
  cap          INTEGER NOT NULL,
  completed    INTEGER NOT NULL DEFAULT 0,
  may_leave    INTEGER NOT NULL,
  state        TEXT NOT NULL,
  checkpoint   INTEGER NOT NULL DEFAULT 0,
  setup_state  TEXT NOT NULL,
  setup_config TEXT NOT NULL DEFAULT '',
  setup_error  TEXT NOT NULL DEFAULT '',
  PRIMARY KEY(room_id, agent_id)
);
CREATE TABLE think_tank_attempts (
  attempt_id   TEXT PRIMARY KEY,
  room_id      TEXT NOT NULL REFERENCES think_tanks(room_id) ON DELETE CASCADE,
  agent_id     TEXT NOT NULL,
  turn         TEXT NOT NULL,
  token        TEXT NOT NULL UNIQUE,
  generation   TEXT NOT NULL,
  turn_id      TEXT NOT NULL,
  head         INTEGER NOT NULL,
  checkpoint   INTEGER NOT NULL,
  delivered_to INTEGER NOT NULL,
  delivered_at INTEGER NOT NULL DEFAULT 0,
  state        TEXT NOT NULL,
  disposition  TEXT NOT NULL DEFAULT '',
  message      TEXT NOT NULL DEFAULT '',
  entry_seq    INTEGER NOT NULL DEFAULT 0,
  failure      TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL,
  finished_at  TEXT
);
CREATE UNIQUE INDEX idx_think_tank_one_running ON think_tank_attempts(room_id) WHERE state = 'running';
CREATE INDEX idx_think_tank_attempts_agent ON think_tank_attempts(agent_id, state);
CREATE TABLE think_tank_entries (
  room_id     TEXT NOT NULL REFERENCES think_tanks(room_id) ON DELETE CASCADE,
  seq         INTEGER NOT NULL,
  kind        TEXT NOT NULL,
  agent_id    TEXT NOT NULL DEFAULT '',
  agent_name  TEXT NOT NULL DEFAULT '',
  project     TEXT NOT NULL DEFAULT '',
  body        TEXT NOT NULL,
  attempt_id  TEXT NOT NULL DEFAULT '',
  input_id    TEXT NOT NULL DEFAULT '',
  context     TEXT NOT NULL DEFAULT '',
  undiscussed INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL,
  PRIMARY KEY(room_id, seq)
);
CREATE TABLE think_tank_inputs (
  input_id   TEXT PRIMARY KEY,
  room_id    TEXT NOT NULL REFERENCES think_tanks(room_id) ON DELETE CASCADE,
  command_id TEXT NOT NULL,
  kind       TEXT NOT NULL,
  body       TEXT NOT NULL,
  context    TEXT NOT NULL DEFAULT '',
  entry_seq  INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  UNIQUE(room_id, command_id)
);
CREATE TABLE think_tank_activity (
  room_id    TEXT NOT NULL REFERENCES think_tanks(room_id) ON DELETE CASCADE,
  seq        INTEGER NOT NULL,
  attempt_id TEXT NOT NULL,
  agent_id   TEXT NOT NULL,
  agent_name TEXT NOT NULL,
  project    TEXT NOT NULL,
  cwd        TEXT NOT NULL DEFAULT '',
  generation TEXT NOT NULL,
  turn_id    TEXT NOT NULL,
  source_seq INTEGER NOT NULL,
  payload    TEXT NOT NULL,
  truncated  INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  PRIMARY KEY(room_id, seq)
);
CREATE INDEX idx_think_tank_activity_attempt ON think_tank_activity(attempt_id, seq);
`)
	if err != nil {
		return fmt.Errorf("state: create think tanks: %w", err)
	}
	return nil
}

func newThinkTankID(prefix string, n int) (string, error) {
	b := make([]byte, n)
	if _, err := randRead(b); err != nil {
		return "", fmt.Errorf("state: read think tank random: %w", err)
	}
	return prefix + hex.EncodeToString(b), nil
}

// ThinkTankCreate is the validated setup intent. Members carry reserved
// identities: SetupConfig non-empty marks a new agent whose launch is still
// owed; empty marks an existing agent that joins ready.
type ThinkTankCreate struct {
	CommandID string
	// Title is the caller's explicit title; empty selects the goal-derived
	// fallback at insert, so legacy replay intents keep matching.
	Title         string `json:",omitempty"`
	Goal          string
	OriginProject string
	Openings      bool
	JudgeConfig   string
	Members       []ThinkTankMember
	// Pipeline origin and stage context; only the stage transaction sets them
	// (TS-14.R19). Omitted from standalone intents so their replay is unchanged.
	PipelineRunID   string `json:",omitempty"`
	PipelineStageID string `json:",omitempty"`
	PipelineTaskID  string `json:",omitempty"`
	StageContext    string `json:",omitempty"`
}

func validateThinkTankCreate(c ThinkTankCreate) error {
	if strings.TrimSpace(c.CommandID) == "" {
		return thinkTankInvalid("command id is required")
	}
	if strings.TrimSpace(c.Goal) == "" {
		return thinkTankInvalid("goal is required")
	}
	if utf8.RuneCountInString(c.Goal) > ThinkTankMaxGoalRunes {
		return thinkTankInvalid("goal exceeds %d characters", ThinkTankMaxGoalRunes)
	}
	if utf8.RuneCountInString(c.Title) > ThinkTankMaxTitleRunes {
		return thinkTankInvalid("title exceeds %d characters", ThinkTankMaxTitleRunes)
	}
	if strings.TrimSpace(c.OriginProject) == "" {
		return thinkTankInvalid("origin project is required")
	}
	if n := len(c.Members); n < ThinkTankMinParticipants || n > ThinkTankMaxParticipants {
		return thinkTankInvalid("a think tank needs %d to %d participants", ThinkTankMinParticipants, ThinkTankMaxParticipants)
	}
	seen := map[string]bool{}
	for _, m := range c.Members {
		if m.AgentID == "" && m.SetupConfig == "" {
			return thinkTankInvalid("participant agent id is required")
		}
		if m.AgentID != "" && seen[m.AgentID] {
			return thinkTankInvalid("participant %s appears twice", m.AgentID)
		}
		seen[m.AgentID] = true
		if m.Cap < 1 || m.Cap > ThinkTankMaxTurnLimit {
			return thinkTankInvalid("turn limit must be 1 to %d", ThinkTankMaxTurnLimit)
		}
	}
	return nil
}

func sameThinkTankCreate(room ThinkTank, members []ThinkTankMember, c ThinkTankCreate) bool {
	if (c.Title != "" && room.Title != c.Title) || room.Goal != c.Goal || room.OriginProject != c.OriginProject || room.Openings != c.Openings ||
		room.JudgeConfig != c.JudgeConfig || len(members) != len(c.Members) {
		return false
	}
	for i, m := range members {
		w := c.Members[i]
		if (w.SetupConfig == "" && m.AgentID != w.AgentID) || m.Cap != w.Cap || m.MayLeave != w.MayLeave || m.SetupConfig != w.SetupConfig {
			return false
		}
	}
	return true
}

func foldThinkTankWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ThinkTankFallbackTitle derives a readable title from the goal for callers
// and rows without one, without changing the goal (TS-14.R22).
func ThinkTankFallbackTitle(goal string) string {
	r := []rune(foldThinkTankWhitespace(goal))
	if len(r) > ThinkTankMaxTitleRunes {
		r = r[:ThinkTankMaxTitleRunes]
	}
	return strings.TrimSpace(string(r))
}

// migrateThinkTankTitles adds the title column and backfills only that field.
func migrateThinkTankTitles(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE think_tanks ADD COLUMN title TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT room_id, goal FROM think_tanks`)
	if err != nil {
		return err
	}
	titles := map[string]string{}
	for rows.Next() {
		var id, goal string
		if err := rows.Scan(&id, &goal); err != nil {
			rows.Close()
			return err
		}
		titles[id] = ThinkTankFallbackTitle(goal)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for id, title := range titles {
		if _, err := tx.Exec(`UPDATE think_tanks SET title = ? WHERE room_id = ?`, title, id); err != nil {
			return err
		}
	}
	return nil
}

func thinkTankCreateIntent(c ThinkTankCreate) string {
	members := make([]ThinkTankMember, len(c.Members))
	for i, m := range c.Members {
		id := m.AgentID
		if m.SetupConfig != "" {
			id = ""
		}
		members[i] = ThinkTankMember{AgentID: id, Cap: m.Cap, MayLeave: m.MayLeave, SetupConfig: m.SetupConfig}
	}
	c.Members = members
	raw, _ := json.Marshal(c)
	return string(raw)
}

// CreateThinkTank persists setup intent and reserved identities before any
// launch effect (TS-14.R2). An exact replay of CommandID returns the original
// room; a different request under the same id is a conflict.
func (s *Store) CreateThinkTank(c ThinkTankCreate) (ThinkTankDetail, error) {
	// Standalone creation cannot claim a pipeline origin (TS-14.R19).
	c.PipelineRunID, c.PipelineStageID, c.PipelineTaskID, c.StageContext = "", "", "", ""
	tx, err := s.db.Begin()
	if err != nil {
		return ThinkTankDetail{}, fmt.Errorf("state: begin create think tank: %w", err)
	}
	defer tx.Rollback()
	d, err := createThinkTankTx(tx, c)
	if err != nil {
		return ThinkTankDetail{}, err
	}
	if err := tx.Commit(); err != nil {
		return ThinkTankDetail{}, fmt.Errorf("state: commit create think tank: %w", err)
	}
	return d, nil
}

// createThinkTankTx is the one room-creation transaction body shared by
// standalone and pipeline-stage creation (TS-09.R52, INV §2).
func createThinkTankTx(tx *sql.Tx, c ThinkTankCreate) (ThinkTankDetail, error) {
	c.Title = foldThinkTankWhitespace(c.Title)
	if err := validateThinkTankCreate(c); err != nil {
		return ThinkTankDetail{}, err
	}
	intent := thinkTankCreateIntent(c)
	var existing, savedIntent string
	err := tx.QueryRow(`SELECT room_id, create_intent FROM think_tanks WHERE command_id = ?`, c.CommandID).Scan(&existing, &savedIntent)
	if err == nil {
		d, err := readThinkTankDetail(tx, existing)
		if err != nil {
			return ThinkTankDetail{}, err
		}
		if (savedIntent != "" && savedIntent != intent) || (savedIntent == "" && !sameThinkTankCreate(d.Room, participantsOf(d.Members), c)) {
			return ThinkTankDetail{}, thinkTankConflict("command id reused for a different room")
		}
		return d, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ThinkTankDetail{}, fmt.Errorf("state: read think tank command: %w", err)
	}
	c.Members = append([]ThinkTankMember{}, c.Members...)
	reserved := map[string]bool{}
	for _, m := range c.Members {
		reserved[m.AgentID] = true
	}
	for i := range c.Members {
		if c.Members[i].AgentID != "" {
			continue
		}
		for tries := 0; tries < 10; tries++ {
			id, err := newAgentID(tx)
			if err != nil {
				return ThinkTankDetail{}, err
			}
			if !reserved[id] {
				c.Members[i].AgentID, reserved[id] = id, true
				break
			}
		}
		if c.Members[i].AgentID == "" {
			return ThinkTankDetail{}, fmt.Errorf("state: could not reserve unique participant id")
		}
	}
	roomID, err := newThinkTankID("tt_", 8)
	if err != nil {
		return ThinkTankDetail{}, err
	}
	phase := ThinkTankPhaseDiscussion
	if c.Openings {
		phase = ThinkTankPhaseOpenings
	}
	for _, m := range c.Members {
		if m.SetupConfig != "" {
			phase = ThinkTankPhaseSetup
		}
	}
	judgeStatus := ThinkTankJudgeNone
	if c.JudgeConfig != "" {
		judgeStatus = ThinkTankJudgeWaiting
	}
	title := c.Title
	if title == "" {
		title = ThinkTankFallbackTitle(c.Goal)
	}
	now := formatTime(timeNow())
	if _, err := tx.Exec(`
INSERT INTO think_tanks(room_id, command_id, title, goal, origin_project, openings, phase, control,
  judge_config, judge_status, created_at, updated_at, create_intent,
  pipeline_run_id, pipeline_stage_id, pipeline_task_id, stage_context)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		roomID, c.CommandID, title, c.Goal, c.OriginProject, c.Openings, phase, ThinkTankRunning,
		c.JudgeConfig, judgeStatus, now, now, intent,
		c.PipelineRunID, c.PipelineStageID, c.PipelineTaskID, c.StageContext); err != nil {
		return ThinkTankDetail{}, fmt.Errorf("state: insert think tank: %w", err)
	}
	for i, m := range c.Members {
		setup := ThinkTankSetupReady
		if m.SetupConfig != "" {
			setup = ThinkTankSetupPending
		}
		if _, err := tx.Exec(`
INSERT INTO think_tank_members(room_id, agent_id, agent_name, project, role, ord, cap, may_leave,
  state, setup_state, setup_config)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			roomID, m.AgentID, m.AgentName, m.Project, ThinkTankRoleParticipant, i, m.Cap, m.MayLeave,
			ThinkTankMemberActive, setup, m.SetupConfig); err != nil {
			return ThinkTankDetail{}, fmt.Errorf("state: insert think tank member: %w", err)
		}
	}
	if c.StageContext != "" {
		body, err := renderThinkTankStageContext(c.StageContext)
		if err != nil {
			return ThinkTankDetail{}, err
		}
		if _, err := insertThinkTankEntryTx(tx, ThinkTankEntry{RoomID: roomID, Kind: ThinkTankEntryStageContext,
			AgentName: "Pipeline", Project: c.OriginProject, Body: body}); err != nil {
			return ThinkTankDetail{}, err
		}
	}
	return readThinkTankDetail(tx, roomID)
}

// renderThinkTankStageContext is the attributed text every participant and the
// judge read first. The judge's required output and its limit are part of it.
func renderThinkTankStageContext(encoded string) (string, error) {
	var c ThinkTankStageContext
	if err := json.Unmarshal([]byte(encoded), &c); err != nil {
		return "", fmt.Errorf("state: decode stage context: %w", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Pipeline stage context — run %q (%s), stage %q (%s).\n\nRun goal:\n%s\n\nStage objective:\n%s\n",
		c.RunName, c.RunID, c.StageTitle, c.StageID, c.Goal, c.Objective)
	if len(c.Inputs) > 0 {
		b.WriteString("\nStage inputs:\n")
		for _, in := range c.Inputs {
			fmt.Fprintf(&b, "- %s:\n%s\n", in.Name, in.Value)
		}
	}
	fmt.Fprintf(&b, "\nRequired output %q: %s\nThe judge's published synthesis becomes this output exactly; it must be at most %d characters. Completion means a synthesis was produced, not that participants agreed.\n",
		c.Output.Name, c.Output.Description, c.Output.MaxRunes)
	return b.String(), nil
}

func participantsOf(members []ThinkTankMember) []ThinkTankMember {
	out := []ThinkTankMember{}
	for _, m := range members {
		if m.Role == ThinkTankRoleParticipant {
			out = append(out, m)
		}
	}
	return out
}

// queryer is satisfied by *sql.DB and *sql.Tx.
type thinkTankQueryer interface {
	execer
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

const thinkTankColumns = `room_id, command_id, title, goal, origin_project, openings, phase, control, hold,
  end_reason, rotation, revision, judge_config, judge_status, judge_agent_id, judge_error,
  created_at, updated_at, ended_at, pipeline_run_id, pipeline_stage_id, pipeline_task_id, stage_context`

func scanThinkTank(row interface{ Scan(...any) error }) (ThinkTank, error) {
	var r ThinkTank
	var created, updated string
	var ended sql.NullString
	if err := row.Scan(&r.RoomID, &r.CommandID, &r.Title, &r.Goal, &r.OriginProject, &r.Openings, &r.Phase,
		&r.Control, &r.Hold, &r.EndReason, &r.Rotation, &r.Revision, &r.JudgeConfig, &r.JudgeStatus,
		&r.JudgeAgentID, &r.JudgeError, &created, &updated, &ended,
		&r.PipelineRunID, &r.PipelineStageID, &r.PipelineTaskID, &r.StageContext); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ThinkTank{}, ErrNotFound
		}
		return ThinkTank{}, fmt.Errorf("state: scan think tank: %w", err)
	}
	var err error
	if r.CreatedAt, err = parseTime(created); err != nil {
		return ThinkTank{}, wrapTimeErr("think_tank.created_at", err)
	}
	if r.UpdatedAt, err = parseTime(updated); err != nil {
		return ThinkTank{}, wrapTimeErr("think_tank.updated_at", err)
	}
	if r.EndedAt, err = parseOptionalTime(ended); err != nil {
		return ThinkTank{}, wrapTimeErr("think_tank.ended_at", err)
	}
	return r, nil
}

const thinkTankMemberColumns = `room_id, agent_id, agent_name, project, role, ord, cap, completed,
  may_leave, state, checkpoint, setup_state, setup_config, setup_error`

func scanThinkTankMember(row interface{ Scan(...any) error }) (ThinkTankMember, error) {
	var m ThinkTankMember
	if err := row.Scan(&m.RoomID, &m.AgentID, &m.AgentName, &m.Project, &m.Role, &m.Order, &m.Cap,
		&m.Completed, &m.MayLeave, &m.State, &m.Checkpoint, &m.SetupState, &m.SetupConfig,
		&m.SetupError); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ThinkTankMember{}, ErrNotFound
		}
		return ThinkTankMember{}, fmt.Errorf("state: scan think tank member: %w", err)
	}
	return m, nil
}

const thinkTankAttemptColumns = `attempt_id, room_id, agent_id, turn, token, generation, turn_id,
  head, checkpoint, delivered_to, delivered_at, state, disposition, message, entry_seq, failure,
  created_at, finished_at`

func scanThinkTankAttempt(row interface{ Scan(...any) error }) (ThinkTankAttempt, error) {
	var a ThinkTankAttempt
	var created string
	var finished sql.NullString
	if err := row.Scan(&a.AttemptID, &a.RoomID, &a.AgentID, &a.Turn, &a.Token, &a.Generation,
		&a.TurnID, &a.Head, &a.Checkpoint, &a.DeliveredTo, &a.DeliveredAt, &a.State, &a.Disposition,
		&a.Message, &a.EntrySeq, &a.Failure, &created, &finished); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ThinkTankAttempt{}, ErrNotFound
		}
		return ThinkTankAttempt{}, fmt.Errorf("state: scan think tank attempt: %w", err)
	}
	var err error
	if a.CreatedAt, err = parseTime(created); err != nil {
		return ThinkTankAttempt{}, wrapTimeErr("think_tank_attempt.created_at", err)
	}
	if a.FinishedAt, err = parseOptionalTime(finished); err != nil {
		return ThinkTankAttempt{}, wrapTimeErr("think_tank_attempt.finished_at", err)
	}
	return a, nil
}

const thinkTankEntryColumns = `room_id, seq, kind, agent_id, agent_name, project, body, attempt_id,
  input_id, context, undiscussed, created_at`

func scanThinkTankEntry(row interface{ Scan(...any) error }) (ThinkTankEntry, error) {
	var e ThinkTankEntry
	var created string
	if err := row.Scan(&e.RoomID, &e.Seq, &e.Kind, &e.AgentID, &e.AgentName, &e.Project, &e.Body,
		&e.AttemptID, &e.InputID, &e.Context, &e.Undiscussed, &created); err != nil {
		return ThinkTankEntry{}, fmt.Errorf("state: scan think tank entry: %w", err)
	}
	var err error
	if e.CreatedAt, err = parseTime(created); err != nil {
		return ThinkTankEntry{}, wrapTimeErr("think_tank_entry.created_at", err)
	}
	return e, nil
}

func readThinkTankDetail(q thinkTankQueryer, roomID string) (ThinkTankDetail, error) {
	room, err := scanThinkTank(q.QueryRow(`SELECT `+thinkTankColumns+` FROM think_tanks WHERE room_id = ?`, roomID))
	if err != nil {
		return ThinkTankDetail{}, err
	}
	d := ThinkTankDetail{Room: room, Members: []ThinkTankMember{}, Pending: []ThinkTankInput{}, Attempts: []ThinkTankAttempt{}}
	rows, err := q.Query(`SELECT `+thinkTankMemberColumns+` FROM think_tank_members WHERE room_id = ? ORDER BY role DESC, ord`, roomID)
	if err != nil {
		return ThinkTankDetail{}, fmt.Errorf("state: read think tank members: %w", err)
	}
	for rows.Next() {
		m, err := scanThinkTankMember(rows)
		if err != nil {
			rows.Close()
			return ThinkTankDetail{}, err
		}
		d.Members = append(d.Members, m)
	}
	if err := closeRows(rows, "think tank members"); err != nil {
		return ThinkTankDetail{}, err
	}
	// Attempts are bounded by participant ceilings plus explicit retries; the
	// detail keeps unfinished ones and the latest per agent/turn for accounting.
	rows, err = q.Query(`SELECT `+thinkTankAttemptColumns+` FROM think_tank_attempts
WHERE room_id = ? AND (state IN ('running', 'withheld', 'failed') OR turn IN ('opening', 'closing', 'judge'))
ORDER BY created_at, attempt_id`, roomID)
	if err != nil {
		return ThinkTankDetail{}, fmt.Errorf("state: read think tank attempts: %w", err)
	}
	for rows.Next() {
		a, err := scanThinkTankAttempt(rows)
		if err != nil {
			rows.Close()
			return ThinkTankDetail{}, err
		}
		d.Attempts = append(d.Attempts, a)
		if a.State == ThinkTankAttemptRunning {
			d.Running = append(d.Running, a)
			if d.Active == nil {
				active := a
				d.Active = &active
			}
		}
	}
	if err := closeRows(rows, "think tank attempts"); err != nil {
		return ThinkTankDetail{}, err
	}
	rows, err = q.Query(`SELECT input_id, room_id, command_id, kind, body, context, entry_seq, created_at
FROM think_tank_inputs WHERE room_id = ? AND entry_seq = 0 ORDER BY created_at, rowid`, roomID)
	if err != nil {
		return ThinkTankDetail{}, fmt.Errorf("state: read think tank inputs: %w", err)
	}
	for rows.Next() {
		in, err := scanThinkTankInput(rows)
		if err != nil {
			rows.Close()
			return ThinkTankDetail{}, err
		}
		d.Pending = append(d.Pending, in)
	}
	if err := closeRows(rows, "think tank inputs"); err != nil {
		return ThinkTankDetail{}, err
	}
	return d, nil
}

func scanThinkTankInput(row interface{ Scan(...any) error }) (ThinkTankInput, error) {
	var in ThinkTankInput
	var created string
	if err := row.Scan(&in.InputID, &in.RoomID, &in.CommandID, &in.Kind, &in.Body, &in.Context,
		&in.EntrySeq, &created); err != nil {
		return ThinkTankInput{}, fmt.Errorf("state: scan think tank input: %w", err)
	}
	var err error
	if in.CreatedAt, err = parseTime(created); err != nil {
		return ThinkTankInput{}, wrapTimeErr("think_tank_input.created_at", err)
	}
	return in, nil
}

func closeRows(rows *sql.Rows, what string) error {
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("state: iterate %s: %w", what, err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("state: close %s: %w", what, err)
	}
	return nil
}

// ReadThinkTank returns one room's current control state.
func (s *Store) ReadThinkTank(roomID string) (ThinkTankDetail, error) {
	return readThinkTankDetail(s.db, roomID)
}

// ListThinkTanks lists rooms newest first, optionally only those originating
// in one project. The limit is required (INV §16).
func (s *Store) ListThinkTanks(project, agentID string, limit int) ([]ThinkTank, error) {
	rows, err := s.db.Query(`SELECT `+thinkTankColumns+` FROM think_tanks
WHERE (? = '' OR origin_project = ?)
  AND (? = '' OR room_id IN (SELECT room_id FROM think_tank_members WHERE agent_id = ?))
ORDER BY created_at DESC, room_id LIMIT ?`, project, project, agentID, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("state: list think tanks: %w", err)
	}
	out := []ThinkTank{}
	for rows.Next() {
		r, err := scanThinkTank(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, r)
	}
	if err := closeRows(rows, "think tanks"); err != nil {
		return nil, err
	}
	return out, nil
}

// ListActiveThinkTankRooms returns the ids of rooms that may still need
// progression: not ended, or ended with judge work outstanding.
func (s *Store) ListActiveThinkTankRooms() ([]string, error) {
	rows, err := s.db.Query(`SELECT room_id FROM think_tanks
WHERE phase != 'ended' OR judge_status IN ('ready', 'launching', 'starting', 'running') ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("state: list active think tanks: %w", err)
	}
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("state: scan active think tank: %w", err)
		}
		out = append(out, id)
	}
	if err := closeRows(rows, "active think tanks"); err != nil {
		return nil, err
	}
	return out, nil
}

// ListThinkTankEntries returns published entries after afterSeq, in order.
func (s *Store) ListThinkTankEntries(roomID string, afterSeq int64, limit int) ([]ThinkTankEntry, error) {
	return listThinkTankEntries(s.db, roomID, afterSeq, 1<<62, limit)
}

func listThinkTankEntries(q thinkTankQueryer, roomID string, afterSeq, throughSeq int64, limit int) ([]ThinkTankEntry, error) {
	rows, err := q.Query(`SELECT `+thinkTankEntryColumns+` FROM think_tank_entries
WHERE room_id = ? AND seq > ? AND seq <= ? ORDER BY seq LIMIT ?`, roomID, afterSeq, throughSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("state: list think tank entries: %w", err)
	}
	out := []ThinkTankEntry{}
	for rows.Next() {
		e, err := scanThinkTankEntry(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	if err := closeRows(rows, "think tank entries"); err != nil {
		return nil, err
	}
	return out, nil
}

func bumpThinkTankTx(tx *sql.Tx, roomID string) error {
	if _, err := tx.Exec(`UPDATE think_tanks SET revision = revision + 1, updated_at = ? WHERE room_id = ?`,
		formatTime(timeNow()), roomID); err != nil {
		return fmt.Errorf("state: bump think tank: %w", err)
	}
	return nil
}

// ClaimThinkTankMemberSetup fences setup launch effects against Pause/End/Delete.
// Delete refuses the claimed slot until its ordinary launch outcome commits.
func (s *Store) ClaimThinkTankMemberSetup(roomID, agentID string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		if d.Room.Phase != ThinkTankPhaseSetup || d.Room.Control != ThinkTankRunning || d.Room.Hold != "" {
			return thinkTankConflict("room setup is no longer runnable")
		}
		res, err := tx.Exec(`UPDATE think_tank_members SET setup_state = ? WHERE room_id = ? AND agent_id = ? AND role = ? AND setup_state = ?`,
			ThinkTankSetupLaunching, roomID, agentID, ThinkTankRoleParticipant, ThinkTankSetupPending)
		if err != nil {
			return fmt.Errorf("state: claim think tank setup: %w", err)
		}
		if n, err := res.RowsAffected(); err != nil || n != 1 {
			return thinkTankConflict("participant is not pending setup")
		}
		return nil
	})
}

// MarkThinkTankMemberSetup records a reserved new participant's launch
// outcome. When every participant is ready, the room leaves setup. A failed
// slot keeps the room in setup with a hold naming it; retry relaunches only
// that slot, never a ready one (TS-14.R2).
func (s *Store) MarkThinkTankMemberSetup(roomID, agentID, name, launchErr string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		state := ThinkTankSetupReady
		if launchErr != "" {
			state = ThinkTankSetupFailed
		}
		res, err := tx.Exec(`UPDATE think_tank_members SET setup_state = ?, setup_error = ?,
  agent_name = CASE WHEN ? = '' THEN agent_name ELSE ? END
WHERE room_id = ? AND agent_id = ? AND setup_state IN ('pending', 'launching', 'failed')`, state, launchErr, name, name, roomID, agentID)
		if err != nil {
			return fmt.Errorf("state: mark think tank setup: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return thinkTankConflict("participant %s is not awaiting setup", agentID)
		}
		if launchErr != "" {
			return setThinkTankHoldTx(tx, roomID, "Setup failed for a new participant: "+launchErr)
		}
		return nil
	})
}

// SetThinkTankHold records why a room cannot progress (eligibility loss,
// restart, setup failure). An empty reason is ignored; ResumeThinkTank clears.
func (s *Store) SetThinkTankHold(roomID, reason string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		if d.Room.Hold == reason || reason == "" {
			return errNoThinkTankChange
		}
		return setThinkTankHoldTx(tx, roomID, reason)
	})
}

var errNoThinkTankChange = errors.New("no change")

func setThinkTankHoldTx(tx *sql.Tx, roomID, reason string) error {
	if _, err := tx.Exec(`UPDATE think_tanks SET hold = ? WHERE room_id = ?`, reason, roomID); err != nil {
		return fmt.Errorf("state: hold think tank: %w", err)
	}
	return nil
}

// thinkTankTx runs fn over a fresh detail read, then settles phase transitions
// and bumps the revision in the same transaction. fn returning
// errNoThinkTankChange commits nothing and returns the unchanged detail.
func (s *Store) thinkTankTx(roomID string, fn func(*sql.Tx, ThinkTankDetail) error) (ThinkTankDetail, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ThinkTankDetail{}, fmt.Errorf("state: begin think tank: %w", err)
	}
	defer tx.Rollback()
	d, err := readThinkTankDetail(tx, roomID)
	if err != nil {
		return ThinkTankDetail{}, err
	}
	if err := fn(tx, d); errors.Is(err, errNoThinkTankChange) {
		return d, nil
	} else if err != nil {
		return ThinkTankDetail{}, err
	}
	if err := settleThinkTankTx(tx, roomID); err != nil {
		return ThinkTankDetail{}, err
	}
	if err := bumpThinkTankTx(tx, roomID); err != nil {
		return ThinkTankDetail{}, err
	}
	out, err := readThinkTankDetail(tx, roomID)
	if err != nil {
		return ThinkTankDetail{}, err
	}
	if err := tx.Commit(); err != nil {
		return ThinkTankDetail{}, fmt.Errorf("state: commit think tank: %w", err)
	}
	return out, nil
}

// PauseThinkTank requests a pause. An active turn finishes first; the room is
// then paused (FS-21.R18).
func (s *Store) PauseThinkTank(roomID string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		if d.Room.Phase == ThinkTankPhaseEnded {
			return thinkTankConflict("discussion has ended")
		}
		if d.Room.Control != ThinkTankRunning {
			return errNoThinkTankChange
		}
		control := ThinkTankPaused
		if d.Active != nil {
			control = ThinkTankPauseRequested
		}
		return setThinkTankControlTx(tx, roomID, control)
	})
}

// ResumeThinkTank resumes progression and clears an intervention hold. A
// failed attempt is not replayed: the next opportunity is a new attempt with a
// new token (TS-14.R14).
func (s *Store) ResumeThinkTank(roomID string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		if d.Room.Control == ThinkTankEndRequested {
			return thinkTankConflict("discussion is ending")
		}
		if d.Room.Control == ThinkTankRunning && d.Room.Hold == "" {
			return errNoThinkTankChange
		}
		if err := setThinkTankControlTx(tx, roomID, ThinkTankRunning); err != nil {
			return err
		}
		return setThinkTankHoldTx(tx, roomID, unretriedOpeningHold(d))
	})
}

// unretriedOpeningHold keeps a room held while a failed opening awaits
// explicit retry; Resume alone never readmits it (TS-14.R23).
func unretriedOpeningHold(d ThinkTankDetail) string {
	failed := unretriedThinkTankOpenings(d)
	if len(failed) == 0 {
		return ""
	}
	return fmt.Sprintf("%d opening(s) failed. Retry them, or end the discussion to publish the completed openings.", len(failed))
}

// RetryThinkTankOpening authorizes one failed opening's next attempt. With
// no attempt id, exactly one unretried failure must exist; otherwise the
// retry is ambiguous. Retrying an already retried attempt is an exact replay.
// A command id binds to the attempt it retried: its replay is exact even after
// later failures or phase changes, and reuse for another attempt refuses.
// A paused room stays paused; other unresolved failures keep the hold.
// Without failed openings this is the ordinary turn retry (Resume).
func (s *Store) RetryThinkTankOpening(roomID, attemptID, commandID string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		if commandID != "" {
			var bound string
			err := tx.QueryRow(`SELECT attempt_id FROM think_tank_attempts WHERE room_id = ? AND retry_command_id = ?`,
				roomID, commandID).Scan(&bound)
			if err == nil {
				if attemptID != "" && attemptID != bound {
					return thinkTankConflict("command id reused for a different opening")
				}
				return errNoThinkTankChange
			} else if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("state: read think tank retry command: %w", err)
			}
		}
		failed := unretriedThinkTankOpenings(d)
		if attemptID == "" && len(failed) == 0 {
			if d.Room.Control == ThinkTankEndRequested {
				return thinkTankConflict("discussion is ending")
			}
			if d.Room.Control == ThinkTankRunning && d.Room.Hold == "" {
				return errNoThinkTankChange
			}
			if err := setThinkTankControlTx(tx, roomID, ThinkTankRunning); err != nil {
				return err
			}
			return setThinkTankHoldTx(tx, roomID, "")
		}
		if d.Room.Phase != ThinkTankPhaseOpenings || d.Room.Control == ThinkTankEndRequested {
			return thinkTankConflict("openings have closed")
		}
		var target *ThinkTankAttempt
		switch {
		case attemptID != "":
			for i := range d.Attempts {
				if d.Attempts[i].AttemptID == attemptID && d.Attempts[i].Turn == ThinkTankTurnOpening {
					target = &d.Attempts[i]
				}
			}
			if target == nil {
				return thinkTankInvalid("no such opening attempt")
			}
			if target.State == ThinkTankAttemptRetried {
				return errNoThinkTankChange
			}
			if target.State != ThinkTankAttemptFailed {
				return thinkTankConflict("that opening did not fail")
			}
		case len(failed) > 1:
			return thinkTankConflict("several openings failed; choose which to retry")
		default:
			target = &failed[0]
		}
		if _, err := tx.Exec(`UPDATE think_tank_attempts SET state = ?, retry_command_id = ? WHERE attempt_id = ?`,
			ThinkTankAttemptRetried, commandID, target.AttemptID); err != nil {
			return fmt.Errorf("state: retry think tank opening: %w", err)
		}
		after, err := readThinkTankDetail(tx, roomID)
		if err != nil {
			return err
		}
		return setThinkTankHoldTx(tx, roomID, unretriedOpeningHold(after))
	})
}

// ThinkTankLimitChange raises one participant's turn ceiling to an absolute
// higher value, guarded by the ceiling the operator saw (TS-14.R24).
type ThinkTankLimitChange struct {
	RoomID    string
	AgentID   string
	CommandID string
	Expected  int
	Limit     int
}

// IncreaseThinkTankTurnLimit raises a participant's ceiling while openings or
// discussion are open. An exhausted member becomes eligible again; departure,
// counters, control, holds and active attempts are untouched. Exact command
// replay returns the current room; conflicting reuse or a stale expected
// ceiling refuses (FS-21.R49).
func (s *Store) IncreaseThinkTankTurnLimit(c ThinkTankLimitChange) (ThinkTankDetail, error) {
	if strings.TrimSpace(c.CommandID) == "" {
		return ThinkTankDetail{}, thinkTankInvalid("command id is required")
	}
	if c.Limit < 1 || c.Limit > ThinkTankMaxTurnLimit {
		return ThinkTankDetail{}, thinkTankInvalid("turn limit must be 1 to %d", ThinkTankMaxTurnLimit)
	}
	if c.Limit <= c.Expected {
		return ThinkTankDetail{}, thinkTankInvalid("the new turn limit must be higher than the current one")
	}
	return s.thinkTankTx(c.RoomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		var agent string
		var expected, limit int
		err := tx.QueryRow(`SELECT agent_id, expected, turn_limit FROM think_tank_limit_commands WHERE room_id = ? AND command_id = ?`,
			c.RoomID, c.CommandID).Scan(&agent, &expected, &limit)
		if err == nil {
			if agent != c.AgentID || expected != c.Expected || limit != c.Limit {
				return thinkTankConflict("command id reused for a different change")
			}
			return errNoThinkTankChange
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("state: read think tank limit command: %w", err)
		}
		if d.Room.Phase != ThinkTankPhaseOpenings && d.Room.Phase != ThinkTankPhaseDiscussion {
			return thinkTankConflict("turn limits can only change while discussion is open")
		}
		if d.Room.Control == ThinkTankEndRequested {
			return thinkTankConflict("discussion is ending")
		}
		var member *ThinkTankMember
		for i := range d.Members {
			if d.Members[i].AgentID == c.AgentID && d.Members[i].Role == ThinkTankRoleParticipant {
				member = &d.Members[i]
			}
		}
		if member == nil {
			return thinkTankInvalid("%s is not a participant", c.AgentID)
		}
		if member.Cap != c.Expected {
			return thinkTankConflict("the turn limit is now %d", member.Cap)
		}
		state := member.State
		if state == ThinkTankMemberExhausted && member.Completed < c.Limit {
			state = ThinkTankMemberActive
		}
		if _, err := tx.Exec(`UPDATE think_tank_members SET cap = ?, state = ? WHERE room_id = ? AND agent_id = ?`,
			c.Limit, state, c.RoomID, c.AgentID); err != nil {
			return fmt.Errorf("state: raise think tank turn limit: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO think_tank_limit_commands(room_id, command_id, agent_id, expected, turn_limit, created_at)
VALUES(?, ?, ?, ?, ?, ?)`, c.RoomID, c.CommandID, c.AgentID, c.Expected, c.Limit, formatTime(timeNow())); err != nil {
			return fmt.Errorf("state: record think tank limit command: %w", err)
		}
		return nil
	})
}

func setThinkTankControlTx(tx *sql.Tx, roomID, control string) error {
	if _, err := tx.Exec(`UPDATE think_tanks SET control = ? WHERE room_id = ?`, control, roomID); err != nil {
		return fmt.Errorf("state: set think tank control: %w", err)
	}
	return nil
}

// EndThinkTank ends discussion at the operator's request. An active turn
// finishes normally first; otherwise discussion ends now without waiting on
// any participant's private work (FS-21.R32).
func (s *Store) EndThinkTank(roomID string) (ThinkTankDetail, error) {
	return s.thinkTankTx(roomID, func(tx *sql.Tx, d ThinkTankDetail) error {
		if d.Room.Phase == ThinkTankPhaseEnded || d.Room.Control == ThinkTankEndRequested {
			return errNoThinkTankChange
		}
		if d.Active != nil {
			return setThinkTankControlTx(tx, roomID, ThinkTankEndRequested)
		}
		return endThinkTankTx(tx, d, ThinkTankEndOperator)
	})
}

// endThinkTankTx ends discussion. Completed withheld openings publish as a
// partial set with explicit missing markers; pending input publishes and every
// user input no completed participant turn followed is marked undiscussed
// (FS-21.R37). The judge becomes ready when configured.
func endThinkTankTx(tx *sql.Tx, d ThinkTankDetail, reason string) error {
	roomID := d.Room.RoomID
	if _, err := tx.Exec(`UPDATE think_tank_members SET setup_state = ? WHERE room_id = ? AND role = ? AND setup_state IN ('pending', 'failed')`,
		ThinkTankSetupAbandoned, roomID, ThinkTankRoleParticipant); err != nil {
		return fmt.Errorf("state: abandon think tank setup: %w", err)
	}
	if d.Room.Phase == ThinkTankPhaseOpenings {
		if err := publishOpeningsTx(tx, d, true); err != nil {
			return err
		}
	}
	if err := publishPendingInputsTx(tx, roomID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
UPDATE think_tank_entries SET undiscussed = 1
WHERE room_id = ? AND kind IN ('user', 'annotation')
  AND seq > COALESCE((SELECT MAX(seq) FROM think_tank_entries
    WHERE room_id = ? AND kind IN ('opening', 'reply', 'departure', 'closing')), 0)`,
		roomID, roomID); err != nil {
		return fmt.Errorf("state: mark undiscussed think tank input: %w", err)
	}
	judge := d.Room.JudgeStatus
	if judge == ThinkTankJudgeWaiting {
		judge = ThinkTankJudgeReady
	}
	endReason := d.Room.EndReason
	if reason == ThinkTankEndOperator || endReason == "" {
		endReason = reason
	}
	now := formatTime(timeNow())
	if _, err := tx.Exec(`
UPDATE think_tanks SET phase = ?, control = ?, hold = '', end_reason = ?, judge_status = ?, ended_at = ?
WHERE room_id = ?`, ThinkTankPhaseEnded, ThinkTankRunning, endReason, judge, now, roomID); err != nil {
		return fmt.Errorf("state: end think tank: %w", err)
	}
	return nil
}

func nextThinkTankSeqTx(tx *sql.Tx, roomID string) (int64, error) {
	var seq int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) + 1 FROM think_tank_entries WHERE room_id = ?`, roomID).Scan(&seq); err != nil {
		return 0, fmt.Errorf("state: next think tank seq: %w", err)
	}
	return seq, nil
}

func insertThinkTankEntryTx(tx *sql.Tx, e ThinkTankEntry) (int64, error) {
	seq, err := nextThinkTankSeqTx(tx, e.RoomID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`
INSERT INTO think_tank_entries(room_id, seq, kind, agent_id, agent_name, project, body, attempt_id,
  input_id, context, created_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.RoomID, seq, e.Kind, e.AgentID, e.AgentName, e.Project, e.Body, e.AttemptID, e.InputID,
		e.Context, formatTime(timeNow())); err != nil {
		return 0, fmt.Errorf("state: insert think tank entry: %w", err)
	}
	return seq, nil
}

func publishPendingInputsTx(tx *sql.Tx, roomID string) error {
	rows, err := tx.Query(`SELECT input_id, room_id, command_id, kind, body, context, entry_seq, created_at
FROM think_tank_inputs WHERE room_id = ? AND entry_seq = 0 ORDER BY created_at, rowid`, roomID)
	if err != nil {
		return fmt.Errorf("state: read pending think tank inputs: %w", err)
	}
	pending := []ThinkTankInput{}
	for rows.Next() {
		in, err := scanThinkTankInput(rows)
		if err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, in)
	}
	if err := closeRows(rows, "pending think tank inputs"); err != nil {
		return err
	}
	for _, in := range pending {
		seq, err := insertThinkTankEntryTx(tx, ThinkTankEntry{RoomID: roomID, Kind: in.Kind, Body: in.Body,
			InputID: in.InputID, Context: in.Context})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE think_tank_inputs SET entry_seq = ? WHERE input_id = ?`, seq, in.InputID); err != nil {
			return fmt.Errorf("state: publish think tank input: %w", err)
		}
	}
	return nil
}

// publishOpeningsTx publishes withheld openings in configured order. When
// partial, members without a completed opening get an explicit missing marker
// rather than invented content (FS-21.R37, TS-14.R8).
func publishOpeningsTx(tx *sql.Tx, d ThinkTankDetail, partial bool) error {
	withheld := map[string]ThinkTankAttempt{}
	for _, a := range d.Attempts {
		if a.Turn == ThinkTankTurnOpening && a.State == ThinkTankAttemptWithheld {
			withheld[a.AgentID] = a
		}
	}
	for _, m := range participantsOf(d.Members) {
		a, ok := withheld[m.AgentID]
		if !ok {
			if partial {
				if _, err := insertThinkTankEntryTx(tx, ThinkTankEntry{RoomID: d.Room.RoomID,
					Kind: ThinkTankEntryMissingOpening, AgentID: m.AgentID, AgentName: m.AgentName,
					Project: m.Project, Body: ""}); err != nil {
					return err
				}
			}
			continue
		}
		seq, err := insertThinkTankEntryTx(tx, ThinkTankEntry{RoomID: d.Room.RoomID,
			Kind: ThinkTankEntryOpening, AgentID: m.AgentID, AgentName: m.AgentName, Project: m.Project,
			Body: a.Message, AttemptID: a.AttemptID})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE think_tank_attempts SET state = ?, entry_seq = ? WHERE attempt_id = ?`,
			ThinkTankAttemptFinalized, seq, a.AttemptID); err != nil {
			return fmt.Errorf("state: finalize withheld opening: %w", err)
		}
	}
	return nil
}

// settleThinkTankTx applies the automatic phase transitions: setup completion,
// the opening barrier, and discussion stopping when fewer than two eligible
// participants remain (FS-21.R20). It never starts a turn.
func settleThinkTankTx(tx *sql.Tx, roomID string) error {
	for i := 0; i < 4; i++ {
		d, err := readThinkTankDetail(tx, roomID)
		if err != nil {
			return err
		}
		next, reason := thinkTankTransition(d)
		if next == "" {
			return nil
		}
		switch next {
		case ThinkTankPhaseEnded:
			if err := endThinkTankTx(tx, d, reason); err != nil {
				return err
			}
		case ThinkTankPhaseDiscussion:
			if d.Room.Phase == ThinkTankPhaseOpenings {
				if err := publishOpeningsTx(tx, d, false); err != nil {
					return err
				}
				if err := publishPendingInputsTx(tx, roomID); err != nil {
					return err
				}
			}
			fallthrough
		default:
			if _, err := tx.Exec(`UPDATE think_tanks SET phase = ?, end_reason = ?,
  hold = CASE WHEN ? THEN '' ELSE hold END WHERE room_id = ?`,
				next, reason, d.Room.Phase == ThinkTankPhaseSetup, roomID); err != nil {
				return fmt.Errorf("state: advance think tank phase: %w", err)
			}
		}
	}
	return errors.New("state: think tank transitions did not settle")
}

func eligibleThinkTankMembers(d ThinkTankDetail) []ThinkTankMember {
	out := []ThinkTankMember{}
	for _, m := range participantsOf(d.Members) {
		if m.State == ThinkTankMemberActive && m.Completed < m.Cap {
			out = append(out, m)
		}
	}
	return out
}

func thinkTankStopReason(d ThinkTankDetail) string {
	left, exhausted := false, false
	for _, m := range participantsOf(d.Members) {
		switch {
		case m.State == ThinkTankMemberDeparted:
			left = true
		case m.State == ThinkTankMemberExhausted || m.Completed >= m.Cap:
			exhausted = true
		}
	}
	switch {
	case left && exhausted:
		return ThinkTankEndLeftAndExhausted
	case left:
		return ThinkTankEndParticipantsLeft
	default:
		return ThinkTankEndAllowanceExhausted
	}
}

// thinkTankTransition returns the next phase and its reason, or "" when the
// room is settled.
func thinkTankTransition(d ThinkTankDetail) (string, string) {
	r := d.Room
	switch r.Phase {
	case ThinkTankPhaseSetup:
		for _, m := range participantsOf(d.Members) {
			if m.SetupState != ThinkTankSetupReady {
				return "", ""
			}
		}
		if r.Openings {
			return ThinkTankPhaseOpenings, ""
		}
		return ThinkTankPhaseDiscussion, ""
	case ThinkTankPhaseOpenings:
		if d.Active != nil {
			return "", ""
		}
		withheld := map[string]bool{}
		for _, a := range d.Attempts {
			if a.Turn == ThinkTankTurnOpening && a.State == ThinkTankAttemptWithheld {
				withheld[a.AgentID] = true
			}
		}
		for _, m := range participantsOf(d.Members) {
			if !withheld[m.AgentID] {
				return "", ""
			}
		}
		return ThinkTankPhaseDiscussion, ""
	case ThinkTankPhaseDiscussion:
		if d.Active != nil {
			return "", ""
		}
		switch n := len(eligibleThinkTankMembers(d)); {
		case n >= 2:
			return "", ""
		case n == 1:
			return ThinkTankPhaseClosing, thinkTankStopReason(d)
		default:
			return ThinkTankPhaseEnded, thinkTankStopReason(d)
		}
	case ThinkTankPhaseClosing:
		if d.Active != nil {
			return "", ""
		}
		if len(eligibleThinkTankMembers(d)) == 0 {
			return ThinkTankPhaseEnded, thinkTankStopReason(d)
		}
		for _, a := range d.Attempts {
			if a.Turn == ThinkTankTurnClosing && a.State == ThinkTankAttemptFinalized {
				return ThinkTankPhaseEnded, r.EndReason
			}
		}
	}
	return "", ""
}

// NextThinkTankOpportunity names the one turn the room may admit now, or
// false when it must not start any: paused, held, ending, mid-attempt, in
// setup, or finished. Turn-taking cycles the fixed configured order from the
// rotation cursor, skipping only departed and exhausted participants
// (FS-21.R34). Busy speakers are not skipped; the caller waits for them.
func NextThinkTankOpportunity(d ThinkTankDetail) (ThinkTankOpportunity, bool) {
	r := d.Room
	if r.Phase == ThinkTankPhaseOpenings {
		if next := ThinkTankOpeningOpportunities(d); len(next) > 0 {
			return next[0], true
		}
		return ThinkTankOpportunity{}, false
	}
	if d.Active != nil || r.Hold != "" || r.Control != ThinkTankRunning {
		return ThinkTankOpportunity{}, false
	}
	switch r.Phase {
	case ThinkTankPhaseDiscussion:
		eligible := eligibleThinkTankMembers(d)
		if len(eligible) < 2 {
			return ThinkTankOpportunity{}, false
		}
		for _, m := range eligible {
			if m.Order >= r.Rotation {
				return ThinkTankOpportunity{AgentID: m.AgentID, Turn: ThinkTankTurnDiscussion}, true
			}
		}
		return ThinkTankOpportunity{AgentID: eligible[0].AgentID, Turn: ThinkTankTurnDiscussion}, true
	case ThinkTankPhaseClosing:
		if eligible := eligibleThinkTankMembers(d); len(eligible) == 1 {
			return ThinkTankOpportunity{AgentID: eligible[0].AgentID, Turn: ThinkTankTurnClosing}, true
		}
	case ThinkTankPhaseEnded:
		if r.JudgeStatus == ThinkTankJudgeStarting && r.JudgeAgentID != "" {
			return ThinkTankOpportunity{AgentID: r.JudgeAgentID, Turn: ThinkTankTurnJudge}, true
		}
	}
	return ThinkTankOpportunity{}, false
}

// ThinkTankOpeningOpportunities lists every opening that may start now, in
// member order: independent openings run concurrently (FS-21.R48). A member
// with a running, withheld or published opening is done for now; one whose
// opening failed waits for explicit retry, never a sweep or Resume.
func ThinkTankOpeningOpportunities(d ThinkTankDetail) []ThinkTankOpportunity {
	r := d.Room
	if r.Phase != ThinkTankPhaseOpenings || r.Hold != "" || r.Control != ThinkTankRunning {
		return nil
	}
	taken := thinkTankOpeningTaken(d)
	out := []ThinkTankOpportunity{}
	for _, m := range participantsOf(d.Members) {
		if !taken[m.AgentID] {
			out = append(out, ThinkTankOpportunity{AgentID: m.AgentID, Turn: ThinkTankTurnOpening})
		}
	}
	return out
}

// thinkTankOpeningTaken marks members whose opening needs no new admission:
// running, withheld, published or failed without an explicit retry.
func thinkTankOpeningTaken(d ThinkTankDetail) map[string]bool {
	taken := map[string]bool{}
	for _, a := range d.Attempts {
		if a.Turn != ThinkTankTurnOpening {
			continue
		}
		switch a.State {
		case ThinkTankAttemptRunning, ThinkTankAttemptWithheld, ThinkTankAttemptFinalized, ThinkTankAttemptFailed:
			taken[a.AgentID] = true
		}
	}
	return taken
}

// unretriedThinkTankOpenings lists failed openings still awaiting explicit
// retry; each keeps the room held.
func unretriedThinkTankOpenings(d ThinkTankDetail) []ThinkTankAttempt {
	out := []ThinkTankAttempt{}
	if d.Room.Phase != ThinkTankPhaseOpenings {
		return out
	}
	// A pre-upgrade failure followed by a later opening is already superseded.
	superseded := map[string]bool{}
	for _, a := range d.Attempts {
		if a.Turn == ThinkTankTurnOpening && a.State != ThinkTankAttemptFailed && a.State != ThinkTankAttemptRetried {
			superseded[a.AgentID] = true
		}
	}
	for _, a := range d.Attempts {
		if a.Turn == ThinkTankTurnOpening && a.State == ThinkTankAttemptFailed && !superseded[a.AgentID] {
			out = append(out, a)
		}
	}
	return out
}

// DeleteThinkTank removes one room and only its own rows. It is refused unless
// the room is paused or ended with no active attempt or running judge
// (FS-21.R40); pending room work, including an unstarted judge, goes with it.
func (s *Store) DeleteThinkTank(roomID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("state: begin delete think tank: %w", err)
	}
	defer tx.Rollback()
	d, err := readThinkTankDetail(tx, roomID)
	if err != nil {
		return err
	}
	if d.Active != nil {
		return thinkTankConflict("a room turn is active")
	}
	if d.Room.Phase != ThinkTankPhaseEnded && d.Room.Control != ThinkTankPaused {
		return thinkTankConflict("pause or end the room before deleting it")
	}
	if d.Room.JudgeStatus == ThinkTankJudgeLaunching {
		return thinkTankConflict("the judge is starting")
	}
	for _, m := range d.Members {
		if m.SetupState == ThinkTankSetupLaunching {
			return thinkTankConflict("a participant is starting")
		}
	}
	if _, err := tx.Exec(`DELETE FROM think_tanks WHERE room_id = ?`, roomID); err != nil {
		return fmt.Errorf("state: delete think tank: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: commit delete think tank: %w", err)
	}
	return nil
}
