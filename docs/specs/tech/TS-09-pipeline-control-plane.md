# TS-09 — Pipeline control plane

**Status:** Current
**Code:** `internal/pipeline`, `internal/config`, `internal/state`, `internal/server`, `internal/messaging`, `internal/cli`, `ui/src/features/pipelines`
**Absorbed:** —

## 1. Scope

This spec owns the native pipeline engine: template representation, durable run/attempt/value state,
exactly-once transition claims, stage assignment/result tools, restart reconciliation, and the
AgentDecker proposal seam. FS-14 owns visible behavior. TS-01 owns the process and launch boundaries,
TS-02 owns the persistence classes and migration rules, TS-03 owns HTTP/SSE conventions, TS-04 owns
the shared MCP transport, and TS-05 owns the existing same-user local trust model.

The first version is deliberately an in-process sequential state machine. It is not a distributed
workflow service, background job system, arbitrary expression engine, or parallel graph executor.

## 2. Design & constraints

**R1 — One in-process authority.** `internal/pipeline` owns template validation and one
server-resident manager/reconciler. The server process is the only pipeline-state writer and invokes
the manager directly; there is no second daemon, embedded scheduler database, message broker, or
self-HTTP call. Reconciliation is event-driven with a bounded startup sweep as recovery, not an
unbounded polling loop.

**R2 — Templates are versioned configuration.** Pipeline templates are filename-addressed
version-1 JSON under `$AGENTDECK_HOME/pipelines/{id}.json`, written through the config store's
owner-only atomic-write path. The filename supplies the immutable validated slug id. Reads reject an
unknown version and return per-template diagnostics without making other valid templates disappear.
Create refuses an existing id; update never changes the id; delete removes only the template file.

**R3 — One canonical validator.** Config reads, template CRUD/validation, run start, and
AgentDecker proposals call the same pure validator. It checks structural limits; unique stage/run-input
and stage-local declaration names; existing roles on template save and run start; input/output
bindings; complete success/failure routes; reachable destinations; final outcomes; and a positive
`max_visits` bound for every stage participating in a cycle. Read/list returns an invalid
hand-edited template with bounded field diagnostics so it remains repairable, while save/start
rejects it before side effects. Multiple stages may intentionally produce the same run-wide value
key so a repair stage can supersede an earlier value. Validation itself performs no process or
database side effect.

**R4 — Runs are immutable snapshots plus mutable state.** Starting a run stores one
canonical template snapshot, run display name, project, goal, declared input values, per-stage
backend/model assignments, and initial transition state in `state.db` before any agent process is
started. Later template/config edits cannot rewrite that snapshot. Run, attempt, report, current-value,
and idempotency records are authoritative machine state; JSON templates and transcripts are not
scanned to reconstruct them.

**R5 — The durable state machine names every side-effect boundary.** A run transaction
records its current stage/attempt, monotonic revision, state, and one pending action such as
`launch_stage`, `await_result`, `await_quiescence`, `stop_agent`, `await_approval`, `resume_blocked`,
or `finish_run`. A compare-and-swap on run id plus revision claims each action once. The claim and
resulting state are committed before process, notification, or next-stage effects; retrying a claimed
operation is idempotent or generation-checked (INV §5, §15).

**R6 — Pipeline launch reuses the lifecycle composition seam.** The ordinary HTTP launch
handler and the pipeline manager call one server-owned launch service that wraps `composeLaunch`,
identity/session persistence, registry launch, rollback, and registration teardown. A pipeline launch
adds an immutable run/stage/attempt association and its assignment prompt; it does not reimplement
role/project/backend/model, permission, credential, environment, MCP, transcript, or cleanup logic.
The association is durable before the process can act and is exposed by session reads through a
pipeline-state join rather than a group/name convention (INV §2, §4, §6).

