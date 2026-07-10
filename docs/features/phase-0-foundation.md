# Phase 0 — Foundation: data model, stores, server & CLI skeleton

**Status:** ready to build
**Features:** none directly (substrate for all of F1–F13)
**Depends on:** nothing
**Enables:** every subsequent phase

---

## 1. Goal

Stand up the skeleton everything else hangs off of: the data model under `~/.agentdeck/` (config as JSON files, machine state in a SQLite `state.db`), the typed store layer that reads/writes both, a Go server binary that binds `127.0.0.1`, the `agentdeck` CLI entrypoint, and a build/install path. No agents run yet. This phase exists because the master PRD's "core loop" milestone silently assumes all of this is present.

When this phase is done you can build, install, and start the server; it serves a health endpoint and an empty `GET /sessions`; and the stores round-trip every core object with tests.

---

## 2. Scope

### In scope
- Repo structure for a Go single-binary server (hosts the in-process MCP server and embeds the UI) + React/Vite UI (build wiring, not the features).
- `~/.agentdeck/` layout creation: config directories/files + the SQLite `state.db` (schema migrations applied on open).
- A typed **config file store** (roles, projects, backends, config, layout) and a typed **SQLite state store** (agent identity, running registry, status, messages) — both over the data model in §3 of the master PRD.
- Go HTTP server skeleton bound to `127.0.0.1:{port}` with health + empty `GET /sessions`/`GET /roles`/`GET /projects`/`GET /backends` returning seeded/empty data.
- `agentdeck` CLI skeleton: `dashboard start`, `dashboard stop`, `dashboard open`, `--version`.
- `install.sh` that builds the binary (UI embedded) and installs the CLI.
- Seed data: default `backends.json`, seed roles (`implementer`, `reviewer`, `researcher`, `pm`), example project.

### Out of scope (later phases)
- Any runtime, ACP, agent launching (Phase 1).
- SSE bus, `POST /api/hook` ingest (Phase 2).
- Config editing UI (Phase 3).
- FTS5 search indexing (Phase 4) — Phase 0 only creates the `state.db` and its base tables.

---

## 3. On-disk layout to create

Create lazily on first run if absent; never overwrite existing user data.

```
~/.agentdeck/
  # config — plain JSON files (hand-editable, git-friendly)
  roles/{role}.json           role definitions         (seed 4)
  projects/{project}.json     project definitions       (seed 1 example)
  backends.json               provider + model config   (seed default)
  layout.json                 dashboard card order + density
  config.json                 port, default_project, default_role, skip_permissions

  # state — SQLite (server is sole writer); base tables created/migrated on open
  state.db                    agent identity, running registry, status, messages
                              (+ session/transcript metadata & FTS5 index added in Phase 4)

  # agent-CLI-owned transcripts (populated from Phase 1 on)
  sessions/{agent_id}/        transcript history
```

`config.json` seed:
```jsonc
{
  "version": 1,
  "port": 4317,
  "default_project": "my-app",
  "default_role": "implementer",
  "skip_permissions": false
}
```

Use the exact schemas from master PRD §3: `roles/`, `projects/`, `backends.json` are JSON files; agent identity, running registry, and status are rows in `state.db` (the JSON shapes in §3 describe their logical columns).

---

## 4. Detailed requirements

### 4.1 Config file store (Go)
- Typed structs for Role, Project, BackendsConfig, Layout, Config matching master PRD §3.
- `Read`, `Write`, `List`, `Delete` per object; atomic writes (write-temp-then-rename) so readers never see partial files.
- Path resolution honoring `~` expansion and an `AGENTDECK_HOME` override env var (critical for tests and CI).
- Graceful handling of missing/corrupt files: corrupt file → logged, treated as absent/default, never crashes the server.

### 4.2 SQLite state store (Go)
- `state.db` opened in WAL mode; schema created and migrated on open (versioned).
- Typed structs for Agent identity, RunningEntry, Status, Message matching master PRD §3.
- `Read`/`Write`/`List`/`Delete` per object as SQL operations; the **server is the sole writer**.
- `agent_id` generation (`a_` + short random hex, stable, collision-checked).
- Phase 0 creates the base tables (identity, running, status, messages); the FTS5 search index and transcript-metadata tables are added in Phase 4.

### 4.3 Server skeleton (Go)
- Binds `127.0.0.1:{config.port}` only — assert the bind address is never `0.0.0.0`.
- Routes (all under `/api`): `GET /health`, `GET /sessions`, `GET /roles`, `GET /projects`, `GET /backends`, `GET /layout`. Sessions/identity/status come from `state.db`; roles/projects/backends/layout from the config store (empty/seeded as appropriate).
- Structured logging; clean shutdown on SIGINT/SIGTERM.
- CORS allowing the local Vite dev origin.

### 4.4 CLI skeleton
- `agentdeck dashboard start` — launches the server (foreground + a `--detach` background mode writing a pidfile).
- `agentdeck dashboard stop` — stops via pidfile.
- `agentdeck dashboard open` — opens the UI URL in the default browser.
- `agentdeck --version`.
- Reserve the `agentdeck <role>@<project>` launch syntax (no-op stub returning "not yet implemented" — implemented in Phase 1).

### 4.5 Build & install
- `install.sh`: build the UI bundle, embed it, build the Go binary, install `agentdeck` onto PATH, create `~/.agentdeck/` (config seed + `state.db`) on first run.
- Prereqs: Go 1.22+ and Node 18+/npm for source builds (Node is build-time only). No runtime Node, no python3.

---

## 5. Acceptance criteria

- [ ] `./install.sh` produces an `agentdeck` binary (UI embedded) with no manual steps.
- [ ] `agentdeck dashboard start` binds `127.0.0.1:4317`; an external interface cannot reach it.
- [ ] First start creates `~/.agentdeck/` with the config files (seeded `backends.json`, 4 seed roles, example project, `config.json`) and a `state.db` with the base tables.
- [ ] `GET /api/health` returns 200; `GET /api/roles` returns the 4 seed roles; `GET /api/sessions` returns `[]`.
- [ ] Config-store unit tests round-trip every config object and survive a deliberately corrupted file without crashing; state-store tests round-trip identity/running/status/message rows.
- [ ] Re-running start does not clobber existing user data (config files or `state.db`).

---

## 6. Open questions
- Default port choice (PRD leaves `{port}` abstract; using 4317 as a placeholder — confirm).
- Single binary embedding the UI assets vs. serving from disk? (Recommend embedding for distribution simplicity.)
- `state.db` migration tooling: hand-rolled versioned migrations vs. a small library — decide before Phase 4 grows the schema.
