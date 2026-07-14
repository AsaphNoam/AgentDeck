# FS-00 — Product Overview

**Status:** Current
**Code:** `internal/server`, `internal/runtime`, `internal/state`, `internal/config`, `ui/src` · **Journeys:** J1, J3
**Absorbed:** [`agent-dashboard-prd.md`](../../archive/agent-dashboard-prd.md) §§1–3, §7

This is the entry-point spec: it fixes the vocabulary and the object model that every other feature
spec (FS-01…FS-09) builds on. It states product-level truths as R-items; detailed behavior lives in
the per-feature specs, and architecture lives in the TS-series.

## 1. Purpose

AgentDeck is a **local-first desktop tool for running and supervising many AI coding-agent sessions
in parallel**. It wraps existing agent CLIs (Claude Code, Codex, and additional backends) and gives
every session a persistent identity, live status, a durable normalized agent-event history,
file/command tracking, and a
messaging channel so agents coordinate with each other. The user is a developer who delegates several
concurrent tasks to AI agents and needs one place to see what each is doing, intervene, and resume
past work — without juggling a dozen terminal tabs.

- **R1** — All product data is local. AgentDeck-owned config and Phase 7 native-CLI source bindings
  are plain files; machine state is a single local SQLite database. There is no cloud component and
  no account.
- **R2** — The server binds `127.0.0.1` only and is never exposed publicly. The API is unauthenticated
  on loopback; `/api/hook` and `/mcp` additionally require a per-launch token (see TS-05).
- **R3** — The product goals are: run N sessions concurrently, each addressable as `role@project`;
  show live status at a glance; provide a full streaming chat view per agent; persist and
  search/resume every session; let agents message each other; and support multiple
  backends/models switchable on a live agent without losing history.

**Non-goals (v1):** no cloud sync, no remote/multi-user access, no auth layer; no built-in code
editor (AgentDeck observes and orchestrates, it does not replace the IDE); no support for agent
runtimes that are not CLI/ACP-compatible; no billing, telemetry, or analytics.

## 2. Core concepts

Four backbone objects underlie every view. Shapes below are **logical** (shown as JSON); §3 says
where each is stored.

### 2.1 Agent — identity vs. session

An **agent** is a running or historical session with a **stable identity** that survives resume
and backend/model/interface swaps, separate from the **ephemeral runtime session id** the
underlying CLI assigns.

- **R4** — Every agent has a stable `agent_id`, minted once at launch, that never changes for the
  life of the agent. The CLI's `session_id` is ephemeral and changes on every start/resume/fork.
  Everything that "switches" re-launches on the same `agent_id` and resumes; cloning mints a new
  identity carrying selected source configuration (see FS-01).

```jsonc
// agent identity (state.db)
{ "agent_id": "a_8f3c12", "name": "Atlas", "role": "implementer", "project": "my-app",
  "backend": "claude", "model": "sonnet-4-6", "interface": "chat",
  "created_at": "2026-06-22T10:00:00Z", "group": "auth-migration" }
```

The running registry (`agent_id`, `pid`, `session_id`, `interface`, `tty`, `started_at`) holds one
row per live agent; live status (`state ∈ {busy, idle, waiting_input, done, error}`, `detail`,
`last_trace`, `busy_since`, `context_pct`) is written by the server from hook POSTs / the ACP stream.

### 2.2 Role — the persona

- **R5** — A **role** is a reusable persona defining *how* an agent behaves, independent of where it
  works: a `system_prompt`, display `title`, and a `skip_permissions` policy (`null` inherits the
  global config value; `true`/`false` override it). Roles are stored as config files and seeded if
  absent, never overwriting user edits (see FS-04). Seed roles: `agentdecker`, `implementer`,
  `reviewer`, `researcher`, `pm`, `teammate`.

### 2.3 Project — the workspace

- **R6** — A **project** is a reusable workspace defining *where* and *on what* an agent works: a
  working directory (`cwd`), an injected `context_prompt`, extra accessible directories (`add_dirs`),
  and display metadata (`title`, `color`). Stored as config files.

### 2.4 Backend — the provider runtime

- **R7** — A **backend** is a provider runtime (`backends.json`, version 2) that exposes multiple
  **models**, each optionally carrying its own `env` (API key/endpoint). Backend-level `env` applies
  to all models; per-model `env` overrides it. One backend is marked default; one model is default
  per backend. Backend types include `claude-acp`, `codex-acp`, and additional adapters (see FS-09).

### 2.5 Interface — chat vs. terminal

- **R8** — Every agent runs under one **interface**: `chat` (the cross-platform default, ACP over
  stdio; the server derives status from the stream) or `terminal` (an embedded terminal emulator;
  status comes from hooks). Both wrap the **same** CLI and the **same** `agent_id`, which is what
  makes interface/backend/model switching non-destructive. Only `claude-acp` currently supports the
  terminal interface (see FS-01, FS-07).

### 2.6 Addressing — `role@project`

- **R9** — An agent is addressed as `role@project` (e.g. `implementer@my-app`). The CLI launch form
  splits the positional on the **last** `@`; both sides are required. The same composition is
  available through the New Agent modal (see FS-01.R1).