**R7 — Assignments are deterministic bounded text.** One renderer builds the first prompt
from the frozen goal, stage title/instruction, locally named resolved input values and descriptions,
relevant prior structured results, scope, required result vocabulary, and declared outputs. It clips
each field and the total prompt to documented limits, records a hash/version with the attempt, and
never inserts unrelated transcripts or guesses at file contents.

**R8 — Stage results use the existing scoped MCP identity.** The shared in-process MCP
server adds `report_pipeline_stage_result`. Its arguments contain only outcome, bounded summary,
bounded details/checks, and declared output text. Caller identity comes from the per-launch token;
the store joins that identity to the current run/stage/attempt. A caller with no current assignment,
an old generation, a stopped run, an undeclared output, or an accepted result receives a stable
tool error and causes no mutation.

**R9 — Result acceptance is one transaction.** Accepting a report validates the exact
outcome/output set and the frozen route's destination inputs, then writes the immutable attempt
report, current named values with source provenance, and `await_quiescence` in one SQLite
transaction. The result is visible before the tool returns. A missing destination value rejects the
report without partially installing outputs. The selected route and destination visit bound are
recomputed from the immutable snapshot and claimed with the next-attempt transaction only after
quiescence; blocked has no destination and becomes paused at that boundary.

**R10 — Quiescence is a normalized runtime event, not inferred status.** The server fans
the persisted normalized `turn_end` event to the pipeline manager in addition to the ordinary bus.
Only a matching current attempt that already has an accepted report may cross the quiescent boundary.
`idle`, `done`, hooks, free-form mail, process exit, and transcript text cannot synthesize a report.
The fan-out is direct and lossless to the manager; SSE remains a lossy presentation channel.

**R11 — Advancing is restart-safe and sequential.** After a success/failure report reaches
quiescence, reconciliation claims the transition, stops the reporting agent through the ordinary
idempotent lifecycle path, commits that boundary, then either records an approval pause/final result
or creates and launches one next attempt. A blocked report instead pauses with that same idle agent
available for continuation. At most one attempt per run may own an active agent. A backward route
increments the destination's visit count and pauses with `loop_limit_reached` instead of exceeding
`max_visits`.

**R12 — Recovery actions preserve identity rules.** Blocked Continue stores the new user
text and creates a new attempt record linked to the earlier attempt but associated with the same agent
id. It sends the bounded continuation assignment to that agent when still live and idle, or uses the
shared resume service after restart/inactivity before prompting it. Explicit Retry, launch/crash
recovery, and later loop visits create a fresh agent id. Approval Continue only releases an
already-durable transition. Stop atomically prevents future claims before invoking ordinary
cancel/stop. Concurrent controls use the run revision so one wins and the others return conflict
without duplicate effects.

**R13 — Exit and restart reconcile from durable ownership.** The registry's server-owned
exit callback fans out to both ordinary teardown and the pipeline manager with agent generation and
exit cause. Startup loads every unfinished run and resumes restart-safe launch/stop actions. A run
that was awaiting a result or reporting-turn quiescence is durably paused with a stable recovery
reason and its persisted process is checked/stopped through the ordinary lifecycle/reaper boundary;
malformed run detail is isolated the same way. Startup never treats an empty in-memory registry as
proof that a process is dead and never changes a missing report into success.

**R14 — Deletion separates pipeline records from agents.** Deleting a non-active run
removes its run/attempt/value/idempotency rows transactionally but does not delete agent identities,
session snapshots, transcripts, or archive/index records. Attempt-to-agent references therefore do
not cascade into the existing agent tables. Template CRUD is serialized per store, and existing run
snapshots remain readable after template deletion.

**R15 — AgentDecker proposals are validated, soft-gated data.** The shared MCP server adds
proposal tools for a model-neutral template draft and a saved-template run configuration. They are
available to a token-bound AgentDecker-role chat session, call the canonical validator, and return a
canonical payload plus digest and proposal id without saving or starting anything. Before reporting
MCP success, the server commits one content-addressed canonical proposal record to SQLite; a retry of
the same proposal retains one record. The Pipelines UI reads those records as its approval authority,
renders the exact payload, and performs the normal Save or Start request only after a one-time explicit
confirmation. Save and Start are separate; an edited/different digest cannot reuse confirmation.

