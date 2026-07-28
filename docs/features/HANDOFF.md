# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Active change:** None.
- **State:** AgentDeck-launched Codex agents now run in a private `CODEX_HOME`
  (`<agentdeck-home>/codex`, `0700`), composed as the final, reserved child-env layer in
  launch/resume/switch via `codexHomeEnv`/`composeEnv`, so their rollouts and native session index
  never enter the user's personal `codex` resume picker or app history. Before every codex-acp child
  starts, a serialized `config.WithRefreshedCodexProfile` critical section one-way mirrors the user's
  effective `${CODEX_HOME:-~/.codex}` setup (top-level config/auth/setup entries, session and
  history entries excluded) into owner-only copies, tracked by a `cache/codex-profile.json`
  managed-path manifest that prunes copies of removed personal setup while never touching the
  child's own session data, creating a source symlink, or following one out of the personal root; an
  unsafe or uncopyable selected asset fails the start before spawn. AgentDeck's own process
  `CODEX_HOME` is untouched, so federation discovery and Codex model autosync still read the real
  home. FS-08.R32/A9, FS-09.R43/R44/A17, TS-02.R19, and TS-04.R20/R21 are shipped and no longer
  `(planned)`. The packaged `codex-acp` actually honoring a non-default `CODEX_HOME`, recognizing
  the refreshed setup, and native-resuming a new isolated session remains a credentialed live-CLI
  gate (FS-09.A7 / TS-04.R21). Pre-existing AgentDeck Codex sessions in the personal home are not
  migrated and may no longer native-resume, as accepted by the human. Chat and archived transcripts no longer carry a standing per-event **Annotate**
  button; annotating is now a right-click action on the event, capturing the highlighted text when
  a highlight sits inside that event and the whole event otherwise (FS-13.R1/R19/A11). The diff
  block's line-number range selection is unchanged. No live-browser pass was run for this change;
  component coverage stands in for it. The AgentDecker pipeline builder now picks its own project. It launched into
  `default_project` with no picker, so the seeded `my-app` (whose `~/Projects/my-app` cwd is absent
  on a fresh box) could only be discovered as a rejected launch, with nothing on the Pipelines page
  able to change it; its readiness check also only asked whether a default was configured, not
  whether that default still resolved. Both regressions were verified to fail against the old code,
  and a real-browser check confirmed the picker renders and that a real project clears the
  directory rejection while `my-app` still reports it. Stopped Dashboard cards now expose the existing resume operation through their right-click
  menu; running cards do not offer it, and a rejected resume surfaces an error toast. Dashboard state badges now visibly render their text labels; a broad selector had painted
  every nested span, including the label, as a solid state-coloured block. Component coverage and the
  browser visual matrix now exercise every state. Dashboard cards otherwise show configured project titles with a durable-id fallback, and every
  context meter visibly labels its percentage, including zero. Direct human inspection found both
  prior presentations ambiguous; focused regressions, the visual matrix, and J5 now cover the
  observations. The prior seven findings from the review of `ccc2b50`→`cc9d498` are fixed and the queue is
  empty. Chat Resume now carries its launch generation, so a resumed crash tears down ownership,
  registration, and pauses its pipeline stage; the missing-agent annotation recovery waits for
  hydration; pipeline attempt and builder links classify by `running`; the archived header reads the
  newest session metadata; Archive paging de-duplicates and recovers rows that move between page
  requests; and both onboarding steps render field-level validation detail. Each fix landed with a
  regression that was verified to fail against the old code. FS-05.R28/A12 and FS-13.R16/A8 gained
  the new boundaries; the other five restore existing requirements. Specification checks, both Go
  test variants, all 149 UI tests, presentation/source/UI builds, the distribution build, focused
  `-race` on the resume crash path, and whitespace checks pass. The post-fix browser rerun passed
  J1–J4 and the exercised J5 layout paths, but J5 restart/delete and J6–J14 remain unconfirmed, so
  none of these fixes has been exercised end-to-end in a real browser. A follow-up retry showed
  that native confirmation now completes, but prior tabs contaminated the reused J5 fixture and
  the in-app browser then dropped its tab after each successful transition; J7's stopped-agent,
  transcript, and Archive-list surfaces rendered before that new block. The exact coverage is in
  [`../archive/reviews/usability-review-run-2026-07-26-rerun.md`](../archive/reviews/usability-review-run-2026-07-26-rerun.md)
  and [`../archive/reviews/usability-review-run-2026-07-26-browser-retry.md`](../archive/reviews/usability-review-run-2026-07-26-browser-retry.md).
  Credentialed provider and terminal compatibility remain separate manual release gates.
- **Last reviewed code:** `547ca43` (2026-07-28), the continuous range after `a574076`.
- **Branch:** `main`.

## Active change

None. Agents do not choose future work for themselves.

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

The post-fix usability-review and current code-review state are committed locally on `main`; pushing
those commits to the shared `origin/main` branch needs explicit human authorization.

## Review findings

- **Worth fixing** — (no invariant class) — SDD traceability for `0b6e793`, `ceef7ee`, `9f0dbde`,
  and `547ca43`: these commits ship Dashboard Resume, builder-project selection, transcript
  right-click behavior, and Codex profile isolation without a committed ready-change/active-change
  plan. For `547ca43`, the ideas-only parent is followed by one commit that adds and immediately
  ships all requirements and code while both handoffs say `Active change: None`; its brief names a
  ready change that never exists in the tree. The required spec-first planned trail cannot be
  audited. Use a committed ready change and active handoff plan before the next behavior change; do
  not create a retroactive waiting change for work already shipped.

The one-off Archive `unterminated string` 500 still did not reproduce under direct or suite coverage,
and the API-only `tmux` calls without explicit timeouts remain an unreproduced source-risk lead; they
are not promoted to findings without a repeatable failure.

## Recent changelog

_(Newest first; durable product truth is in FS/TS and history is in git.)_

