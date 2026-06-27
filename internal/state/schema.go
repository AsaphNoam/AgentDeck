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
}
