# TS-01 — Architecture

**Status:** Partial
**Code:** `internal/server`, `internal/runtime`, `internal/state`, `internal/index`, `internal/bus`, `internal/config`, `internal/configsource`, `internal/messaging`, `internal/backend`, `internal/archive`, `internal/transcript`, `internal/cli`; `ui/src`
**Absorbed:** architecture contract from [`agent-dashboard-prd.md`](../../archive/agent-dashboard-prd.md); rationale remains in [`architecture-decisions.md`](../../architecture-decisions.md) D1–D5

## 1. Scope

The system boundaries: which processes exist, which packages own which responsibility, how the two runtimes are abstracted, where the source of truth for each kind of data lives, and how live state flows from producer to browser. It is the authoritative statement of the seams the review history keeps stress-testing (launch/resume/switch composition, sole-writer state, stable identity). It does **not** cover wire formats (TS-03/TS-04), schema/migrations (TS-02), the security boundary (TS-05), or build/test (TS-06).

Relationship to sibling docs: `docs/architecture-decisions.md` (D1–D5) is the **rationale record** — why each choice was made and what was rejected; it is not overridden here. `architecture-flow.md` is descriptive orientation and has known drift (§5). Where either disagrees with this spec on a binding architectural contract, this spec wins.

## 2. Design & constraints

**R1 — Two long-lived processes plus agent CLIs, all local.** The runtime topology is: browser UI
(React/Vite, served by the Go binary) ⇄ REST + SSE ⇄ Go server (single binary) ⇄ stdio/PTY ⇄ an
agent CLI (Claude, Codex, OpenCode, or OpenHands where supported). No other process is required at
runtime — the messaging MCP server and embedded UI live inside the Go binary (D3).

**R2 — The server binds `127.0.0.1` only, and fails closed otherwise.** `BindHost` is the hard-coded constant `127.0.0.1` (`internal/server/bind.go`); `LocalAddr` validates the port range and `assertLoopback` refuses any non-loopback listener address at runtime. Binding `0.0.0.0` is prohibited. The loopback bind is a network-reachability limit, **not** a security boundary — that model is TS-05.

**R3 — Package/boundary map.** Each package owns one seam; cross-package calls go through the owner, never around it:

| Package | Owns |
|---|---|
| `internal/server` | HTTP surface, launch/resume/switch composition, hook ingest, SSE fan-out, MCP registration wiring, reconciliation watcher |
| `internal/runtime` | Process lifecycle: the `Runtime` interface, chat (ACP) + terminal (PTY) implementations, the interface-keyed `Registry`, permission/cancel races |
| `internal/state` | `state.db` — sole SQLite writer: identity, running registry, live status, messages, session/transcript metadata |
| `internal/index` | FTS5 full-text index over transcript content; in-memory accumulators feeding replace-style writes |
| `internal/bus` | In-process pub/sub bus backing SSE; snapshot+subscribe atomicity |
| `internal/config` | Plain-JSON config store under `~/.agentdeck`, atomic writes, slug validation, layout/dir modes |
| `internal/configsource` | Phase 7 federation: Claude/Codex native-config discovery, binding, effective view |
| `internal/messaging` | In-process MCP server (`list_agents`/`send_message`/`check_messages`) + token→agent registry |
| `internal/backend` | Backend/model adapter contracts, env layering, credential checks (`credcheck`) |
| `internal/archive` | Session archive queries + FTS-backed search |
| `internal/transcript` | Append-only normalized AgentDeck transcript reader/writer; tolerant reads of session artifacts |
| `internal/cli` | Cobra CLI: `dashboard start/open`, pidfile, reindex |
| `ui/src` | React 18 + Vite SPA (Zustand, React Query, Radix, xterm); consumes REST + SSE only |

**R4 — The `Runtime` abstraction is interface-keyed dispatch.** The server programs against a single `Runtime` interface (`internal/runtime/runtime.go`) with methods `Start`, `SendPrompt`, `Cancel`, `Stop`, `Resume`, `CheckMessages`, `Permission`, `Subscribe`, `Transcript`. Two implementations exist: **chat** (ACP JSON-RPC/NDJSON over stdio) and **terminal** (PTY-backed). The `Registry` dispatches every agent by `agent.interface` (`byIface["chat"]` / `byIface["terminal"]`, `internal/runtime/registry.go`). Both implementations wrap the **same** CLI under the **same** stable identity — that is what makes interface/backend/model switching non-destructive (D4).

