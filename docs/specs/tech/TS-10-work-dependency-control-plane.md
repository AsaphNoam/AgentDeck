# TS-10 — Work dependency control plane

**Status:** Partial
**Code:** `internal/state`, `internal/server`, `internal/messaging`, `ui/src/features/tasks`
**Absorbed:** —

## 1. Scope

The durable task, arm, attachment, and shared-result records behind FS-16, the evaluation that decides
when armed work becomes ready, and the start path that turns a ready task into a running agent. It
covers the `dependency` activation kind's contract on the existing activation primitive, the shared
result layer that pipelines and tasks both write, restart recovery, and the HTTP/MCP/SSE surfaces.

Out of scope: pipeline template authoring, routing, revisits, and recovery (TS-09); context reference
identity, canonicalization, and bounded reads (TS-05.R16 and the context service); agent launch,
resume, stop, and archive mechanics (TS-01). This specification consumes those seams and adds no
parallel copy of them.

Every requirement is `(planned)`; none has shipped.

## 2. Design & constraints

- **R1** `(planned)` — **One in-process authority, no second scheduler.** Arm evaluation and start
  scheduling run inside the existing server process alongside the activation executor and the pipeline
  reconciler. There is no second daemon, embedded scheduler database, message broker, external queue,
  or self-HTTP call, matching TS-01.R5 and TS-09.R1. `internal/state` remains the sole writer of
  `state.db` (TS-02.R2).
- **R2** `(planned)` — **Durable rows are authoritative; the bus is a fast path.** Arm satisfaction,
  task state, and start confirmation are decided only from committed rows. The in-process bus and the
  activation channel are latency hints that may be dropped without changing any outcome, exactly as
  FS-06.R9 treats the mail fast path. A dropped notification costs latency, never correctness.
- **R3** `(planned)` — **Evaluation is event-driven with a bounded startup sweep.** A committed result
  registration (R7) or signal fire re-evaluates only the arms that name that source. Startup performs
  one bounded sweep over unfinished tasks. There is no unbounded polling loop and no periodic full
  graph walk.
- **R4** `(planned)` — **Becoming ready and starting are atomic claims made before any side effect.**
  The `armed → ready` and `ready → running` transitions are single-statement conditional updates that
  decide availability and take the claim together, following the `ClaimMailActivation` pattern. The
  durable transition commits before any launch, resume, or prompt is issued (INV §5, INV §15), so a
  crash between commit and effect leaves recoverable state rather than a lost or duplicated start.
- **R5** `(planned)` — **The `dependency` activation kind declares its own contract.** The existing
  activation record (TS-01.R19, TS-02.R23) gains a nullable stable source work id, which the
  `dependency` kind sets to its owning task id and `mail` leaves empty. Its uniqueness key is one
  pending row per `(agent_id, source_id)`, not mail's one-pending-row-per-agent. Its retry policy is
  the one TS-01.R21 already reserves: a dependency activation remains actionable until its owning task
  records a confirmed start, so a lost claim, a busy lifecycle, or a failed resume is released back to
  pending rather than retired. Repeated start failure is bounded and parks the task (FS-16.R8) instead
  of retrying forever. The record still carries no instruction, prompt, arm set, context reference, or
  retry counter as payload.
- **R6** `(planned)` — **Starting reuses the existing launch, resume, and stop seams.** A launch-spec
  task composes its launch through the existing launch composition helpers, and an existing-agent task
  resumes through the single resume seam that explicit resume and mail wake already use. Finishing a
  task stops its agent through the shared stop seam (FS-01.R6, R34). This plane takes the exclusive
  per-agent lifecycle claim (TS-01.R16) rather than inventing a second one, and reimplements no
  identity, permission, credential, environment, MCP registration, transcript, or cleanup logic
  (INV §2, TS-09.R6).
- **R7** `(planned)` — **One canonical result-acceptance path serves tasks and pipelines.** The result
  vocabulary, its validation, its bounded summary/details limits, and the transaction that accepts a
  reported result are defined once and used by both the task report tool and
  `report_pipeline_stage_result` (TS-09.R8–R9). Accepting a result and registering it in the shared
  result layer happen in the same transaction that commits the owning domain's terminal state. Two
  separate acceptance implementations are a defect under INV §2.
