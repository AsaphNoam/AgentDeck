# Chuck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then the requirements they
name. Settled state is archived in
[`HANDOFF-through-2026-10-07`](../archive/state/HANDOFF-through-2026-10-07.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Release:** `v0.11.0` is tagged at `ddf8dda` and pushed with `main` (51 commits after the
  prior remote main). The user requested a minor release and authorized the push.
  `make test` (both Go variants), 692 UI tests, 41 presentation/style checks,
  `make dist VERSION=0.11.0`, FTS5 build-tag proof, shell syntax and old-name checks passed.
  Release run `37655214991` first failed creating the GitHub Release with transient GitHub
  HTTP 500s (workflow unchanged from v0.10.0); a later `gh run rerun --failed` succeeded and
  published `v0.11.0` with the arm64 archive, `manifest.json` and `install.sh`.
  CI `37655215773` concluded `success`.
- **Repository:** GitHub is still `AsaphNoam/AgentDeck`. Installer/updater defaults point at
  `AsaphNoam/Chuck` until the postponed rename; use `CHUCK_REPO=AsaphNoam/AgentDeck` and
  `chuck update --repo AsaphNoam/AgentDeck` meanwhile. Release CI publishes to the current repository.
- **Active change:** None.
- **Work units:** `migrate-internal-actions-from-mcp.md` remains paused on its transport blocker.
  `shared-pipeline-orchestrator-instructions.md` is waiting to start with approved specifications.
  Other available/resumable design work is in `docs/ideas.md`.
- **Review units / findings:** Think Tank pipeline stages, readable workspace consent and
  collapsible tasks (`fb715a1`..`3d9ab39`, reviewed 2026-10-08) remains open for six lifecycle,
  lineage and acceptance-recovery findings below, including FS-14 §6's four recorded gaps.
  Clone first-message failure investigation (2026-10-08) has an
  unresolved field failure and a confirmed diagnostic gap; see Review findings. Chat links,
  tables and tabs (`7661d97`..`70614c8`, reviewed 2026-10-08) is closed after its three
  verification/specification findings were fixed; closure evidence is in the changelog.
- **Known verification issue:** `internal/server` `TestOrdinaryStageAgentStopPausesPipelineRun`
  intermittently returns 409 "a resume is already in progress" (pre-existing at `74c8e84`);
  synchronization fix remains separate work.
- **Branch:** `main`.

## Active change

None. Think Tank pipeline stages, readable workspace consent and collapsible tasks finished
2026-10-08 (`fb715a1`..this closure commit); rendered evidence is reproducible with
`cd ui && node scripts/think-tank-stage-render.mjs [outDir]` (318 checks, stubbed API, no server).
Recorded gaps for review are in FS-14 §6 (Stop does not tear down setup runtimes claimed before
it; room-turn tasks are not attributed as stage descendants; produced-value context overflow is
caught when the room is created; acceptance retry count is in memory). Key seams:
`pipeline/rooms.go`, `state.AcceptThinkTankStageOutput`/`commitStageResultTx`,
`insertPipelineStageRowsTx`, `requirePipelineRoomOpenTx`, `server.projectRoomTask`.

## Acceptance gates still owed

- FS-14.A50 / FS-21.A31: bounded packaged Claude/Codex Think Tank pipeline-stage probe (room
  stage start → fresh participants → judge → accepted output → next stage). Fake-provider
  end-to-end (`TestPipelineThinkTankStageEndToEnd`) and the 318-check rendered pass are no live
  receipt. FS-14.A49 restart/race matrix and FS-21.A32 live Stop-cancel race remain planned.

- FS-21 / TS-06.R33: bounded credentialed Claude/Codex Think Tank smoke (room read/submit,
  addressed input, approval/denial, private Send/Steer, native resume, end-only judge and retained
  exact result). Automated/race closure and the fake-provider rendered journey passed 2026-10-07
  (60 main plus nine annotation-mail/deletion checks across appearances); fake ACP is no live receipt.
- FS-10.R25/A14 / TS-02.R41: supervised cutover rehearsal on a disposable real-home copy under
  `docs/chuck-cutover.md`, with receipt. GitHub rename to `AsaphNoam/Chuck` is still owed.
- FS-02.A46: real-browser toast click and manual macOS desktop-notification click; automated
  component/SSE coverage passes. FS-02.A27: six-tab shared-stream check, A46's real-browser J14
  pass, and Sky & Grove with Codex capabilities.
- FS-18.A12 / TS-11.R17: six manual role scenarios plus role-free follow-ups on pinned Claude
  and Codex. FS-18.A13: credentialed fresh/resumed Claude native-preset adoption; pinned adapter
  0.85.1 source proves object `_meta.systemPrompt` requests preset append, not its live adoption.
- FS-20.A1/A5/A6/A8: real tailnet, Android, iPhone and `pmset -g assertions`. iPhone Home Screen
  PNG touch icon remains absent. The 390px fake-provider pass covers A3/A4/A7/A9 except the
  fast-mode picker and Continue on approval pause. A10–A13 focused server/UI suites pass;
  combined 390px dashboard/project/agent-management/Files/Commands/retired-task journey is owed.
- TS-06.R21: credentialed Claude/Codex login/chat.
- TS-06.R31 / FS-09.A40/A42/A46/A47/R78 / FS-10.A10–A12: finite four-combination provider smoke
  (each provider with current Bundle and one current Installed CLI), plus Installed update →
  Refresh → model and missing Installed → Bundle → retry → Installed-with-overrides rendered
  journeys. Requires authorization/credentials. Claude Installed fresh launch at `claude-opus-5-5`
  passed 2026-10-05 (ACP 0.75.1, SDK 0.3.257, Claude Code 2.1.282, macOS, existing login).
- TS-06.R26/R32: credentialed Codex 2.1.1 receipt for FS-03.A41/A42 and FS-01.A20.
- TS-06.R32: two-point Claude/Codex smoke on ACP 0.85.1/2.1.1 and Codex 0.159.3, one notice,
  Steer during a running command (FS-03.A48's live half), and a refused model switch where
  account policy allows. Automated assembly/fake-runtime proofs passed 2026-10-06; v0.11.0
  release CI assembly and native-provider probes passed 2026-10-07. Live receipts remain owed.

## Blocked on human

No human decision is needed. The v0.11.0 release published its three assets after the rerun, but
its body is still empty (checked 2026-10-08): set the notes from
[`RELEASE-v0.11.0-notes.md`](../archive/state/RELEASE-v0.11.0-notes.md) with `gh release edit`
when publication is authorized. Do not retag or recut a new version.

## Review findings

### Think Tank pipeline stages, workspace consent and task collapse — review 2026-10-08 — **Fix model:** medium — Codex Terra or Claude Opus.

Reviewed `fb715a1^`..`3d9ab39`; the interleaved shared-standing-instructions design is a separate
planned unit, not an implementation obligation here. FS-14 §6 records deviations but does not
relax the shipped FS/TS requirements. The four recorded gaps therefore remain fix findings.

- **Must fix** — Stop can finish before claimed setup settles and leaves its runtime alive;
  fix complexity medium (INV §4/§5/§15).
  `internal/pipeline/reconcile.go:85–97` considers only running room attempts, not claimed
  participant setup or a reserved judge launch. `internal/server/pipeline_lifecycle.go:44–59`
  cancels only attempts with a turn id; `internal/server/think_tanks.go:363–445` records the
  completed launch without tearing down a newly idle runtime after Stop. Stop during a slow
  launch can therefore report stopped before launch finishes and leave that process registered.
  This violates FS-14.R84, FS-21.R42 and TS-09.R54. A temporary claim→Stop→Reconcile probe
  reproduced `stopped` while the participant was still `launching`. Track the existing setup/
  judge obligation through settlement, then use generation-scoped ordinary teardown; keep the
  run stopping until that effect settles. Test blocked participant and judge launches raced with
  Stop, including private work on a later generation.
- **Must fix** — room close/cancel effects are attempted once rather than recovered;
  fix complexity medium (INV §4/§9/§15).
  `internal/pipeline/actions.go:315–333` commits stopping, then logs a failed `StopRoom` call;
  `internal/pipeline/reconcile.go:100–121` never retries that call. A crash between those operations
  or a transient close failure leaves no durable room cancellation intent. With no running
  attempt, cleanup can mark the run stopped while the room is still in setup/discussion; with
  an active turn, it can wait indefinitely without sending cancellation. The server also logs
  guarded-cancel errors without retaining their retry obligation (`pipeline_lifecycle.go:54–57`).
  TS-09.R54/TS-14.R20 require durable closure/cancel intent and bounded cleanup recovery. An
  injected first-close failure followed by three reconciliation passes reproduced one call,
  a stopped run and an unclosed setup room. Commit the room closure obligation with Stop and
  recover its captured generation/turn effects through shared cleanup; test close/cancel failure
  and restart immediately after the stop fence.
- **Must fix** — tasks created during room turns escape stage lineage and cleanup;
  fix complexity medium (INV §2/§5/§10/§15).
  `internal/state/tasks.go:393–415` inherits pipeline lineage only from a task assigned to the
  creator agent/generation. The new room-backed stage has no assigned agent, and its participants/
  judge own room attempts rather than ordinary parent-task assignments. Their delegated tasks
  consequently have no stage parent/run lineage and can remain active after synthesis acceptance
  or Stop. FS-16.R48 and TS-10.R38 require inheritance from the authoritative room attempt in the
  creation transaction. Extend the existing lineage resolver with captured room generation/turn
  authority, same-project and closure checks. Test participant/judge-created work joining stage
  cleanup, post-Stop refusal and unrelated private-turn work retaining no pipeline ownership.
- **Must fix** — downstream context overflow commits producer success and strands the run;
  fix complexity medium (INV §8/§15/§16).
  `internal/state/pipeline_tasks.go:299–345` installs the successful immutable producer result
  before `internal/pipeline/actions.go:157–171` checks the next room's encoded context. A valid
  44,000-rune output of `<` exceeds the 256 KiB context bound after JSON escaping; a temporary
  probe reproduced an accepted successful Draft task and paused run with no pending action.
  Continue requires failure/blocked or an approval gate, so this hold provides no ordinary
  correction path for that immutable output. TS-09.R56 and FS-14.A52 require refusal before
  producer acceptance. Validate the prospective values against later room contexts in the shared
  result transaction, preserving the unaccepted producer and its correctable output; test escaped
  text, multi-input totals and both ordinary/room producer paths.
- **Must fix** — output-acceptance recovery loses its retry budget on restart;
  fix complexity medium (INV §1/§9/§15).
  `internal/pipeline/rooms.go:94–107,175–182` counts transient failures only in the manager's
  `roomOutputFailures` map. `accept_room_output` is installed only at exhaustion, so a restart
  before the third failure loses both retry progress and the required pending-acceptance state.
  An injected-write probe failed twice, recreated the manager over the same store and failed a
  third time; the run remained running with an empty pending action. Repeated restarts permit
  unlimited automatic attempts instead of the bounded durable recovery of FS-14.R86,
  TS-09.R53 and FS-14.A49. Persist acceptance intent/source and retry progress at the existing
  stage authority; test restart after each failure and explicit retry of the same synthesis.
- **Must fix** — held outputs can starve automatic acceptance of other rooms;
  fix complexity easy (INV §7/§16).
  `internal/state/pipeline_tasks.go:427–430` limits the sweep to 64 open stage rows without run
  eligibility filtering or deterministic ordering. `internal/pipeline/rooms.go:63–66` then
  skips exhausted acceptance holds and terminal runs indefinitely. Those retained rows can fill
  every batch, so a later room's lost completion kick never recovers (TS-09.R53). A temporary
  single-store reproduction used valid syntheses and injected transient write failures to hold
  200 runs through the normal three-attempt path, then removed the fault: all 64 selected rows
  were held, and the final active room remained unaccepted after the sweep. Filter ineligible/
  explicitly held runs before LIMIT, and order eligible work stably; test more than one batch
  with retained holds and terminal runs, proving eligible output progresses without manual Retry.

Verification: focused `go test ./internal/pipeline ./internal/state ./internal/server -run
'ThinkTank|RoomStage|Pipeline' -count=1` passed (server rerun outside the sandbox for its local
test listeners); focused pipeline/state `-race` passed. UI verification passed 73 tests:
`cd ui && npm test -- --run src/features/tasks/taskWork.test.ts
src/features/tasks/collapseStore.test.ts src/features/tasks/TasksPage.test.tsx
src/features/pipelines/RunStartForm.test.tsx src/schemas/thinkTankStage.test.ts` (71), plus
`npx vitest run src/features/pipelines/TemplateEditor.test.tsx` (2). The npm pretest also passed
all 41 style/presentation checks. `cd ui && node scripts/think-tank-stage-render.mjs
/tmp/chuck-review-think-tank-render` passed all 318 checks; representative warning/collapse
screenshots were inspected. This script renders stubbed states rather than driving a live
room/judge progression, so the existing live-provider and A49/A32 race gates remain owed.
Temporary expected-contract probes in `/tmp/chuck-review-stop-probe_test.go`, loaded with
`/tmp/chuck-review-overlay.json`, reproduced the four failures described above; the starvation
characterization lives in a temporary checkout at `/tmp/chuck-review-repro`. No product code or
specifications changed.

Local choices confirmed: one shared stage-result transaction and room creation helper; room
context as a durable first entry; frozen slot keys separated from ordinary assignments; ordinary
owner reuse across room stages; explicit desktop room recovery with phone route refusal;
cycle-safe parent-only collapse with persisted session choices and active-interaction protection.
No additional consent/collapse UI defect found. INV §1–§11/§13–§17 have applicable surfaces;
findings above cover §1/§2/§4/§5/§7–§10/§15/§16, and the other applicable classes have no separate
finding. INV §12 has no applicable changed external-command invocation.

### Clone first-message failure — investigation 2026-10-08 — **Fix model:** medium — Codex Terra or Claude Opus.

**Report (verbatim):** “cloning a chat gave it a generic name (Atlas), when I sent the agent a
message it didn't work - returned Internal Error. Looking in the console I saw Failed to load
resource: the server responded with a status of 409 (Conflict)”

Reported provider, failed URL/body, agent id, version and reproduction environment are unknown.
Investigation checkout: main, clean at startup; macOS. The local installed server is v0.11.0,
but it cannot be identified as the reported server: its two current sessions are Codex, neither
is Atlas, and neither `.chuck/dashboard.log` nor the legacy `.agentdeck/dashboard.log` contains
a clone request or a structured 409 access-log entry. No correlated provider diagnostic was supplied.
An optional request for provider and Network URL/body was sent; no answer at closure.

Naming is **confirmed works as specified** under FS-01.R4/R36: `clone.go:72` omits Name;
`launch.go:331` calls `suggestName`, whose first unused suggestion is Atlas. R36 does not carry
the source display name. This is independent of sending and is not a fix finding.

- **Must fix** — probable field behavior; root cause undetermined; fix complexity medium
  (no invariant class) — field route and root cause remain unidentified.
  The reported first message to a successful clone fails instead of continuing its conversation
  (FS-01.R36). `internal/server/sessions.go:27` accepts chat input via SendPromptOrHold;
  `chat.go:484` queues busy input rather than returning ErrTurnInFlight. A provider prompt RPC
  failure happens asynchronously at `chat.go:776` after the HTTP 202 and emits a protocol error.
  Thus the reported 409 cannot yet be attributed to that provider error: the exact request/body
  must identify whether it is a prompt/wake conflict or an unrelated control request. Do not
  change clone ownership or add retries based on the status alone. Capture the target id,
  failed route/code, transcript error and bounded provider diagnostic, then reproduce clone →
  first prompt on the reported provider and add the resulting regression. Native clone dates
  to `2c9bb8e` (2026-09-22); no evidence pins a regression to that change. Existing fake-provider
  tests `TestCloneForksTheConversationIntoANewAgent` and
  `TestForkLaunchCopiesHistoryThroughTheBoundary` pass; the latter sends and completes a clone
  turn. They do not reproduce the field failure or prove live-provider compatibility.
- **Worth fixing** — confirmed observability gap; fix complexity easy.
  `internal/runtime/jsonrpc.go:24–31` retains RPC code/data but Error() returns only Message;
  `internal/runtime/chat.go:776–790` emits only that string for a live-process session/prompt
  failure and logs no correlated diagnostic. A generic “Internal Error” therefore loses RPC
  classification/context at the user-visible boundary, while the captured provider stderr ring
  is not consulted on this path. This prevents determining whether a successful fork is rejected
  by the provider on its next turn. Add a bounded, sanitized diagnostic at the existing failure
  seam identifying agent, backend, operation and RPC code, plus allowlisted provider reason when
  present (workflow §12.5, TS-04.R12, INV §8/§11). Never dump raw Data, prompt text or stderr.
  Verify with a fake peer returning a generic message plus code/detail: the transcript remains
  safe and the diagnostic identifies the failure without secrets.

No product code, specifications or tests changed. Focused fake-provider tests passed with loopback
permission after the sandbox blocked the test listener. No live provider turn was started.

## Decisions needing your input

- Standardizing TS-03.R3–R4's mixed legacy error envelopes or TS-04.R3's provider model-ID
  ownership would be a compatibility change.
- Decide whether a pipeline stage paused after failed launch/resume keeps withholding **Open
  agent**, matching restart recovery, or permits chat under a wider continuation contract.

## Design consistency notes

- Next presentation review: decide whether crisp asymmetric technical-surface radii are deliberate
  under FS-12.R52.
- When the direct-action change resumes, align TS-01.R25/TS-03.R32 with TS-04.R40 and scope
  FS-17 section 6's opening sentence to the planned boundary.
- Injected-steer lifetime edge case needs `/investigate-bug` before `/fix`; FilesTab
  still copies silently through bare `writeText`.

## Changelog

- **2026-10-08 — Chat links, tables and tabs review findings closed (INV §10/§17).**
  Replaced the full-chat overflow literal with measured overflow and scroll movement, with matching
  archive/file-viewer checks and hidden-overflow/compressed-cell mutation proofs. The phone journey
  taps the link, checks separate-page navigation and unchanged conversation, and catches prevented
  activation. Clarified desktop three-appearance coverage versus Core-only phone coverage in FS-03,
  FS-12 and TS-08. `make check-specs`, script syntax and `git diff --check` passed; the stubbed
  browser journey passed all 175 checks. No product code changed. Ready for parent commit.

- **2026-10-08 — Think Tank stage implementation reviewed.** Recorded six open Stop, lineage,
  context-admission and acceptance-recovery findings; four were already declared deviations.
  Focused Go/race, 73 UI tests, 41 presentation checks and 318 rendered checks passed; temporary
  probes reproduced Stop settlement/retry, late context refusal, reset retry budget and sweep
  starvation. The unit remains open; fix recommendation is medium (Codex Terra or Claude Opus).

- **2026-10-08 — Think Tank pipeline stages, readable consent and task collapse finished.**
  Pipelines can run a stage as a fresh same-project room whose published judge synthesis becomes
  the stage output; shared-workspace consent and run-hero text are readable in all appearances;
  Tasks collapse per parent lineage. FS-14.R81–R86/A48/A50–A52, FS-16.R46–R48/A30–A32,
  FS-21.R41–R42/A31, TS-08.R92–R95, TS-09.R51–R56, TS-10.R38, TS-14.R19–R21 shipped (FS-16,
  TS-08, TS-10 now Current). Rendered pass fixed collapsed rows staying visible and hero text
  contrast. A timing-dependent mail activation test now waits for confirmed read state.
  `make test`, focused `-race`, 720 UI tests, UI build, `make build` and 318 rendered checks passed.

- **2026-10-08 — Shared pipeline instruction design ready.** User approved the optional template
  field as standing Claude system/Codex developer instructions, without ordinary-message bootstrap
  or stage repetition. Added planned FS-14.R87–R89/A53–A55, TS-09.R57–R60 and TS-11.R20;
  `shared-pipeline-orchestrator-instructions.md` is waiting to start. Guidance freezes into newly
  launched owner/coordinator session prompts and survives recovery/replacement; stage exceptions
  are explicit, roles/permissions are preserved and workers/rooms have no automatic inheritance.
  Removed the source idea; the separate Think Tank implementation remains active. Documentation,
  twin-skill and whitespace checks passed; no product code edited by this design.

- **2026-10-08 — Chat links, tables and tabs finished.** Web links open in new tabs with Open in
  new tab/Copy link on right-click, composed with annotation menus; Markdown tables get dividers,
  roomier cells and local scrolling on every renderer surface; Commands left desktop and phone
  agent views (`?tab=commands` opens Transcript) while tracking/API/history stay. FS-03.R78–R80,
  FS-05.R40, FS-12.R60, FS-20.R42, TS-08.R107–R110 shipped. `make test`, 697 UI tests, style and
  presentation checks, `make embed`, `make build` and the 161-check rendered script passed in
  Core, Sky & Grove and Studio.

- **2026-10-07 — Chat cleanup design ready.** User approved web-link new-tab/copy actions,
  local-file viewer preservation, readable tables and Commands-tab removal including phones.
  Added planned FS-03.R78–R80/A59–A61, FS-05.R40/A23, FS-12.R60/A32, FS-20.R42/A14 and
  TS-08.R107–R110; promoted the idea to `agent-chat-links-tables-and-tabs.md`, waiting to start.
  Existing react-markdown/GFM renderer is retained; command tracking/API/history are preserved.
  No active implementation or product edits. Documentation and twin-skill checks passed.
- **2026-10-07 — Operating skill audit fixes finished.** Corrected the terminal messaging/task
  boundary, clarified that existing task targets use current session settings, and documented
  explicit fresh-judge retry without reopening discussion. Guidance now matches existing behavior;
  no product or specification change. `go test ./internal/agentknowledge`, `make check-specs`, and
  `git diff --check` passed. All three approved audit items are closed.
- **2026-10-07 — Release preparation: v0.11.0.** Minor release for Think Tank workspace/live
  controls, quiet completed chat turns, UI polish and the 2026-10 provider refresh. Corrected the
  optional source install's Claude ACP pin to 0.85.1 and regenerated the UI embed through `make dist`.
  The shipped operator package already matched the range. Full Go/UI/build checks passed;
  archived the settled pre-release handoff and retained manual gates above. Main/tag pushed;
  macOS assembly/install proofs passed, but GitHub release creation/rerun returns HTTP 500.
  Release notes are preserved for publication recovery; the release is not published yet.
