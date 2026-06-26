# Phase 0 — Foundation: Implementation Tech Spec

**Mirrors:** [phase-0-foundation.md](../phase-0-foundation.md)
**Master PRD:** [agent-dashboard-prd.md](../../agent-dashboard-prd.md) §3, §6, §7
**Rationale:** [architecture-decisions.md](../../architecture-decisions.md) (the *why* behind the storage split and runtime choices)
**Status:** ready to implement
**Audience:** a coding agent implementing the substrate with no further design decisions required.

---

## 1. Overview & scope recap

Stand up the skeleton every later phase hangs off of: the persistence substrate under `~/.agentdeck/`, a Go HTTP **server** bound strictly to `127.0.0.1:{port}`, the `agentdeck` **CLI** (`dashboard start|stop|open`, `--version`), seed-data bootstrap, and an `install.sh` that builds the binary + UI. No agents run, no runtimes, no SSE, no file watching, no config UI — those are Phases 1–4+.

Persistence splits by **who writes the data**:
- **Config** (human-edited, low-volume, `git`-friendly) lives in **plain JSON files**: `roles/`, `projects/`, `backends.json`, `config.json`, `layout.json`. A typed **config file store** reads and writes these with atomic temp-then-rename.
- **Machine state** (agent identity, running registry, live status, messages) lives in a single **SQLite `state.db`**. A typed **SQLite state store** owns it; the Go server is the **sole writer**. Phase 0 creates the base tables only.

**In scope:** repo/package layout; lazy `~/.agentdeck/` creation; typed config file store (Read/Write/List/Delete, atomic writes, `AGENTDECK_HOME` + `~` expansion, corrupt-file survival); typed SQLite state store (`state.db` in WAL mode, versioned migrations on open, base tables, `agent_id` generation); GET-only `/api/*` endpoints sourced from the two stores; CLI skeleton with pidfile/detach; seed data (4 roles, 1 example project, `backends.json`, `config.json`) that never clobbers existing data; `install.sh`.

**Out of scope:** ACP/runtimes/launch (P1), SSE bus + `POST /api/hook` ingest (P2), config CRUD/onboarding (P3), FTS5 search index + transcript-metadata tables (P4), all `POST`/`PUT` mutation endpoints.

---

## 2. Technology choices

| Concern | Choice | Rationale |
|---|---|---|
| Language / toolchain | **Go 1.22** (`go 1.22` in `go.mod`) | Master PRD §7 mandates Go 1.22+ single binary. 1.22 gives the enhanced `net/http` `ServeMux` with method+path patterns (`GET /api/health`), removing the need for a third-party router. |
| HTTP router | **stdlib `net/http.ServeMux`** (1.22 pattern routing) | All Phase 0 routes are static `GET`s. The 1.22 mux matches `"GET /api/sessions"` natively. Zero external router dependency keeps the binary lean and the single-binary promise honest. Revisit (chi/gorilla) only if path params + many methods appear; they don't in Phase 0. |
| State storage | **SQLite via `github.com/mattn/go-sqlite3`**, WAL mode | Machine state needs transactional, queryable, single-file storage with a clear path to full-text session search (FTS5, Phase 4). `go-sqlite3` is the mature, cgo-backed driver with WAL and FTS5 support. The server is the sole writer, so there is no multi-process write contention. |
| Logging | **stdlib `log/slog`** with a `slog.NewJSONHandler` to stderr | "Structured logging" (req 4.3) without a dependency. JSON lines are greppable and machine-readable; level via `AGENTDECK_LOG_LEVEL` (default `info`). |
| CLI framework | **`github.com/spf13/cobra` v1.8.x** | The PRD reserves a multi-verb surface (`dashboard start/stop/open`, `--version`, future `<role>@<project>`). Cobra gives subcommands, flags, help, and version wiring for free and is the de-facto Go CLI standard. (Acceptable lighter alternative: hand-rolled `flag` + switch — but Cobra is the locked choice for ergonomics and Phase 1+ growth.) |
| Open-browser | **`github.com/pkg/browser` v0.0.0** | `dashboard open` must work on macOS (`open`) and Linux (`xdg-open`). This one tiny lib abstracts both; avoids per-OS `exec` branching. |
| UI build | **Vite 5 + React 18 + TypeScript 5** (scaffold only) | Master PRD §7. Phase 0 only scaffolds (`npm create vite@latest ui -- --template react-ts`) and wires the build so `install.sh` produces `ui/dist`. No app logic. Node is **build-time only**. |
| UI asset serving | **`embed` (`//go:embed ui/dist`) compiled into the binary** | See §11 Resolved Decisions. The Go binary serves the built UI at `/` from an embedded FS, so distribution is one file. A build tag `dev` falls back to disk for live Vite dev. |
| Atomic config writes | stdlib only: `os.CreateTemp` + `f.Sync()` + `os.Rename` | No dependency needed; rename is atomic on the same filesystem on macOS/Linux. |
| Testing | stdlib `testing` + `net/http/httptest` | No assertion library; table-driven tests. |
| ID randomness | stdlib `crypto/rand` | `agent_id` must be unguessable-enough and collision-resistant; `crypto/rand` over `math/rand`. |

Dependency budget for Phase 0: **cobra**, **pkg/browser**, **mattn/go-sqlite3**. Everything else is stdlib. Node is a build-time tool (Vite UI, embedded into the binary), never a runtime dependency.

---

## 3. Repository / package layout

