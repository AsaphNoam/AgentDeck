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
  collapsible tasks (`fb715a1`..`3d9ab39`, reviewed 2026-10-08) is closed after its six
  lifecycle, lineage and acceptance-recovery findings were fixed. Clone first-message failure
  investigation (2026-10-08): its diagnostic gap is fixed; the field failure stays open, not
  reproduced live, blocked on field evidence (see Review findings, Blocked on human). Chat links,
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
The six review findings are closed. Key seams: `pipeline/rooms.go`,
`state.AcceptThinkTankStageOutput`/`commitStageResultTx`, `insertPipelineStageRowsTx`,
`requirePipelineRoomOpenTx`, and `server.StopRoom`.

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

The v0.11.0 release published its three assets after the rerun, but
its body is still empty (checked 2026-10-08): set the notes from
[`RELEASE-v0.11.0-notes.md`](../archive/state/RELEASE-v0.11.0-notes.md) with `gh release edit`
when publication is authorized. Do not retag or recut a new version.

Clone first-message failure needs field evidence: on the next occurrence, with a build that has
TS-04.R85, capture the `runtime: provider prompt failed` line from `~/.chuck/dashboard.log` and
the failing request's URL and response body from the browser Network tab. Four live Claude
variants did not reproduce it.

## Review findings

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
  **Fix run 2026-10-09 — not reproduced; blocked on field evidence (§3).** The provider was
  Claude: Codex refuses clone (`clone_unavailable`). On an isolated v-main server
  (`CHUCK_HOME` scratch, port 4399) live Claude Sonnet clone → first prompt succeeded with
  history intact for a running source, a stopped source (clone named Atlas), a clone of a
  clone, and a woken stopped clone; no 409 and no WARN was logged. The live-process prompt
  failure now logs `runtime: provider prompt failed` with agent, backend, RPC code and data key
  names (TS-04.R85), so the next occurrence's `dashboard.log` identifies it. Keep the
  no-retry/no-ownership-change constraint above.

The observability **Worth fixing** finding was fixed 2026-10-09 (see Changelog).

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

- **2026-10-09 — Clone first-message investigation: diagnostic gap fixed (INV §8/§11).**
  A live-process `session/prompt` RPC failure now logs one bounded diagnostic (agent, backend,
  operation, RPC code, sorted data key names, recognized reason) and adds only a recognized
  reason to the transcript (TS-04.R85); `TestPromptFailureLogsBoundedDiagnostic` proves no raw
  data or prompt text leaks. The field failure was not reproduced in four live Claude clone
  variants and stays open, blocked on evidence. `make test` (both Go variants), `make build`
  passed.

- **2026-10-08 — Think Tank stage review findings closed (INV §1/§2/§4/§5/§7/§8/§9/§10/§15/§16).**
  Stop now durably retries room close/cancel and generation-scoped claimed-launch teardown; room-turn
  tasks inherit stage lineage only for the captured turn. Producer results and judge submissions
  refuse oversized later-room context before immutable success, with same-turn judge correction.
  Acceptance source, pending intent and three-attempt budget survive manager restart; the bounded
  sweep skips retained holds and terminal runs. Focused pipeline/state/runtime and server Think Tank
  tests passed, including injected failure, restart, private-turn and >64-row recovery cases.
  The broader live-provider and planned A49/A32 race gates remain open. Closure (2026-10-09):
  `make test` (both Go variants), `make build`, focused pipeline/state/runtime `-race` and server
  Think Tank/Stop tests passed.

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
