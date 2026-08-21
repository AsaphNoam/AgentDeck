# TS-01 — Architecture

**Status:** Partial
**Code:** `internal/server`, `internal/runtime`, `internal/state`, `internal/index`, `internal/bus`, `internal/config`, `internal/configsource`, `internal/messaging`, `internal/pipeline`, `internal/backend`, `internal/archive`, `internal/transcript`, `internal/cli`, `ui/src`
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
| `internal/server` | HTTP surface, launch/resume/switch composition, hook ingest, SSE fan-out, MCP registration wiring, activation execution/reconciliation |
| `internal/runtime` | Process lifecycle: the `Runtime` interface, chat (ACP) + terminal (PTY) implementations, the interface-keyed `Registry`, permission/cancel races |
| `internal/state` | `state.db` — sole SQLite writer: identity, running registry, live status, messages, activations, session/transcript metadata, pipeline run state |
| `internal/index` | FTS5 full-text index over transcript content; in-memory accumulators feeding replace-style writes |
| `internal/bus` | In-process pub/sub bus backing SSE; snapshot+subscribe atomicity |
| `internal/config` | Plain-JSON config store under `~/.agentdeck`, atomic writes, slug validation, layout/dir modes, pipeline templates |
| `internal/configsource` | Phase 7 federation: Claude/Codex native-config discovery, binding, effective view |
| `internal/messaging` | In-process agent-facing MCP gateway and token→agent registry; handlers delegate messaging and TS-09 pipeline result/proposal semantics to their owning services |
| `internal/pipeline` | Template validation, durable sequential run state machine, transition reconciliation, stage-result and AgentDecker proposal services |
| `internal/backend` | Backend/model adapter contracts, env layering, credential checks (`credcheck`) |
| `internal/archive` | Session archive queries + FTS-backed search |
| `internal/transcript` | Append-only normalized AgentDeck transcript reader/writer; tolerant reads of session artifacts |
| `internal/cli` | Cobra CLI: `dashboard start/open`, pidfile, reindex |
| `ui/src` | React 18 + Vite SPA (Zustand, React Query, Radix, xterm); consumes REST + SSE only |

**R4 — The `Runtime` abstraction is interface-keyed dispatch.** The server programs against a single `Runtime` interface (`internal/runtime/runtime.go`) with methods `Start`, `SendPrompt`, `Cancel`, `Stop`, `Resume`, `CheckMessages`, `Permission`, `Subscribe`, `Transcript`. Two implementations exist: **chat** (ACP JSON-RPC/NDJSON over stdio) and **terminal** (PTY-backed). The `Registry` dispatches every agent by `agent.interface` (`byIface["chat"]` / `byIface["terminal"]`, `internal/runtime/registry.go`). Both implementations wrap the **same** CLI under the **same** stable identity — that is what makes interface/backend/model switching non-destructive (D4).

**R5 — Source-of-truth rules, split by writer (D1).**
- **Config = plain JSON files** under `~/.agentdeck` (`roles/`, `projects/`, `pipelines/`, `backends.json`, `config.json`, `layout.json`, `config-sources.json`). Hand-editable, git-friendly, single-writer, low-volume.
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

**R8 — Event flow: producer → server → `state.db` → SSE; reconciliation is fallback only.** Status producers are (a) lifecycle hooks that `POST /api/hook` with a per-launch token, and (b) the chat runtime deriving status from the ACP stream. The server applies every change to `state.db` and emits an SSE event over the `internal/bus`. SSE event types include `state_update`, `new_message`, `pipeline_update`, `notification`, and `ping`. The reconciliation watcher over `sessions/` (`internal/server/reconcile.go`) repairs missed projections from AgentDeck's own normalized `runtime.Event` NDJSON log; provider-native transcript formats are not compatible reconciliation inputs. It is not the primary status channel and must not stomp in-vocabulary status fields (INV §1, §8).

**R9 — The composition seam is shared-helper-only (binding rule).** Launch, resume, and switch are the seam where config, runtime, state, hooks, and MCP registration compose. Any field or cleanup step added to one path must be added to all three, via the shared helpers of R6 — never as an inline subset. This rule is the mechanical form of INV §2 and is enforced by review, not by the compiler.