- 2026-07-28 — Fixed the five Codex-isolation findings and both scoped follow-ups. The private
  setup mirror now selects an explicit setup allowlist, preserves executable setup assets as
  owner-only, rejects unsafe profile destinations/source overlap before mutation, and stages a full
  generation with its manifest before transactional publication; manifest failure restores the old
  profile. Refresh stays locked through child process creation, and unsafe Codex switches are
  rejected before stopping a live agent while failed rollback registrations are torn down. The
  environment-composition matrix now covers the shared launch/resume/switch/rollback composer and
  the stale federation qualifier is removed. Regression coverage includes runtime-state exclusion,
  executable mode, source/destination safety, manifest rollback, concurrent-start barrier, and
  unsafe-switch preservation. `make check-specs`, both Go test variants, `make build`, `make dist`,
  and whitespace checks pass.

- 2026-07-28 — Reviewed the continuous range after `a574076` through `547ca43`, including the
  previously reviewed AgentDecker catalog-membership fix and the new Codex session-isolation feature.
  Five Must-fix and three Worth-fixing findings are recorded above. **INV §12/§10** caught the
  open-ended setup mirror copying the pinned/current provider's real state and stripping executable
  setup modes; **INV §5/§15** caught the partial-generation refresh/start window; **INV §14**
  caught the destination-symlink escape; and **INV §4/§15** caught the failed-switch rollback
  leak. **INV §2** confirmed one final child-env helper is correctly shared by launch, resume, and
  switch but found the promised path matrix unpinned; **INV §10** also found one stale "planned"
  spec qualifier. Clean/not applicable on the remaining classes: §1 is covered by the refresh
  boundary finding; §3 adds no persisted form/one-shot field; §6 adds no runtime/interface; §7
  surfaces changed read failures; §8 uses existing bounded start errors; §9 adds no PID/protocol/
  SQLite migration and its durability concern is covered by the staged-generation finding; §11
  adds no API collection and writes a non-null manifest list; §13 adds no Codex UI/CSS selector;
  the remaining §14/§15 surfaces are owner-only and pre-spawn. Specification checks, both Go test
  variants, focused config/runtime/server race tests, all 163 UI tests, presentation/source/UI
  builds, the distribution build, and whitespace checks pass. The credentialed packaged-Codex gate
  remains unrun and correctly gated. No product code or specifications changed during review.

- 2026-07-28 — Shipped Codex session isolation (FS-08.R32/A9, FS-09.R43/R44/A17, TS-02.R19,
  TS-04.R20/R21). **INV §2** is load-bearing: one `codexHomeEnv` layer, appended last to the shared
  `composeEnv` in launch/resume/switch, points every codex-acp child at `<home>/codex` and overrides
  any ambient/backend/model `CODEX_HOME`; `spawnCmd` reads that composed value back as the single
  source of truth and refreshes the profile before spawn, so a resumed session never opens a
  different store. The new `internal/config/codexprofile.go` provisions the `0700` profile and, under
  a package mutex, one-way mirrors the personal `${CODEX_HOME:-~/.codex}` top-level setup into
  owner-only files while excluding session/history entries, dereferencing in-root symlinks, failing
  on a root-escaping one, and pruning managed copies of removed personal setup via
  `cache/codex-profile.json` without touching the child's own session data. **INV §12** shapes the
  gate: the packaged `codex-acp` honoring a non-default `CODEX_HOME` and the refreshed setup stays a
  credentialed live-CLI acceptance, and fake-ACP tests assert only the composed env and refresh
  contract. Config profile-refresh, env-composition, and a codex spawn-refresh test were added; both
  Go test variants, `make build`, `make check-specs`, focused `-race` on the config/runtime refresh,
  and whitespace checks pass. No UI changed. AgentDeck's own process `CODEX_HOME` is untouched, so
  federation discovery and model autosync still read the real home.

- 2026-07-28 — Refined the planned Codex session isolation with the human; no product code changed.
  `CODEX_HOME=<agentdeck-home>/codex` remains the always-on, new-launch-only boundary, but the
  private profile now refreshes personal Codex setup before every launch/resume/switch rather than
  symlinking only auth/config: auth, configuration, skills, agents, rules, plugins, and MCP setup
  are copied one way; session/history data is excluded; source additions/edits/removals apply at the
  next child start; and AgentDeck never writes to the personal home. This keeps AgentDeck Codex
  capable with the user's normal setup while its sessions stay private. Pre-existing AgentDeck Codex
  sessions may no longer native-resume, as accepted by the human. FS-08 gained R32/A9; FS-09 R43/R44
  and A17, TS-02 R19, TS-04 R20/R21, and the ready change now define the profile-refresh boundary,
  final `CODEX_HOME` precedence, managed-path cleanup, source-link safety, and the exact packaged
  live-CLI gate. `make check-specs` and whitespace checks pass; no product code or tests changed.

- 2026-07-27 — Fixed the AgentDecker builder's stale-project launch. **INV §10** found the wiring
  incomplete: readiness treated any non-empty `project` state as launchable, so a project deleted
  from Settings or another tab after the builder opened left the controlled select showing no
  selection while the launch still posted the removed id and was rejected. Readiness now requires the
  selection to be a current catalog member (`projects.data?.[project]`), and a selection that leaves
  the catalog is cleared so the seed effect and launch gate re-evaluate against the live catalog.
  This restores FS-14.R26's "launch limited to a listed project" boundary, so no specification change
  was needed. The catalog-refresh regression was verified to fail against the old code; specification
  checks, both Go test variants, all 163 UI tests, the UI/distribution builds, and whitespace checks
  pass.

- 2026-07-27 — Reviewed the continuous 12-hour range from `cc9d498` through `a574076`, including
  all committed UI/runtime/test/specification changes and the clean working tree. Code and
  specifications agree on the repaired review batch, Dashboard presentation/Resume, builder picker,
  annotation context menu, and the planned effort-selection change; every invariant class was
  swept. One Must-fix remains: a selected project that disappears after the builder opens can still
  be launched because readiness checks only for a non-empty id rather than a current catalog member
  (FS-14.R26). One Worth-fixing process finding remains: the three new behavior changes were shipped
  without the required ready-change/active-plan trail, so spec-first sequencing is not auditable.
  `make check-specs`, both Go test variants, the full 162-test UI suite, the source build, the
  release build, and whitespace checks pass. No product code or specifications changed during this
  review.

