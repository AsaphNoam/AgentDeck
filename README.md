# Chuck

A local dashboard for launching and orchestrating coding agents (Claude Code,
Codex, OpenCode, and OpenHands) from one place. Human-editable config lives as JSON files under
`~/.chuck/`; machine state lives in `state.db`. A Go single binary serves a
React UI and a `127.0.0.1`-only REST API.

Launch a Claude Code or Codex chat agent against any project/role, watch it work on a
live dashboard, resume past sessions, and let agents message each other. Claude Code
can also run in the embedded interactive terminal. A high-level tour of the moving pieces lives
in [architecture-flow.md](architecture-flow.md).

**Status:** Core launch/chat/dashboard/config/archive/messaging/terminal/switch features and native
configuration federation are implemented. Real-provider compatibility has explicit credentialed
acceptance gates; see [the live handoff](docs/features/HANDOFF.md). Contributors start from the
[feature and technical specifications](docs/specs/README.md), not archived phase plans.
New product ideas belong in [ideas and improvements](docs/ideas.md). Once specified and approved to
start, a change lives in [ready changes](docs/ready-changes/README.md).

## Install a macOS release

Chuck was previously called AgentDeck. An existing AgentDeck installation is not upgraded in place;
move its data with the supervised [cutover guide](docs/chuck-cutover.md).

Chuck releases currently support **Apple-silicon Macs only**. They include the Chuck binary,
private Node runtime, pinned Claude/Codex ACP adapters, and one bundled copy each of Claude Code and
Codex—no repository checkout, Go, Node, npm, Homebrew, administrator access, or global adapter
installation is needed.