**R11 — Pipeline orchestration stays inside the existing process and lifecycle seams.**
The server constructs one `internal/pipeline` manager over the config/state stores and registry, fans
persisted normalized turn boundaries and generation-scoped exits into it, and starts its bounded
reconciler with the other server loops. Manual HTTP and pipeline launches/resumes/stops share
server-owned lifecycle services; the pipeline manager does not call local HTTP or construct a
partial `LaunchSpec`. TS-09 owns its state machine and data contracts.

**R12 — Effort resolution is one helper on the shared composition seam.** A single
`resolveEffort` joins `resolveSkip`/`expandAddDirs`/`composeEnv` in `internal/server/launch.go` and
is the only place that applies FS-09.R41's precedence (explicit → bound-source override → the
model's `default_effort` → empty, meaning "send nothing"). Launch, a fresh-source resume, switch
target defaulting, and pipeline launch use it; ordinary resume/switch re-apply the frozen resolved
value rather than resolving the catalog again. The resolved value travels as one `Effort` field on
`LaunchSpec`, so a path that forgets it loses effort entirely rather than resolving it differently —
the R9 rule applied to a new field.
Catalog-level capability (which levels a model declares, and whether a level is valid for it) stays
in `internal/config` beside the existing backend/model validator. Its backend capability check
derives from the selected adapter's `EffortDelivery` contract rather than a second type allowlist,
so the pipeline manager and HTTP handlers share one authority rather than each checking the catalog
themselves.

**R13.** Archive/restore is one server-owned lifecycle service, not an HTTP handler
calling another handler or a UI-side sequence of Stop and config writes. One server-owned transition
gate serializes archive/restore per project and per agent. Every path that can start a process —
launch, clone, resume, switch-runtime, pipeline start/Continue/Retry/recovery, and builder launch —
acquires a shared project start lease and holds it through running-row registration or rollback.
Project archive acquires the exclusive `archiving` claim before stopping anything, waits for existing
start leases, blocks new ones, and holds the claim through the agent-flag/project-config commit or its
compensation; individual agent archive takes the corresponding exclusive agent claim against Resume.
The service invokes ordinary runtime/pipeline stop paths; delegates durable agent flags only to
`internal/state`; delegates project JSON reads/writes only to `internal/config`; and publishes the
resulting full agent state through the existing bus. This is the atomic-claim boundary required by
INV §5, not a check of `project.archived` followed later by process registration.

**R14 — Native model autosync is one bounded startup import into the AgentDeck
catalog.** After seeding and before the server starts, `internal/config` reads the validated
`backends.json` snapshot once, detects which provider types opted in, invokes only those providers'
pure local catalog readers, merges every successful candidate set in memory through one shared
add-only helper, and publishes all additions through at most one ordinary atomic `backends.json`
rewrite. A provider reader never writes its native source, starts a provider process, reads
credentials or private account caches, or uses the network; one missing, malformed, or incompatible
source contributes no candidates without suppressing another provider's valid additions. The Claude
reader decodes only `model`, `availableModels`, and the array-or-legacy-string `fallbackModel` from
the fixed user-level settings path, rejects a wrong shape before returning any candidate, and never
logs or returns unrelated source content. Candidate validation reuses the same provider-model-string
rule as backend PUT validation. Existing model entries and defaults win; imported entries become
ordinary user-owned AgentDeck configuration and are not retracted when the native source changes.
This path does not use federation bindings, project precedence, previews, watches, provenance, or
effective generations. It adds no API, config version, cache, sidecar, SQLite state, or migration.

**R15 — Dashboard process logging has one durable sink.** After the AgentDeck home exists and before
startup work that can emit diagnostics, `internal/cli` configures the dashboard's structured logger
and the process-wide `slog` default to append to `$AGENTDECK_HOME/dashboard.log`. A foreground process
uses a multi-writer to retain interactive stderr output; a detached child writes only to stderr
because its parent redirects that stream to the same file. This preserves one copy of each record
and brings package-level `slog` calls under the configured level and format. The file obeys TS-05.R7
and INV §14; this requirement does not authorize persistence of raw provider or terminal output.

