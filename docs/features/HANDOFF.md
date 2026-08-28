# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Bug investigation:** The 2026-08-28 pipeline `stale_assignment` report is diagnosed and open for
  `/fix`: answering a blocked stage agent in its own chat, instead of through **Continue**, gets its
  next report refused as `stale_assignment` even though it is the run's current attempt. Reproduced
  by the committed skipped test in `internal/pipeline/blocked_chat_answer_test.go`. Four findings
  below; the second needs a product decision. The earlier 2026-08-27 all-200/no-page-load incident
  is fixed. Same-origin dashboard tabs now share one long-lived SSE connection, leaving the
  browser's HTTP/1.x pool available for REST queries; the related config-source,
  transcript-reconciliation, and card-preview amplification paths are bounded as described in the
  changelog.
- **Review state:** Every review and usability finding through 2026-08-28 is closed in code, tests,
  or an explicit specification boundary, including the six recorded against the split Pipelines
  surface. The four bug-investigation findings below are open.
- **Active change:** None. The Pipelines surface split is finished and committed (`9114df7`, with
  its usability fixes in `69c2f99`) and is ready for an independent review; its change file is
  removed, and FS-14 is the authority on what shipped.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  remain owed before claiming those adapters accept structured results.
- **Usability state:** The split Pipelines surface was driven through a real Chromium on
  2026-08-28 against the working tree's own release build. Every FS-14.A14–A23 item was exercised;
  A14, A16–A20, A22 and A23 passed outright, and the A15 and A21 gaps are now fixed in code and
  tests. The badge, rail-order and append-motion fixes are unverified in a real browser: no browser
  run has been made since them.
  The earlier v0.2.2 → v0.2.3 delta remains closed: FS-02.A24 is closed and FS-04.A22 remains
  narrowed to the native panel.
- **Last reviewed code:** `895348e` (2026-08-26).
- **Branch:** `main`.

## Active change

**Change:** None.

**State:** Ready for the next human-selected change.

**Next:** Run `/fix` on the four open bug-investigation findings, starting by un-skipping
`internal/pipeline/blocked_chat_answer_test.go`. The second finding needs the user's product
decision before it can close. The independent code review of the committed Pipelines redesign is
still owed.

## Changelog

- **2026-08-28 — bug investigation:** Diagnosed the field report that a blocked pipeline stage agent
  got `stale_assignment` when it tried to continue. Reproduced the most reachable route locally: a
  `blocked` report leaves the stage agent live and idle and the run offers **Open agent** beside
  **Continue**, so answering the question in that chat makes the agent's next report land on
  `internal/pipeline/actions.go:198`, which returns `stale_assignment` / "caller is not the current
  stage attempt" for a caller that *is* the current attempt under the current generation — a retry
  class of `never` (FS-17) on a false reason, discarding a full turn of work. Refusing is specified
  (FS-14.R19); the code, the message, and the silence around them are not. Four findings recorded:
  the misclassified refusal, the unspecified blocked-pause boundary (needs a user product
  decision), the total absence of refusal logging, and an unlocked-read compare-and-swap in
  `OnTurnEnd`/`OnExit` that can park a run at `await_quiescence` (INV §5, probable). No product code
  or specification changed; the only tree change is the skipped reproduction test.

- **2026-08-28 — docs:** Retired the completed *Projects page problems* and 2026-08-10
  play-session ideas at the user's direction, and closed out the Pipelines redesign paperwork: its
  finished change file is removed and it no longer appears under *Changes waiting to start*, with
  FS-14 left as the authority on what shipped. The card drag-and-drop and Content-Security-Policy
  ideas were checked against the tree and kept: drag listeners are still bound to the handle button
  alone and no CSP exists in the server, the UI shell, or the specs.

- **2026-08-28 — fix:** No open review findings. `## Review findings` was empty and the
  working tree was clean, so no code, test, or specification changed. The committed Pipelines
  redesign and its six usability fixes are still waiting for their independent code review.

