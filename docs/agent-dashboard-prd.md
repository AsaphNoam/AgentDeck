# Product Requirements Document — Local Multi-Agent Dashboard

**Working name:** AgentDeck (rename freely)
**Document type:** Feature-focused PRD intended to brief a coding agent and to serve as a reference for the product's full feature set.
**Status:** buildable spec. Architecture decisions and their rationale are recorded in [architecture-decisions.md](architecture-decisions.md); this PRD states the resulting design as fact.

---

## 1. Summary

AgentDeck is a **local-first desktop tool for running and supervising many AI coding-agent sessions in parallel**. It wraps existing agent CLIs (Claude Code, OpenAI Codex) and gives every session a persistent identity, live status, full chat history, file/command tracking, and a messaging channel so agents can coordinate with each other. Everything runs on `localhost`; all data is local (config as plain files, machine state in a local SQLite database); there is no cloud component and no account.

The product is for a developer who delegates several concurrent tasks to AI agents and needs one place to see what each is doing, intervene when needed, and resume past work — without juggling a dozen terminal tabs.

---

## 2. Goals and non-goals

### Goals
- Run N agent sessions concurrently, each addressable as `role@project`.
- Show live status of every agent at a glance (busy / idle / waiting / done).
- Provide a full streaming chat view per agent with tool calls, diffs, and permission prompts.
- Persist every session and allow search + resume with full history.
- Let agents message each other programmatically (no manual copy-paste relay).
- Support multiple backends/models, switchable on a live agent without losing history.
- Keep all data local; bind the server to `127.0.0.1` only.

### Non-goals (v1)
- No cloud sync, no remote/multi-user access, no auth layer.
- No built-in code editor — the tool observes and orchestrates, it does not replace the IDE.
- No support for agent runtimes that are not CLI/ACP-compatible.
- No billing, telemetry, or analytics.

---

## 3. Core concepts and data model

These four objects are the backbone. Everything in the UI is a view over them.

### 3.1 Agent (session)
A running or historical session. Has a **stable identity** that survives resume, clone, and backend swaps, separate from the **ephemeral runtime session id** assigned by the underlying CLI.

The four backbone objects below are shown as JSON for their logical shape. Roles and projects are stored as config files; agent identity, the running registry, and live status are rows in `state.db` (see §3.5).

```jsonc
// agent identity — state.db (logical shape shown as JSON)
{
  "agent_id": "a_8f3c12",        // stable, never changes
  "name": "Atlas",                // human-friendly display name, user-editable
  "role": "implementer",          // references a role definition
  "project": "my-app",            // references a project definition
  "backend": "claude",            // references a backend
  "model": "sonnet-4-6",
  "interface": "chat",            // "chat" | "terminal"
  "created_at": "2026-06-22T10:00:00Z",
  "group": "auth-migration"       // optional task-group label
}
```

```jsonc
// running registry — state.db (row removed when stopped)
{
  "agent_id": "a_8f3c12",
  "pid": 48213,                   // process group id of the CLI
  "session_id": "claude-sess-xyz",// ephemeral, changes on fork/resume
  "interface": "chat",
  "tty": "/dev/ttys004",          // only for terminal interface
  "started_at": "2026-06-22T10:00:01Z"
}
```

```jsonc
// live status — state.db (written by the server from hook POSTs / the ACP stream)
{
  "agent_id": "a_8f3c12",
  "state": "busy",                // "busy" | "idle" | "waiting_input" | "done" | "error"
  "detail": "Editing src/auth.ts",
  "last_trace": "PostToolUse: Edit",
  "busy_since": "2026-06-22T10:04:11Z",
  "context_pct": 0.42             // context window usage 0..1
}
```

### 3.2 Role
A reusable persona: system prompt + display metadata + permission policy. Defines *how* an agent behaves, independent of where it works.

```jsonc
// roles/{role}.json
{
  "title": "Reviewer",
  "system_prompt": "Review changes for correctness, edge cases, and consistency.",
  "skip_permissions": null         // null = inherit global; true/false = override
}
```
Seed roles to ship (written only if absent — `SeedIfAbsent` never overwrites user edits, so existing installs pick up new roles without losing changes to old ones):

