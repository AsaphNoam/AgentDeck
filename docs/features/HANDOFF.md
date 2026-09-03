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
- **Last reviewed code:** `a6a8c0f` (2026-09-03), across the continuous range `e46e66b..a6a8c0f`.
  `3d474b3` was also reviewed on 2026-09-03, out of order and alongside a concurrent review of
  `5485cd7`, so it does not advance the marker. **Next review unit:** release commit `5485cd7`
  (concurrently under review); the remaining unreviewed range is `5485cd7..50c242e` less `3d474b3`.
  Once `5485cd7`'s review lands, the marker moves through `3d474b3`.
- **Open findings:** One **Worth fixing** item from the review of `3d474b3`: the injected **Release**
  bullet carries settled publication evidence that the changelog already holds. The generation race
  in crash registration teardown and the contradiction in FS-02.A43's acceptance evidence from the
  review of `3c1dc96` are fixed. The four workflow-efficiency findings from the review of `7c9ee44`
  and the earlier product-code review's eight findings are also closed.
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

**Next:** Independently review the next unreviewed unit beginning at `5485cd7`. The proposal
Reject/Delete design is fully specified and
waiting to start as [`decline-pipeline-proposals.md`](../ready-changes/decline-pipeline-proposals.md);
its ordering question is answered and must not be reopened during implementation. Keep the
credentialed Claude/Codex browser journey as an acceptance gate until a human authorizes real
provider sessions.

## Changelog

Earlier entries are in the [archived handoff](../archive/state/HANDOFF-through-2026-09-03.md).

- **2026-09-03 — review: release-record commit `3d474b3` (TS-06.R21; INV §1–§15):** Reviewed the
  handoff-only record of the published `v0.4.0` in both directions, against the live release rather
  than against the commit's own prose. Every factual claim holds: the tag resolves to `5485cd7`,
  release run `33722011193` succeeded on that head SHA, the archive, `install.sh`, and
  `manifest.json` are attached, the published manifest's `sha256` and size match GitHub's archive
  asset digest and size, `latest` resolves to `v0.4.0`, and the pre-release range `v0.3.0..a6a8c0f`
  is 35 commits as stated. The changelog's CI claim mirrors TS-06.R21 and §16.6 word for word and
  correctly names the credentialed Claude and Codex gates as owed rather than passed, which R21
  requires; the release workflow does run the FTS5-tagged release/CLI suites, the pinned-component
  and Apple-silicon checks, `TestVerifyArchiveRejectsCorrupt`, and the fresh-home install with an
  `AGENTDECK_HOME` sentinel. No user-visible behavior or architectural rule is introduced, so no
  FS/TS coverage is missing. One **Worth fixing** finding is recorded: the commit moved settled
  publication evidence into the injected Current position slice that the changelog entry already
  carries, contrary to §4 and §16.7, and states the run duration as 3m56s where GitHub reports 4m0s.
  The INV trigger index has no applicable surface in a documentation-only diff — no code, UI markup,
  runtime, persistence, HTTP, or external-CLI change — so §1–§15 are all stated as no-surface. No
  local-choice notes were outstanding. `make check-specs` and `git diff --check` pass. The reviewed
  marker does not advance: `5485cd7` sits between it and this unit and is under concurrent review.

- **2026-09-03 — review: handoff-only commit `a6a8c0f` (INV §1–§15):** Reviewed the review-marker
  correction in both directions. No finding: the commit's claims all hold. `7c9ee44` is the
  workflow-efficiency commit and `3c1dc96` the product fix, the stated order matches commit order so
  the reviewed range stays continuous, the marker was correctly left at `636781b` because a fix run
  does not advance it, and the statement it replaced was genuinely stale — it called `7c9ee44`'s work
  uncommitted one minute after that commit landed. Skipping the intervening `67651f6` is correct;
  that commit is state-only (`BRIEFS.md`, `HANDOFF.md`), not code. Later history followed the plan:
  `67df26d` reviewed `7c9ee44` and `82c94b5` reviewed `3c1dc96`. INV §10 was the only class with an
  applicable surface, and it shows one residual drift the commit left and later commits already
  resolved: at `a6a8c0f` the injected header simultaneously named `636781b` as last reviewed and, in
  **Active change**, called that same commit's code unreviewed and the next thing to review; HEAD
  carries neither statement, so it is recorded here rather than as an open finding. INV §1–§9 and
  §11–§15 had no applicable surface in a docs-only diff. No local-choice notes were outstanding. As
  a state correction, the previous "later commits through `4eebf31`" wording omitted `82c94b5` and
  `50c242e`; the remaining unreviewed range is now named as `5485cd7..50c242e`. `make check-specs`
  and `git diff --check` pass.

