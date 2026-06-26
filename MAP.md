# AgentDeck — Project Map

Central index of the planning docs: what each file is, the build order, and the facts worth keeping in one place. Start here.

## Documents

| File | What it is |
|------|-----------|
| [docs/agent-dashboard-prd.md](docs/agent-dashboard-prd.md) | **Master PRD.** Full product spec: concepts, data model, architecture, all features (F1–F13), REST/SSE surface, tech stack, open questions. Source of truth. |
| [docs/architecture-decisions.md](docs/architecture-decisions.md) | **Architecture decisions & rationale.** Why config=files/state=SQLite, hooks→HTTP, in-process Go MCP. The "why" behind the PRD's design. |
| [docs/phases/README.md](docs/phases/README.md) | Phase plan overview: phase map, dependency graph, milestone mapping, how to brief an agent per phase. |
| [docs/phases/phase-0-foundation.md](docs/phases/phase-0-foundation.md) | Data model, file store, server & CLI skeleton. Substrate. |
| [docs/phases/phase-1-core-loop.md](docs/phases/phase-1-core-loop.md) | ACP chat runtime, launch, streaming chat. (F4, F3 min) |
| [docs/phases/phase-2-state-dashboard.md](docs/phases/phase-2-state-dashboard.md) | State manager, SSE bus, dashboard card grid. (F1) |
| [docs/phases/phase-3-config-onboarding.md](docs/phases/phase-3-config-onboarding.md) | Config CRUD & onboarding. (F5, F6, F12) |
| [docs/phases/phase-4-persistence-archive.md](docs/phases/phase-4-persistence-archive.md) | Archive, search, resume, file/command tracking. (F9, F10) |
| [docs/phases/phase-5-coordination.md](docs/phases/phase-5-coordination.md) | MCP messaging, nudger, budgets, notifications. (F8, F11) |
| [docs/phases/phase-6-flexibility.md](docs/phases/phase-6-flexibility.md) | Terminal runtime, switch-runtime, task groups. (F7, F2) |
| [docs/phases/phase-7-polish-activity-map.md](docs/phases/phase-7-polish-activity-map.md) | Activity map / ambient viz. Optional. (F13) |

### Implementation tech specs

Each phase PRD above has a mirror **tech spec** under `docs/phases/tech/` — the implementation-ready companion (concrete libs, package/file layout, in-code data structures, exact API/SSE JSON, algorithms, ordered task breakdown, tests, resolved open questions). Build from these; the phase PRD is the *what*, the tech spec is the *how*.

| Tech spec | Mirrors |
|-----------|---------|
| [tech/phase-0-foundation-techspec.md](docs/phases/tech/phase-0-foundation-techspec.md) | Phase 0 |
| [tech/phase-1-core-loop-techspec.md](docs/phases/tech/phase-1-core-loop-techspec.md) | Phase 1 |
| [tech/phase-2-state-dashboard-techspec.md](docs/phases/tech/phase-2-state-dashboard-techspec.md) | Phase 2 |
| [tech/phase-3-config-onboarding-techspec.md](docs/phases/tech/phase-3-config-onboarding-techspec.md) | Phase 3 |
| [tech/phase-4-persistence-archive-techspec.md](docs/phases/tech/phase-4-persistence-archive-techspec.md) | Phase 4 |
| [tech/phase-5-coordination-techspec.md](docs/phases/tech/phase-5-coordination-techspec.md) | Phase 5 |
| [tech/phase-6-flexibility-techspec.md](docs/phases/tech/phase-6-flexibility-techspec.md) | Phase 6 |
| [tech/phase-7-polish-activity-map-techspec.md](docs/phases/tech/phase-7-polish-activity-map-techspec.md) | Phase 7 |

## Build order

```
0 ─▶ 1 ─▶ 2 ─┬▶ 3   (config/onboarding)
             ├▶ 4   (persistence) ─┐
             └▶ 5   (coordination) ─┴▶ 6 (flexibility) ─▶ 7 (polish)
```

