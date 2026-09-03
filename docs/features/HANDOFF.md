# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Open the other sections only when those point at them. Settled state is
archived in [`../archive/state/HANDOFF-through-2026-09-03.md`](../archive/state/HANDOFF-through-2026-09-03.md)
and [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only and is cut
back to that at every release (§16.7). Injected Current position plus Active change budget: 8 KiB.

## Current position

- **Active change:** None. The last two changes are finished and verified: automatic pane opening on
  a waiting transition (FS-02.R61/A43, the narrowed FS-02.R51/A30, TS-08.R49) and unattended
  pipeline runs (FS-03.R40–R44/A24–A27, FS-04.R46/A26, FS-06.R28–R29/A18–A19, FS-14.R52–R56/A29–A32,
  TS-01.R27, TS-02.R28, TS-03.R34–R35, TS-04.R41–R44, TS-05.R20, TS-08.R50–R52, TS-09.R29–R31).
  Earlier finished work is in the archived handoff; the named FS/TS requirements are the authority
  on what shipped.
- **Release:** `v0.4.0` is published and verified. The tag is on `5485cd7`, `main` is pushed through
  the same commit, and release run `33722011193` succeeded in 3m56s: the archive, `install.sh`, and
  `manifest.json` are attached, the manifest's `sha256` matches GitHub's asset digest, and `latest`
  resolves to `v0.4.0`. It was cut over the 35-commit range from `v0.3.0` at the user's explicit
  direction to release with two **Must fix** findings still open, which workflow §16.1 otherwise
  blocks on. The credentialed Claude and Codex checks are not covered by release CI and remain owed
  (TS-06.R21). A customized
  `agentdecker` role is deliberately not migrated (FS-04.R44), so it keeps the superseded product
  manual beside the current skill; nothing user-facing says so.
- **Last reviewed code:** `636781b` (2026-09-03), across the continuous range `e46e66b..636781b`.
  **Next review unit:** workflow-efficiency commit `7c9ee44`; product-fix commit `3c1dc96` follows
  as a second unit. Both shipped in `v0.4.0` unreviewed.
- **Open findings:** eight, listed below — two **Must fix** (streamed Mermaid remount; FS-14 Reject
  versus approval ordering) and six **Worth fixing**. Both **Must fix** items shipped in `v0.4.0`.
  The 2026-09-02 bug investigation's one remaining **Must fix** is the silent-stage product decision
  under *Decisions needing your input*. The task-cancel release flake is open as a **Worth fixing**
  finding.
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

**Change:** None.

**Next:** Independently review the automatic-pane-opening implementation and the unattended-pipeline
fix — `7c9ee44` first, then `3c1dc96`. After that, the two open **Must fix** findings are the
highest-value `/fix` unit, since `v0.4.0` shipped with both. Keep the credentialed Claude/Codex
browser journey as an acceptance gate until a human authorizes real provider sessions.

## Changelog

Earlier entries are in the [archived handoff](../archive/state/HANDOFF-through-2026-09-03.md).

- **2026-09-03 — release `v0.4.0` (FS-18.R1/R4/R5, TS-11.R1/R8; FS-19, FS-03.R40/R43):** Cut the
  35-commit range `v0.3.0..main`. Minor rather than patch because the range adds user-visible
  capability: worktree projects, unattended pipeline runs, active-project navigation tabs, and
  automatic pane opening on a waiting transition. Released at the user's explicit direction with
  two **Must fix** findings still open, which §16.1 would otherwise block on; both are named in
  *Current position* and neither is a data-loss risk.

  Operator-package refresh: the range's earlier package edit (`a0b9e13`) had already carried the
  configured mail budget and the accepted-result attempt boundary, so two gaps remained and both
  landed in `references/operate-agents.md` under TS-11.R8's lifecycle/interface/project-resource
  ownership. First, a project's working directory may be an AgentDeck-owned Git worktree, so
  parallel isolation means another project rather than another directory, and the checkout is
  disposable while the branch and identities are durable (FS-19.R1/R4/R5/R7/R8). Second, calling
  one of AgentDeck's own agent-facing tools never waits for a human approval, while every other
  tool keeps its gate and, absent a configured deadline, now waits indefinitely rather than
  auto-denying (FS-03.R40/R43) — stated as a category, since R8 excludes a registration inventory.
  FS-17.R13–R20 are `(planned)` and correctly stayed out.
  Nothing else in the range was agent-facing: no tool was added or removed, and the retry-class
  extraction into `internal/toolresult` changed no classification an agent observes.

  Nothing falsified the release-matched claims: `README.md`, `install.sh`, `scripts/release/`, and
  `.github/workflows/` are byte-identical across the range, and the pinned Node, ACP adapter, and
  Codex components are unchanged. `make test`, `make check-specs`, `make dist VERSION=0.4.0` (the
  binary reports `0.4.0` and carries `sqlite_fts5`), and `git diff --check` pass. Release run
  `33722011193` then verified archive contents, FTS5 tagging, pinned components, checksum
  rejection, and a fresh installation on an Apple-silicon runner. The credentialed Claude and Codex
  checks are not covered by that run and remain owed (TS-06.R21).

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