| Role | Purpose |
|------|---------|
| `agentdecker` | Built-in AgentDeck expert. Teaches the product's features (launch syntax, config files, dashboard, switch-runtime, archive, messaging) and orchestrates multi-agent workflows — it can launch agents itself via the `agentdeck` CLI and drive them over MCP messaging. |
| `implementer` | Ships code changes: focused diffs, tests, verification before declaring done. |
| `reviewer` | Reviews diffs for correctness, edge cases, and consistency; reports findings rather than rewriting. |
| `researcher` | Investigates and summarizes; gathers context and evidence before proposing actions. |
| `pm` | Breaks work down, assigns and tracks it across agents. |
| `teammate` | Messaging-fluent worker for coordinated multi-agent runs: checks its MCP mail on wake (the nudger's wake-up carries no instruction), treats coordinator messages as its task queue, reports outcomes back, and respects the per-turn message budget. |

### 3.3 Project
A reusable workspace: working directory + injected context + extra dirs + display metadata. Defines *where* and *on what* an agent works.

```jsonc
// projects/{project}.json
{
  "title": "My App",
  "color": [100, 180, 255],        // display accent
  "cwd": "~/Projects/my-app",
  "add_dirs": [],                  // extra directories the agent may access
  "context_prompt": "Project-specific context injected into every agent here."
}
```

### 3.4 Backend
A provider runtime. Each backend exposes multiple models, each optionally with its own API key/endpoint.

```jsonc
// backends.json
{
  "version": 2,
  "backends": {
    "claude": {
      "name": "Claude",
      "type": "claude-acp",
      "default": true,
      "default_model": "sonnet-4-6",
      "models": {
        "sonnet-4-6": { "name": "Sonnet 4.6", "model": "claude-sonnet-4-6" },
        "opus-4-7":   { "name": "Opus 4.7",   "model": "claude-opus-4-7" }
      }
    },
    "codex": {
      "name": "Codex",
      "type": "codex-acp",
      "default_model": "gpt-5.5",
      "models": {
        "gpt-5.5": { "name": "GPT 5.5", "model": "gpt-5.5" },
        "gpt-4o":  { "name": "GPT-4o",  "model": "gpt-4o",
                     "env": { "OPENAI_API_KEY": "sk-...", "OPENAI_BASE_URL": "https://..." } }
      },
      "env": { "CODEX_HOME": "/path/to/sessions" }
    }
  }
}
```
Backend-level `env` applies to all its models; per-model `env` overrides it. Composition at launch: `project.cwd` + `project.context_prompt` + `role.system_prompt` + `backend/model` → CLI invocation.

### 3.5 On-disk layout (storage split by writer)

Storage is split by *who writes the data*: human-edited **config is plain JSON files**; machine-generated **state lives in SQLite** (the Go server is the sole writer). Fully local-first: one SQLite file, no server process, no cloud.

```
~/.agentdeck/
  # --- Human-edited config: plain JSON files (git-friendly, hand-editable) ---
  roles/{role}.json           role definitions
  projects/{project}.json     project definitions
  backends.json               provider + model config
  layout.json                 dashboard card order + density
  config.json                 port, default_project, default_role, skip_permissions

  # --- Machine-generated state: SQLite (server is sole writer) ---
  state.db                    SQLite: agent identity, running registry, live status,
                              messages, session/transcript metadata + FTS5 search index

  # --- Agent-CLI-owned transcripts (written by Claude Code / Codex), indexed into state.db ---
  sessions/{agent_id}/        raw transcript history for resume (source for the FTS5 index)
```

Rationale for the split: config is tiny, rarely written, and genuinely better as hand-editable / `git`-tracked text; state is queried, searched, and machine-written at high frequency, where SQLite's transactions and FTS5 win. Because the server is the only writer to `state.db`, there is no multi-process contention and the DB is authoritative (no derived-index drift).

---

## 4. System architecture

Two runtime processes (plus the agent CLIs), all local. The messaging MCP server is hosted in-process in the Go binary (no Node at runtime); hooks report to the server over localhost HTTP (+ per-launch token).

```
┌────────────────────────────────────────────────────────────┐
│ WEB DASHBOARD  (React + Vite, runs in local browser)        │
│  Agent cards · Chat panel · Session archive · Settings      │
└───────────────┬────────────────────────────────────────────┘
                │ REST (commands) + SSE (live state stream)
                ▼
┌────────────────────────────────────────────────────────────┐
│ LOCAL SERVER  (Go single binary, binds 127.0.0.1)          │
│  • SSE event bus (per-client buffer, drop-oldest, keepalive)│
│  • State manager (SQLite state.db; server is sole writer)   │
│       └─ reconciliation watcher over sessions/ as fallback  │
│  • Hook ingest endpoint  POST /api/hook (token-authed)      │
│  • Runtime registry (dispatches by agent.interface)         │
│       ├─ Chat runtime  → ACP JSON-RPC / NDJSON over stdio   │
│       └─ Terminal runtime → drives a terminal emulator      │
│  • Nudger (wakes idle agents that have pending messages)    │
│  • In-process MCP messaging server (Go MCP SDK, stdio)      │
└───────────────┬────────────────────────────────────────────┘
                │ both runtimes wrap the same agent CLI
                ▼
┌────────────────────────────────────────────────────────────┐
│ AGENT CLI  (Claude Code / Codex)                            │
│  launched with: resume <session_id>, system-prompt append,  │
│  cwd, model, MCP server registration                        │
│  hooks fire on lifecycle events → POST /api/hook (+ token)  │
└────────────────────────────────────────────────────────────┘
```

### 4.1 Runtime abstraction
Define a `Runtime` interface the server programs against, with two implementations:

- **Chat runtime (default):** speaks ACP (Agent Communication Protocol) to the CLI over stdio as JSON-RPC / NDJSON. Streams the transcript (assistant text, tool calls, tool results, diffs, permission requests) back to the server, which republishes over SSE.
- **Terminal runtime:** launches the CLI inside a real terminal emulator (e.g. iTerm2 on macOS via AppleScript), writing prompts to the TTY and managing tab title/color/focus. Status comes from hooks rather than the ACP stream.

The registry dispatches each agent to a runtime based on `agent.interface`. Both wrap the **same** CLI and the **same** stable identity, which is what makes interface/backend/model switching non-destructive.

Interface methods (minimum): `Start(agent)`, `SendPrompt(agent, text)`, `Cancel(agent)`, `Stop(agent)`, `Resume(agent, session_id)`, `CheckMessages(pid)`.

### 4.2 State manager
The state manager owns `state.db` (agent identity, running registry, live status, messages, session/transcript metadata + FTS5 index) and is the **sole writer**. Status production is decoupled from consumption via two ingest paths: the **hook ingest endpoint** (`POST /api/hook`, the primary channel — see §4.4) and the **chat runtime** (which derives status from the ACP stream directly). A reconciliation watcher over `sessions/` exists only as a fallback to pick up transcript files written out-of-band by the agent CLI. Every applied change emits an SSE `state_update`.

### 4.3 SSE event bus
One event stream per connected browser client. Per-client bounded buffer with drop-oldest backpressure, plus a periodic keepalive ping (~10s). Event types:
- `state_update` — an agent's status/identity changed.
- `new_message` — chat transcript delta for an agent.
- `notification` — agent finished / needs input / permission required.
- `ping` — keepalive.

### 4.4 Hooks
Lifecycle hook scripts registered with the agent CLI. They fire on `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`, and **POST the event to `POST /api/hook`** including the per-launch token they were given at start. The server validates the token and applies the update to `state.db`. Hooks stay thin (a small shell `curl`), keeping the channel language-agnostic. Terminal-interface agents rely on hooks for status; chat-interface agents derive most status from the ACP stream and may skip redundant hook POSTs (gate by interface).

### 4.5 MCP messaging server
An **in-process MCP server** registered with each launched agent (stdio), exposing three tools:
- `list_agents` — discover other live agents (name, role, project, state) — an in-process read of `state.db`.
- `send_message(to, body)` — enqueue a message for the recipient (messages table in `state.db`).
- `check_messages` — read + flag/delete the caller's pending messages.

Delivery model: messages are rows in `state.db` (server is sole writer). Delivery happens either by the recipient agent polling `check_messages`, or by the **Nudger** — a server loop that detects an idle agent with pending mail and wakes it (calls `Runtime.CheckMessages(pid)`) so it processes the message without user intervention. Apply a per-turn message budget (e.g. 15 messages/turn) to prevent runaway agent-to-agent loops. Because the MCP server shares the server's process and state, `list_agents`/messaging are in-process operations with no serialization boundary.

---

## 5. Feature requirements

Each feature below is written so it can be built and verified independently.

### F1 — Multi-agent dashboard (card grid)
**What:** The home view lists every active agent as a card. Card shows: name, role, project, backend/model, live state badge (color-coded), context-usage indicator, and a preview of the most recent output line.
**Behavior:**
- Cards update in real time from SSE `state_update`.
- Cards are drag-reorderable; order + density (cards per row, gap) persist to `layout.json`.
- Right-click a card → context menu: Open chat, Switch runtime, Rename, Clone, Stop, Move to group.
**Acceptance:** Launching an agent adds a card within 1s; status badge reflects busy/idle/done live; reload preserves card order.

### F2 — Task groups
**What:** Agents can be assigned a `group` label (e.g. "auth-migration"). The dashboard renders groups as collapsible sections.
**Behavior:** Collapse/expand persists. A group can be "released" (all its agents stopped) in one action.
**Acceptance:** Creating two agents in the same group renders them under one collapsible header; releasing the group stops all of them.

### F3 — Streaming chat panel
**What:** Clicking a card opens the full conversation for that agent.
**Behavior:**
- Renders assistant markdown, tool calls (with arguments), tool results, file diffs, and inline permission prompts (Approve / Deny) sourced from the ACP stream.
- User can send a prompt from the panel; streams the response token-by-token via SSE.
- Cancel button interrupts the current turn (`/cancel`).
- Shows context-window usage and current model.
**Acceptance:** Sending a prompt streams output incrementally; a tool call that needs permission surfaces an Approve/Deny control that gates execution.

### F4 — Launch an agent
**What:** "New Agent" flow (modal + CLI).
**Behavior:**
- Modal fields: name (auto-suggested), role, project, backend, model, interface.
- CLI equivalent: `agentdeck implementer@my-app`, `agentdeck reviewer@my-app --backend codex`.
- On launch, server composes config (cwd, context, system prompt, model), starts the runtime, registers the MCP messaging server, writes identity + running files.
**Acceptance:** Both modal and CLI produce an identical running agent with a card and an open-able chat.

### F5 — Projects & roles management
**What:** CRUD for projects and roles via Settings UI and/or direct JSON edit.
**Behavior:** Editing a role/project changes future launches; existing agents keep their composed config until restarted. Files live in `roles/` and `projects/`.
**Acceptance:** Adding a custom role makes it selectable in the New Agent modal without restart.

### F6 — Backend & model configuration
**What:** Settings UI over `backends.json`. Per-model API key + endpoint overrides.
**Behavior:** Validate credentials on save where possible. Mark one backend default; mark one model default per backend.
**Acceptance:** Configuring a second model with a custom endpoint routes that model's calls to the override URL.

### F7 — Switch runtime on a live agent
**What:** Change interface (chat ↔ terminal), backend (Claude ↔ Codex), or model on a running agent, preserving conversation history.
**Behavior:** Right-click → Switch runtime. Server stops the current runtime, re-launches with new params using the stable `agent_id` and resumes the session.
**Acceptance:** Switching model mid-session keeps the full prior transcript and continues the same logical session.

### F8 — Agent-to-agent messaging
**What:** Agents coordinate via the MCP messaging server (F in §4.5).
**Behavior:** `list_agents` / `send_message` / `check_messages`; nudger auto-wakes idle recipients; per-turn budget caps loops. Dashboard shows a message indicator on sender/recipient cards.
**Acceptance:** An implementer can `send_message` a review request to a reviewer agent; the reviewer (if idle) is nudged and processes it without user action.

### F9 — Session history, search & resume (archive)
**What:** A browsable archive of all sessions, active and inactive.
**Behavior:**
- List every session with name, role, project, timestamps.
- Full-text search across name, role, project, and transcript content.
- Resume any session → restores full history and config and re-attaches a runtime.
**Acceptance:** A stopped agent appears in the archive, is findable by a phrase from its transcript, and resumes with history intact.

### F10 — File & command tracking
**What:** Per-agent tabs listing every file the agent edited and every shell command it ran.
**Behavior:** Captured from tool calls / hooks. Searchable and copyable. Files link to diffs where available.
**Acceptance:** After an agent edits three files and runs two commands, all five appear in the respective tabs and are copyable.

### F11 — Notifications
**What:** Desktop/in-app notifications on significant state transitions.
**Behavior:** Fire on: task complete (`done`), `waiting_input`, and permission-required. Driven by SSE `notification` events. User can mute per type.
**Acceptance:** Backgrounding the dashboard and letting an agent finish produces a "task complete" notification.

### F12 — Onboarding
**What:** Guided first-run flow.
**Behavior:** Steps: (1) configure ≥1 backend with valid credentials, (2) create first project, (3) launch first agent. Blocks the main dashboard until a minimum viable config exists.
**Acceptance:** A fresh install with empty `~/.agentdeck/` walks the user to a first running agent.

### F13 — Activity map *(optional / lower priority)*
**What:** A spatial visualization of agents — a 2D map where each agent is a marker that moves between "idle" and "busy" zones and animates when it sends a message.
**Behavior:** Purely a visualization layer over existing state; no new data. Zones are paintable in a config file. This is an ambient-awareness nicety, not core.
**Acceptance:** Markers reflect live state and animate on message delivery. Safe to ship in a later milestone.

---

## 6. REST API (representative)

All under `http://127.0.0.1:{port}/api`. Commands are REST; live updates come over SSE at `/api/events`.

```
GET    /sessions                       list active agents
POST   /sessions                       launch agent {role, project, backend, model, interface}
GET    /sessions/{id}                   agent detail + status
POST   /sessions/{id}/prompt            send a prompt {text}
POST   /sessions/{id}/cancel            interrupt current turn
POST   /sessions/{id}/stop              stop session
POST   /sessions/{id}/rename            {name}
POST   /sessions/{id}/switch-runtime    {interface?, backend?, model?}
POST   /sessions/{id}/resume            resume from archive
GET    /sessions/{id}/files             tracked files
GET    /sessions/{id}/commands          tracked commands
GET    /archive?q=...                   search historical sessions
GET    /roles  POST /roles  ...         role CRUD
GET    /projects POST /projects ...     project CRUD
GET    /backends  PUT /backends         backend config
GET    /layout    PUT /layout           card order + density
POST   /hook                            hook lifecycle ingest {event, ...} (per-launch token)
GET    /events                          SSE stream (state_update, new_message, notification, ping)
```

---

## 7. Tech stack & constraints

- **Server:** Go, compiled to a single binary. Binds `127.0.0.1` by default; never expose publicly. Hosts the messaging MCP server in-process (official Go MCP SDK).
- **UI:** React + Vite + TypeScript, runs in the local browser, talks to the local server only. (Node is a **build-time** dependency only — the built UI is embedded in the Go binary; end users need no Node.)
- **Hooks:** thin shell scripts registered with the agent CLI that `POST /api/hook` with a per-launch token.
- **Platforms:** macOS and Linux. Terminal runtime is optional and deferred; when built, prefer a cross-platform path (embedded xterm.js / tmux) over macOS-only iTerm2/AppleScript. Chat runtime is the cross-platform default.
- **Prereqs:** at least one authenticated agent CLI (Claude Code and/or Codex). For source builds: Go 1.22+, Node 18+, npm. No runtime Node and no python3.
- **State:** human-edited **config as plain JSON files**; machine state in a single **SQLite** file (`state.db`, FTS5 search), server is sole writer — under `~/.agentdeck/`. No cloud, no account; user owns all data.
- **Distribution:** `install.sh` builds binary + UI (UI embedded) and installs an `agentdeck` CLI; `agentdeck dashboard start && agentdeck dashboard open` launches and opens the UI. Prebuilt binary needs no Node/python at runtime.

---

## 8. Suggested build milestones

1. **Core loop:** Go server + ACP chat runtime + one backend; launch one agent, send prompts, stream responses (F4, F3 minimal).
2. **State & dashboard:** SQLite state manager + `POST /api/hook` ingest + SSE bus + React card grid with live status (F1).
3. **Config:** projects/roles/backends CRUD + onboarding (F5, F6, F12).
4. **Persistence:** session save + SQLite FTS5 archive search + resume (F9), file/command tracking (F10).
5. **Coordination:** in-process Go MCP messaging server (begin with the SDK handshake spike) + nudger + budgets (F8); notifications (F11).
6. **Flexibility:** terminal runtime + switch-runtime (F7); task groups (F2).
7. **Polish:** activity map and any ambient visualizations (F13).

---

## 9. Open questions to resolve before/while building

- Exact ACP message schema for the target CLI versions (tool-call, diff, permission-request shapes).
- Permission model: global skip vs per-role vs per-tool prompting — how granular?
- Concurrency limits: max simultaneous agents before resource pressure; do you queue launches?
- Message-loop safety: is a 15/turn budget enough, or do you also need loop detection across turns?
- Terminal runtime: embedded xterm.js vs tmux as the cross-platform default — which first?
- **Go MCP SDK handshake:** confirm `modelcontextprotocol/go-sdk` (stdio) registers cleanly with **both** Claude Code and Codex — resolve with a ~1h Phase 5 spike before committing the in-process server.
- **SQLite schema:** finalize `state.db` tables (identity, running, status, messages, transcript metadata) + the FTS5 index shape; how transcripts (CLI-owned files) are attributed to `agent_id` on index.
- **Hook token:** where the per-launch token is stored/passed to hooks and rotated on resume.
