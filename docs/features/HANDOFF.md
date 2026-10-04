# AgentDeck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-10-04 is archived in
[`HANDOFF-through-2026-10-04`](../archive/state/HANDOFF-through-2026-10-04.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** `rename-product-to-chuck.md` (in progress; see **Active change**).
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
- **Fix units:** `notifications-open-conversation` and
  `phone-desktop-flow-and-agent-management.md` each keep one Worth-fixing UI-coverage finding; the
  phone unit's Must-fix items are closed. Claude 5.5 launch compatibility keeps one Worth-fixing
  finding; its Must-fix is closed; the installed-provider correction it depends on still owes
  the credentialed gate.
  The former bundled-default, opt-in recovery draft FS-09.R64–R67/A33–A36 is retired;
  no product change has shipped from it.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

`rename-product-to-chuck.md` — in progress (started 2026-10-04). Module path becomes
`github.com/AsaphNoam/Chuck`; release repo `AsaphNoam/Chuck` (GitHub rename is operational).

Slices:
1. Go/scripts/release identity: module, `cmd/chuck`, `CHUCK_*`, `~/.chuck`, install tree, MCP
   name/header, hook scripts, tmux prefix, FirstMate role, `operating-chuck`, authored strings,
   annotation prefix with legacy recognition, CI old-name assertion. Keep historical
   `testdata/superseded_*` bytes; drop the `agentdecker` digest entry (no new digest machinery).
2. UI: branding, renamed storage keys with copy-forward (TS-08.R58), SharedWorker, phone surfaces.
3. Docs: README and non-archived docs/specs prose (delegated pass, reviewed here), cutover runbook
   (FS-10.R25), planned→shipped marks; closure matrix + `make dist`.

Slices 1–2 are committed together (one cut; build/tests green). Slice 3 is next: the delegated
prose pass sits uncommitted in the tree and needs review before commit. Exceptions kept on purpose:
legacy annotation line recognition, storage copy-forward keys, rejection tests, historical
`testdata/superseded_*`. `scripts/check-old-name.sh` is the TS-06.R24 guard (CI + release).
`TestCodexProberAcceptsNativeLoginWithoutAPIKey` timed out once under full-suite load; passes alone.

Tasks wire fixture regeneration: `CHUCK_UPDATE_TASK_FIXTURE=1 go test ./internal/server
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
  verify the implemented installed-provider/Bundle change (FS-09.R75–R78/A45–A47) and complete
  the bounded fixed-adapter gate in TS-06.R31.
  This finding remains open until the credentialed runtime-selection correction is verified;
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

- **2026-10-04 — Release: `v0.9.0` published.** 46 commits after `v0.8.0`. The operator package and
  README already matched the range; pins unchanged. `make test`, full UI suite (591) and
  `make dist VERSION=0.9.0` pass. Settled state archived to `HANDOFF-through-2026-10-04`. Release
  workflow passed; Linux CI's darwin-only bundle test now skips off macOS (test-only).
