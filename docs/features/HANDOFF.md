# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Active change:** none.
- **State:** paused — no active change. Project shared resources shipped (FS-11 Current). The open
  installer flag-preservation review finding still blocks a macOS release; credentialed provider
  acceptance remains a manual gate.
- **Last reviewed code:** `87d6251` (2026-07-16), across the continuous range after `d260f93`.
- **Branch:** `main`.

## Decisions needing your input

These are shipped boundaries documented in the specifications, not blockers. A future reversal needs
an explicit specification update; remove an item when the human accepts the current rule or queues
that update.

- **Local API authentication:** TS-05.R3 documents the current same-machine trust boundary. Decide
  whether a token/UI handshake is worth the added setup and compatibility cost.
- **Child-process environment:** TS-05.R8 documents full environment inheritance minus backend strip
  keys. Decide whether to trade provider compatibility for allowlists.
- **Terminal and messaging support boundary:** FS-07/TS-04 document Claude-only terminal support and
  non-messageable terminal agents pending real-CLI verification.
- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.

## Acceptance gates

- [ ] Run pinned, credentialed Claude and Codex chat/MCP/resume checks before claiming those combinations.
- [ ] Run pinned Claude terminal flags/hooks and live xterm journeys before claiming full terminal support.
- [ ] Run pinned OpenCode/OpenHands launch/credential checks before claiming those backends beyond fakes.
- [ ] Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude and
  Codex installations before promoting FS-08/TS-07 from Partial.

## Blocked on human