**R16 — Wake-on-message is the resume seam, not a fourth path, behind one exclusive
claim.** FS-01.R33's wake is one server-owned helper that wraps the exact resume flow
`POST /resume` uses — the same wakeability gates, `composeResumeSpec` composition, and failure
teardown — and the prompt route (TS-03.R25), the messaging wake path (TS-04.R26), **and the
explicit resume handler itself** all route through that helper. No wake caller composes its own
resume subset (R6/R9). The helper takes one **exclusive per-agent resume claim** (atomic
check-then-act, INV §5) *before any registration side effect* — hook-token minting, MCP
registration, hook-settings composition — and releases it only after the registry resume settles;
the registry's nil-sentinel remains the inner guard. This is required because the existing
`acquireAgentStart` lease is counting, not exclusive, and the agent-keyed registration
(`rememberHookToken`, `registerMessagingMCP`, `teardownAgentRegistration`) means a second
concurrent composer would replace the winner's artifacts and the loser's teardown would then revoke
them (INV §4). A losing concurrent waker therefore returns the existing conflict outcome without
composing or tearing down anything, and the winner's hook token, MCP registration, and
hook-settings file remain intact and in use.

**Stop takes the same claim**, making start and stop one exclusive lifecycle transition per agent
(FS-01.R34). `Registry.Stop` reads an in-progress resume's nil sentinel as `ErrNoHandle`, which is
indistinguishable from "this agent is not running", so an unclaimed Stop landing inside a wake took
the idempotent already-stopped path — reaping and running `teardownAgentRegistration` on the
artifacts the live resume had just minted — and answered success while that resume continued.
A Stop that loses the claim returns the same conflict a losing resume returns. Because the claim is
the only thing standing between a stop and a live resume's registration, **every** stopping verb runs
one shared server-side stop-and-teardown helper rather than its own spelling of stop + cleanup
(INV §2): the Stop route and group release (FS-02.R20) both call it, since a second unclaimed stop
path reintroduces the identical defect through a different door.

**Every lifecycle transition that starts, stops, or resumes an agent's registration takes this one
claim**, not only explicit resume/wake and Stop: runtime switch's stop→resume window
(`handleSwitchRuntime`), pipeline stage launch/initial prompt (`LaunchStage`), and pipeline stage
resume/stop (`ContinueStage`/`StopStage`) all take it before any registration side effect and hold it
across the whole transition. Bulk group release (`releaseAgents`) first reserves every member's
claim, then stops and cleans them up in parallel; if one member is busy it releases its reservations
and returns conflict before stopping any member. Because wake-on-message makes a stopped agent's transient window
wakeable and `acquireSwitch`/`acquireAgentStart` are switch-scoped or counting (not mutually
exclusive with a resume), these paths were otherwise reachable by a concurrent wake or explicit
resume/stop that minted a second registration whose teardown then revoked the winner's
token/MCP/hook-settings (INV §4). Agent/project **archive** stop is exempt because its exclusion
already holds: `beginAgentArchive`/`beginProjectArchive` set flags that make a concurrent resume or
wake fail `acquireAgentStart`, so archive can never mint a competing registration.

**R17 — Chat drafts stay in one bounded browser-local seam.** One feature-local UI module
owns a single `localStorage` record containing non-empty draft text and last-edited timestamps keyed
by `agent_id`; the composer reads and writes that module directly rather than adding server state,
an API, a database/config field, or a global synchronization path. Each write and rehydration keeps
only the 20 most recently edited entries, ignores malformed stored data by starting empty, and
treats unavailable or full browser storage as a persistence failure only: the live composer remains
usable and Send behavior is unchanged. The composer discards its stored entry only after an accepted
prompt or an explicit empty value, and the existing SSE `state_update.removed` branch discards it on
agent deletion beside the annotation tray cleanup. Archive, stop, resume, runtime switch, and
transcript events do not clear or copy the draft. This is the browser-local boundary discipline of
INV §1 without a timer, expiry service, server cleanup, migration, or second source of chat truth.

