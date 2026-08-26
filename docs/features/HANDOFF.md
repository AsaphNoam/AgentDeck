# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

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
- **Last reviewed code:** `895348e` (2026-08-26).
- **Branch:** `main`.

## Active change

**State:** none in progress.

## Changelog

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
- [ ] Run J2 and J9 in a real macOS browser to confirm the native folder panel opens in front,
  selects, and cancels (FS-04.A22); component tests stand in until then.
- [ ] Run J5 in a real browser to confirm a right-click anywhere on the projects canvas — including
  the padding frame below and beside the cards — opens **New project** (FS-02.A24); the stylesheet
  assertion stands in until then.
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