Live-provider acceptance is waiting for human authorization because it invokes real provider sessions
and creates disposable local configuration homes. On 2026-07-15 this machine has Claude Code 2.1.202,
the retired `claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new
`claude-agent-acp`, OpenCode, and OpenHands are not installed globally.

## Review findings

- **Must fix — locking re-exec drops explicit non-interactive/no-start behavior.**
  `scripts/release/install.sh:73-77` consumes `--non-interactive` and `--no-start` before
  `scripts/release/install.sh:108-114` re-executes the script under `lockf`, but that command passes
  only `AGENTDECK_VERSION` and the now-empty `"$@"`. On an interactive terminal, the locked child
  resets both flags to zero and can prompt, edit `.zshrc`, run provider sign-in, and start the
  dashboard despite the original flags. This violates FS-10.R6/R12 and makes the new locking path
  unsafe for scripted use. Preserve the parsed flags across the re-exec (or acquire the lock before
  parsing) and add a pseudo-terminal bootstrap test for both options.

- **Must fix — J9: an incomplete `backends.json` kills the whole dashboard.** A syntactically valid
  but incomplete `backends.json` (e.g. `{"version":2}` with no `backends` key, or a backend with
  `"models":null`) is neither `ErrNotFound` nor `ErrCorrupt`, so `internal/config/backends.go`
  `ReadBackends` / `internal/server/handlers.go` skip the `DefaultBackends()` fallback and
  `GET /api/backends` returns `{"backends":null}`. `ui/src/features/settings/BackendsEditor.tsx:87`
  then runs `Object.entries(cfg.backends)` unguarded (also `:256` on `models:null`), throwing
  `TypeError: Cannot convert undefined or null to object`; the app-level ErrorBoundary replaces the
  entire dashboard with "Something went wrong in dashboard." Reproduced live (config is documented as
  hand-editable, so this is a realistic edit). Normalize an incomplete backends file to defaults (like
  the missing/corrupt cases) and/or guard `cfg.backends ?? {}` / `backend.models ?? {}`, and name the
  offending file in any error. Relevant: TS-03 serialization contract, FS-04 backends, INV
  nil-collection class. Repro in the 2026-07-16 usability run report.

- **Must fix — J8/J3: user prompts are never persisted to the durable transcript.** `SendPrompt`
  (`internal/runtime/chat.go`) emits no user event and `internal/runtime/event.go` has no user/prompt
  event type, so `GET /api/sessions/{id}/transcript` and the archive view contain only assistant/tool/
  turn events. The chat panel shows the user's text optimistically (FS-03.R7) but FS-03.R12 replaces
  the view with the durable transcript on every reconnect, so after any reload/resume/archive the
  user's side of the conversation is gone, and FS-05.R5 transcript-content search cannot match what
  the user typed (`q=<assistant word>` matches; `q=<user word>` returns 0). Extends the 2026-07-12
  MINOR advisory with the archive-one-sided and unsearchable dimensions. Persist a user-prompt
  transcript event (or explicitly document the omission in FS-03/FS-05). Repro in the 2026-07-16 run
  report.

- **Worth fixing — J2: credential-check failures show raw codes and a misleading fix hint.**
  `ui/src/features/onboarding/steps/BackendStep.tsx` renders `Credential check: skipped —
  cli_not_installed. Please check your settings and try again.` (and `failed — not_logged_in`,
  `no_api_key`). The snake_case status codes and generic "check your settings" misdirect: a missing
  adapter or a not-logged-in provider is not a settings problem and the copy names no next step.
  Replace with human copy naming the specific problem and remediation (install the adapter / run
  guided sign-in / add the API key). FS-04.R17 leaves the copy unspecified — candidate to pin.

- **Worth fixing — S2/J9: the config-source panel renders unstyled.**
  `ui/src/features/settings/ConfigSourcePanel.tsx` references `source-unbound`, `source-bound`,
  `source-preview`, `source-field-value`, `src-override-model`, `src-override-effort` — none defined
  in the stylesheets; `.source-unbound` computes with no border/padding/background. Adds to the
  previously-noted `interface-controls` / `cred-chip` undefined cluster. Define or remove the classes.
  Functional (degrades, no crash), so lower priority.

## Recent changelog

_(Newest first; durable product truth is in FS/TS and history is in git.)_

- 2026-07-16 — Usability review drove J1–J3, J5, J8 (tagged + untagged), and J9 (incl. FS-11) against
  the real binary in a browser. Four findings recorded: a hand-edited incomplete `backends.json`
  crashes the whole dashboard (new, Must fix); user prompts are never persisted to the transcript so
  archives are one-sided and user text is unsearchable (Must fix, extends a known advisory); credential
  failures show raw codes with a misleading hint (Worth fixing); the config-source panel is unstyled
  (Worth fixing). J1/J3/J5/J8/J9 core paths and the full onboarding walk passed with zero console
  errors; FS-11's read-only resource_dir surfaces correctly. J4/J6/J7/J10/J11 were not exercised. Full
  report: [`../archive/reviews/usability-review-run-2026-07-16.md`](../archive/reviews/usability-review-run-2026-07-16.md).
- 2026-07-16 — Review found no unreviewed product code after the recorded project-resources review boundary. The installer flag-preservation finding remains the only open review finding.
- 2026-07-16 — Review through `87d6251` found the project shared-resources work sound: launch,
  resume, and switch inject the owner-only resource directory through one shared helper; project
  responses expose only the path and never the contents; and the specifications match the code in
  both directions. No new findings. The open installer flag-preservation regression still stands.
  Spec checks and the targeted config/server tests pass.
- 2026-07-15 — Shipped project shared resources (FS-11 Current): every project gets an
  AgentDeck-owned owner-only `project-resources/{id}/` directory outside its repository, created on
  project creation and lazily before launch, injected into launch/resume/switch as
  `AGENTDECK_PROJECT_RESOURCES` + an add_dir + a composed instruction, exposed as a read-only
  `resource_dir` in project responses and Settings, and retained on project deletion. FS-11, TS-02,
  TS-03, and TS-05 flip to Current. `make check-specs`, `make test`, `make build`, `ui` test/build,
  and `make dist` pass.
- 2026-07-15 — Review through `ccd0a51` found that the release-installer lock re-exec loses
  explicit no-start/non-interactive flags. Specification, test, build, and distribution checks pass,
  but the existing non-terminal bootstrap test does not cover the interactive trigger.
- 2026-07-15 — Renamed the explicit review command to `/review` in the Codex and Claude skill
  copies; it retains the same unreviewed-range review behavior.
- 2026-07-15 — Renamed the explicit build/finding-fix commands to `/work` and `/fix`. `/work`
  now finds the sole waiting ready change (or asks the user to choose when several wait), so an
  explicit request no longer reports no work while implementable work is available.
- 2026-07-15 — Defined the waiting project shared-resources change: every project will receive an
  AgentDeck-owned owner-only folder outside its repository, injected consistently into agent
  launches and retained after project deletion. It is ready to start and is not active work.
- 2026-07-15 — Fixed the release-path review findings (INV §9): bootstrap and updater lock claims
  now cover resolution/download through activation, the stable shim is fsynced then atomically
  renamed, and the arm64 macOS release workflow runs release/CLI coverage plus a bootstrap journey.
  `make check-specs`, `make test`, `make build`, and `make dist` passed.
- 2026-07-15 — Review through `d260f93` recorded three must-fix macOS release defects: full-operation
  installer/update contention is not serialized, the stable shim is written in place, and release CI
  omits required delivery checks. Shared specification, Go (both variants), build, and distribution
  checks passed.
- 2026-07-15 — Shipped the Apple-silicon macOS GitHub Releases installer: verified private Node and
  Claude/Codex ACP runtime, guided sign-in, stable shim, explicit update/rollback, no-start mode,
  release assembly/publish workflow, and release documentation. Automated checks are green; real
  provider sign-in remains credential-gated.
- 2026-07-15 — Claude chat and credential checks now target the pinned official
  `@agentclientprotocol/claude-agent-acp` package; source installs enforce its Node 22 floor.
- 2026-07-15 — Defined the waiting macOS arm64 GitHub Releases installer change: a private Node and
  Claude/Codex ACP runtime, optional guided sign-in, explicit update/rollback, checksums, and no
  signing/notarization. It is ready to start and does not make the release installer active yet.
- 2026-07-15 — Added a collaborative feature-design workflow that turns one idea into confirmed
  planned specifications and a ready change without starting implementation.
- 2026-07-14 — Codex backends can opt into `autosync_models`: on startup AgentDeck add-only merges
  the Codex CLI's `models_cache.json` into the catalog (FS-09.R28/A8). Claude autosync stays an idea.
- 2026-07-15 — Confirmed detached federation import remains deliberately unshipped: `detach=true`
  returns `501 not_implemented`; source assets remain reference-only until a verified provider launch-
  injection design exists. It is a known capability gap, not a human decision awaiting resolution.
- 2026-07-14 — New Agent modal now defaults the name to just the (capitalized) role instead of
  `Role-project` (FS-01.R1 auto-suggest; format not pinned).
- 2026-07-14 — Project ids are now server-derived from the title (`slug(title)-<timestamp>`); the
  Settings and onboarding project forms no longer ask for a slug (FS-04.R31/A11).
- 2026-07-14 — Replaced letter-number future-work labels with plain-language ideas, known
  improvements, ready changes, and current-change records.
- 2026-07-14 — Limited Claude and Codex workflow skills to their explicit slash-command triggers.
- 2026-07-14 — Added archive notices explaining that old process labels are historical and must
  not be followed; older live briefs now carry the same context.
- 2026-07-14 — Removed repeated user-intent classification from agent instructions; only the
  no-self-prioritization rule remains.
- 2026-07-14 — Simplified agent instructions: removed specialist process labels while keeping
  stable requirement IDs and plain-language human updates.
- 2026-07-13 — SDD foundation complete: authoritative FS/TS/INV contracts, lifecycle, archive
  manifest, requirement-link lint, local hook, CI, role workflows, and verification landed.
- 2026-07-14 — Changes waiting to start moved out of the handoff; the handoff now records only the
  change in progress.
- 2026-07-12 — Federation bindings hydrate on restart so watch/sweep detects external edits.
- 2026-07-12 — Restart-orphaned runtimes are reaped by Stop/Switch/Release.
- 2026-07-12 — Onboarding completion write failures remain visible and retryable.
- 2026-07-12 — Canonical Phase 0–7 usability review recorded; no remaining usability BLOCKER.
- 2026-07-12 — End-to-end code review through `4036e78` recorded two restart blockers, since fixed.
- 2026-07-12 — Untagged Archive search falls back to metadata `LIKE` when FTS5 is unavailable.
- 2026-07-11 — Configuration federation 7.5–7.7 shipped with resolver, manager, API, launch, and UI.