```
AgentDeck/
  go.mod                          module github.com/agentdeck/agentdeck, go 1.22
  go.sum
  install.sh                      build Go binary + UI bundle, install CLI on PATH, seed ~/.agentdeck
  README.md                       prereqs + quickstart (Go 1.22+, Node 18+ build-time, npm)
  Makefile                        convenience targets: build, test, ui, run, clean

  cmd/
    agentdeck/
      main.go                     CLI entrypoint; wires cobra root + version, delegates to internal/cli

  internal/
    cli/
      root.go                     cobra root command, --version, global flags
      dashboard.go                `dashboard` parent + start/stop/open subcommands
      launch_stub.go              reserved `agentdeck <role>@<project>` → "not yet implemented"
      pidfile.go                  write/read/remove pidfile, liveness check (signal 0)
    server/
      server.go                   Server struct, New(), Start(ctx), graceful shutdown
      bind.go                     127.0.0.1 bind + assertion that host is never 0.0.0.0
      routes.go                   ServeMux wiring of all GET /api/* routes
      handlers.go                 handler funcs (health, sessions, roles, projects, backends, layout)
      middleware.go               CORS (local Vite origin) + slog request logging
      static.go                   embedded UI FS handler (//go:embed) with dev-disk fallback
      json.go                     writeJSON / writeError helpers (consistent envelope + status)
    config/
      config.go                   Config file store: Store struct, New(home), Home resolution
      paths.go                    AGENTDECK_HOME + ~ expansion, per-object path builders
      layout_dirs.go              EnsureLayout(): create all config dirs lazily, never clobber
      types.go                    config data structs (§4.1) with json tags
      roles.go                    Read/Write/List/Delete Role
      projects.go                 Read/Write/List/Delete Project
      backends.go                 ReadBackends/WriteBackends (single file)
      appconfig.go                ReadConfig/WriteConfig (single file)
      layout.go                   ReadLayout/WriteLayout (single file)
      atomic.go                   writeJSONAtomic (temp+fsync+rename), readJSON (corrupt → ErrCorrupt)
      seed.go                     SeedIfAbsent(): 4 roles, example project, backends.json, config.json
      errors.go                   ErrNotFound, ErrCorrupt sentinels
    state/
      state.go                    State store: Store struct, Open(home) → *sql.DB in WAL mode, Close()
      migrate.go                  versioned schema migrations applied on Open
      schema.go                   embedded migration SQL (CREATE TABLE … for base tables)
      types.go                    state data structs (§4.2) with db column mapping
      agents.go                   ReadAgent/WriteAgent/ListAgents/DeleteAgent + NewAgentID
      running.go                  Read/Write/List/Delete RunningEntry
      status.go                   Read/Write/List/Delete Status
      messages.go                 Read/Write/List/Delete Message
      errors.go                   ErrNotFound sentinel
    version/
      version.go                  var Version, Commit, Date (ldflags-injected)

    config/
      testdata/                   corrupt JSON fixtures (lives at internal/config/testdata/)
        corrupt_role.json
        corrupt_backends.json

  ui/
    index.html
    package.json                  vite + react + typescript deps, "build" → dist/
    tsconfig.json
    vite.config.ts                base "/", build.outDir "dist"
    src/
      main.tsx                    React root render
      App.tsx                     placeholder: pings /api/health, shows "AgentDeck — Phase 0"
    dist/                         build output, embedded by internal/server/static.go (gitignored)

  docs/                           (existing) PRD + phase docs + this tech spec
```

Notes:
- `internal/` keeps packages unimportable outside the module — intentional, this is an app not a library.
- The two stores are separate packages: `internal/config` (JSON files) and `internal/state` (SQLite). They share nothing but the resolved home directory.
- Test fixtures live in `internal/config/testdata/` (Go's `testdata` convention — ignored by the build, accessible from tests in the package).

---

## 4. Data structures (Go)

### 4.1 Config file store types

Config structs live in `internal/config/types.go`. JSON tags match master PRD §3 exactly. Use pointers for nullable/override fields so `null` vs absent vs value is distinguishable.

```go
package config

// ---- Role: roles/{role}.json (PRD §3.2) ----
type Role struct {
	Title           string `json:"title"`
	SystemPrompt    string `json:"system_prompt"`
	SkipPermissions *bool  `json:"skip_permissions"` // null = inherit global; true/false = override
}

// ---- Project: projects/{project}.json (PRD §3.3) ----
type Project struct {
	Title         string   `json:"title"`
	Color         [3]int   `json:"color"`            // RGB display accent, e.g. [100,180,255]
	Cwd           string   `json:"cwd"`              // "~/Projects/my-app"
	AddDirs       []string `json:"add_dirs"`         // extra accessible directories
	ContextPrompt string   `json:"context_prompt"`
}

// ---- Backend config: backends.json (PRD §3.4) ----
type BackendsConfig struct {
	Version  int                `json:"version"`     // == 2
	Backends map[string]Backend `json:"backends"`    // keyed by backend id ("claude","codex")
}

type Backend struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`              // "claude-acp" | "codex-acp"
	Default      bool              `json:"default,omitempty"` // exactly one backend default
	DefaultModel string            `json:"default_model"`
	Models       map[string]Model  `json:"models"`            // keyed by model id
	Env          map[string]string `json:"env,omitempty"`     // backend-level env, applies to all models
}

type Model struct {
	Name  string            `json:"name"`
	Model string            `json:"model"`                // provider model string ("claude-sonnet-4-6")
	Env   map[string]string `json:"env,omitempty"`        // per-model env; overrides backend env
}

// ---- Layout: layout.json (PRD §3.5) ----
type Layout struct {
	Order   []string `json:"order"`                     // agent_id card order
	Density Density  `json:"density"`
}

type Density struct {
	CardsPerRow int `json:"cards_per_row"`
	Gap         int `json:"gap"`                         // px
}

