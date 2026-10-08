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
  collapsible tasks (`fb715a1`..closure, finished 2026-10-08) awaits review; its recorded gaps are
  in FS-14 §6. Clone first-message failure investigation (2026-10-08) has an
  unresolved field failure and a confirmed diagnostic gap; see Review findings. Chat links,
  tables and tabs (`7661d97`..`70614c8`, reviewed 2026-10-08) remains open for three
  verification/specification findings below; no product defect found. Fix model: trivial/easy —
  Claude Sonnet or Codex Luna.
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

### Chat links, tables and tabs — review 2026-10-08 — **Fix model:** trivial/easy — Claude Sonnet or Codex Luna.

Reviewed implementation `7661d97`..`70614c8` against its design/spec delta at `da63ac6`.

- **Worth fixing** — desktop overflow receipt cannot fail; fix complexity easy (INV §17).
  `ui/scripts/chat-cleanup-render.mjs:216` records the full-chat table width check with literal
  `true`. A compressed or clipped table therefore still earns a passing receipt at both desktop
  sizes, contrary to FS-03.A60/TS-08.R110. Dashboard/phone width comparisons do not prove this
  full-chat surface. Assert overflow for the deliberately wide fixture and exercise `scrollLeft`
  to prove the content is accessible; apply equivalent checks to the archive and file viewer.
  Confirm the check fails when the wrapper's overflow is changed to hidden or cells are compressed.
- **Worth fixing** — phone link navigation is not exercised; fix complexity easy (INV §10/§17).
  `ui/scripts/chat-cleanup-render.mjs:374–376` checks only `target="_blank"`; it never taps the
  link. FS-03.A59 and FS-20.A14 explicitly require a phone tap opening separately. A handler
  preventing activation would pass this receipt. Tap the existing stubbed destination, assert a
  new page and unchanged conversation, and retain the attribute check as supporting evidence.
- **Worth fixing** — cleanup specs overpromise phone appearances; fix complexity easy (INV §10).
  FS-03.R79/A60 and FS-12.R60/A32 combine phone coverage with all three appearances without the
  existing phone exception. FS-20.R16 and TS-08.R73 keep the phone Core-only;
  `ui/src/remote/main.tsx` and `ui/src/styles/remote.css` implement that contract. The Core-only
  phone check is appropriate to the shipped architecture, but the new acceptance wording is
  contradictory. State the desktop three-appearance matrix and Core-only phone coverage explicitly;
  do not introduce phone skins to satisfy this cleanup's accidental promise.

88 focused UI tests, all 41 style/presentation checks and the browser script's 161 reported checks
passed. Browser report: `/tmp/chuck-review-chat-cleanup/report.json` (stubbed APIs, no provider);
the findings qualify that report's acceptance coverage. No product code/specs changed. The shared
link-menu claim, local-file behavior, stable renderer map, tab fallback and retained backend
tracking are sound on inspection. INV §2/§8/§10/§13/§17 have applicable surfaces; §10/§17 findings
are above and the other applicable classes have no finding. Classes §1/§3–§7/§9/§11–§12/§14–§16
have no applicable changed surface (no lifecycle, persistence, runtime/protocol, HTTP route,
external CLI, durable side effect or unbounded retained collection change).

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