- 2026-07-27 — Designed launch-time effort selection with the human; no product code changed.
  Verified both pinned adapters rather than assuming: `codex-acp` 1.1.2 encodes effort **inside** the
  ACP model id as `model[effort]` (so no new protocol field is needed), and its per-model levels are
  already published in the `models_cache.json` that FS-09.R28 autosync reads; `claude-agent-acp`
  0.62.0 accepts effort only as a post-`session/new` configuration option seeded from the native
  settings chain AgentDeck must not write; and Claude Code 2.1.202 takes `--effort` for terminal
  launches. FS-09.R35–R42/A14–A16 now specify optional per-model `efforts`/`default_effort`,
  rejection of bracketed provider model strings, add-only autosync of levels, the claude/codex-only
  capability boundary, the Claude post-session teardown, precedence, and undeclared-level rejection.
  FS-01.R30/A14 add the launch field, `--effort`, and effort's place in runtime identity; FS-14.R31/A11
  add per-stage run assignment; FS-08.R31/A8 make the long-inert effort override real. TS-01.R12 pins
  one `resolveEffort` on the shared composition seam, TS-02.R18 keeps `backends.json` at version 2 and
  adds a forward-only `sessions.effort` column, TS-03.R19 adds optional fields to existing routes only,
  TS-04.R18–R19 pin the three adapter-declared delivery mechanisms and fail-closed rejection, TS-07.R14
  feeds the federation override into the resolver, and TS-09.R24 snapshots per-stage effort.
  **INV §2** is load-bearing (the Codex suffix must be composed once for both `sessionNewParams` and
  `sessionLoadParams`), with **INV §4** on the pre-registration teardown, **INV §1** on re-applying
  effort at resume, and **INV §12** deliberately departed from (fail closed, never retry bare).
  Recorded FS-08's pre-existing spec/code mismatch as a deviation rather than silently fixing it.
  FS-01, FS-14, TS-02, TS-03, and TS-09 move to Partial while their planned items are unshipped. The
  source idea was promoted to the waiting `agent-effort-selection.md` change; specification,
  twin-skill, and whitespace checks pass.

- 2026-07-27 — Gave the AgentDecker builder its own project selector. **INV §10** found the
  shipped wiring incomplete: the builder launches an ordinary chat agent, which needs a real project
  directory, but exposed only backend and model and sent `default_project` unconditionally — so the
  seeded `my-app` produced a launch the page could not repair. **INV §2** supplied the fix shape: the
  builder now uses the same existence-checked default seed and project `<select>` as `RunStartForm`,
  rather than a second, weaker way of choosing the same thing. Readiness follows the selected
  project, so a default naming a removed project holds the launch closed instead of enabling a button
  whose only outcome is a rejected launch. FS-14.R26/A10 gained the project choice and that
  boundary. Both regressions were verified to fail against the old code; all 158 UI tests, both Go
  test variants, specification checks, the UI/source/distribution builds, and whitespace checks pass.
  A real-browser check on an isolated home confirmed the picker renders and behaves in both
  directions with no console error.

- 2026-07-26 — Added direct Dashboard Resume. **INV §10** found no new invariant boundary: the
  existing resume operation already restores the frozen session snapshot, but was unnecessarily
  reachable only through Archive. A stopped card's right-click menu now offers **Resume**, while a
  running card omits it; failures use the established error toast. FS-02.R27–R28/A14 pin menu
  visibility, the endpoint, and retryable failure behavior. Focused menu regressions, all 156 UI
  tests, both Go test variants, specification checks, and the distribution build pass.

- 2026-07-26 — Restored visible Dashboard state-badge labels. **INV §10** found a presentation
  selector that matched every `span` inside a badge, converting its label into a state-coloured
  rectangle. The selector now targets only `.ad-badge-indicator`, while the base badge variant
  supplies its state-coloured text. FS-02.R3's existing vocabulary is restored; all six labels,
  the visual matrix, the full 153-test UI suite, both Go test variants, and the release build pass.
  A browser check confirms Busy, Idle, Waiting, Done, Error, and Unknown render as readable labels.

- 2026-07-26 — Clarified Dashboard card identity and context. **INV §10** found no additional
  invariant breach: the problem was an explicit but misleading presentation rule. FS-02.R25 now
  resolves the subtitle's durable project id through the current project configuration and falls
  back safely if that definition is absent; FS-02.R26 replaces the blank zero-context track with a
  visible `0% context used` label. Card/grid/context regressions cover title, fallback, and zero;
  the visual matrix renders the zero state and human-readable project label; and J5 now requires
  those observations. Specification checks, both Go test variants, all 152 UI tests, source/UI and
  distribution builds, and a real-browser visual-matrix check pass.

- 2026-07-26 — Retried the blocked browser validation without changing product code or specifications.
  **No invariant class** applied because no completed browser path produced a finding. Native confirmation
  completed, unlike the earlier stalled run, but that listener belonged to an older review home and was
  excluded as evidence. A new isolated J7 listener rendered the stopped-agent dashboard, transcript,
  and Archive list, after which the in-app browser repeatedly lost its tab after page transitions.
  J5 restart/delete and J6–J14 remain unconfirmed, not passed. `make check-specs` and whitespace
  checks pass; the full result is
  [`../archive/reviews/usability-review-run-2026-07-26-browser-retry.md`](../archive/reviews/usability-review-run-2026-07-26-browser-retry.md).

