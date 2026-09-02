# TS-01 — Architecture

**Status:** Partial
**Code:** `internal/server`, `internal/runtime`, `internal/state`, `internal/index`, `internal/bus`, `internal/config`, `internal/configsource`, `internal/messaging`, `internal/contextref`, `internal/pipeline`, `internal/backend`, `internal/archive`, `internal/transcript`, `internal/cli`, `ui/src`
**Absorbed:** architecture contract from [`agent-dashboard-prd.md`](../../archive/agent-dashboard-prd.md); rationale remains in [`architecture-decisions.md`](../../architecture-decisions.md) D1–D5

## 1. Scope

The system boundaries: which processes exist, which packages own which responsibility, how the two runtimes are abstracted, where the source of truth for each kind of data lives, and how live state flows from producer to browser. It is the authoritative statement of the seams the review history keeps stress-testing (launch/resume/switch composition, sole-writer state, stable identity). It does **not** cover wire formats (TS-03/TS-04), schema/migrations (TS-02), the security boundary (TS-05), build/test (TS-06), or the planned product-managed agent-knowledge package and overlay (TS-11).

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
| `internal/contextref` | Canonical context-reference identity, typed source resolution/rendering, direct-grant authorization, and bounded reads; no artifact payload store or scheduler |
| `internal/pipeline` | Template validation, durable sequential run state machine, transition reconciliation, stage-result and AgentDecker proposal services |
| `internal/backend` | Backend/model adapter contracts, env layering, credential checks (`credcheck`) |
| `internal/archive` | Session archive queries + FTS-backed search |
| `internal/transcript` | Append-only normalized AgentDeck transcript reader/writer; tolerant reads of session artifacts |
| `internal/cli` | Cobra CLI: `dashboard start/open`, pidfile, reindex |
| `ui/src` | React 18 + Vite SPA (Zustand, React Query, Radix, xterm); consumes REST + SSE only |

**R4 — The `Runtime` abstraction is interface-keyed dispatch.** The server programs against a single `Runtime` interface (`internal/runtime/runtime.go`) with methods `Start`, `SendPrompt`, `Cancel`, `Stop`, `Resume`, `StartActivation`, `Permission`, `Subscribe`, `Transcript`. Two implementations exist: **chat** (ACP JSON-RPC/NDJSON over stdio) and **terminal** (PTY-backed). The `Registry` dispatches every agent by `agent.interface` (`byIface["chat"]` / `byIface["terminal"]`, `internal/runtime/registry.go`). Both implementations wrap the **same** CLI under the **same** stable identity — that is what makes interface/backend/model switching non-destructive (D4).

**R5 — Source-of-truth rules, split by writer (D1).**
- **Config = plain JSON files** under `~/.agentdeck` (`roles/`, `projects/`, `pipelines/`, `backends.json`, `config.json`, `layout.json`, `config-sources.json`). Hand-editable, git-friendly, single-writer, low-volume.
- **State = SQLite `state.db`, server-sole-writer.** Nothing else opens the DB for writing. This is what makes SQLite safe here (no multi-process write contention) and authoritative (no derived-index drift). Enabled by the hook-over-HTTP channel (R8) so only the server touches the DB.
- **Transcripts = AgentDeck normalized log plus provider artifacts.** The chat runtime appends
  normalized events to `sessions/{id}/transcript.ndjson`; provider-owned session/history artifacts
  may coexist. AgentDeck indexes the normalized log into FTS5 (`internal/index`, `internal/transcript`).
- **Federation authority is one-way (D1 Phase-7 refinement).** For a bound Claude/Codex backend, the native user/project files remain authoritative; AgentDeck stores only a `config-sources.json` binding plus explicit overrides and derives a redacted effective view. A mirror is disposable cache, never a second authority; only a future explicit detached import makes AgentDeck authoritative.

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
A Stop that loses the claim returns the same conflict a losing resume returns. **Shutdown quiesces
this claim**: it closes the gate to new transitions and waits for the in-flight ones before
snapshotting and clearing the registry, because an activation resumes its recipient from a detached
goroutine and the runtime's durable running/status rows are written before it joins the registry —
a snapshot inside that window walked past the agent and let the resume register a live orphan into
an already-cleared registry (INV §4/§9). Because the claim is
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

