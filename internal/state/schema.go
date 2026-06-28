package state

type migration struct {
	version int
	sql     string
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
}
