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
  Persistent pipeline orchestration and its mail extension are implemented and reviewed; fixes remain open.
  TS-01.R31–R33, TS-02.R34 and TS-04.R53 complete shared prompt preparation, bounded batches,
  transactional budget/read settlement, uncertain-delivery recovery and deferred retention.
  The design decisions for clean legacy reset, descendant cancellation, project boundaries,
  subordinate coordination and ordinary stop/resume remain confirmed; implementation gaps are
  recorded below. Streaming agent thinking stays part-decided (live-only decided; rendering default and whether
  `plan` ships still open). The permanently unaddressable pipeline agent is the newest `New ideas`
  entry and needs `/design-feature` before code. The Deckhand rename is fully specified and promoted
  to the work queue; no design decision remains open for it.
- **Open findings:** `persistent-pipeline-orchestration` carries seven Must-fix and seven
  Worth-fixing findings after the second review and consolidation: cleanup convergence, closure
  fences, execution-handle validation, managed-work authority, report sharing, stage projection,
  and deferred-mail activation. Also open: the
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

**Changelog — 2026-09-13 (workflow):** Clarified verified slice commits versus final closure,
required checkpoint/resumption notes and stable delegation ownership, and added an actionable-work
check before ending a turn. This administrative update leaves the product work and role queues intact.
Diff checks pass; `make check-specs` reports 14 existing finding-label errors, also present in `HEAD`.
Skill frontmatter is unchanged; its validator could not run because the available Python lacks PyYAML.

**State:** `persistent-pipeline-orchestration` was reviewed again and consolidated on 2026-09-13;
seven Must-fix and seven Worth-fixing findings remain. See **Review findings** for fix routing.

**Changelog — 2026-09-13 (second review and consolidation):** Reviewed
`c18b43d^..a7af504`, including the initial implementation omitted by the earlier recorded range.
Confirmed the earlier five Must-fix symptoms, broadened cleanup/closure and managed-owner findings,
and added execution-handle and deferred-only activation failures. Consolidated duplicate report
publication/control-plane and current-task findings; corrected the open-stage-only accessor fix and
the creator-column explanation; dropped unsupported Detail/loop claims while retaining the lock
leak. The intended cursor/task split is appropriate, but its cross-layer contracts remain incomplete.
Focused Go suites pass; no product code/specs changed and provider/browser gates remain unverified.

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

**Available by role:** `/review` has no unreviewed unit; the explicitly requested second review is
complete, with the same unit open for fixes. `/work` may take
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

Second review covers `c18b43d^..a7af504`, including the initial implementation and mail extension
omitted by the earlier recorded range. Seven Must-fix and seven Worth-fixing findings remain.
The cursor-over-tasks design is appropriate; the implementation does not consistently enforce its
ownership, closure and observation contracts. Fix these at the existing transaction/dispatcher
seams, without adding another scheduler or rewriting the task domain.

**Consolidation decisions:** All five earlier Must-fix symptoms remain supported. The cleanup
finding now includes missing descendant cleanup; the state-machine finding includes missing
transactional closure fences; coordinator visibility includes replacement authority. Merge report
publication and messaging-domain routing into that state-machine finding. Merge the repeated
current-stage lookup into the assignee-selector finding, but withdraw the proposed universal
`state = 'open'` accessor: release, report sharing and approval need closing tasks too. Keep
duplicate task inserts as a separate maintenance finding, withdrawing the claim that omitted
columns alone caused coordinator authority: the caller also supplies no owner, and immutable
creator history must not be rewritten on replacement. Drop the non-iterating reconcile loop as a
separate bug; a single-step event-driven reconciler is valid once its committed effects re-drive it.
Drop the `Detail` partial-result claim: it returns its read error, and inspected callers check it;
derived presentation fields alone are not a defect. Keep the independently verifiable lock leak.

