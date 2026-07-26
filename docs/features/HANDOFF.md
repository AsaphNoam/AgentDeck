# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Active change:** None.
- **State:** configurable pipeline runs are complete and now reviewed. FS-14 and TS-09 are Current;
  the reusable template store, durable sequential manager, shared lifecycle execution, scoped
  result/proposal tools, REST/SSE/CLI controls, notification/association paths, and Pipelines UI are
  shipped. Specification checks, both Go test variants, 120 UI tests, source/UI builds, and the
  distribution build succeed, and the review swept the diff against every invariant class.
  Five findings are open from this range — a solicited stop that wedges a run, an assignment prompt
  that can truncate away its own reporting instruction, attempt-history transcript links that point
  at the live agent route, and two smaller presentation/lifecycle gaps. All five are paths the
  current tests do not exercise, which is why the green gates do not close them. Two findings from
  the previous range also remain: the onboarding wizard's Set up later action can race a
  mounted-step mutation, and index event/flush boundaries can interleave.
  Credentialed provider acceptance remains a separate manual release gate; native prompt/confirm
  actions also need replay in a browser that supports those dialogs.
  The onboarding change is verified by automated tests plus a real isolated-home run; **J2 has not
  been replayed in a browser**, so the wizard's Set up later and Check again controls are unproven
  against real rendering and pointer interaction.
- **Last reviewed code:** `ccc2b50` (2026-07-26), the continuous range after `eb63dd5`.
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

## Review findings

- **Must fix** — A solicited stop of a pipeline stage agent leaves its run wedged.
  `internal/runtime/registry.go:281-287` deletes `generationByAgent` before the runtime exits, so
  `handleAgentExit` hands the server's exit hook an empty generation and
  `internal/pipeline/actions.go:281-283` returns early on the generation mismatch. That is correct
  for the pipeline's own `StopStage`, but the ordinary `POST /api/sessions/{id}/stop`
  (`internal/server/sessions.go:387-408`), a group stop, and `handleSwitchRuntime` take the same
  path — and FS-14.R16 explicitly keeps stage agents as ordinary cards with those actions. After a
  user stops the current stage agent the run stays `running`/`await_result` with a dead agent: no
  attention reason, no notification, and `Manager.Retry` refuses because it requires
  `state == "paused"` (`internal/pipeline/actions.go:90`). There is no periodic sweep (TS-09.R1), so
  the run only recovers on dashboard restart. Distinguish pipeline-solicited stop from any other
  exit — pass the generation through to `handleAgentExit` and let the manager decide — and add a
  server test that stops a stage agent through the ordinary endpoint and asserts the run pauses with
  a recovery action (FS-14.R12/R16, TS-09.R13, **INV §4**).
- **Must fix** — A large named value silently truncates the assignment's own reporting instruction.
  `internal/pipeline/assignment.go:19-60` writes the goal, declared inputs, and prior results first
  and the mandatory "call `report_pipeline_stage_result` exactly once" line plus the declared-output
  list last, then clips the whole prompt to `maxAssignmentRunes` (48000). `MaxValueRunes` is 64000
  and `validateStart` accepts inputs up to that bound, so one pasted specification larger than the
  prompt budget removes the protocol instruction the stage agent needs. The agent then never
  reports, and the run hangs in `await_result` with the same missing recovery as the finding above.
  Render the fixed protocol/outputs section before or independently of the variable body, budget the
  variable fields against the prompt limit, and add a renderer test asserting the report instruction
  survives a maximum-size input (FS-14.R5, TS-09.R7/R19, **INV §8**).
- **Must fix** — Attempt-history transcript links cannot open a finished stage's transcript.
  `ui/src/features/pipelines/RunBrowser.tsx:123` renders **Open transcript** for every attempt as
  `/agent/{agent_id}`, but that route reads the live agent store
  (`ui/src/components/chat/ChatPanel.tsx:30,67-73`) and stopped agents are removed from it
  (`ui/src/store/agentStore.ts:34`). Every completed attempt in a finished run therefore lands on
  "Agent not found" — the archive route `/archive/{agent_id}` is the working one. Route the link by
  liveness (or always to the archive for a completed attempt) and cover it in the Pipelines UI test
  that renders attempt history (FS-14.R8/R11, FS-14.A7, **INV §10**).
- **Worth fixing** — Pipeline notifications identify the run by its opaque id and cannot tell success
  from failure. `internal/server/pipeline_lifecycle.go:149-151` passes only the run id, so
  `internal/bus/bus.go:180-189` builds a title of `Pipeline pr_<hex> needs attention` instead of the
  run's display name, and the completion notification carries an empty body because
  `AttentionReason` is cleared at completion — the terminal `success`/`failure` outcome FS-14.R29
  names never reaches the toast. Carry the display name and final outcome on `PipelineUpdate` and
  assert both in a bus test (FS-14.R29, TS-09.R17, **INV §8**).
- **Worth fixing** — The AgentDecker builder agent id is persisted with no lifecycle cleanup.
  `ui/src/features/pipelines/AgentDeckerBuilder.tsx:9,61,101` writes
  `agentdeck.pipeline-builder-agent` to `localStorage` and never clears it, so once a builder has run
  the Pipelines page permanently shows a "Builder session" block whose **Open AgentDecker chat** link
  goes to the same "Agent not found" view after the agent is stopped or deleted, and re-fetches that
  dead transcript on every mount. This is the annotation-tray class again: browser-local per-agent
  state needs a boundary drop plus its own retention bound. Clear the key when the agent is gone and
  add a store test for the stale-id case (TS-09.R21, **INV §1**).
- **Must fix** — `ui/src/features/onboarding/OnboardingWizard.tsx:97-106` disables **Set up later**
  only for its own config mutation; it has no knowledge of the backend, project, source, launch, or
  launch-completion mutation in the mounted step. Clicking it while **Checking…**, **Creating…**, or
  **Launching…** is pending closes the wizard as skipped, but the other request can still save the
  backend, create a project, or launch an agent, contradicting FS-04.R32/A13. Serialize the wizard's
  completion choices or propagate a shared pending/claimed state, with a deferred-request UI test
  for every mutating step (**INV §5**).
- **Must fix** — Turn and annotation document boundaries are two calls rather than one atomic index
  operation: `internal/runtime/chat.go:883-897` calls `OnEvent` then `OnTurnEnd`, and
  `internal/server/sessions.go:247-259` calls live `AppendAnnotationAndSync`/flush then publishes.
  `internal/index/indexer.go:93-181` releases the per-agent buffer lock between those calls, so a
  concurrent next-turn prompt, nudge, or active-source event can enter the buffer before the prior
  boundary flush. The later event is then stored in the wrong immutable document, while streaming
  reindex processes sequence order and builds a different projection. Make event consumption plus a
  boundary flush one sequence-aware atomic operation and add deterministic interleaving tests for
  both turn-end and annotation boundaries (TS-02.R16, FS-05.R25, **INV §2/§5**).

## Recent changelog

_(Newest first; durable product truth is in FS/TS and history is in git.)_

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