- **2026-09-03 — fix: crash/resume registration race and dashboard acceptance evidence (TS-01.R16,
  TS-04.R7, FS-02.A43, TS-08.R49; INV §4/§5/§10):** Unsolicited exit teardown now waits for the
  shared per-agent lifecycle claim and rechecks the live registry generation before removing
  agent-keyed hook and MCP artifacts, so a delayed generation N exit cannot revoke generation
  N+1's registration. A deterministic overlapping-claim regression preserves the newer token and
  files; focused server/runtime tests, the focused race run, the full Go suite in both build modes,
  the specification checker, and the product build pass. FS-02.A43 now names the store's
  `observed`/`observedSeq` ordering index and retains its single-writer requirement. Both findings
  from the review of `3c1dc96` are closed.

- **2026-09-03 — review: product-fix commit `3c1dc96` (TS-01.R16, TS-04.R7, FS-02.A43,
  TS-08.R49; INV §1–§15):** Reviewed the Git-version fallback, pending-permission cleanup, and
  tied-timestamp pane ordering in both directions against their requirements. Two findings are
  recorded. The crash exit handler admits a generation under the registry lock, releases that lock,
  then runs agent-id-only registration teardown; a resume can register the next generation in that
  window and have its hook token, MCP identity/file, and hook settings deleted by the old exit.
  Separately, FS-02.A43 still says `agentStore` gains no field even though this fix adds and TS-08.R49
  requires its observation index. INV §1, §2, §4, §5, §9, §10, and §12 had applicable surfaces;
  §3, §6, §7, §8, §11, §13, §14, and §15 had none. The full Go suite in both build modes, the
  full UI build and presentation checks, focused CardGrid/store/worktree/server cases, and the
  permission-teardown race case pass. The continuous reviewed marker advances through `3c1dc96`;
  the remaining unreviewed range begins at `a6a8c0f`.

- **2026-09-03 — fix: workflow-efficiency review findings (INV §7/§10):** Closed all four
  Worth-fixing findings from the review of `7c9ee44`. The edit guard now normalizes lexical `.` and
  `..` segments before checking generated, cache, and handoff paths, including Write targets that do
  not exist yet; traversal-form fixtures cover relative and absolute paths. The Codex transcript
  audit now reports and skips decoded non-object records and session metadata with non-object
  payloads, then continues to valid later records. The UI closure commands run in subshells so the
  repository working directory survives each line. Every state-writing Claude and Codex role
  launcher now requires startup inspection of `git status` and the existing diff before an edit,
  while a check wired into `make check-specs` enforces startup, commit, and close rules across both
  launcher trees. No FS/TS change was needed: these fixes restore existing workflow and read-path
  contracts. Hook and transcript fixtures, shell syntax, Python compilation, the launcher contract,
  twin-skill comparison, `make check-specs`, the full UI suite and build from the documented root
  sequence, and `git diff --check` pass.

- **2026-09-03 — review: workflow-efficiency commit `7c9ee44` (INV §1–§15):** Reviewed the
  documentation, mirrored launchers, Claude hooks, and transcript-audit utility in both directions
  against the workflow contract. Four Worth-fixing findings are recorded: lexical paths bypass the
  edit guard, a valid non-object JSONL record aborts the Codex audit, the documented UI closure
  commands retain the changed directory across lines, and several role launchers omit the startup
  dirty-tree check. INV §4, §7, §8, and §10 had applicable surfaces; the other indexed classes had
  no matching surface. The hook and transcript fixtures, shell syntax, Python compilation,
  twin-skill comparison, `make check-specs`, and `git diff --check` pass. Concurrent proposal-design
  edits briefly interrupted the closure rerun; after they settled, the check passed, and those edits
  remain preserved and excluded from this review commit. The continuous reviewed marker advances
  only through `7c9ee44`; `3c1dc96` remains unreviewed as the next unit.