- **2026-08-28 — fix:** Closed all six Pipelines usability findings. The attempt badge sits on the
  timeline rule instead of off the left window edge and the run rail no longer hoists itself above
  the timeline at the desktop floor (INV §13). The start path explains every gate it applies: with
  no valid template **Start run** is refused with a link to Templates, and a disabled step control
  names the missing value and marks the empty required input (INV §8/§10) — new behavior, specified
  as FS-14.R46/A24. A newly appended timeline entry now plays the one brief entrance FS-14.R45/A21
  already promised, tracked from the ids present at first render so neither the initial list nor a
  background refetch replays it (INV §10). A deleted run explains itself in product language, while
  any other read failure says so honestly and keeps its transport detail (INV §8).

- **2026-08-28 — usability review:** Drove the split Pipelines surface through a real Chromium
  against a `make dist` build of the working tree, on three isolated fixtures: an empty home, a home
  where one run was started end to end through the dialog, and a seeded home carrying a seven-attempt
  repair loop, the approval/blocked/crashed pause branches, 121 retained runs, 26 delegated tasks on
  one attempt, a second-hop task, a task whose agent record is gone, a 32-stage template, and one
  pending proposal of each kind. Zero blockers; five **Worth fixing** findings and one polish item
  recorded below. Routing, the execution timeline, attempt agent cards and their live/archive routes,
  the delegated cap and true counts, bounded run history to an exact end, frozen stage titles after
  template deletion, the focused editor, action gating, legacy links, background-refetch stability,
  reduced motion and the Sky & Grove appearance all behaved as specified with no console error.
  Full run:
  [`../archive/reviews/usability-review-run-2026-08-28-pipelines.md`](../archive/reviews/usability-review-run-2026-08-28-pipelines.md).

- **2026-08-28 — work:** Shipped the Pipelines redesign. Runs and Templates now have separate,
  addressable list/detail experiences; run supervision is timeline-first with frozen setup, named
  values, complete paginated history, and reload-safe stage/delegate agent summaries; template
  authoring is a focused one-stage workspace; and start-run uses a stable three-step dialog. The
  additive run projections use one indexed, bounded task read and preserve the existing pipeline
  control plane. The full UI and Go suites, spec checks, release build, and real-browser checks at
  the supported desktop floor are green; browser QA also found and closed a runtime-dialog footer
  overflow before completion.

- **2026-08-27 — fix:** Closed the active-first Ordering design finding (INV §10). FS-02.R45/A28
  now require the rendered running-first id order to drive drag geometry, same-block drops to map
  back to the unchanged flat manual order, and cross-block drops to perform no reorder or layout
  write. J5 and the ready change cover the visible in-drag boundary and persistence behavior. The
  ready change now names all six `AgentStatus` values, including `unknown`; the broader cross-group
  and whole-card drag problems remain separately out of scope.

- **2026-08-27 — design review:** Reviewed the waiting active-first Ordering change against FS-02,
  FS-12, the shipped card-grid/layout/SSE seams, dnd-kit's pinned sortable implementation, and every
  invariant class. The render-time `running` partition cleanly extends the existing group projection
  and needs no server or persistence change, but the design leaves `SortableContext` keyed to the
  differently ordered flat manual list. One implementation-gating drag finding and one consistency
  note remain. No product code, specification, or change file was edited.

