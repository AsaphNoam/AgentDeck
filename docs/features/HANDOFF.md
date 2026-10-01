# AgentDeck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-10-01 is archived in
[`HANDOFF-through-2026-10-01`](../archive/state/HANDOFF-through-2026-10-01.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** `phone-desktop-flow-and-agent-management.md` — in progress. Planned slices:
  shared desktop/phone project derivation and narrowed home data; remote route/filter and tracked-file
  security contracts; phone dashboard/project/launch/pipeline flow; phone agent management and
  Files/Commands; removal of phone task/New work surfaces; final browser and closure verification.
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
  `rename-product-to-deckhand.md` are waiting to start. `phone-desktop-flow-and-agent-management.md`
  is in progress.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Review units:** the test-only `post-release-flaky-test-synchronization` fixes are available.
- **Fix units:** none available.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

### Phone desktop flow and agent management

- **Direction:** experienced remote operators need the dashboard → project → chat path to make the
  next agent-level action fastest. Home reads Needs you first, then a dense project list; project
  pages put New agent and Start pipeline together above ordered agents and active runs. Lifecycle
  state, runtime identity, and tracked work are the AgentDeck-specific proof, while configuration,
  task UI, mail, and arbitrary files stay quiet or absent. Reuse the compact phone shell and shared
  chat primitives; no new skin, motion system, or desktop behavior. Use no decorative motion.
- **UX risk:** removing New work/tasks must leave old links and notifications with a clear Home
  recovery; rejected launches and pipeline starts must preserve entered values; archive must state
  that restore remains desktop-only.
- **Next:** implement the shared derivations and narrowed server projections first, with focused
  tests, then checkpoint before adding phone routes and controls.
- **Verified checkpoint:** tailnet Home/push now exclude tasks and expose active runs; runtime
  defaults, approved agent-management and tracking routes, complete task-route denial, and exact
  tracked-path file reads with final-component no-follow are implemented. `go test
  ./internal/server -count=1` passes. The phone/dashboard UI slice is committed separately; its
  runtime-default and archive-confirmation integration fixes remain in progress.
- **Next:** close the phone UI integration findings, retire superseded source/tests, then run the
  combined browser and closure matrix.

## Acceptance gates still owed

- FS-20.A1/A5/A6/A8: real tailnet, Android, iPhone, and `pmset -g assertions` checks. The iPhone
  Home Screen experience also still lacks a PNG touch icon. The 390px fake-provider browser pass
  covered A3/A4/A7/A9, except the fast-mode picker and Continue on an approval pause.
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
