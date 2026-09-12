# TS-10 — Work dependency control plane

**Status:** Partial
**Code:** `internal/state`, `internal/server`, `internal/messaging`, `ui/src/features/tasks`
**Absorbed:** —

## 1. Scope

The durable task, arm, attachment, and shared-result records behind FS-16, the evaluation that decides
when armed work becomes ready, and the start path that turns a ready task into a running agent. It
covers the `dependency` activation kind's contract on the existing activation primitive, the
dispatcher that admits ready work against the concurrency budget, the shared turn-end fan-out that
releases a reported result, the result layer that
pipelines and tasks both write, restart recovery, and the HTTP/MCP/SSE surfaces.

Out of scope: pipeline template authoring, routing, revisits, and recovery (TS-09); context reference
identity, canonicalization, and bounded reads (TS-05.R16 and the context service); agent launch,
resume, stop, and archive mechanics (TS-01). This specification consumes those seams and adds no
parallel copy of them.

## 2. Design & constraints

- **R1** — **One in-process authority, no second scheduler.** Arm evaluation and start
  scheduling run inside the existing server process alongside the activation executor and the pipeline
  reconciler. There is no second daemon, embedded scheduler database, message broker, external queue,
  or self-HTTP call, matching TS-01.R5 and TS-09.R1. `internal/state` remains the sole writer of
  `state.db` (TS-02.R2).
- **R2** — **Durable rows are authoritative; the bus is a fast path.** Arm satisfaction,
  task state, and start confirmation are decided only from committed rows. The in-process bus and the
  activation channel are latency hints that may be dropped without changing any outcome, exactly as
  FS-06.R9 treats the mail fast path. A dropped notification costs latency, never correctness.
- **R3** — **Evaluation is event-driven with a bounded startup sweep.** A committed result
  registration (R7) or signal fire re-evaluates only the arms that name that source. Startup performs
  one bounded sweep over unfinished tasks. There is no unbounded polling loop and no periodic full
  graph walk.
- **R4** — **Starting is a claim, an effect, and then a confirmation — three steps, not
  two.** `armed → ready` and `ready → starting` are single-statement conditional updates that decide
  availability and take the claim together, following the `ClaimMailActivation` pattern. The
  `ready → starting` statement also takes the exclusive assignment claim for the target agent, so
  admission, capacity, and exclusivity are decided together and a second task for the same agent
  cannot interleave between them (R18). The `starting` row durably records the start attempt: its
  attempt id, the `agent_id` and generation it reserves, whether that start will create, wake, or
  borrow the runtime, and its attempt count. Those are reservation fields: they identify the attempt
  for recovery and hold the exclusive claim, and they authorize nothing on their own. Confirmation
  promotes the reservation into durable assignee membership, which is what authorizes attached-context
  reads (R12, FS-16 §3); an abandoned reservation is cleared and never becomes membership. That
  row commits before any launch, resume, or prompt is issued (INV §5, INV §15). Only when the runtime
  is confirmed — the agent is registered live under the recorded generation and holds the assignment —
  does the task move `starting → running`. `running` therefore always means a confirmed live runtime,
  and a crash between commit and effect leaves a `starting` row naming exactly what to reap and
  retry rather than a `running` row that is not running. Confirmation is only meaningful inside the
  process that owns the runtime; it is never re-derived after a restart (R15). A failed stop or reap
  never clears the `starting` row: preserving the durable runtime claim takes precedence over settling
  the task, and startup recovery retries the reap before admitting the work again (INV §4, INV §15).
- **R17** — **Capacity is granted by a dispatcher, not by the arm evaluator.** Arm
  evaluation decides readiness only. A separate bounded admission step grants capacity to ready tasks
  in the order they became ready, up to the configurable install-wide budget (FS-16.R7, R21), and only
  starts that will create or wake a runtime consume a slot. A slot is held by the task's runtime claim
  and released when the task finishes, is cancelled, loses its agent (FS-16.R16), or abandons its
  start attempt. A deferral — no capacity, a lost assignment claim, a busy lifecycle — returns the
  task to `ready` with its admission order and attempt allowance intact and is never counted as a
  failed attempt (FS-16.R25), so contention cannot exhaust work. Slot accounting is derived from
  committed runtime claims and recomputed at startup, never held only in memory (INV §9), so it cannot
  drift into a permanent deadlock after a crash.
