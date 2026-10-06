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
  The 2026-10 provider bundle refresh is finished (its live-provider smokes are owed).
- **Review units:** the test-only `post-release-flaky-test-synchronization` fixes are available.
  The 2026-10 provider bundle refresh (`7c95fa9..2f39c3f`, excluding the interleaved `docs:`
  design commits) is available. Review note: TS-04.R81 was revised from "decode the backgrounded
  marker" to deriving the state from the linked task; confirm that reading.
  Think Tanks (`46379da..539ab11`) is closed again: its second-pass findings are fixed.
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

None open.

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
