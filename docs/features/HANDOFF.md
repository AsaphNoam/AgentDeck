# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Open the other sections only when those point at them. Settled state is
archived in [`../archive/state/HANDOFF-through-2026-09-03.md`](../archive/state/HANDOFF-through-2026-09-03.md)
and [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only and is cut
back to that at every release (§16.7). Budget: 300 lines.

## Current position

- **Active change:** None. Automatic pane opening on a waiting transition is finished and verified
  (FS-02.R61/A43, the narrowed FS-02.R51/A30, TS-08.R49); its rendered half is an open acceptance
  gate below, and the code is unreviewed. The previously shipped work is unattended pipeline runs (FS-03.R40–R44/A24–A27,
  FS-04.R46/A26, FS-06.R28–R29/A18–A19, FS-14.R52–R56/A29–A32, TS-01.R27, TS-02.R28, TS-03.R34–R35,
  TS-04.R41–R44, TS-05.R20, TS-08.R50–R52, TS-09.R29–R31), finished and verified. Earlier finished
  work is listed in the archived handoff; the named FS/TS requirements are the authority on what
  shipped.
- **Release:** `v0.3.0` is published and verified. The credentialed Claude and Codex checks are not
  covered by release CI and remain owed (TS-06.R21). A customized `agentdecker` role is deliberately
  not migrated (FS-04.R44), so it keeps the superseded product manual beside the current skill;
  nothing user-facing says so.
- **Last reviewed code:** `636781b` (2026-09-03), across the continuous range
  `e46e66b..636781b`. **Next review unit:** none committed; the uncommitted workflow-efficiency
  documentation and hook changes remain awaiting independent review.
- **Open findings:** eleven, listed below — two **Must fix** (streamed Mermaid remount; FS-14 Reject
  versus approval ordering) and nine **Worth fixing**. The 2026-09-02 bug investigation's one
  remaining **Must fix** is the silent-stage product decision under *Decisions needing your input*.
  The task-cancel release flake is open as a **Worth fixing** finding.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  remain owed before claiming those adapters accept structured results.
- **Usability state:** The Pipelines pages and dashboard grid were driven through a real Chromium on
  2026-08-30 against a `make dist` build of the shipped tree; that run is closed except for the
  items still owed here. Owed: A25's stage-boundary wording (needs a live report cycle `fakeacp`
  cannot drive), A18's consumption on approval, A32's unknown-agent and cross-project id cases, and
  the refused-drag pointer's computed-cursor pass (FS-02.R53, an open finding below). Full run:
  [`usability-review-run-2026-08-30-new-pages.md`](../archive/reviews/usability-review-run-2026-08-30-new-pages.md).
- **Branch:** `main`.

## Active change

**Change:** None. Automatic pane opening on a waiting transition is finished; its change file is
removed and FS-02.R61/A43 with TS-08.R49 are the authority on what shipped. Unattended pipeline runs
is likewise finished.

**State:** A chat agent that newly enters `waiting_input` expands its own pane on the mounted grid,
through the same append-and-cap path a click takes. `CardGrid` holds one ref of the last observed
`state` per agent id; it is reseeded and fires nothing while `agentStore` is hydrating and until it
has hydrated once, so a first load, a reload, a reconnect, and an agent first seen already waiting
all open nothing. FS-02 and TS-08 now carry no `(planned)` item and are `Current`.

**Next:** Independently review the automatic-pane-opening implementation and the unattended-pipeline
fix. Keep the credentialed Claude/Codex browser journey as an acceptance gate until a human
authorizes real provider sessions.

## Workflow efficiency investigation

**Awaiting independent review.** Documentation, skills, and hooks only — no product code. Reviewer:
judge whether the diagnosis is supported and whether the changes are proportionate; the author's
reasoning is stated here so it can be disagreed with.

**Request.** The user reported that workflow runs consume a growing share of the 5-hour quota window
and asked for an investigation of a month of Claude Code and Codex transcripts, then recommendations.
Five Sonnet subagents read partitioned scopes; the author verified their claims before acting.

