# Chuck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-10-04 is archived in
[`HANDOFF-through-2026-10-04`](../archive/state/HANDOFF-through-2026-10-04.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** `think-tanks.md` (in progress; see **Active change**).
- **Release:** `v0.9.0` is tagged at `ae93666` and published; the macOS release workflow passed. The
  GitHub Release carries the 293,150,597-byte `darwin-arm64` archive, `install.sh`, and a `0.9.0`
  manifest matching that size. Linux CI then failed `TestPublishedRootSelectsTheBundledProviders`:
  the bundle path exists only on darwin by design, so the test now skips elsewhere. The 46-commit range after `v0.8.0` ships the installed-provider
  default with explicit per-backend bundle, lean personas with shared operating context, Tasks as
  project-grouped work in motion, notifications that open the conversation, exact context tokens,
  and the phone desktop flow. The operator package and README already matched the range.
  Credentialed Claude/Codex gates remain owed.
- **Work units:** `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Review units:** `use-installed-provider-clis` (2026-10-04, `4d1e9cc^`..`6c5c52f`: shared provider
  resolver, Installed default/explicit AgentDeck bundle, release wrapper, typed recovery,
  provider_runtimes + Refresh provider, Settings/New Agent UI, docs; FS-09.R68/R70–R77,
  FS-10.R21/R23–R24, TS-03.R52–R54, TS-04.R71–R77, TS-06.R30) was reviewed 2026-10-04 and its
  findings were fixed the same day; the unit is closed apart from its owed credentialed and
  rendered gates. The test-only `post-release-flaky-test-synchronization` fixes are available.
  `rename-product-to-chuck` (2026-10-04, `36afbf2`..`3855419`: module
  `github.com/AsaphNoam/Chuck`, `cmd/chuck`, `CHUCK_*`/`~/.chuck`, Chuck install tree/archive,
  `chuck-messaging`/`X-Chuck-Token`, `chuck-` tmux prefix, resident role (now Chucky),
  `operating-chuck`, UI/phone branding, legacy annotation recognition, storage copy-forward, `scripts/check-old-name.sh`,
  docs and `docs/chuck-cutover.md`; FS-00.R19, FS-04.R52, FS-10.R15/R25–R26, FS-13.R24,
  FS-18.R18, TS-02.R40–R41, TS-04.R78, TS-06.R24, TS-08.R58, TS-11.R18) was reviewed 2026-10-05;
  its findings were fixed 2026-10-05 and the resident role renamed FirstMate → Chucky (`chucky`);
  the unit is closed apart from its owed rehearsal, repository rename and release notice.
  Historical old-name mentions were kept deliberately (retired items, `AGENTDECK_CODEX_VERSION`
  history, the Figma URL, the retired BRIEFS, the uncommitted `docs/ideas.md` edit).
- **Fix units:** `phone-chat-streamed-deltas` closed 2026-10-05 (phone folds the transcript with
  `foldTranscript`; `ui/scripts/phone-render.mjs` renders the phone conversation at iPhone size).
  `notifications-open-conversation` closed 2026-10-05 apart from its owed browser/macOS click
  checks. `phone-desktop-flow-and-agent-management.md` closed 2026-10-05 apart from its owed
  fakeACP browser passes at phone size (FS-20.A10–A11). Claude 5.5 launch compatibility closed
  2026-10-05 (live Installed probe plus alias-row fix); the broader TS-06.R31 smoke stays owed.
  The former bundled-default, opt-in recovery draft FS-09.R64–R67/A33–A36 is retired;
  no product change has shipped from it.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

`think-tanks.md` — in progress (started 2026-10-06). FS-21 over TS-14; all items stay `(planned)`
until their slice ships. Slices (each closes with focused tests, handoff update and commit):

1. **State** — done 2026-10-06 (`internal/state/think_tank{s,_turns}.go`, schema v36; tests
   `go test ./internal/state -run ThinkTank`, both variants and `-race` passed). Seam map and
   slice notes: `docs/plans/think-tanks.md`.
2. **Room engine** (next) — server service selecting the next opportunity and dispatching a `think_tank`
   activation through `StartActivation`; turn-end finalization; executing-turn-id emission (R4).
3. **MCP tools** — `read_think_tank`/`submit_think_tank_turn`, approval exemption, redacted
   activity projector.
4. **REST/SSE** — `/api/think-tanks` family, `think_tank_update`/`think_tank_activity`, wire fixture.
5. **Activity capture, judge, recovery/deletion.**
6. **UI** — project button and setup, room page, Archive entries, annotations/file route, agent-view
   room-turn identification.
7. **Knowledge/docs + closure matrix**, rendered and credentialed gates recorded as owed.

Tasks wire fixture regeneration: `CHUCK_UPDATE_TASK_FIXTURE=1 go test ./internal/server
-run TestTaskWireFixture`.

## Acceptance gates still owed

- FS-10.R25/A14, TS-02.R41: one supervised cutover rehearsal on a disposable copy of the real
  AgentDeck home, following `docs/chuck-cutover.md`, with a receipt. Before the first Chuck release
  the GitHub repository must be renamed to `AsaphNoam/Chuck` (installer and updater fetch there),
  and the release notes must say paired phones re-pair at the new `chuck` address.

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

None.

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

- **2026-10-06 — Design: Think Tanks ready to implement.** Human confirmed the remaining feature
  defaults and SQLite plus explicit room tools. FS-21.R34–R40/A24–A30 and TS-14.R1–R18 close
  setup, retention, recovery, publication/read checkpoints and turn ownership; adjacent FS/TS
  requirements join shared launch, action, approval, persistence, UI and knowledge contracts.
  Added `docs/ready-changes/think-tanks.md`, removed the source idea, and left implementation
  inactive. Spec lint, twin-skill comparison and diff checks passed; a focused consistency check
  confirmed the ownership/read/privacy rules and clarified pending-judge deletion and catalog wording.

- **2026-10-05 — Design: Think Tank judge, manual ending and Steer confirmed.** FS-21.R31–R33/
  A21–A23 record the fresh end-only judge, graceful End discussion, and normal private Steer with
  visible room-turn identity. FS-03.R69/A50 add planned agent-view identification. Remaining product
  defaults are consolidated as unconfirmed proposals in FS-21 §6; technical design awaits scope
  approval. No product code was written.

- **2026-10-05 — Design: Think Tank conversation page and annotation destinations confirmed.**
  FS-21.R27/R30/A20 and FS-13.R27/A18 record the ordinary full conversation-page navigation and
  Room, selected-agent and New task destinations. New task retains its current meaning of launching
  a new normal agent with annotations as initial work. After room completion, selected-agent/new-agent
  follow-up remains available. Judge setup, End discussion and active-room steering await choices.

- **2026-10-05 — Design: Think Tank workspace entry and independent follow-up clarified.**
  FS-21.R27–R29/A17–A19 record the project-started separate workspace, normal participant cards in
  their own projects, continued room scheduling with waits for private work, and room-only activity.
  FS-02.R65/A47 add the planned button beside New agent (FS-02 now Partial). Page versus browser
  window, annotation targets and active-room steering remain open. No product code or technical
  design was added.

- **2026-10-05 — Design: Think Tank workspace and retention confirmed.** FS-21.R22–R26/A14–A16
  record cross-project participation limited to non-archived projects, local retention until explicit
  room deletion, and a full group-chat workspace with familiar features plus independent agent
  cards. FS-13.R26/A17 add planned room annotation sources (FS-13 now Partial). Card arrangement,
  private-send scheduling and room-versus-private activity visibility await choices. No product
  code or technical design was added.

- **2026-10-05 — Design: Think Tank pause, intervention and closing message confirmed.**
  FS-21.R18–R21/A11–A13 let the active turn finish on pause, hold the room visibly for approval or
  failure intervention, and end below two eligible participants with a closing-message opportunity
  within the sole remaining agent's allowance. Failed attempts do not consume the contribution
  limit and are not retried automatically. R8 is superseded by R12/R20; FS-21 remains Draft.

- **2026-10-05 — Design: Think Tank budget and operator controls confirmed.** FS-21.R14–R15/A8–A9
  record per-agent turn ceilings and pause/resume plus user messages between participant turns.
  FS-21.R16–R17/A10 also record the confirmed remaining-ceiling instruction, opening/departure
  accounting, and separate end-only judge budget. Pause/recovery and sole-participant closure were
  subsequently confirmed in R18–R21. No technical design or product code was added.

- **2026-10-05 — Design: Think Tank participation and synthesis confirmed.** FS-21.R11–R13/A6–A7
  record mixed new/existing participants, configurable departure permission and optional final
  departure message, and optional synthesis by a judge activated only after discussion ends.
  Discussion budget and operator controls are awaiting choices; the feature remains Draft.

- **2026-10-05 — Design: Think tanks resumed.** Replaced the initial consensus/group-chat framing
  with independent normal provider sessions and a canonical append-only shared discussion,
  incremental retrieval, optional independent openings, bounded discussion, and permitted departure
  ending when one participant remains. FS-21 is Draft; unresolved behavior is recorded there and
  in `docs/ideas.md`. No technical specification, ready change, or product code was written.

- **2026-10-05 — Design: provider bundle refresh ready.** Upstream check found Claude ACP 0.85.1,
  Codex ACP 2.1.1 (Codex `^0.159.1`, so 0.159.3), ACP SDK 1.7.0 and go-sdk 1.8.0. All ACP Wait-list
  gates stay closed; the Codex steering patch is still required; the MCP-migration gate is rechecked
  and closed, with MCP-over-ACP recorded as the transport to watch (TS-04 §5, FS-17 §6).
  `refresh-provider-bundle-2026-10.md` is waiting to start: FS-03.R67–R68/A48–A49,
  FS-09.R79/A48, TS-04.R79–R83, TS-06.R32, TS-08.R86 (TS-08 now Partial).

- **2026-10-05 — Claude 5.5 launch compatibility closed.** A live probe (release adapter over
  installed Claude Code 2.1.282) served `claude-opus-5-5`; the bundle still refuses it as too old.
  The probe exposed a remaining launch failure (INV §12 accepted-is-not-honored, over-applied):
  the Claude adapter answers a full model ID with its alias row (`opus`/`opus[1m]`), which Chuck
  rejected as an ignored setting. A listed reported row now honors an unlisted Claude model
  request (TS-04.R46 note); listed requests and other backends stay strict. Fake-adapter test
  failed first; Chuck's real launch path passed live and failed without the fix.

- **2026-10-05 — Phone UI coverage completed; Manage refusals shown.** Request/response tests now
  cover project-page launch and Start pipeline (with refusals keeping entered values), rename,
  fast/effort, clone, Commands, Open file, `RunScreen` controls, and the `/task/<id>` link
  (INV §17 independent oracle: MSW doubles return the server's error envelope). The refused-rename
  test failed first: the Manage tab never showed the Mac's reason (FS-20.R27); it now does. Phone
  UI tests (45) pass. Unit `phone-desktop-flow-and-agent-management` closed apart from its owed
  fakeACP phone-size browser passes.

- **2026-10-05 — Notification stale-agent path proved.** `NotificationCenter.test.tsx` now mounts
  the real `ChatPanel` route with a hydrated store lacking the agent, clicks the toast, and asserts
  **Agent not found** plus dismissal (INV §17 independent oracle; fails when the store is left
  unhydrated). UI tests pass. Unit `notifications-open-conversation` closed apart from FS-02.A46's
  owed real-browser and macOS click checks.

- **2026-10-05 — Phone chat streamed replies fixed.** The phone conversation folds its joined
  windows with the desktop's `foldTranscript`, so a streamed reply is one message (INV §2 canonical
  helpers); the forked permission folding is gone. `ui/scripts/phone-render.mjs` screenshots the
  phone conversation at iPhone size with stubbed APIs (INV §13), documented in TS-06; Playwright
  1.63.0 is a UI dev dependency. Reproduction test un-skipped (failed before, passes); UI tests
  pass. Unit closed.

- **2026-10-05 — Rename fixes; resident role is Chucky.** Cutover now rewrites and audits frozen
  session paths (`cwd`, `add_dirs`, prompt; `launch_config_json` audited), and A14 rehearses a
  resumed worktree session with the source unavailable (INV §7/§10/§15). Storage rationale names
  `~/.chuck` (INV §10). FirstMate/`firstmate` → Chucky/`chucky` across code, UI and specs. Both Go
  variants, spec/old-name checks and 107 affected UI tests pass. Rehearsal, repository rename and
  phone re-pair notice remain.

- **2026-10-05 — Rename reviewed.** Runtime, protocol, release and UI wiring passed the focused
  audit. Recorded the cutover's missing frozen-session path repair and stale current storage
  rationale; the rename unit remains open for those fixes and its already-recorded cutover gate.
- **2026-10-04 — Rename: AgentDeck is now Chuck.** Code, release, UI and docs renamed in one cut;
  supervised cutover guide written. `make test`, UI suite (595) and `make dist` pass. Rehearsal,
  GitHub repository rename and first Chuck release remain.

- **2026-10-04 — Release: `v0.9.0` published.** 46 commits after `v0.8.0`. The operator package and
  README already matched the range; pins unchanged. `make test`, full UI suite (591) and
  `make dist VERSION=0.9.0` pass. Settled state archived to `HANDOFF-through-2026-10-04`. Release
  workflow passed; Linux CI's darwin-only bundle test now skips off macOS (test-only).
