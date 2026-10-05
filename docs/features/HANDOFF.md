# Chuck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-10-04 is archived in
[`HANDOFF-through-2026-10-04`](../archive/state/HANDOFF-through-2026-10-04.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** none.
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
  fakeACP browser passes at phone size (FS-20.A10–A11). Claude 5.5 launch compatibility keeps one Worth-fixing
  finding; its Must-fix is closed; the installed-provider correction it depends on still owes
  the credentialed gate.
  The former bundled-default, opt-in recovery draft FS-09.R64–R67/A33–A36 is retired;
  no product change has shipped from it.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

None. Tasks wire fixture regeneration: `CHUCK_UPDATE_TASK_FIXTURE=1 go test ./internal/server
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
  probes run first in release CI. The Claude 5.5 finding below closes only with that receipt.
- TS-06.R26: the credentialed Codex 1.12.0 receipt gating FS-03.A41/A42 and FS-01.A20.
- FS-02.A27: six-tab real-browser shared-stream check; A46's real-browser J14 pass; Sky & Grove
  with Codex capabilities.

## Blocked on human

None.

## Review findings

### Claude 5.5 launch compatibility — reported 2026-10-01 — **Fix model:** medium — Codex Terra or Claude Opus.

**Report (verbatim).** “claude can't deploy opus5.5 or sonnet 5.5. Setting backend to sonnet / opus defaults to version
  5, setting to claude-sonnet- 5-5 or claude-opus-5-5 both fail runtime: provider rejected the
  setting: model: Internal error. After launching claude opus 5 I can run /model
  claude-sonnet-5-5 and it works, but running /model claude-opus-5-5 returns API error: 400
  {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"message\":\"Claude Code
  2.1.257 does not support this model; version 2.1.280 or newer is required. Run 'claude update', or
  update the Claude desktop app, then try again.\",\"details\":{\"error_code\":
  \"claude_code_version_too_old\"}},\"request_id\":\"req_011Cfb3xhNe67bnHyTaUpWs5\"}. My local
  CLI version is 2.1.286, checked in the same terminal running agentdeck before starting the
  dashboard. Asking the claude within agentdeck revealed that the agentdeck bundle is at 2.1.257.
  Why did we choose to bundle versions separately and not use what the user has installed locally?”
  AgentDeck version was not stated; the current shipped release is v0.8.0. No separate log file was
supplied.

- **Worth fixing** — the packaged Claude runtime lags the minimum needed for Opus 5.5, with no
  version disclosure analogous to packaged Codex (**confirmed specification gap and compatibility
  limitation**). **Where:** `scripts/release/package.json:10` pins `claude-agent-acp` 0.75.1, whose
  locked `@anthropic-ai/claude-agent-sdk` 0.3.257 embeds Claude Code 2.1.257; the release wrapper
  prepends that private adapter to PATH. FS-09.R29 and TS-04.R13 deliberately make that adapter own
  the chat executable, while FS-09.R59 exposes exact packaged-runtime/cache compatibility only for
  Codex. **Normal-use trigger:** the local Claude CLI is new enough for a newly available model, but
  a chat launch uses the older immutable release runtime. **Why it matters:** exact model selectors
  can be configured and passed verbatim yet remain unusable until AgentDeck ships a dependency
  bump, and the UI gives no way to distinguish that state from the user's installed CLI. The moving
  `sonnet`/`opus` aliases resolving to version 5 is otherwise expected under FS-09.R46, not a model
  translation bug; terminal agents remain direct-user-CLI launches. **Requirement:** coverage gap
  beside FS-09.R29/R46/R59, TS-04.R13, TS-06.R14-R15, and INV §10/§12/§17. **Suggested fix/test:**
  verify the implemented installed-provider/Bundle change (FS-09.R75–R78/A45–A47) and complete
  the bounded fixed-adapter gate in TS-06.R31.
  This finding remains open until the credentialed runtime-selection correction is verified;
  the retired bundled-default/opt-in draft is not the intended fix.

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