**Findings (31 days, ~91 Claude main sessions).** 8,267 requests, 26.1M cache-creation and 1.02B
cache-read tokens; Codex adds a separate ~913M. All content ever read into context totalled ~5.6M
tokens, so context was re-sent roughly 180 times. Three drivers: (1) sessions never end — one ran
16.5h/339 requests to 553k context for 110M cache-read tokens, and cost grows with the square of
session length; (2) 62 of 84 sessions delegated nothing and account for 67% of main-chain tokens,
because discovery sweeps run in the orchestrator and pin their output in context; (3) mandated
reading is unconditional and unbounded — HANDOFF.md reached 138KB and was read up to 67 times in one
session, INVARIANTS.md was loaded in full "always" but revisited in only 8 of 43 sessions.

**Changes and why.** AGENT-WORKFLOW §1.1 makes the injected handoff header the read instead of a
prelude to reading the file. §1.3 makes delegating discovery a numbered step, since the same rule
already existed as a principle in CLAUDE.md/AGENTS.md and was not followed. §1.4 limits a run to one
unit and §§7–8 apply it to reviews and findings. §16.7 archives the state files at each release.
INVARIANTS.md gained a 15-row trigger index so a diff opens only the classes it touches; the file was
not split because 1,895 references point at it. The eight skill launchers carry matching steps in
both trees. `session-start.sh` now injects Current position and Active change and states that the
injection is the read. `guard-edit.sh` holds HANDOFF.md to 300 lines and BRIEFS.md to 400. This
handoff was cut 1,685 → ~300 lines, with the full prior file preserved in
[`../archive/state/HANDOFF-through-2026-09-03.md`](../archive/state/HANDOFF-through-2026-09-03.md).
AGENTS.md gained a Codex delegation section naming `spawn_agent` with `model: "gpt-5.6-luna"` and
`fork_turns: "none"`, because Codex's own instructions accept AGENTS.md as a source for those
overrides and reject them on a full-history fork.

**Reverted during the session.** A sweep budget in `guard-bash.sh` returned `permissionDecision:
"ask"` after 8 repository-wide searches. That overrides the permission mode, asked on every
subsequent match with no decay, and would block an unattended run. It was removed, not retuned: the
permission channel cannot nudge the model without interrupting the human. `guard-edit.sh`'s budget
has the same property but fires only when a state file is over budget.

**Open to challenge.** The 300-line budget is asserted, not derived. The one-unit-per-run limit is
untested against real sessions and may trade token cost for more handoff churn. The token weighting
uses API pricing (cache reads 0.1x, 1-hour writes 2x) as a proxy for subscription quota, which is not
published. Two author claims were wrong mid-session and corrected: that Codex spawns cannot take a
cheaper model, and that a fix run should stop after one finding rather than one review's findings.
This file is currently above its own 300-line budget; closing this review returns it.

## Changelog

Earlier entries are in the [archived handoff](../archive/state/HANDOFF-through-2026-09-03.md).

- **2026-09-03 — review (continuous post-`bd797bd` range through `636781b`; worktree fixes,
  unattended pipelines and fixes, automatic waiting-pane opening; INV §1–§15):** Reviewed the
  range in both directions against its FS/TS requirements. Three Worth-fixing findings are recorded
  below: older supported Git versions cannot answer the worktree common-directory query; a crashed
  agent can leave permission diagnostic entries behind for the process lifetime; and a batch of
  waiting transitions with tied millisecond timestamps can evict by agent insertion order rather
  than transition order. The focused 54-case CardGrid/SSE suite and presentation checks pass. The
  uncommitted workflow-efficiency files were preserved and remain the next independent review unit.

