# Chuck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-10-06 is archived in
[`HANDOFF-through-2026-10-06`](../archive/state/HANDOFF-through-2026-10-06.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** `think-tank-workspace-and-live-controls.md` — in progress (slices 1–5 of 6 done).
- **Release:** `v0.10.0` is tagged at `2904c8e` and published to `AsaphNoam/AgentDeck`; the macOS
  release workflow and CI passed. The GitHub Release carries the 293,367,237-byte `darwin-arm64`
  archive, `install.sh`, and a `0.10.0` manifest matching that size; the `AsaphNoam/Chuck` releases
  API still returns 404 until the rename. It is the first Chuck release: 39 commits after `v0.9.0` ship the Chuck rename and Think Tanks.
  Its notes tell paired phones to re-pair at the new `chuck` address and point existing users at
  `docs/chuck-cutover.md`.
- **Repository name:** the GitHub repository is still `AsaphNoam/AgentDeck`; the human postponed
  the rename to `AsaphNoam/Chuck` and chose to release first. Release CI publishes to the current
  repository, but the shipped installer bootstrap, README one-liner and `chuck update` default to
  `AsaphNoam/Chuck` and 404 until the rename; workarounds are `CHUCK_REPO=AsaphNoam/AgentDeck`
  and `chuck update --repo AsaphNoam/AgentDeck`. Renaming later fixes them without a new release.
- **Work units:** `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
  The 2026-10 provider bundle refresh is finished (its live-provider smokes are owed).
- **Review units:** the test-only `post-release-flaky-test-synchronization` fixes
  (`32da712`, `eadab5a`) are reviewed and closed without findings.
  The 2026-10 provider bundle refresh (`7c95fa9^..2f39c3f`, excluding the interleaved `docs:`
  design commits) is closed: its BU-01 fix landed.
  Think Tanks (`46379da..539ab11`) is closed again: its second-pass findings are fixed.
  Quiet completed chat turns (`2cf6cfa^..36f055a`) was reviewed and stays open for QT-01–QT-03
  below. Review notes: TS-08.R102 now keys a turn by its opening boundary seq (no key
  adoption); notices stay visible in completed turns as outcomes; scroll anchoring through
  automatic collapse relies on native `overflow-anchor`; the phone render uses a real transcript
  through `phone-render.mjs`, not a paired device.
  UI polish — auto-grow fields, icon actions, plain labels (`eec9038`, `221980d`, `8609d00`,
  `8e06375`) is closed: its UP-01 fix landed.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

[`think-tank-workspace-and-live-controls.md`](../ready-changes/think-tank-workspace-and-live-controls.md)
— in progress. Slices (each closes with focused tests, spec marks, handoff, commit):

1. **Done** — title + title group (FS-21.R51, TS-14.R22 shipped; R43 header/fallback shipped,
   card/Archive/chat cues finish with slice 6). Migration 39 `migrateThinkTankTitles`; explicit
   title kept in create intent with `omitempty` so pre-title replays still match; group applied in
   `launchReservedThinkTankAgent` (existing identities never regrouped).
2. **Done** — live ceiling increase (FS-21.R49, TS-14.R24 shipped): `IncreaseThinkTankTurnLimit`,
   migration 40 `think_tank_limit_commands` receipts, REST `.../participants/{agent_id}/turn-limit`,
   `TurnLimitEditor` in the room roster. A36's rendered live-budget journey is owed with slice 6.
3. **Done** — structured mentions server half (TS-14.R25 shipped): `AddThinkTankMessage` +
   `think_tank_mentions.go` snapshot `{"addressees":[...]}` into input/entry context (REST entries
   already expose `context`); MCP read adds `addressed_to`/`addresses_you` and an `addressed` note.
   Picker/composer UI is slice 6 (FS-21.R46 stays planned). Departed-target refusal lacks a test.
4. **Done** — judge synthesis result (FS-21.R50, TS-14.R26 shipped): migration 41
   `think_tank_results` (FK to agents, not rooms) written in judge finalization via
   `FinalizeThinkTankAttemptAt(..., ev.Seq)`; `GET /api/sessions/{id}/think-tank-results`;
   `mergeThinkTankResults` in `TranscriptView` places the row before the anchoring `turn_end`.
   Review note: every room update invalidates mounted results queries (one fetch per open chat).
   The remote route inventory must classify every new route (`remote_routes.go` denied list) —
   run the whole `internal/server` package, not only `-run ThinkTank`.
5. **Done** — concurrent openings (FS-21.R48, TS-14.R23 shipped): migration 42 splits the running
   index (one non-opening per room, one running attempt per agent); `ThinkTankOpeningOpportunities`
   admits openings without revision checks; boundary pause/end waits for the last running opening;
   failed openings wait for `RetryThinkTankOpening` (`attempt_id` on `{target:"turn"}` retry);
   `startThinkTankOpenings` bounds starts 32/room, 128/process. Wire: `active_attempts`, singular
   actor only for a sole attempt (part of TS-14.R27). Review notes: retry idempotence rides on the
   attempt state (`retried`), not a command receipt; `command_id` is accepted but unused.
6a. **Done** — list summaries carry `roster`/`total_remaining`/`judge_enabled`/`judge_name`,
   `GET /api/think-tanks?agent_id=` membership filter (migration 43 index) and `clipped`. Fixed the
   presentation contract (`goal`/`turn-limit`/`result` slots): `npm test`'s pretest runs
   `check:styles` — run `npm test`, not bare `vitest`, before committing UI.
6b. **Done** — room cards (FS-02.R71, FS-21.R44, TS-08.R96 shipped) in `RoomList`, before the
   agent grid. Stubbed render: `(cd ui && node scripts/room-render.mjs <out> project)` (also
   `live`/`ended`). Direction: title+phase → status/attention → quiet 2-line goal → wrapping roster
   chips with remaining → total/judge; inline-start rule is the collective cue; no motion.
6c. **Done** — chat `RoomCue` + `ThinkTankTab` (in `RoomTurnNotice.tsx`) via
   `useAgentThinkTanks` (FS-03.R71–R72, FS-21.R43/R45, TS-08.R98 shipped).
6. UI remaining: room cards
   (FS-02.R71), participant Think Tank tab (FS-03.R71–R72), anchored composer + mention picker,
   speech tints (FS-21.R44–R47, TS-08.R96–R99), then rendered A33 journey and TS-06 closure.

Turn journey: `make embed`, then `go run -tags sqlite_fts5 ./scripts/stress-fixture -port 4411
-scenario activity_showcase` and `(cd ui && node scripts/turns-journey.mjs http://127.0.0.1:4411
<outDir>)`.

Tasks wire fixture regeneration: `CHUCK_UPDATE_TASK_FIXTURE=1 go test ./internal/server
-run TestTaskWireFixture`. Think Tank room fixture: `CHUCK_UPDATE_THINK_TANK_FIXTURE=1 go test
./internal/server -run TestThinkTankWireFixture`. Room screenshots: `(cd ui && node
scripts/room-render.mjs <outDir> live|ended)`.

## Acceptance gates still owed

- FS-21 / TS-06.R33 (Think Tanks): the real-binary fake-ACP rendered journey (creation →
  discussion → annotation/private follow-up → End/judge → retained Archive at 1024px and wider in
  Core, Sky & Grove and Studio) and the bounded credentialed Claude/Codex smoke (room-tool
  read/submit, an ordinary approval/denial, private Send/Steer, native resume, end-only judge).
  Stubbed-data renders of live and ended rooms in all three appearances passed 2026-10-06.

- FS-10.R25/A14, TS-02.R41: one supervised cutover rehearsal on a disposable copy of the real
  AgentDeck home, following `docs/chuck-cutover.md`, with a receipt. The GitHub repository rename
  to `AsaphNoam/Chuck` (installer and updater fetch there) is still owed; see Current position.

- FS-02.A46: real-browser toast click check and a manual macOS desktop-notification click are
  owed; automated component and `sse.test.ts` coverage passes.

- FS-18.A12 / TS-11.R17: the six manual role scenarios with a role-free follow-up against the
  pinned Claude and Codex providers have not been run; no qualitative receipt exists yet.
- FS-18.A13: the credentialed fresh and resumed Claude chat check of native-preset adoption is
  owed. Automated coverage proves only the sent shape against pinned
  `claude-agent-acp` 0.85.1 (`scripts/release/node_modules/.../dist/acp-agent.js` forwards an
  object `_meta.systemPrompt` as a preset append; a string replaces the preset).

- FS-20.A1/A5/A6/A8: real tailnet, Android, iPhone, and `pmset -g assertions` checks. The iPhone
  Home Screen experience also still lacks a PNG touch icon. The 390px fake-provider browser pass
  covered A3/A4/A7/A9, except the fast-mode picker and Continue on an approval pause.
- FS-20.A10–A13: focused server and UI suites pass; the remaining phone UI-coverage gaps are listed
  under Review findings. The combined 390px fake-provider browser journey
  for the new dashboard, project, agent-management, Files/Commands, and retired task flow also remains
  owed.
- TS-06.R21: credentialed Claude and Codex login/chat checks.
- TS-06.R31 / FS-09.A40/A42/A46/A47/R78, FS-10.A10–A12 (installed providers): at most four real
  combinations — Claude and Codex, each with the current bundle and one current installed CLI —
  running the finite smoke (fresh chat, native resume, model/effort, one approval/denial and
  cancel, Steer, a role/skill and an MCP action), plus two rendered fake-provider journeys
  (Installed update → Refresh → choose new model; missing Installed → Bundle save → retry →
  back to Installed with overrides). Needs authorization and credentials; `assemble.sh`'s native
  probes run first in release CI. Claude Installed fresh launch at `claude-opus-5-5` passed live
  2026-10-05 (adapter 0.75.1/SDK 0.3.257, Claude Code 2.1.282, macOS, the user's existing login); the
  Bundle still refuses it with Claude Code 2.1.257's version error, as designed.
- TS-06.R26/R32: the credentialed Codex 2.1.1 receipt gating FS-03.A41/A42 and FS-01.A20.
- TS-06.R32 (2026-10 bundle, Claude ACP 0.85.1 / Codex ACP 2.1.1 / Codex 0.159.3): the two-point
  Claude and Codex smoke plus one notice, one Steer during a running command (FS-03.A48's
  credentialed half) and one refused model switch where policy allows staging it. Automated
  assembly proofs and fake-runtime tests passed 2026-10-06; a release CI `assemble.sh` run is
  also still owed.
- FS-02.A27: six-tab real-browser shared-stream check; A46's real-browser J14 pass; Sky & Grove
  with Codex capabilities.

## Blocked on human

None.

## Review findings

### Quiet completed chat turns — **Fix model:** medium — Codex Terra or Claude Opus.

**Unit:** `2cf6cfa^..36f055a`.

- **Must fix** — **QT-01 (INV §1/§11): Reasoning admitted before hydration is backfilled into old history.**
  `ui/src/api/sse.ts:189–193` admits a delta even while the first transcript fetch is pending,
  using an empty or unreconciled window for its anchor and turn key.
  `ui/src/components/chat/runtimeActivity.ts:62–75` then inserts that span solely by list index;
  the stored turn key only participates in its message id. Opening a running conversation with
  earlier completed turns can therefore hide its current thought inside the first completed turn
  when REST hydration arrives. Leaving and reopening a source also retains old reasoning while
  `registerOpenAgent` discards its transcript. This violates TS-08.R103's proven ownership and
  no historical backfill rules. A temporary projection probe confirmed a delta admitted at
  anchor 0 before hydration lands in the old completed turn, absent from the current turn.
  Gate association on a reconciled active boundary and validate ownership when inserting spans;
  add a real SSE-path test with delayed initial hydration and source reopen. Fix complexity: medium.
- **Must fix** — **QT-02 (INV §1): Child completion does not close opened tool detail.**
  `ui/src/components/chat/renderers/ChildActivity.tsx:25–45` retains an explicit open choice
  through a terminal child state and only changes the thought context's `live` flag. Nested
  `toolRun.tsx:28–36` and `renderers/ToolCall.tsx:4–16` retain their open state. Open a child's
  tool run/arguments while it executes, then receive `activity_state=completed` before root
  completion: tool detail stays expanded, violating TS-08.R102. Reset nested disclosure choices
  once at the child's terminal transition and test completed/failed/stopped/disconnected with
  the root still live, preserving later manual reopening. A temporary component probe confirmed
  the opened tool run remains expanded after child completion; the probe was removed.
  Fix complexity: trivial/easy.
- **Must fix** — **QT-03 (INV §17): The rendered journey does not prove its claimed closure.**
  `ui/scripts/turns-journey.mjs:72–80,172–195` accepts any existing root turn end while waiting
  for the second turn, swallows the wait error, and never asserts that the second turn completed.
  Its expanded-activity PASS ignores the computed thoughts/tools/child checks. Lines 200–219
  report Archive/dashboard success from navigation alone; the agent is not archived and no
  transcript/disclosure assertion establishes those surfaces. The script also never exercises
  keyboard operation, focus during automatic collapse or reading above the tail, although
  FS-03.A58/TS-08.R106 require rendered proof and closure marks them shipped. Wait for a new
  terminal seq, fail missing content/surfaces, and run the specified keyboard/focus/scroll
  scenarios with receipts; keep unverified acceptance gates live until then. The phone transcript
  render fallback is documented and is not itself a defect. Fix complexity: medium.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** decide whether a pause after failed launch/resume keeps
  withholding **Open agent**, matching restart recovery, or permits chat under a wider continuation
  contract.

## Design consistency notes

- At the next presentation review, decide whether the remaining crisp asymmetric radii on technical
  surfaces are deliberate under FS-12.R52.
- When the paused direct-action change resumes, align TS-01.R25 and TS-03.R32 with TS-04.R40 and
  scope FS-17 section 6's opening sentence to the intended planned boundary.
- The injected-steer lifetime edge case needs `/investigate-bug` before `/fix`; FilesTab and
  CommandsTab still copy silently through bare `writeText`.

## Changelog

- **2026-10-07 — Work: Think Tank workspace slice 5 (concurrent openings).** Independent openings
  now run at the same time, stay hidden until all settle, and publish in member order; failures,
  Pause, End and restart wait for running peers and need explicit per-opening retry. State/server/
  messaging in both Go variants, focused `-race` and room UI tests passed.

- **2026-10-07 — Work: Think Tank workspace slice 4 (judge results in chat).** A finalized
  synthesis is retained once in the judge's own history (survives room deletion) and shows in its
  live and archived chat with a room link. Also classifies slice 2's turn-limit route as phone-denied
  (the slice 2 commit had left `TestRemoteRouteInventoryIsClassified` failing).

- **2026-10-07 — Work: Think Tank workspace slice 3 (shared mentions, server).** Shared room
  messages accept validated participant mention ranges; addressees are snapshotted, visible to all
  and flagged to the addressee on its next room read. State/MCP/server tests passed.

- **2026-10-07 — Work: Think Tank workspace slice 2 (live turn limits).** Operators can raise a
  participant's turn ceiling during openings/discussion (paused or held included); exhausted members
  regain eligibility, nothing resumes or steers, replays are exact and stale/ended changes refuse.
  State/server tests in both Go variants and room UI tests passed.

- **2026-10-07 — Work: Think Tank workspace slice 1 (titles, title groups).** Rooms carry a fixed
  short title (legacy rows/API callers get a goal-derived fallback via migration 39); the room
  header leads with it and the goal moves into a disclosure; new participants/judge launch into the
  title group. State, server (`ThinkTank` filter) and room UI tests passed; wire fixture regenerated.

- **2026-10-07 — Fix: UI polish UP-01 (INV §8/§10).** The phone Message/Reply composer passes
  `maxHeight="40vh"`, so a long draft caps near 40% of the viewport and scrolls inside while text
  and Send behave as before (FS-02.R66, TS-08.R88). AgentScreen test asserts the cap; restores
  specified behavior, no specification change. Unit closed. Remote UI tests (48) and the
  AutoGrowTextarea suite passed; `tsc` clean. Still open: Quiet completed chat turns QT-01–QT-03.

- **2026-10-07 — Review: UI polish.** One must-fix (UP-01); the unit stays open. Plain labels,
  archived Tasks filtering, icon semantics, header ordering and desktop field sizing match the
  specs. Focused UI tests: 108/108; styles/presentation: 40/40 plus contract check;
  isolated loopback fake-ACP `polish-render.mjs`: 17/17, screenshots inspected in all three skins.
  INV sweep: §1/§2/§3/§4/§8/§10/§13/§17 checked; no applicable surface for
  §5/§6/§7/§9/§11/§12/§14/§15/§16. Existing submit/persistence paths are unchanged;
  resize listener cleanup is paired; tests use explicit requirements and DOM/layout observations.
  The concurrent BU-01 fix and its handoff closure were preserved.

- **2026-10-07 — Fix: 2026-10 provider bundle refresh BU-01 (INV §1/§2).** `markBackgrounded`
  now reads `collectTasks`, so a tool's background label follows the resume and clone fences:
  after either boundary it reads "Ran in background", matching the task list (FS-01.R36,
  TS-08.R86). Resume/clone tool-label tests added; restores specified behavior, no specification
  change. Unit closed. `make test`, `make build`, UI tests (661) and UI build passed.
  Still open: Quiet completed chat turns QT-01–QT-03; UI polish review.

- **2026-10-07 — Review: post-release test synchronization.** Reviewed `32da712` and
  `eadab5a`: held-context prompt delivery is observed before the no-new-turn baseline, and
  New Agent options wait for the rendered default model before changing runtime controls.
  Existing provider-count, persistence, launch-payload and runtime-control assertions remain
  intact; no product or specification change, findings, or unresolved local choices. Unit closed.
  **Fix model:** trivial/easy — Claude Sonnet or Codex Luna (no open fixes).
  INV §17 reviewed; classes 1–16 have no applicable changed production surface.
  Both context integration tests passed three repetitions untagged and once with `sqlite_fts5`;
  all 26 New Agent tests passed. The adjacent onboarding synchronization was also inspected and
  its two tests passed, but is outside this unit. Go tests required sandbox-external loopback
  listeners. Concurrent chat edits were preserved; this review changes only the handoff.

- **2026-10-07 — Implementation: UI polish.** Every text field auto-grows with no grip (composers
  cap near 40% of the window); Send, Cancel, Collapse and Collapse all are icons; expanded cards put
  badge and Collapse top-right with the context meter beneath; selectors show plain names with ids
  only for duplicates; Tasks hides archived projects. Closure: `make test` (one run failed while the
  stress fixture was being stopped; the rerun passed), `make build`, UI tests/build, style checks and
  `polish-render.mjs` 17/17.

- **2026-10-06 — Review: quiet completed chat turns.** Three must-fix findings (QT-01–QT-03);
  unit stays open. Shared boundary keys, conservative final-passage selection, visible approvals,
  outcomes/background controls and phone/attempt projection wiring otherwise match the specs.
  Opening-boundary identity and visible notices are sound local choices; native scroll anchoring
  still needs the required rendered proof. Invariant sweep: 1, 2, 8, 10, 11, 13, 16 and 17
  reviewed; 3, 4, 5, 6, 7, 9, 12, 14 and 15 have no applicable changed surface.
  All 117 focused UI tests passed; temporary projection/component probes confirmed QT-01 and
  QT-02 and were removed. The concurrent field-polish commit also captured the initial review
  record; this review's final verification receipt is a separate handoff-only commit.
  Stylelint and all 40 script tests passed, but the presentation audit failed on concurrently
  added `AutoGrowTextarea.tsx` inline styles outside this unit. Preserved concurrent work;
  this review changes only the handoff and does not rerun or claim the rendered acceptance gates.

- **2026-10-06 — Work: quiet completed chat turns.** Shared turn projection and `TurnList` across
  full chat, dashboard pane, Archive, phone and Think Tank attempt activity; live thoughts start
  open with per-turn/scope collapse; cut-short turns label their response partial. Rendered
  real-binary journey (fake ACP `activity_showcase`) passed at 1024px/1600px in Core, Sky & Grove
  and Studio plus reduced motion; phone rendered at 390px from that run's transcript. Invariants:
  1, 2, 8, 10, 11, 13, 16, 17 applied; others have no changed surface.

- **2026-10-06 — Review: 2026-10 provider bundle refresh.** One must-fix finding (BU-01); unit
  stays open. Confirmed pins, patch/hash proofs, MCP revision guard, notice persistence/preview
  exclusion and model-policy error shapes against the installed dependency sources. Confirmed
  TS-04.R81's local choice to derive background state from the linked task. Invariant sweep:
  applicable classes 1, 2, 8, 10, 11, 12, 13, 16 and 17 reviewed; classes 3, 4, 5, 6, 7, 9, 14
  and 15 have no changed surface. Runtime, transcript, CLI, release and messaging suites passed;
  CLI needed sandbox-external loopback binds. All 104 focused UI tests, style and presentation
  checks passed. Live-provider smokes and release-CI assembly remain owed as listed above.

- **2026-10-06 — Work: 2026-10 provider bundle refresh.** Pins Claude ACP 0.85.1 (SDK 0.3.286),
  Codex ACP 2.1.1 with Codex 0.159.3 (steering patch regenerated, still required), go-sdk 1.8.0.
  AIR shapes Chuck decodes were unchanged. Adds notice rows, steer-backgrounded tool state and
  model-policy refusal reasons. Closure: `make test`, `make build`, UI test/build/style checks and
  a focused runtime `-race` pass all passed. TS-06.R32 stays planned until its smokes run.

- **2026-10-06 — Design: quiet completed chat turns.** Human confirmed one per-turn activity
  disclosure and unchanged ephemeral thought retention. Ready change `quiet-completed-chat-turns.md`
  is Waiting to start: FS-03.R73–R77/A54–A58 and TS-08.R100–R106 cover open live thoughts,
  respected manual choices, automatic terminal collapse, last-passage compatibility, actionable
  attention/background status and shared desktop/dashboard/archive/phone behavior. No protocol,
  durable-history or retention change; rendered long-chat acceptance remains an implementation
  gate. No product code changed in this design unit; the provider refresh stays active.

- **2026-10-06 — Design: Think Tank workspace and live controls.** Human confirmed the scope
  and required the room composer to match standard Chuck chat input. Ready change
  `think-tank-workspace-and-live-controls.md` is Waiting to start. FS-21.R43–R51/A33–A37,
  FS-02.R71/A53 and FS-03.R71–R72/A52–A53 cover titled cards, compact chat/tab navigation,
  anchored shared mentions, speaker tints, concurrent openings, live ceilings, exact judge-chat
  results and new-agent grouping. TS-14.R22–R28 and TS-08.R96–R99 reuse room admission,
  finalization, normal launch and shared presentation; independent audits checked pending-design
  overlap and execution guards. R23 explicitly fences pipeline Stop and failed-opening retries;
  judge results commit atomically with room synthesis as agent-owned read projections. Integrate
  shared textarea/icon seams with `ui-polish-fields-icons-labels.md`; keep pipeline-stage work
  independent and applicable closure guards shared. All new requirements remain planned; no
  product code changed and the active provider-refresh work remains active.

- **2026-10-06 — Design intake: Think Tank workspace and live controls.** Recorded the requested
  improvements and proposed experience in `docs/ideas.md` under Ideas being defined: titled
  single-column room cards before project agents, compact participant chats with a Think Tank tab,
  anchored shared composer and agent mentions, stable speaker tints, concurrent isolated openings,
  live ceiling increases, judge-chat synthesis and grouping of newly deployed agents. Awaiting
  feature-side confirmation of mention scheduling, existing-group preservation, terminal budget
  behavior and title compatibility. No product code, new TS design or ready change in this unit;
  the active provider refresh and other specification/code work are preserved.

- **2026-10-06 — Design: Think Tank pipeline stages and task collapse.** Human confirmed the
  scope; ready change `pipeline-think-tank-stages-and-task-collapse.md` is Waiting to start.
  FS-14.R81–R86/A48–A52, FS-21.R41–R42/A31–A32 and FS-16.R46–R48/A30–A32 cover fresh same-project
  rooms, required judge synthesis as direct stage output, retained history/recovery/scoped Stop,
  readable shared-workspace consent and parent-level collapse. TS-09.R51–R56, TS-14.R19–R21,
  TS-10.R38 and TS-08.R92–R95 reuse room execution and task result authority, add durable managed
  stage binding and session collapse choices. Independent read-only seam/review pass resolved output,
  state and acceptance-retry contracts. All new requirements remain planned; no product code changed
  in this design unit and the active provider-refresh change stays active.

- **2026-10-06 — Design: UI polish batch.** Ready change `ui-polish-fields-icons-labels.md`
  (FS-02.R66–R70, TS-08.R88–R91): auto-grow textareas, icon Send/Cancel/Collapse, expanded card
  header order, duplicate-gated plain labels, archived projects hidden on Tasks. Absorbs the
  "Finish hiding raw ids" idea.

- **2026-10-06 — Fix: Think Tanks second pass (TT2-01–TT2-08).** Must fix TT2-01 (INV §1, §16): a
  stopped speaker whose resume fails before its attempt holds the room with a reason after one
  attempt. Worth fixing: TT2-02 (INV §5) a room change during resume leaves the agent running
  idle; TT2-03 (INV §16) per-room bounded progression goroutines (cap 4); TT2-04/07 (INV §16)
  migration 38 adds a byte-counted `activity_bytes` total and the attempts room index; TT2-05
  (INV §16) live activity refills from the cached tail; TT2-06 (FS-21.R31) judge read omits
  ceiling fields and the instruction is role-neutral; TT2-08 (INV §1) a store failure at turn end
  holds the room with recovery text (no regression test: needs a store-failure seam). TS-14 updated.

- **2026-10-06 — Review: Think Tanks second pass.** Human-requested behind-the-scenes review of
  `46379da..539ab11`: context delivery verified sound; one Must fix (silent resume-failure retry
  loop) and seven Worth fixing (resume-window stop, serial worker, quadratic capture cap,
  UI activity refetch storm, judge framing, missing attempts index, store-failure stuck attempt).
  Focused `-race` ThinkTank state/server tests pass.

- **2026-10-06 — Release: `v0.10.0` published.** 39 commits after `v0.9.0`: the Chuck rename, Think
  Tanks, phone streamed-reply and Manage-refusal fixes, notification stale-agent coverage and the
  Claude 5.5 alias-row fix. The operator package (Think Tank reference already added), README and
  pins already matched the range. `make test`, UI suite (625) and `make dist VERSION=0.10.0` pass.
  Released before the repository rename at the human's choice. Settled state archived to
  `HANDOFF-through-2026-10-06`. Release workflow and CI passed; assets verified.