- **2026-08-27 — work:** Shipped Mermaid rendering in assistant messages (FS-03.R37–R38,
  TS-08.R40). A closed ```mermaid fence becomes a themed diagram; an open one stays a code block
  until its closing delta arrives, decided from react-markdown's source span. `renderers/mermaid.ts`
  owns the 50,000-code-unit bound, the refused external-image grammar, strict initialization, and a
  directly declared DOMPurify pass that also strips remote references from the generated `<style>`;
  `MermaidDiagram.tsx` is the only markup-insertion seam, now enforced by a new `renderer-markup`
  rule in the presentation audit plus its manifest entry and a repository check. Mermaid and
  DOMPurify load on demand, so the initial bundle grows only by the host code (1,813.87 → 1,818.43
  kB). J3's diagram steps were driven through a real Chromium against the production server and
  embedded UI on the deterministic `diagram_stream`/`diagram_injection` fake-ACP scenarios: open
  fence stayed a code block, the closing delta promoted it, the source toggle round-tripped, a live
  skin change regenerated the mounted diagram, reload and the archived transcript matched, and the
  injected script/HTML-label case rendered inert with no console error and no external request.

- **2026-08-27 — fix:** Closed all three Mermaid design-review findings (INV §8/§10). The design
  now rejects Mermaid's pinned external-image grammar before rendering, directly owns DOMPurify,
  replaces the unenforceable main-thread timeout with an exact 50,000-code-unit source bound, and
  reuses the existing presentation observer to regenerate mounted diagrams after a skin change.
  It explicitly avoids a worker, iframe renderer, or custom parser; acceptance tests cover the
  no-request preflight, size fallback, and live palette change.

- **2026-08-27 — design review:** Reviewed the waiting Mermaid chat-rendering change against FS-03,
  TS-08, the shipped Markdown/presentation/archive seams, Mermaid 11.17.2, and every invariant
  class. The existing `AssistantText` and presentation adapters are the right extension points, and
  the closed-fence state is recoverable from react-markdown's source-position metadata. Three
  implementation-gating findings remain: Mermaid image nodes can make an attacker-chosen request
  before returned SVG can be sanitized, a main-thread render cannot be interrupted by the promised
  elapsed-time budget, and an already-rendered SVG does not observe a later skin change. No product
  code, specification, or change file was edited.

- **2026-08-27 — design:** Defined two small UI changes and held a third. FS-03.R37–R38 and
  A20–A22 specify Mermaid rendering in assistant messages, gated on the closed fence because every
  delta re-renders the whole message, with a code-block fallback and one sanitizing injection seam;
  TS-08.R40 puts the renderer on the existing R13 adapter seam as a dynamically imported chunk
  pinned at or above the release fixing the known diagram HTML-injection defect. FS-02.R45 and A28
  order running agents before stopped ones inside each group while preserving the manual order, and
  FS-12.R37 narrows R10's ordering clause, which forbade it. FS-02, FS-03, FS-12 and TS-08 are now
  Partial; J3 and J5 cover the new evidence. Queued `mermaid-diagrams-in-chat.md` and
  `active-agents-before-stopped.md`. Edit-a-sent-message was **not** specified: ACP v1 as
  implemented has no rewind method, the transcript and FTS documents are append-only, and the only
  rewind-shaped seam is the lossy switch primer, so the user chose to hold it with those findings
  recorded in `../ideas.md`. The absent Content-Security-Policy is recorded there too.
  `make check-specs`, the twin-skill comparison and `git diff --check` are green.

- **2026-08-27 — fix:** Closed all four Pipelines design-review findings (INV §7/§8/§10).
  FS-14.R36/R39 now require frozen human stage titles, complete attempt-agent cards, correct
  same-agent continuation attribution, and retained-history pagination; R44–R45 and A20–A23 make
  the focused hierarchy, progressive disclosure, stable refetch state, restrained motion, and
  reduced-motion result observable. TS-03.R29–R30 define the additive detail/list contracts,
  TS-09.R28 replaces the project-wide task read with the capped attempt-window projection, and
  TS-02.R26 owns its one composite read index. J14 and the ready change cover the dense, paginated,
  live-update, fallback, and reduced-motion evidence. The stale effort note is removed.

- **2026-08-27 — design review:** Reviewed the waiting Pipelines split against FS-12, FS-14,
  TS-03, TS-08, TS-09, the shipped frontend/server seams, and every invariant class. The separation
  into Runs and Templates is sound, but four implementation-gating findings remain: the requested
  high-end interaction outcome has no observable contract, the list/card data shapes cannot render
  all promised human-facing content, retained run history silently stops at the first page, and the
  delegated-work response cap does not bound the work used to build it. One stale effort sentence
  is recorded separately as a consistency note. No product code, specification, or change file was
  edited.

- **2026-08-27 — design:** Defined the Pipelines surface split. FS-14.R25 is superseded by R35–R43
  and A14–A19; TS-09.R28 specifies the derived one-hop delegated-agent projection composed at the
  HTTP boundary with no new schema; TS-03.R29 specifies the additive capped run-detail block and its
  `task_update` refetch rule. FS-14, TS-09 and TS-03 are now Partial and J14 covers the split
  surface. Queued `docs/ready-changes/split-pipelines-surface.md`; the source idea is removed.
  `make check-specs`, the twin-skill comparison and `git diff --check` are green.

- **2026-08-27 — fix:** Closed all thirteen open findings. Browser tabs share one SSE stream and
  six-tab REST capacity is specified (INV §1/§8); unchanged config generations no longer publish,
  session writes reconcile incrementally after a debounce, and background streams retain bounded
  card previews (INV §7/§8). Project warnings stay visible, task supervision names and links real
  work, defaults/actions/copy match the shipped contracts, the lifecycle flake waits for ownership
  to settle (INV §5/§8), and FS-16/FS-17 now state their verified boundaries accurately (INV §4/§10/§15).

- **2026-08-27 — bug investigation correction and reproduction:** Built a deterministic production-UI
  stress fixture with one Claude Haiku-labelled orchestrator, six workers, and 7,000 streamed ACP
  deltas. A real Chromium reproduced the field symptom at the sixth same-origin tab: the shell and
  SSE opened, REST queries never reached the server, and the page remained on `Loading project…`.
  Closing one existing tab released an SSE connection and the stalled tab completed in 127 ms. This
  confirms browser HTTP/1.x connection starvation as the root cause. The earlier default-home
  analysis inspected a different computer and is not incident evidence; its config-source and
  transcript findings remain independent scaling defects only.
- **2026-08-27 — usability review:** Drove the user-facing changes released between `v0.2.2` and
  `v0.2.3` through a real Chromium against the release binary on isolated fixtures: dependent work
  end to end, the dashboard attention count, the task-concurrency budget, browser-local chat drafts,
  the directory-browse control, and the projects-canvas context menu. One **Must fix** and five
  **Worth fixing** findings recorded below; no blocker. The FS-02.A24 right-click gate is closed and
  the FS-04.A22 gate is narrowed to the native panel alone. J15–J17 were added to the journey matrix
  because FS-15, FS-16 and FS-17 shipped citing no journey at all. Full run:
  [`../archive/reviews/usability-review-run-2026-08-27-release-delta.md`](../archive/reviews/usability-review-run-2026-08-27-release-delta.md).
- **2026-08-26 — review:** Verified the fixes for the nine findings against the tree. Seven are
  closed with real regression coverage, including a three-level chain test that also exposed the
  restart re-evaluation having been a no-op because `TasksInStates` never populated `Arms`. Three
  residual items are recorded below. `make test`, `make build`, `make check-specs`,
  `git diff --check`, `npm test` (251 passing), and `npm run build` are green;
  `TestRetryRunsAgainOnTheSameAssignee` failed once in three full-suite runs and did not reproduce.
- **2026-08-26 — work:** `/work` found no active change and no waiting ready change, so no
  implementation started. The repository remains clean and ready for the next designed change.
- **2026-08-26 — fix:** Closed all nine review findings. Source evaluation now publishes and
  recursively propagates across pipeline and restart boundaries (INV §1/§10/§15); task start
  generations are confirmed at effect time and failed wake confirmation settles the attempt (INV
  §4/§5); context authorization has one transaction-safe predicate (INV §2); task and dashboard
  errors surface accurately (INV §8); and agent-tool retry coverage and its SDK boundary now match
  FS-17 (INV §10). The two review-named pipeline conflict codes are HTTP-only, not MCP refusals.
- **2026-08-26 — review:** Reviewed `e1e827b`, `b121fd0`, and `895348e` against FS-02, FS-16, FS-17,
  TS-03, TS-04, TS-05, TS-10, and every invariant class. Two **Must fix** and seven **Worth fixing**
  findings recorded below; `make check-specs`, the Go suite for the touched packages, `tsc --noEmit`,
  and `npm test` all pass, while `git diff --check` reports one trailing-whitespace line.
- **2026-08-26 — implementation:** Shipped the agent-facing tool result contract. Every MCP refusal
  now carries a centralized four-class retry hint, successful and refused JSON objects are mirrored
  into structured content without changing the text block, and registration-derived tests cover the
  complete tool surface and deferred output-schema boundary.
- **2026-08-25 — fix:** Closed all thirteen dependent-work review findings. Task starts now retain
  the runtime's true generation, cancellation and start serialize per task, failed dependencies
  propagate fully, dispatcher transitions publish after commit, failed cleanup retains ownership,
  attachment creation is atomic and bounded, and the Tasks UI and API cover their specified paths.
- **2026-08-25 — design:** Defined FS-17 (agent-facing tool result contract) and TS-04.R30–R31, and
  queued `docs/ready-changes/agent-tool-retry-classification.md`. Investigation of the "richer
  agent-facing orchestration API" idea found most of it already shipped; the remainder is trimmed in
  `../ideas.md`. The task-graph query exclusion (FS-16 §6, TS-04.R29) was reaffirmed by the user.

## Decisions needing your input

These are product decisions needed for a future change or shipped boundaries whose reversal needs
an explicit specification update. Remove an item when the human resolves it or queues that update.

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.

## Acceptance gates

- [ ] Run pinned, credentialed Claude and Codex chat/MCP/resume checks before claiming those combinations.
- [ ] Run pinned Claude terminal flags/hooks and live xterm journeys before claiming full terminal support.
- [ ] Run pinned OpenCode/OpenHands launch/credential checks before claiming those backends beyond fakes.
- [ ] Run J2/J9/J16 in a real macOS browser to confirm the native folder panel opens in front,
  selects, and cancels (FS-04.A22). Narrowed on 2026-08-27: a real browser confirmed the **Browse…**
  controls are present and enabled for `cwd` and the pending `add_dirs` entry in both the Settings
  project form and the New project modal, and that the onboarding wizard renders styled. Only the
  native `osascript` panel itself is still unverified, and it needs a human at the machine.
- [x] **Closed 2026-08-27.** A real Chromium confirmed a right-click anywhere on the projects
  canvas opens **New project** (FS-02.A24): eight background points including the padding frame on
  every edge and corner, while a card right-click still opens the card menu, and the menu opens a
  styled create modal. Evidence in the J16 section of
  [`../archive/reviews/usability-review-run-2026-08-27-release-delta.md`](../archive/reviews/usability-review-run-2026-08-27-release-delta.md).
- [ ] Run a task start, an assignment turn, and a reported result against the pinned Claude and Codex
      adapters before claiming dependent work works with real providers (FS-16 §6).
- [ ] Run one successful and one refused MCP tool call through pinned Claude and Codex adapters before
      claiming they accept structured tool results without losing the text block (FS-17.A6).
- [ ] Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude and
  Codex installations before promoting FS-08/TS-07 from Partial.

## Blocked on human

Live-provider acceptance is waiting for human authorization because it invokes real provider sessions
and creates disposable local configuration homes. On 2026-07-15 this machine has Claude Code 2.1.202,
the retired `claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new
`claude-agent-acp`, OpenCode, and OpenHands are not installed globally.