- **R8** `(planned)` — **The shared result layer is a small keyed registration, not a second history.**
  A registration records the source kind (`task` or `pipeline_run`), the source id, the normalized
  outcome (FS-16.R3, R13), the raw template-defined label where one exists, and a bounded summary. It
  is unique per source and immutable once written. Arm evaluation reads only this layer, so the
  scheduler never reaches into pipeline internals and pipelines gain no dependency on the scheduler.
- **R9** `(planned)` — **Acyclicity is enforced inside the write transaction.** The reachability check
  that rejects a cycle runs in the same transaction that inserts or replaces a task's arms, so two
  concurrent writers cannot interleave into a cycle. A rejected write mutates nothing (FS-16.R15).
- **R10** `(planned)` — **Task state advances monotonically under compare-and-set.** Every task row
  carries a revision that only increases. Mutations are compare-and-set against the observed revision,
  an accepted outcome is immutable, and recovery may resume a pending transition but never rewrites
  history or reopens a finished task, following TS-09.R20.
- **R11** `(planned)` — **Publication follows the commit.** A `task_update` SSE event is published only
  after its authoritative commit (TS-03.R8, TS-01.R8). Its bounded payload carries the task id, the
  monotonic revision, the state, the outcome, and the attention reason; clients ignore stale revisions
  and refetch detail over REST. Reconnect hydrates the Tasks view through REST rather than replaying
  an event log, following TS-03.R17.
- **R12** `(planned)` — **Attachment authorization delegates to the context service.** A task
  attachment stores only the task id, the canonical reference id, and its bounded label and
  description. Task ids, assignees, arm state, and task state never enter reference identity, never
  synthesize a direct grant, and never appear in the global direct-share list (TS-01.R23, FS-15.R1).
  Reading an attached reference asks the context service to validate and read that reference for the
  task's assignee; this plane owns the membership answer, and the context service owns the read.
- **R13** `(planned)` — **Tools extend the one MCP authority.** The task tools register on the existing
  `/mcp` server with the existing per-launch scoped token and generation-scoped teardown (TS-04.R6–R7,
  TS-04.R17). Caller identity, target task ownership, and assignment are all server-derived; no tool
  argument names another agent, another agent's task, a filesystem path, or a raw SQLite key
  (TS-05.R14, R16). Tools return bounded structured JSON with stable outcome codes, and cursor and
  size bounds come from one shared limits module, following TS-04.R28. There is no second MCP server.
- **R14** `(planned)` — **HTTP follows the shared envelope and route discipline.** New routes return
  the shared `{"error":{"code","message","details"}}` envelope (TS-03.R3), serialize empty collections
  as `[]` (TS-03.R6), and are added to the TS-03 route inventory in the same completed change
  (TS-03.R5).
- **R15** `(planned)` — **Startup recovery is bounded and failing it is fatal to startup.** One sweep
  releases claims that no live work owns, re-evaluates unfinished tasks, and starts exactly once any
  ready task whose start was never confirmed. A per-task failure is isolated and parks that task
  rather than aborting the sweep (INV §7). Recovery failing as a whole is fatal to startup for the
  same reason it is for mail activations (TS-01.R20): an unreleased claim would be invisible for the
  life of the process and its work would silently never run.
- **R16** `(planned)` — **Storage is forward-only and does not cascade across domains.** Task, arm,
  attachment, and result rows arrive in one forward-only migration recorded in `schema_migrations`
  (TS-02.R6). Arms and attachments cascade from their task. Agent ids, pipeline run ids, and context
  reference ids are logical references without cascades, so deleting an agent, a pipeline run, or a
  reference never deletes task history and deleting a task never reaches into agent identity,
  transcripts, the archive, a pipeline run, or a canonical reference (TS-02.R23, TS-09.R14, FS-16.R18).

## 3. Interfaces & data shapes

**Durable rows** (`internal/state`, one forward-only migration):

- `tasks` — `task_id` TEXT PK, `project`, `display_name`, `instruction`, `target_kind`
  (`agent` | `launch`), `target_agent_id` nullable, launch fields (`role`, `backend`, `model`),
  `state` (`armed` | `ready` | `running` | `finished` | `dependency_failed`), `outcome` nullable
  (`success` | `failure` | `blocked` | `cancelled`), `outcome_summary`, `outcome_details`,
  `attention_reason`, `assigned_agent_id` nullable, `assigned_generation` nullable, `revision`
  INTEGER, `created_at`, `updated_at`, `started_at` nullable, `finished_at` nullable.