**R18 — Control, context/artifact, and conversation are separate architectural
planes.** Control facts are typed state owned by `internal/state` and domain services; they advance
through transactions, claims, lifecycle events, and bounded reconciliation rather than prose
protocols between models. Context/artifact payloads remain in their existing authoritative stores
and cross a boundary through an explicit bounded read or reference. A model conversation receives
only a user instruction, an intentional assignment/continuation, or an explicit activation for work
that needs reasoning. SSE and in-process channels may announce that state changed, but
they are lossy accelerators and never replace the durable authority or carry rich context by
default.

**R19 — Activation is one small control-plane primitive.** `internal/state` owns a
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

**R20 — Source fact and activation commit together; execution is a separate
service.** A state-owned transaction first commits the authoritative domain mutation and the
activation signal appropriate to that domain. Agent and reserved-user mail use one transaction for
message insert plus a mail-specific rule that permits at most one pending `mail` activation per
agent. The post-commit in-process signal only wakes the server-owned activation executor; a bounded
sweep and startup recovery discover the same durable pending rows. **Bounded is a fixed number, both
ways**: one sweep reads at most a fixed batch of pending rows, oldest first, and admits an activation
only while the count in flight is under the same fixed bound. The bound spans sweeps rather than
resetting per tick, because an activation that wakes a stopped recipient starts a process and
outlives the tick. A row the sweep does not admit is not read, claimed, or modified, so the backlog
drains over later sweeps in age order instead of launching every backlogged recipient at once. **Startup recovery failing is
fatal to startup**, because the executor lists only `pending` rows: a claimed pre-attempt row that
recovery could not release is invisible for the life of that process and its source work never runs
(INV §9/§15). The executor dispatches by closed
kind and lets the kind handler check source availability/eligibility and apply its transition policy;
it does not impose mail's coalescing or replay rule on every kind. For `mail`, the handler claims
with a unique token, performs no external effect until that claim is durably marked attempted,
releases or recovers stale pre-attempt claims, and never replays an attempted row. The executor
publishes no activation SSE/history surface; mail's existing unread state remains the user-facing
projection.

**R21 — Activation uses the runtime and lifecycle seams without reopening their
races.** The runtime activation operation is agent-id-keyed,
kind-aware activation operation with an atomic turn-start gate. For an already-running chat
agent, the runtime rechecks idle/no-active-turn, holds the turn gate while the server durably marks
the kind-owned pre-side-effect transition, commits the ordinary budget/status turn state, and only
then issues the provider instruction. If another turn wins, no external effect occurs and the kind
handler decides how its claim remains actionable. The executor also **defers to an in-flight
exclusive lifecycle transition** rather than racing it: a stage launch/continue composes its first
durable assignment inside R16's claim while the runtime is already registered and idle, so an
activation starting in that window won the turn gate and failed the transition's own prompt.
The deferral is decided by **taking R16's claim across the turn start**, for a running recipient as
well as a stopped one; a non-claiming read of it is at most a fast path, because it cannot order a
transition that claims after the read. Deferring is free because the opportunity is durable, and
losing the claim is pre-attempt. Ordinary status/turn state commits before the
provider frame, and a failure to commit it releases the turn gate and surfaces rather than prompting
the model behind an `idle` record (INV §15). For a stopped mail recipient, the executor takes
TS-01.R16's exclusive lifecycle claim, applies FS-06's attempted transition immediately before the
first resume side effect after all fallible, side-effect-free resume validation, and routes through
the same claimed resume/composition path; after a
successful resume it starts the activation before releasing that transition. Lifecycle-claim loss
is pre-attempt and returns mail to pending. Once a mail wake or provider side effect has been
attempted, failure, cancellation, crash, or restart can omit that mail turn but cannot repeat it.
This is FS-06's policy, not a default for future tasks: a future dependency-backed activation may
remain actionable until its owning durable task/attempt records a successful start. A fixed
kind-specific instruction is the only activation data sent to the provider, and it is not appended
as a user-authored AgentDeck transcript event.

