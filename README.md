# AgentDeck

A local dashboard for launching and orchestrating coding agents (Claude Code,
Codex, OpenCode, and OpenHands) from one place. Human-editable config lives as JSON files under
`~/.agentdeck/`; machine state lives in `state.db`. A Go single binary serves a
React UI and a `127.0.0.1`-only REST API.

Launch a Claude Code or Codex chat agent against any project/role, watch it work on a
live dashboard, resume past sessions, and let agents message each other. Claude Code
can also run in the embedded interactive terminal. A high-level tour of the moving pieces lives
in [architecture-flow.md](architecture-flow.md).

**Status:** Core launch/chat/dashboard/config/archive/messaging/terminal/switch features and native
configuration federation are implemented. Real-provider compatibility has explicit credentialed
acceptance gates; see [the live handoff](docs/features/HANDOFF.md). Contributors start from the
[feature and technical specifications](docs/specs/README.md), not archived phase plans.

## Prerequisites

- **Go 1.25** — server / single binary (authoritative version: `go.mod`)
- **Node 20+ and npm** — UI build only
- macOS or Linux. The default terminal runtime is an embedded xterm.js/PTY bridge;
  tmux is optional and the optional iTerm2 driver is macOS-only.
- At least one authenticated agent CLI. `install.sh` does not install optional ACP adapters unless
  requested (`INSTALL_ACP=1`); chat launch needs the selected adapter on `PATH`.
- `curl` and `jq` for shell-hook integrations used by terminal agents.

## Quickstart

```sh
# Build the UI + binary and install `agentdeck` on PATH
./install.sh

# Start the dashboard (seeds ~/.agentdeck on first run, binds 127.0.0.1:4317)
agentdeck dashboard start

# In another terminal, open the UI
agentdeck dashboard open

# Stop it
agentdeck dashboard stop
```

### Run from source (no install)

```sh
make dist      # build UI, embed it, build ./bin/agentdeck
./bin/agentdeck --version
./bin/agentdeck dashboard start
```

### Development (live UI)

```sh
# Terminal 1: Go API with on-disk UI fallback
go run -tags dev ./cmd/agentdeck dashboard start

# Terminal 2: Vite dev server (proxies /api to :4317)
cd ui && npm ci && npm run dev   # http://localhost:5173
```

## CLI

| Command | Description |
|---|---|
| `agentdeck --version` | print version, commit, build date |
| `agentdeck dashboard start [--port N] [--detach]` | start the server (foreground or backgrounded) |
| `agentdeck dashboard stop` | stop the server via pidfile |
| `agentdeck dashboard open` | open the UI in the default browser |
| `agentdeck <role>@<project> [--backend B] [--model M] [--name N]` | launch an agent (resumes a single inactive match by default; `--new` forces a fresh one) |
| `agentdeck resume <agent_id>` | resume a specific inactive persisted session |
| `agentdeck reindex` | rebuild the archive search index from `sessions/` |

## Layout (`~/.agentdeck/`)

```
roles/{role}.json     personas (seeded: agentdecker, implementer, reviewer, researcher, pm, teammate)
projects/{p}.json     workspaces (seeded: my-app)
backends.json         providers + models (version 2)
config-sources.json   optional Claude/Codex native-config bindings
layout.json           card order + density
config.json           port, defaults (version 1)
state.db              agent identity, running registry, status, messages
sessions/{id}/        normalized transcript + session artifacts
cache/config-sources/ redacted, regenerable federation mirror data
```

`AGENTDECK_HOME` overrides `~/.agentdeck/` (used by tests/CI).
`AGENTDECK_LOG_LEVEL` sets the slog level (`debug|info|warn|error`, default `info`).

### Seeded roles

Seeding is per-file and if-absent: new roles appear on the next `dashboard start`,
and your edits to existing ones are never overwritten. Edit them in Settings or
directly in `roles/{role}.json`.

- **`agentdecker`** — built-in AgentDeck expert. Ask it how anything works
  (launch syntax, config files, switch-runtime, archive, messaging), or hand it
  a goal: it can launch other agents via the `agentdeck` CLI and coordinate
  them over MCP messaging when the selected real CLI passes the credentialed HTTP-MCP
  compatibility gate recorded in the specifications.
- **`implementer` / `reviewer` / `researcher` / `pm`** — the classic worker
  archetypes: ship focused changes with tests, review diffs, investigate before
  acting, break down and track work.
- **`teammate`** — a worker built for multi-agent runs: checks its MCP mail on
  wake, treats coordinator messages as its task queue, and reports outcomes
  back. Pair it with `pm` or `agentdecker` as the coordinator.

## HTTP API (`127.0.0.1:{port}`)

All routes are loopback-only. Browser API routes rely on the loopback/Host/Origin boundary;
hook and MCP producer routes use per-launch tokens. Full surface in
[internal/server/routes.go](internal/server/routes.go).

- **Health/state:** `GET /api/health` · `GET /api/sessions` · `GET /api/archive`
  · `GET /api/capabilities`
- **Session lifecycle:** `POST /api/sessions` (launch) · `GET /api/sessions/{id}`
  · `.../transcript` · `.../files` · `.../commands` · `.../messages` ·
  `POST .../prompt` · `.../cancel` · `.../stop` · `.../rename` · `.../identity`
  · `.../permission` · `.../resume` · `.../switch-runtime`
- **Config CRUD:** `GET/POST /api/roles`, `PUT/DELETE /api/roles/{role}` (same
  shape for `/api/projects`) · `GET/PUT /api/backends` · `GET/PUT /api/config` ·
  `GET/PUT /api/layout`
- **Groups:** `POST /api/groups/{group}/release`
- **Config federation:** `GET /api/config-sources` · `POST .../preview` ·
  `PUT .../{backend_id}` · `POST .../{backend_id}/refresh` · `DELETE .../{backend_id}`
- **Producers / live channels:** `POST /api/hook` (agent lifecycle, token-authed)
  · `GET /api/events` (SSE: `state_update`, `new_message`, `notification`,
  `config_source_update`, `ping`) · `GET /api/sessions/{id}/terminal/ws` (PTY↔WebSocket bridge) ·
  `/mcp` (in-process MCP messaging server)

## Development tasks

```sh
make check-specs # validate the authoritative spec set
make test   # spec lint + both Go variants
make vet    # go vet ./...
make ui     # build the UI only
make dist   # full release build
make clean  # remove build artifacts
```
