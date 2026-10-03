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
- **Work units:** `rename-product-to-chuck.md` is waiting to start.
  `use-installed-provider-clis.md` is waiting to start (FS-09.R68–R74, FS-10.R20–R22;
  managed adapters with user-installed provider defaults and explicit recovery).
  `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Review units:** the test-only `post-release-flaky-test-synchronization` fixes are available.
  `notifications-open-conversation` (agent toasts and desktop notifications open the conversation;
  FS-02.R64, TS-03.R51) is available.
- **Fix units:** `phone-desktop-flow-and-agent-management.md` keeps one Worth-fixing UI-coverage
  finding; its Must-fix items are closed. Claude 5.5 launch compatibility keeps one Worth-fixing
  finding; its Must-fix is closed. The ready `use-installed-provider-clis.md` change specifies the
  remaining runtime-selection correction and its credentialed gate. The former bundled-default,
  opt-in recovery draft FS-09.R64–R67/A33–A36 is retired; no product change has shipped from it.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

None. Tasks wire fixture regeneration: `AGENTDECK_UPDATE_TASK_FIXTURE=1 go test ./internal/server
-run TestTaskWireFixture`.

## Acceptance gates still owed

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
- TS-06.R26: the credentialed Codex 1.12.0 receipt gating FS-03.A41/A42 and FS-01.A20.
- FS-02.A27: six-tab real-browser shared-stream check; A46's real-browser J14 pass; Sky & Grove
  with Codex capabilities.

## Blocked on human

None.

## Review findings

### Notifications open the agent's conversation — reviewed 2026-10-03 — **Fix model:** trivial/easy — Claude Sonnet or Codex Luna.

- **Worth fixing** — the stale-agent acceptance path is not proved by the notification tests.
  **Where:** `ui/src/components/shell/NotificationCenter.test.tsx:66-96` routes notification clicks
  to a placeholder component, so it cannot assert the required existing **Agent not found** view.
  **Normal-use trigger:** an agent disappears after its toast is raised but before the person clicks
  it. **Why it matters:** FS-02.A46 explicitly includes this recovery path, and the current test can
  keep passing if the real `/agent/:id` route stops rendering current truth for a vanished agent.
  **Requirement:** FS-02.A46, INV §17. **Suggested fix/test:** mount the real `ChatPanel` route with
  the agent store hydrated and the target absent, click the toast, and assert both **Agent not
  found** and toast dismissal.

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
  implement `docs/ready-changes/use-installed-provider-clis.md` (FS-09.R68–R74/A37–A44), selecting
  the installed provider through the managed adapter, exposing effective runtime details, and
  reporting provider-owned update guidance. Prove that an old dependency CLI cannot shadow it and
  complete the fixed-adapter cross-version gate. This finding remains open until verified;
  the retired bundled-default/opt-in draft is not the intended fix.

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

- **2026-10-03 — Design: use installed provider CLIs.** Added waiting ready change
  `docs/ready-changes/use-installed-provider-clis.md`: local Claude/Codex by default with managed
  adapters/SDK/Node, persistent existing overrides, next-process adoption, scoped runtime metadata,
  explicit local-only refresh and provider-owned recovery. FS-09.R68–R74/A37–A44,
  FS-10.R20–R22/A10–A12, TS-03.R52–R53, TS-04.R70–R74 and TS-06.R28–R29 are planned.
  Retired the unshipped bundled-default/temporary-local draft and promoted its source idea.
  A Luna design check tightened resume failures, unsaved metadata identity and refresh races;
  fixed-adapter real-provider receipts remain pre-ship gates, not existing evidence. Spec lint,
  twin-skill comparison and whitespace checks pass. No product code changed; Active change stays none.

- **2026-10-03 — Fix: Claude 5.5 launch compatibility (Must fix).** A model the bundled Claude
  executable is too old for no longer surfaces as `model: Internal error`: a recognized
  `claude_code_version_too_old` rejection now names the bundled and required Claude Code versions
  and says to update AgentDeck, not the local `claude` CLI; other provider error data stays
  unreported (INV §8, TS-04.R9 updated). Fake-ACP launch regressions cover both paths. The
  Worth-fixing adapter-bump/override finding stays open.

- **2026-10-03 — Fix: Tasks work in motion.** Both Must-fix findings closed; the unit is closed.
  Parent rows no longer claim they "lead to" delegated children — only prerequisite successors feed
  `next` (INV §8, FS-16.R42). The projection now builds successor/indegree indexes and components
  once, orders rows with a creation-ordered ready heap plus an earliest-created cycle fallback, and
  path-compresses its union-find (INV §16, TS-08.R84); 5,000-task chain and fan-out tests pin the
  order. Specifications already required this behavior. UI, Go, spec, and build checks pass.

- **2026-10-03 — Review: lean personas and shared operating context.** No findings; the unit is
  closed. Fresh and upgraded role seeding, exact-only retained-role migration, the runtime-only
  overlay across every lifecycle composer, and Claude's shared new/load native-preset builder match
  the governing requirements. The already-recorded manual role scenarios and credentialed Claude
  fresh/resume check remain acceptance gates, not code-review failures. INV 1–3, 7–12, 15 and 17
  applied without a violation; INV 4–6, 13–14 and 16 had no applicable changed surface. Spec lint
  and the focused config, runtime and server suites pass. **Fix model:** trivial/easy — Claude
  Sonnet or Codex Luna.

- **2026-10-03 — Review: notifications open the agent's conversation.** The implementation matches
  the notification-navigation requirements, but one Worth-fixing coverage gap keeps the unit open:
  the toast test substitutes a placeholder route and does not prove the specified stale-agent
  **Agent not found** recovery. INV 2, 8, 10 and 13 applied without a violation; INV 17 applies to
  the finding; INV 1, 3–7, 9, 11–12 and 14–16 had no applicable changed surface. The 31 focused UI
  tests, style/presentation checks, production UI build and diff check pass; A46's real-browser
  toast check and manual macOS desktop-notification click remain owed. **Fix model:** trivial/easy —
  Claude Sonnet or Codex Luna.

- **2026-10-03 — Work: notifications open the agent's conversation.** Agent toasts (all four types)
  open `/agent/<id>` and dismiss; a × control only dismisses; error/pipeline toasts stay
  non-navigating. Desktop Web Notification click focuses, navigates, and closes. One helper,
  `ui/src/lib/agentConversation.ts`, builds the path for both; `NotificationCenter` registers the
  router's navigate for the SSE client (importing the router from `sse.ts` would cycle). FS-02 is
  now Current. `make test`, `make build`, UI tests and `make dist` pass. INV 2, 8, 13 apply and were
  checked; no other class has a surface. Owed: A46's real-browser toast check and manual macOS
  desktop click.

- **2026-10-03 — Review: Tasks as project-grouped work in motion.** Two Must-fix findings keep the
  unit open: delegation successors are incorrectly labelled as sequential “leads to” work, and the
  graph projection is quadratic instead of the specified O(tasks + links) bound. INV 8, 16 and 17
  applied to the findings; INV 1–2, 7, 10–11 and 13 applied without another violation; INV 3–6,
  9, 12, 14–15 had no applicable changed surface. The 40 focused Tasks UI tests, Go-produced task
  wire fixture check and style/presentation contract pass. **Fix model:** medium — Codex Terra or
  Claude Opus.

- **2026-10-03 — Work: Tasks as project-grouped work in motion.** Tasks opens on All projects
  (≤4 project reads in flight), each project showing related-work groups built only from
  task-result prerequisites and parent lineage, with typed links, branches/joins, unavailable
  references, quiet creator/assignee labels and collapsed settled history. Detail expands inline
  with existing controls; run-lineage work withholds stage-restricted controls until run detail
  confirms ownership. Create and Fire signal are closed bottom disclosures with explicit projects.
  The UI now parses `waiting`, lineage, outputs and cleanup flags against a Go-marshalled fixture.
  Rendered in Core, Sky & Grove and Studio at 1024/1440px against that fixture; a live multi-agent
  run through the built binary was not done. `make test`, UI tests and `make dist` pass.

- **2026-10-03 — Work: lean personas and shared operating context.** Fresh homes seed
  AgentDecker, Implementer, Reviewer and Researcher with lean FS-18.R16 prompts; `pm`/`teammate`
  are no longer seeded or migrated, and existing files, defaults and references are untouched. The
  previous retained prompts joined the exact-match digests. Every lifecycle composer now appends a
  runtime-only standing context, with the skill pointer still package-gated. Claude chat new/load
  send the native preset with an append. `make build` passes; `make test` passed except one
  `sqlite_fts5` run of `TestCoalescedMailProducesOnePromptAndIsNeverReplayed` (unread count read
  before completion projected it), which then passed 20/20 in each variant — an existing timing
  flake, not fixed here. A12 and A13's credentialed receipt remain owed gates.

- **2026-10-02 — Design: lean personas and shared operating context.** User confirmed four
  roles, coordination within AgentDecker and preservation of existing PM/Teammate roles/references.
  FS-04.R50–R51/A30–A31, FS-18.R15–R17/A11–A13, TS-11.R15–R17 and TS-04.R69 specify lean
  personas, internal/external Researcher, runtime-only shared guidance and additive Claude prompt
  delivery. Luna research and pinned adapter/SDK evidence are retained in
  `docs/ready-changes/lean-personas-and-operating-context.md`, waiting to start; source idea promoted.
  No product code changed or active work selected. Spec, twin-skill and diff checks pass.

- **2026-10-02 — Design: notifications open the agent's conversation.** User widened the approval
  idea to every agent notification and chose the full conversation route, with the toast body
  opening and a close control dismissing. FS-02.R64/A46 and TS-03.R51 specify client-only routing
  over the existing `agent_id` payload; `docs/ready-changes/notifications-open-conversation.md` is
  waiting to start; source idea promoted. No product code changed or active work selected.

- **2026-10-02 — Design: Tasks as project-grouped work in motion.** User confirmed project-first
  connected task rows with creator as supporting context. FS-16.R41–R45/A27–A29 and TS-08.R82–R85
  specify active relationships, waiting/cleanup visibility, secondary completed history and bottom
  manual creation. `docs/ready-changes/tasks-work-in-motion.md` is waiting to start; source idea
  promoted. Existing API/query seams suffice. Incumbent browser tab returned HTTP 504; implementation
  owes the recorded before/after rendered gates. No product code changed or active work selected.

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