- 2026-07-26 — Fixed all seven open review findings. **INV §4** now copies the launch generation
  onto a resumed chat runtime, so a resumed or switch-resumed crash drops registry ownership, tears
  down hook/MCP/settings registration, allows an immediate re-resume, and pauses its pipeline stage
  as `agent_crash`; the resume→crash regression asserts the delivered generation and a matching
  pipeline test pins that an empty generation is ignored. **INV §1** makes the missing-source
  annotation recovery wait for completed agent hydration, so a deep link or reload shows a loading
  state instead of offering to discard live drafts, and classifies pipeline attempt transcripts and
  the persisted AgentDecker builder by `running` rather than identity presence, so finished attempts
  reach Archive and a stopped builder's browser key expires. **INV §1** also selects the newest
  session metadata for the archived header, so a switched session shows the identity Resume will
  restore. **INV §10** makes Archive paging render every matching session exactly once: a repeated
  row proves the `updated_at` ordering shifted, so the first page is refetched and whatever moved
  into it is recovered. **INV §8** routes both onboarding steps through the shared config error
  parser, so a repairable 400 names its field and reason instead of `Error: HTTP 400`. FS-13.R16/A8
  and FS-05.R28/A12 gained the new hydration and reachability boundaries; the other five fixes
  restore existing requirements. Every regression was verified to fail against the old code.
  Specification checks, both Go variants, 149 UI tests, presentation/source/UI builds, the
  distribution build, focused `-race` on the resume crash path, and whitespace checks pass.

- 2026-07-26 — Reviewed the continuous range after `ccc2b50` through `cc9d498` in both
  specification directions and against every invariant class. Two Must-fix and five Worth-fixing
  findings are recorded above. **INV §1/§2/§4** caught the missing generation on chat Resume and the
  pre-hydration destructive annotation recovery; **INV §1/§10** caught identity presence being used
  as liveness in two pipeline links, stale archived session metadata, and mutable offset paging;
  **INV §8** promoted the two carried onboarding `HTTP 400` leads after confirming the shared parser
  is bypassed. Clean on the remaining changed surfaces: §3 seeded forms remain merge-preserving; §5
  onboarding/index/pipeline claims are serialized; §6 added no new runtime or driver and its changed
  lifecycle contract is covered by the Resume finding; §7 readers check their real error signals;
  §9 FTS downgrade and cancellation cause handling are bounded; §11 collections and client schemas
  stay non-null/in lockstep; §12 the Claude optional-flag retry is fixed-command and fail-closed;
  §13 new selectors and presentation states resolve; §14 adds no route and existing routes remain
  under `localOnly`; §15 annotation/index/notification side effects follow their durable writes.
  The one-off Archive 500 and untimed tmux calls remain unconfirmed, not findings. Specification
  checks, both Go variants, 139 UI tests, source/UI/presentation builds, the distribution build, and
  whitespace checks pass. No product code or specifications were changed.

- 2026-07-26 — Confirmed the `/fix` queue is empty. **No invariant class** applied: every
  recorded Must-fix and Worth-fixing item is already resolved, no active change is selected, and
  the worktree is clean. `make check-specs` passes; no product or specification change was needed.

- 2026-07-26 — Re-ran the post-fix usability matrix at `c59dd2c`. **No invariant class** applied
  because the completed browser paths produced no finding. J1–J4 and the exercised J5 layout paths
  passed in the release build with new isolated homes and zero unexpected console errors; the
  onboarding/provider fallback and all four permission labels were confirmed live and after reload.
  The in-app browser stalled on J5's native confirm, then the execution account refused further
  loopback servers after reaching its usage limit. J5 restart/delete and J6–J14 are recorded blocked,
  not passed. Static serialization/CSS/null sweeps were clean; external-CLI and mutation-error hits
  remain source leads only. All 139 UI tests, presentation checks/build, specification checks, and
  focused tagged/untagged recent-fix regressions pass. Full report:
  [`../archive/reviews/usability-review-run-2026-07-26-rerun.md`](../archive/reviews/usability-review-run-2026-07-26-rerun.md).

- 2026-07-26 — Distinguished permission outcomes. **No invariant class** applied; FS-03.R4/R17/A6
  now retain the runtime's approved, denied, cancelled, and timed-out outcomes through both live and
  replay transcript projection. Permission chips render **Approved**, **Denied**, **Cancelled**, or
  **Timed out** with declared presentation states. Store and renderer regressions cover all four;
  unknown legacy decisions remain conservatively denied.

- 2026-07-26 — Identified archived transcripts before Resume. **No invariant class** applied;
  FS-05.R31/A15 now require the read-only header to show recorded name, project, backend/model, and
  creation date. The view requests the transcript's existing session metadata instead of relying on
  live-agent state, retains a loading fallback, and renders the identity alongside its archived
  state. A UI regression verifies the metadata query and every header field.

- 2026-07-26 — Identified cancellation-escalation exits. **INV §8** now marks the exact turn when a
  peer ignores cooperative cancellation and receives fallback SIGINT; if that signal ends the
  process, the durable fatal error, error status, and turn-end reason identify Cancel as the cause
  while ordinary crash diagnostics remain unchanged. FS-01.R7 and FS-03.R9/A5 pin the outcome. The
  hung-peer regression now verifies running-row removal, status detail, transcript error, and reason.

- 2026-07-26 — Rejected blank project identity fields. **INV §8** now treats whitespace-only title
  and cwd values as missing in the shared project validator used by both create and update, so such
  a project cannot enter the New Agent target list. A field-specific config regression requires
  both diagnostics. No FS/TS delta was needed because the fix restores FS-04.R7.

- 2026-07-26 — Unified agent display-name validation. **INV §8** now routes launch, rename, and
  identity updates through one trim/nonblank/NUL-free/256-character boundary; omitted launch names
  still receive a curated suggestion, while explicit whitespace can no longer create a titleless
  card. FS-01.R4/R8/R22/A1/A6 pin the contract. Focused regressions cover the Unicode limit and
  launch rejection for whitespace, NUL, and overlong input.

- 2026-07-26 — Aligned live and replayed permission resolution. **INV §2** now makes the live
  resolver stop at the newest matching permission request, exactly like the durable transcript
  fold, so a reused tool-call id cannot rewrite earlier chips. A store regression covers both an
  incoming live resolution and the optimistic user-action path. No FS/TS delta was needed because
  the fix restores FS-03.R4/R12/A6.