## Review findings

From the 2026-08-28 bug investigation of the field report: *"An AgentDecker session blocked the
progression of a pipeline because it needed an answer from me, then when it tried to continue it got
`stale_assignment` from AgentDeck."* No AgentDeck version, environment, or log was supplied, and this
machine's `~/.agentdeck` holds no run newer than 2026-08-01, so it is not the incident home — the
diagnosis below is from the code path and a local reproduction, not from incident logs.

- **Must fix** (confirmed) `internal/pipeline/actions.go:198` refuses with
  `stale_assignment` / "caller is not the current stage attempt" for three different conditions,
  including one where the caller *is* the current stage attempt under the current generation. A
  `blocked` report pauses the run with its stage agent still live and idle (FS-14.R11) and the run
  detail offers **Open agent** beside **Continue** (`ui/src/features/pipelines/RunBrowser.tsx:132`),
  so the ordinary human move — answering the question in the chat that is showing it — leads
  straight here: the agent finishes the stage, calls `report_pipeline_stage_result`, and is told it
  is not the current attempt. Refusing is right (FS-14.R19 forbids a second accepted result), but
  the code and message are wrong: the real reason is that this attempt's report was already
  accepted and the run is paused waiting for a human Continue, and FS-17 classifies
  `stale_assignment` as retry `never`, so the agent abandons the stage on a false reason and a full
  turn of work is silently discarded. `already_reported` is the shared vocabulary's name for this
  condition and the task path already returns it
  (`internal/messaging/task_tools.go:114`), so the second meaning here is also INV §2 drift.
  Reproduced by the committed skipped test
  `internal/pipeline/blocked_chat_answer_test.go` (`TestBlockedStageAgentAnsweredInChatIsNotCalledStale`);
  un-skip it to start the fix. Fix: split the predicate — keep `stale_assignment` for a genuine
  agent/generation mismatch, return `already_reported` naming the human Continue for the run's own
  current attempt (INV §8).

