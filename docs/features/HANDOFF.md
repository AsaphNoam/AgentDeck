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
- **Work units:** `rename-product-to-deckhand.md` is waiting to start.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Review units:** the test-only `post-release-flaky-test-synchronization` fixes are available.
- **Fix units:** `phone-desktop-flow-and-agent-management.md` keeps one Worth-fixing UI-coverage
  finding; its Must-fix items are closed.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

None.

## Acceptance gates still owed

- FS-20.A1/A5/A6/A8: real tailnet, Android, iPhone, and `pmset -g assertions` checks. The iPhone
  Home Screen experience also still lacks a PNG touch icon. The 390px fake-provider browser pass
  covered A3/A4/A7/A9, except the fast-mode picker and Continue on an approval pause.
- FS-20.A10–A13: focused server and UI suites pass; the remaining phone UI-coverage gaps are listed
  under Review findings. The combined 390px fake-provider browser journey
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

- **Worth fixing** — the phone UI suite still does not prove several A10-A12 workflows. The
  2026-10-02 fix added desktop agent/project archival, switch-runtime effort/live-sync, and exact
  **Open diff** target tests. **Still uncovered:** request/response UI tests for New agent launch
  and Start pipeline from a project page, rename, effort/fast, clone, refusal-preserves-draft paths,
  the Commands view, **Open file** content, focused `RunScreen.tsx` controls, and the `/task/<id>`
  redirect. **Normal-use trigger:** regress one of those phone controls while the suite stays green.
  **Requirement:** FS-20.A10-A13; INV §17. **Suggested fix/test:** add those UI request tests and
  update FS-20 §7 traceability, which now names each gap.

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

- **2026-10-02 — Design: lean personas and shared operating context.** Drafted
  FS-04.R50–R51/A30–A31 and FS-18.R15–R17/A11–A13 from the user's four-role request and Luna
  research. Proposed AgentDecker-owned coordination, internal/external Researcher, concise shared
  context for every role and additive native-provider prompt delivery. Awaiting confirmation of
  coordination placement and preservation of existing PM/Teammate roles/references before technical
  design. Recorded research and resumption details in `docs/ideas.md`; no product code or ready change.

- **2026-10-02 — Design: notifications open the agent's conversation.** User widened the approval
  idea to every agent notification and chose the full conversation route, with the toast body
  opening and a close control dismissing. FS-02.R64/A46 and TS-03.R51 specify client-only routing
  over the existing `agent_id` payload; `docs/ready-changes/notifications-open-conversation.md` is
  waiting to start; source idea promoted. No product code changed or active work selected.

- **2026-10-02 — Review: exact context tokens and expanded-card runtime metadata.** No findings;
  the unit is closed. **Fix model:** trivial/easy — Claude Sonnet or Codex Luna. The ACP decode,
  durable status/session tuple, turn rollup and reindex paths, resume/switch restoration,
  `AgentState` projection, shared meter, and expanded-card runtime identity match the named
  requirements. The rendered matrix at 1024×900 and 1280×720 kept long metadata, state, Collapse,
  and the exact-token meter separated without overflow in Core, Sky & Grove, and Studio. INV
  1–3, 5, 7–11, 13, and 16–17 applied without a violation; 4, 6, 12, 14, and 15 had no applicable
  changed surface. Both Go variants, the tagged build, all 558 UI tests, the style contract, and
  the production UI build pass.

- **2026-10-02 — Work: exact context tokens and expanded-card runtime metadata.** ACP
  `usage_update` decodes one reading (capped percentage plus the reported used/size pair; a
  missing, non-integer or out-of-range pair rejects the update). Migration 35 adds nullable pairs to
  `status` and `sessions`; status writes, turn_end, rollup, reindex, resume and switch carry the
  pair with the percentage, and a percentage-only hook clears it. `AgentState` gains optional
  `context_used`/`context_size`; `ContextBar` renders `12,345 / 200,000 tokens · 6% context used`
  on the chat header and expanded card, and the expanded card shows backend · model · effort. A
  real-browser check of the visual matrix at 1024/1440 in all three appearances found the longer
  label running under the state badge; the expanded action side now wraps the meter onto its own
  row, verified overlap-free. INV 1–3, 7–8, 10–11, 13, 16–17 applied; 4–6, 9, 12, 14–15 had no
  changed surface. A follow-up live-app pass (`go run ./scripts/stress-fixture`: production server
  and embedded UI with the fake ACP) showed `44,000 / 200,000 tokens · 22% context used` on the
  expanded project card and the agent screen, kept it across reload, showed `claude · haiku ·
  medium` on the expanded card, and no meter on collapsed cards. No paid real-provider run.

- **2026-10-02 — Fix: phone desktop flow and agent management.** Open phone agent and project
  screens now enter the archived state when the desktop archives them (INV §1 republish derived state; project
  configuration refetches on the live revision); **Open diff** shows exactly the requested diff
  (INV §8); the switch-runtime form resets effort to the chosen model's default and follows live
  runtime changes, and the immediate Effort control uses the live model (INV §1/§3). FS-20 A3/A4/A9 and
  traceability plus TS-13's runtime-options shape and R17 trace now describe the shipped phone (INV
  §17). An empty project archived on the desktop still reaches an open phone screen only on refresh
  (FS-20 §6). The UI-coverage Worth-fixing item stays open.
- **2026-10-02 — Review: phone desktop flow and agent management.** Four Must-fix behavior/spec
  defects and one Worth-fixing acceptance-coverage gap keep the unit open. The tailnet field filters,
  tracked-file no-follow guard, task-route denial, narrowed attention projection, runtime-option
  secrecy/defaults, shared desktop/phone dashboard helper, and presentation hooks otherwise match
  the selected requirements. INV 1–4, 7–8, 10–11, and 13–17 were reviewed; 5–6, 9, and 12 had no
  applicable changed surface. Focused remote server tests, 21 focused UI tests, the presentation
  contract, and the range whitespace check pass.