- 2026-07-26 — Added stale annotation-tray recovery. **INV §1** now gives a missing-agent route a
  direct discard action with its retained draft count, and SSE reconnect contains the corresponding
  missing-transcript rejection instead of surfacing an uncaught error. FS-13.R16/A8 pin the
  recovery boundary; route and reconnect regressions cover it without treating archived sources
  as deleted.

- 2026-07-26 — Kept annotation delivery controls visible. **No invariant class** applied;
  FS-13.R18/A10 now split the bounded pending tray into an independently scrolling draft body and
  a fixed delivery footer, so detailed drafts cannot push target selection, errors, or Send below
  the visible edge. A three-draft UI regression verifies the body/footer boundary.

- 2026-07-26 — Made diff-line selection discoverable. **No invariant class** applied;
  FS-13.R17/A9 now require a visible instruction and pointer cursor on selectable line numbers.
  `DiffBlock` explains that line-number clicks select a range, the renderer theme supplies the
  pointer affordance, and the existing library-shaped selection regression now checks the helper.

- 2026-07-26 — Made Archive search capability honest. **INV §8** now reports
  `search_mode` on every Archive response from one shared SQLite capability detector; the metadata-
  only path removes transcript from the placeholder and explains that transcript text/snippets are
  unavailable, while full-text mode retains the one-turn guidance. FS-05.R30/A14, TS-02.R10, and
  TS-03.R18 pin the API/UI contract. Tagged, untagged, server, and UI regressions cover both modes.

- 2026-07-26 — Explained Archive search document boundaries. **INV §8** now gives
  the search input an accessible scope note stating that all terms must match one metadata record,
  transcript turn, or annotation and are not combined across turns, so an empty result no longer
  implies absence from the whole session. FS-05.R29/A13 specify the affordance and its UI regression.

- 2026-07-26 — Kept lifecycle usable across an FTS5 capability downgrade. **INV §8**
  now probes an existing virtual search table before each derived-document boundary; when the
  SQLite driver reports the module unavailable, AgentDeck skips only the FTS document and still
  commits authoritative session metadata, counters, and turn rollups. FS-01.R29/A13 and TS-02.R10
  now pin that degradation and its bounded error surface. Focused tests simulate the unavailable
  writer in both SQLite build variants.

- 2026-07-26 — Added Archive paging. **INV §10** now exposes **Load more** whenever
  the reported match total exceeds the rendered rows, requests the next offset, appends the page,
  and preserves the first page if a later request fails. FS-05.R28/A12 now specify the UI contract,
  and TS-03 no longer records pagination as incomplete. A two-page UI regression verifies offset
  progression and complete reachability.

- 2026-07-26 — Fixed run-start validation feedback. **INV §8** now routes rejected
  start errors through the shared pipeline diagnostic parser and renders each field and repairable
  message alongside the summary, matching the template editor. A UI regression submits a valid
  form, injects an assignment diagnostic, and verifies the field and message remain visible. No
  FS/TS delta was needed because the fix restores TS-09.R3/R23 and FS-14.R18.

- 2026-07-26 — Fixed Codex pipeline assignments. **INV §2** no longer applies the
  role/project filename slug rule to backend and model catalog keys before the shared lifecycle
  validator checks the configured catalog, so seeded `gpt-5.6-sol` assignments now start and reach
  the ordinary launch path. A manager regression starts a run with that exact model id. No FS/TS
  delta was needed because the fix restores FS-14.R2/A1 and FS-09.R33.

- 2026-07-26 — Fixed diff-line annotation selection. **INV §10** now parses the
  `react-diff-viewer-continued` line-id contract (`L-<line>` / `R-<line>`) instead of a format the
  dependency never emits, restoring FS-13.R1/R2 in live and archived transcripts. A component
  regression drives a library-shaped `L-2` id through selection and verifies the captured anchor
  and excerpt. No FS/TS delta was needed because the fix restores existing requirements.

- 2026-07-26 — Drove the week's shipped work through the real rendered UI: J14 pipelines, J13
  annotations, J8 archive/search on both build variants, J1/J3/J4/J11 core regressions, and J12
  restart durability, against isolated homes and the fake ACP peer. Stage results were reported
  through the local MCP endpoint with each stage agent's own minted token while its turn was held
  open, so runs advanced through genuine turn quiescence. Sixteen findings are recorded above, six
  Must fix. The two that matter most are unreachable shipped capabilities rather than regressions:
  **INV §10** — `DiffBlock` parses a line id format the diff library does not emit, so FS-13's
  diff-line annotation has never worked in a browser and no test covers that third-party seam;
  **INV §2** — pipeline stage assignment validates model ids with the role/project filename slug
  rule, which forbids dots, so both seeded Codex models are rejected and FS-14.R2's own
  Codex-plus-Claude example is impossible. **INV §10** also caught the archive rendering only its
  first 50 sessions while reporting the true total, and **INV §8** caught three dead ends: dropped
  run-start diagnostics, a raw `no such module: fts5` blocking every launch on a downgraded home,
  and cross-turn searches answering "No results" with no signal. Clean on the rest: the full
  pipeline lifecycle including approval gates, repair loops, revision compare-and-swap, stop/
  blocked/retry recovery, shared-workspace confirmation, restart recovery and run deletion; the
  annotation tray, its three delivery routes and archive searchability; both archive build variants;
  a styled empty home with zero console errors; delta coalescing; all four permission outcomes
  including cancel-while-pending; and restart durability across agents, layout, archive and paused
  runs. Static sweeps were clean on serialization, CSS wiring and null hostility. The full report is
  [`../archive/reviews/usability-review-run-2026-07-26-week.md`](../archive/reviews/usability-review-run-2026-07-26-week.md).
  No product code or specifications were changed; credentialed providers remain gated.