**R16 — Approval is interaction control, not authentication.** Proposal tools never call
mutating pipeline methods and cannot approve themselves. Template PUT is naturally idempotent; run
start also carries a client/proposal request id with a unique database constraint so confirmation
retries return the original run instead of creating another. A run proposal's request id is its own
derived proposal id: a caller-supplied idempotency key is dropped before the digest, the returned
payload, and the persisted record, so two otherwise-identical proposals cannot return one payload to
MCP while the approval surface holds another and approval cannot replay an older run. This guided path does not claim to stop
a same-user shell process from calling the ordinary loopback API, consistent with TS-05.R3.

**R17 — One publication path follows commits.** Every committed run mutation publishes a
bounded `pipeline_update` containing run id, revision, state, current stage/agent, and attention
reason. The UI refetches detail for full history. Needs-attention and completion notifications use the
existing notification builder and mute pipeline; transition updates do not create notifications.
Restart hydration and reconnect list active runs from SQLite, so missing SSE events do not lose state.

**R18 — Shared-workspace detection stays advisory.** Start-time validation queries active
ordinary agents and runs by project and returns the conflicting ids/names for the UI warning. It does
not lock a project directory, create worktrees, rewrite project configuration, or serialize separate
runs.

**R19 — Bounds are centralized.** Template count/size, stages, declarations, instruction,
goal, value, summary/details/checks, proposal, and list-page limits live in one pipeline limits module
used by JSON config, HTTP, MCP, assignment rendering, and tests. Collection fields are initialized as
arrays/maps at every JSON boundary. Opaque text is stored and rendered as data; it is not treated as a
secret field, filesystem authority, markup, command, or condition expression.

**R21 — Builder sessions remain ordinary AgentDecker agents.** Create with AgentDecker
launches the configured `agentdecker` role through the shared chat launch service with the user's
chosen backend/model and the configured default project. The agent remains visible, transcript-backed,
archivable, and stoppable through ordinary surfaces; its backend/model never enters the template. A
missing role, unusable default project, non-chat-capable backend, or failed readiness check prevents
the builder launch with the ordinary bounded error. The seeded AgentDecker prompt gains the pipeline
proposal and CLI behavior only when those capabilities ship.

**R22 — The CLI is a thin API client.** `agentdeck pipeline` subcommands cover template
list/validate and run start/show/continue/retry/stop using the same local REST requests and structured
errors as the Pipelines UI. They do not open SQLite or template files directly while the server is
running, do not contain a second transition engine, and require the dashboard server like other live
control commands.

**R23 — The Pipelines page has no second state authority.** The React surface uses the
shared API client/schema and React Query for template/run/proposal server state. It invalidates
proposal reads after the server publishes a committed proposal update, rather than reconstructing
proposals from ACP transcript tool-result content or a browser-local builder-session pointer. Unsaved
editor form state is local to the page; only template CRUD or an approved proposal writes a template,
and only the run-start endpoint creates a run. CSS selectors, mocks, errors, confirmation pending
state, and navigation ship with the page (INV §8, §10, §11, §13).

**R24 — Per-stage effort is frozen execution data validated at start.** A run's frozen
assignment record gains an optional effort per stage beside its backend and model. Each created
attempt copies that level beside its own backend/model through the same forward-only migration style
and non-null decoding as the rest of the run state (TS-02.R17).
Templates are untouched: effort is a run-time assignment, so the version-1 template schema, its
canonical validator, and every stored template stay byte-identical. Start-time validation calls the
same `internal/config` effort-capability check the manual launch path uses (TS-01.R12) inside the
existing all-or-nothing start validation, so one undeclared level prevents the entire run from
starting and no stage process begins — the rule already applied to an unknown backend or model. Stage
launches, continuation, and recovery read effort from the frozen attempt through the shared lifecycle
services, so a catalog or future assignment edit cannot change an in-flight attempt's level, and a
retried or looped attempt reuses the snapshot's value rather than re-resolving it.