**R22 — Context references are one in-process context-plane
service.** `internal/contextref` owns canonicalization of typed immutable source locators, source
validation, bounded source composition, direct-grant authorization, and personal direct-share
projection. It delegates durable rows to `internal/state`, transcript reads and normalized-event
projection to `internal/transcript`, pipeline-attempt reads to the existing pipeline/state
authority, recipient resolution to one context-specific durable chat-agent directory, and agent-facing transport to
`internal/messaging`. It stores no copied source payload and calls no runtime prompt, activation,
mail, lifecycle, SSE, or local HTTP path. The server constructs one service and supplies it to the
existing MCP server; there is no context daemon, second database writer, generic artifact store,
provider-specific reader, or workflow engine.

`internal/transcript.ProjectEvent` is the one Go seam that decodes a `runtime.Event` into typed,
presentation-neutral text parts and an explicit disposition. The existing indexer derives its
search bag from that projection, while the context renderer adds readable role/type framing and
folds adjacent assistant deltas from the same projection; neither owns another event-type switch.
`runtime.AllEventTypes` is the closed registry used by a table-driven projection test, so every
current normalized `runtime.Ev*` is classified as rendered or deliberately metadata-only and an
unknown type produces an explicit bounded marker rather than disappearing. The browser keeps its
separate UI-object projection because it is a different-language presentation artifact, but its
live and replay paths continue to share the existing `appendRenderedEvent` reducer. This is the
cross-consumer boundary required by INV §2, not an attempt to make search text, pull-context prose,
and chat bubbles identical.

The transcript reader also exposes an additive skipped-record diagnostic to the context path while
preserving the existing tolerant reader behavior for current callers. If a physical NDJSON record
exceeds the reader's 8 MiB safety limit inside the selected turn/span, context composition emits one
bounded omission marker at that record's stream position and continues; it never returns a clean
page that silently implies the oversized record was rendered.

The context recipient directory reuses the shared address-matching helper but not FS-06's
mail-addressable/wake-candidate query: it contains non-archived durable chat identities whether
running or stopped, and pipeline association is irrelevant because a grant starts no process. This
separate query is one state snapshot and one code-owned predicate so context sharing cannot inherit
mail retry/lifecycle policy by accident (INV §2/§5).

**R23 — Reference, authorization, attachment, and personal state
remain separate.** A canonical reference is keyed only by its immutable source locator. Direct
grants authorize an agent and own grant-specific presentation; personal hidden/visible preference
affects only the recipient's ad-hoc list. A future work domain owns its own attachment and durable
participant membership and presents attached reference ids through that work's API. It may ask the
context service to validate/read a reference for a participant, but it must not encode work ids,
assignees, labels, task state, or read state into reference identity, synthesize direct grants, or
make the global direct-share list the assignment-discovery protocol. Terminal work state alone does
not revoke participant access; reassignment or explicit participant removal is the owning work
domain's authorization transition.

- **R24** — **`dependency` is the second activation kind, and it declares its own
  contract.** The activation record gains a nullable stable source work id so a kind may name the
  durable work it belongs to, as R19 and TS-02.R23 anticipated; `mail` leaves it empty and its
  one-pending-row-per-agent index is unchanged. The `dependency` kind sets it to its owning task id,
  keys uniqueness on `(agent_id, source_id)`, and takes the retry policy R21 already reserved for it:
  it remains actionable until its owning task records a confirmed start, rather than mail's
  at-most-once retirement. The record still carries no instruction, prompt, arm set, context
  reference, or retry counter. The task domain that owns it is the "future work domain" of R23: it
  owns its own attachment and assignee membership, presents attached reference ids through its own
  API, and never encodes task ids or assignees into context-reference identity. The shipped turn-end
  dispatch invokes one hard-coded consumer; it becomes a generation-scoped subscriber fan-out shared
  by the pipeline and task domains rather than gaining a second dispatch path (INV §2, TS-10.R19).
  See TS-10.