- **2026-09-03 — work (automatic pane opening on a waiting transition; FS-02.R61/A43, the narrowed
  FS-02.R51/A30, TS-08.R49; INV §1/§2/§10):** A chat agent that newly enters `waiting_input` now
  expands its own pane on the grid the person is looking at. Detection is one effect in `CardGrid`
  over the durable `state` each `state_update` carries — not the `notification` stream, which the
  per-type mute list filters and the server never replays — keyed against a per-grid ref of the last
  observed state that the hydration flags reseed rather than fire on, so only a newly observed
  transition opens anything. Eligibility reuses the grouped set the grid already renders, so a
  terminal agent, an out-of-project agent, and an agent in a collapsed section open nothing and the
  section is not expanded. Both the click path and the automatic opening now go through one
  `expandPane` helper, so R48's cap of four and its least-recently-used eviction cannot drift apart.
  A30's `waiting_input` clause moved to `done`, since R51 still holds for every other transition.
  FS-02 and TS-08 lost their last `(planned)` items and moved to `Current` in the spec index.
  `make test`, `make build`, the 372-case UI suite, `npm run build`, `npm run check:styles`, and
  `make check-specs` all pass. J5 gained the rendered checks jsdom cannot make: that an opening pane
  moves only the rows below it and that the eviction is visible rather than silent.

- **2026-09-03 — fix (unattended pipeline implementation findings; FS-03.R41/R43–R44/A24–A25,
  FS-04.R46/A26, FS-06.R28–R29/A19, FS-14.R53–R54/A29–A30, TS-02.R28, TS-03.R34–R35,
  TS-04.R43–R44, TS-08.R50, TS-09.R29–R31; INV §1/§2/§4/§5/§8/§10/§15):** Closed all five
  Must-fix and two Worth-fixing findings from the unattended-run implementation review. Same-revision
  permission attention now refreshes an open run page; concurrent approvals retain attention until
  all resolve; Stop cancels held approvals and timers before teardown; invalid on-disk message
  budgets default independently; auto-approval records its resolution before releasing the provider;
  every refused stage report carries shared retry guidance; and archived projects no longer receive
  dead-end Resume advice. Added focused regressions, including the rendered run state. `make test`,
  `make build`, the 368-case UI suite with presentation checks, focused race tests, `make dist`, and
  `git diff --check` pass. No specification changed because every fix restores existing requirements.

- **2026-09-02 — review (unattended pipeline implementation only; FS-03.R40–R44/A24–A27,
  FS-04.R46/A26, FS-06.R28–R29/A18–A19, FS-14.R52–R56/A29–A32, TS-01.R27, TS-02.R28,
  TS-03.R34–R35, TS-04.R41–R44, TS-05.R20, TS-08.R50–R52, TS-09.R29–R31; INV §1–§15):**
  Reviewed commit `a0b9e13` in both directions, with an independent Terra/high pass and without
  advancing the continuous reviewed-code marker past the intervening worktree-fix range. Seven
  findings are recorded below: five Must fix and two Worth fixing. The Must-fix set is a
  same-revision `pipeline_update` that the open run
  page deliberately ignores, Stop abandoning a pending permission and leaving its timer live,
  one run-level slot losing concurrent pending approvals, and tolerant message-budget config
  specified as a plain `int` so one non-numeric field makes the whole config unreadable, plus
  auto-approval releasing the provider before recording its resolution. The lower-priority set is
  transient stage-report refusals bypassing the new retry guidance and stopped pipeline-recipient
  wording that falsely promises Resume when the project is archived.

  Invariant sweep: §1 produced the live-attention and Stop findings; §2 produced the report-guidance
  finding while the registered-tool and assignment/refusal composition otherwise holds; §3's
  valid-config partial merge holds; §4 produced the pending-permission teardown finding; §5
  produced the concurrent-approval finding; §7's existing iteration and repair paths have no new
  surface; §8
  produced the report and recipient wording findings while permission logging, pause copy, and
  output bounds hold; §9's process-lifetime attention state is implicated by the Stop finding but
  produces no separate one; §10 produced the same-revision UI and whole-config fallback findings;
  §15 produced the Stop and auto-approval ordering findings. §11's new collection/string shapes and
  §13's new class names hold. §6 has no new interface/runtime, §12's live-provider identity shape
  remains the explicit acceptance gate, and §14 has no new route or widened authorization boundary. `make check-specs`, `make build`, both Go
  test variants, the focused runtime/pipeline/messaging race run, the 366-case UI suite with style
  and presentation checks, `make dist`, and `git diff --check` pass. The first sandboxed server test
  attempt could not bind a loopback port; the unchanged authorized rerun passed. No product code or
  specification changed.