- **0 → 1 → 2** is a strict chain — each requires the previous.
- **3, 4, 5** all sit on 2 and are independent of each other → parallelizable / reorderable by priority.
- **6** needs 4 (reuses resume machinery). **7** needs 2 + 5.

## Feature → phase

F1→2 · F2→6 · F3→1(min)+2(full) · F4→1(API)+3(modal) · F5→3 · F6→3 · F7→6 · F8→5 · F9→4 · F10→4 · F11→5 · F12→3 · F13→7

## Architecture in one breath

Two runtime processes (+ agent CLIs): **React/Vite UI** ⇄ REST + SSE ⇄ **Go server** (binds `127.0.0.1` only) ⇄ stdio ⇄ **agent CLI** (Claude Code / Codex). The **messaging MCP server is hosted in-process in the Go binary** (Go MCP SDK, stdio) — no runtime Node. Hooks **POST to `/api/hook`** (+ per-launch token). No cloud, no auth.

## Source of truth: `~/.agentdeck/` (config = files, state = SQLite)

Config is plain JSON (hand-editable, git-friendly). Machine state is SQLite, written **only** by the server. Producers (hooks via `/api/hook`, chat runtime via ACP) and consumers (UI via SSE) are decoupled through the server.

```
# config — plain JSON files
roles/{role}.json    persona: system_prompt + permission policy
projects/{p}.json    workspace: cwd + context_prompt + add_dirs
backends.json        providers + models + per-model env/keys (version 2)
layout.json          card order + density
config.json          port, default_project, default_role, skip_permissions

# state — SQLite (server is sole writer)
state.db             identity (stable agent_id), running registry (pid, session_id, tty),
                     live status, messages, session/transcript metadata + FTS5 search index

# agent-CLI-owned transcripts (indexed into state.db for search)
sessions/{id}/       raw transcript history for resume
```

## Load-bearing concepts

- **Stable `agent_id` vs ephemeral `session_id`.** Identity survives resume/clone/backend-swap; the CLI's session id changes. Everything that "switches" (model/backend/interface, F7) re-launches on the same `agent_id` and resumes. Get this right in Phase 0/1.
- **Hook POST → server → SQLite → SSE.** Hooks `POST /api/hook` (token-authed); the server applies to `state.db` and emits `state_update`. A reconciliation watcher over `sessions/` is a fallback only. SSE event types: `state_update`, `new_message`, `notification`, `ping`.
- **Server is sole SQLite writer.** No multi-process contention; `state.db` is authoritative (no derived-index drift).
- **Two runtimes, one CLI, one identity.** Chat (ACP over stdio, cross-platform default) and Terminal (deferred Phase 6; cross-platform xterm.js/tmux preferred over iTerm2). Registry dispatches by `agent.interface`.
- **Messaging in-process + nudger.** `list_agents`/`send_message`/`check_messages` are in-process reads/writes of `state.db`; the nudger wakes idle recipients. Per-turn budget (default 15) caps loops.
- **Config composition at launch:** `project.cwd` + `project.context_prompt` + `role.system_prompt` + `backend/model` → CLI invocation. Edits affect future launches only.

## Conventions / placeholders to confirm

- **Port:** `4317` (placeholder — master PRD leaves it abstract).
- **`AGENTDECK_HOME`** env var overrides `~/.agentdeck/` (for tests/CI).
- Bind address must never be `0.0.0.0`.
- Each phase PRD is self-contained: restates data shapes, lists the REST/SSE it adds, and has an acceptance checklist. Brief one coding agent per phase.

## Stack & prereqs

Go 1.22+ (single binary: server + in-process MCP + embedded UI), Node 18+ + npm (**build-time only**, for the Vite UI), ≥1 authenticated agent CLI. **No runtime Node, no python3.** Deps: `modelcontextprotocol/go-sdk` (MCP), `mattn/go-sqlite3` (state). Platforms: macOS + Linux (terminal runtime deferred/optional). Install via `install.sh`; run `agentdeck dashboard start && agentdeck dashboard open`.
