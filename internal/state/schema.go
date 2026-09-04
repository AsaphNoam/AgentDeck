package state

import "database/sql"

type migration struct {
	version int
	sql     string
	apply   func(*sql.Tx) error
}

var migrations = []migration{
	{
		version: 1,
		sql: `
CREATE TABLE IF NOT EXISTS agents (
    agent_id   TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL,
    project    TEXT NOT NULL,
    backend    TEXT NOT NULL,
    model      TEXT NOT NULL,
    interface  TEXT NOT NULL,
    created_at TEXT NOT NULL,
    grp        TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS running (
    agent_id   TEXT PRIMARY KEY REFERENCES agents(agent_id) ON DELETE CASCADE,
    pid        INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    interface  TEXT NOT NULL,
    tty        TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS status (
    agent_id    TEXT PRIMARY KEY REFERENCES agents(agent_id) ON DELETE CASCADE,
    state       TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    last_trace  TEXT NOT NULL DEFAULT '',
    busy_since  TEXT,
    context_pct REAL NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    from_agent TEXT NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
    to_agent   TEXT NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
    body       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    read_at    TEXT
);

CREATE INDEX IF NOT EXISTS idx_messages_to ON messages(to_agent, read_at);
`,
	},
	{
		version: 2,
		sql: `
ALTER TABLE status ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0;
`,
	},
	{
		version: 3,
		sql: `
ALTER TABLE running ADD COLUMN hook_token TEXT NOT NULL DEFAULT '';
`,
	},
	{
		version: 4,
		sql: `
CREATE TABLE IF NOT EXISTS sessions (
  agent_id        TEXT PRIMARY KEY,
  name            TEXT NOT NULL,
  role            TEXT NOT NULL,
  project         TEXT NOT NULL,
  backend         TEXT NOT NULL,
  model           TEXT NOT NULL,
  interface       TEXT NOT NULL,
  grp             TEXT NOT NULL DEFAULT '',
  cwd             TEXT NOT NULL,
  system_prompt   TEXT NOT NULL,
  env_keys        TEXT NOT NULL DEFAULT '[]',
  last_session_id TEXT NOT NULL DEFAULT '',
  last_seq        INTEGER NOT NULL DEFAULT 0,
  last_context_pct REAL NOT NULL DEFAULT 0,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL,
  turn_count      INTEGER NOT NULL DEFAULT 0,
  event_count     INTEGER NOT NULL DEFAULT 0,
  files_touched   INTEGER NOT NULL DEFAULT 0,
  commands_run    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at DESC);

CREATE TABLE IF NOT EXISTS tracked_files (
  agent_id     TEXT NOT NULL,
  path         TEXT NOT NULL,
  abs_path     TEXT NOT NULL,
  edit_count   INTEGER NOT NULL DEFAULT 0,
  first_seq    INTEGER NOT NULL,
  last_seq     INTEGER NOT NULL,
  first_ts     TEXT NOT NULL,
  last_ts      TEXT NOT NULL,
  has_diff     INTEGER NOT NULL DEFAULT 0,
  diff_refs    TEXT NOT NULL DEFAULT '[]',
  PRIMARY KEY (agent_id, path)
);
CREATE INDEX IF NOT EXISTS idx_files_agent_ts ON tracked_files(agent_id, last_ts DESC);

CREATE TABLE IF NOT EXISTS tracked_commands (
  agent_id     TEXT NOT NULL,
  seq          INTEGER NOT NULL,
  ts           TEXT NOT NULL,
  tool_call_id TEXT NOT NULL,
  command      TEXT NOT NULL,
  exit_status  TEXT NOT NULL DEFAULT 'in_progress',
  exit_error   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (agent_id, seq)
);
CREATE INDEX IF NOT EXISTS idx_commands_agent_seq ON tracked_commands(agent_id, seq DESC);
`,
	},
	{
		// Phase 5 messaging (techspec §4.1, §6.1). Replaces the Phase-0
		// placeholder messages table (different shape + TEXT message_id PK, and
		// no agent FK so a stopped agent's mail survives until the janitor — §4.3).
		version: 5,
		sql: `
DROP TABLE IF EXISTS messages;

CREATE TABLE messages (
  message_id     TEXT PRIMARY KEY,
  from_agent     TEXT NOT NULL,
  from_address   TEXT NOT NULL,
  from_name      TEXT NOT NULL,
  to_agent       TEXT NOT NULL,
  subject        TEXT NOT NULL DEFAULT '',
  body           TEXT NOT NULL,
  created_at     TEXT NOT NULL,
  read           INTEGER NOT NULL DEFAULT 0,
  read_at        TEXT,
  delivered_via  TEXT NOT NULL DEFAULT 'pending',
  in_reply_to    TEXT
);
CREATE INDEX IF NOT EXISTS idx_messages_to_unread  ON messages(to_agent, read);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);

CREATE TABLE IF NOT EXISTS turn_budget (
  agent_id  TEXT NOT NULL,
  turn_id   TEXT NOT NULL,
  inbound   INTEGER NOT NULL DEFAULT 0,
  outbound  INTEGER NOT NULL DEFAULT 0,
  breached  INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (agent_id, turn_id)
);
`,
	},
	{
		// Phase 6 terminal runtime (techspec §3.1 step 6): the running row records
		// which TerminalDriver backs the agent (xterm/tmux/iterm2) and any
		// driver-specific identifiers (e.g. iTerm2 window/tab/session ids, the
		// tmux session name). Empty for chat agents. driver_ids is a JSON object.
		version: 6,
		sql: `
ALTER TABLE running ADD COLUMN driver     TEXT NOT NULL DEFAULT '';
ALTER TABLE running ADD COLUMN driver_ids TEXT NOT NULL DEFAULT '{}';
`,
	},
	{
		// Freeze skip_permissions and add_dirs into the composed-config snapshot so
		// resume/switch reproduce the original launch composition instead of
		// re-reading the current role/project files (techspec §12.4 frozen-snapshot
		// rule; the master-PRD invariant that a running agent's spec is frozen and
		// edits affect future launches only). add_dirs is a JSON array of strings.
		// Pre-existing rows default to skip=0 (fail closed: never auto-approve) and
		// no extra dirs.
		version: 7,
		sql: `
ALTER TABLE sessions ADD COLUMN skip_permissions INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN add_dirs         TEXT    NOT NULL DEFAULT '[]';
`,
	},
	{
		// Freeze the resolved configuration-federation launch object into the
		// session snapshot (Phase 7 techspec §2.5): a redacted, versioned JSON of
		// requested-vs-resolved model/effort/provider, the binding
		// backend/provider/profile, the source generation + fingerprints, and
		// whether native defaults were inherited. Resume reads this frozen
		// high-level object; an ACP-observed model is separate runtime state and
		// must never rewrite it. Pre-existing rows default to an empty object.
		version: 8,
		sql: `
ALTER TABLE sessions ADD COLUMN launch_config_json TEXT NOT NULL DEFAULT '{}';
`,
	},
	{
		// Replace the single whole-session FTS document with a metadata document
		// plus immutable transcript documents. The migration is capability-aware:
		// tagged builds create FTS5, while untagged builds keep the plain fallback.
		version: 9,
		apply:   ensureSessionsFTS,
	},
	{
		// Native pipeline control-plane state (TS-02.R17 / TS-09). Template JSON
		// remains in the config store; these tables own immutable run snapshots,
		// attempt lineage/reports, current named values, and idempotent starts.
		// Agent ids are deliberately logical references with no foreign key so
		// deleting a run cannot cascade into ordinary agents or transcripts.
		version: 10,
		sql: `
CREATE TABLE pipeline_runs (
  run_id                 TEXT PRIMARY KEY,
  template_id            TEXT NOT NULL,
  template_snapshot_json TEXT NOT NULL DEFAULT '{}',
  display_name           TEXT NOT NULL,
  project                TEXT NOT NULL,
  goal                   TEXT NOT NULL,
  inputs_json            TEXT NOT NULL DEFAULT '{}',
  assignments_json       TEXT NOT NULL DEFAULT '{}',
  state                  TEXT NOT NULL,
  revision               INTEGER NOT NULL DEFAULT 1,
  pending_action         TEXT NOT NULL DEFAULT '',
  current_stage_id       TEXT NOT NULL DEFAULT '',
  current_attempt_id     TEXT NOT NULL DEFAULT '',
  current_agent_id       TEXT NOT NULL DEFAULT '',
  attention_reason       TEXT NOT NULL DEFAULT '',
  final_outcome          TEXT NOT NULL DEFAULT '',
  created_at             TEXT NOT NULL,
  updated_at             TEXT NOT NULL
);
CREATE INDEX idx_pipeline_runs_state_updated ON pipeline_runs(state, updated_at DESC);
CREATE INDEX idx_pipeline_runs_project_state ON pipeline_runs(project, state);

CREATE TABLE pipeline_attempts (
  attempt_id          TEXT PRIMARY KEY,
  run_id              TEXT NOT NULL REFERENCES pipeline_runs(run_id) ON DELETE CASCADE,
  stage_id            TEXT NOT NULL,
  attempt_no          INTEGER NOT NULL,
  visit_no            INTEGER NOT NULL,
  parent_attempt_id   TEXT,
  agent_id            TEXT NOT NULL DEFAULT '',
  agent_generation    TEXT NOT NULL DEFAULT '',
  backend             TEXT NOT NULL,
  model               TEXT NOT NULL,
  state               TEXT NOT NULL,
  assignment_text     TEXT NOT NULL DEFAULT '',
  assignment_hash     TEXT NOT NULL DEFAULT '',
  assignment_version  INTEGER NOT NULL DEFAULT 1,
  report_outcome      TEXT NOT NULL DEFAULT '',
  report_summary      TEXT NOT NULL DEFAULT '',
  report_details      TEXT NOT NULL DEFAULT '',
  report_checks       TEXT NOT NULL DEFAULT '',
  report_outputs_json TEXT NOT NULL DEFAULT '{}',
  reported_at         TEXT,
  quiescent_at        TEXT,
  created_at          TEXT NOT NULL,
  updated_at          TEXT NOT NULL,
  UNIQUE(run_id, attempt_no)
);
CREATE INDEX idx_pipeline_attempts_run_attempt ON pipeline_attempts(run_id, attempt_no);
CREATE INDEX idx_pipeline_attempts_agent ON pipeline_attempts(agent_id, run_id);

CREATE TABLE pipeline_values (
  run_id            TEXT NOT NULL REFERENCES pipeline_runs(run_id) ON DELETE CASCADE,
  name              TEXT NOT NULL,
  value             TEXT NOT NULL,
  source_kind       TEXT NOT NULL,
  source_attempt_id TEXT NOT NULL DEFAULT '',
  updated_at        TEXT NOT NULL,
  PRIMARY KEY(run_id, name)
);

CREATE TABLE pipeline_requests (
  request_id   TEXT PRIMARY KEY,
  request_hash TEXT NOT NULL,
  run_id       TEXT NOT NULL REFERENCES pipeline_runs(run_id) ON DELETE CASCADE,
  created_at   TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_pipeline_requests_run ON pipeline_requests(run_id);
`,
	},
	{
		// Project-first dashboard and grouped Archive. The archive bit belongs to
		// the durable agent identity, not a session snapshot or search projection.
		version: 11,
		sql: `
ALTER TABLE agents ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_agents_project_archived ON agents(project, archived);
		`,
	},
	{
		// Effort is frozen at the lifecycle boundary. The agent identity reflects a
		// later switch, while sessions preserve the archived/resume choice.
		version: 12,
		sql: `
ALTER TABLE agents ADD COLUMN effort TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN effort TEXT NOT NULL DEFAULT '';
`,
	},
	{
		// Each attempt owns its execution identity, so continuation never recovers
		// effort from a run assignment that could later gain reassignment support.
		version: 13,
		sql: `
ALTER TABLE pipeline_attempts ADD COLUMN effort TEXT NOT NULL DEFAULT '';
		`,
	},
	{
		// AgentDecker proposal tool calls must survive adapter transcript shapes and
		// be discoverable by a fresh Pipelines page before MCP reports success.
		version: 14,
		sql: `
CREATE TABLE pipeline_proposals (
  proposal_id  TEXT PRIMARY KEY,
  kind         TEXT NOT NULL,
  digest       TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  created_at   TEXT NOT NULL
);
CREATE INDEX idx_pipeline_proposals_created ON pipeline_proposals(created_at DESC, proposal_id);
`,
	},
	{
		// A proposal is an offer, not a standing action: once its approved Save or
		// Start commits, the record must stop offering that approval. Existing rows
		// adopt '' and stay pending; retention bounds the never-approved rest.
		version: 15,
		sql: `
ALTER TABLE pipeline_proposals ADD COLUMN consumed_at TEXT NOT NULL DEFAULT '';
		`,
	},
	{
		// Explicit, payload-free mail activation (TS-02.R23). The backfill is an
		// apply function because legacy rows need one coalesced activation per
		// recipient, not one activation per message.
		version: 16,
		apply:   migrateActivations,
	},
	{
		// Pull-based context links (TS-02.R24). References are keyed only by an
		// immutable source locator; grants own presentation and authorization, and
		// personal hidden state is a separate row. Reference rows deliberately have
		// no foreign key to source agents, sessions, pipeline runs, or attempts, so
		// deleting a source leaves an honest tombstone rather than cascading or
		// aliasing the id. No backfill: nothing existed before this table.
		version: 17,
		apply:   migrateContextLinks,
	},
	{
		// Dependency-aware work (TS-10.R16). Tasks, arms, attachments, and the
		// shared result layer arrive together, plus the activation source id the
		// dependency kind keys its pending row on. Agent, pipeline-run, and
		// context-reference ids stay logical references without cascades, so this
		// plane can never delete work history or reach into another domain.
		version: 18,
		apply:   migrateTasks,
	},
	{
		// The run-detail delegated-agent projection reads only assigned,
		// agent-created work for one creator/generation window (TS-02.R26).
		// This is a read index only: it changes no task ownership or retention.
		version: 19,
		sql: `
CREATE INDEX idx_tasks_project_creator_created
  ON tasks(project, created_by_agent_id, created_by_generation, created_at DESC, task_id)
  WHERE created_by_kind = 'agent' AND assigned_agent_id IS NOT NULL;
`,
	},
	{
		// AgentDeck-owned Git worktree ownership (TS-02.R27, TS-12.R2). The row's
		// presence is the sole test of whether a checkout may ever be deleted, so
		// it lives in state.db rather than a hand-editable config file. `project`
		// is a logical, non-cascading reference like R25's task references:
		// archiving or deleting a project never removes it, only the explicit
		// consented deletion flow does.
		version: 20,
		sql: `
CREATE TABLE project_worktrees (
  project        TEXT PRIMARY KEY,
  repo_path      TEXT NOT NULL,
  branch         TEXT NOT NULL,
  checkout_path  TEXT NOT NULL,
  created_at     TEXT NOT NULL,
  setup_ok       INTEGER,
  setup_at       TEXT,
  setup_output   TEXT
);
`,
	},
	{
		// A launch-spec task may name the reasoning level its agent runs at
		// (FS-16.R27, TS-10 §3). Empty is the unrequested value, matching the
		// effort columns migration 12 and 13 added, so launch composition resolves
		// the level for a task exactly as it does for any other launch.
		version: 21,
		sql: `
ALTER TABLE tasks ADD COLUMN effort TEXT NOT NULL DEFAULT '';
`,
	},
}
