# Chuck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-10-06 is archived in
[`HANDOFF-through-2026-10-06`](../archive/state/HANDOFF-through-2026-10-06.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** none.
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
  `refresh-provider-bundle-2026-10.md` is ready and waiting to start.
- **Review units:** the test-only `post-release-flaky-test-synchronization` fixes are available.
  Think Tanks (`46379da..539ab11`) is reopened by a human-requested second pass with eight
  findings (TT2-01–TT2-08) under Review findings; `/fix` takes them.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

None.

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
  `claude-agent-acp` 0.75.1 (`scripts/release/node_modules/.../dist/acp-agent.js` forwards an
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
- TS-06.R26: the credentialed Codex 1.12.0 receipt gating FS-03.A41/A42 and FS-01.A20.
- FS-02.A27: six-tab real-browser shared-stream check; A46's real-browser J14 pass; Sky & Grove
  with Codex capabilities.

## Blocked on human

None.

## Review findings

### Think Tanks second-pass review (`46379da..539ab11`, human-requested 2026-10-06)

Behind-the-scenes pass over context delivery, lifecycle and growth cost. Context delivery is sound:
frozen heads, checkpoint advance only on success, own-entry suppression, blind-opening barrier,
UTF-8 continuation and full-discussion judge reads all match TS-14.R7–R8/R13. Turn ownership,
exit/restart fencing, commit-before-publish and capture teardown hold. INV §2–§4, §6, §8, §10–§14
and §17: no new applicable surface. Steer during a room turn reaching the contribution is designed
(FS-21.R33), not a finding.

**Fix model:** medium — Codex Terra or Claude Opus.

- **Must fix — TT2-01 (INV §1, §16):** `server/think_tanks.go` `startThinkTankTurn` stopped-agent
  branch (~L203–221). When `wakeCandidate` errors/returns `!ok` or `resumeSessionWithHooks` fails
  before `before()` runs (missing backend/binary, removed cwd/worktree, ACP handshake timeout),
  `begun` is nil so `abandon` is a no-op: no hold is set, the room shows Running, and every 5s
  sweep and kick launches another resume attempt indefinitely. A judge stuck this way stays
  `starting`. Violates FS-21.R37/TS-14.R6 (hold with a reason for intervention) and repeats
  process launches. Fix: on failure with nothing begun, `SetThinkTankHold(room, "<name> could not
  start: …")` as `thinkTankIneligible` does. Test: resume failing → one resume attempt, room held
  with reason, no second attempt on the next sweep.
- **Worth fixing — TT2-02 (INV §5):** same branch with `server/resume.go` `after()` (~L339–351).
  `begin` pins `d.Room.Revision` read before a multi-second resume; a room message, annotation or
  Pause during the resume makes `BeginThinkTankAttempt` refuse, `after()` errors, and
  `stopAgentClaimed` kills the freshly resumed agent; the next sweep resumes it again. A human
  prompt landing between `Registry.Resume` and `after()` (gate busy → not started) is likewise
  stopped mid-turn. Fix: in the room resume path, treat "not admitted" (revision conflict / busy
  gate) as leave-running-idle so the next sweep takes the running branch. Test: bump revision
  inside the resume window → agent stays running, next sweep admits.
- **Worth fixing — TT2-03 (INV §16):** `server/think_tanks.go` worker (~L35–62).
  `launchThinkTankSetup`, `launchThinkTankJudge` and resumes run inline on the single progression
  goroutine, so a room with several new participants or a slow/failing resume (amplified by
  TT2-01) delays admission for every other room. Fix: keep selection on the worker, run
  launch/resume in a goroutine with a per-room in-flight guard (existing setup claim, judge
  reservation and the one-running-attempt index already fence double admission).
- **Worth fixing — TT2-04 (INV §16):** `state/think_tank_activity.go:56` `AppendThinkTankActivity`
  runs `SUM(LENGTH(payload))` over the attempt's rows for every captured event, re-reading up to
  64 MiB of payload per tool event — quadratic in a long tool-heavy turn, inside a write
  transaction. `LENGTH` on TEXT also counts characters, so the TS-14.R17 byte cap is
  under-enforced for non-ASCII payloads. Fix: keep a running byte total on the in-memory
  `thinkTankCapture` (or a column on the attempt row) using `len()` bytes.
- **Worth fixing — TT2-05 (INV §16):** `ui/src/api/sse.ts:120–123` invalidates the room activity
  query on every `think_tank_activity` event, and `fetchAllActivity` (`ui/src/api/thinkTanks.ts`
  ~L131) re-pages from `after=0` up to the 5,000-record window (≈10 requests of up to 1 MiB).
  With an open room during a tool-heavy turn, each event cancels and restarts the full walk.
  Fix: append from the last seen seq on activity events (full refetch only on gap/revision per
  TS-14.R16), or throttle the invalidation.
- **Worth fixing — TT2-06 (FS-21.R31, TS-14.R3):** `runtime/activation_kinds.go:43–49` gives the
  judge the participant instruction ("your turn in a Think Tank discussion … your remaining turn
  ceiling"), and `messaging/think_tank_tools.go:79–86` returns `turn_limit/turns_remaining: 0`,
  `may_leave: false` on the judge's first page (judge member cap 0). Only the trailing guidance
  string says discussion ended. Every judge run starts with contradictory framing. Fix: omit the
  ceiling fields for the judge role and make the instruction role-neutral or add a judge kind.
  Test: judge first page has no ceiling fields.
- **Worth fixing — TT2-07 (INV §16):** `state/think_tanks.go:270–291` — `think_tank_attempts` has
  no `room_id` index (only a partial running index and `(agent_id, state)`). `readThinkTankDetail`
  (~L650) runs for every active room on each 5s sweep and every update, and room deletion cascades,
  each scanning all attempts ever recorded. Fix: migration adding
  `idx_think_tank_attempts_room(room_id)`.
- **Worth fixing — TT2-08 (INV §1):** `server/think_tanks.go` `finishThinkTankTurn` (~L327–344).
  A non-NotFound store error from Finalize/Fail logs and returns after the capture was removed;
  the attempt stays `running`, Pause/End stay requested and Delete is refused, with nothing in the
  room saying how to recover (only Stop or restart fences it). Rare, permanent when hit. Fix:
  best-effort `SetThinkTankHold`, or have the sweep fail running attempts whose turn has ended.

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

- **2026-10-06 — Design: UI polish batch.** Ready change `ui-polish-fields-icons-labels.md`
  (FS-02.R66–R70, TS-08.R88–R91): auto-grow textareas, icon Send/Cancel/Collapse, expanded card
  header order, duplicate-gated plain labels, archived projects hidden on Tasks. Absorbs the
  "Finish hiding raw ids" idea.

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