**R5 — Source-of-truth rules, split by writer (D1).**
- **Config = plain JSON files** under `~/.agentdeck` (`roles/`, `projects/`, `backends.json`, `config.json`, `layout.json`, `config-sources.json`). Hand-editable, git-friendly, single-writer, low-volume.
- **State = SQLite `state.db`, server-sole-writer.** Nothing else opens the DB for writing. This is what makes SQLite safe here (no multi-process write contention) and authoritative (no derived-index drift). Enabled by the hook-over-HTTP channel (R8) so only the server touches the DB.
- **Transcripts = AgentDeck normalized log plus provider artifacts.** The chat runtime appends
  normalized events to `sessions/{id}/transcript.ndjson`; provider-owned session/history artifacts
  may coexist. AgentDeck indexes the normalized log into FTS5 (`internal/index`, `internal/transcript`).
- **Federation authority is one-way (D1 Phase-7 refinement).** For a bound Claude/Codex backend, the native user/project files remain authoritative; AgentDeck stores only a `config-sources.json` binding plus explicit overrides and derives a redacted effective view. A mirror is disposable cache, never a second authority; only an explicit detached import (planned) makes AgentDeck authoritative.

**R6 — Config composition happens at launch, through one shared helper.** `project.cwd` + `project.context_prompt` + `role.system_prompt` + backend/model + resolved `skip_permissions`/`add_dirs`/env compose into a `LaunchSpec`. Launch, resume, and switch each build a `LaunchSpec` and MUST route through the shared composition helpers rather than hand-rolling a subset: `composeLaunch` (launch), `composeResumeSpec` (resume), `composeSwitchSpec` (switch), plus the field resolvers `resolveSkip`, `expandAddDirs`, `composeEnv`, and the single `teardownAgentRegistration` cleanup (all `internal/server/launch.go`, with resume/switch in their own files). Edits to config affect **future** launches only; a launched agent's composed spec is frozen into its `sessions` snapshot.

**R7 — Stable identity is separate from ephemeral session identity.** An existing `agent_id`
(e.g. `a_8f3c12`) survives resume and backend/interface/model swaps; clone creates a different
identity. `session_id` is the CLI-assigned ephemeral runtime id and changes on (re)launch. Every
switch re-launches on the same `agent_id`; `running` maps it to current pid/session/tty.

**R8 — Event flow: producer → server → `state.db` → SSE; reconciliation is fallback only.** Status producers are (a) lifecycle hooks that `POST /api/hook` with a per-launch token, and (b) the chat runtime deriving status from the ACP stream. The server applies every change to `state.db` and emits an SSE event over the `internal/bus`. SSE event types: `state_update`, `new_message`, `notification`, `ping`. The reconciliation watcher over `sessions/` (`internal/server/reconcile.go`) repairs missed projections from AgentDeck's own normalized `runtime.Event` NDJSON log; provider-native transcript formats are not compatible reconciliation inputs. It is not the primary status channel and must not stomp in-vocabulary status fields (INV §1, §8).

**R9 — The composition seam is shared-helper-only (binding rule).** Launch, resume, and switch are the seam where config, runtime, state, hooks, and MCP registration compose. Any field or cleanup step added to one path must be added to all three, via the shared helpers of R6 — never as an inline subset. This rule is the mechanical form of INV §2 and is enforced by review, not by the compiler.

## 3. Interfaces & data shapes

**Runtime interface** (`internal/runtime/runtime.go`, minimum surface):
```go
type Runtime interface {
    Start(ctx, spec LaunchSpec) (*Handle, error)
    SendPrompt(ctx, agentID, text string) error
    Cancel(ctx, agentID string) (bool, error)   // false when idle no-op
    Stop(ctx, agentID string) error              // idempotent
    Resume(ctx, spec LaunchSpec, sessionID string) (*Handle, error)
    CheckMessages(ctx, pid int) error            // nudge drain
    Permission(ctx, agentID, toolCallID, decision string) error
    Subscribe(agentID string) (<-chan Event, func(), error) // buffered, drop-oldest
    Transcript(agentID string) ([]Event, error)
}
```