- 2026-07-26 — Fixed the J2 Claude readiness blocker. **INV §12** now recognizes common
  `unknown`/`unrecognized`/`unsupported`/`invalid` option or flag diagnostics, Go's undefined-flag
  wording, and unexpected-argument wording only when the output also names `-no-color`; it then
  retries the same fixed status argv without the optional flag. Unrelated status/auth failures do
  not enter the fallback, and raw output remains behind the bounded result vocabulary. No FS/TS
  delta was needed because the fix restores FS-04.R17/R34/A14, FS-09.A5, and TS-04.R15. The focused
  regression, specification checks, both Go variants, source/presentation/UI builds, the
  distribution build, and whitespace checks pass. Credentialed providers remain gated.

- 2026-07-26 — Replayed J2 onboarding through the real rendered UI against two fresh isolated homes.
  Set up later closes the modal by pointer click, persists completion, leaves the seeded backend
  catalog/defaults and project set unchanged, and launches no session. Missing-adapter, signed-out,
  and ready Check again transitions work; the ordinary project, optional Config, first fake-ACP
  launch, completion write, and restart reread also pass with no browser console error. **INV §12**
  found one reproducible blocker: a ready Claude adapter that rejects optional `--no-color` as
  `unknown flag` rather than the one hard-coded `unknown option` phrase is wrongly reported failed,
  so the wizard cannot advance. The full report is
  [`../archive/reviews/usability-review-run-2026-07-26-j2.md`](../archive/reviews/usability-review-run-2026-07-26-j2.md).
  No product code or specifications were changed; credentialed providers remain gated.

- 2026-07-26 — Fixed all seven open review findings. **INV §4** now carries the concrete launch
  generation through every runtime exit and tells pipeline-owned stops from ordinary user stops, so
  stopping a stage card pauses the run as `agent_stopped` with Retry available instead of wedging
  `await_result`. **INV §8** moved the immutable stage-reporting protocol and every declared output
  name outside the variable prompt budget, and pipeline notifications now use the run display name
  plus final outcome. **INV §10** routes stopped attempt transcripts to Archive; **INV §1** drops a
  persisted AgentDecker builder id after live-agent hydration proves it stale. **INV §5** gives Set
  up later, backend validation, project creation, source actions, and first launch/completion one
  synchronous UI mutation claim. **INV §2/§5** makes turn-end and annotation consumption plus flush
  one sequence-bounded index operation; later-sequence text remains buffered for the next immutable
  document. No FS/TS delta was needed because each fix restores existing requirements. Regression
  coverage includes the ordinary stage stop endpoint, a maximum-size Unicode input, route/lifecycle
  presentation, notification payloads, four deferred onboarding mutations, and deterministic index
  interleavings. Specification checks, both Go variants, 126 UI tests, source/UI builds, focused race
  tests, the distribution build, and whitespace checks pass.

- 2026-07-26 — Reviewed the continuous range after `eb63dd5` through `ccc2b50` — the six-finding fix
  batch and the whole configurable-pipeline-runs implementation — in both specification directions
  and swept the diff against every invariant class. FS-14.R1–R30 and TS-09.R1–R23 match the code:
  templates are filename-addressed version-1 JSON behind one canonical validator, runs are frozen
  snapshots advanced only by an explicit authenticated result plus a persisted turn boundary, every
  transition is a revision compare-and-swap, and the shipped surfaces (REST, SSE, MCP tools, CLI,
  Pipelines page) all exist. Three must-fix and two worth-fixing findings are recorded above.
  **INV §4** caught that a solicited stop drops the launch generation before the exit callback, so an
  ordinary Stop on a stage agent wedges its run with no valid recovery action. **INV §8** caught the
  assignment renderer clipping its own mandatory reporting instruction when a named value approaches
  the value bound, and the notification builder naming runs by opaque id with no terminal outcome.
  **INV §10** caught attempt-history transcripts linking to the live agent route rather than the
  archive. **INV §1** caught the builder agent id persisted in `localStorage` with no boundary
  cleanup. Clean on the remaining classes: §2 manual and pipeline launch/resume/stop share
  `launchAgent`/`composeResumeSpecWithGeneration` and one validator; §3 run snapshots are frozen and
  template PUT is a deliberate whole-document replace; §5 every mutation is a revision CAS and the
  report/quiescence writes are single row-count-checked transactions; §6 stage agents are ordinary
  chat agents and the `Lifecycle` seam carries the checklist; §7 all four new list queries check
  `rows.Err()` and malformed runs/templates are isolated per entity; §9 launch/continue/stop
  corroborate ownership through `Owns` plus PID-checked reaping and startup never invents a result;
  §11 `NormalizeTemplate`, `nonEmptyJSON`, and literal-initialised slices keep every collection
  non-null; §12 the range adds no external CLI invocation and tightens two; §13 all 62 static plus 6
  dynamic Pipelines class names resolve; §14 all 13 new routes register inside the `localOnly` mux
  and template files keep `ValidSlug` on read/write/delete; §15 report, values, and action intent all
  commit before the tool returns or a process starts. Specification checks, both Go test variants,
  120 UI tests, and source/UI builds pass; the open paths need stop-injection, prompt-bound, and
  archive-link coverage. No product code or specifications were changed.

- 2026-07-25 — Shipped configurable pipeline runs (FS-14, TS-09). Added reusable model-neutral
  version-1 JSON templates with bounded repairable validation; forward-only SQLite run, attempt,
  value, and idempotency state; a compare-and-swap sequential reconciler driven by explicit scoped
  stage results and persisted turn quiescence; shared manual/pipeline launch, resume, stop, teardown,
  and generation ownership; restart/crash/blocked/approval/retry/loop/stop recovery; exact
  AgentDecker template/run proposals; and local REST, revisioned SSE, notifications, and thin CLI
  controls. Added the Pipelines page with structured editing, run setup/supervision/history,
  advisory shared-workspace confirmation, transcript/agent links, notification settings, and
  ordinary-agent pipeline associations. Malformed run detail is isolated in summaries/startup,
  collection shapes stay non-null, and all routes inherit Host/Origin enforcement. FS-14/TS-09 and
  their TS-01–TS-05/TS-08/FS-12 deltas are Current; J14 now owns the composed usability charter.
  Specification checks, both Go variants, 120 UI tests, presentation/UI builds, focused race tests,
  distribution build, whitespace checks, and an isolated real-browser Pipelines pass succeed. The
  browser pass caught and fixed form overflow and empty-run layout defects. Credentialed providers
  remain the existing manual gate, and the two unrelated review findings remain open.