- `task_arms` — `arm_id` TEXT PK, `task_id` FK cascade, `kind` (`work_result` | `signal`),
  `source_kind` nullable (`task` | `pipeline_run`), `source_id` nullable, `satisfying_outcomes`
  (bounded closed-vocabulary set), `signal_name` nullable, `state` (`unsatisfied` | `satisfied` |
  `unsatisfiable`), `satisfied_at` nullable.
- `task_attachments` — `(task_id, context_ref_id)` PK, `task_id` FK cascade, `context_ref_id` logical
  reference, `label`, `description`, `created_at`.
- `work_results` — `(source_kind, source_id)` unique, `outcome`, `raw_label`, `summary`,
  `recorded_at`. Immutable once written.
- `activations` — gains nullable `source_id`, plus a partial unique index on
  `(agent_id, source_id) WHERE state='pending' AND kind='dependency'`.

**MCP tools** on the existing `/mcp` server, all with server-derived caller identity:
`create_task`, `get_assigned_task`, `report_task_result`, `cancel_task`. Stable outcome codes include
`task_not_found`, `not_assigned`, `already_reported`, `invalid_outcome`, `dependency_cycle`,
`target_ineligible`, and `validation`; an unauthorized task and an unknown task are indistinguishable.

**HTTP** (added to the TS-03 route inventory): create, list, and read tasks; cancel, retry, re-arm,
and delete a task; and fire a project-scoped signal.

**SSE**: `task_update` with `{task_id, revision, state, outcome, attention_reason}`.

## 4. Invariants

- **INV §1** — a stop, resume, switch, archive, or restart boundary must reset or republish this
  plane's derived state; a task's view of its agent is re-derived after the boundary, never assumed.
- **INV §2** — one canonical result-acceptance path (R7), one launch/resume/stop seam (R6), one
  activation executor (R5). Any second implementation of these is a defect.
- **INV §4** — the work-derived context route and the exclusive lifecycle claim are torn down on every
  exit path through one generation-scoped teardown.
- **INV §5** — `armed → ready` and `ready → running` are atomic claims, never check-then-act (R4).
- **INV §7** — the startup sweep and evaluation isolate per-task failures rather than aborting or
  amplifying (R15).
- **INV §8** — task state, outcome, and attention reason reaching the UI are bounded and
  in-vocabulary; a failure to start always surfaces rather than leaving work silently armed.
- **INV §9** — any in-memory index of armed work reseeds lazily from the durable table on first use
  and is never treated as authoritative.
- **INV §15** — the durable transition commits before the launch, resume, prompt, or stop it
  authorizes, and no retryable error is returned after an irreversible effect (R4).

## 5. Deviations & open decisions

- Nothing in this specification has shipped.
- **Pipelines converge only at the result layer.** Absorbing pipeline runs into tasks would require
  this plane to grow revisit and back-edge semantics, because a pipeline run is a cyclic walk over a
  frozen template with one active agent, while a task graph is acyclic with real fan-out. R7 and R8
  remove the duplicated result concept without forcing that. Revisit only with evidence that two run
  layers cost more than the cyclic model would.
- **No general condition or query engine.** Arms are a conjunction over registered results and fired
  signals. There is no expression language, no derived predicate, and no agent-facing graph query, so
  this plane cannot become a workflow DSL by accident.
- **The in-process bus stays non-durable.** This plane deliberately does not add an event log or
  replayable stream; durable rows plus a bounded startup sweep are the recovery mechanism (R2, R15).

## 6. Traceability

Anchors (planned): task, arm, attachment, and result rows plus their forward-only migration in
`internal/state`; arm evaluation and start scheduling beside the existing activation executor in
`internal/server`; the `dependency` activation kind on the shared activation primitive; the shared
result-acceptance path with `internal/pipeline`; scoped tools on the existing MCP server in
`internal/messaging`; attached-reference reads through `internal/contextref`; the Tasks view in
`ui/src/features/tasks`.
