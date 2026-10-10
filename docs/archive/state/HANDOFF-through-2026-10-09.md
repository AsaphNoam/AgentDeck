# Chuck — Implementation handoff

Archived for `v0.12.0` on 2026-10-09. The user closed the unconfirmed clone first-message
finding, to reopen if reproduced, and explicitly waived the pending clone-name review for this
release. The user confirmed `v0.12.0` and authorized publishing `main` and the tag. The content
below preserves the pre-release record; current open state lives in `docs/features/HANDOFF.md`.

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
  Other available/resumable design work is in `docs/ideas.md`.
- **Review units / findings:** Shared pipeline orchestrator instructions (`2020733`, reviewed
  2026-10-09) is closed after its corrupt-snapshot finding was fixed. Clone keeps its source's name
  (`3448dae`, 2026-10-09) remains available for review. Think Tank pipeline stages, readable workspace consent and
  collapsible tasks (`fb715a1`..`3d9ab39`, reviewed 2026-10-08) is closed after its six
  lifecycle, lineage and acceptance-recovery findings were fixed. Clone first-message failure
  investigation (2026-10-08): its diagnostic gap is fixed; the unconfirmed field failure was closed
  by the user on 2026-10-09, to reopen if it recurs (see Review findings). Chat links,
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
- FS-14.A54: bounded packaged Claude/Codex probe that a standing owner/dedicated coordinator
  adopts the shared orchestrator instructions; fake-provider delivery capture is no live receipt.
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

Release readiness checked 2026-10-09: the range from `v0.11.0` to `main` contains 32 commits
through `2ebd148`. No version was proposed, tag created, or push performed.
The user closed the unconfirmed clone first-message finding on 2026-10-09 because no further
evidence is available, with reopening only if it recurs; that finding no longer blocks release.
The clone-name change (`3448dae`) remains an open review unit and needs review or an explicit
user decision before tagging. Package refresh and release verification have not run.

The v0.11.0 release published its three assets after the rerun, but
its body is still empty (checked 2026-10-08): set the notes from
[`RELEASE-v0.11.0-notes.md`](../archive/state/RELEASE-v0.11.0-notes.md) with `gh release edit`
when publication is authorized. Do not retag or recut a new version.

## Review findings

No open findings.

## Closed investigation: clone first-message failure — 2026-10-08

**User disposition 2026-10-09:** closed as unconfirmed, not fixed; no further evidence is
available. Reopen if the issue recurs. Capture the failing request URL/body and the bounded
`runtime: provider prompt failed` diagnostic from `~/.chuck/dashboard.log` then. Four live
Claude variants did not reproduce the failure. This is no longer a release blocker.

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
`launch.go:331` calls `suggestName`, whose first unused suggestion is Atlas. Independent of sending.
Superseded 2026-10-09 by user request: FS-01.R39 names a clone "<source name> Copy".

- **Closed unconfirmed** — reported field behavior; root cause undetermined; fix complexity medium
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
  **Fix run 2026-10-09 — not reproduced; subsequently closed by user.** The provider was
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

- **2026-10-09 — Shared pipeline orchestrator corrupt-snapshot finding closed (INV §7/§11/§17).**
  Validate the frozen template's version-2 structure before reading optional guidance, so malformed,
  incomplete or unreadable context fails task start while valid snapshots with omitted or empty
  guidance remain compatible. Regression coverage checks malformed and incomplete snapshots,
  storage read failure, valid optional-field cases, and repeated dispatcher failure without process
  launch. Focused tests, `make test` (both Go variants), `make build`, and `git diff --check` passed.
  The credentialed provider-adoption gate remains owed above.

- **2026-10-09 — Shared pipeline orchestrator instructions reviewed (`2020733`).** One
  must-fix corrupt-snapshot validation finding keeps the same unit open; no product/spec edits.
  Focused automated checks and the 72-check rendered journey passed; a separate temporary
  store/resolver fault probe failed as expected for `null`, `{}` and an incomplete template.
  The clone-name review unit remains available independently.

