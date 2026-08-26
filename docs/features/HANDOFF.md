# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Review state:** The review through `895348e` covers the two dependent-work fix commits and the
  agent-facing tool result contract. It found two **Must fix** and seven **Worth fixing** findings,
  listed under `## Review findings`. The thirteen earlier findings are confirmed closed except where
  a finding below names the path they missed.
- **Active change:** None. Agent-facing retry classification and structured result delivery is shipped;
  FS-17 and TS-04.R30–R31 are Current.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  remain owed before claiming those adapters accept structured results.
- **Last reviewed code:** `895348e` (2026-08-26).
- **Branch:** `main`.

## Active change

**State:** none in progress.

## Changelog

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

- **Must fix** — a pipeline prerequisite that fails, and a restart that re-evaluates arms, neither
  publish nor propagate. `PublishPipelineUpdate` discards the tasks `EvaluateSource` changed
  (`internal/server/pipeline_lifecycle.go:216-220`), and task recovery does the same for every armed
  task it re-evaluates (`internal/server/task_dispatcher.go:500-508`). Both call sites were left
  behind when the task-source path gained `publishTaskUpdate` and `propagateTaskFailure`. Normal-use
  trigger: a task armed on a pipeline run, with a task of its own armed behind it. When the run
  finishes outside the satisfying outcome set, the first task is parked `dependency_failed` in the
  database, the Tasks view and the dashboard attention count keep showing it as armed until a manual
  refresh, and the task behind it stays armed forever with nothing left to satisfy it — the exact
  two failures the previous review recorded, surviving on the pipeline and restart paths. Route both
  sites through the same publish-and-propagate loop `evaluateTaskResult` already uses, and add the
  three-level chain test FS-16.A4 names (one for a pipeline source, one across a restart); no test
  anywhere currently asserts a `task_update` publish or a second-level propagation
  (FS-16.R8/R13/R14/A4/A7, TS-10.R3/R11, TS-03.R28, **INV §1/§10/§15**).

- **Must fix** — the agent-facing pipeline tools emit five refusal codes the retry classifier does
  not know, so permanent refusals advertise themselves as retryable. `pipelineToolError` puts
  `pipeline.ControlError.Code` straight into the `error` field
  (`internal/messaging/pipeline_tools.go:95-101`), and `assignment_unknown`, `stale_assignment`,
  `validation_failed`, `request_conflict`, and `revision_conflict` are absent from
  `refusalRetryClasses` (`internal/messaging/tools.go:30-58`), so FS-17.R9 falls them through to
  `transient`. Normal-use trigger: a stage agent calls `report_pipeline_stage_result` after its
  attempt was superseded and gets `stale_assignment` with `retry.class: "transient"` — "a later
  identical call may succeed" — for a refusal that can never succeed; an AgentDecker proposing a
  malformed template gets the same advice for a pure caller error. This is the one failure mode
  FS-17 exists to prevent. FS-17.A2 promises a guard test that fails when an emitted code is
  unclassified, but `TestRefusalRetryClasses` only compares the map to a literal copy of itself
  (`internal/messaging/tool_result_contract_test.go:12-33`), so it can never detect this. Classify
  the five codes in FS-17.R3 and make A2's guard derive the emitted set from the source or from real
  calls (FS-17.R3/R8/R9/A2, TS-04.R30, **INV §2/§10**).

- **Worth fixing** — context-attachment authorization is now implemented twice, and the copies have
  already drifted once. `insertTaskAttachments` inlines the direct-grant-or-work-derived predicate as
  SQL (`internal/state/tasks.go:344-357`) that duplicates `ContextReadAuthorized`
  (`internal/state/context_links.go:373-398`) clause for clause; `e1e827b` shipped only the
  direct-grant half and `b121fd0` had to patch the work-derived half back in a day later. The next
  change to the authorization rule applied to one copy either lets an agent attach a reference it
  cannot read or refuses one it can. Extract the predicate into one exported SQL constant or a
  querier-taking helper both call, and assert the two agree for a work-derived reference
  (TS-05.R17, FS-16.R10, **INV §2**).