By default every Claude or Codex backend runs **your installed** Claude Code or Codex, so updating it
(`claude update`, `npm i -g @openai/codex`, Homebrew…) makes newer models usable at the next agent
start without a Chuck release. Install them from the official
[Claude Code](https://code.claude.com/docs/en/setup) or [Codex](https://developers.openai.com/codex/cli)
instructions. In **Settings → Backends** a backend can instead choose the **Chuck bundle**, which
changes only when Chuck updates. Chuck never installs, updates, or switches providers on its
own; neither choice promises new models, account access, or that a native session can move back to an
older provider version.

```sh
curl -fsSL https://github.com/AsaphNoam/Chuck/releases/latest/download/install.sh | bash
```

Pass `--version X.Y.Z` to select a release, `--no-start` to install without launching the dashboard,
or `--non-interactive` for scripts. The installer verifies the release archive against its published
SHA-256 manifest before activation. It may offer to add its one command directory to your zsh profile
and to sign in to Claude (skipped with guidance when no Claude Code is installed); it never collects
credentials itself. After installation, use:

```sh
chuck auth claude       # or: chuck auth codex
chuck dashboard start --detach
chuck dashboard open
chuck update --check
chuck update            # asks before downloading
chuck update --yes      # non-interactive update
chuck update --rollback
```

`chuck update` changes Chuck and its bundled copies only; it never touches your installed
providers, and each backend keeps its provider choice. `--rollback` restores the previous Chuck
release, not a provider version or native session format. A release from before this provider choice
(0.8.x and earlier) always ran its bundled providers, so rolling back to one restores that behavior.

Release artifacts are intentionally **not code-signed or notarized**. macOS may ask you to approve
an unidentified developer on first open. Do not bypass Gatekeeper or enter an administrator password
for Chuck—the installer never asks for either. Published checksums detect corruption, but without
signing they do not independently authenticate a compromised release account or manifest.

## Prerequisites

- **Go 1.26.6** — server / single binary (authoritative version: `go.mod`)
- **Node 20+ and npm** — UI build only; Node 22+ is required at runtime when source-installing the
  optional official Claude ACP adapter with `INSTALL_ACP=1`
- macOS or Linux. The default terminal runtime is an embedded xterm.js/PTY bridge;
  tmux is optional and the optional iTerm2 driver is macOS-only.
- At least one authenticated agent backend. `install.sh` installs the pinned official Claude ACP
  adapter only when requested (`INSTALL_ACP=1`); chat launch needs the selected adapter on `PATH`
  and an installed Claude Code or Codex. Source builds have no Chuck bundle.
- `curl` and `jq` for shell-hook integrations used by terminal agents.

## Quickstart

```sh
# Build the UI + binary and install `chuck` on PATH
./install.sh

# Start the dashboard (seeds ~/.chuck on first run, binds 127.0.0.1:4317)
chuck dashboard start

# In another terminal, open the UI
chuck dashboard open

# Stop it
chuck dashboard stop
```

### Run from source (no install)

```sh
make dist      # build UI, embed it, build ./bin/chuck
./bin/chuck --version
./bin/chuck dashboard start
```

### Development (live UI)

```sh
# Terminal 1: Go API with on-disk UI fallback
go run -tags dev ./cmd/chuck dashboard start

# Terminal 2: Vite dev server (proxies /api to :4317)
cd ui && npm ci && npm run dev   # http://localhost:5173
```

## CLI

| Command | Description |
|---|---|
| `chuck --version` | print version, commit, build date |
| `chuck dashboard start [--port N] [--detach]` | start the server (foreground or backgrounded) |
| `chuck dashboard stop` | stop the server via pidfile |
| `chuck dashboard open` | open the UI in the default browser |
| `chuck auth <claude\|codex> [--backend ID] [--model ID]` | sign in with the provider that backend/model launches (default backend otherwise) |
| `chuck update [--check\|--yes\|--rollback]` | explicitly check, install, or roll back a release |
| `chuck <role>@<project> [--backend B] [--model M] [--name N]` | launch an agent (resumes a single inactive match by default; `--new` forces a fresh one) |
| `chuck resume <agent_id>` | resume a specific inactive persisted session |
| `chuck reindex` | rebuild the archive search index from `sessions/` |

## Layout (`~/.chuck/`)

```
roles/{role}.json     personas (seeded: chucky, implementer, reviewer, researcher)
projects/{p}.json     workspaces (seeded: my-app)
backends.json         providers + models (version 2)
config-sources.json   optional Claude/Codex native-config bindings
layout.json           card order + density
config.json           port, defaults (version 1)
state.db              agent identity, running registry, status, messages
sessions/{id}/        normalized transcript + session artifacts
cache/config-sources/ redacted, regenerable federation mirror data
cache/agent-skills/   product-managed operating skill, republished and verified at each start
```

`CHUCK_HOME` overrides `~/.chuck/` (used by tests/CI).
`CHUCK_LOG_LEVEL` sets the slog level (`debug|info|warn|error`, default `info`).
Every `dashboard start` appends structured application logs to
`$CHUCK_HOME/dashboard.log`, including foreground starts; foreground logs are also shown in the
terminal. To collect the latest records for debugging on another computer, run:

```sh
tail -n 500 "${CHUCK_HOME:-$HOME/.chuck}/dashboard.log" > chuck-dashboard.log
```

Review the resulting `chuck-dashboard.log`, then attach or send that file with the bug report.

### Seeded roles

Seeding is per-file and if-absent: new roles appear on the next `dashboard start`,
and your edits to existing ones are never overwritten. Edit them in Settings or
directly in `roles/{role}.json`.

- **`chucky`** — built-in Chuck expert and coordinator. Ask it how anything works
  (launch syntax, config files, switch-runtime, archive, messaging), or hand it
  a goal: it can launch other agents via the `chuck` CLI and coordinate
  them over MCP messaging when the selected real CLI passes the credentialed HTTP-MCP
  compatibility gate recorded in the specifications.
- **`implementer`** — completes a requested change within existing conventions and reports the
  verification it actually ran. It is the default role.
- **`reviewer`** — assesses work against its requirements and reports evidence-backed findings.
- **`researcher`** — investigates code, specifications and history, or external documentation,
  and reports sourced findings.

Every role, including one you write yourself, also receives a short Chuck operating context at
launch. Homes created before these four roles keep any `pm` or `teammate` role files as ordinary
editable roles; Chuck no longer creates them.

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
