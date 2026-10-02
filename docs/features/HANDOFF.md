# AgentDeck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-10-01 is archived in
[`HANDOFF-through-2026-10-01`](../archive/state/HANDOFF-through-2026-10-01.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** none.
- **Release:** `v0.8.0` is tagged at `a9f33c5` and published. The 13-commit range after `v0.7.0`
  ships unrestricted on-demand local text-file viewing and file-selection annotations in chat,
  plus reliable Claude model application after resume. The operator skill explains the file-viewer
  boundary; README claims and pinned release components remain current. The macOS release workflow
  passed. CI's first run hit one non-reproducing context-sharing timing assertion; the full rerun
  passed. A post-release test-only fix now waits for the held prompt at the fake-provider wire before
  measuring that context operations add no turns. Its first CI run exposed a second existing flake:
  launch-modal tests changed runtime controls before the asynchronous default model reached state;
  they now wait for that rendered default first. The GitHub Release carries the 293,072,856-byte
  `darwin-arm64` archive, `install.sh`, and a `0.8.0` manifest whose size and SHA-256 match the archive
  asset.
- **Work units:** `show-exact-context-and-runtime-metadata.md` and
  `rename-product-to-deckhand.md` are waiting to start.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Review units:** the test-only `post-release-flaky-test-synchronization` fixes and
  `phone-desktop-flow-and-agent-management.md` are available.
- **Fix units:** none available.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

None.

## Acceptance gates still owed

- FS-20.A1/A5/A6/A8: real tailnet, Android, iPhone, and `pmset -g assertions` checks. The iPhone
  Home Screen experience also still lacks a PNG touch icon. The 390px fake-provider browser pass
  covered A3/A4/A7/A9, except the fast-mode picker and Continue on an approval pause.
- FS-20.A10–A13: focused server and current UI suites pass, but the phone desktop-flow review found
  open behavior and acceptance-coverage gaps below. The combined 390px fake-provider browser journey
  for the new dashboard, project, agent-management, Files/Commands, and retired task flow also remains
  owed.
- TS-06.R21: credentialed Claude and Codex login/chat checks.
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

- **Must fix** — Claude model rejection discards the actionable provider reason (**confirmed code
  defect**). **Where:** `@agentclientprotocol/claude-agent-acp` 0.75.1 converts a failed SDK
  `query.setModel` into JSON-RPC `Internal error` with the original message under `error.data`;
  `internal/runtime/jsonrpc.go:24-31` decodes that data, but `rpcError.Error` returns only `Message`,
  and `internal/runtime/chat.go:2379-2385` therefore surfaces only `provider rejected the setting:
  model: Internal error`. **Normal-use trigger:** launch a Claude chat with an explicit model that
  the packaged Claude executable rejects, including `claude-opus-5-5` under the shipped 2.1.257
  executable. **Why it matters:** the person is told neither that AgentDeck is running a different
  Claude version nor the provider's minimum-version recovery, so `claude update` appears ineffective
  and the launch is not an honest, actionable compatibility failure. **Requirement:** TS-04.R9/R22,
  INV §8/§12. **Suggested fix/test:** preserve a bounded, sanitized provider detail for recognized
  session-configuration failures and add a launch regression whose ACP error carries
  `claude_code_version_too_old`, asserting that the response identifies the packaged runtime and
  required update without leaking arbitrary provider data.
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
  beside FS-09.R29/R46/R59, TS-04.R13, TS-06.R14-R15, and INV §10/§12/§17. **Suggested fix/test:** decide explicitly
  whether Claude chat remains release-pinned or gains a reviewed local-CLI override; at minimum bump
  the official adapter to a version embedding Claude Code 2.1.280+ after its credentialed
  compatibility gate, expose the effective packaged Claude/adapter versions before launch, and test
  that release PATH selection cannot be mistaken for the ambient CLI.

### Phone desktop flow and agent management — reviewed 2026-10-02 — **Fix model:** medium — Codex Terra or Claude Opus.

- **Must fix** — open phone screens do not enter the required archived state when the desktop
  archives their agent or project. **Where:** `ui/src/remote/AgentScreen.tsx:215,241-350` never
  branches on `agent.archived`; `ui/src/remote/ProjectScreen.tsx:65-69` filters archived projects
  out before its archived check; and that screen plus `ui/src/remote/HomeScreen.tsx:75-80` cache
  `/api/projects` under a revision-independent key. **Normal-use trigger:** keep an agent or project
  open on the phone and archive it from the Mac. The agent keeps showing its transcript and
  management actions, while the project either stays active from cached configuration or later says
  only that it is no longer on the Mac. **Why it matters:** the phone neither reflects current truth
  nor provides the promised archived explanation and return path. **Requirement:** FS-20.R17/R39,
  A10/A11; INV §1/§8/§10. **Suggested fix/test:** retain enough last-known identity for the terminal
  archived view, test `archived` before the active-project derivation, and refetch project
  configuration from an appropriate live event/revision; cover agent and project archival from the
  desktop while each phone screen is mounted.
- **Must fix** — **Open diff** does not open the selected file's diff. **Where:**
  `ui/src/remote/AgentScreen.tsx:253` discards `tracked.diff_refs[0]` and only changes the active tab
  to Chat. **Normal-use trigger:** choose **Files**, then tap **Open diff** for a changed file.
  **Why it matters:** the conversation opens at its existing position with no selected or revealed
  diff, so the principal Files-to-conversation action is inert. **Requirement:** FS-20.R37/A12;
  INV §8/§10. **Suggested fix/test:** carry the diff sequence into the conversation view and reveal
  or focus that exact diff, with a UI test that distinguishes the requested diff from other events.
- **Must fix** — the phone switch-runtime form can submit an effort from the previous model while
  displaying the new model's default. **Where:** `ui/src/remote/AgentScreen.tsx:354-366` keeps the
  draft for the component lifetime, resets effort when backend changes but not when model changes,
  and also derives the live Effort control from that draft rather than the agent's current runtime.
  **Normal-use trigger:** switch between two models on one backend whose effort vocabularies differ,
  or switch the open agent from the desktop. **Why it matters:** the selector can visibly show the
  new model/default while sending a stale unsupported effort, causing an avoidable refusal; the
  open management view can also remain based on the old runtime. **Requirement:** FS-20.R17/R36/A11;
  INV §1/§3/§8. **Suggested fix/test:** use one model-selection helper that resets to the selected
  model's default, derive immediate controls from the live agent runtime, and synchronize or reset
  the switch draft when the live runtime changes; assert the exact switch and session-config bodies.
- **Must fix** — the governing specifications still require and claim verification for phone task
  and New-work behavior removed by this unit. **Where:** `FS-20-mobile-remote-control.md:287-315`
  retains the old Home/New work/task acceptance journeys, and lines 362-373 cite the deleted
  `WorkScreens.test.tsx` and removed Re-arm test; `TS-13-remote-control.md:266-269,299-304` still
  labels shipped runtime defaults as planned and traces task attention/Re-arm. **Normal-use
  trigger:** use the FS/TS acceptance and traceability sections to decide what the current phone must
  do or which test proves it. **Why it matters:** the normative acceptance contract contradicts
  R33-R41 and points reviewers at artifacts that no longer exist, so removed task UI can appear both
  required and verified. **Requirement:** FS-20.R33-R41/A10-A13, TS-13.R19-R23; INV §10/§17.
  **Suggested fix/test:** rewrite or explicitly retire every superseded A3/A4/A9 clause, replace the
  dead traceability entries with the new dashboard/project/management/file/task-denial coverage,
  and describe R23's shipped response shape without planned markers.
- **Worth fixing** — the UI suite does not prove the new A10-A12 workflows it claims to cover.
  **Where:** `ui/src/remote/ProjectScreen.test.tsx:23-34` checks only displayed defaults without
  launching or starting a pipeline; the only new management assertion in
  `AgentScreen.test.tsx:222-228` checks archive-confirmation copy; and `RunScreen.tsx` has no focused
  test. There is no assertion for rename, effort/fast, switch, clone, desktop-driven archival,
  Files/Commands, current-file read, or the target of **Open diff**. **Normal-use trigger:** regress
  any of those newly shipped phone controls while the current shallow render tests stay green.
  **Why it matters:** the archived-state, stale-effort, and inert-diff defects above all pass the
  reported UI suite, so the acceptance evidence is not independent of the implementation's gaps.
  **Requirement:** FS-20.A10-A12; INV §17. **Suggested fix/test:** add request/response UI tests for
  every mutation and refusal-preserves-draft path, live archival tests, Files/Commands and exact
  diff-target tests, and focused Run-screen control coverage before calling the automated halves
  complete.

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

- **2026-10-02 — Review: phone desktop flow and agent management.** Four Must-fix behavior/spec
  defects and one Worth-fixing acceptance-coverage gap keep the unit open. The tailnet field filters,
  tracked-file no-follow guard, task-route denial, narrowed attention projection, runtime-option
  secrecy/defaults, shared desktop/phone dashboard helper, and presentation hooks otherwise match
  the selected requirements. INV 1–4, 7–8, 10–11, and 13–17 were reviewed; 5–6, 9, and 12 had no
  applicable changed surface. Focused remote server tests, 21 focused UI tests, the presentation
  contract, and the range whitespace check pass.