- **2026-09-03 — fix: the proposal Reject/Delete design, on the user's decision (FS-14.R57, A27;
  TS-02.R29, TS-03.R36, TS-09.R32; INV §5/§10/§11):** The user chose the recommended ordering, which
  closes the last two findings of the 2026-09-03 product-code review.

  **The ordering.** FS-14.R57 states it in one place: an approval that commits always beats a
  Reject, because an approval's effect is external — a template file written, a run started with
  agents launched — and no later status update can withdraw it safely. Reject withdraws the offer,
  never the mutation, which is the same rule R49 already stated from the other side and is
  consistent with R50's rule that declining is not a standing block on that content. A record an
  approval consumes leaves both the pending and the declined list as consumed. Each of Reject,
  Delete, and approval claims the record with one conditional durable write (INV §5), so a race has
  exactly one effect and every loser is told the state the row is actually in. The accepted failure
  mode is stated rather than engineered around: a crash between a committed mutation and its
  consumption mark leaves one listed offer for content that already exists, which R50's re-arm
  resolves on the next identical proposal.

  **The technical design the review found missing.** TS-02.R29 adds `declined_at`, the three record
  states, the conditional claim each transition uses — consumption matching `consumed_at IS NULL`
  alone is what lets an approval overwrite a decline — and keeps retention state-blind. TS-03.R36
  adds the decline and delete routes, the two-collection non-null list shape that replaces the
  pending-only body in UI lockstep, the typed refusals for each losing state, and reuses the existing
  `pipeline_proposal_update` refetch signal. TS-09.R32 fixes the control-plane boundary: these are
  pure record operations with no agent-facing effect, and the approval paths keep the shape R26 gives
  them — no proposal id through their API, no pre-check against a decline, no cross-store
  transaction. TS-02 and TS-09 are now Partial, since both carry planned items. A27 gained the real
  interleavings, the concurrent-loser cases, and the leftover-offer failure mode.

  **State.** FS-14 §7 carries the traceability, the idea left `docs/ideas.md` for the ready change
  `decline-pipeline-proposals.md`, which records that the ordering must not be reopened during
  implementation. Documentation-only: `make check-specs` and `git diff --check` pass. The four
  workflow-efficiency findings from `7c9ee44` are a different review's unit and stay open.

- **2026-09-03 — fix: the 2026-09-03 review's findings (INV §2/§10/§15):** Seven of eight closed
  here; the eighth was answered by the user and closed in the entry above.

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

  **Raised for the user.** The FS-14 Reject-versus-approval **Must fix** was a product decision, put
  to the user with both readings and a recommendation. `make test`, `make build`, the UI suite, the
  UI build, the presentation-contract check, and `git diff --check` pass.

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

- **Worth fixing** — `HANDOFF.md` *Current position*, the **Release** bullet (from `3d474b3`;
  workflow §4 and §16.7; no INV class applies). The bullet restates the publication evidence — run id, "in
  3m56s", the manifest/asset digest match, `latest` resolution — that the `v0.4.0` changelog entry
  below already carries, and the text it replaced deliberately pointed at that entry instead
  ("Publication state is recorded in the changelog entry below"). Normal-use trigger: every session
  pays for the injected Current position slice, and evidence about a finished release is settled
  history rather than state a cold agent needs for the next change; §16.7 names that slice as the
  recurring cost. The duration is also slightly wrong — GitHub reports the run as 4m0s. Fix: cut the
  bullet to the live facts (v0.4.0 published and verified on tag `5485cd7`, credentialed
  Claude/Codex checks still owed under TS-06.R21, `agentdecker` deliberately unmigrated under
  FS-04.R44) and leave the evidence in the changelog. No test applies; `make check-specs` covers the
  file mechanically.

## Design consistency notes

- The change file cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 both cite `TS-04.R32–R39` and
  omit R40, the direct-action redaction clause. One range is wrong; the three should agree.
- FS-17 §6 opens with "The contract is shipped. Live-provider compatibility remains tracked as
  acceptance gate A6," which reads as covering the whole section, but §6 now also carries the planned
  direct-cutover boundary for R13–R19. Scope the opening sentence to R1–R12.
