# AgentDeck

A local dashboard for launching and orchestrating coding agents (Claude Code,
Codex) from one place. Human-editable config lives as JSON files under
`~/.agentdeck/`; machine state lives in `state.db`. A Go single binary serves a
React UI and a `127.0.0.1`-only REST API.

Launch a Claude Code or Codex agent against any project/role, watch it work on a
live dashboard, chat with it or drop into a real terminal, resume past sessions,
and let agents message each other. A high-level tour of the moving pieces lives
in [architecture-flow.md](architecture-flow.md).

**Status:** Phases 0–5 complete; Phase 6 (terminal runtime, switch-runtime, task
groups) is in progress. See [docs/phases/HANDOFF.md](docs/phases/HANDOFF.md) for
live state. Working today: launch, streaming chat, the state dashboard, config
CRUD & onboarding, archive/search/resume, agent↔agent MCP messaging, the
terminal runtime, and switch-runtime.

## Prerequisites

- **Go 1.22+** — server / single binary
- **Node 18+ and npm** — UI build only
- macOS or Linux. The default terminal runtime (embedded xterm.js / tmux) is
  cross-platform; the optional iTerm2 driver is macOS-only.
- At least one authenticated agent CLI (`claude-code-acp` for chat; the real
  `claude`/`codex` CLI for the terminal interface).

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
cd ui && npm install && npm run dev   # http://localhost:5173
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
layout.json           card order + density
config.json           port, defaults (version 1)
state.db              agent identity, running registry, status, messages
sessions/{id}/        transcript history
```

`AGENTDECK_HOME` overrides `~/.agentdeck/` (used by tests/CI).
`AGENTDECK_LOG_LEVEL` sets the slog level (`debug|info|warn|error`, default `info`).

## HTTP API (`127.0.0.1:{port}`)

All routes are loopback-only and unauthenticated. Full surface in
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
- **Producers / live channels:** `POST /api/hook` (agent lifecycle, token-authed)
  · `GET /api/events` (SSE: `state_update`, `new_message`, `notification`,
  `ping`) · `GET /api/sessions/{id}/terminal/ws` (PTY↔WebSocket bridge) ·
  `/mcp` (in-process MCP messaging server)

## Development tasks

```sh
make test   # go test ./...
make vet    # go vet ./...
make ui     # build the UI only
make dist   # full release build
make clean  # remove build artifacts
```