## Decisions needing your input

These are product decisions needed for a future change or shipped boundaries whose reversal needs
an explicit specification update. Remove an item when the human resolves it or queues that update.

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether the chat should
  remain reachable with a wider continuation contract.
- **Refused card drag feedback:** Confirm whether the cross-block refusal should remain an in-flight
  pointer signal (FS-02.R53) or whether snap-back alone is the intended behavior. The shipped pointer
  implementation currently has an open wiring finding below.
- **Detecting a pipeline stage that can no longer advance:** A run parked at `await_result` with a
  silent stage agent is invisible today, and that cost the 2026-09-02 report about nineteen hours.
  Fixing it needs your call on what qualifies, because a stage agent legitimately ends many turns
  while its delegated children work: an idle stage agent with no running delegated tasks, an elapsed
  time threshold, or an explicit heartbeat from the stage. Also say whether the result is a new
  attention reason under FS-14.R29 — which puts it in the same notification category as blocked and
  crash — or a weaker run-page disclosure like FS-14.R40's delegated-agent count.
- **How long a stopped agent stays unaddressable after its pipeline stage:** FS-06.R22 excludes
  pipeline-associated agents from the addressable set while stopped, and the association is a
  `pipeline_attempts` row that lives as long as its run record. So an agent that ran one stage of a
  run that finished weeks ago can never be sent a message or a task again unless someone resumes it
  or deletes the run. The rule's stated reason — a pipeline stage agent was deliberately stopped by
  its state machine, so no message may revive it — only applies while that state machine still owns
  it. Confirm that the permanent version is intended, or scope the exclusion to a run that is still
  active, which changes FS-06.R22 and `stoppedWakeGates`.
- **Reactivating a worktree project after a consented checkout deletion:** FS-19 did not say what
  happens, and the review found its two paths answered in opposite ways. The smaller reading shipped
  on 2026-09-02 and is now stated in FS-19 §3: accepting the deletion ends AgentDeck's ownership, so
  restoring the project later gives an ordinary missing-directory error and nothing is recreated —
  recreating a checkout the person just chose to delete would undo their decision, and the branch is
  still there to fork again from. Confirm, or say that restore should re-materialize the checkout,
  which needs the ownership row to survive the deletion.

## Acceptance gates

Gates closed on or before 2026-08-30 are in the archived handoff.

- [ ] Run FS-03.A26/J14 with a pinned real chat provider: an AgentDeck stage-result action proceeds
  without a prompt, a file edit still prompts, and approval after more than three minutes continues
  the same stage. Automated exact-identity, fail-closed, no-default-deadline, and attention checks
  pass; this rendered provider boundary needs human authorization.
- [ ] Run pinned, credentialed Claude and Codex chat/MCP/resume checks before claiming those combinations.
- [ ] Run pinned Claude terminal flags/hooks and live xterm journeys before claiming full terminal support.
- [ ] Run pinned OpenCode/OpenHands launch/credential checks before claiming those backends beyond fakes.
- [ ] Run J2/J9/J16 in a real macOS browser to confirm the native folder panel opens in front,
  selects, and cancels (FS-04.A22). Narrowed on 2026-08-27: a real browser confirmed the **Browse…**
  controls are present and enabled for `cwd` and the pending `add_dirs` entry in both the Settings
  project form and the New project modal, and that the onboarding wizard renders styled. Only the
  native `osascript` panel itself is still unverified, and it needs a human at the machine.
- [ ] Drive a chat agent into a permission request with the dashboard on screen in a real browser
      and confirm its pane opens by itself, that only the rows below it move and no card changes
      column (FS-02.R55/A43, J5), that a reload with that agent still waiting opens nothing, and
      that a fifth waiting agent's eviction of the least-recently-used pane is visible rather than
      silent with the evicted draft returning on re-expansion. jsdom evaluates no layout, so the
      unit cases cannot close this.
