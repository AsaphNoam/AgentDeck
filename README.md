# AgentDeck

A local dashboard for launching and orchestrating coding agents (Claude Code,
Codex) from one place. All state lives in plain JSON files under `~/.agentdeck/`;
a Go single binary serves a React UI and a `127.0.0.1`-only REST API.

This repository is at **Phase 0 — Foundation**: the data substrate, file store,
HTTP server skeleton, and CLI. No agents run yet (that is Phase 1+).

## Prerequisites

- **Go 1.22+** — server / single binary
- **Node 18+ and npm** — UI build (and the Phase 5 MCP server)
- **python3** — used by later phases (hooks/runtimes)
- macOS or Linux (the terminal runtime is macOS-only, Phase 6)

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
| `agentdeck <role>@<project>` | reserved launch syntax — prints "not yet implemented" (Phase 1) |

## Layout (`~/.agentdeck/`)

```
agents/{id}.json      stable identity
running/{id}.json     active session registry
status/{id}.json      live state
roles/{role}.json     personas (seeded: implementer, reviewer, researcher, pm)
projects/{p}.json     workspaces (seeded: my-app)
backends.json         providers + models (version 2)
messages/{id}/        per-agent mailbox
sessions/{id}/        transcript history
layout.json           card order + density
config.json           port, defaults (version 1)
```

`AGENTDECK_HOME` overrides `~/.agentdeck/` (used by tests/CI).
`AGENTDECK_LOG_LEVEL` sets the slog level (`debug|info|warn|error`, default `info`).

## REST API (Phase 0, all GET, `127.0.0.1:{port}/api`)

`GET /api/health` · `GET /api/sessions` · `GET /api/roles` · `GET /api/projects`
· `GET /api/backends` · `GET /api/layout`

## Development tasks

```sh
make test   # go test ./...
make vet    # go vet ./...
make ui     # build the UI only
make dist   # full release build
make clean  # remove build artifacts
```
