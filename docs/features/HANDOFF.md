# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Bug investigation:** A 2026-08-27 field report of post-pipeline dashboard unresponsiveness is
  diagnosed below. The probable incident cause is a confirmed configuration-source refresh/refetch
  storm, with two confirmed transcript scaling paths as possible contributors. The inspected
  default home now has zero running rows; its dashboard shut down gracefully at 07:47 and four
  surviving browser tabs show `RECONNECTING`.
- **Review state:** Verification of the fixes for the review through `895348e`: seven of the nine
  findings are closed in code and test, and three residual items are listed under
  `## Review findings`. The two purported agent-facing `request_conflict` and `revision_conflict`
  codes were independently confirmed HTTP-only — neither `ProposeRun`, `ProposeTemplate`, nor
  `Report` reaches the call sites that raise them — so classifying the other three is correct and
  complete.
- **Active change:** None. Agent-facing retry classification and structured result delivery is shipped;
  FS-17 and TS-04.R30–R31 are Current.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  remain owed before claiming those adapters accept structured results.
- **Usability state:** The v0.2.2 → v0.2.3 user-facing delta was driven through a real browser on
  2026-08-27. Dependent work, the attention count, the concurrency budget and chat drafts all
  behave as specified; one **Must fix** and five **Worth fixing** items are listed under
  `## Review findings`. FS-02.A24 is closed; FS-04.A22 is narrowed to the native panel.
- **Last reviewed code:** `895348e` (2026-08-26).
- **Branch:** `main`.

## Active change

**State:** none in progress.

## Changelog

- **2026-08-27 — bug investigation:** Diagnosed the all-200/no-page-load report against the live
  default home, its 71 MB dashboard log, durable state, four surviving browser tabs, and the
  config-source/session/SSE paths. Unchanged source generations publish updates and globally
  invalidate active queries; file-writing agents can accelerate the loop through watched project
  roots. The log reached 100 duplicate config-source reads per minute while handlers stayed fast.
  Recorded the confirmed storm, two probable transcript amplifiers, and missing shutdown/hot-path
  observability below; added one skipped reproduction test. No product code or specification changed.
- **2026-08-27 — usability review:** Drove the user-facing changes released between `v0.2.2` and
  `v0.2.3` through a real Chromium against the release binary on isolated fixtures: dependent work
  end to end, the dashboard attention count, the task-concurrency budget, browser-local chat drafts,
  the directory-browse control, and the projects-canvas context menu. One **Must fix** and five
  **Worth fixing** findings recorded below; no blocker. The FS-02.A24 right-click gate is closed and
  the FS-04.A22 gate is narrowed to the native panel alone. J15–J17 were added to the journey matrix
  because FS-15, FS-16 and FS-17 shipped citing no journey at all. Full run:
  [`../archive/reviews/usability-review-run-2026-08-27-release-delta.md`](../archive/reviews/usability-review-run-2026-08-27-release-delta.md).
- **2026-08-26 — review:** Verified the fixes for the nine findings against the tree. Seven are
  closed with real regression coverage, including a three-level chain test that also exposed the
  restart re-evaluation having been a no-op because `TasksInStates` never populated `Arms`. Three
  residual items are recorded below. `make test`, `make build`, `make check-specs`,
  `git diff --check`, `npm test` (251 passing), and `npm run build` are green;
  `TestRetryRunsAgainOnTheSameAssignee` failed once in three full-suite runs and did not reproduce.
- **2026-08-26 — work:** `/work` found no active change and no waiting ready change, so no
  implementation started. The repository remains clean and ready for the next designed change.
- **2026-08-26 — fix:** Closed all nine review findings. Source evaluation now publishes and
  recursively propagates across pipeline and restart boundaries (INV §1/§10/§15); task start
  generations are confirmed at effect time and failed wake confirmation settles the attempt (INV
  §4/§5); context authorization has one transaction-safe predicate (INV §2); task and dashboard
  errors surface accurately (INV §8); and agent-tool retry coverage and its SDK boundary now match
  FS-17 (INV §10). The two review-named pipeline conflict codes are HTTP-only, not MCP refusals.
- **2026-08-26 — review:** Reviewed `e1e827b`, `b121fd0`, and `895348e` against FS-02, FS-16, FS-17,
  TS-03, TS-04, TS-05, TS-10, and every invariant class. Two **Must fix** and seven **Worth fixing**
  findings recorded below; `make check-specs`, the Go suite for the touched packages, `tsc --noEmit`,
  and `npm test` all pass, while `git diff --check` reports one trailing-whitespace line.