**On-disk layout (source of truth by writer):**
```
~/.agentdeck/            (0700; $AGENTDECK_HOME overrides)
  roles/{role}.json      persona: system_prompt + skip_permissions (null=inherit)
  projects/{p}.json      cwd + context_prompt + add_dirs
  backends.json          providers + models + per-model env/keys (version 2)
  config.json            port, default_project/role, skip_permissions, mutes
  layout.json            card order + density + group collapse
  config-sources.json    Claude/Codex bindings + overrides (Phase 7)
  state.db               SQLite — server sole writer (identity, registry, status, messages, FTS5)
  sessions/{id}/         AgentDeck normalized transcript + provider session artifacts
```

**Identity vs session (logical):** `agent_id` stable in `agents`/identity rows; `session_id`, `pid`, `tty` live in the `running` registry row keyed by `agent_id` and are rewritten on each (re)launch.

## 4. Invariants

- **INV §2 — Parallel paths share one helper.** Binds R6/R9: launch/resume/switch WILL drift unless routed through `composeLaunch`/`composeResumeSpec`/`composeSwitchSpec` and the shared field resolvers. Its corollary (permission re-resolution fails closed) governs `resolveSkip` inputs (see TS-05).
- **INV §4 — Create/teardown symmetry.** Binds R6/R8: every artifact created at registration (hook token, MCP session/file, DB row) is torn down by the single `teardownAgentRegistration` on every exit path, generation-scoped, old-before-new.
- **INV §1 — Crossing a boundary resets or republishes derived state.** Binds R7/R8: resume/switch/reconnect must reset or republish state derived from the old identity/connection; the reconcile fallback must not overwrite fresher runtime-derived state.
- **INV §6 — A new runtime joins every existing contract.** Binds R4: any new interface/driver walks the §6 checklist (persistence, full LaunchSpec via R6 resolvers, fan-out/drain, messaging, turn boundaries, reconcile, teardown) before it is first-class.
- **INV §14 — Loopback is not a security boundary.** Binds R2: the bind constraint keeps remote sockets out but is not access control; TS-05 owns the actual boundary.
- **R10 (local invariant) — `state.db` has exactly one writer.** No package other than `internal/state` (driven by the server process) opens the DB for writing. A second writer is a defect regardless of correctness, because D1's safety argument depends on single-writer.

## 5. Deviations & open decisions

- **Optional terminal drivers are not selectable in the normal UI.** The terminal runtime itself is
  installed by `internal/server` and xterm is usable; tmux/iTerm2 APIs/capabilities exist but the UI
  has no driver picker (FS-07).
- **Detached federation import is `(planned)`.** `detach=true` returns `501`; TS-07.R11 owns the
  future materialization boundary.
- **Full env inheritance is deliberate.** Child agents inherit `os.Environ()` minus adapter strip
  keys, then composed overrides. TS-05.R8 owns the security/compatibility tradeoff.

## 6. Traceability

- Bind/loopback: `internal/server/bind.go` (`BindHost`, `assertLoopback`); `internal/server/security.go`.
- Runtime abstraction + dispatch: `internal/runtime/runtime.go` (`Runtime`), `internal/runtime/registry.go` (`byIface`, `handleAgentExit`, `SetExitHook`).
- Composition seam: `internal/server/launch.go` (`composeLaunch`, `resolveSkip`, `expandAddDirs`, `composeEnv`, `teardownAgentRegistration`), `resume.go` (`composeResumeSpec`), `switch.go` (`composeSwitchSpec`).
- Sole-writer state: `internal/state/*`, token validation in `internal/state/manager.go`, reconcile
  fallback `internal/server/reconcile.go`.
- Event flow: `internal/server/hook.go`, `internal/server/sse.go`, `internal/bus/bus.go` (`SubscribeWithSnapshot`).
- Regression anchors: `TestSwitchRuntimeKeepsTargetRegistration`, `TestCrashTearsDownAgentRegistration`, `TestSessionParamsOmitModelWhenInherited`.
