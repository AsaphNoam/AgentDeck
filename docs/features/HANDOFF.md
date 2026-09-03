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
- **Open findings:** one, listed below — the **Worth fixing** completion of TS-02/TS-03/TS-09 for
  FS-14's proposal Reject/Delete design, which is blocked on the Reject-versus-approval ordering
  decision. The 2026-09-03 review's other seven findings were fixed on 2026-09-03; the
  Reject-versus-approval **Must fix** became a product decision and moved to *Decisions needing your
  input*, beside the 2026-09-02 investigation's silent-stage decision. Both of the **Must fix**
  items `v0.4.0` shipped with are now closed as code: the streamed Mermaid remount is fixed, and
  the FS-14 one governs `(planned)` behavior that nothing shipped.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  remain owed before claiming those adapters accept structured results.
- **Usability state:** The Pipelines pages and dashboard grid were driven through a real Chromium on
  2026-08-30 against a `make dist` build of the shipped tree; that run is closed except for the
  items still owed here. Owed: A25's stage-boundary wording (needs a live report cycle `fakeacp`
  cannot drive), A18's consumption on approval, A32's unknown-agent and cross-project id cases, and
  the refused-drag pointer's computed-cursor pass (FS-02.R53), which is an acceptance gate below
  rather than a finding. Full run:
  [`usability-review-run-2026-08-30-new-pages.md`](../archive/reviews/usability-review-run-2026-08-30-new-pages.md).
- **Branch:** `main`.

## Active change

**Change:** None.

**Next:** Independently review the automatic-pane-opening implementation and the unattended-pipeline
fix — `7c9ee44` first, then `3c1dc96`. FS-14's proposal Reject/Delete design is the next design unit
once the human answers the Reject-versus-approval ordering question under *Decisions needing your
input*; nothing should implement R49–R51 before that answer and the TS-02/TS-03/TS-09 completion it
gates. Keep the credentialed Claude/Codex browser journey as an acceptance gate until a human
authorizes real provider sessions.

## Changelog

Earlier entries are in the [archived handoff](../archive/state/HANDOFF-through-2026-09-03.md).

- **2026-09-03 — fix: the 2026-09-03 review's findings (INV §2/§10/§15):** Seven of eight closed;
  the eighth is blocked on a product decision.

  **Must fix, streamed Mermaid remount (FS-03.R37/A20, TS-08.R40; INV §10).** `AssistantText`
  memoized react-markdown's component map on the message text, so every streamed delta after a
  closed ```mermaid fence rebuilt the map, remounted `MermaidDiagram`, dropped the settled SVG back
  to source, and re-ran main-thread Mermaid work. The map now reads the current text through a ref
  and memoizes on `[]`, so it stays stable for the life of a mounted message. TS-08.R40's stability
  clause was scoped to "while message text is unchanged" — which ratified the gap — and now covers
  streamed deltas; FS-03.A20 gained the delta case, reproduced first as a failing test.

  **Cancel-response release flake (FS-16.R3/R4, TS-10.R15/R19; INV §15).** The specifications
  already decide this: the stop follows the terminal commit and cannot be transactional, so a
  refused stop leaves the durable intent standing for recovery. `task_http_test.go` asserted the
  synchronous cleared state off the cancel response, which is a guarantee no requirement makes —
  the shipped race is a cancel landing while the task's own launch still holds the lifecycle claim,
  so `StopStage` returns "a lifecycle transition is already in progress". The case now asserts the
  terminal state and that no claim survives without its intent, then drives the recovery backstop to
  the released state; twenty consecutive runs pass. No specification or product change: R4 and R19
  already state it. Worth the human's attention separately — nothing but a restart currently retries
  a release whose stop was refused.

  **Meter tone versus density (TS-08.R14/R48, FS-02.R59; INV §2).** `ContextBar` carried two
  orthogonal dimensions on one `data-variant`, so a compact meter exposed no low/medium/high tone
  through the presentation contract. `data-variant` is the tone in both forms now and the compact
  form is a `context-meter` state in `contract.json`, with the stylesheet, the visual-matrix case,
  and the card case following; TS-08.R48 records why the dimensions stay separate.

  **Specification corrections (INV §10).** FS-02.R47's drag clause still required a pane's
  "two-column footprint" after R55 narrowed the span to one track, which would have invited someone
  to reintroduce the span R55 removed; it now reads as TS-08.R43 was already corrected. FS-14 §2 no
  longer claims every requirement is shipped, which contradicted its own Partial status and its
  `(planned)` R49–R51. FS-14.R51 now says where a `start_run` summary's template title comes from —
  the template as it stands now, naming the id and saying the template is gone once it is deleted —
  because that proposal's durable payload carries only `template_id`, and A28 covers the
  pending/declined × Save/Start matrix at maximum record size rather than one pending Save.

  **Left open.** The FS-14 Reject-versus-approval **Must fix** is a product decision and moved to
  *Decisions needing your input* with both readings and a recommendation; the **Worth fixing**
  completion of TS-02/TS-03/TS-09 for that design stays a finding, because it cannot be written
  before the answer. `make test`, `make build`, the UI suite, the UI build, the presentation-contract
  check, and `git diff --check` pass.

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
  implementation is still owed its real-browser computed-cursor pass, which is an acceptance gate below.
- **Rejecting a proposal versus approving it (FS-14.R49/R50/A27, TS-09.R15/R16/R26; INV §5/§15):**
  FS-14 §4.4 defines only one ordering — an approval that already consumed the proposal beats a
  later Reject — and never says what happens the other way round. The case is two tabs on the same
  pending offer: one rejects it, and the other's already-open review flow then saves the template or
  starts the run. Today's approval path would let that through, because it commits the template file
  or run before marking the proposal consumed and template approval deliberately carries no proposal
  id (TS-09.R26), so the offer the person rejected still takes effect and no post-mutation status
  update can undo an external effect safely. Your call, because both readings are defensible:
  (a) **the mutation always wins** — Reject only withdraws the offer, a losing Reject reports the
  proposal as consumed exactly as R49 already says for the other ordering, and this stays consistent
  with R50's rule that declining is not a standing block on that content; or (b) **the first action
  to claim the record wins** — the approval must durably claim the proposal before it commits, which
  means routing a proposal id through the template and start APIs and reversing TS-09.R26's design.
  Recommendation: (a), because it needs no new claim across two stores and keeps one meaning for a
  person's deliberate save. Either answer then needs the durable claim or ordering rule written down
  with its crash and failure recovery, the behavior of a stale action, and A27 exercising the real
  interleavings rather than only an already-consumed row. Nothing is shipped: R49–R51 are
  `(planned)`, and the open **Worth fixing** finding gates implementation on this answer.
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

## Design consistency notes

- The change file cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 both cite `TS-04.R32–R39` and
  omit R40, the direct-action redaction clause. One range is wrong; the three should agree.
- FS-17 §6 opens with "The contract is shipped. Live-provider compatibility remains tracked as
  acceptance gate A6," which reads as covering the whole section, but §6 now also carries the planned
  direct-cutover boundary for R13–R19. Scope the opening sentence to R1–R12.