- 2026-07-25 — Defined and received final human confirmation for the feature-side scope of
  configurable pipeline runs. Draft FS-14 now specifies model-neutral
  generic stage templates, run-time backend/model assignment, named opaque-text inputs/outputs,
  sequential outcome routing with approval gates and bounded repair loops, explicit authenticated
  stage results, restart-safe attempts, safe blocked/retry identity, retained run history, and
  visible shared-workspace concurrency. The run-start editor changes only the run name, project,
  goal, declared input values, and per-stage backend/model; structure and stage semantics remain
  template-owned. A dedicated Pipelines page supports manual creation and a
  provider/model-selected AgentDecker builder. AgentDecker can propose exact Save or Start actions;
  each executes only after a separate one-time UI confirmation, as a soft interaction guard under
  the existing same-user local-API trust model. TS-09 and small TS-01–TS-05 deltas now specify one
  in-process transactional reconciler, JSON templates, SQLite run state, scoped MCP result/proposal
  tools, shared lifecycle services, REST/SSE/CLI surfaces, restart recovery, and the Pipelines UI—no
  queue, second service, parallel graph engine, or new authentication layer. Effort remains the first
  separate idea. The source idea was promoted to the waiting `configurable-pipeline-runs.md` change;
  specification/whitespace checks and the Claude/Codex design-skill comparison pass, and no product
  code changed.

- 2026-07-25 — Fixed six of eight open review findings. **INV §15** closed: live annotations
  now use AppendAnnotationAndSync to persist and flush index before publishing, ensuring
  append failures block delivery and retries do not duplicate. **INV §15** closed: session
  upsert and metadata document replacement moved into one atomic SQLite transaction. **INV §12**
  closed: Codex prober reserves bounded 2-second deadline for native login so hung CLI cannot
  exhaust API-key fallback path; added regression test with hanging Codex CLI. **INV §8/§12**
  closed: Claude status failures return only bounded vocabulary ("status_check_failed") instead
  of raw output containing account identity. **INV §8** closed: sign-in error message updated
  to remove misleading dashboard reference. **INV §10** closed: FS-13 spec citation updated
  from retired R5 to active R25-R27. Specification checks, both Go test variants, 113 UI tests,
  source/UI builds pass. Two must-fix findings remain (onboarding wizard race, index boundaries
  atomicity).

- 2026-07-25 — Reviewed the continuous range after `8b84e4f` through `eb63dd5` in both spec
  directions and swept every invariant class. Six must-fix and two worth-fixing findings are recorded
  above. **INV §15** caught that live annotation persistence still hides append/sync failures and that
  metadata replacement is split from the session transaction. **INV §2/§5** caught non-atomic index
  document boundaries; §5 also caught the competing onboarding completion actions. **INV §8/§12**
  caught provider-output leakage, the exhausted Codex fallback deadline, and misleading sign-in copy;
  **INV §10** caught FS-13's citation to retired search behavior. Clean on the remaining classes:
  §1 annotation-tray lifecycle cleanup is bounded; §3 onboarding merge-preserves catalogs; §4 the
  installer owns both temporary artifacts on all exits; §6 has no new interface/runtime/driver; §7
  the new streaming/SQL readers check iteration errors; §9 migrations preserve legacy content and
  probe liveness is bounded overall; §11 collection shapes stay non-null; §13 all new UI selectors
  resolve; §14 adds no route and the existing annotation route remains under `localOnly`.
  Specification lint, both Go variants, 113 UI tests, source/UI builds, and distribution build pass;
  the open paths need new injected-failure and interleaving coverage.

- 2026-07-25 — Replaced whole-session transcript indexing with immutable turn documents. **INV §10**
  keeps raw NDJSON authoritative and the FTS projection rebuildable: a migration splits the old row
  into current metadata plus a preserved legacy content document, while new turns and annotation
  flushes append deterministic documents and metadata updates replace only metadata. **INV §5** no
  longer requires restart seeding because no replace-style transcript accumulator exists; the only
  buffer is the current turn and it is cleared after commit. **INV §2/§7** route live and reindex
  through the same event/document helpers, with `Reader.ForEach` making reindex and sequence recovery
  streaming rather than whole-session reads. Archive FTS groups document hits back to one session,
  counts/paginates distinct sessions, chooses the best transcript snippet, and intentionally requires
  every term or phrase inside one document. Existing fallback metadata search is unchanged. FS-05 and
  TS-02 return to Current; the more complete cross-turn/size-bounded alternative and its evidence
  threshold remain in `ideas.md`. Specification checks, both Go variants, focused index `-race`,
  source build, and distribution build pass.

- 2026-07-25 — Repaired onboarding credentials and defaults, the last waiting ready change.
  **INV §2** was the load-bearing find: `agentdeck auth` and the credential prober each carried their
  own copy of the provider commands, and the CLI's copy was simply wrong — `codex-acp` is a stdio ACP
  server that ignores argv, so `agentdeck auth codex` started a server and hung instead of signing
  anyone in. One `internal/backend/providerauth` table now owns both verbs for both providers, with a
  test asserting login and readiness share an executable. Codex readiness asks that CLI for
  `login status` before falling back to the API key, so a ChatGPT-signed-in user with no
  `OPENAI_API_KEY` is ready (FS-09.R34); verified against the real signed-in CLI and through
  `PUT /api/backends` on an isolated home, where the gate went from held-closed to satisfied.
  **INV §12** shapes the failure vocabulary: an uninterrogable CLI reports skipped, never failed, and
  "not logged in" is matched before "logged in" so the substring read cannot invert. The wizard gained
  the **Set up later** completion path (FS-04.R32) and lost both model-identifier fields (R33),
  submitting the seeded catalog unchanged per **INV §3**; provider sign-in is now named guidance plus
  Check again over the same backend save (R34, TS-03.R15). Fresh homes seed `sonnet`/`gpt-5.6-sol`
  aliases rather than dated pins (FS-09.R33), and re-seeding still leaves an existing catalog
  byte-for-byte intact. TS-06.R22 pins `@openai/codex` as a direct release dependency — the version
  was already resolved transitively, so the lockfile moved three lines — validated before packaging
  and proven to resolve with no global Codex installed. FS-04, TS-03, and TS-06 flip to Current.
  Specification checks, both Go test variants, focused `-race`, 113 UI tests, source/UI/dist builds
  pass. J2 in a real browser is the remaining unproven surface.