- [ ] Drag a running card over the stopped block in a real browser and confirm the computed cursor
      on the card under the pointer states the refusal, clears when the pointer returns to its own
      block, and clears when the drag ends (FS-02.A35, J5). jsdom evaluates no CSS, so the unit
      cases cover only the marked state and the stylesheet rule.
- [ ] Run a task start, an assignment turn, and a reported result against the pinned Claude and Codex
      adapters before claiming dependent work works with real providers (FS-16 §6).
- [ ] Run one successful and one refused MCP tool call through pinned Claude and Codex adapters before
      claiming they accept structured tool results without losing the text block (FS-17.A6).
- [ ] Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude and
  Codex installations before promoting FS-08/TS-07 from Partial.
- [ ] Run J16's worktree steps in a real browser against a `make dist` build (FS-19.A1, FS-02.A42):
  the card-menu and scoped-header entry points, the pre-filled creation form, the new card appearing
  with its branch without a manual refresh, and an agent launched into the new checkout. The API
  half was driven end to end against a real repository with the built binary on 2026-09-02 — create,
  fork, setup bootstrap, dirty disclosure, declined archive, consented archive, surviving branch —
  and the component halves are covered by tests; only the rendered surface is unverified.
- [ ] Run FS-19.A4's manual gate: archive a worktree project holding uncommitted work in a real
  browser and confirm the dialog defaults to keeping, names the uncommitted state, and that
  accepting removes the checkout while the branch survives.
- [ ] Run the six-tab same-origin dashboard check against a `make dist` build (FS-02.A27). The
  transport half is now covered by `ui/src/api/sse.test.ts` and A27 has been narrowed to say so;
  the browser half has never been run against a build carrying the shared stream.
  `scripts/stress-fixture` (TS-06 §6) is the fixture.

## Blocked on human

