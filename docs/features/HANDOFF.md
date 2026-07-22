# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Active change:** None.
- **State:** paused — no implementation change is active. A post-fix usability rerun found no open
  product finding and confirmed the cancelled-permission fix in the built app. Credentialed provider
  acceptance remains a separate manual release gate; native prompt/confirm actions also need replay
  in a browser that supports those dialogs.
- **Last reviewed code:** `ef4ee18` (2026-07-22), across the continuous range after `61b234d` (the
  current-history equivalent of the earlier pre-rehash marker `4195ed0`).
- **Branch:** `main`.

## Decisions needing your input

These are shipped boundaries documented in the specifications, not blockers. A future reversal needs
an explicit specification update; remove an item when the human accepts the current rule or queues
that update.

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

- **Worth fixing** — `scripts/release/install.sh`: the piped-install path (`curl | bash`) creates a
  temporary bootstrap with `mktemp`, registers `trap 'rm -f "$bootstrap"' EXIT`, then
  `exec bash "$bootstrap"`. `exec` replaces the shell image, so the EXIT trap never fires and the
  owner-only temp file is left in `$TMPDIR` after every piped install (the re-exec'd child is a fresh
  shell that neither knows the path nor carries the trap). Normal-use trigger: the documented
  `curl | bash` install, on every run. Not data loss and cleaned at reboot, so low severity. Suggested
  fix: have the re-exec'd bootstrap delete its own file once it no longer needs to re-read itself
  (e.g. pass the temp path in an env var and `rm` it after the lock re-exec resolves, or sweep stale
  `agentdeck-bootstrap.*` at start), guarded so a real `bash <install.sh>` invocation never deletes a
  user-provided script. No requirement pins temp-file cleanup; this is a hygiene defect, not a
  behavior-vs-spec mismatch.

## Recent changelog

_(Newest first; durable product truth is in FS/TS and history is in git.)_

- 2026-07-22 — Reviewed the continuous range after `61b234d` through `ef4ee18` (the current-history
  span since the last review; `61b234d` is the rehashed equivalent of the old `4195ed0` marker). The
  shipped product code — the permission busy-before-release race fix and cancelled-decision emission,
  the transcript-replay assistant-delta folding, the onboarding wizard latch, and the release-archive
  symlink dereference — matches its requirements in both directions (FS-03.R4/R9/A4/A5/A6, FS-04.R23,
  INV §9). The design/spec-only commits (annotate-and-assign, onboarding-credentials) carry consistent
  `(planned)` tags and ship no code. One Worth-fixing finding recorded: the piped installer leaks its
  temporary bootstrap file because `exec` discards the cleanup trap. Spec check, Go build, and the
  touched runtime/release/cli package tests pass.

- 2026-07-22 — The human accepted the current local-API trust and child-environment inheritance
  boundaries for now, and moved those plus the terminal-capability boundary to the known-issues
  backlog. Codex remains supported for chat; its terminal interface is intentionally rejected until
  a Codex-specific interactive-CLI hook/flag path is verified.

- 2026-07-22 — Defined the waiting onboarding-credentials change with the human. It adds an
  explicit Set up later completion path; removes onboarding model fields; gives Claude/Codex
  provider-owned sign-in guidance plus Check again; treats Codex native login or API key as ready;
  updates fresh-only defaults to `sonnet`/`gpt-5.6-sol`; preserves existing `backends.json`; and
  pins a private Codex CLI readiness probe. There is no embedded terminal, dashboard-started login,
  credential transport, or new auth API. The observed `agentdeck auth` failure is an installed
  v0.1.0 binary predating that command, not absent current source. FS-04, FS-09, TS-03, TS-04, and
  TS-06 are planned/Partial; `repair-onboarding-credentials.md` waits to start. Spec, twin-skill,
  and whitespace checks pass; no product code changed.

- 2026-07-21 — Published the verified piped-installer fix in GitHub Release v0.1.1. The tag's
  Apple-silicon release workflow completed successfully: it assembled the private runtime, passed
  the release transaction/bootstrap and fresh-install checks, and uploaded the archive,
  `manifest.json`, and corrected `install.sh`. The documented `releases/latest/download/install.sh`
  endpoint now serves the fixed bootstrap. The two pending `main` commits (the installer fix and
  the previously committed annotate-and-assign specification work) are pushed to `origin/main`.

- 2026-07-21 — Fixed the documented `curl | bash` release installer path. Its lock re-exec had
  treated `bash` as the script pathname, causing the lock-holding child to resume midway through
  the pipe with helpers such as `die` and `on_path` undefined. A piped invocation now first writes
  an owner-only executable temporary bootstrap, then safely re-execs that complete file under the
  lock. The new fake-release regression exercises the exact pipe → lock → install sequence;
  specification checks, the full Go test suite, source build, and distribution build pass. The
  v0.1.0 release asset remains unchanged until a new release is published.