- **Must fix** (confirmed) Nothing tells a stage agent that a `blocked` report ends its
  participation until a new assignment arrives. `renderAssignment`
  (`internal/pipeline/assignment.go:35`) says only "call `report_pipeline_stage_result` exactly
  once", and the accepted report returns `"awaiting":"quiescence"`
  (`internal/messaging/pipeline_tools.go:35`) — neither states that chat input received during the
  pause is out of band, that the person's answer arrives as a new assignment, or that work done
  meanwhile cannot be recorded. FS-14 and TS-09 have no requirement covering this, so it is a
  specification gap as well as the trigger for the finding above. The same dead end is reachable a
  second way and there the work is unrecoverable by design: after a restart `RecoverRuns`
  (`internal/pipeline/manager.go:320`) pauses an `await_result` run as `restart_recovery` and stops
  the agent, `Continue` rejects that state (`internal/pipeline/actions.go:35`), and an ordinary
  chat resume of that agent mints an unrelated generation
  (`internal/server/resume.go:419`), so its report is refused forever and only **Retry** — a fresh
  agent, from scratch — moves the run. FS-14.R20 puts that boundary in place deliberately, so
  closing this needs a product decision from the user, not an agent's choice: either state the
  boundary and say it in the assignment text and the refusal, or give the blocked pause a real
  in-chat route. The **Open agent** action shipped on a pause where following it is a dead end
  (INV §10).