- **R10** — At launch the server **composes** the effective invocation from the four objects:
  `project.cwd` + `project.context_prompt` + `role.system_prompt` + `backend/model`. Editing a
  role/project/backend affects **future** launches only; a live agent keeps its composed (frozen)
  config for ordinary resume and switch as well; only an explicit federation refresh recomposes
  the federated portion (see FS-04, FS-01.R3/R10/R12).

## 3. Storage — split by writer

- **R11** — Persistence is split by *who writes the data*. Human-edited **config is plain JSON
  files** (hand-editable, `git`-friendly). Machine-generated **state lives in one SQLite file**, and
  the Go server is its **sole writer** — so there is no multi-process contention and the DB is
  authoritative (no derived-index drift). Everything lives under `~/.agentdeck/` (overridable by
  `AGENTDECK_HOME`).

```
~/.agentdeck/
  # config — plain JSON files (server + hand-editable)
  roles/{role}.json          persona: system_prompt + permission policy
  projects/{project}.json    workspace: cwd + context_prompt + add_dirs
  backends.json              providers + models + per-model env (version 2)
  config-sources.json        Claude/Codex source bindings + overrides (federation)
  layout.json                dashboard card order + density
  config.json                port, default_project, default_role, skip_permissions

  # state — SQLite, server is sole writer
  state.db                   agent identity, running registry, live status, messages,
                             session/transcript metadata + FTS5 search index

  # AgentDeck normalized transcripts + any external CLI history used for resume/indexing
  sessions/{agent_id}/       append-only normalized transcript and session artifacts
```

- **R12** — AgentDeck's chat runtime appends normalized events to
  `sessions/{agent_id}/transcript.ndjson`; external CLI session/history artifacts may coexist and
  remain provider-owned. AgentDeck indexes the durable normalized transcript into FTS5 and can
  rebuild that projection (see FS-03, FS-05, TS-02).

## 4. Glossary

- **ACP (Agent Communication Protocol)** — the JSON-RPC / NDJSON protocol the chat runtime speaks to
  the CLI over stdio; the server republishes the stream over SSE. See TS-04.
- **MCP (Model Context Protocol)** — the protocol for the in-process agent-to-agent messaging server,
  hosted inside the Go binary and mounted over loopback HTTP at `/mcp` (no runtime Node). See FS-06,
  TS-04.
- **`agent_id` vs. `session_id`** — `agent_id` is AgentDeck's stable identity (R4); `session_id` is
  the CLI's ephemeral per-start id. Resume/clone/switch preserve the former and mint a new latter.
- **Hook** — a thin shell script registered with the agent CLI that fires on lifecycle events
  (`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`) and POSTs to
  `/api/hook` with its per-launch token. The primary status channel for terminal agents. See TS-04.
- **Nudger** — a server loop that detects an idle agent with pending mail and wakes it so it
  processes messages without user intervention; bounded by a per-turn message budget. See FS-06.
- **Federation** — binding AgentDeck to a backend's native Claude/Codex config files so those files
  stay authoritative and AgentDeck stores only bindings, overrides, and a derived redacted view.
  See FS-08.
- **Required checks** — the build and test work a change must pass before it is considered done. The
  definition lives in `docs/features/AGENT-WORKFLOW.md` §2, not here.

## 5. Deviations & open decisions

- **Local API trusts same-machine callers.** The dashboard
  API is unauthenticated on loopback. Browser attack paths are closed (Host/Origin guard) and
  `/api/hook` + `/mcp` require per-launch tokens, but any local process that can reach the port can
  read transcripts/config and drive agents. Adding real API auth is a product-scope decision.
- **Agent env inheritance by design.** Child agent
  processes inherit the full server environment (minus each backend's stripped keys), so unrelated
  host credentials are visible to agents. Deliberate: CLIs need PATH/HOME/locale plus arbitrary
  provider keys. Detail and reversal path in FS-01 / TS-05.
- **Federation is credential-gated (credential-gated acceptance).** The federation object model (§2.6,
  FS-08) ships and is tested against fakes; live acceptance against pinned real Claude/Codex config
  surfaces (FS-08.A7) is credential-gated. Detached config-source import returns `501` until a
  verified launch-injection path exists.
- The shipped port default is `4317`, overridable via `config.json`; operational CLI behavior still
  needs the dedicated feature coverage listed in the specification backlog.

## 6. Traceability

- Storage split & sole-writer rule: `internal/state`, `internal/config`; rationale in
  `docs/architecture-decisions.md` D1–D3.
- Config composition at launch (R10): `internal/server/launch.go` `composeLaunch`.
- Stable-id-vs-session-id (R4): minted in `composeLaunch` before Start; `internal/runtime`.
- Loopback bind + Host/Origin guard (R2): `internal/server/security.go`; tests
  `TestDNSRebindingHostRejected`, `TestIsLocalHost`.
- Owner-only file modes (R11): `TestHomeTreeIsOwnerOnly`, `TestStateDBIsOwnerOnly`.