- **R5** — **The `dependency` activation kind declares its own contract.** The existing
  activation record (TS-01.R19, TS-02.R23) gains a nullable stable source work id, which the
  `dependency` kind sets to its owning task id and `mail` leaves empty. Its uniqueness key is one
  pending row per `(agent_id, source_id)`, not mail's one-pending-row-per-agent. Its retry policy is
  the one TS-01.R21 already reserves: a dependency activation remains actionable until its owning task
  confirms its start under R4, so a lost claim, a busy lifecycle, or a failed resume returns the
  activation to pending and the task to `ready` rather than retiring either. Repeated start failure is bounded and parks the task (FS-16.R8) instead
  of retrying forever. The record still carries no instruction, prompt, arm set, context reference, or
  retry counter as payload. Making a second kind real is a wider change than the record: the shipped
  state layer writes the mail kind as a literal in every activation statement, and the runtime's
  activation start accepts only `mail` and emits mail's prompt and mail's status detail
  (`internal/state/activations.go`, `internal/runtime/chat.go`). Those statements become
  kind-parameterized, and the per-kind instruction and status detail come from one code-owned table
  rather than a literal at the call site, so a third kind adds a row instead of a branch (INV §2) and
  no kind can inherit another's instruction by accident (FS-16.R26).
- **R6** — **Starting reuses the existing launch, resume, and stop seams.** A launch-spec
  task composes its launch through the existing launch composition helpers, and an existing-agent task
  resumes through the single resume seam that explicit resume and mail wake already use. Finishing a
  task always releases its runtime claim and its slot, and stops the agent through the shared stop
  seam (FS-01.R6, R34) only when the recorded claim says this task created or woke that runtime
  (FS-16.R4); a borrowed runtime is released untouched. For an agent-reported result that release and
  stop are deferred to the reporting turn's end (R19), never issued from inside the tool call. This plane takes the exclusive
  per-agent lifecycle claim (TS-01.R16) rather than inventing a second one, and reimplements no
  identity, permission, credential, environment, MCP registration, transcript, or cleanup logic
  (INV §2, TS-09.R6).
- **R7** — **The shared result code is the vocabulary and validation, not one transaction.**
  The agent-reportable vocabulary (`success`, `failure`, `blocked`), its bounded summary, details, and
  checks limits, and the staleness check that a caller still owns the work it is reporting on are
  defined once and used by both the task report tool and `report_pipeline_stage_result` (TS-09.R8–R9);
  duplicating those is a defect under INV §2. The accepting transaction is not shared, because the two
  are genuinely different writes: pipeline acceptance also upserts declared stage outputs into
  run-scoped values and sets that run's `await_quiescence` pending action, neither of which a task has
  (`internal/state/pipelines.go`). Each domain keeps its own transaction and calls the shared helper
  inside it. Registering a normalized outcome in the shared result layer happens in whichever
  transaction commits that domain's terminal state — for a task its result, for a pipeline its run
  reaching a terminal state, which is a later and separate event from a stage report.
- **R8** — **The shared result layer is a small keyed registration, not a second history.**
  A registration records the source kind (`task` or `pipeline_run`), the source id, the normalized
  outcome (FS-16.R3, R13), the raw template-defined label where one exists, and a bounded summary. It
  is unique per source and immutable once written. Arm evaluation reads only this layer, so the
  scheduler never reaches into pipeline internals and pipelines gain no dependency on the scheduler.
- **R9** — **Acyclicity is enforced inside the write transaction.** The reachability check
  that rejects a cycle runs in the same transaction that inserts or replaces a task's arms, so two
  concurrent writers cannot interleave into a cycle. A rejected write mutates nothing (FS-16.R15).
- **R10** — **Task state advances monotonically under compare-and-set.** Every task row
  carries a revision that only increases. Mutations are compare-and-set against the observed revision,
  an accepted outcome is immutable, and recovery may resume a pending transition but never rewrites
  history or reopens a finished task, following TS-09.R20.
- **R11** — **Publication follows the commit.** A `task_update` SSE event is published only
  after its authoritative commit (TS-03.R8, TS-01.R8). Its bounded payload carries the task id, the
  monotonic revision, the state, the outcome, and the attention reason; clients ignore stale revisions
  and refetch detail over REST. Reconnect hydrates the Tasks view through REST rather than replaying
  an event log, following TS-03.R17.
- **R12** — **Attachment authorization delegates to the context service.** A task
  attachment stores only the task id, the canonical reference id, and its bounded label and
  description. Task ids, assignees, arm state, and task state never enter reference identity, never
  synthesize a direct grant, and never appear in the global direct-share list (TS-01.R23, FS-15.R1).
  Reading an attached reference asks the context service to validate and read that reference for the
  task's assignee; this plane owns the membership answer, and the context service owns the read.
  Membership is a durable row on the task and is the only thing that authorizes the read. The
  per-launch MCP registration through which a live agent calls is generation-scoped and is torn down
  on every runtime exit, which is a different object with a different lifetime: a stop and resume
  destroys and rebuilds the registration while leaving membership untouched, so a resumed assignee
  reads exactly what it could read before, and a finished task keeps its route until the task is
  deleted. Teardown must never be written against membership.
