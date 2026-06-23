# Phase 0 — Foundation: data model, file store, server & CLI skeleton

**Status:** ready to build
**Features:** none directly (substrate for all of F1–F13)
**Depends on:** nothing
**Enables:** every subsequent phase

---

## 1. Goal

Stand up the skeleton everything else hangs off of: the on-disk data model under `~/.agentdeck/`, a typed file-store layer that reads/writes the four core objects, a Go server binary that binds `127.0.0.1`, the `agentdeck` CLI entrypoint, and a build/install path. No agents run yet. This phase exists because the master PRD's "core loop" milestone silently assumes all of this is present.

When this phase is done you can build, install, and start the server; it serves a health endpoint and an empty `GET /sessions`; and the file store round-trips every core object with tests.

---

## 2. Scope

### In scope
- Repo structure for a Go single-binary server + React/Vite UI + Node MCP server (folders + build wiring, not the features).
- `~/.agentdeck/` directory layout creation and a versioned `config.json`.
- A typed Go file-store package over the layout in §3 of the master PRD.
- Go HTTP server skeleton bound to `127.0.0.1:{port}` with health + empty `GET /sessions`/`GET /roles`/`GET /projects`/`GET /backends` returning seeded/empty data.
- `agentdeck` CLI skeleton: `dashboard start`, `dashboard stop`, `dashboard open`, `--version`.
- `install.sh` that builds the binary + UI and installs the CLI.
- Seed data: default `backends.json`, seed roles (`implementer`, `reviewer`, `researcher`, `pm`), example project.

### Out of scope (later phases)
- Any runtime, ACP, agent launching (Phase 1).
- SSE, file watching (Phase 2).
- Config editing UI (Phase 3).

---

## 3. On-disk layout to create

Create lazily on first run if absent; never overwrite existing user data.

```
~/.agentdeck/
  agents/{agent_id}.json      stable identity
  running/{agent_id}.json     active session registry
  status/{agent_id}.json      live state
  roles/{role}.json           role definitions        (seed 4)
  projects/{project}.json     project definitions      (seed 1 example)
  backends.json               provider + model config  (seed default)
  messages/{agent_id}/        per-agent mailbox
  sessions/{agent_id}/        transcript history
  layout.json                 dashboard card order + density
  config.json                 port, default_project, default_role, skip_permissions
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

Use the exact schemas from master PRD §3 for `agents/`, `running/`, `status/`, `roles/`, `projects/`, `backends.json`.

---

## 4. Detailed requirements

### 4.1 File-store package (Go)
- Typed structs for Agent identity, RunningEntry, Status, Role, Project, BackendsConfig, Layout, Config matching master PRD §3.
- `Read`, `Write`, `List`, `Delete` per object type; atomic writes (write-temp-then-rename) to avoid partial files readers might pick up later.
- Path resolution honoring `~` expansion and an `AGENTDECK_HOME` override env var (critical for tests and CI).
- ID generation for `agent_id` (`a_` + short random hex, stable, collision-checked).
- Graceful handling of missing/corrupt files: corrupt file → logged error, treated as absent, never crashes the server.

### 4.2 Server skeleton (Go)
- Binds `127.0.0.1:{config.port}` only — assert the bind address is never `0.0.0.0`.
- Routes (all under `/api`): `GET /health`, `GET /sessions`, `GET /roles`, `GET /projects`, `GET /backends`, `GET /layout`. Return data sourced from the file store (empty arrays where nothing exists yet).
- Structured logging; clean shutdown on SIGINT/SIGTERM.
- CORS allowing the local Vite dev origin.

### 4.3 CLI skeleton
- `agentdeck dashboard start` — launches the server (foreground + a `--detach` background mode writing a pidfile).
- `agentdeck dashboard stop` — stops via pidfile.
- `agentdeck dashboard open` — opens the UI URL in the default browser.
- `agentdeck --version`.
- Reserve the `agentdeck <role>@<project>` launch syntax (no-op stub returning "not yet implemented" — implemented in Phase 1/4).

### 4.4 Build & install
- `install.sh`: build Go binary, build UI bundle, install `agentdeck` onto PATH, create `~/.agentdeck/` with seed data on first run.
- Document prereqs from master PRD §7 (Go 1.22+, Node 18+, npm, python3).

---

## 5. Acceptance criteria

- [ ] `./install.sh` produces an `agentdeck` binary and a built UI with no manual steps.
- [ ] `agentdeck dashboard start` binds `127.0.0.1:4317`; an external interface cannot reach it.
- [ ] First start creates `~/.agentdeck/` with all directories, a seeded `backends.json`, 4 seed roles, and `config.json`.
- [ ] `GET /api/health` returns 200; `GET /api/roles` returns the 4 seed roles; `GET /api/sessions` returns `[]`.
- [ ] File-store unit tests round-trip every core object and survive a deliberately corrupted file without crashing.
- [ ] Re-running start does not clobber existing user data.

---

## 6. Open questions
- Default port choice (PRD leaves `{port}` abstract; using 4317 as a placeholder — confirm).
- Single binary embedding the UI assets vs. serving from disk? (Recommend embedding for distribution simplicity.)
