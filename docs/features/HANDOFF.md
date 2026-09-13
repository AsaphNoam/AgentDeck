# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in `../archive/state/`: the dated
[`HANDOFF-through-2026-09-12`](../archive/state/HANDOFF-through-2026-09-12.md),
[`-11`](../archive/state/HANDOFF-through-2026-09-11.md),
[`-10`](../archive/state/HANDOFF-through-2026-09-10.md),
[`-09`](../archive/state/HANDOFF-through-2026-09-09.md),
[`-07`](../archive/state/HANDOFF-through-2026-09-07.md),
[`-06`](../archive/state/HANDOFF-through-2026-09-06.md) and
[`-03`](../archive/state/HANDOFF-through-2026-09-03.md) files, plus
[`HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** None.
- **Release:** `v0.4.3` is tagged and published; **Release state** and the release record carry its
  contents. `v0.4.2` and earlier are in the state archive.
- **Review units:** `persistent-pipeline-orchestration` was reviewed 2026-09-13 and stays open on its
  findings; `/fix` may take it. `stop-telling-agents-to-poll` shipped without entering this queue on
  the operator's explicit 2026-09-10 instruction; it can be added later.
- **Work units:** `rename-product-to-deckhand.md` is Waiting to start: the AgentDeck → Deckhand rename with its
  one-time state migration, role rename to FirstMate, and two named read-compatibility paths.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport
  blocker; the ACP wait-list in `docs/ideas.md` holds the rest behind an adapter contract.
  Queue hygiene: `bump-pinned-acp-adapters.md` reads `State: Finished` but is still in
  `docs/ready-changes/` and absent from that directory's index; per its README a finished change's
  file is removed. Left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume; `New ideas` entries are available.
  Persistent pipeline orchestration and its mail extension are shipped and available for review.
  TS-01.R31–R33, TS-02.R34 and TS-04.R53 complete shared prompt preparation, bounded batches,
  transactional budget/read settlement, uncertain-delivery recovery and deferred retention.
  The clean legacy reset, descendant cancellation, project boundaries, subordinate stage
  coordination and ordinary stop/resume remain confirmed. Streaming agent thinking stays part-decided (live-only decided; rendering default and whether
  `plan` ships still open). The permanently unaddressable pipeline agent is the newest `New ideas`
  entry and needs `/design-feature` before code. The Deckhand rename is fully specified and promoted
  to the work queue; no design decision remains open for it.
- **Open findings:** `persistent-pipeline-orchestration` carries five Must-fix findings — stalled run
  cursor after deferred cleanup, unreachable dedicated coordination, dead pipeline-report sharing,
  stages rendered as their predecessor's work, and an unfenced run state machine — plus ten
  Worth-fixing. Also open: the
  injected-steer lifetime edge case, live-gate finding durability, provider-contract oracles, and the
  unverified OpenCode/OpenHands paths. See **Review findings**.
- **Bug reports:** BR-1, BR-2, and BR-3 are investigated and archived with this release; BR-3 is
  fixed and closed. BR-1's Codex model/effort defect is fixed and reviewed; BR-2 is fixed and closed.
  BR-1's still-open findings are listed above. Pinned Claude model delivery
  through `_meta` works; an ACP model `currentValue` can be stale and is no execution-model oracle.
- **State:** The file viewer's credentialed rendered forms are owed: journey J3 now carries the
  file-link steps (docked and transcript-width forms, the refusal branch, the dashboard-pane
  navigation), and none of them has been exercised against a real browser.
  Automated MCP contract verification is green. The full post-fix Claude/Codex acceptance
  matrix remains open and must not be called verified; the 2026-09-08/09 probes were limited contract
  and model-delivery checks, not that matrix. Real Claude and Codex steering is unexercised.
- **Branch:** `main`.

## Active change

**Change:** None.

**State:** `persistent-pipeline-orchestration` was reviewed 2026-09-13 and stays open on five
Must-fix and ten Worth-fixing findings. **Fix model:** difficult — Codex Sol.

**Changelog — 2026-09-13 (review):** Reviewed `persistent-pipeline-orchestration`
(`c18b43d..a7af504`). Classes 2, 5, 8, 10, 15, 16 and 17 had applicable surfaces; 1, 3, 4, 6, 7, 9,
11, 12, 13 and 14 had none in this diff. Four Must-fix: deferred task cleanup never re-drives the run
cursor and no control recovers a `finishing` run; a dedicated coordinator is invisible to its
standing owner through assignment, wait and read scope; `resolvePipelineReport` still requires a v1
attempt row, so no task-backed run can share its report; stage lineage makes every stage render as
the previous stage's delegated work. A second pass on abstraction and integration added a fifth
Must-fix: `UpdatePipelineRunCAS` validates no transition, six divergent caller-side terminal
predicates stand in for it, and `pauseStartupRun` can move a `stopping` run back to `paused` and
re-enable Retry. Ten Worth-fixing: unemitted per-stage cleanup projection, the retained v1
report/turn-end/exit engine and its tests, an unfiltered `PipelineStageTaskForAssignee`, assignments
rendered without prior stage results, unbounded run task reads, an unpublished run mutation on
result acceptance, "current stage task" open-coded eight times, five parallel `INSERT INTO tasks`
statements, leaking run locks with a `Reconcile` loop that never iterates, and `internal/messaging`
reaching past its own control-plane seam into pipeline tables. The task layer is soundly built; the
run layer no longer has a single owner. Go tests for `pipeline`, `state`, `server`, `messaging` and
`runtime` pass. No product code or specification changed; four settled 2026-09-12 changelog entries
were archived for the header budget.

**Changelog — 2026-09-13 (work):** Replaced pipeline execution with durable standing-owner stage
tasks, managed dedicated coordinators, task-owned reports and waits, explicit replacement, guarded
cleanup recovery, bounded waking/deferred mail, and a one-time checkpointed v1 reset. Removed the
old stage-result tool, lifecycle callbacks, wake veto and attempt-based supervision fallback. The
repository test/build matrix and the 436-test UI suite pass; credentialed provider/browser gates in
Acceptance gates remain explicitly open.

**Release state:** `v0.4.3` is published and verified on tag `8ad5261`. Release and CI runs passed,
the local distributable reports `0.4.3` with `sqlite_fts5`, and the GitHub Release carries the
darwin/arm64 archive, `install.sh`, and a manifest declaring `0.4.3` with its SHA-256.
The release shipped with five open Must-fix findings on the operator's explicit decision; all five
are now closed. The credentialed Claude and Codex journeys under
**Acceptance gates** are owed; real steering has never been exercised against a provider.

**Available by role:** `/review` has no eligible unit; `/work` may take
`rename-product-to-deckhand`; `/fix` may take `persistent-pipeline-orchestration` or BR-1;
`/design-feature` may choose an available or resumable idea. Queues are independent.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Acceptance gates

**Not blocking as of 2026-09-05.** These gates have not been run, and no agent may describe them as
verified, passed, or closed. The operator chose to let roles proceed with them open.

- [ ] Pinned real-provider stage-result/file-edit approval journey (FS-03.A26/J14).
- [ ] Post-fix credentialed Claude and Codex chat, MCP, resume, task, effective-model/effort, and
      reported-result checks. The historical 2026-07-26 run failed model precedence and is evidence,
      not closure for the current implementation.
- [ ] Pinned Claude terminal flags/hooks and live xterm journeys.
- [ ] Pinned OpenCode/OpenHands launch and credential checks.
- [ ] Real macOS native folder-panel checks (FS-04.A22/J2/J9/J16).
- [ ] Real-browser permission-pane and drag-refusal journeys (FS-02.A35/A43).
- [ ] Phase 7 federation matrix against real Claude and Codex installations.
- [ ] Real-browser worktree creation/launch and archive-with-uncommitted-work journeys (FS-19).
- [ ] Six-tab same-origin dashboard check against a `make dist` build (FS-02.A27).

## Blocked on human

- None.

## Review findings

### persistent-pipeline-orchestration — **Fix model:** difficult — Codex Sol.

- **Must fix** — a deferred cleanup effect never re-drives the run cursor, so a run can park in
  `finishing` or `stopping` for the rest of the process lifetime. **Where:**
  `internal/server/task_dispatcher.go` `finishTaskCleanup`/`reconcileTaskCleanup` complete a release
  or yield without telling the pipeline manager; the only callers of `Manager.Reconcile` are
  `dispatchTurnEnd` in the same file and `Manager.Startup`. **Trigger:** a stage report is accepted,
  the turn ends, `StopStage` fails once (busy handle, reap race), the failure is recorded with
  backoff, and the retry timer later succeeds — `CompleteTaskRelease` commits and nothing advances
  the run past `finishing`/`release_stage_task`. The same shape leaves a `stopping`/`cleanup_run` run
  short of `stopped` after `Startup` cancels its members. **Why it matters:** the run stops
  progressing with no agent left to produce another turn boundary, and there is no recovery control:
  `pipelineRunControls.RepairCleanup` is eligible only when `run.State == "stopping"`
  (`internal/server/pipeline_projection.go`) and `Manager.RepairCleanup` refuses anything else, so a
  stuck `finishing` run offers only Stop. **Requirement:** TS-09.R42 ("transient failure resumes
  automatically"), R41, `INV §10`, `INV §15`. **Suggested fix/test:** after a successful
  `CompleteTaskRelease`/`CompleteTaskYield`, resolve the stage task's run and reconcile it (finishing
  stop cleanup when the pending action is `cleanup_run`); regression test injects one `StopStage`
  failure, runs the cleanup pass, and asserts the next stage task exists.

- **Must fix** — a `coordination: dedicated` stage's coordinator is invisible and unmanageable to the
  standing owner it exists to serve. **Where:** `Manager.coordinatorTask`
  (`internal/pipeline/manager.go`) creates the child with `CreatedByKind: "pipeline_coordinator"` and
  no `CreatedByAgentID`, and `renderAssignment` (`internal/pipeline/assignment.go`) never mentions
  the child at all. `WaitForTasks` (`internal/state/task_waits.go`) refuses any source whose
  `created_by_agent_id` is not the caller, and `agentOwnedTask`
  (`internal/server/task_handlers.go`) requires `CreatedByKind == "agent"`. **Trigger:** start any run
  whose stage declares `coordination: dedicated`. The standing owner is never told a coordinator
  exists; `wait_for_tasks`, `get_task`, `list_tasks` and `cancel_task` all answer "no such task" for
  it. **Why it matters:** R49's "the standing owner yields to let it run" and "its result is
  delivered upward through durable task observation" cannot happen, so the owner reports the stage
  without the delegated work; the child likewise gets no standing-owner identity to report to.
  **Requirement:** TS-09.R39, TS-09.R49, `INV §10`. **Suggested fix/test:** render the child's task id
  and objective into the standing assignment and the owner's identity into the child's instruction,
  and make the bound standing owner an authorized observer of its coordinator in the wait/read scope;
  test a dedicated stage end to end through `wait_for_tasks`.

- **Must fix** — no task-backed run can create a pipeline-report context reference. **Where:**
  `Service.resolvePipelineReport` (`internal/contextref/service.go`) still resolves the friendly
  selector through `store.CurrentPipelineAttemptForAgent`, which reads `pipeline_attempts`; v2 runs
  write no attempt row, so the function returns "You have no current pipeline attempt with an
  accepted report" and the stage-task branch added directly below it is unreachable. **Trigger:** a v2
  stage agent calls `share_context` with the current-pipeline-report selector after its result is
  accepted. **Why it matters:** the `Read` side was adapted to render a task result, so the feature
  looks shipped while its only creation path is dead for every new run. **Requirement:** TS-09.R48,
  FS-15.R4, `INV §10`. **Suggested fix/test:** resolve from `PipelineStageTaskForAssignee` and the
  accepted task result first, keeping the attempt lookup only as the legacy fallback; test the share
  window on a task-backed run.

- **Must fix** — every stage task is rendered as "Stage work" belonging to the previous stage.
  **Where:** `advanceTaskStage`, `continueTaskStage` and `Replace`
  (`internal/pipeline/actions.go`) pass `ParentTaskID: current.TaskID`, so `task_lineage` chains the
  stages together; `pipelineTaskRunProjection`
  (`internal/server/pipeline_projection.go`) builds `childrenByParent` from that column and filters
  only the coordinator, and `RunBrowser.tsx` renders `task.work` as delegated stage work.
  **Trigger:** any run that reaches stage 2 — stage 1's card lists stage 2's task, whose children list
  stage 3's, and so on. **Why it matters:** the run page misattributes each stage as agent-created
  work of its predecessor and repeats the whole tail of the run under every earlier stage.
  **Requirement:** TS-09.R44, FS-14.R39, `INV §8`. **Suggested fix/test:** exclude tasks that have
  their own `pipeline_stage_tasks` row from the descendant projection; add a two-stage projection
  test asserting an empty `work` list.

- **Must fix** — the run state machine has no transition legality, only six divergent caller-side
  guards, and one of them can reopen a stopped run. **Where:** `UpdatePipelineRunCAS`
  (`internal/state/pipelines.go`) is a bare revision CAS: any caller may move a run from any state to
  any state. Legality lives in hand-written predicates that disagree — `Stop` and
  `pauseStartupRun` (`internal/pipeline/actions.go`, `manager.go`) treat only
  `completed|stopped` as closed, `OnStageTaskInterrupted` adds `stopping`, and
  `CreatePipelineStageTask` (`internal/state/pipeline_tasks.go`) uses
  `stopping|stopped|completed`. **Trigger:** a run is stopping when the server restarts; `Startup`
  calls `Reconcile`, `reconcileRunCleanup` fails on a store read or `CancelTask`, and
  `pauseStartupRun` CASes the run to `paused` because `stopping` is not in its closed set. From
  `paused`, `pipelineRunControls` offers Continue/Retry/Replace and
  `RetryInterruptedPipelineStageTask` — which requires exactly `state = 'paused'` — re-admits the
  stage task, so the dispatcher relaunches the agent of a run the operator stopped. **Why it
  matters:** Stop is the safety control; R42 makes `stopping` plus a closure revision a fence that
  creation, admission, wake, Retry and Re-arm must respect, and a reconcile error silently reverts
  it. The same rootless modelling shows up as roughly eighty bare state strings across
  `internal/pipeline`, `internal/state` and `internal/server`, where the task domain in this same
  change defines `TaskReady`/`TaskRunning`/`TaskWaiting` constants. **Requirement:** TS-09.R41,
  TS-09.R42, `INV §2`, `INV §5`. **Suggested fix/test:** give the run states and pending actions
  named constants and one `closedToNewWork` predicate, and push the legal from-states into
  `UpdatePipelineRunCAS` as a `WHERE state IN (...)` fence; test that a failed startup reconcile
  leaves a stopping run stopping and Retry refused.

- **Worth fixing** — accepting a stage result commits a run mutation that nothing publishes.
  **Where:** `handleReportTaskResult` (`internal/messaging/task_tools.go`) discards the
  `PipelineRunRecord` returned by `AcceptPipelineStageTaskResult`, and `internal/messaging` holds no
  pipeline publisher; the only `m.publish` on a report path is in the now-dead `reportStageTask`.
  **Trigger:** any stage report — the run moves to `finishing` with a new revision and an attention
  reason, and the Pipelines page keeps showing the pre-report state until the reporting turn ends.
  **Why it matters:** R17 requires every committed run mutation to publish a bounded
  `pipeline_update`; routing the report around the manager dropped the manager's publication and
  notification duties along with its run lock, and when the release stalls the staleness becomes
  permanent. **Requirement:** TS-09.R17, `INV §1`. **Suggested fix/test:** have the report path
  commit through a manager entry point that owns the lock and the publish, or hand the tool a
  pipeline update sink; assert a `pipeline_update` is emitted on acceptance.

- **Worth fixing** — "the current stage task" is reimplemented eight times. **Where:**
  `stages[len(stages)-1]` appears in `internal/pipeline/actions.go` (four sites),
  `manager.go`, `reconcile.go` (two sites) and as `out[len(out)-1]` in
  `internal/server/pipeline_projection.go`. Each site re-lists every stage task of the run and
  depends on an implicit `ORDER BY stage_index, attempt_number`. **Trigger:** none on its own; it is
  the shared cause of the `PipelineStageTaskForAssignee` finding above and of `Detail` and
  `Reconcile` disagreeing about which stage is current after a concurrent write. **Why it matters:**
  `pipeline_stage_tasks` already carries `idx_pipeline_stage_tasks_current`, a partial unique index
  on `(run_id, stage_index) WHERE state = 'open'` that defines "current" exactly, and no call site
  uses it. **Requirement:** `INV §2`. **Suggested fix/test:** add one `CurrentPipelineStageTask(runID)`
  accessor keyed on `state = 'open'` and route every caller through it.

- **Worth fixing** — five hand-written `INSERT INTO tasks(...)` statements now bypass the canonical
  task-creation path. **Where:** `internal/state/pipeline_tasks.go` (three) and
  `internal/state/pipelines.go` (two) each spell their own column list beside
  `CreateTaskWithAttachments` in `internal/state/tasks.go`; three distinct lists already exist.
  **Trigger:** this change added twelve columns to `tasks` (`execution_handle`, `pending_yield`,
  `wait_version`, the `cleanup_*` family, `execution_turn`), and each new site had to be audited by
  hand. **Why it matters:** the drift already happened — the coordinator and replacement inserts omit
  `created_by_agent_id`, `created_by_generation` and `ready_at` entirely, which is the direct cause
  of the dedicated-coordinator Must-fix above; the canonical path also carries the inherited-closure
  check these bypass. **Requirement:** `INV §2`, TS-09.R37, TS-09.R49. **Suggested fix/test:** route
  pipeline task creation through one shared `insertTaskTx` helper; assert a coordinator task records
  its standing owner as creator.

- **Worth fixing** — manager internals carry dead and leaking control flow. **Where:**
  `Manager.runLock` (`internal/pipeline/manager.go`) inserts a mutex per run id into `m.locks` and
  never removes it, unlike the server's refcounted `lockTaskStart`; `Reconcile`'s
  `for step := 0; step < maxReconcileSteps; step++` loop (`internal/pipeline/reconcile.go`) returns
  on all seven branches, so it never iterates and `maxReconcileSteps` is dead; `Manager.Detail`
  assigns `ListPipelineValues`'s error without checking it, then keeps building and returns a
  populated `RunDetail` alongside a non-nil error, and overwrites durable `CurrentAgentID` and
  `AttentionReason` fields in memory so callers cannot tell the view from the record.
  **Trigger:** long-lived servers accumulate one mutex per run ever touched; a `Detail` caller that
  ignores the error gets a half-built struct. **Why it matters:** the loop is exactly the mechanism
  that would drive a run to quiescence, so its presence hides the single-shot reconcile behind the
  stalled-cursor Must-fix. **Requirement:** `INV §7`, `INV §16`. **Suggested fix/test:** refcount and
  drop run locks, either make `Reconcile` loop or delete the loop, and return early on `Detail`'s
  read errors.

- **Worth fixing** — `internal/messaging` reaches past its own control-plane seam into pipeline
  tables. **Where:** `create`/`cancel`/`list`/`get`/`retry`/`rearm` in
  `internal/messaging/task_tools.go` go through the injected `TaskControl` interface, while
  `report_task_result`, `wait_for_tasks` and `get_assigned_task` call `*state.Store` directly, and
  the report handler routes itself on `ReadPipelineStageTaskByTask`/`ReadPipelineRun`. **Trigger:**
  none directly; it is why the report path has no publisher and no run lock. **Why it matters:** the
  MCP transport package now owns the pipeline-versus-ordinary domain decision and the run-revision
  read that guards the stage transaction, which TS-09.R35 assigns to the controller. **Requirement:**
  TS-09.R35, TS-09.R40, `INV §2`. **Suggested fix/test:** move the stage/ordinary routing behind the
  same `TaskControl` seam the sibling tools use.

- **Worth fixing** — retained cleanup state never reaches the stage view. **Where:**
  `ui/src/schemas/pipeline.ts` declares `stage_tasks[].cleanup` and `RunBrowser.tsx` renders it, but
  `pipelineStageTaskDetail` (`internal/server/pipeline_projection.go`) has no such field.
  **Trigger:** a release whose effect is retained with `cleanup_unsafe` set. **Why it matters:** the
  `cleanup_phase`/`cleanup_unsafe`/`cleanup_last_error` columns R42 says must expose human attention
  are written and never displayed. **Requirement:** TS-09.R42, FS-14.R44, `INV §8`, `INV §10`.
  **Suggested fix/test:** emit the per-stage cleanup projection the schema already expects and assert
  it in the run-detail handler test.

- **Worth fixing** — the v1 engine survives the cutover R46 said would remove it. **Where:**
  `Manager.Report`'s attempt branch, `reportStageTask`, `refuseReport`, `currentAttempt`, `OnTurnEnd`
  and `OnExit` in `internal/pipeline/actions.go` now have no production caller — this change unwired
  `OnTurnEnd`/`OnExit` in `internal/server/server.go` and `task_dispatcher.go`. **Trigger:** none at
  runtime; it is reachable only from `internal/pipeline/*_test.go`. **Why it matters:** `reportStageTask`
  is a second implementation of the stage-report validation `handleReportTaskResult` and
  `AcceptPipelineStageTaskResult` own, and its tests assert behaviour the product no longer executes,
  so the suite reports coverage the shipped path does not have. **Requirement:** TS-09.R46, `INV §2`,
  `INV §10`, `INV §17`. **Suggested fix/test:** delete the dead entry points with their attempt-era
  tests, and move any case still worth keeping onto the task report path.

- **Worth fixing** — `PipelineStageTaskForAssignee` can resolve a closed stage. **Where:**
  `internal/state/pipeline_tasks.go` — the query filters neither `p.state = 'open'` nor the task
  state and has no ordering, though its comment promises "only a live standing-owner assignment".
  **Trigger:** a stage dispatched onto an already-running standing agent takes `ClaimBorrowed`, whose
  release does not stop the runtime, so the next stage task is admitted under the same
  `(assigned_agent_id, assigned_generation)` and two rows match; `QueryRow` returns an arbitrary one.
  **Why it matters:** `Manager.OnPermissionEvent` then sees `stage.State != "open"` and silently drops
  the run's "awaiting permission approval" attention while the stage really is blocked on a prompt.
  **Requirement:** TS-09.R40, `INV §2`, `INV §5`, `INV §8`. **Suggested fix/test:** restrict the query
  to `p.state = 'open'` (and the assignment's live task states); test two successive borrowed stage
  tasks on one generation.

- **Worth fixing** — replacement and continuation assignments carry no prior stage results.
  **Where:** every v2 caller of `renderAssignment` (`internal/pipeline/manager.go`,
  `internal/pipeline/actions.go`) passes `attempts = nil`, so its "Prior structured results" block
  never renders. **Trigger:** replace an interrupted standing owner — the new agent starts on a fresh
  conversation with the stage objective, bound input values and one sentence telling it to inspect
  retained work, and no record of what earlier stages reported. **Why it matters:** the replacement
  has to rediscover the run's history from tools rather than from its frozen handoff.
  **Requirement:** TS-09.R39 ("relevant prior report summaries"), TS-09.R41. **Suggested fix/test:**
  render accepted prior stage task results into the assignment; assert a replacement assignment
  contains the previous stage's summary.

- **Worth fixing** — run cleanup and run detail read a run's tasks unbounded. **Where:**
  `ListTasksForPipelineRun` (`internal/state/tasks.go`) has no `LIMIT`, and
  `pipelineTaskRunProjection` then issues a `ReadTaskLineage` per task plus a recursive
  `ReadAgent`/`ReadRunning` per work node on a single-connection store. **Trigger:** a long run whose
  stages create many descendants. **Why it matters:** R42 specifies keyset-paged cleanup and R28
  specifies a bounded targeted read; both are whole-table reads here. **Requirement:** TS-09.R42,
  TS-09.R28, `INV §16`. **Suggested fix/test:** page the cleanup sweep and batch the projection's
  lineage/agent reads; assert the query count does not grow per descendant.

### BR-1 — **Fix model:** difficult — Codex Sol.

- **Must fix** — BR-1 finding state was not durable across concurrent roles (**confirmed**).
  **Where:** `docs/archive/reviews/live-provider-acceptance-2026-07-26.md` recorded the exact Codex
  model failure as a Must-fix on July 26, but the live acceptance session could not edit HANDOFF
  while another session owned the shared docs. The report and a contradictory effort design were
  then swept into `7d294fb` without the finding entering HANDOFF. **Why it matters:** the mandatory
  read order made the archived report invisible to every later role, so an already-detected critical
  defect was treated as an open gate for another six weeks. **Requirement:** workflow §§1.1/12,
  `INV §1`, `INV §10`. **Suggested fix/test:** a failed live gate must be recorded in HANDOFF before
  its role can close; if state-file ownership blocks that write, leave the role explicitly blocked
  rather than archiving the only finding. A commit/review that contains an acceptance report must
  reconcile every Must-fix in it with live state.
- **Must fix** — provider-contract claims can still be proved by a self-authored oracle
  (**confirmed**). **Where:** TS-04.R18 asserted a `model[effort]` request shape after inspecting
  `codex-acp`'s internal `ModelId` parser without tracing `session/new` to `threadStart`; FS-09.A15
  and `fakeacp` then checked only that AgentDeck emitted the asserted field. The fake accepts unknown
  request members that the pinned ACP decoder drops. The same oracle error recurred in the 2026-09-08
  finding-fix: it called ACP `configOptions.model.currentValue` an independent effective-model oracle
  and declared Claude `_meta` delivery broken without sending a prompt. A real prompt then showed
  stale `currentValue = opus` alongside requested Haiku/Sonnet in SDK init, assistant, and usage
  signals. **Why it matters:** design, implementation, review, and an adapter-local readback can all
  agree and remain wrong about the external provider; following the false Claude finding would add a
  redundant delivery path and change launch failure/order behavior. **Requirement:** `INV §11`,
  `INV §12`, `INV §17`. **Suggested fix/test:** require provider-behavior statements to cite a
  complete reachability trace or a credentialed prompt receipt; label ACP `currentValue` as adapter
  configuration evidence, not provider execution evidence; and add an independently derived contract
  oracle that rejects out-of-schema standard fields instead of mirroring `sessionNewParams`.
- **Worth fixing** — equivalent OpenCode/OpenHands fields remain unverified (**undetermined**).
  **Where:** neither CLI is installed. OpenHands model delivery has a separate `LLM_MODEL` env path,
  so it does not depend on the suspect ACP `model` member, but both adapters still receive an
  out-of-schema top-level `systemPrompt`; OpenCode also still depends on the top-level `model`.
  **Why it matters:** the same silent-ignore class may be live on surfaces explicitly advertised by
  AgentDeck. **Requirement:** FS-09.A6, `INV §12`. **Suggested fix/test:** keep the claims gated until
  each pinned CLI is installed and its model/prompt delivery is checked at the effective provider;
  remove any redundant unsupported top-level fields once their real mechanism is known.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