- 2026-07-20 — Defined the annotate-and-assign feature with the human: new planned FS-13
  (diff-line and transcript-event selection in live and archived transcripts, a per-browser pending
  tray, batch send to the current agent, another chat agent, or a new prefilled launch, a durable
  structured `annotation` transcript event, archive search), planned FS-06 reserved user-sender
  mail (no turn-budget consumption, unforgeable), planned TS-02 annotation-event and user-mail
  persistence, and the planned TS-03 annotations batch endpoint. The human confirmed all four scope
  decisions (surfaces, batch tray, new-task-as-prefilled-launch, mail delivery) in conversation.
  The ready change `annotate-and-assign.md` is waiting to start; no product code changed.
  Specification, twin-skill, and whitespace checks pass.

- 2026-07-19 — Re-ran every runnable non-credentialed journey J1–J12 against the release-style
  build with isolated homes. The cancelled-permission prompt now resolves live and after reload;
  approve, deny, the real timeout, double-fire rejection, grid/restart, both archive search builds,
  Settings, messaging, recovery, and durability passed with no new finding. J6 and credentialed
  provider branches remain human-gated. The in-app browser cannot execute native prompt/confirm
  dialogs, so affected J5/J7/J9 UI actions are recorded as blocked while their backing operations
  and rendered results passed. Full report:
  [`../archive/reviews/usability-review-run-2026-07-19-post-fix.md`](../archive/reviews/usability-review-run-2026-07-19-post-fix.md).

- 2026-07-19 — Fixed the worth-fixing J4 finding: cancelling a turn with a pending permission now
  emits and persists a `permission_resolved` (decision `cancelled`), matching the deny and timeout
  paths. The withheld prompt renders a resolved chip on the live view and after reload instead of
  leaving Approve/Deny clickable forever (which returned `409 permission already resolved`). FS-03.R9
  and A5 pin the behavior; `TestCancelDuringPendingPermission` now asserts the emitted event.
  Specification checks, both Go test variants (incl. focused `-race` on the permission path), and the
  source build pass. No open review findings remain.

- 2026-07-19 — Drove the full non-credentialed usability matrix J1–J12 (J6 and the credentialed J2
  branch skipped as gated) with Playwright against the real binary, then re-verified J3/J4 on a
  rebuild at `c64d7bf`. Browser-level confirmation that the permission-deny race fix holds (3/3
  deny turns return to idle) and that reloaded transcripts coalesce streamed deltas like live chat.
  One new Worth-fixing finding recorded above (cancel-during-pending leaves a stale actionable
  permission prompt). All other journeys passed, including grid/layout persistence, resume/switch
  identity, both archive-search builds, settings round-trips, MCP messaging/nudge/unread, failure
  recovery, and restart durability. Full report:
  [`../archive/reviews/usability-review-run-2026-07-19.md`](../archive/reviews/usability-review-run-2026-07-19.md).

- 2026-07-18 — Fixed the permission-denial completion race: the runtime now records the temporary
  resolved/busy state before responding to ACP, so a fast peer can only write the final idle status
  through normal `turn_end` completion. The same ordering protects timeout resolution. A two-agent
  HTTP/SSE fake-ACP regression asserts idle after each denied `turn_end`; specification checks, both
  Go test variants, source build, and distribution build pass.

- 2026-07-18 — Re-ran the complete non-credentialed usability matrix after the onboarding and
  transcript-replay fixes. Both fixes now pass in the real built app: polling no longer ejects the
  wizard, and Archive/resume folds streamed replies exactly like live chat. Grid reorder/restart,
  tagged and fallback Archive search, Settings round-trips, two-agent MCP messaging and unread
  durability, fake live xterm input/resize/reattach, reconnect, crash recovery, and the presentation
  matrix passed. Found one new must-fix defect: a fast permission denial can race `turn_end` and
  overwrite idle back to busy, leaving the composer stuck on Cancel. Full report:
  [`../archive/reviews/usability-review-run-2026-07-18-post-fix.md`](../archive/reviews/usability-review-run-2026-07-18-post-fix.md).

- 2026-07-18 — Fixed both must-fix usability findings. An opened onboarding wizard is now latched
  until successful Launch completion, so the 10-second config refresh cannot replace Project or
  Config with the Dashboard after backend validation. Full transcript replay now uses the same
  consecutive-assistant folding helper as live Server-Sent Events, so Archive and resume keep one
  streamed reply in one message. FS-03, FS-04, and FS-05 now pin the behavior; focused gate, store,
  and Archive regressions pass along with the specification checks, 104 UI tests, both Go test
  variants, source/UI builds, and the distribution build.