- **2026-08-26 — implementation:** Shipped the agent-facing tool result contract. Every MCP refusal
  now carries a centralized four-class retry hint, successful and refused JSON objects are mirrored
  into structured content without changing the text block, and registration-derived tests cover the
  complete tool surface and deferred output-schema boundary.
- **2026-08-25 — fix:** Closed all thirteen dependent-work review findings. Task starts now retain
  the runtime's true generation, cancellation and start serialize per task, failed dependencies
  propagate fully, dispatcher transitions publish after commit, failed cleanup retains ownership,
  attachment creation is atomic and bounded, and the Tasks UI and API cover their specified paths.
- **2026-08-25 — design:** Defined FS-17 (agent-facing tool result contract) and TS-04.R30–R31, and
  queued `docs/ready-changes/agent-tool-retry-classification.md`. Investigation of the "richer
  agent-facing orchestration API" idea found most of it already shipped; the remainder is trimmed in
  `../ideas.md`. The task-graph query exclusion (FS-16 §6, TS-04.R29) was reaffirmed by the user.

## Decisions needing your input

These are product decisions needed for a future change or shipped boundaries whose reversal needs
an explicit specification update. Remove an item when the human resolves it or queues that update.

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.

## Acceptance gates

- [ ] Run pinned, credentialed Claude and Codex chat/MCP/resume checks before claiming those combinations.
- [ ] Run pinned Claude terminal flags/hooks and live xterm journeys before claiming full terminal support.
- [ ] Run pinned OpenCode/OpenHands launch/credential checks before claiming those backends beyond fakes.
- [ ] Run J2/J9/J16 in a real macOS browser to confirm the native folder panel opens in front,
  selects, and cancels (FS-04.A22). Narrowed on 2026-08-27: a real browser confirmed the **Browse…**
  controls are present and enabled for `cwd` and the pending `add_dirs` entry in both the Settings
  project form and the New project modal, and that the onboarding wizard renders styled. Only the
  native `osascript` panel itself is still unverified, and it needs a human at the machine.
- [x] **Closed 2026-08-27.** A real Chromium confirmed a right-click anywhere on the projects
  canvas opens **New project** (FS-02.A24): eight background points including the padding frame on
  every edge and corner, while a card right-click still opens the card menu, and the menu opens a
  styled create modal. Evidence in the J16 section of
  [`../archive/reviews/usability-review-run-2026-08-27-release-delta.md`](../archive/reviews/usability-review-run-2026-08-27-release-delta.md).
- [ ] Run a task start, an assignment turn, and a reported result against the pinned Claude and Codex
      adapters before claiming dependent work works with real providers (FS-16 §6).
- [ ] Run one successful and one refused MCP tool call through pinned Claude and Codex adapters before
      claiming they accept structured tool results without losing the text block (FS-17.A6).
- [ ] Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude and
  Codex installations before promoting FS-08/TS-07 from Partial.

## Blocked on human