**R18 `(planned)` — Control, context/artifact, and conversation are separate architectural
planes.** Control facts are typed state owned by `internal/state` and domain services; they advance
through transactions, claims, lifecycle events, and bounded reconciliation rather than prose
protocols between models. Context/artifact payloads remain in their existing authoritative stores
and cross a boundary through an explicit bounded read or reference. A model conversation receives
only a user instruction, an intentional assignment/continuation, or an explicit activation for work
that needs reasoning. SSE and in-process channels may announce that state changed, but
they are lossy accelerators and never replace the durable authority or carry rich context by
default.

**R19 `(planned)` — Activation is one small control-plane primitive.** `internal/state` owns a
payload-free activation record with a stable id, stable target `agent_id`, closed code-owned kind,
state, claim token, and operational timestamps. It represents an opportunity to initiate work; it
does not define universal coalescing, retry, successful-start, or completion semantics. Those belong
to the source domain. `mail` is the only valid kind in this change, and FS-06 owns its deliberately
at-most-once policy. The record contains no message body, prompt, transcript excerpt, context
reference, dependency, assignment, retry policy, or arbitrary metadata; adding a kind requires an
owning FS requirement, an explicit server handler, and a declared source identity/coalescing/retry
contract. There is no generic activation CRUD API, UI, graph, plug-in registry, or workflow DSL.
The shared concept is reusable by later context-link, dependency, and semantic orchestration
features; the initial mail schema and policy are not declared sufficient for those features.

**R20 `(planned)` — Source fact and activation commit together; execution is a separate
service.** A state-owned transaction first commits the authoritative domain mutation and the
activation signal appropriate to that domain. Agent and reserved-user mail use one transaction for
message insert plus a mail-specific rule that permits at most one pending `mail` activation per
agent. The post-commit in-process signal only wakes the server-owned activation executor; a bounded
sweep and startup recovery discover the same durable pending rows. The executor dispatches by closed
kind and lets the kind handler check source availability/eligibility and apply its transition policy;
it does not impose mail's coalescing or replay rule on every kind. For `mail`, the handler claims
with a unique token, performs no external effect until that claim is durably marked attempted,
releases or recovers stale pre-attempt claims, and never replays an attempted row. The executor
publishes no activation SSE/history surface; mail's existing unread state remains the user-facing
projection.

**R21 `(planned)` — Activation uses the runtime and lifecycle seams without reopening their
races.** The current `Runtime.CheckMessages(pid)` operation is replaced by an agent-id-keyed,
kind-aware activation operation with an atomic turn-start gate. For an already-running chat
agent, the runtime rechecks idle/no-active-turn, holds the turn gate while the server durably marks
the kind-owned pre-side-effect transition, commits the ordinary budget/status turn state, and only
then issues the provider instruction. If another turn wins, no external effect occurs and the kind
handler decides how its claim remains actionable. For a stopped mail recipient, the executor takes
TS-01.R16's exclusive lifecycle claim, applies FS-06's attempted transition immediately before the
first resume side effect, and routes through the same claimed resume/composition path; after a
successful resume it starts the activation before releasing that transition. Lifecycle-claim loss
is pre-attempt and returns mail to pending. Once a mail wake or provider side effect has been
attempted, failure, cancellation, crash, or restart can omit that mail turn but cannot repeat it.
This is FS-06's policy, not a default for future tasks: a future dependency-backed activation may
remain actionable until its owning durable task/attempt records a successful start. A fixed
kind-specific instruction is the only activation data sent to the provider, and it is not appended
as a user-authored AgentDeck transcript event.

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

When R21 ships, `CheckMessages(pid)` leaves this interface. The replacement is keyed by stable
`agent_id`, accepts only a server-selected activation kind/instruction, and has a before-side-effect
commit hook (or an equivalent two-phase turn token) so runtime turn arbitration and the kind-owned
durable transition cannot race. The exact Go spelling may follow the implementation, but it must
return whether a turn actually started and must never report success when idle/turn ownership was
lost.