- **2026-10-09 — Shared pipeline orchestrator instructions shipped (FS-14.R87–R89/A53–A55,
  TS-09.R57–R60, TS-11.R20).** Optional template `orchestrator_instructions` (16,000-code-point
  field-named bound, omitempty so old digests are stable) freezes with the run snapshot.
  `startLaunchedTask` resolves it via `pipelineOrchestratorInstructions` →
  `state.PipelineOrchestratorTemplateSnapshot` (stage task with agent execution, or coordinator
  located by its lineage parent's binding; indexed keys only) and composes one labelled block into
  `LaunchSpec.SystemPrompt`; a pipeline-created task with missing binding/snapshot or a read error
  fails its start. Editor textarea beside the orchestrator role; knowledge reference updated.
  Evidence: `TestPipelineOrchestratorInstructions*` (session/new + session/load capture, frozen
  prompt, no assignment copy, eligibility), template/proposal tests, editor test,
  `ui/scripts/orchestrator-instructions-render.mjs` (72 checks, three appearances). `make test`
  (both Go variants), `make build`, 721 UI tests and UI build passed. Reviewer note: dedicated
  switch/failed-load/run-deletion tests for this block were not added (shared frozen-prompt path);
  the editor's post-save refetch clears the "Template saved." notice almost immediately
  (pre-existing). New review unit.

- **2026-10-09 — Clone keeps its source's name (FS-01.R39).** User-requested: a clone is named
  "<source name> Copy" (source shortened to fit 256 characters; an unnamed source still gets the
  curated suggestion). `cloneName` in `server/clone.go`; covered by
  `TestCloneNameAppendsCopyWithinTheLimit` and the clone fork test. `make test` (both Go
  variants) and `make build` passed. New small review unit.

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

## Settled closures moved from the live handoff — 2026-10-09

- **Review closed — Settings composition (FS-12.R62/A34, TS-08.R113):** every Settings tab
  adopts the Figma Make `Settings` study: 180px left section list with soft done-tinted selection,
  mono eyebrow + section title (Theme, Configured backends, Dependent work) via the new
  `SettingsHeader`, softly raised role/project/phone rows with text actions (destructive in error
  color; roles without an override read "Inherit global"), one raised card per backend with ruled
  mono-labelled groups and ruled model rows, ruled Notifications/Tasks lists, and three equal theme
  cards whose `preview-sky` band holds the action bar and signal over three surfaces (skin rules
  unchanged, no contract change). Primary actions use `--ad-action-primary` with a pressed edge.
  Excluded: study sample data, prototype notes, Appearance default tab, concept theme, invented
  backend fields; sub-1024px layout (below the desktop floor). Remote's `window.confirm` revoke is
  pre-existing and untouched. 730 UI tests, presentation/style checks, UI build, `make test`
  (`CHUCK_RUNTIME_ROOT=`), `make build` and `make embed` passed. Real-browser captures of all seven
  tabs in Core, Sky & Grove and Studio at 1024/1280 (fake-backend fixture: roles with/without
  override, active/archived long-name project, four backends) show no overflow or page errors.
  Harness and captures: shared project resources `settings-design/` (`harness/`, `runs/final`);
  Make source in `settings-design/App.tsx`. Independent review of `22dd186` found no confirmed
  defects. Source/spec review passed all 730 UI tests and 41 presentation/style checks. The
  invariant sweep found no violations: applicable classes §3, §8, §10, §13 and §17 passed;
  §1–2, §4–7, §9, §11–12 and §14–16 have no applicable changed surface. No unresolved local
  choices or open fixes remain; no fix-model recommendation is needed.
  Independent approved Chromium execution used the working UI, the built real binary, a fresh
  review-owned home and fake backend on loopback only. All seven tabs in all three appearances
  at 1024/1280 passed (42 static captures), plus hover, keyboard focus, invalid Tasks/disabled
  Save, expanded model environment, linked configuration-source effective view, long paired-phone
  row, rename/disabled Save, pairing code and pending pairing (42 state captures). All captures
  have zero page errors and horizontal overflow; screenshots were inspected against the study's
  hierarchy and spacing. Display-safe browser fixtures supplied linked-source and phone states;
  this closes presentation coverage, not live-provider or real-tailnet/device gates.
  Evidence and reusable harness: `/private/tmp/settings-independent/` (`static/report.json`,
  `states/report.json`, screenshots and `harness/`). Review-owned processes shut down; unrelated
  concurrent dirty-tree work was preserved. This closes the unit without a new review obligation.
- **Review fixes closed:** mobile companion unit `50e0d97^..d3e737f` and Think Tank disclosure
  unit `7b76bc4` are closed. Resume refusals stay visible on Files (INV §8); dismissed sheets and
  confirmations restore opener focus (INV §10); bounded activity rows carry durable publication
  state so clipped contributions cannot reappear as unfinished (INV §8/§10/§11/§16/§17).
  Both Go variants, 727 UI tests, 41 presentation checks and UI/Go builds passed in
  `/private/tmp/chuck-fix-closure`, excluding pre-existing chat presentation edits. Mobile browser
  Cancel/Close/Escape focus and Files refusal checks passed across all three palettes; receipt:
  `/private/tmp/mobile-fix-browser-final.log`. The Think Tank regression fails against the old
  renderer and passes with the fix. Source-build Go checks need `CHUCK_RUNTIME_ROOT=`; the first
  native-login probe timed out, then focused and full retries passed. Existing real-device and
  credentialed-provider gates remain owed. These fixes create no new review unit.
- **Review closed — desktop agent page:** active and archived agent pages adopt the Figma Make
  `AgentConversation` composition (FS-12.R61/A33, TS-08.R112): breadcrumb with agent id, raised
  card, monogram/state/role header with labelled context meter, low runtime band with local
  Discard and a switch-style fast toggle, quiet tabs, ~700px reading column with tinted bubbles,
  ruled activity, structured permission card (Deny before Approve, shared with the dashboard pane),
  and an inset composer with `@`/`#` insert buttons and a labelled Send (supersedes FS-02.R67 on
  this page only). Workspace styles are scoped to `.chat-panel`; the dashboard pane and phone keep
  their compositions. Sky & Grove's user-tint selector now targets the bubble, not its row.
  Study data, per-message times, date dividers, Files count and decorative hints were excluded.
  730 UI tests, presentation checks, UI build, both Go variants (`CHUCK_RUNTIME_ROOT=`) and
  `make embed` passed; the server package needed one rerun. Real-browser comparison against the
  rendered study passed in Core, Sky & Grove and Studio at 1024/1280 with fake-backend agents
  (permission, activity, long name, staged runtime + Discard, `@` picker, live turn, archive,
  header Copy thread identity). Reference source, renders, harness and receipts: shared project
  resources `agent-design/` (`runs/final`).
  Static review of `ea8dfc8` found no confirmed defects; presentation/style checks and all 730 UI
  tests passed again. Independent rendered review now also passed through bounded approved
  execution in an approval-capable agent: fresh isolated fake-backend fixture, Core, Sky & Grove
  and Studio at 1024/1280, 24 static captures without horizontal overflow or page errors, plus
  activity, permission, runtime staging/Discard, `@`/`#` pickers, live turn, archive and Copy thread
  identity checks. Fresh screenshots were inspected by the reviewer and sampled by the parent.
  Evidence: shared project resources `agent-design/runs/independent-20261009/` (`report.json`,
  three `*-states.json` receipts and screenshots). The native review child's sandbox cannot
  request escalation; workflow §7/§14.4 now routes that browser work to an approval-capable
  independent agent. No product code or permission defaults changed. This closes the review unit
  and its access block; existing credentialed-provider and real-device gates remain owed.
  The review used `/private/tmp/chuck-agent-page-review` built from `c189d95`, the working UI via
  Vite, and fresh review-owned `CHUCK_HOME` fixtures. Both review-owned harness sessions shut down;
  existing processes were preserved. Fresh captures were compared with `agent-design/ref/` study
  renders.

## Settled closures moved from the live handoff — 2026-10-10

- **Fix closed — mobile history (MOBILE-HISTORY-01):** phone transcript windows now hold 750
  events, load older pages automatically on upward scrolling with viewport anchoring, preserve
  bounded loaded history across live refreshes, and recover intervening gaps. The formerly skipped
  reproduction and focused server/UI coverage pass; rendered phone evidence is
  `/tmp/mobile-history-phone.png`.
- **Review closed — Think Tank room cards (`578ca50`, FS-12.R63/A35, TS-08.R114):**
  independent source/spec and rendered review passed without findings on 2026-10-10. Project
  and Archive cards passed in all three appearances at 1024/1440, including removed origins,
  separate keyboard focus and hover. Evidence: `/tmp/chuck-room-review-20261010/`. The later
  room-list width follow-up `ce46d56` has ROOM-WIDTH-01 open; the original card review stays closed.
- **Review closed — project-dashboard agent cards (`4dc1b34`, FS-12.R65/A37, TS-08.R116):**
  independent source/spec and rendered study comparison passed without findings on 2026-10-10.
  Core/Sky & Grove/Studio at 1024/1280 covered live busy/idle/waiting/error/stopped states,
  long names, terminal/mail/pipeline presentation, expanded permission/activity/composer,
  collapse/navigation, pane cycling, focus/hover, Send/Cancel and persisted drag reorder.
  Evidence: `/tmp/chuck-card-review-20261010/`.