- **R13** — **Tools extend the one MCP authority.** The task tools register on the existing
  `/mcp` server with the existing per-launch scoped token and generation-scoped teardown (TS-04.R6–R7,
  TS-04.R17). Caller identity, target task ownership, and assignment are all server-derived; no tool
  argument names another agent's task, a filesystem path, or a raw SQLite key (TS-05.R14, R16). A
  task's target is the one selector a caller may name, and it is a friendly recipient selector
  resolved server-side against durable identities through the same resolution coordination already
  uses (FS-06, FS-16.R12) — never a raw agent id, and never a selector naming a transcript, a
  generation, a path, or a key. Tools return bounded structured JSON with stable outcome codes, and cursor and
  size bounds come from one shared limits module, following TS-04.R28. There is no second MCP server.
- **R14** — **HTTP follows the shared envelope and route discipline.** New routes return
  the shared `{"error":{"code","message","details"}}` envelope (TS-03.R3), serialize empty collections
  as `[]` (TS-03.R6), and are added to the TS-03 route inventory in the same completed change
  (TS-03.R5).
- **R15** — **Startup recovery ends runtimes rather than adopting them, and failing it is
  fatal to startup.** The runtime registry is in-process and starts empty, and stale reconciliation
  deliberately leaves a still-live agent process as an unadopted orphan rather than re-adopting it
  (FS-01.R20–R21, `internal/runtime/registry.go`, `internal/runtime/reconcile.go`). There is also no
  durable generation for an ordinary running agent. Recovery therefore must not ask whether a
  pre-crash runtime is still live and holding an assignment; that question is unanswerable by
  construction, and any design that depends on the answer is wrong. Instead one bounded sweep:
  re-evaluates unfinished tasks; resolves every `starting` attempt by the runtime claim it reserved —
  a `created` or `woke` reservation has its leftover agent reaped through the ordinary stop seam and
  returns the task to `ready` within its attempt limit, while a `borrowed` reservation is never
  reaped, because that runtime belongs to someone else (FS-16.R4), and its task becomes `interrupted`
  since whether the assignment reached that conversation cannot be known; moves every `running` task to `interrupted` (FS-16.R16–R17); completes any
  release and stop a reporting turn never finished (R19); recomputes slot accounting from surviving
  claims; and releases claims that no live work owns. A per-task failure is isolated and parks that
  task rather than aborting the sweep (INV §7). Recovery failing as a whole is fatal to startup for
  the same reason it is for mail activations (TS-01.R20).
- **R16** — **Storage is forward-only and does not cascade across domains.** Task, arm,
  attachment, and result rows arrive in one forward-only migration recorded in `schema_migrations`
  (TS-02.R6). Arms and attachments cascade from their task. Agent ids, pipeline run ids, and context
  reference ids are logical references without cascades, so deleting an agent, a pipeline run, or a
  reference never deletes task history and deleting a task never reaches into agent identity,
  transcripts, the archive, a pipeline run, or a canonical reference (TS-02.R23, TS-09.R14, FS-16.R18).
  Deletion of a task is refused in the same statement that checks it whenever the task still holds a
  runtime claim or a `pending_release`, so the cascade can never remove the only record of a live
  runtime or an unfinished release (INV §4, INV §15).
- **R21** — **A deleted pipeline run needs no dependency fan-out, because its result
  already outlived it.** The shipped delete path refuses any run that is not `completed` or `stopped`
  (`ErrPipelineActive`, `internal/state/pipelines.go`), and a run reaching either state registers its
  normalized outcome in the same transaction that commits it (TS-09.R27, FS-14.R34). Every deletable
  run has therefore already resolved every arm waiting on it — satisfying those whose set it matched
  and making the rest unsatisfiable — before deletion is even possible. The registration is keyed to
  the source and holds no foreign key into `pipeline_runs`, so the row's removal does not take it
  away. No hook into the pipeline delete path is added, and none is needed: an arm cannot be left
  waiting on a run that no longer exists. Acceptance proves this rather than assuming it (FS-16.A14).

- **R18** — **Exclusive assignment is a durable index, not a scheduling accident.** A
  partial unique index over `assigned_agent_id` for tasks in `starting` or `running` makes
  FS-16.R2's one-active-task-per-agent promise a database guarantee. The exclusive per-agent lifecycle
  claim (TS-01.R16) serializes start effects but is released after each transition, so it cannot
  provide this on its own, and the `dependency` activation key `(agent_id, source_id)` deliberately
  permits one row per task. Losing the assignment claim leaves the task `ready`, not failed.