- 2026-07-25 — Fixed all five open findings. **INV §15** (Must fix): the annotations endpoint
  delivered reserved mail or started the prompt turn before appending the source annotation event, so
  any append failure returned 500 after an irreversible effect and the preserved tray's retry
  delivered a second copy. The handler now validates the target, composes its payload, appends the
  durable event, and only then delivers through one deferred closure; FS-13.R5 and TS-03.R14 pin the
  ordering, including the honest residue that a delivery failure after the append records a second
  annotation event on retry. `TestAnnotationAppendFailureDeliversNoMailAndRetrySendsOnce` blocks the
  transcript directory and asserts one mail row across the retry (new FS-13.A7). **INV §2**: the
  duplicated `clipExcerpt` collapsed into `ui/src/lib/annotations.ts` with the server marker
  authoritative, and both local `AnnotationDraft` re-declarations now import the canonical type.
  **INV §1**: pending trays are dropped when an agent is deleted, and expire/cap on rehydration
  (30 days, 20 sources) — new FS-13.R16/A8, since nothing on the server owns that state and the live
  agent list deliberately excludes archived sources. **INV §4**: the piped installer's temporary
  bootstrap is now removed by the lock-holding child, the last process that reads it, guarded to the
  installer's own `mktemp` name; the piped-install regression runs under a private `TMPDIR` and
  asserts nothing is left behind. Specification checks, both Go test variants, focused `-race` on the
  annotation path, 107 UI tests, source build, and distribution build pass.

- 2026-07-25 — Audited the prior two weeks of fixes against the invariant catalog and kept only
  repeatable, cross-cutting lessons. New INV §15 requires local durable state before releasing an
  external peer or observable side effect, with atomicity, rollback, or idempotency for retryable
  multi-store mutations; the open annotation partial-delivery finding now cites it. INV §2 now names
  live/replay event projection as one logical artifact and records the shared transcript reducer.
  INV §11 records the second null-collection recurrence from incomplete nested backend maps and
  adds server validation plus `?? {}` to the canonical pattern. Release-only and provider-specific
  fixes stayed in their narrower FS/TS requirements and regression tests rather than bloating INV.
  Documentation checks and whitespace validation pass; no product code changed.

- 2026-07-24 — Closed the routing hole that let the invariant sweep be skipped. The read order in
  every launcher said to read "the relevant FS/TS/INV items named by the handoff/request", so when no
  INV item was named an agent resolved that to nothing and never opened the file where the sweep
  instruction lives. `/review`, `/work`, and `/fix` (both twin copies) now name `INVARIANTS.md` as an
  unconditional read and state the sweep, the §6 checklist, and the changelog class tag directly.
  `check-specs.sh` gained a `## Review findings` check: each bullet starts with **Must fix**/**Worth
  fixing**, and a bullet citing code carries an `INV §n` tag or the literal `(no invariant class)`.
  Both rules were already documented (workflow §7 and INVARIANTS.md); the check only makes omitting
  them fail a command the loop already runs, and it verifies the tag rather than the thinking. It
  runs on the full sweep and on any `--file` HANDOFF edit, so the post-edit hook catches it too.
  Applying it retro-tagged the open installer finding as INV §4 — `exec` replacing the process image
  so the EXIT trap never fires is a cleanup that misses an exit path. Not done, and still open for a
  decision: the workflow-text guards against inheriting a prior review's framing and against
  test-coverage findings standing in for defect findings.

- 2026-07-24 — Re-reviewed the same range after `61b234d` through `8b84e4f` at the human's request,
  this time sweeping the diff against every INVARIANTS class as `INV` requires of `/review`. The
  earlier pass checked only FS/TS requirement conformance and test coverage, so it missed four
  defects now recorded above: an unrolled-back partial delivery (INV §4, breaking FS-13.R5), two
  duplicated-helper drifts (INV §2), and an unbounded per-agent tray with no lifecycle cleanup
  (INV §1). Clean on the remaining classes: §13 all 15 new annotation selectors resolve; §14 the new
  route inherits `localOnly`; §5 the self-target idle pre-check is advisory but `SendPrompt` claims
  `turnActive` atomically; §10 both call sites gate `sourceActive`/`annotationsEnabled` correctly
  (FS-13.R15); §11 an empty batch cannot marshal to `null`. Spec conformance findings from the first
  pass stand unchanged; no product code or specs were edited.

- 2026-07-24 — Reviewed the continuous range after `61b234d` through `8b84e4f`, validating
  the annotate-and-assign implementation. All specification requirements (FS-13.R1-R15,
  FS-06.R21, TS-02.R14-R15, TS-03.R14) match the code in both directions. Required tests pass:
  UI store persistence, message delivery and budget integrity, diff/event selection and clipping,
  archive indexing, endpoint validation and routing. Go test suite (both FTS5 and non-FTS5 variants),
  UI tests (105 tests), and full builds pass with no regressions. J13 real-browser usability journey
  documented but pending. Recorded no findings — superseded by the re-review above, which found four.

- 2026-07-24 — Implemented annotate-and-assign. Live and archived chat transcripts now support
  diff-line and event annotations in a browser-local pending tray, delivery to the current agent,
  another running chat agent, or a prefilled new-agent launch, durable annotation cards, reserved
  dashboard-user mail, and archive indexing. The new regression coverage verifies validation,
  mail-size clipping, no-budget reserved mail, durable delivery, and annotation search; spec, UI,
  both Go test variants, source build, and distribution build pass. J13 is the remaining real-browser
  usability journey for this new surface.

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