- **Worth fixing** (confirmed) No refusal from `report_pipeline_stage_result` is logged anywhere:
  `internal/messaging/pipeline_tools.go` and `internal/pipeline/actions.go` have no logger on the
  report path, and `internal/pipeline` logs only from `proposals.go`. `~/.agentdeck/dashboard.log`
  records every HTTP request but not one control-plane refusal, so this report could not be
  corroborated from the server log and the next one will be equally undiagnosable — the only trace
  is inside the agent's own transcript. Log at Warn on refusal: run id, attempt id, caller agent
  id/generation, the attempt's agent id/generation, the run's pending action, and the code. That
  set is exactly what separates the three conditions the first finding conflates
  (no invariant class).

- **Worth fixing** (probable) `OnTurnEnd` and `OnExit` (`internal/pipeline/actions.go:281` and
  `:305`) read the run *outside* `runLock`, then compare-and-swap on that captured revision *inside*
  the lock and return `nil` when it conflicts. `Report` re-reads the run under the lock; these two
  do not (INV §5). A revision bump in that window leaves a run parked at `await_quiescence` — state
  `running`, no attention reason, no notification, no log — with no further turn boundary coming, so
  the run silently stops progressing and any later report is refused with the same
  `stale_assignment`. Not reproduced: it needs a concurrent bump in a narrow window, and no field
  log exists to confirm it happened here. Fix: re-read the run under the lock as `Report` does, and
  log the conflict.