- **R19** — **Every terminal path commits a durable release intent before its stop.**
  An agent-reported result, a person-recorded result, and a cancel each commit the terminal task state
  and `pending_release` in one transaction while still holding the runtime claim and slot. The stop is
  the effect that follows, never part of the same transaction, because stopping a process cannot be
  transactional. Only the timing differs: an agent-reported result waits for that agent's turn to end,
  checked generation-scoped against the recorded assignee, mirroring the `await_quiescence` boundary
  pipelines already hold between `report_pipeline_stage_result` and stopping the reporter
  (TS-09.R9–R11) — without it an immediate stop would cut off the MCP response the reporting agent is
  still waiting on. A person-recorded result and a cancel have no turn to wait for and stop
  immediately after their commit. In all three cases the claim is released only once the stop has
  happened or the runtime has been reaped, and a `pending_release` whose effect never completed is
  finished by recovery (R15), so no terminal state can discard ownership of a live runtime (INV §15).
  Cancelling a `starting` task is decided by the same claim: the cancel commits with its release
  intent, and the in-flight attempt either had not yet produced an effect, or produces one that the
  standing intent immediately releases. The shipped turn-end dispatch invokes exactly one hard-coded
  consumer (`internal/server`), so this change converts that call site into a generation-scoped
  subscriber fan-out shared by pipelines and tasks rather than adding a second dispatch path (INV §2).
- **R20** — **Creator provenance is server-derived and durable.** Each task records the
  creator kind, and for an agent creator the stable `agent_id` resolved from the caller's token at
  creation, plus the launch generation as provenance only. Cancel authority for an agent compares the
  stable id, so a stopped-and-resumed agent retains it and a new generation is not a new principal;
  no tool argument may supply or override it (TS-05.R17, FS-16.R24).

- **R22** — **Retry eligibility is projected, never restated.** The store decides
  whether Retry can succeed on a task (FS-16.R23/R25) in exactly one place, and read and list
  project that same decision onto the task JSON as `retry_eligible`. It is derived, not stored: no
  column holds it, and it is recomputed from the task's state and its arms on every read. Any
  surface that offers or withholds Retry reads the field rather than re-deriving the condition,
  because a second copy of the switch across the language boundary had already drifted once —
  narrowing Retry to `interrupted` and silently stranding work parked by exhausted start
  attempts (INV §2).

- **R23** — **A task's launch specification is validated by the launch
  composer's own seam, in both places, and is never re-implemented.** The stored effort reaches the
  provider by setting the existing `launchRequest.Effort` on the call the dispatcher already makes,
  so `resolveEffort` and `config.ValidateModelEffort` stay the only precedence and validation code
  and this plane adds no second copy of either (INV §2). Creation-time validation (FS-16.R28,
  FS-09.R49) calls the same `config.ValidateModelEffort` against the backend and model the launch
  composer would select — including the install defaults for an omitted field — rather than
  reimplementing selection: selecting the concrete backend/model target and resolving the effort
  over it are two shared seams, and every path runs both, in that order. Launch composition calls
  the selection seam, resolves the backend's bound configuration source, then calls the effort seam
  with that source's override, because a bound source has to be resolved between choosing the model
  and applying effort precedence. The two authoring paths, which have no source to consult, call the
  one composed helper that runs the same two seams back to back. Neither seam exists twice, and the
  precedence and capability checks live only inside them. The composed helper
  reads the catalog and takes no transaction, so a task-creating request stays a single write. The
  check is advisory by construction: it runs against the catalog as it is at creation, and the
  authoritative check remains the one inside launch composition, which cannot be bypassed.