- 2026-07-18 — Re-ran the behavior-driven usability review after an interrupted run left no durable
  checkpoint. The tagged production binary, untagged archive fallback, isolated fake-ACP homes, and
  development visual matrix covered first paint, onboarding, launch/chat, permission approve/deny,
  layout/restart, Archive/search/resume, dense Settings, disconnect/reconnect, and agent crash.
  Found two must-fix defects: config polling ejects a fresh user from the four-step wizard after
  Backend succeeds, and Archive/resume renders each stored assistant stream delta as a separate
  message. The redesigned presentation otherwise remained coherent and the presentation checks,
  101 UI tests, production UI build, spec check, and tagged/untagged Go builds passed. J6 live
  terminal and J10 messaging remain unexercised. Full report:
  [`../archive/reviews/usability-review-run-2026-07-18.md`](../archive/reviews/usability-review-run-2026-07-18.md).

- 2026-07-18 — Reviewed the continuous range after `87d6251` through `4195ed0`: the Codex
  role-prompt delivery fix, the installer/usability fixes, and the full core-interface redesign.
  The redesign is behavior-preserving presentation only — screens, data, routes, and actions are
  unchanged, the development-only visual matrix stays out of the production bundle, and third-party
  renderers read the shared semantic values. The two fixes match their specifications (Codex config
  overlay, corrupt-backend fallback, persisted/searchable user prompts, installer flag
  preservation). FS-12/TS-08 and the touched FS/TS agree with the code in both directions.
  Specification, presentation, UI (101 tests), and Go checks pass. No findings.

- 2026-07-18 — Shipped the product-native core interface across the shell, Dashboard, agent screen,
  Archive, Settings, onboarding, overlays, and third-party renderers. Layered semantic CSS, local
  fonts and mark, shared presentation primitives, stable future-skin hooks, a development-only
  visual matrix, Stylelint, and the TSX/CSS contract checker now form one maintained presentation
  authority. Real-browser review covered baseline/high-variance fixtures and the embedded release;
  it found and fixed hidden Settings panels consuming layout space. Specification, UI, both Go
  variants, source build, and distribution checks pass.

- 2026-07-18 — Finished the core frontend feature design after the human selected layered plain CSS.
  TS-08 now pins the cascade, exact core values/fonts/assets, stable manifest-backed skin hooks,
  third-party renderer adapters, migration sequencing, and unattended-maintenance safeguards:
  Stylelint, cross-TSX/CSS contract checks, stale-exception rejection, pretest/prebuild enforcement,
  deterministic visual fixtures, and local frontend agent instructions. The source idea moved to the
  ready change `redesign-core-interface.md`; specification and whitespace checks pass.

- 2026-07-18 — The human confirmed the product-native, presentation-only FS-12 behavior. Audited
  the current styling/build/component seams and added draft TS-08 with common constraints for local
  assets, third-party renderer theming, data-driven inline styles, shared presentation primitives,
  stable future-skin hooks, and visual/style-contract verification. Technical completion is paused
  on the A/B/C presentation-contract choice; specification checks pass.

- 2026-07-18 — Revised the planned frontend behavior after human feedback. FS-12 now makes the
  default presentation AgentDeck's product-native core, removes all Field Atlas metaphors, and
  excludes responsive/zoom/keyboard/accessibility expansion, new recovery states, and dedicated
  browser-dialog replacements. It retains a distinctive editorial/technical visual direction and a
  future-skin compatibility boundary. Specification checks pass; technical design still waits for
  confirmation of this revision.

- 2026-07-17 — Started the requested frontend redesign definition. Added planned FS-12 for a
  cross-product Field Atlas interface covering the shell, dashboard, agent workspace, archive,
  settings, onboarding, overlays, accessibility, responsive behavior, and the future-skin product
  boundary. Technical architecture and ready-change creation wait for human confirmation of the
  visual direction, desktop floor, and dedicated-dialog scope. Specification checks pass.

- 2026-07-17 — Audited every entry under `Known things to improve` against the current
  specifications, implementation, and focused tests. Removed fixed Codex-role, user-prompt, and
  installer claims; removed vague or unreachable subclaims; and narrowed partially fixed entries to
  their evidenced remainder. The installer lock re-exec preserves no-start/non-interactive flags and
  no longer blocks release; live-provider acceptance remains gated.

- 2026-07-16 — Codex chat now receives the frozen composed project/role prompt through the
  official `codex-acp` `CODEX_CONFIG.developer_instructions` overlay on launch and resume; invalid
  overlays fail before spawn, unrelated config remains intact, and Codex no longer receives the
  unsupported generic ACP `systemPrompt`. Runtime regression tests plus `make check-specs`,
  `make test`, and `make build` pass. A real authenticated Codex role-adherence new-turn/resume
  check remains an explicit acceptance gate.

- 2026-07-16 — Fixed all recorded installer and usability findings: the locked bootstrap preserves
  no-start/non-interactive choices under a pseudo-terminal test; incomplete hand-edited backend
  catalogs fall back safely with the filename in diagnostics and the UI guards null collections;
  accepted user prompts are sequenced, persisted, replayed, and indexed; onboarding names useful
  credential recovery steps; and the config-source panel has its missing styles. Specifications,
  focused Go/UI tests, `make test`, `make build`, and `make dist` pass.

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