- **Worth fixing** — a woken task whose real generation cannot be recorded is left `starting`
  forever. When `SetTaskStartGeneration` reports no update, `startExistingAgentTask` releases the
  activation and returns without settling the row (`internal/server/task_dispatcher.go:293-301`), so
  no attempt is spent, no deferral happens, and no publish fires. Normal-use trigger: the woken agent
  exits between its resume and the generation write, so `registry.Generation` is empty. The task then
  holds that agent's assignment claim with `starting`, which is not an attention state
  (`ui/src/schemas/task.ts:16`), so a person sees a task stuck mid-start with no signal and no repair
  until the server restarts. Settle the attempt on that branch — fail it, since the assignment turn
  was already delivered — and cover it with an injected empty generation. The same
  stuck-in-`starting` shape is deliberate on the two stop-failure branches
  (`task_dispatcher.go:208-214`, `:524-531`); if it is meant to be indefinite there, FS-16 should say
  so (FS-16.R25, TS-10.R4, **INV §4**).

- **Worth fixing** — a borrowed runtime's generation is captured at planning time and never
  re-checked at effect time. `planExistingAgentStart` reads `registry.Generation` before admission
  (`internal/server/task_dispatcher.go:121-136`), and unlike the woken path
  (`:293-301`) the borrowed path confirms with that value unchanged. Normal-use trigger: the target
  agent is stopped and relaunched between admission and the worker goroutine's effect. The task then
  runs under a dead generation, `report_task_result` refuses it as `not_assigned` forever, and
  `interruptTaskForAgent` cannot match it on exit either, so the task holds that agent's claim
  permanently. Re-read the generation under the per-task lock and either confirm it matches or write
  it through `SetTaskStartGeneration`, exactly as the woken path does (FS-16.R4/R12, TS-10.R4,
  **INV §5**).

- **Worth fixing** — the dashboard attention link still reports "0 tasks need attention" when its
  query fails. `TaskAttentionLink` collapses an error into an empty list
  (`ui/src/components/grid/CardGrid.tsx:29-37`); the Tasks page half of this finding was fixed, this
  half was not. Normal-use trigger: the tasks request fails while parked work exists, and the
  dashboard actively asserts there is nothing to look at. Render an unknown state rather than a
  count, and test the failed query (FS-02.R44/A26, **INV §8**).

- **Worth fixing** — an unknown context reference is reported as an arm error in raw internal
  wording. `insertTaskAttachments` returns the bare `ErrTaskArmSource` sentinel for a missing
  reference (`internal/state/tasks.go:344-349`), and the HTTP mapper echoes `err.Error()`
  (`internal/server/task_handlers.go:357-358`). Normal-use trigger: a person mistypes the Tasks
  view's "Context reference (optional)" field and the form shows
  `state: task arm names an unusable source`, which names the wrong field and leaks the storage
  layer's prefix. Give the attachment case its own typed refusal and message
  (FS-16.R20, **INV §8**).

- **Worth fixing** — FS-17's acceptance criteria describe enumerations the shipped tests do not
  perform, and A4 contradicts R1. A1 requires every refusal code in R3's table to be produced by a
  real call to the tool that emits it; A3 requires assertions over that same enumeration; A4 requires
  one successful and one refused call per tool. `tool_result_contract_test.go` calls each registered
  tool once with an unknown session, so it covers exactly one code and no successful call. Separately,
  A4's "byte-identical to the encoding produced before this change" is false by construction, because
  R1 adds `retry` to every refusal's text block; the criterion should say the delivery change adds
  nothing to the text channel. Build the per-code enumeration and correct A4's wording
  (FS-17.A1/A3/A4, **INV §10**).

- **Worth fixing** — argument-decoding refusals bypass the result contract entirely. When a caller's
  arguments fail schema validation or unmarshalling, the MCP SDK answers before any handler runs and
  returns a plain-text `IsError` result with no `ok`, no `error` code, no `retry`, and no
  `structuredContent` (`go-sdk@v1.6.1/mcp/server.go:325-337`). Normal-use trigger: a model sends
  `check_messages` a string `limit`. FS-17.R8 claims the contract covers the whole agent-facing
  surface, so either wrap `tools/call` so these answer in the shared shape, or record the exclusion
  in FS-17 §6 (FS-17.R1/R8, **INV §10**).

- **Worth fixing** — `git diff --check` fails on the reviewed range: trailing whitespace at
  `ui/src/api/tasks.ts:108`. Workflow §2 lists that check as required, so the range did not pass its
  own gate (no invariant class).
