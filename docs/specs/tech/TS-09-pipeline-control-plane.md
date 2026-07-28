# TS-09 — Pipeline control plane

**Status:** Partial
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
canonical payload plus digest and proposal id without saving or starting anything. The Pipelines UI
renders the exact payload and performs the normal Save or Start request only after a one-time explicit
confirmation. Save and Start are separate; an edited/different digest cannot reuse confirmation.

**R16 — Approval is interaction control, not authentication.** Proposal tools never call
mutating pipeline methods and cannot approve themselves. Template PUT is naturally idempotent; run
start also carries a client/proposal request id with a unique database constraint so confirmation
retries return the original run instead of creating another. This guided path does not claim to stop
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
shared API client/schema, React Query for template/run server state, the existing transcript
projection for builder proposal tool results, and revision-checked `pipeline_update` invalidation.
Unsaved editor form state is local to the page; only template CRUD or an approved proposal writes a
template, and only the run-start endpoint creates a run. CSS selectors, mocks, errors, confirmation
pending state, and navigation ship with the page (INV §8, §10, §11, §13).

**R24 `(planned)` — Per-stage effort is run-snapshot data validated at start.** A run's frozen
assignment record gains an optional effort per stage beside its backend and model, written by the
same forward-only migration style and non-null decoding as the rest of the run state (TS-02.R17).
Templates are untouched: effort is a run-time assignment, so the version-1 template schema, its
canonical validator, and every stored template stay byte-identical. Start-time validation calls the
same `internal/config` effort-capability check the manual launch path uses (TS-01.R12) inside the
existing all-or-nothing start validation, so one undeclared level prevents the entire run from
starting and no stage process begins — the rule already applied to an unknown backend or model. Stage
launches read effort from the frozen snapshot through the shared lifecycle services, so a catalog
edited mid-run cannot change an in-flight run's levels, and a retried or looped attempt reuses the
snapshot's value rather than re-resolving it.

**R25 — (planned).** After acquiring TS-01.R13's exclusive project-archiving claim, project archive
calls the pipeline manager before changing durable archive state. The manager atomically blocks future
transition claims and stops every active run in that project through its ordinary stop path, then
returns control to the archive service to stop/archive the stage agent. Run start, Continue, Retry,
recovery, and builder launch acquire the shared project start lease before claiming a transition and
hold it through process registration, so none can enter the stop-to-commit window; they reject an
archived project or an archive operation already in progress before a process starts. Restoring a
project does not alter stopped-run state or create a new claim; the person must explicitly start a
new run.

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
```

Exact columns and indexes live in a forward-only TS-02 migration. Foreign keys cascade only within
the four pipeline tables; `agent_id` remains a non-cascading logical reference to existing agent
state so pipeline deletion cannot remove an agent.

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
- **INV §7:** template/run list and startup reconciliation isolate malformed rows/files and surface
  iteration errors without deleting unrelated state.
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

- Product behavior and acceptance: FS-14.R1–R30 and FS-14.A1–A10.
- Existing lifecycle/composition: TS-01.R4–R9; FS-01.
- Persistence and migrations: TS-02.R1–R8, R12, R17.
- HTTP/SSE and UI/API lockstep: TS-03.R1–R11, R16–R17.
- MCP identity and process protocol: TS-04.R5–R9, R12, R17.
- Trust boundary and redaction: TS-05.R1–R11, R13–R14.
- Regression anchors: `internal/pipeline`, `internal/state/pipelines_test.go`,
  `internal/messaging/pipeline_tools_test.go`, `internal/server/pipeline_handlers_test.go`, and
  `ui/src/{api,features/pipelines,schemas/pipeline.ts}`.