// ---- Config: config.json (PRD §3.5 + phase-0 §3) ----
type Config struct {
	Version         int    `json:"version"`              // == 1
	Port            int    `json:"port"`                 // 4317
	DefaultProject  string `json:"default_project"`      // "my-app"
	DefaultRole     string `json:"default_role"`         // "implementer"
	SkipPermissions bool   `json:"skip_permissions"`     // false
}
```

### 4.2 State store types

State structs live in `internal/state/types.go`. The JSON tags match master PRD §3 (used for API serialization); the same fields map to SQLite columns (the JSON shapes in PRD §3 describe their logical columns).

```go
package state

import "time"

// ---- Agent identity: row in `agents` (PRD §3.1) ----
type Agent struct {
	AgentID   string    `json:"agent_id"`            // stable, never changes ("a_8f3c12") — PK
	Name      string    `json:"name"`               // human display name, user-editable
	Role      string    `json:"role"`               // references roles/{role}.json
	Project   string    `json:"project"`            // references projects/{project}.json
	Backend   string    `json:"backend"`            // references a backend key in backends.json
	Model     string    `json:"model"`              // model key within the backend
	Interface string    `json:"interface"`          // "chat" | "terminal"
	CreatedAt time.Time `json:"created_at"`          // RFC3339
	Group     string    `json:"group,omitempty"`    // optional task-group label
}

// ---- Active session registry: row in `running` (PRD §3.1) ----
type RunningEntry struct {
	AgentID   string    `json:"agent_id"`            // PK; FK → agents.agent_id
	PID       int       `json:"pid"`                // process group id of the CLI
	SessionID string    `json:"session_id"`         // ephemeral, changes on fork/resume
	Interface string    `json:"interface"`          // "chat" | "terminal"
	TTY       string    `json:"tty,omitempty"`      // only for terminal interface
	StartedAt time.Time `json:"started_at"`         // RFC3339
}

// ---- Live state: row in `status` (PRD §3.1) ----
type Status struct {
	AgentID    string     `json:"agent_id"`          // PK; FK → agents.agent_id
	State      string     `json:"state"`             // "busy"|"idle"|"waiting_input"|"done"|"error"
	Detail     string     `json:"detail,omitempty"`  // "Editing src/auth.ts"
	LastTrace  string     `json:"last_trace,omitempty"`
	BusySince  *time.Time `json:"busy_since,omitempty"`
	ContextPct float64    `json:"context_pct"`       // 0..1
}