- **Must fix — stage/run cleanup lacks a complete convergence path.** **Where:**
  `internal/pipeline/reconcile.go` `reconcileTaskStageRelease` waits only for the owner's
  `PendingRelease`; `advanceTaskStage` then creates the next stage or completes the run.
  It never cancels unfinished stage descendants or waits for their release/yield intents.
  `internal/server/task_dispatcher.go` `finishTaskCleanup` also completes deferred release/yield
  without re-driving the run. **Trigger:** accept success with a running/ready child, or let
  `StopStage` fail once and later succeed in the cleanup timer. The former advances with old work
  still alive; the latter parks `finishing`/`stopping` indefinitely. `reconcileRunCleanup`
  handles only `cleanup_run`, not stage completion. **Requirement:** FS-14.R71,
  TS-09.R40/R42/R43; INV §5/§10/§15. **Fix/test:** use one stage/run cleanup completion contract:
  page and cancel unfinished scoped members, retain claims until effects settle, and re-drive the
  cursor after every successful release. Permit next-stage/final writes only after all relevant
  cleanup settles. Verify a running child, a ready child, transient stop failure and startup
  cleanup; verify retained unsafe cleanup has an effective repair route, including `finishing`.
  Fix complexity: difficult.

- **Must fix — closure is not enforced at every transactional writer.** **Where:**
  `internal/state/tasks.go` `AdmitReadyTask`, `RetryTask`, `RearmTask`;
  `internal/state/task_waits.go` `WaitForTasks`/`NotifyTaskWaiters`; and
  `internal/state/pipeline_tasks.go` `AcceptPipelineStageTaskResult`.
  Creation checks inherited closure, but these paths do not. Result acceptance checks revision
  and stage state without checking run state; `internal/messaging/task_tools.go` reads the
  latest run revision itself, so a report after Stop can use the new revision and overwrite
  `stopping` with `finishing` before cancellation reaches its task.
  **Trigger:** a ready child or watched change survives stage completion/Stop, or a report arrives
  between Stop's fence and cancellation. Separately, failed startup reconciliation calls
  `pauseStartupRun`, which permits `stopping → paused` and re-enables Retry.
  **Requirement:** TS-09.R35/R40–R43, TS-10.R29/R32; INV §2/§5/§15.
  **Fix/test:** define the allowed state/action transitions and inherited closure checks once,
  enforce them inside each mutation transaction, and retain the stop fence on recovery errors.
  Route report domain decisions through the existing control-plane seam while keeping state
  acceptance atomic. Its post-commit path must publish the currently discarded run update
  (TS-09.R17, INV §1); no duplicate publication finding remains. Test Stop versus report/admit/wake,
  closed-stage retry/rearm, failed stopping recovery, and report-time SSE publication.
  Constants or a manager mutex alone do not repair the missing SQL fences. Fix complexity: difficult.

- **Must fix — stage reports lack execution-handle and yielded-execution fencing.** **Where:**
  `reportTaskArgs`/`handleReportTaskResult` in `internal/messaging/task_tools.go` have no
  execution handle; `AcceptPipelineStageTaskResult` checks neither that handle nor
  `pending_yield`. **Trigger:** an owner registers a wait then reports in the same turn,
  producing simultaneous yield and release intents; an old report on a reused generation can
  also target a later assignment because the handler resolves the currently assigned task.
  **Requirement:** TS-09.R40, TS-10.R28/R31; INV §5/§11.
  **Fix/test:** require and transactionally match the handle for stage reports, reject pending
  yield, and validate an optional handle for ordinary reports without breaking old ordinary
  clients. Exercise serialized stale-handle and wait-then-report calls and assert no partial
  result/run mutation. Fix complexity: medium.