**On-disk layout (source of truth by writer):**
```
~/.agentdeck/            (0700; $AGENTDECK_HOME overrides)
  roles/{role}.json      persona: system_prompt + skip_permissions (null=inherit)
  projects/{p}.json      cwd + context_prompt + add_dirs
  pipelines/{id}.json    reusable model-neutral pipeline templates
  backends.json          providers + models + per-model env/keys (version 2)
  config.json            port, default_project/role, skip_permissions, mutes
  layout.json            card order + density + group collapse
  config-sources.json    Claude/Codex bindings + overrides (Phase 7)
  state.db               SQLite — server sole writer (identity, registry, status, messages,
                         activations, pipelines, FTS5)
  sessions/{id}/         AgentDeck normalized transcript + provider session artifacts
```

**Identity vs session (logical):** `agent_id` stable in `agents`/identity rows; `session_id`, `pid`, `tty` live in the `running` registry row keyed by `agent_id` and are rewritten on each (re)launch.

## 4. Invariants

- **INV §2 — Parallel paths share one helper.** Binds R6/R9: launch/resume/switch WILL drift unless routed through `composeLaunch`/`composeResumeSpec`/`composeSwitchSpec` and the shared field resolvers. Its corollary (permission re-resolution fails closed) governs `resolveSkip` inputs (see TS-05).
- **INV §4 — Create/teardown symmetry.** Binds R6/R8: every artifact created at registration (hook token, MCP session/file, DB row) is torn down by the single `teardownAgentRegistration` on every exit path, generation-scoped, old-before-new.
- **INV §1 — Crossing a boundary resets or republishes derived state.** Binds R7/R8: resume/switch/reconnect must reset or republish state derived from the old identity/connection; the reconcile fallback must not overwrite fresher runtime-derived state.
- **INV §6 — A new runtime joins every existing contract.** Binds R4: any new interface/driver walks the §6 checklist (persistence, full LaunchSpec via R6 resolvers, fan-out/drain, messaging, turn boundaries, reconcile, teardown) before it is first-class.
- **INV §14 — Loopback is not a security boundary.** Binds R2: the bind constraint keeps remote sockets out but is not access control; TS-05 owns the actual boundary.
- **INV §5/§15 — Claim before firing; commit before side effects.** Bind R19–R21: one claim token
  owns each activation transition, and the kind-owned durable attempt/start transition plus ordinary
  turn state precedes wake or provider effects. Channels, status reads, and unread counts cannot
  substitute for that claim.
- **R10 (local invariant) — `state.db` has exactly one writer.** No package other than `internal/state` (driven by the server process) opens the DB for writing. A second writer is a defect regardless of correctness, because D1's safety argument depends on single-writer.

## 5. Deviations & open decisions

- **Optional terminal drivers are not selectable in the normal UI.** The terminal runtime itself is
  installed by `internal/server` and xterm is usable; tmux/iTerm2 APIs/capabilities exist but the UI
  has no driver picker (FS-07).
- **Detached federation import is.** `detach=true` returns `501`; TS-07.R11 owns the
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
- Browser-local chat drafts (R17): one feature-local UI store consumed by
  `ui/src/components/chat/Composer.tsx` and discarded from `ui/src/api/sse.ts`'s existing removed
  branch; component/store/SSE tests cover restore, pruning, send outcomes, malformed storage, and
  deletion cleanup.
- Activation plane (R18–R21): `internal/state/activations.go`, the server activation executor,
  runtime turn-start arbitration, and mail/wake integration tests.
- Archive transition gate: `internal/server/archive_gate.go`, `archive_actions.go`, and
  `archive_gate_test.go` (project reservation and agent/project transition barriers).
- Model catalog autosync: `internal/config/{codexmodels,claudemodels,modelautosync}.go`,
  invoked after seeding by `internal/cli/dashboard.go` (R14).
- Dashboard process logging: `newDashboardLogger` and the scoped process-default logger in
  `internal/cli/dashboard.go` (R15), with mode/append/sink regressions in `internal/cli/cli_test.go`.
- Regression anchors: `TestSwitchRuntimeKeepsTargetRegistration`, `TestCrashTearsDownAgentRegistration`, `TestSessionParamsOmitModelWhenInherited`.