- **R24** — **A task's fast mode rides the same composer seam, and its
  two-point check is capability-only.** The stored fast mode reaches the provider by setting the
  existing launch request's `Fast` on the call the dispatcher already makes, so `resolveFast` and the
  `internal/config` fast-capability validator stay the only precedence and validation code and this
  plane adds no second copy of either (R23's rule, INV §2). Creation-time validation (FS-16.R29)
  calls that same validator against the backend and model the launch composer would select, through
  the identical composed helper R23 already defines — one more check inside the existing seam, not a
  new path. What the two-point check can and cannot cover differs from effort, and the difference is
  the point: it validates *catalog capability*, which is knowable at creation, and it does not and
  cannot validate *session availability*, which only exists once a session is created. A task whose
  model declares fast-mode capability but whose eventual session does not advertise the option runs
  at normal speed and its attempt succeeds (FS-09.R55, TS-04.R45); the dispatcher records the
  applied value on the created agent and never treats it as a start failure or a reason to consume a
  start attempt.

### 2.1 Persistent assignment, dynamic work and durable waiting

- **R25** (planned) — **Task lineage is durable and server-derived.** Add immutable parent task,
  run, stage and creation-attempt provenance when a token-bound assignee creates work. Derive it from
  the caller's current assignment/managed execution, never caller-supplied ownership fields. Run
  members remain in one project and cannot detach from a live run. A reporting turn whose assignment
  has closed cannot fall back to creating unowned work. Store lineage separately from erasable task
  detail so a deleted intermediate task cannot hide descendants from cancellation or authorization.
  Lineage is provenance, not a dependency edge, and introduces no cycles into arms.
- **R26** (planned) — **Inspection and repair share one authority.** Bounded list/read operations
  expose state/revision, assignee selector, result, outputs, parent/run/stage and valid repair
  actions for owned work. Retry/Re-arm reuse existing state helpers. Creator authority survives
  resume; an assigned task can inspect/manage its descendants; the current run orchestrator can
  inspect/manage that run under TS-05.R22. No API supplies a creator, reporter, project override,
  or arbitrary context source. Unknown and unauthorized work share the same refusal. Results and
  lineage survive task-detail deletion, but deleted work cannot be re-armed/retried.
- **R27** (planned) — **Waiting is an unfinished assignment, not another task.** Add `waiting`
  plus a continuation-pending marker on ready/starting tasks, and a distinct `pending_yield` intent.
  `wait_for_tasks` accepts at most 64 `{task_id,after_revision}` observations of readable work; it
  atomically checks current assignment/execution, reads results/attention/deletion changes, and
  either returns those changes or persists the wait set and yield intent. The trigger is any watched
  task reaching a result or actionable interruption/park/deletion, not a conjunction of success
  predicates. An immutable terminal result is returned immediately rather than waited on forever.
  A self wait or wait on work targeted to the same reserved assignee returns `wait_conflict` with
  a repair message. Watch creation and result registration serialize in one state transaction, so
  completion between read and subscribe cannot be lost. Ordinary dependency arms remain unchanged.
- **R28** (planned) — **Yield releases runtime capacity, not assignment authority.** At matching
  turn-end, apply `pending_yield` through the existing release/stop seam. A created/woke runtime is
  stopped before clearing its capacity claim; a borrowed runtime is left intact. Preserve assignee
  membership, task identity and watch version. Represent runtime ownership/release separately from
  the exclusive assignment claim, which also covers waiting and continuation-ready work; ordinary
  ready tasks do not acquire that claim early. Outcome `pending_release` and unfinished
  `pending_yield` are distinct, mutually exclusive intents. Return the wait tool response before
  any stop, fence further task mutations from the yielded execution, and never set a task outcome.
  The existing task budget continues to count created/woke runtimes, with no pipeline exemption.
- **R29** (planned) — **Wake is ordinary admission of the same assignment.** A watched change
  commits a resume-needed marker keyed by task/wait version, checking the current execution handle,
  assignment generation and run/stage closure in the same CAS. Only after yield cleanup completes
  may one CAS return the waiting task to ready with continuation pending. Multiple changes coalesce;
  none may launch another task or resume while the old runtime still owns a capacity claim. Admission
  re-plans created/woke/borrowed ownership and generation and uses the shared resume plus dependency
  activation seam. Confirmation replaces the prior released runtime ownership; subsequent yield,
  cancellation and terminal release use this newly confirmed ownership, never the old claim.
  `get_assigned_task` exposes the saved assignment and watched changes; successful
  confirmation settles that continuation version. Prompt interruption before the agent acts leaves
  the assignment interrupted with results still readable. Wake/read do not delete the source results.
  Manual input or mail for a waiting assignee routes through this same assignment continuation before
  ordinary turn delivery; it must not create an untracked wake or bypass capacity/closure checks.
- **R30** (planned) — **Wait recovery never adopts processes.** Startup finishes pending yields
  through the existing generation/PID-checked reaper, retains settled waits, re-evaluates watched
  facts, and recomputes capacity from outstanding runtime claims. A settled stopped wait can become
  ready automatically. A pre-crash running/ambiguous borrowed assignment becomes interrupted under
  the existing recovery rules. Unexpected exit while running remains interruption; an exit matching
  committed yield is not interruption. Failed yield cleanup retains intent/claim and blocks wake
  until repaired by automatic reconciliation under R35. Cancellation wins over pending yield/wake
  and prevents any later continuation.
- **R31** (planned) — **Stages use the task result authority.** Add bounded named text outputs
  and shared checks to the task result store/read shape; preserve them in immutable result storage
  after task detail deletion. `report_task_result` uses one shared accepting transaction; a
  current standing-owner stage task additionally applies TS-09.R40's output/closure writes there.
  Descendant or coordinator lineage is not that association and never closes/advances a stage. No second
  stage report is registered. A non-secret execution handle combines task id and start/continuation
  attempt id and is rechecked with token identity/generation. It is required for stage reports and
  waits, and validated when supplied for ordinary task reports; existing standalone report clients
  remain valid without it. This closes same-agent/same-runtime stale-report races without treating
  an opaque id as authorization.
- **R32** (planned) — **Cancellation is scoped and replayable.** Task create/admit/repair/wake
  joins run/stage closure in its transaction. Run cleanup pages lineage, not agent creator history,
  and commits cancellations and release intents through the shared helper before effects. For a
  borrowed runtime, cancel only the task's generation/turn-scoped executing turn, wait for its
  settlement, then release the assignment without killing subsequent unrelated turns. Created/woke
  runtimes use the ordinary stop path. A task in terminal cleanup or yield cannot be deleted.
  Cancel and yield share one effect claim so they cannot stop a later resumed generation.
  Extend the existing agent-scoped cancellation seam with an expected generation/turn guard checked
  under its turn lock before both the initial cancel and escalation; an unguarded check followed by
  `Cancel(agentID)` is insufficient. Reuse the existing cancel→SIGINT grace policy, then bound the
  release attempt to ten seconds. Failure retains ownership/intent and enters automatic durable
  reconciliation under R35; it never hard-kills a later unrelated turn or immediately demands a click.
- **R33** (planned) — **Waiting and observation are bounded durable data.** Work list pages have
  a maximum of 100 items (default 25), waits 64 sources, result summaries/details/checks use existing
  shared limits, and named outputs reuse TS-09's text limits with a 256 KiB aggregate result limit.
  Scan/cancel/recovery batches are at most 100 rows with a shared in-flight cap of eight effects;
  cursors are stable keysets. Store one active wait set per assignment, replacing it by version;
  discard watches after completed continuation, terminal cancellation or task deletion. Source results
  and lineage retain existing history semantics. No periodic provider prompt, unbounded graph walk,
  in-memory event history or extra broker is introduced. A dropped bus hint changes only latency;
  startup and bounded state sweeps discover unsettled work.
- **R34** (planned) — **Expose changes through the existing surfaces.** On the current MCP
  authority add `list_tasks`, `get_task`, `retry_task`, `rearm_task`, `wait_for_tasks`; extend
  `get_assigned_task` and `report_task_result` as above. Control reads return explicit action
  eligibility and reasons; task HTTP/UI states include waiting and ready-to-resume, and stage
  controls link to the governing run action instead of presenting a guaranteed refusal. All tools
  share FS-17 structured results/classification and TS-04 registration/redaction. The paused direct
  transport migration is not a prerequisite. Mutation effects precede neither committed intent nor
  the tool's safe reply boundary. No new provider capability or ACP extension is required.
- **R35** (planned) — **Cleanup retries are durable reconciliation, not task retries.** The shared
  task release/yield/cancel path stores cleanup phase, effect key, consecutive failure count,
  first-failure time, next retry time, and bounded classified last error with its existing intent.
  One generation/intent-version CAS owns an effect at a time. Retry transient timeout, stop/reap
  or release-write failures automatically after 2 seconds, doubling to at most 60 seconds; restart
  preserves due times/counts and continues due work through the existing bounded dispatcher sweep.
  Actual completed cleanup progress resets that phase's failure streak; a process restart, a lost
  claim or a busy lifecycle does not. Contention consumes no failure attempt. Each effect retains
  R32's time bound and R33's shared in-flight cap. No provider polling turn or extra timer service.
  Escalate immediately when the classified condition cannot be repaired safely (for example,
  uncorroborated process ownership or denied stop permission), or after ten consecutive failures
  without progress, exposing the cause and required repair. Transient retries do not emit
  needs-attention notifications. After persistent-failure attention, safe reconciliation may keep
  checking at the capped interval and clear attention when repaired; no manual click is needed to
  recognize recovery. An unsafe effect waits for a relevant state change or explicit repair before
  another guarded attempt. Retry cleanup revalidates safety and retries only the retained effect;
  task execution attempts, outcomes, stage position and immutable reports are untouched. Never
  discard claims, advance a stage or declare Stop complete while a required effect remains unresolved.
- **R36 — superseded 2026-09-12 by FS-14.R78 and FS-06.R30–R36:** Separate coordinator-update
  records, continuation requests and delivery watermarks are withdrawn. Intervention awareness uses
  ordinary deferred mail under TS-01.R31–R33, TS-02.R34 and TS-04.R53.
- **R37** (planned) — **Coordinator replacement is explicit managed work.** Extend `create_task`
  with optional `replaces_coordinator_task_id`, accepted only from the current standing owner for
  its bound coordinator and only after the predecessor is terminal and its release is settled.
  Create a child beneath the standing assignment and CAS its coordinator binding in one transaction;
  a unique successor key makes request replay return the same child. Normal Retry retains the
  same coordinator task and identity. A successor may target the original agent for follow-up or a
  new launch spec; its standing owner supplies context through the assignment or ordinary mail,
  without automatic inbox or intervention-history transfer. It inherits only
  that coordinator's delegated scope under TS-05.R22. Neither operation changes the standing stage
  assignee, accepted reports, or immutable parent lineage. A cancelled/closed stage refuses it.

## 3. Interfaces & data shapes

**Planned additions (R25–R37; stop/resume confirmed 2026-09-12):**

```text
task_lineage: task_id PK, parent_task_id?, run_id?, stage_index?, stage_attempt?, project
task result: task_id, immutable outcome/summary/details/checks/outputs{}, lineage tombstone
task execution: existing assignment + execution_handle, continuation_pending, pending_yield,
                wait_version, resume_needed, runtime-release ownership separate from assignment
task_waits: (waiting_task_id,wait_version,source_task_id) UNIQUE, after_revision
task cleanup intent: effect_key, phase, failure_count, first_failure_at?, next_retry_at?, last_error?
create_task: existing fields + replaces_coordinator_task_id?
list_tasks: optional parent_task_id, cursor, limit; server-derived permitted scope
get_task: task_id; bounded result and provenance, including deleted-result tombstone
retry_task: task_id, expected_revision
rearm_task: task_id, expected_revision, arms[]
wait_for_tasks: execution_handle, observations[{task_id,after_revision}]
report_task_result: outcome, summary, details?, checks?, outputs?, execution_handle?
get_assigned_task: existing fields + execution_handle, stage?, inputs{}, output_declarations[],
                   prior_results[], observed_changes[], continuation_reason?
```

Reads page oversized collections; task ids/handles select records but never grant access. The
runtime-only activation instruction remains a fixed request to read the current assignment. No
general event stream, arbitrary condition expression, automatic task graph or child workflow DSL.
New refusal codes include `wait_conflict`, `assignment_stale`, `scope_closed`, and
`pipeline_control_required`; they are classified once under FS-17. Existing task creation and
ordinary HTTP routes keep their shapes; added fields are optional where compatibility requires it.
Result registration for a pipeline run remains distinct from its stage task's own result.

**Durable rows** (`internal/state`, one forward-only migration):

- `tasks` — `task_id` TEXT PK, `project`, `display_name`, `instruction`, `target_kind`
  (`agent` | `launch`), `target_agent_id` nullable, launch fields (`role`, `backend`, `model`,
  `effort` declared exactly `TEXT NOT NULL DEFAULT ''`, one forward-only migration adding it with the same
  non-null empty default the other effort columns use; empty means the launch composer resolves the
  level as it does for an unrequested one, FS-16.R27. The column is never nullable, because task
  reads scan it into a non-null string and a `NULL` would fail that task's read, so a later
  migration or repair keeps the `NOT NULL DEFAULT` shape a schema assertion pins),
  `state` (`armed` | `ready` | `starting` | `running` | `interrupted` | `finished` |
  `dependency_failed`), `outcome` nullable (`success` | `failure` | `blocked` | `cancelled`),
  `outcome_source` nullable (`agent` | `person`), `outcome_summary`, `outcome_details`,
  `attention_reason`, `created_by_kind` (`person` | `agent`), `created_by_agent_id` nullable,
  `created_by_generation` nullable, `assigned_agent_id` nullable, `assigned_generation` nullable,
  `runtime_claim` nullable (`created` | `woke` | `borrowed`), `pending_release` INTEGER,
  `start_attempt_id` nullable, `start_attempt_count` INTEGER, `ready_at` nullable,
  `start_claimed_at` nullable, `revision` INTEGER, `created_at`, `updated_at`, `started_at` nullable,
  `finished_at` nullable.
  `ready_at` is the admission order for the dispatcher; `runtime_claim` decides both slot accounting
  and whether finishing stops the agent; `pending_release` marks a recorded result whose stop and
  release its reporting turn has not yet completed (R19). A partial unique index over
  `assigned_agent_id WHERE state IN ('starting','running')` enforces exclusive assignment (R18).
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
`create_task`, `get_assigned_task`, `report_task_result`, `cancel_task`. `create_task` gains an
optional `effort` argument beside `role`, `backend`, and `model`; supplying it together
with `to` returns `validation`, as does a backend, model, or effort the catalog rejects at creation
(FS-16.R27, R28). Stable outcome codes include
`task_not_found`, `not_assigned`, `not_creator`, `already_reported`, `invalid_outcome`,
`invalid_state`, `retry_requires_rearm`, `dependency_cycle`, `target_ineligible`, and `validation`; an
unauthorized task and an unknown task are indistinguishable.

**HTTP** (added to the TS-03 route inventory): create, list, and read tasks; record a person result
(FS-16.R22); cancel, retry, re-arm, and delete a task; and fire a project-scoped signal. Every task
object carries the derived boolean `retry_eligible` (R22) beside its stored fields.

**SSE**: `task_update` with `{task_id, revision, state, outcome, attention_reason}`.

## 4. Invariants

- **INV §1** — a stop, resume, switch, archive, or restart boundary must reset or republish this
  plane's derived state; a task's view of its agent is re-derived after the boundary, never assumed.
- **INV §2** — one canonical result-acceptance path (R7), one launch/resume/stop seam (R6), one
  activation executor (R5). Any second implementation of these is a defect.
- **INV §4** — the per-launch MCP registration and the exclusive lifecycle claim are torn down on
  every exit path through one generation-scoped teardown. Durable task membership and the runtime
  claim are deliberately *not* in that set: they outlive a runtime by design (R12), and writing
  teardown against them would revoke authorization that a stop and resume must preserve.
- **INV §5** — `armed → ready` and `ready → starting` are atomic claims, never check-then-act, and
  admitting a task against the budget takes its slot in the same statement that grants it (R4, R17).
- **INV §7** — the startup sweep and evaluation isolate per-task failures rather than aborting or
  amplifying (R15).
- **INV §8** — task state, outcome, and attention reason reaching the UI are bounded and
  in-vocabulary; a failure to start always surfaces rather than leaving work silently armed.
- **INV §9** — any in-memory index of armed work or slot accounting reseeds lazily from the durable
  rows on first use and is never treated as authoritative. Liveness is weaker than it looks here in a
  specific way: after a restart there is no live-registry or pid answer to "did this runtime survive
  and is it still holding the assignment", so recovery must not ask it and must resolve a `starting`
  row from the durable record alone (R15).
- **INV §15** — the durable transition commits before the launch, resume, prompt, or stop it
  authorizes, and no retryable error is returned after an irreversible effect (R4). A recorded result
  commits with its pending release before the reporter is stopped, so the claim is never released
  ahead of the effect it authorizes and recovery can always finish an interrupted release (R19).

## 5. Deviations & open decisions

- **Task-backed pipeline replacement.** R25–R37 extend this plane under FS-14.R61–R77 and
  FS-16.R30–R38. They replace R7's separate pipeline acceptance transaction, R8's prohibition on
  pipeline/task coordination, R13's no-query surface, and R18's starting/running-only assignment
  index. They specialize R4/R6/R15/R17/R19 for unfinished waits while preserving normal task result
  and release semantics, and extend R16 for retained result/lineage tombstones. Acyclic task arms
  remain unchanged; the old result-only-convergence and no-query exclusions below end when the
  replacement ships. Runtime continuity is confirmed in FS-14 §6. R35 replaces the initial
  manual-first cleanup policy; R37 provides explicit succession. R36's separate coordinator
  awareness protocol is withdrawn for FS-06.R30–R36's mail extension. Deferred mail must not request
  a task continuation or satisfy a task wait by its mere arrival; R29's mail wake applies to waking
  sends only. Mail scope is confirmed in FS-06 §6; TS-01.R31–R33, TS-02.R34 and TS-04.R53
  complete its technical delivery design.

- **The dispatcher's notification path is the ticker.** Arm evaluation runs on the committing event,
  but the admission pass itself is woken only by its two-second sweep rather than by a channel, so a
  newly ready task starts within one tick. Durable rows are the authority either way (R2), and a
  channel is a latency change, not a correctness one.
- **Pipelines converge only at the result layer.** Absorbing pipeline runs into tasks would require
  this plane to grow revisit and back-edge semantics, because a pipeline run is a cyclic walk over a
  frozen template with one active agent, while a task graph is acyclic with real fan-out. R7 and R8
  remove the duplicated result concept without forcing that. Revisit only with evidence that two run
  layers cost more than the cyclic model would.
- **No general condition or query engine.** Arms are a conjunction over registered results and fired
  signals. There is no expression language, no derived predicate, and no agent-facing graph query, so
  this plane cannot become a workflow DSL by accident.
- **The concurrency budget is a resource policy, never a dependency semantic.** Readiness is decided
  entirely by arms; the budget only decides when a ready task is admitted. Nothing about the budget
  may narrow, reorder, or fail a dependency, so a graph's meaning is identical at any budget value and
  the setting can be changed at any time without changing what the graph says.
- **The in-process bus stays non-durable.** This plane deliberately does not add an event log or
  replayable stream; durable rows plus a bounded startup sweep are the recovery mechanism (R2, R15).

## 6. Traceability

Anchors: `internal/state/tasks.go` (rows, migration 18, admission, settlement, evaluation),
`internal/state/work_report.go` (the shared vocabulary, limits, and staleness check),
`internal/server/task_dispatcher.go` (admission, starting, turn-end release, recovery),
`internal/server/task_handlers.go` (HTTP and the agent-facing control plane),
`internal/state/pipelines.go` (`registerPipelineRunOutcomeTx`), the `dependency` activation kind on the shared activation primitive; the shared
result-acceptance path with `internal/pipeline`; scoped tools on the existing MCP server in
`internal/messaging`; attached-reference reads through `internal/contextref`; the Tasks view in
`ui/src/features/tasks`.

Projected retry eligibility (R22): `retryEligible` in `internal/state/tasks.go`, pinned against the
verb it describes by `TestRetryEligibleProjectionAgreesWithRetryTask`, and read by the Tasks view in
`ui/src/features/tasks/TasksPage.test.tsx`.