- **R25 (planned) — Internal actions become one in-process server capability with a thin packaged
  client only after FS-17.R20 passes.** The server remains the sole domain writer and owns one
  provider-neutral action registry; a reviewed narrowly scoped adapter exposes it only to
  generation-authenticated chat launches, and the
  running `agentdeck` executable is the client. Runtime composition adds the client path, reviewed
  transport parameters, credential, and short discovery prompt once for every chat lifecycle, alongside the optional
  knowledge overlay but outside frozen session configuration. No daemon, provider-specific action
  implementation, second domain service, or data migration is introduced. When this ships,
  `LaunchSpec` and runtime adapters stop carrying AgentDeck's internal MCP registration while
  provider-owned MCP configuration remains outside this boundary (FS-17.R13–R20, TS-04.R32–R40).
  Until the gate passes, the shipped internal MCP composition remains unchanged.


- **R26 — Worktree checkout resolution joins the composition seam as one shared step.**
  The launch, resume, and switch composers all call the single `ensureWorktreeCheckout` helper
  (TS-12.R4) before any process start, replacing the two inline cwd stat checks. Both pipeline
  start paths inherit it through those composers; pipeline stage *validation* keeps its own
  read-only check and never calls the mutating helper (TS-12.R4's 2026-09-02 correction). The
  frozen-snapshot rule is unchanged: resume/switch keep `snap.Cwd`; the helper only re-materializes
  a missing owned checkout at that recorded path. No path grows a private variant of this step
  (R9, INV §2).

## 3. Interfaces & data shapes

**Runtime interface** (`internal/runtime/runtime.go`, minimum surface):
```go
type Runtime interface {
    Start(ctx, spec LaunchSpec) (*Handle, error)
    SendPrompt(ctx, agentID, text string) error
    Cancel(ctx, agentID string) (bool, error)   // false when idle no-op
    Stop(ctx, agentID string) error              // idempotent
    Resume(ctx, spec LaunchSpec, sessionID string) (*Handle, error)
    StartActivation(ctx, agentID, kind string, before func(turnID string) error) (bool, error)
    Permission(ctx, agentID, toolCallID, decision string) error
    Subscribe(agentID string) (<-chan Event, func(), error) // buffered, drop-oldest
    Transcript(agentID string) ([]Event, error)
}
```

`StartActivation` is keyed by stable `agent_id`, accepts only a server-selected activation
kind/instruction, and has a before-side-effect
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
- Context plane (R22–R23): `internal/contextref`, durable rows in `internal/state`, the
  shared `internal/transcript.ProjectEvent` seam plus skipped-record diagnostics, the existing
  `internal/messaging` authority, and separation/coverage named by FS-15.A1/A5/A6.
- Archive transition gate: `internal/server/archive_gate.go`, `archive_actions.go`, and
  `archive_gate_test.go` (project reservation and agent/project transition barriers).
- Model catalog autosync: `internal/config/{codexmodels,claudemodels,modelautosync}.go`,
  invoked after seeding by `internal/cli/dashboard.go` (R14).
- Dashboard process logging: `newDashboardLogger` and the scoped process-default logger in
  `internal/cli/dashboard.go` (R15), with mode/append/sink regressions in `internal/cli/cli_test.go`.
- Regression anchors: `TestSwitchRuntimeKeepsTargetRegistration`, `TestCrashTearsDownAgentRegistration`, `TestSessionParamsOmitModelWhenInherited`.
- Planned agent-facing knowledge package and lifecycle overlay: TS-11, which extends the R6/R9
  composition seam without changing frozen user configuration.
