# Architecture Decisions & Rationale

The load-bearing architecture choices for AgentDeck and *why* they were made. The [PRD](agent-dashboard-prd.md) and phase specs state these as fact; this doc records the reasoning and the alternatives weighed, so the choices can be revisited deliberately rather than re-litigated by accident.

---

## D1 — Storage split by writer: config in files, state in SQLite, server is sole writer

**Decision.** Split persistence by *who writes the data*:
- **Human-edited config → plain JSON files** (`roles/`, `projects/`, `backends.json`, `config.json`, `layout.json`). Tiny, rarely written, zero concurrency, and genuinely better hand-edited / `git`-tracked / `cat`-able.
- **Machine-generated state → SQLite** (`state.db`: agent identity, running registry, live status, messages, session/transcript metadata + an **FTS5** full-text index). Queryable, transactional, searchable, no per-file scanning.
- **The Go server is the sole writer to SQLite.** Nothing else opens the DB for writing. This is what makes SQLite safe here — no multi-process writer contention — and it is enabled by D2 (status arrives over HTTP, so only the server touches the DB).

**Why.** Full-text session search (F9) over loose transcript files means scanning and parsing every file on disk — it does not scale with session count. SQLite + FTS5 gives real full-text search for free and stays fully local-first (single file, no server process, no account). Config stays in files because a DB there is pure downside: migrations, and you can't read your own config without a tool.

**Alternatives considered.**
- *Everything in files (with a derived, rebuildable index).* Works, but leaves an index that can drift from the files and still needs reconciliation logic. Since the server is the sole writer (D2), SQLite can simply be **authoritative** for state — no drift, no reconciliation.
- *Everything in SQLite, including config.* Loses the transparency/hand-editability that makes config pleasant to live with, for no real gain (config is low-volume and single-writer already).

**Local-first is preserved.** One SQLite file under `~/.agentdeck/`, no server process, no cloud, user owns the file; config remains plain text. The agent CLI's own transcript files (written by Claude Code / Codex, outside our control) stay where the CLI writes them under `sessions/`; we **index** those into FTS5, but our own state is SQLite-native.

**Phase 7 federation refinement.** “Config in files” does not require AgentDeck to duplicate config
already owned by Claude Code or Codex. For linked backends, the native user/project files remain the
authoritative plain-text configuration; AgentDeck stores a small `config-sources.json` binding plus
explicit overrides and derives a redacted effective view. A mirror, when native pass-through is not
possible, is disposable cache rather than a second authority. Only an explicit detached import makes
AgentDeck authoritative for the copied values/assets. This one-way authority rule avoids an
irreconcilable two-writer merge while retaining local ownership, inspectability and hand editing.

---

## D2 — Hooks report to the server over localhost HTTP (+ per-launch token)

**Decision.** Lifecycle hooks (and any external status producer) **POST to the server** (`POST /api/hook`). Each launched agent's hooks receive a **one-time token** at launch and include it on every POST, so other local processes can't spoof status. A reconciliation sweep over `sessions/` exists only as a fallback for transcript files written out-of-band by the agent CLI — it is not the status channel.

**Why.** Direct HTTP is ordered, immediate, and race-free. The server **already** exposes localhost HTTP for the browser UI, so this adds **zero new transport** — hooks just hit an endpoint that already exists. Keeping hooks thin (a shell `curl`) keeps the channel language-agnostic and debuggable.

**Alternatives considered.**
- *File-watcher as the channel (hooks write `status/`/`running/`, server watches via fsnotify).* This is fragile: fsnotify can miss events under load (kqueue on macOS especially), forcing a periodic reconciliation sweep to paper over misses, and partial-write races. Demoted to a fallback only.
- *Unix domain socket.* Marginally "more local," but the browser UI can't speak it, so we'd run *two* transports. HTTP-over-`127.0.0.1` is already local-only (never touches the network); the trust gap (any local process could POST) is closed by the per-launch token.

---

## D3 — Host the messaging Model Context Protocol server in-process in Go; no runtime Node or python3

**Decision.** Implement the agent-to-agent messaging Model Context Protocol (MCP) server **inside the
Go binary** using the official Go MCP SDK, instead of a separate Node.js process. The dashboard mounts
the streamable HTTP transport at loopback-only `/mcp`; each launched agent receives a scoped registration
and token. Drop **python3** entirely.

**Precise scope.** This drops **runtime** Node only:
- **Build-time Node stays** — the React/Vite UI compiles with it. Unavoidable and fine.
- **Runtime Node goes** — for a prebuilt-binary distribution (Go binary with embedded UI), the end user then needs **nothing but the binary + their agent CLI**. That delivers the local-first single-binary promise for real.

**Why.** Fewer moving parts and a simpler install: one process supervises itself, and the MCP tools (`list_agents` / `send_message` / `check_messages`) become **in-process** reads/writes of `state.db` with no serialization boundary — which is exactly what makes D2 trivial (hooks-calling-back is just a function call away from the same state). python3's only use was installer JSON-escaping, replaceable by the Go binary or `jq`.

**Interoperability boundary.** The in-process HTTP server and tool calls are tested, but real Claude Code
and Codex registration remains a credentialed acceptance gate. If either CLI rejects loopback HTTP MCP,
the fallback must be a working stdio proxy into this same server—not a second state-owning MCP process.

**Why the "Node ecosystem" argument doesn't block this.** External MCP servers a user adds are launched by *their agent CLI* as independent processes regardless of our language — hosting Node ourselves buys nothing for those. Our messaging server is the only Node we'd ship at runtime, and it is small.

---

## D4 — Terminal runtime is cross-platform by default

The built terminal path uses an embedded xterm.js terminal over a pseudo-terminal, with tmux available
as another cross-platform driver. iTerm2/AppleScript remains an optional macOS-only extra behind a
capability probe, never the core. AppleScript-driven control is the most brittle, least portable option.

## D5 — UI delivery: browser for v1; native desktop shell deferred

Ship the browser-served UI for v1. Native OS notifications are handled via the Web Notifications API. A native shell (e.g. Tauri) would give nicer notifications and window management but adds a Rust toolchain plus per-OS packaging/signing — which directly contradicts D3's "fewer runtimes." Revisit post-v1 only if the browser-tab experience becomes a real complaint.