**R25.** After acquiring TS-01.R13's exclusive project-archiving claim, project archive
calls the pipeline manager before changing durable archive state. The manager atomically blocks future
transition claims and stops every active run in that project through its ordinary stop path, then
returns control to the archive service to stop/archive the stage agent. Run start, Continue, Retry,
recovery, and builder launch acquire the shared project start lease before claiming a transition and
hold it through process registration, so none can enter the stop-to-commit window; they reject an
archived project or an archive operation already in progress before a process starts. Restoring a
project does not alter stopped-run state or create a new claim; the person must explicitly start a
new run.

**R26 — A proposal is consumed by its own approved mutation and otherwise
expires by retention.** The durable proposal record is an offer, not a standing action. It is marked
consumed only after the exact mutation it describes commits — the saved template write or the
created run — and a consumed record stops appearing on the pending approval surface while remaining
durable. Consumption is keyed by the same content-addressed id creation used (a run proposal's
request id, a saved draft's recomputed digest), so no proposal id travels through the template or
start API and an approval that fails changes nothing. Consumption follows the durable mutation and
can never undo it; the call that actually consumes a record publishes one proposal update so the
Pipelines page refetches. Proposing the same content again **re-arms** that one record rather than
adding a second or leaving it silently consumed: the id is content-derived, so a transport retry and
a genuine re-proposal are indistinguishable, and both must leave exactly one pending offer. Without
re-arming, an agent re-proposing something already approved would receive success with nothing on
any human surface — the discoverability defect the durable record exists to remove. Unapproved
proposals are bounded by the same centralized limits module
(R19), which retains the newest records and prunes older ones at each write; R32 adds the explicit
decline and delete actions on top of that bound, and this requirement's consumption rule is
unchanged by them.

- **R27** — **The result layer converges with the task domain; the run layer does not.**
  The agent-reportable outcome vocabulary, its bounded summary/details/checks limits, and the
  staleness check are defined once and used by both `report_pipeline_stage_result` (R8–R9) and the
  task report tool; duplicating them is a defect under INV §2. The accepting transaction stays
  domain-owned, because stage acceptance also writes declared outputs into run-scoped values and sets
  `await_quiescence`, which no task has; the shared helper is called inside each domain's own
  transaction (TS-10.R7). A run reaching a
  terminal state registers its normalized outcome in the shared result layer in the same transaction
  that commits that terminal state (FS-14.R34), which is what lets other work depend on a run without
  the scheduler reading pipeline internals. The run layer stays separate on purpose: a run is a cyclic
  walk over a frozen template with at most one active agent (R11, R20), while a task graph is acyclic
  with real fan-out, so absorbing runs into tasks would force the task domain to grow revisit and
  back-edge semantics it does not need. The `await_quiescence` boundary between an accepted report
  and stopping the reporter (R9–R11) is the pattern the task domain follows, and the single
  hard-coded turn-end consumer becomes a shared generation-scoped fan-out serving both domains rather
  than a second dispatch path (TS-10.R19). See TS-10.

- **R28** — **The delegated-agent view is a derived read, not control-plane
  state.** A stage agent may create dependent work (FS-16), and the run page shows those agents
  under the attempt that spawned them (FS-14.R39). The projection is computed per request by
  joining each attempt's non-empty `agent_id`/`agent_generation` and creation-time window against
  assigned tasks in the run's project on `created_by_agent_id`/`created_by_generation`. A task is
  attributed to the latest matching attempt whose creation time is not after the task and before
  the next attempt that reuses that same agent generation, so blocked continuation cannot duplicate
  one agent's tasks under both attempts. The projection is followed exactly one hop and stores no
  projection or pipeline-owned task state. The pipeline manager stays free of task-store knowledge
  — the composition happens at the HTTP boundary, where `Detail` output is joined with targeted
  state reads, so neither `internal/pipeline` nor the pipeline tables gain a task dependency.

  The targeted state read is not `ListTasks`: it returns only the task summary fields TS-03.R29
  names, never loads arms/attachments, and uses a windowed query over the attempt creator windows
  to compute each attempt's newest 20 rows, true total, and distinct running-agent count in one
  result. The partial `(project, created_by_agent_id, created_by_generation, created_at DESC,
  task_id)` index of TS-02.R26 bounds the scan to assigned agent-created tasks for those creators
  instead of all retained tasks in the project. The cap lives beside the existing centralized
  pipeline list/presentation bounds. Query or row iteration failure fails the detail read visibly
  under INV §7; an individual missing agent state
  produces TS-03.R29's honest fallback summary rather than dropping the task or failing the page.

  Stage-agent presentation data comes from one bulk state read, while the targeted delegate query
  joins the same durable identity/running/status sources. When `status.detail` is empty, the composer
  falls back to the attempt result summary for a stage agent or the task outcome summary for a
  delegate, then to a short explicit no-activity label; the selected text is clipped to the existing
  preview bound. This derived block may be stale relative to a later state event, but the frontend's
  normal `state_update` path refreshes an available card in place and `task_update` refetches task
  membership as TS-03.R29 specifies.

  A delegated agent is informational only. It never reports a stage result, never satisfies or
  gates a transition, is not stopped when the run advances or terminates, and cannot change a run
  revision. R20's run monotonicity and the advance rule of FS-14.R7 are untouched, so a stale or
  missing projection can never corrupt run state — at worst a card carries the explicit unavailable
  fallback until a later read or state event can enrich it.

- **R29** — **The awaiting-approval attention state is a derived read on the
  existing fan-out, not a new durable field.** A run whose current stage agent holds an unanswered
  approval (FS-14.R54) is recognized through the same direct server→manager fan-out that already
  carries `turn_end` (R10): the persisted normalized permission-request and permission-resolution
  events for an agent that owns a current attempt are fanned to the manager alongside it, so one
  seam carries every runtime fact the control plane acts on rather than a second parallel channel
  (INV §2). The resulting attention value is **derived**, in the same sense R28's delegated-agent
  view is derived: the run's durable `attention_reason` continues to hold pause reasons only, no
  migration or new column is added, and no run revision, pending action, transition, or agent
  lifecycle is affected by a request being raised or resolved. Run monotonicity (R20) and the
  advance rule are untouched, so a lost or stale permission signal can never corrupt run state — at
  worst the run renders without the wait until the next read re-derives it. The derived value is
  recomputed on detail read and on reconnect hydration, so a joining or reloading tab sees the
  current state rather than a missed edge (INV §1).

- **R30** — **The needs-attention notification for that state is edge-triggered
  and idempotent per request.** Entering the waiting state publishes one bounded `pipeline_update`
  through R17's single publication path and builds one needs-attention notification through the
  existing notification builder and mute pipeline; resolution publishes the clearing update and
  builds none. A repeated or replayed signal for a request already known to be pending produces no
  second notification, so a reconnect cannot restage a toast (INV §5). Because a pending request
  does not survive a process end, restart hydration finds no waiting state to restore and re-derives
  from live runtime ownership rather than resurrecting a stale one (INV §9).

- **R31** — **Report refusals draw their wording from one vocabulary beside the
  retry classification.** The statement a refusal makes about what the attempt still owes
  (FS-14.R53) is produced in the same module that owns the refusal code and its FS-17 retry class,
  so a code cannot exist with a class but no statement, or gain a statement that contradicts its
  class (INV §2, INV §8). The assignment renderer (R7) states the same boundary in the same terms
  from that one source, so the instruction an agent re-reads every turn and the refusal it receives
  cannot disagree — the disagreement between them is the defect this closes. Acceptance remains one
  transaction (R9) and a refusal remains a non-mutation: it changes no attempt, no run revision, and
  no pending action, exactly as today.

- **R32** — **Declining and deleting are proposal-record actions;
  the approval paths are untouched.** Reject and Delete (FS-14.R49) reach the same
  `pipeline_proposals` authority the approval surface already reads, through the conditional claims
  of TS-02.R29 and the routes of TS-03.R36. They are pure record operations: neither writes a
  template, creates or changes a run, launches or stops an agent, touches `pipeline_attempts` or run
  values, nor produces any agent-facing effect — no message, no tool result, and no change to a
  result an agent already received. Because FS-14.R57 makes the durable mutation the winner of every
  race, the template-save and run-start paths keep the shape R26 gives them: they still consume by
  content-addressed id, still carry no proposal id through their API, still commit their mutation
  before marking the record, and gain no pre-check against a decline — the consumption claim simply
  may now overwrite a declined row as well as a pending one. Nothing in this design lets a proposal
  record block, delay, or reverse a mutation a person confirmed, so the ordering needs no
  cross-store transaction and no distributed claim, and the one accepted failure mode is a leftover
  listed offer for content that already exists, which R26's re-arm already resolves on the next
  identical proposal. Retention (R19) is unchanged and stays blind to which state a record is in.
  The surface follows R23: it holds no second authority for declined records, re-reads both
  collections after the server publishes the committed `pipeline_proposal_update`, and derives no
  proposal state from transcript tool results or browser-local pointers.

- **R33** — **The stage group label is the stage agent's own name, reused on
  an existing field.** A stage's group label is `stage.Title + " — " + run.DisplayName`, which is the
  string `stageExecution` already composes as that stage agent's name
  (`internal/pipeline/reconcile.go`). It is computed once there and used for both, because writing
  the convention a second time would be two paths building the same string and would let the group
  and the agent name drift apart under a later edit to either (INV §2); `StageExecution` therefore
  gains no field. `LaunchStage` passes that value as the existing `launchRequest.Group`
  (`internal/server/pipeline_lifecycle.go`), where it lands on the same `state.Agent.Group` the
  identity endpoint already writes. That is the whole mechanism: no new launch field, request or
  response shape, durable column, migration, or grid code, and the dashboard's existing label-keyed
  sectioning renders it with no pipeline awareness. A section header consequently repeats the text
  of its single card until a retry or loop revisit adds a second agent, which is accepted rather than
  worked around. No
  guard is needed for a reused agent id, because the only path that reuses one is a blocked
  continuation, which reconciles to `resume_blocked` and reaches `ContinueStage` — a resume that
  reads the stored agent and never composes a launch request. No guard is needed for an empty label
  either: a stage title is validated required, and an omitted run display name defaults to the
  template title at run start, so both halves are non-empty — and the grid already renders an empty
  or a literal `_ungrouped` label as the Ungrouped section in any case. R6 is
  unchanged: the pipeline-state join stays the sole authority for run/stage association, and no read
  path infers membership from a group label.

## 3. Interfaces & data shapes

**Template JSON (logical version-1 shape):**

```json
{
  "version": 1,
  "title": "Implement and verify",
  "inputs": [
    {"name": "spec", "description": "Specification to implement", "required": true}
  ],
  "stages": [
    {
      "id": "work",
      "title": "Work",
      "role": "implementer",
      "instruction": "Implement the requested change.",
      "inputs": [{"name": "specification", "value": "spec", "required": true}],
      "outputs": [{"name": "implementation", "value": "implementation", "description": "What changed"}],
      "max_visits": 2,
      "transitions": {
        "success": {"stage": "review", "approval": "automatic"},
        "failure": {"final": "failure", "approval": "required"}
      }
    }
  ]
}
```

Local input/output `name` is what the stage agent sees; `value` is the run-wide named-value key.
A transition contains exactly one destination: `stage` or terminal `final` (`success` or `failure`).
`blocked` has no template route because it always pauses.

**SQLite logical ownership:**

```text
pipeline_runs       run identity, frozen template/run config, state, revision, pending action
pipeline_attempts   immutable visit/attempt lineage, agent link, assignment hash, report/quiescence
pipeline_values     current run-wide text values plus source provenance
pipeline_requests   unique start request ids and their resulting run ids
pipeline_proposals  canonical AgentDecker proposals, their digest, and consumption state
```

Exact columns and indexes live in a forward-only TS-02 migration (TS-02.R17, TS-02.R22). Foreign
keys cascade only within the four run-scoped pipeline tables; `agent_id` remains a non-cascading
logical reference to existing agent state so pipeline deletion cannot remove an agent, and
`pipeline_proposals` has no foreign key in either direction so approval history is independent of
run and template deletion.

**Manager inputs:** validated start/control commands, accepted stage-result calls, persisted
`turn_end`, generation-scoped runtime exits, and startup reconciliation. Each command returns the
new durable run revision or a structured validation/conflict result.

## 4. Invariants

- **INV §2:** manual and pipeline launch/resume/stop use shared lifecycle services; config, prompt,
  registration, and cleanup are not recomposed inside the pipeline package.
- **INV §4:** stage-agent registrations are torn down by the ordinary generation-scoped helper on
  every requested and crash exit.
- **INV §5:** run revision and pending-action claims choose exactly one transition/control winner.
- **INV §6:** pipeline agents are ordinary chat agents and join every existing persistence,
  messaging, transcript, status, and teardown contract.
- **INV §7:** template/run/proposal list reads and startup reconciliation isolate malformed
  rows/files and surface iteration errors without deleting unrelated state; one unreadable timestamp
  or payload never empties an approval or supervision list.
- **INV §1:** the derived awaiting-approval attention value is recomputed on detail read and
  reconnect hydration rather than carried across a boundary as stale state.
- **INV §8:** errors and attention reasons use bounded stable vocabulary; every mutation failure is
  visible in the Pipelines UI.
- **INV §9:** restart recovery corroborates process ownership and all sweeps/shutdown paths are
  bounded.
- **INV §11:** every collection is non-null in server responses, UI schemas, and test doubles.
- **INV §14:** all pipeline HTTP/MCP routes stay behind the existing `localOnly` wrapper.
- **INV §15:** report, route, value, and action intent commit before tool success, process launch,
  notification, or another externally visible stage effect.
- **R20 — Run monotonicity.** A run revision and attempt lineage only advance; accepted
  reports and completed attempts are immutable. Recovery may continue from a pending action but may
  not rewrite history or decrement visit/attempt counters.

## 5. Deviations & open decisions

- The builder confirmation is deliberately a soft interaction guard under the existing unauthenticated
  same-user loopback API. Hard per-agent API capabilities remain outside this feature.
- Effort, terminal-stage agents, parallel branches/joins, child pipelines, typed/file artifacts,
  arbitrary condition expressions, and filesystem isolation are excluded from the first version.

## 6. Traceability

- Product behavior and acceptance: FS-14.R1–R32 and FS-14.A1–A12.
- Existing lifecycle/composition: TS-01.R4–R9; FS-01.
- Persistence and migrations: TS-02.R1–R8, R12, R17.
- HTTP/SSE and UI/API lockstep: TS-03.R1–R11, R16–R17.
- MCP identity and process protocol: TS-04.R5–R9, R12, R17.
- Trust boundary and redaction: TS-05.R1–R11, R13–R14.
- Regression anchors: `internal/pipeline`, `internal/state/pipelines_test.go`,
  `internal/messaging/pipeline_tools_test.go`, `internal/server/pipeline_handlers_test.go`, and
  `ui/src/{api,features/pipelines,schemas/pipeline.ts}`.
- Archive containment: `Manager.Start`, `Continue`, `Retry`, and `Reconcile`;
  `TestStartRejectsProjectArchiveClaimBeforeDurableMutation`.