- **Must fix — managed-work authority is still restricted to the original creator.** **Where:**
  `Manager.coordinatorTask` creates `pipeline_coordinator` work without creator agent identity;
  `renderAssignment` omits its id/objective and the child lacks the standing-owner handoff.
  `agentOwnedTask` in `internal/server/task_handlers.go` requires an agent creator, while
  `WaitForTasks` requires `created_by_agent_id == caller`. Replacement preserves old work
  but gives its new standing owner no authority to read/manage/watch that work.
  **Trigger:** any dedicated stage, especially at capacity one, or replacement of an owner that
  already created children. The owner cannot perform the required delegation/wait workflow.
  **Requirement:** TS-09.R37/R39/R41/R49, FS-14.R73; INV §2/§10.
  **Fix/test:** derive managed observer/control authority from the current standing assignment
  and retained run/stage binding, separately from immutable creation provenance. Include child
  identity/objective and upward report context in bounded assignments. Test dedicated
  owner→wait→child→resume and replacement reading/cancelling/watching predecessor work, with
  unrelated callers still refused. A generic insert helper alone cannot fix this.
  Fix complexity: difficult.

- **Must fix — task-backed reports cannot be shared.** **Where:**
  `internal/contextref/service.go` `resolvePipelineReport` requires
  `CurrentPipelineAttemptForAgent` before its task-backed branch. New runs create no attempt
  row. **Trigger:** `share_context` selects the current pipeline report after acceptance;
  it always refuses despite the task-aware renderer. **Requirement:** TS-09.R48, FS-15.R4;
  INV §10. **Fix/test:** derive the immutable task result from caller identity and the
  accepted-report-through-turn-end window. Preserve legacy tombstone semantics; do not use an
  open-stage-only query, since acceptance marks the stage closing. Test share/read within the
  window and refusal after release or from another generation. Fix complexity: medium.

- **Must fix — run supervision confuses stage succession with delegated work.** **Where:**
  `advanceTaskStage`, `continueTaskStage` and `Replace` link the next task to its predecessor;
  `pipelineTaskRunProjection` in `internal/server/pipeline_projection.go` recursively renders
  those lineage edges as `work`. **Trigger:** stage two appears under stage one's delegated work,
  with the remaining stage tail repeated under earlier cards. **Requirement:** TS-09.R44,
  FS-14.R39; INV §8/§11. **Fix/test:** distinguish stage-attempt succession from subordinate
  work in the projection, preserving immutable lineage. Test multi-stage, continuation and
  replacement histories against the serialized response. Fix complexity: medium.

- **Must fix — deferred mail keeps stale waking opportunities alive.** **Where:**
  `internal/state/activations.go` `ClaimMailActivation` tests any unread message;
  `internal/runtime/chat.go` `StartActivation` tests only `hasInlineMail`, and
  `PrepareInlineMail` can return deferred-only content. **Trigger:** send waking mail,
  consume it before activation, and leave one deferred FYI unread. A provider turn still starts
  just for the deferred message. **Requirement:** TS-01.R33, FS-06.R30/R33; INV §5/§15.
  **Fix/test:** use waking-source eligibility at claim and recheck it under the turn gate
  after resume/preparation; retire a stale opportunity without provider input, preserving the
  deferred row. Test both running and stopped recipients with consumed waking source plus
  deferred backlog. Fix complexity: easy.

- **Worth fixing — task construction is duplicated across pipeline transactions.** **Where:**
  five `INSERT INTO tasks` statements in `internal/state/pipeline_tasks.go` and
  `pipelines.go` duplicate `CreateTaskWithAttachments`'s persistence shape, with differing
  creator/ready-time columns. **Trigger:** any task schema/default change must update all paths.
  **Requirement:** INV §2, TS-09.R37. **Fix/test:** extract shared transaction-local task-row
  initialization/insertion, retaining specialized atomic run/stage writes and explicit ownership
  semantics. Do not route through a public method that opens a nested transaction or bypasses
  closure checks. Test first stage, continuation, coordinator and replacement initialization.
  Fix complexity: medium.

- **Worth fixing — per-run mutexes are retained forever.** **Where:**
  `internal/pipeline/manager.go` `runLock` appends to `m.locks` without removal.
  **Trigger:** a long-lived server starts/reads controls for ever more runs, including deleted
  runs. **Requirement:** INV §16. **Fix/test:** use the existing refcounted-lock pattern from
  `lockTaskStart`, dropping only after the last holder/waiter releases; test same-run exclusion
  and map reclamation. The single-step reconcile loop is not an independent correctness finding.
  Fix complexity: medium.