Live-provider acceptance is waiting for human authorization because it invokes real provider sessions
and creates disposable local configuration homes. On 2026-07-15 this machine has Claude Code 2.1.202,
the retired `claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new
`claude-agent-acp`, OpenCode, and OpenHands are not installed globally.

## Review findings

- **Worth fixing** (FS-19.R1, TS-12.R1; INV §12) — `internal/worktree/git.go:158` always invokes
  `git rev-parse --path-format=absolute --git-common-dir`. Git versions before `--path-format`
  reject that option, so an otherwise usable repository cannot be forked even though AgentDeck's
  Git boundary specifies version tolerance and no minimum version. Retry with `--git-common-dir`
  when the option is unsupported and normalize the returned path locally; cover both argv forms
  with the Git command fixture.

- **Worth fixing** (FS-03.R43–R44, FS-14.R54, TS-09.R29; INV §1/§4/§9) —
  `internal/server/server.go:357` records every pending permission's tool name in
  `permissionTools`, but `internal/runtime/chat.go:890` abandons pending requests on an unsolicited
  transport close without emitting `permission_resolved`, the only path that deletes those keys.
  Repeated agent crashes while approval is pending therefore retain generation-scoped diagnostic
  entries for the server process lifetime. Clear the generation prefix from the runtime exit hook
  (alongside `ClearPermissionAttention`) and add a crash-with-pending-permission regression.

- **Worth fixing** (FS-02.R61/A43, TS-08.R49; INV §1/§10) —
  `ui/src/components/grid/CardGrid.tsx:157` orders a batched set of waiting transitions only by
  `AgentState.updated_at`, which is a millisecond wall-clock value. Five updates can share that
  timestamp; JavaScript's stable sort then preserves `agents` insertion order, not the observation
  order TS-08.R49 requires, so the wrong pane can be evicted under A43's five-at-once case. The test
  assigns distinct timestamps and cannot expose the tie. Carry a client observation sequence (or
  an equivalent deterministic event order) and add a tied-timestamp batch regression.

- **Worth fixing** (FS-16.R3/R4, TS-10.R15/R19; INV §15) — `internal/server/task_http_test.go:244`
  asserts the cancel response already carries `pending_release=false` and an empty runtime claim,
  but `finishInterruptedRelease` only clears them when its `StopStage` succeeds; a failed stop is
  specified to log and leave the release for recovery (TS-10.R19/R15). Observed once on 2026-08-31
  during a full `internal/server` run under load: the response carried `RuntimeClaim:created
  PendingRelease:true` and the case failed. It passes alone, twenty times under `-race`, and on a
  repeat full-package run, so it is a load-dependent flake rather than a new regression. Decide
  which side is wrong — either the cancel path owes a completed release before it answers, or the
  case should assert the recovery-completed state instead of the synchronous one — and record it in
  FS-16/TS-10 rather than loosening the assertion.

- **Must fix** (FS-03.R37/A20/A22, TS-08.R40; INV §10) —
  `ui/src/components/chat/renderers/AssistantText.tsx:25` memoizes the react-markdown component map
  on `[text]`, so the map is rebuilt on every streamed delta and React remounts the `MermaidDiagram`
  under it. The scroll case the fix targeted is closed; the live-stream case is not. Normal-use
  trigger: an assistant writes a closed ```mermaid fence and then keeps streaming explanatory prose,
  which is the ordinary shape of a diagram reply. Reproduced here against the shipped component —
  after the diagram settles, one appended delta drops the `<svg>` back to the source code block and
  re-invokes `mermaid.render` with a fresh id (`ad-diagram-1` then `ad-diagram-2`). That is exactly
  the reported "spazzing between display and source", and it contradicts R37's "the reader therefore
  never sees a diagram flicker or error mid-stream"; it also repeats uninterruptible main-thread
  Mermaid work per delta, which is the cost TS-08.R40 bounds the input to avoid. Note TS-08.R40's
  new sentence is scoped to "while message text is unchanged", so the technical spec currently
  ratifies the gap rather than closing it. Fix: hold `text` in a ref updated each render, read
  `textRef.current` inside the `code` component, and memoize with `[]`; then widen R40. Test: in
  `AssistantText.test.tsx`, rerender with `CLOSED + "\ntrailing prose"` after the diagram settles
  and assert the SVG is still mounted and `mermaid.render` was called once.

- **Worth fixing** (FS-02.R47/R55; INV §10) — `docs/specs/features/FS-02-dashboard.md:319` still
  requires that a collapsed card dragged past a pane "must see the pane's **two-column** footprint",
  and R55 supersedes only "R47's `min(2, perRow)` span" while asserting "every other clause of R47
  stands", which makes the stale clause normative. The shipped pane spans one track. TS-08.R43 was
  corrected in the same change to "a wider-or-taller footprint" and `CardGrid.tsx:121` to "one
  column"; FS-02.R47 was not. Normal-use trigger: a later reader trusts R47, concludes the grid is
  wrong, and reintroduces the two-track span R55 exists to remove. Fix: correct R47's footprint
  clause in place the way TS-08.R43 was corrected — the reason to keep the expanded id in its
  `SortableContext` still holds, because the pane is taller than its neighbours.

- **Worth fixing** (TS-08.R14/R48, FS-02.R59; INV §2) — `ui/src/components/grid/ContextBar.tsx:6`
  now emits `data-variant={compact ? "compact" : tone}`, so one attribute carries two orthogonal
  dimensions and the compact meter exposes no low/medium/high tone through the presentation
  contract. The tone survives only on the `context-bar high` className, which TS-08 §3.3 excludes
  from the skin surface. Normal-use trigger: a skin styles `[data-ui="context-meter"]
  [data-variant="high"]` red, and the expanded card's meter — the only context reading FS-02.R59
  leaves on the dashboard — silently keeps the default ramp. Nothing is visibly wrong today because
  no shipped skin reads this hook, which is why it is Worth fixing. TS-08.R48 says the compact form
  "differs from the full meter in presentation only", yet it drops a contract hook. Fix: keep
  `data-variant` as the tone and express density separately (a second `data-*` dimension registered
  in `contract.json`), and extend `ContextBar.test.tsx` to assert a compact meter still reports its
  tone through the contract attribute.