Live-provider acceptance is waiting for human authorization because it invokes real provider sessions
and creates disposable local configuration homes. On 2026-07-15 this machine has Claude Code 2.1.202,
the retired `claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new
`claude-agent-acp`, OpenCode, and OpenHands are not installed globally.

## Review findings

- **Must fix** — **Probable incident cause; refresh/refetch mechanism confirmed.** Field report
  (verbatim; AgentDeck version/commit unknown): “After running a pipeline it became Super
  unresponsive, something fishy is going on, nothing sus is being logged in the terminal (all 200s)
  but the pages don't load. Memory isn't chocked up, MacBook Pro showing 24 physical memory, 18
  memory, 5 cached and only 5 swap. It looks like the agent sessions might still be running and
  progressing, one of the windows I already have open show the notifications popping up, I just
  can't be sure”. The default-home log contains 768 `GET /api/config-sources` requests in its final
  hour and peaks at 100/minute; all completed in 0–5 ms. Four open `127.0.0.1:4317` tabs exactly
  match the four long-lived SSE handlers closed at shutdown, so fast 200s and live notifications do
  not rule out client saturation. `SourceManager` refreshes every retained generation each 30-second
  sweep and on debounced events beneath approved project roots
  (`internal/configsource/watch.go:33-55,87-112,124-165`), while `commit` publishes
  `config_source_update` even when `changedFields` is empty, contradicting its own changed-view
  comment (`internal/configsource/manager.go:254-279`). Every tab globally invalidates the source
  query (`ui/src/api/sse.ts:120-126`), and the closed New Agent modal keeps that query active
  (`ui/src/features/launch/NewAgentModal.tsx:95-98,109-118`). Pipeline agents writing project files
  can therefore amplify unchanged refreshes into repeated discovery/refetch/render work. Suppress an
  unchanged publication (or make the client ignore a genuinely empty/no-health-change update), then
  unskip `TestSweepDoesNotPublishUnchangedGeneration` and cover multiple mounted clients. Confirm
  whether FS-08.R15 needs a clarification that only materially changed generation/health state is
  announced; the current behavior defeats TS-07.R7's acceleration boundary and the bounded,
  non-blocking intent of TS-03.R9 (INV §8).

- **Worth fixing** — **Probable incident contributor; server amplification path confirmed.** Every
  create/write event in the sessions tree runs a whole-tree reconciliation
  (`internal/server/reconcile.go:43-55,75-93`), and that pass reads each complete transcript and
  rebuilds its assistant preview line by line (`:122-154`). Runtime streaming persists each
  normalized delta as another append (`internal/runtime/chat.go:1028-1067`,
  `internal/transcript/writer.go:89-129`), so an active pipeline can repeatedly rescan all historical
  transcript bytes while its stage agent streams. The incident log has no scan timing/file/byte
  telemetry, so contribution is not attributable after the fact. Debounce/coalesce session writes or
  reconcile only the changed session incrementally, and add a regression that streams many deltas
  with several retained transcripts while bounding scans and latency (FS-14.R16, INV §7/§9).

- **Worth fixing** — **Probable incident contributor; browser amplification path confirmed.** The SSE
  client appends every agent's `new_message`, not only the open chat
  (`ui/src/api/sse.ts:96-117`); each append clones the global per-agent transcript map and target
  array (`ui/src/store/transcriptStore.ts:99-134`). `CardGrid` subscribes to the whole transcript map
  and recomputes every visible card's latest text (`ui/src/components/grid/CardGrid.tsx:49,77-89,
  118-153,221-229`). Pipeline agents are intentionally ordinary agents (FS-14.R16), but their
  streamed deltas need not force full-dashboard transcript allocation and recomputation. The
  surviving tab state and continued notifications fit main-thread churn, but no performance trace
  survived, so causation remains probable. Store card previews separately/boundedly or select only
  per-card preview state, with a render-count/load regression for a busy multi-stage run
  (FS-02.R9, FS-03.R10, INV §8).

- **Worth fixing** — **Undetermined shutdown cause; missing observability confirmed.** The default
  dashboard logged a graceful shutdown at `2026-08-27T07:47:57+03:00`; only the signal-cancelled
  context reaches that path, but the log records neither PID nor SIGINT/SIGTERM/caller. It also logs
  no config-source refresh trigger/change count, session reconciliation elapsed/files/bytes, or
  browser event-handler backlog. The durable snapshot after shutdown has zero `running` rows; its
  only run is an older paused run with no current agent, so the reported AgentDeck pipeline sessions
  are confirmed not running now, but why the dashboard stopped is undetermined. Log the shutdown
  cause plus bounded active-run/agent counts, and sampled source-refresh/session-reconcile rates and
  elapsed work, so a future all-200 report distinguishes server churn, browser churn, and an external
  stop (TS-09.R13, TS-03.R9, INV §8/§9).

- **Worth fixing** — FS-17.A1 and A3, and half of A4, still describe verification that does not
  exist. A4's contradictory "byte-identical" wording is corrected and A2's guard now genuinely
  guards, but nothing in `internal/messaging` produces each code in R3's table from a real call to
  the tool that emits it (A1), asserts over that enumeration that no refusal leaks a non-participant
  identifier or a count/delay/deadline (A3), or makes one *successful* call per tool (A4).
  `TestRegisteredToolsShareResultContract` exercises only `session_unknown`, and
  `TestPipelineRefusalsUseDeclaredRetryClasses` calls the `pipelineToolError` helper rather than a
  tool. No test file cites `FS-17.A1` or `FS-17.A3`. Adjacent: A2's guard scans `"error"` literals in
  `internal/messaging` only, so the three dynamically forwarded `pipeline.ControlError` codes are a
  hand-written list (`internal/messaging/tool_result_contract_test.go:38-41`) and a new control-plane
  code would still reach an agent unclassified. Build the per-code enumeration, or narrow A1/A3/A4 to
  what the suite actually proves (FS-17.A1/A3/A4, **INV §10**).

- **Worth fixing** — a task can still sit in `starting` indefinitely on the two stop-failure
  branches, and no requirement says so. The lost-generation path now settles its attempt, but
  `startLaunchedTask` still returns without settling when stopping the launched runtime fails
  (`internal/server/task_dispatcher.go:208-215`), and `recoverStartAttempt` does the same after a
  failed reap (`internal/server/task_dispatcher.go:553-560`). Both are the right call for runtime
  ownership, but `starting` is not an attention state (`ui/src/schemas/task.ts:16`), so the person
  who created the task sees it stalled mid-start with no signal and no repair until the next server
  start. Either surface the state or record the indefinite outcome in FS-16 alongside R25's attempt
  bound (FS-16.R25, TS-10.R4, **INV §4/§8**).

- **Worth fixing** — `TestRetryRunsAgainOnTheSameAssignee` is flaky under full-suite load. It failed
  once in three `make test` runs with `409 a resume is already in progress` on its first Stop
  (`internal/server/task_http_test.go:390-392`), and did not reproduce in three further full-package
  runs at this commit or three at `f97cc89`, so it is not attributable to the fix. Diagnosis: the
  test polls the durable row for `running`, but `startLaunchedTask` holds the per-agent lifecycle
  claim across `confirmTaskStart`'s write and publish, so a Stop issued inside that window gets the
  designed conflict. Have the test wait for the claim to settle rather than only for the row
  (no invariant class).

- **Must fix** — J17: saving a project working directory that does not exist warns nobody. In
  Settings → Projects → Edit, setting `cwd` to a non-existent path and pressing Update closes the
  dialog and persists the path with no alert and no inline warning. The server already returns
  `warnings: [{code: "cwd_not_found", message: "directory … does not exist yet"}]` and
  `ui/src/features/settings/ProjectForm.tsx:107` already has the markup to render it — but
  `ProjectsEditor.tsx:69-72` calls `setWarnings(resp.warnings)` and `setOpen(false)` in the same
  success handler, unmounting the only component that shows it. The create path (`:78-81`) has the
  identical shape. The person discovers the broken project later, when a launch in it fails. This is
  pre-existing rather than new in this release, but the release added the Browse… control to this
  same form. Suggested fix: keep the dialog open, or surface the warning outside the dialog, when
  the response carries one; regression test that a `cwd_not_found` response renders visibly on both
  create and edit (FS-04.A22, FS-04 §6, **INV §8**).

- **Worth fixing** — J15: the Tasks view never says which agent is doing a launch-target task. The
  row's meta line reads only `launches implementer`; even once the task is `running` and the API
  carries `assigned_agent_id`, no id, name, or link to that conversation appears, because
  `ui/src/features/tasks/TasksPage.tsx:136` shows an assignee only when `target_kind === "agent"`
  (and then as a raw id). A person supervising work has no route from the task to the agent doing
  it. Suggested fix: render the assigned agent for both target kinds, linked to `/agent/{id}`
  FS-16.A8 (no invariant class).

- **Worth fixing** — J15: an armed task names its prerequisite by durable id. The row reads
  `Waiting on: task tk_8caec11d865acf33 → success` while the prerequisite's display name is rendered
  two rows above, because `waitingOn()` (`TasksPage.tsx:39-47`) formats `arm.source_id` directly.
  The product forbids exactly this elsewhere — J5 requires each card show its project title, not its
  durable id. Suggested fix: resolve the id against the task list already in hand and fall back to
  the id only when it is not there FS-16.R14/A8 (no invariant class).

- **Worth fixing** — J15: the New task form ignores the configured default role. `TasksPage.tsx:181`
  is `role || roleNames[0]`, so the select preselects `agentdecker` — the internal AgentDeck-expert
  role — rather than the configured `default_role`. `NewAgentModal.tsx:53-64` deliberately prefers
  the configured default, so the two launch surfaces disagree, and a person who does not notice the
  dropdown silently assigns work to the wrong kind of agent. Suggested fix: seed the select from
  `config.default_role` with `roleNames[0]` as fallback FS-16.A8, FS-04 (no invariant class).

- **Worth fixing** — J15: a parked task offers a Retry button that can never succeed. Retry renders
  for `dependency_failed` as well as `interrupted`, and on a parked task it is always refused with
  `422 "this task is parked by an unsatisfiable prerequisite; re-arm it instead"`. The refusal is
  correct and well-worded (FS-16.A11), and Re-arm sits in the same row — but offering a control that
  cannot work for that state costs the person a failed round trip to learn it. Suggested fix: render
  Retry only for `interrupted`, and give Re-arm the prominence on a parked row FS-16.A11 (no invariant class).

- **Worth fixing** — J16: the attention count does not pluralise its verb. With exactly one item it
  reads "1 task need attention". `ui/src/components/grid/CardGrid.tsx:35` pluralises the noun but
  not the verb. One-line fix FS-02.A26 (no invariant class).