- **Worth fixing — retained cleanup diagnostics never reach stage supervision.** **Where:**
  `ui/src/schemas/pipeline.ts` and `RunBrowser.tsx` expect `stage_tasks[].cleanup`, but
  `pipelineStageTaskDetail` has no such field. **Trigger:** retained unsafe/persistent cleanup;
  the stored phase, unsafe flag and error are invisible. **Requirement:** TS-09.R42,
  FS-14.R44; INV §8/§10/§11. **Fix/test:** emit the bounded expected projection and verify the
  real serialized shape and rendered attention state. Repair behavior belongs to the cleanup
  Must-fix above. Fix complexity: easy.

- **Worth fixing — the old report/lifecycle engine and its tests survive the cutover.**
  **Where:** `Manager.Report`, `reportStageTask`, `refuseReport`, `currentAttempt`,
  `OnTurnEnd` and `OnExit` in `internal/pipeline/actions.go` have no production entry point
  after callback/tool removal. **Trigger:** maintenance or green tests against this dead path
  can be mistaken for coverage of the real MCP transaction. **Requirement:** TS-09.R46;
  INV §2/§10/§17. **Fix/test:** complete the production call-site audit, remove obsolete engine
  code/tests and move relevant behavioral assertions onto actual task report/control boundaries.
  Preserve context-reference tombstones and reset requirements. Fix complexity: medium.

- **Worth fixing — live assignee lookup and latest cursor lookup have different contracts.**
  **Where:** `PipelineStageTaskForAssignee` in `internal/state/pipeline_tasks.go` is
  unordered and unfiltered; manager/projection sites repeatedly list all stages to select the
  last. **Trigger:** two successive borrowed stage tasks share agent/generation; permission
  attention can select an old closed stage and be dropped. **Requirement:** TS-09.R40;
  INV §2/§5/§8. **Fix/test:** centralize bounded latest-cursor lookup and define separate
  eligibility for live attention/reporting versus accepted-report release/sharing.
  `dispatchTurnEnd` currently uses this same accessor after acceptance: blindly adding
  `state = 'open'` would break progression. The partial index is per stage, not a definition
  of the run's latest cursor. Test same-generation successive tasks, closing-stage turn-end
  and approval/recovery reads. Fix complexity: medium.

- **Worth fixing — persisted handoffs omit prior accepted task results.** **Where:** all v2
  `renderAssignment` callers pass nil attempt history, so the old prior-results block is dead.
  **Trigger:** continuation or replacement in a fresh conversation receives inputs/objective
  but no prior stage report summaries or concrete retained-work index. **Requirement:**
  TS-09.R39/R41; INV §10. **Fix/test:** render bounded task-result summaries and authoritative
  sources through the one assignment builder; test replacement and continuation with earlier
  accepted results. Managed-work authorization remains the separate Must-fix.
  Fix complexity: medium.

- **Worth fixing — run reads and cleanup are unbounded and amplify per-task queries.**
  **Where:** `ListTasksForPipelineRun` has no limit; `pipelineTaskRunProjection` reads
  lineage per task and recursively reads agents/running state. **Trigger:** long runs with many
  descendants or attempts. **Requirement:** TS-09.R28/R42; INV §7/§16.
  **Fix/test:** keyset-page cleanup and bound/paginate task history; batch projection joins.
  Filtering succession edges fixes misattribution, but does not bound these reads. Verify bounded
  page/query work on a large fixture. Fix complexity: medium.

**Verification and limits:** Existing Go tests for pipeline, state, server, messaging, runtime and
contextref pass (cached). Source/call-path inspection confirms the findings; this review added no
product code, specs or reproduction tests and did not run credentialed providers or browser journeys.
The invariant trigger sweep includes classes 1–5, 7–11 and 13–17 in the expanded range; class 6 has
no new runtime/adapter implementation (the existing interface gains guarded cancellation), and class
12 has no new external-CLI invocation surface. Applicable classes without new findings retain their
existing shared paths. No implementation local-choice note requires a new human decision.

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