- **Must fix** (FS-14.R49/R50/A27, TS-09.R15/R16/R26; INV §5/§15) —
  `docs/specs/features/FS-14-configurable-pipelines.md:353` adds Reject and says that an approval
  which already consumed the proposal wins, but it never defines the opposite ordering. The shipped
  approval path commits the template file or run before it marks the proposal consumed
  (`internal/server/pipeline_handlers.go:58`, `internal/pipeline/manager.go:94`), and template
  approval deliberately carries no proposal id (TS-09.R26). Normal-use trigger: two tabs show the
  same pending offer; one rejects it, then the other's already-visible review flow saves or starts
  it anyway. The offer the person rejected still takes effect, and no simple post-mutation status
  update can undo that external effect safely. Fix the design before implementation: choose the
  Reject-versus-approval winner, specify a durable atomic claim plus crash/failure recovery across
  SQLite and the template/run mutation, define stale action behavior, and make A27 run the real
  interleavings and failure boundaries rather than only act on an already-consumed row.

- **Worth fixing** (FS-14.R49–R51, TS-02.R22, TS-03.R16/R17, TS-09.R23/R26;
  INV §7/§8/§9/§10/§11) — the product spec adds durable declined/deleted state, new mutations,
  proposal-update behavior, error/retry outcomes, timestamps, and fallback rows, but every governing
  technical spec still describes the shipped pending/consumed-only design. TS-02.R22 has only
  `consumed_at` and pending-only reads, TS-03.R16 exposes only `GET /api/pipeline-proposals`, and
  TS-09.R26 still says there is no dismissal action. No planned successor defines the schema and
  retention ordering, mutation routes/status codes, list shape, update publication, invalid-row
  isolation, non-null collections, or bounds; FS-14 §7 has no new traceability, and no ready-change
  file or design brief owns the implementation. Normal-use trigger: an implementer must invent these
  incompatible contracts, so persistence, server, and UI can each choose a different state model.
  Fix by completing TS-02/TS-03/TS-09 first, adding the ready change and traceability, and only then
  moving the design out of `docs/ideas.md`'s “being defined” section.

- **Worth fixing** (FS-14 status contract; INV §10) —
  `docs/specs/features/FS-14-configurable-pipelines.md:23` still says every requirement in the
  feature reflects shipped behavior, while this same diff makes the spec Partial and adds R49–R51
  and A27–A28 as planned. Normal-use trigger: a reader follows the feature's own scope statement and
  treats Reject/Delete/collapse as available now. Fix the introduction to distinguish shipped and
  explicitly planned requirements, as the status header already does.

- **Worth fixing** (FS-14.R51/A28; INV §8/§10) —
  `docs/specs/features/FS-14-configurable-pipelines.md:530` verifies collapse and bounded summaries
  only for one pending `save_template` proposal, although R51 governs both pending and declined
  records of both kinds. It also requires a `start_run` summary to name the template title, but that
  proposal's durable payload contains only `template_id` (`internal/pipeline/manager_types.go:16`),
  with no rule for whether a current, renamed, or deleted template supplies the title. Normal-use
  trigger: a declined or Start proposal regresses to the full-height payload, or shows a drifting/
  missing title, while A28 remains green. Fix R51's title provenance and fallback, then cover the
  pending/declined × Save/Start matrix (including a maximum-size record) in A28.

## Design consistency notes

- The change file cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 both cite `TS-04.R32–R39` and
  omit R40, the direct-action redaction clause. One range is wrong; the three should agree.
- FS-17 §6 opens with "The contract is shipped. Live-provider compatibility remains tracked as
  acceptance gate A6," which reads as covering the whole section, but §6 now also carries the planned
  direct-cutover boundary for R13–R19. Scope the opening sentence to R1–R12.