// ---- Message: row in `messages` (PRD §3.6 agent-to-agent messaging) ----
type Message struct {
	ID        int64     `json:"id"`                  // autoincrement PK
	FromAgent string    `json:"from_agent"`          // sender agent_id
	ToAgent   string    `json:"to_agent"`            // recipient agent_id
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`          // RFC3339
	ReadAt    *time.Time `json:"read_at,omitempty"`  // null until the recipient checks messages
}
```

### 4.3 Config file store interface

`internal/config` exposes a single `*Store` value holding the resolved home directory. Per-object methods follow one shape. Generic helpers in `atomic.go` back them.

```go
// config.go
type Store struct {
	home string // absolute, resolved from AGENTDECK_HOME or ~/.agentdeck
}

func New() (*Store, error)              // resolves home, does NOT create dirs
func (s *Store) Home() string

// paths.go — resolution rules
// 1. If $AGENTDECK_HOME set and non-empty → use it (expand leading ~).
// 2. Else → filepath.Join(userHomeDir, ".agentdeck").
// Leading "~" in any stored path (e.g. project.cwd) is expanded by ExpandTilde() on read by callers, NOT by the store.
func ExpandTilde(p string) (string, error)

// layout_dirs.go
func (s *Store) EnsureLayout() error    // mkdir -p config dirs in §3 (roles/, projects/, sessions/);
                                        // idempotent; never deletes/overwrites

// atomic.go — internal generics
func writeJSONAtomic(path string, v any) error // mkdir parent; CreateTemp in same dir; encode (indent 2);
                                               // f.Sync(); f.Close(); os.Rename(tmp, path); on any error remove tmp
func readJSON(path string, v any) error        // os.ReadFile; if not exist → ErrNotFound;
                                               // if json.Unmarshal fails → log + ErrCorrupt

// errors.go
var ErrNotFound = errors.New("config: not found")
var ErrCorrupt  = errors.New("config: corrupt file")
```

Per-type methods (Role/Project keyed by their id; Backends/Config/Layout are single-file Read/Write):

```go
// roles.go
func (s *Store) ReadRole(id string) (Role, error)         // ErrNotFound if absent; ErrCorrupt if unparseable
func (s *Store) WriteRole(id string, r Role) error        // atomic
func (s *Store) ListRoles() (map[string]Role, error)      // skip corrupt entries (log+continue), never error whole list
func (s *Store) DeleteRole(id string) error               // os.Remove; ErrNotFound tolerated as nil

// projects.go — same shape as roles.go, keyed by project id

// Single-file objects:
func (s *Store) ReadBackends() (BackendsConfig, error)
func (s *Store) WriteBackends(BackendsConfig) error
func (s *Store) ReadConfig() (Config, error)
func (s *Store) WriteConfig(Config) error
func (s *Store) ReadLayout() (Layout, error)
func (s *Store) WriteLayout(Layout) error
```

**`List*` corrupt-survival contract:** `List*` reads every `*.json` in the dir, skips files that fail to parse (log a warning via slog with the path), and returns the parseable ones. A corrupt single file never fails the whole list. Single-file readers (`ReadConfig`, `ReadBackends`, `ReadLayout`) return `ErrCorrupt`; callers fall back to defaults (see §7).

### 4.4 SQLite state store interface

`internal/state` owns `state.db`. `Open` resolves the DB path under the home dir, opens it in WAL mode, applies migrations, and returns a `*Store` wrapping the `*sql.DB`. The server is the **sole writer**.

```go
// state.go
type Store struct {
	db *sql.DB
}

// Open opens (creating if absent) ~/.agentdeck/state.db, sets WAL + foreign_keys,
// applies pending migrations, and returns the store. home is the resolved AgentDeck home.
func Open(home string) (*Store, error)
func (s *Store) Close() error
func (s *Store) DB() *sql.DB    // escape hatch for later phases

// agents.go — agent_id minting lives here now
// NewAgentID generates "a_" + 6 lowercase hex chars from crypto/rand,
// retrying on collision against the agents table (max 10 tries).
func (s *Store) NewAgentID() (string, error)

func (s *Store) ReadAgent(id string) (Agent, error)       // ErrNotFound if no row
func (s *Store) WriteAgent(a Agent) error                 // INSERT … ON CONFLICT(agent_id) DO UPDATE
func (s *Store) ListAgents() ([]Agent, error)             // ordered by created_at; [] when empty
func (s *Store) DeleteAgent(id string) error              // DELETE; cascades to running/status/messages

// running.go / status.go — same shape, keyed by agent_id (one row per agent)
func (s *Store) ReadRunning(id string) (RunningEntry, error)
func (s *Store) WriteRunning(RunningEntry) error
func (s *Store) ListRunning() ([]RunningEntry, error)
func (s *Store) DeleteRunning(id string) error

func (s *Store) ReadStatus(id string) (Status, error)
func (s *Store) WriteStatus(Status) error
func (s *Store) ListStatus() ([]Status, error)
func (s *Store) DeleteStatus(id string) error

// messages.go
func (s *Store) WriteMessage(m Message) (int64, error)    // INSERT; returns new id
func (s *Store) ListMessages(toAgent string) ([]Message, error)
func (s *Store) DeleteMessage(id int64) error

// errors.go
var ErrNotFound = errors.New("state: not found")
```

All writes go through `database/sql` with parameterized statements. `Open` sets the connection pool to a single writer-friendly configuration (`db.SetMaxOpenConns(1)` is acceptable for Phase 0 given the sole-writer model; WAL still allows concurrent readers via separate handles in later phases).

### 4.5 SQLite schema (Phase 0 base tables)

`internal/state/schema.go` embeds migration SQL. Phase 0 ships migration `0001_base`. The FTS5 search index and transcript-metadata tables arrive in Phase 4 as later migrations.

```sql
-- migration 0001_base

CREATE TABLE IF NOT EXISTS agents (
    agent_id   TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL,
    project    TEXT NOT NULL,
    backend    TEXT NOT NULL,
    model      TEXT NOT NULL,
    interface  TEXT NOT NULL,
    created_at TEXT NOT NULL,          -- RFC3339
    grp        TEXT NOT NULL DEFAULT '' -- "group" is reserved; column is grp, JSON tag "group"
);

CREATE TABLE IF NOT EXISTS running (
    agent_id   TEXT PRIMARY KEY REFERENCES agents(agent_id) ON DELETE CASCADE,
    pid        INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    interface  TEXT NOT NULL,
    tty        TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL           -- RFC3339
);

CREATE TABLE IF NOT EXISTS status (
    agent_id    TEXT PRIMARY KEY REFERENCES agents(agent_id) ON DELETE CASCADE,
    state       TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    last_trace  TEXT NOT NULL DEFAULT '',
    busy_since  TEXT,                  -- RFC3339, nullable
    context_pct REAL NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    from_agent TEXT NOT NULL,
    to_agent   TEXT NOT NULL,
    body       TEXT NOT NULL,
    created_at TEXT NOT NULL,          -- RFC3339
    read_at    TEXT                    -- RFC3339, nullable
);

CREATE INDEX IF NOT EXISTS idx_messages_to ON messages(to_agent, read_at);
```

A `schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)` table records which migrations have run. `migrate.go` opens a transaction, checks the max applied version, applies each pending migration in order, records it, and commits — so re-opening an up-to-date `state.db` is a no-op and never clobbers data.

---

## 5. Component design

### 5.1 Server bootstrap & 127.0.0.1 bind assertion

`internal/server/bind.go`:

```go
const BindHost = "127.0.0.1"

// LocalAddr builds the listen address and refuses anything but loopback.
func LocalAddr(port int) (string, error) {
	if port <= 0 || port > 65535 { return "", fmt.Errorf("invalid port %d", port) }
	return net.JoinHostPort(BindHost, strconv.Itoa(port)), nil
}
```

`internal/server/server.go`:
- `New(cfgStore *config.Store, st *state.Store, cfg config.Config, log *slog.Logger) *Server`.
- `Start(ctx context.Context) error`:
  1. `addr, _ := LocalAddr(cfg.Port)`.
  2. `ln, err := net.Listen("tcp", addr)` — **assert** `ln.Addr().(*net.TCPAddr).IP.IsLoopback()` is true; if not, close and fatal. This is the runtime guard backing the "never 0.0.0.0" requirement and is directly testable (§9).
  3. Build `*http.Server{Handler: routes()}`; serve via `srv.Serve(ln)` in a goroutine.
  4. Block on `ctx.Done()` (SIGINT/SIGTERM via `signal.NotifyContext`), then `srv.Shutdown(timeoutCtx)` (5s), close the state store, remove pidfile if owned.
- Structured logging through `slog` injected logger; every request logged by middleware (method, path, status, duration).

### 5.2 Route table (GET-only, Phase 0)

`internal/server/routes.go` using the 1.22 mux:

| Pattern | Handler | Source |
|---|---|---|
| `GET /api/health` | `handleHealth` | static + version |
| `GET /api/sessions` | `handleSessions` | `state.ListRunning()` joined w/ `state.ListAgents()` → `[]` when empty |
| `GET /api/roles` | `handleRoles` | `config.ListRoles()` |
| `GET /api/projects` | `handleProjects` | `config.ListProjects()` |
| `GET /api/backends` | `handleBackends` | `config.ReadBackends()` |
| `GET /api/layout` | `handleLayout` | `config.ReadLayout()` (default if absent) |
| `GET /` and `GET /{path...}` | `handleStatic` | embedded UI FS; SPA fallback to `index.html` |

Sessions / identity / status come from `state.db`; roles / projects / backends / layout come from the config file store. All `/api/*` handlers are wrapped by `middleware(cors, requestLog)`. Unmatched `/api/*` → 404 JSON. Non-GET to a GET route → mux returns 405 automatically.

### 5.3 CLI commands

`internal/cli` (cobra):

- `agentdeck --version` → prints `version.Version (commit, date)` and exits. Wired via `rootCmd.Version`.
- `agentdeck dashboard start [--port N] [--detach]`:
  - Resolve config-store home, `EnsureLayout()`, `SeedIfAbsent()`, open the state store (`state.Open(home)` — applies migrations, creates base tables), `ReadConfig()` (port flag overrides config.Port for this run only; not persisted).
  - **Foreground (default):** install `signal.NotifyContext`, write pidfile (`{home}/dashboard.pid` with current PID + chosen port), call `server.Start(ctx)`, close the state store and remove pidfile on exit.
  - **`--detach`:** re-exec self with an internal hidden flag `--__daemon` (or fork via `exec.Command(os.Args[0], "dashboard","start", ...)` with `Setsid`), detach stdio to `{home}/dashboard.log`, parent writes child PID to pidfile and prints `started pid=<n> http://127.0.0.1:<port>`, then exits 0.
- `agentdeck dashboard stop`:
  - Read pidfile; if missing → "not running" (exit 0). If PID not alive (`syscall.Kill(pid, 0)` → ESRCH) → remove stale pidfile, report "not running". Else send SIGTERM, poll up to 5s for exit, then SIGKILL fallback; remove pidfile.
- `agentdeck dashboard open`:
  - Read pidfile for port (fallback to config.Port); `browser.OpenURL("http://127.0.0.1:<port>/")`.
- `agentdeck <role>@<project>` (reserved): `launch_stub.go` parses the `role@project` arg form and prints `launch not yet implemented (Phase 1)`; exit code 0. Detected when the first arg contains `@` and is not a known subcommand.

**pidfile format** (`{home}/dashboard.pid`): a single JSON line `{"pid":48213,"port":4317}` — so `stop` and `open` both read it. Liveness via signal 0.

### 5.4 Seed-data bootstrap (never clobbers)

`internal/config/seed.go` → `SeedIfAbsent()`. Called after `EnsureLayout()`. Each item written **only if the target path does not already exist** (`os.Stat` → `IsNotExist`). Never overwrites. Seeding touches **config files only** — the state store is created empty (no seeded rows).

Seed set:
- `config.json` (if absent):
  ```json
  { "version": 1, "port": 4317, "default_project": "my-app", "default_role": "implementer", "skip_permissions": false }
  ```
- `backends.json` (if absent): the master PRD §3.4 example trimmed to safe defaults (no real keys):
  ```json
  {
    "version": 2,
    "backends": {
      "claude": {
        "name": "Claude", "type": "claude-acp", "default": true, "default_model": "sonnet-4-6",
        "models": {
          "sonnet-4-6": { "name": "Sonnet 4.6", "model": "claude-sonnet-4-6" },
          "opus-4-7":   { "name": "Opus 4.7",   "model": "claude-opus-4-7" }
        }
      },
      "codex": {
        "name": "Codex", "type": "codex-acp", "default_model": "gpt-5.5",
        "models": {
          "gpt-5.5": { "name": "GPT 5.5", "model": "gpt-5.5" },
          "gpt-4o":  { "name": "GPT-4o",  "model": "gpt-4o" }
        }
      }
    }
  }
  ```
- `roles/{implementer,reviewer,researcher,pm}.json` (each if absent):
  - `implementer` → `{ "title":"Implementer", "system_prompt":"Implement the requested changes carefully; write tests; keep diffs focused.", "skip_permissions": null }`
  - `reviewer` → `{ "title":"Reviewer", "system_prompt":"Review changes for correctness, edge cases, and consistency.", "skip_permissions": null }`
  - `researcher` → `{ "title":"Researcher", "system_prompt":"Investigate and summarize; gather context before proposing actions.", "skip_permissions": null }`
  - `pm` → `{ "title":"PM", "system_prompt":"Coordinate work, break down tasks, and track progress across agents.", "skip_permissions": null }`
- `projects/my-app.json` (if absent):
  ```json
  { "title":"My App", "color":[100,180,255], "cwd":"~/Projects/my-app", "add_dirs":[], "context_prompt":"Project-specific context injected into every agent here." }
  ```
- `layout.json` (if absent): `{ "order": [], "density": { "cards_per_row": 3, "gap": 16 } }`

### 5.5 Corrupt-file & missing-state handling

- `readJSON` distinguishes not-exist (`ErrNotFound`) from parse failure (`ErrCorrupt`, logged with path at WARN).
- `List*` (config) skip corrupt files (log + continue) — one bad file never breaks a listing.
- Single-file getters (`ReadConfig`/`ReadBackends`/`ReadLayout`): on `ErrCorrupt` the **handler** falls back to in-memory defaults and logs; the server never crashes. `dashboard start` on corrupt `config.json` logs WARN and uses the default config (port 4317). Do not rewrite the corrupt file (avoid clobbering data the user may want to fix).
- A corrupt role/project file is treated as absent for that id.
- State store: an empty `state.db` (fresh install) yields `[]` from `ListRunning`/`ListAgents`/`ListStatus`. `Read*` for a missing row returns `ErrNotFound`. The migration step is the only write at startup and is idempotent.

---

## 6. API contracts

Base: `http://127.0.0.1:{port}/api`. All Phase 0 endpoints are `GET`. Responses are `application/json; charset=utf-8`. Error envelope: `{"error":"<message>"}`.

### `GET /api/health` → 200
```json
{ "status": "ok", "version": "0.1.0", "time": "2026-06-22T10:00:00Z" }
```
Always 200 if the process is up. No store reads.

### `GET /api/sessions` → 200
Active agents = join of the `running` table with the `agents` table from `state.db`. Empty store → `[]` (never `null`).
```json
[
  {
    "agent_id": "a_8f3c12",
    "name": "Atlas",
    "role": "implementer",
    "project": "my-app",
    "backend": "claude",
    "model": "sonnet-4-6",
    "interface": "chat",
    "created_at": "2026-06-22T10:00:00Z",
    "group": "auth-migration",
    "running": { "pid": 48213, "session_id": "claude-sess-xyz", "interface": "chat", "started_at": "2026-06-22T10:00:01Z" }
  }
]
```
Phase 0 has no agents, so this returns `[]`. Field `running` is the matched `RunningEntry` or omitted if none.

### `GET /api/roles` → 200
Map of role id → Role, from the config store. Seeded store returns the 4 seed roles.
```json
{
  "implementer": { "title": "Implementer", "system_prompt": "Implement the requested changes carefully; write tests; keep diffs focused.", "skip_permissions": null },
  "reviewer":    { "title": "Reviewer",    "system_prompt": "Review changes for correctness, edge cases, and consistency.", "skip_permissions": null },
  "researcher":  { "title": "Researcher",  "system_prompt": "Investigate and summarize; gather context before proposing actions.", "skip_permissions": null },
  "pm":          { "title": "PM",          "system_prompt": "Coordinate work, break down tasks, and track progress across agents.", "skip_permissions": null }
}
```

### `GET /api/projects` → 200
Map of project id → Project, from the config store. Seeded store returns `my-app`.
```json
{
  "my-app": { "title": "My App", "color": [100,180,255], "cwd": "~/Projects/my-app", "add_dirs": [], "context_prompt": "Project-specific context injected into every agent here." }
}
```

### `GET /api/backends` → 200
The full `backends.json` (version 2), from the config store. Corrupt/missing → in-memory default (the seed above) + WARN log; still 200.
```json
{ "version": 2, "backends": { "claude": { "...": "..." }, "codex": { "...": "..." } } }
```

### `GET /api/layout` → 200
The `layout.json` from the config store; missing/corrupt → default.
```json
{ "order": [], "density": { "cards_per_row": 3, "gap": 16 } }
```

### Status code summary
| Code | When |
|---|---|
| 200 | any successful GET above |
| 404 | unknown `/api/*` path → `{"error":"not found"}` |
| 405 | non-GET method on a defined GET route (mux-generated) |
| 500 | unexpected store error not handled by a default fallback → `{"error":"internal error"}` |

`roles`/`projects` chosen as **maps** (keyed by id) to match the on-disk filename-as-id convention and the PRD JSON shapes; `sessions` is an **array** because it is an ordered list of running agents.

---

## 7. Edge cases & error handling

- **`AGENTDECK_HOME` set but path missing:** `EnsureLayout()` creates it (`mkdir -p`). If it points at a file (not dir) → fatal with a clear message.
- **`~` in `AGENTDECK_HOME` or `project.cwd`:** `ExpandTilde` expands a leading `~` / `~/` using `os.UserHomeDir`. A bare `~user` form is not supported → returned unexpanded (documented).
- **Corrupt `config.json`:** log WARN, use default config (port 4317), continue. Do not rewrite the corrupt file.
- **Corrupt single role/project file:** treated as absent; excluded from `List*`; logged once.
- **Empty store (fresh `~/.agentdeck`):** `/api/sessions` → `[]` (empty `state.db`); `/api/roles` & `/api/projects` → seeded (because `start` seeds config before serving); `/api/layout` & `/api/backends` → seeded.
- **`state.db` locked / open failure:** `state.Open` surfaces the error; `dashboard start` exits non-zero with the SQLite error and does not write a pidfile.
- **Migration on an up-to-date DB:** no-op (the `schema_migrations` check finds nothing pending); never rewrites or clobbers existing rows.
- **Port already in use:** `net.Listen` errors; `start` exits non-zero with `address already in use` and does not write a pidfile.
- **Stale pidfile (process dead):** `stop`/`start` detect via signal-0, remove the stale file. `start` refuses to start if a *live* pidfile process exists (prints "already running pid=<n>").
- **Atomic config write interrupted:** temp file remains; never visible under the real name (rename is the commit point); a leftover temp is overwritten/cleaned next write. Temp files use a `.tmp-*` prefix in the same directory so rename stays on one filesystem.
- **`agent_id` collision:** `NewAgentID` retries up to 10× against the `agents` table; after that returns an error (astronomically unlikely with 24 bits of hex).
- **Disk full / permission denied on config write:** `writeJSONAtomic` returns the error; the caller surfaces it (CLI exits non-zero; handler → 500). No partial file is left under the real name.
- **CORS preflight:** middleware answers `OPTIONS` for `/api/*` with the allowed local Vite origin (`http://localhost:5173`) and 204.
- **Non-loopback bind attempt:** the bind assertion in §5.1 fails closed (process exits) — defense in depth even though the address is hard-coded.

---

## 8. Implementation task breakdown

Ordered, each independently verifiable (`go test ./...` and/or `go build`).

1. **Module bootstrap.** `go mod init github.com/agentdeck/agentdeck`; set `go 1.22`; add cobra + pkg/browser + mattn/go-sqlite3; commit `Makefile` with `build/test/ui/run`.
2. **`internal/version`.** `Version/Commit/Date` vars; ldflags wiring in Makefile/install.sh.
3. **`internal/config/types.go`.** Config structs from §4.1. *Verify:* `go vet` clean.
4. **`internal/config/paths.go` + `config.go`.** Home resolution (`AGENTDECK_HOME` → `~/.agentdeck`), `ExpandTilde`, `New()`. *Verify:* unit test resolution + tilde.
5. **`internal/config/atomic.go` + `errors.go`.** `writeJSONAtomic`, `readJSON`, sentinels. *Verify:* write→read round-trip test; corrupt → `ErrCorrupt`.
6. **`internal/config/layout_dirs.go`.** `EnsureLayout()` idempotent mkdir of config dirs (`roles/`, `projects/`, `sessions/`). *Verify:* run twice, no error, no clobber.
7. **Config per-object methods.** `roles.go`, `projects.go`, `backends.go`, `appconfig.go`, `layout.go`. *Verify:* round-trip test per object; `List*` skips corrupt.
8. **`internal/config/seed.go`.** `SeedIfAbsent()` writing the §5.4 set only when absent. *Verify:* seed into temp home, assert files + contents; re-seed after mutating a file → unchanged (no clobber).
9. **`internal/state` schema + open.** `state.go` (`Open` in WAL mode), `schema.go` (migration `0001_base` SQL from §4.5), `migrate.go` (versioned `schema_migrations`). *Verify:* `Open` on a temp home creates `state.db` with the 4 base tables; re-`Open` is a no-op; migration test asserts `schema_migrations` has version 1.
10. **State per-object methods.** `types.go`, `agents.go` (+`NewAgentID`), `running.go`, `status.go`, `messages.go`, `errors.go`. *Verify:* round-trip test per row type (identity/running/status/message) via temp DB; `Read*` missing → `ErrNotFound`; cascade delete.
11. **`internal/server`**: `bind.go` (+ loopback assertion), `json.go`, `middleware.go`, `handlers.go`, `routes.go`, `server.go`, `static.go`. *Verify:* `httptest` against each route; bind-address test.
12. **`internal/cli`**: `root.go` (+ `--version`), `dashboard.go` (start/stop/open), `pidfile.go`, `launch_stub.go`. *Verify:* `agentdeck --version`; start in foreground binds + `/api/health` 200; stop kills via pidfile.
13. **`cmd/agentdeck/main.go`.** Wire cobra root, execute. *Verify:* `go build ./cmd/agentdeck`.
14. **UI scaffold.** `npm create vite ui --template react-ts`; `App.tsx` pings `/api/health`. *Verify:* `cd ui && npm run build` → `ui/dist`.
15. **Embed UI.** `static.go` `//go:embed ui/dist`; SPA fallback; `dev` build tag → disk. *Verify:* `go build` after `ui/dist` exists serves `/`.
16. **`install.sh`.** Build UI (`npm ci && npm run build`), build Go binary with ldflags (cgo enabled for go-sqlite3), install to `~/.local/bin` or `/usr/local/bin`; seed `~/.agentdeck` (config + `state.db`) on first `start`. *Verify:* clean clone → `./install.sh` → `agentdeck --version`.
17. **README prereqs.** Document Go 1.22+, Node 18+/npm (build-time only), quickstart. The shipped artifact has no runtime language dependencies beyond the Go binary and the user's agent CLI.
18. **Full acceptance pass.** Walk the phase-0 §5 checklist end to end.

---

## 9. Testing strategy

All tests set `AGENTDECK_HOME` to a `t.TempDir()` for isolation — no test touches the real `~/.agentdeck`.

**Config store unit tests (`internal/config`):**
- **Round-trip every config object:** for `Role, Project, BackendsConfig, Layout, Config` — construct → `Write*` → `Read*` → `reflect.DeepEqual`. Table-driven.
- **Corrupt-file survival:** write garbage bytes to a role file → `ReadRole` returns `ErrCorrupt` (not panic); `ListRoles` skips it and returns the rest. Use `internal/config/testdata/corrupt_*.json`.
- **`AGENTDECK_HOME` isolation:** set to temp dir; assert `Store.Home()` equals it and all writes land under it; unset → resolves to `~/.agentdeck` (assert path string only, no write).
- **Tilde expansion:** `ExpandTilde("~/x")` == `filepath.Join(home,"x")`; `ExpandTilde("/abs")` unchanged.
- **Atomic write:** after `writeJSONAtomic`, no `.tmp-*` file remains in the dir; content parses.
- **Seed idempotency / no-clobber:** `SeedIfAbsent` twice; mutate `roles/reviewer.json`; re-seed; assert mutation preserved.

**State store unit tests (`internal/state`):**
- **Round-trip every row type:** for `Agent, RunningEntry, Status, Message` — `Open` a temp DB → `Write*` → `Read*` → compare. Identity/running/status keyed by `agent_id`; message by returned id.
- **`Read*` missing row → `ErrNotFound`** (not panic).
- **`List*` on empty DB → `[]`** (not `nil`/`null`).
- **`NewAgentID`:** matches `^a_[0-9a-f]{6}$`; uniqueness across 1000 calls; collision retry covered by pre-inserting an agent row with a known id.
- **Migration test:** `Open` a fresh temp DB; assert the 4 base tables exist and `schema_migrations` contains version 1; `Open` again → still version 1, no error, no data loss (write a row, re-open, read it back).
- **Cascade:** write an agent + its running/status rows; `DeleteAgent`; assert running/status rows gone.

**Server tests (`internal/server`, `httptest`):**
- Each GET route returns the documented status + JSON shape. `/api/sessions` on empty store → `[]` (assert body == `[]`, not `null`).
- `/api/roles` on a seeded temp home → 4 keys.
- Unknown `/api/x` → 404 JSON; `POST /api/health` → 405.
- Corrupt `backends.json` → `/api/backends` still 200 with default.

**Bind-address test (`internal/server`):**
- Start the server on an ephemeral port via the real `Start`/listener path; assert `ln.Addr().(*net.TCPAddr).IP.IsLoopback()`. Negative guard: a unit test calling the assertion helper with a non-loopback addr returns an error / fails closed. Optionally dial a non-loopback interface IP and assert connection refused (best-effort, may skip in CI).

**CLI smoke (table/exec or scripted):**
- `agentdeck --version` prints non-empty version, exit 0.
- `dashboard start --detach` writes pidfile; `dashboard stop` removes it; second `stop` → "not running".

Target: config + state + server packages at meaningful coverage; the mandatory test classes per acceptance criteria are: config round-trip, config corrupt-survival, `AGENTDECK_HOME` isolation, state-store round-trip (identity/running/status/message), the migration test, and the bind test.

---

## 10. Interfaces produced for later phases

**Config file store (`internal/config`)** — the typed substrate every phase reads/writes for human-edited config:
- `*Store` with `New()`, `Home()`, `EnsureLayout()`, `SeedIfAbsent()`.
- Typed CRUD: `Read/Write/List/Delete` for `Role, Project`; `Read/Write` for `BackendsConfig, Config, Layout`.
- `ExpandTilde()` — used by Phase 1 when composing `project.cwd`.
- Sentinels `ErrNotFound`, `ErrCorrupt`.
- Atomic-write guarantee: readers never see partial config files (write-temp-then-rename).

**SQLite state store (`internal/state`)** — the authoritative machine-state substrate (server is sole writer):
- `Open(home)` / `Close()` lifecycle with WAL + versioned migrations.
- Typed CRUD: `Read/Write/List/Delete` for `Agent, RunningEntry, Status`; `Write/List/Delete` for `Message`.
- `NewAgentID()` — stable-id minting used by Phase 1 launch.
- The migration framework (`schema_migrations` + ordered SQL) that Phase 4 extends with the FTS5 index and transcript-metadata tables.
- `DB()` escape hatch for phases that need bespoke queries.

**Server skeleton (`internal/server`)**:
- `New(configStore, stateStore, config, logger)` + `Start(ctx)` lifecycle that Phase 1 extends with `POST` routes and Phase 2 extends with `/api/events` (SSE) and `POST /api/hook`.
- `routes()` registration point (1.22 mux) — later phases register additional patterns here.
- `writeJSON`/`writeError` envelope + CORS/logging middleware reused by all later handlers.
- The loopback bind guarantee (Phase 2 SSE inherits it).
- Embedded static handler with SPA fallback — Phase 2+ UI builds drop into `ui/dist` and ship in the same binary.

**CLI (`internal/cli`)**:
- `dashboard` command group + pidfile lifecycle reused by all phases.
- Reserved `<role>@<project>` parse hook that Phase 1 fills in with real launch.

**Conventions locked:** `~/.agentdeck/` layout (config files + `state.db` + `sessions/`), `AGENTDECK_HOME` override, RFC3339 timestamps, filename-as-id for config (`roles/{id}.json`), `agent_id` as the SQLite primary key for state rows, config-version fields (`config` v1, `backends` v2), and the `schema_migrations` versioning for `state.db`.

---

## 11. Resolved decisions

**Q: Default port?** → **`4317`.** The project default; `config.json.port` is the source of truth, `--port` flag overrides per-run without persisting. 4317 is unassigned by IANA for common dev servers, avoids clashes with Vite (5173), and is baked into the seed.

**Q: Where does machine state live — files or SQLite?** → **SQLite `state.db`** (one file under `~/.agentdeck/`, server is the sole writer); config stays in plain JSON files. Machine state is high-churn and needs transactional, queryable storage with a path to full-text session search (FTS5, Phase 4); config is low-volume, single-writer, and genuinely better hand-edited and `git`-tracked. The reasoning and alternatives are recorded in [architecture-decisions.md](../../architecture-decisions.md).

**Q: Migration tooling for `state.db`?** → **Hand-rolled versioned migrations** (`schema_migrations` table + ordered embedded SQL applied on `Open`). No external migration library; the migration set is small and fully under our control, and Phase 4 appends to it rather than rewriting.

**Q: Embed UI assets in the binary vs. serve from disk?** → **Embed** (`//go:embed ui/dist`). The single-binary distribution promise (master PRD §7) is only honest if the UI ships inside the binary — `install.sh` produces one artifact, `dashboard start` serves the UI with no separate static-file path or web server. A `dev` build tag falls back to serving `ui/dist` from disk so Vite HMR works during development (run `vite dev` on :5173 and hit the Go API via CORS). SPA routes fall back to `index.html`.

**Q: roles/projects shape over the wire — array or map?** → **Map keyed by id**, matching on-disk filename-as-id and PRD JSON. `sessions` stays an array (ordered running list).

**Q: nullable role override.** → `skip_permissions` is `*bool` so `null` (inherit), `true`, and `false` are all distinguishable on disk and over the API.
