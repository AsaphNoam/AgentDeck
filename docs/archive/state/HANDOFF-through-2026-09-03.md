# AgentDeck — handoff state through 2026-09-03

Archived at the `v0.3.0` epoch under AGENT-WORKFLOW §16.7. This is the full handoff as it stood on
2026-09-03, before the live file was cut back to resumable state. Everything here is settled history:
the live file is [`../../features/HANDOFF.md`](../../features/HANDOFF.md). Earlier phase state is in
[`HANDOFF-pre-sdd.md`](HANDOFF-pre-sdd.md).

---

# AgentDeck — Implementation handoff

**Live agent state.** Read this first, then open the relevant requirements named below. Historical
phase state is archived in [`HANDOFF-pre-sdd.md`](HANDOFF-pre-sdd.md).
Follow [`AGENT-WORKFLOW.md`](../../features/AGENT-WORKFLOW.md) and keep this file limited to resumable current state.

## Current position

- **Release:** `v0.3.0` is published. The tag is on `95136ce`, `main` is pushed through `89014a5`,
  and release run `33296747765` succeeded in 3m23s: the archive, `install.sh`, and `manifest.json`
  are attached, the manifest's `sha256` matches GitHub's asset digest, and `latest` resolves to
  `v0.3.0`. The `v0.2.3..v0.3.0` range was read for agent-facing change and the embedded
  `operating-agentdeck` package needed none, because it shipped in that range's last commit and
  already states FS-14.R47's stage boundary; the README layout block was corrected to name
  `cache/agent-skills/`. The credentialed Claude and Codex checks are not covered by that run and
  remain owed (TS-06.R21). A customized `agentdecker` role is deliberately not migrated (FS-04.R44),
  so it keeps the superseded product manual beside the current skill; nothing user-facing says so.
- **Bug investigation:** Five of the six 2026-09-02 unattended-pipeline-run findings are closed by
  the unattended-runs implementation. Retryable report refusals, stopped pipeline-agent recipient
  wording, permission waits/logging, pause guidance, and declared-output presentation now have
  regression coverage, and the unattended-runs implementation review findings are closed. The
  remaining **Must fix** is the separate product decision below: what
  qualifies a stage agent that can no longer advance when no permission request is pending.
  The 2026-08-28 pipeline `stale_assignment` report is closed. Its three
  findings are all fixed: the refusal code was corrected earlier, the boundary is now stated to the
  stage agent in the assignment, the accepted result, and the refusal (FS-14.R47), the restart pause
  no longer offers a dead-end **Open agent** (FS-14.R48), every refusal is logged with the fields
  that separate its conditions, and `OnTurnEnd`/`OnExit` re-read the run under the run lock. The
  product decision the first finding needed was taken as the smaller of the two named options —
  state the boundary rather than build an in-chat continuation route, and withhold the dead-end
  action rather than widen Continue — and is flagged in the human update for confirmation. The
  earlier 2026-08-27 all-200/no-page-load incident stays fixed, and the shared SSE stream is now
  replayed to a joining tab instead of restarted for every tab.
- **Review state:** The continuous `57b7154..bd797bd` range is reviewed against its requirements and
  every invariant class. The worktree-project implementation (`1b2a8c3..bd797bd`) had a dedicated
  implementation review plus an independent Terra/high pass, and all ten findings from that review
  were fixed in `3e1726b`; the post-fix code is still awaiting independent review, so the last
  reviewed-code marker remains `bd797bd`. The unattended-pipeline implementation commit `a0b9e13`
  has now had its dedicated implementation review plus an independent Terra/high pass; its five
  Must-fix and two Worth-fixing findings are fixed. Because that review named one commit
  and did not cover the intervening worktree-fix range, it does not advance the continuous marker.
  The 2026-09-02 bug investigation retains one open
  **Must fix** against general silent-stage detection; the other five findings are closed. The
  FS-14 proposal-decline design, now committed in `e83c937`, also had a
  dedicated worktree-only review plus an independent Terra/high pass. The earlier design review did
  improve this implementation: ownership is recorded last, pipeline validation stays read-only,
  base fallback uses the main worktree, recreation is serialized, and human-facing setup output is
  clamped. `1b2a8c3` deleted four
  whole sections of this file — decisions, acceptance gates, blocked-on-human, and review findings —
  and they are restored; a later restore of the same sections duplicated the tail, which the
  worktree commit de-duplicated. The task-cancel release flake stays open. The refused-drag pointer
  needs a real-browser computed-cursor pass and both the six-tab shared-stream check and FS-19's two
  browser halves remain acceptance gates. Three behavior choices — two from an earlier fix, one from
  the worktree implementation — still need human confirmation below.
- **Active change:** None. Unattended pipeline runs is finished and verified (FS-03.R40–R44/A24–A27,
  FS-04.R46/A26, FS-06.R28–R29/A18–A19, FS-14.R52–R56/A29–A32, TS-01.R27, TS-02.R28,
  TS-03.R34–R35, TS-04.R41–R44, TS-05.R20, TS-08.R50–R52, TS-09.R29–R31). Worktree projects are shipped and verified (FS-19, TS-12, FS-04.R45/A25, FS-02.R60/A42, TS-01.R26, TS-02.R27, TS-03.R33). Stable, decluttered agent cards are shipped and verified (FS-02.R55–R59/A37–A41, FS-12.R40/A16, TS-08.R45–R48). Active-project navigation tabs are shipped and their review findings are
  closed (FS-02.R53/A35, FS-12.R39/A15, TS-08.R44). Thin AgentDecker and the shared AgentDeck operating skill are finished
  and verified (FS-18, FS-04.R44/A24, TS-11). Expandable dashboard chat panes are finished and verified
  (FS-02.R46–R52/A29–A34, FS-03.R39/A23, FS-12.R38/A14, TS-03.R31, TS-08.R41–R43).
  Running-first card placement shipped on 2026-08-28 (FS-02.R45/A28, FS-12.R37/A13). The Pipelines
  surface
  split is finished and committed (`9114df7`, with its usability fixes in `69c2f99`) and is ready
  for an independent review; its change file is removed, and FS-14 is the authority on what
  shipped.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  remain owed before claiming those adapters accept structured results.
- **Usability state:** The new Pipelines pages and the changed dashboard grid were driven through a
  real Chromium on 2026-08-30 against a `make dist` build of the shipped tree (no product code
  differs between that build point and `v0.3.0`). The three fixes that had never been in a browser —
  attempt badge on the timeline rule, run rail stacking after the timeline at the 1024 floor, and
  the one-shot entrance on an appended timeline entry with reduced motion removing it — all pass.
  So do A14, A18's cross-destination counts, A19, A20 on the 32-stage template, A22, A24 and A26.
  J5's owed items are now closed: running-first placement, the live start/stop boundary crossing in
  both directions, in-drag geometry inside one block, and the refused cross-block drop were all
  driven, along with the pane cap, its least-recently-used eviction, the evicted pane's draft, pane
  persistence, the terminal-card exemption, and the pane name link. Three of its four findings are
  closed; the attempted pointer treatment for the refused cross-block drag was reopened by code
  review because the card under the pointer overrides it (FS-02.R53).
  Still owed: A25's stage-boundary wording (needs a live report cycle `fakeacp` cannot drive),
  A18's consumption on approval, and A32's unknown-agent and cross-project id cases. The earlier
  v0.2.2 → v0.2.3 delta remains closed: FS-02.A24 is closed and FS-04.A22 remains narrowed to the
  native panel. Full run:
  [`usability-review-run-2026-08-30-new-pages.md`](../reviews/usability-review-run-2026-08-30-new-pages.md).
- **Last reviewed code:** `bd797bd` (2026-09-02). Advanced across the continuous range
  `e46e66b..bd797bd`; the implementation portion `1b2a8c3..bd797bd` was read end to end against
  FS-19.R1–R11/A1–A7, FS-04.R45/A25, FS-02.R60/A42, TS-01.R26, TS-02.R27, TS-03.R33,
  TS-12.R1–R10, and every invariant class. The later `0b70f02` review-state commit contains no
  product code and does not advance this code marker.
- **Branch:** `main`.

## Active change

**Change:** None. Unattended pipeline runs is finished; the change file is removed and the named
FS/TS requirements are the authority on what shipped.

**State:** AgentDeck-owned MCP actions are exact-match auto-approved through a runtime-only overlay;
ordinary permissions wait without a default deadline and derive pipeline attention. Message budgets
default to 50 and are configurable per turn. Retryable report refusals, stopped pipeline-agent
recipient errors, pause choices, attempt outputs, and finished named values now explain themselves.

**Next:** Independently review the unattended-pipeline fix. Keep the credentialed Claude/Codex
browser journey as an acceptance gate until a human authorizes real provider sessions.

## Changelog

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

- **2026-09-02 — work (unattended pipeline runs; FS-03.R40–R44/A24–A27, FS-04.R46/A26,
  FS-06.R28–R29/A18–A19, FS-14.R52–R56/A29–A32, TS-01.R27, TS-02.R28, TS-03.R34–R35,
  TS-04.R41–R44, TS-05.R20, TS-08.R50–R52, TS-09.R29–R31; INV §1/§2/§5/§8/§9/§10/§11/§13/§14):**
  Shipped unattended pipeline control. The fifteen AgentDeck MCP identities are derived from the
  actual tool registrations, carried on the shared non-persisted lifecycle overlay, exact-matched
  against the adapter title, and answered only with `allow_once`; all other tools fail closed into
  the ordinary gate. The default permission deadline is off, denied/cancelled/timed-out decisions
  are logged, and generation-scoped permission events derive one edge-triggered pipeline attention
  notification without changing run state or persistence. Messaging defaults to 50 combined actions
  and freezes the configured limit per turn; stopped pipeline-agent refusals name resume. One shared
  refusal vocabulary now ties retry class to report guidance, assignments use the accepted-result
  boundary, and both field reproductions are active. The run page explains Continue versus Retry,
  renders attempt outputs, and opens named values for finished runs. Specs are current, the packaged
  operating guidance matches, focused race tests pass, and the complete Go/UI/spec/build/dist checks
  are green. The credentialed real-provider browser journey remains an explicit acceptance gate.

- **2026-09-02 — feature design (unattended pipeline runs; FS-03.R40–R44/A24–A27, FS-04.R46/A26,
  FS-06.R28–R29/A18–A19, FS-14.R52–R56/A29–A32, TS-01.R27, TS-02.R28, TS-03.R34–R35, TS-04.R41–R44,
  TS-05.R20, TS-08.R50–R52, TS-09.R29–R31; INV §1/§2/§5/§8/§9/§10/§11/§13/§14):** Designed the
  change that stops a pipeline run from needing a person as its control plane, originally queued as
  `unattended-pipeline-runs.md` and now finished.
  Started from the user's own idea (too many MCP action approvals) and absorbed four of the six
  `806bcb7` bug-investigation findings; the source idea is removed from `docs/ideas.md`.

  The core is FS-03.R40: AgentDeck's own fifteen MCP actions are exempt from the approval gate for
  every agent, whatever `skip_permissions` was frozen at launch, because the prompt was never what
  authorized them — each already crosses the loopback boundary with a per-agent generation token and
  is authorized server-side against that agent's identity (TS-05.R20). `create_task` is included by
  explicit user decision. Identification is an exact match on a `mcp__<server>__<tool>` identity
  composed once from the registered MCP server itself (TS-04.R41, never a second hand-kept list) and
  fails closed (R42). Two verifications the design rests on, both read out of the pinned adapters
  rather than assumed: `claude-agent-acp` 0.59.0's `toolInfoFromToolUse` default branch sets
  `title` to the raw tool name, and its ACP mode accepts no `--settings` file, so a provider-side
  allowlist is unavailable and the gate must live in AgentDeck's runtime. The adapters'
  always-allow option was examined and **rejected**: `codex-acp` offers "Allow for This Session" and
  "Allow and Don't Ask Again" under one ACP kind, so a client choosing by kind cannot tell a session
  rule from a persisted one (TS-04.R43); the exemption answers single-use.

  FS-03.R43 removes the 180s auto-deny by default. That alone would trade a stage that fails in
  three minutes for a run that waits in silence, which is the 2026-09-02 report's own failure mode,
  so it ships with FS-14.R54: a run whose stage agent holds an unanswered approval carries an
  attention reason and joins R29's needs-attention category. The user chose that over a
  disclosure-only treatment. Architecturally it is a **derived** read on the fan-out that already
  carries `turn_end` (TS-09.R29–R30) — no new column, no migration, no run revision or transition
  affected, edge-triggered once so a reconnect cannot restage a toast, and nothing to resurrect on
  restart because a pending request dies with the process. FS-03.R44 logs every permission outcome
  that withheld a tool, which is the half of the auto-deny finding R43 does not close: the reported
  run was only reconstructible because the person watched it happen.

  Of the four folded-in findings, all four were judged to specify **new** behavior rather than
  restore specified behavior, and each says so: FS-14.R53 (the assignment implements R47 faithfully,
  so R47's "one call" clause is the defect and is marked superseded in place), FS-06.R29 (R22's
  exclusion is specified, its message is not), FS-14.R55 (R46 requires the explain-yourself pattern
  for the start dialog only), and FS-14.R56 (R23 is already satisfied by the named-value list). The
  two committed skipped reproductions are named as the verification for A29 and A19, to be unskipped
  rather than rewritten. FS-06.R28 makes the per-turn budget configurable at a default of 50, by
  user decision.

  Deliberately **not** designed and left open: no per-template, per-run, per-stage, or per-role
  autonomy setting (the user considered and declined one); no general detector of a stage agent gone
  quiet for reasons other than a held approval; no change to how long a stopped agent stays
  unaddressable. The last two remain under *Decisions needing your input*, and R54 is scoped in
  terms to the wait AgentDeck is itself holding so it does not pre-empt either. The four unrecorded
  product wishes the investigation left for the user are still unplaced in `docs/ideas.md`.

  Judged as one change rather than split: eight of nine items are small and independent, and only
  R54's fan-out seam is architecturally real, so splitting would defer the same seam against a moved
  tree and buy a second review and a second browser pass. FS-03.A26 is a real-browser gate; the
  per-backend tool-name shape rides the existing credentialed gate (FS-09.A7), and an unconfirmed
  backend prompts rather than exempting. No product code changed. This commit also carries the
  earlier uncommitted proposal-decline design (FS-14.R49–R51/A27–A28 and its `docs/ideas.md` entry),
  which was already in the working tree and interleaves with these edits in the same files.

- **2026-09-02 — fix (worktree implementation findings; FS-19.R2–R11/A2/A4–A6,
  FS-04.R28/A6, TS-03.R33, TS-12.R1–R10; INV §2/§4/§5/§7/§8/§10/§12/§14/§15):** Closed only the
  ten findings from the worktree implementation review. Force deletion can no longer remove a
  running agent's checkout; nested forks keep a stable repository anchor; setup capture is bounded
  while streaming; Settings exposes the same checkout disclosure and consent as the dashboard;
  clean-to-dirty changes are rechecked before removal; owned root and leaf symlinks are refused;
  resume, switch, and pipeline continuation carry caller cancellation through recreation while the
  HTTP responses expose setup warnings; fork compensation uses an independent bounded context and
  removes only effects it claimed; lifecycle storage errors stay errors; and Git query helpers
  preserve unexpected failures. Targeted worktree, configuration, server, and UI regressions pass,
  as do the full required verification commands. Unrelated pipeline, task, Mermaid, dashboard,
  proposal-design, and browser-gate findings were left open and untouched.

- **2026-09-02 — bug investigation (unattended pipeline run; FS-14.R6/R7/R15/R19/R29/R37/R40/R47,
  TS-09.R8/R17/R28, FS-03.R17, FS-06.R4/R11/R22, FS-15.R17, FS-16.R12/R20, FS-17.R1–R3;
  INV §2/§8/§10):** Diagnosed a 23h24m pipeline run that finished with outcome `success` but needed
  continuous human supervision: 8 stage attempts, 4 of them reported blocked, 27 delegated tasks of
  which 24 succeeded, 1 Retry, 3 Continues, 1 manual session resume, at least 14 out-of-band user
  messages, and 87 permission prompts across three coordinator sessions of which 10 timed out.
  About 19 of the 23 hours were two stalls that began after the work was already done.

  Those two stalls have one confirmed mechanism, and it is a product defect rather than a template
  one: `renderAssignment` tells every stage agent to call `report_pipeline_stage_result` "exactly
  once" and that "that one call ends your part", while FS-17's shipped retry vocabulary classifies
  several of that same tool's refusals as retryable. A refused call is not a report — the attempt is
  untouched and still owes a result — but nothing tells the agent that, so it stopped, and nothing
  in the run noticed. `OnTurnEnd` is a no-op for an attempt with no accepted report, there is no
  timer anywhere in `internal/pipeline`, and FS-14.R29's two notification categories exclude the
  state by enumeration, so a run parked at `await_result` with a silent agent is indistinguishable
  from one being actively worked, indefinitely. A third defect explains the manual session resume:
  a task aimed at the earlier stage's coordinator is refused as "No agent matches", for an agent the
  same caller can and did share context with.

  Six findings are recorded below, three **Must fix**. Much of the report is the product working as
  written — an accepted `blocked` result is terminal while delegated children run (FS-14.R40 says
  so in terms), Retry means a fresh agent with only bounded prior summaries (FS-14.R12/R20), a run
  claims no filesystem isolation (FS-14.R21), and the run outcome is the reporting agent's own word
  (FS-14.R34) — and those are listed with the findings so the fix session does not chase them. The
  permission-prompt volume is already the user's own 2026-09-02 idea entry; what is new is that the
  180s auto-deny (FS-03.R17) makes it a correctness problem for control-plane calls, not only
  friction. No product code or specification changed; the only tree changes are the two skipped
  reproduction tests.

- **2026-09-02 — review (worktree implementation only; FS-19.R1–R11/A1–A7, FS-04.R45/A25,
  FS-02.R60/A42, TS-01.R26, TS-02.R27, TS-03.R33, TS-12.R1–R10; INV §1–§15):** Reviewed only the
  worktree-project implementation range `1b2a8c3..bd797bd`, leaving the unrelated dirty FS-14
  proposal files and two untracked pipeline/messaging tests that appeared during the final audit
  untouched. An independent Terra/high reviewer ran the same workflow; every
  reported issue was re-read against the implementation and requirements before consolidation.
  Ten findings are recorded below: eight **Must fix** and two **Worth fixing**. The Must-fix set is
  the force-delete/live-checkout race, the disposable parent-repository anchor for nested forks,
  unbounded setup output capture, incomplete Settings archive/delete wiring, a dirty-state consent
  gate that can delete before or after the disclosed state, missing root/use-time symlink checks,
  resume/switch recreation that ignores cancellation and drops warnings, and fork rollback that can
  leave branch/checkout/resource residue. The Worth-fixing set covers storage failures flattened
  into ordinary absence/success and Git query failures flattened into non-repo/missing/detached
  answers.

  The five earlier worktree design findings did matter. Ownership-row ordering, the read-only
  `ValidateStage` boundary, main-worktree base fallback, and the per-project recreation claim are
  correctly resolved. The 2,000-rune human display clamp is also present, but `CombinedOutput`
  still buffers the entire setup stream before the state layer clips it to 64 KiB. The
  fork-of-a-fork test proves base selection only; a direct Git reproduction confirms
  `--show-toplevel` returns the disposable parent checkout while `--git-common-dir` remains stable,
  so removing the parent breaks the child's later recreate/delete operations.

  Invariant sweep: §2 holds for the shared composition seam but the resume/switch callers misuse
  its context/result contract; §4 is implicated by abandoned start/cleanup work; §5 holds for the
  recreation claim but the dirty-state disclosure and fork compensation still have check/act
  gaps; §7 produced the storage/Git error-flattening findings; §8 produced the unbounded setup
  capture and missing warning/disclosure findings; §10 produced the missing Settings wiring; §12
  produced the Git failure-classification finding; §§14–15 produced the symlink, live-checkout
  deletion, and rollback findings. §§1, 3, 6, 9, 11, and 13 have no separate changed-surface
  finding. `make check-specs`, `make build`, both Go test variants, the targeted worktree/server
  race suite, the 360-case UI suite with style/presentation checks, `make dist`, and
  `git diff --check` pass. The first sandboxed full Go run could not read the host build cache; the
  unchanged elevated rerun passed. No product code or specification changed.

- **2026-09-02 — review (worktree only; FS-14.R49–R51/A27–A28, TS-02.R22,
  TS-03.R16–R17, TS-09.R15–R16/R23/R26; INV §1–§15):** Reviewed only the three unstaged
  proposal-decline design files; there were no staged or untracked files, and the last-reviewed-code
  marker does not move. An independent Terra/high reviewer ran the same workflow, and its findings
  were checked against the current proposal state/API/UI paths before consolidation. Four findings
  are recorded below. The Must-fix one is the missing approval-versus-Reject linearization: the
  existing Save/Start path commits its real effect before proposal consumption, while R49 describes
  only the case where approval has already won, so a stale approval can still act after Reject unless
  the design adds a durable claim and crash/retry rules. The remaining findings cover absent and
  contradictory persistence/API/control-plane specifications and change ownership, the feature
  introduction falsely claiming every requirement is shipped, and acceptance/summary coverage that
  tests only a pending Save proposal and leaves Start-title provenance undefined.

  Invariant sweep: §1 holds because R51 resets browser-local expansion on reload. §2 has no new
  parallel construction path in the documentation. §§3–4 and §6 have no applicable changed surface.
  §5 and §15 produced the action-race finding. §§7–11 produced the missing durable read/error/bound/
  wiring/collection contracts and incomplete summary matrix; §9 has no separate primitive to assess
  until that durable design exists. §12 has no external-CLI invocation, and §13 no class-name
  surface. §14's whole-mux guard remains the inherited route boundary, but the missing route contract
  is included in the technical-coverage finding. `make check-specs`, `make build`, both Go test
  variants, the 360-case UI suite with style/presentation checks, `make dist`, and `git diff --check`
  pass; the first sandboxed Go run failed only because it could not bind loopback test ports, then
  passed unchanged with those permissions. No product code or specifications were changed.

- **2026-09-02 — work (FS-19.R1–R11/A1–A7, FS-04.R45/A25, FS-02.R60/A42, TS-01.R26, TS-02.R27,
  TS-03.R33, TS-12.R1–R10; INV §2/§5/§7/§8/§10/§12/§14/§15):** Shipped **worktree projects** across
  three commits. Forking a repo-backed project now creates, as one action, a branch off the
  effective base, a fresh AgentDeck-owned checkout under `$AGENTDECK_HOME/worktrees/{project-id}/`,
  and a project copying the source's colour, prompt, additional directories, base branch, and setup
  command — then runs that setup command inside the new checkout.

  `internal/worktree` is the single Git boundary: argv-only, `git -C`, stdin closed,
  `GIT_TERMINAL_PROMPT=0`, bounded timeouts, plumbing-only parsing. Ownership is a
  `project_worktrees` row written after the checkout exists and removed after it is gone, so no
  crash window can authorize deleting something AgentDeck did not create. `ensureWorktreeCheckout`
  replaced the two duplicated cwd stat checks and is the one step launch, resume, and switch take
  before a process starts; both pipeline start paths inherit it through those composers, while
  `ValidateStage` stays read-only and merely tolerates a recreatable missing checkout. Consented
  deletion runs inside the existing archive claim, re-verifies the canonical path, refuses a
  symlink, and requires a live Git registration before removing anything; external checkouts have
  no row and no deletion path at all.

  On the dashboard, a repo-backed active project offers **New worktree project** from its card menu
  and from the scoped project header; the form asks only for title, branch, and base, deriving the
  branch from the title until it is edited by hand and taking the base from the server's use-time
  detection. A worktree project's card and scoped header name its branch. The archive dialog offers
  checkout deletion only for an owned checkout, defaults to keeping, says the branch survives either
  way, and reports "could not be determined" rather than claiming clean.

  All five design findings the deleted review section had recorded are resolved. Three became
  in-place TS-12 corrections: R3's ownership row moved after the project file, R4 stopped calling
  the mutating helper from the read-only `ValidateStage` pre-flight and reports recreation in the
  start response rather than racing `status.detail`, and R9's fallback reads the repository's main
  worktree branch. Two became implementation: a per-project claim serializes concurrent recreation
  (covered under `-race`), and setup output is clamped to 2,000 runes at the human boundary on top
  of the 64 KiB storage tail. FS-19 §3 now states that a consented deletion ends ownership, so a
  restored project is not re-materialized — the smaller of the two readings, flagged for
  confirmation below.

  Three bugs were caught by the new tests and one live run rather than by review: forking a fork
  branched off the source fork's own branch, consenting to delete an external checkout reported a
  deletion that never happened, and the status endpoint returned "(no output)" for a setup run that
  had succeeded. A `make dist` binary was driven end to end against a real repository — create,
  fork, setup bootstrap, dirty disclosure, declined archive, consented archive, surviving branch —
  and all of it behaved. The browser halves of FS-19.A1/A4 stay owed as acceptance gates.

  Also here: the regenerated embedded `index.html` **Must fix** is closed, and the four handoff
  sections `1b2a8c3` deleted were restored from `e46e66b` plus that commit's own message.

- **2026-09-02 — review (`180ea89..e46e66b`; FS-19.R3/R7/R8, TS-12.R3/R4/R5/R7/R10, TS-01.R26,
  FS-02.R47/R55–R59, FS-12.R40, TS-08.R45–R48, FS-14.R49–R51; INV §1–§15):** Reviewed the previous
  review's state commit, the FS-19/TS-12 worktree design, the shipped card-grid change, and the
  uncommitted FS-14 proposal-decline design against their requirements and every invariant class.
  Closed the previous cycle's two **Must fix** findings on evidence: `make check-specs` reports
  `spec check: ok`, and the card-grid change is fully committed. Recorded four new findings. One is
  **Must fix**: `e46e66b` changed `ui/src` but left the regenerated
  `internal/server/ui/dist/index.html` uncommitted, so `make build` on a checkout of `main` with a
  populated embed directory serves the previous bundle with nothing on screen saying so — every
  earlier UI-changing commit updated that file in the same commit. Three are against the worktree
  design, before it is built: TS-12.R4 routes the mutating `ensureWorktreeCheckout` through pipeline
  `ValidateStage`, which the manager calls once per stage inside a read-only pre-flight that turns
  errors into per-assignment diagnostics; TS-12.R3's crash-safety claim is false for the window
  between its own ownership-row insert and project-file write, leaking a branch, directory, and row
  with no reclaim path; and FS-19 never says what a reactivated worktree project does after a
  consented checkout deletion, where TS-12.R7's normal and crash paths answer in opposite ways. Two
  smaller ones: concurrent checkout recreation has no atomic claim, and the 64 KiB setup-output tail
  reaches a human warning surface with no display bound. The design's INV §12 and §9 handling is its
  strongest part. The card-grid implementation matches FS-02.R55–R59 and TS-08.R45–R48; its two
  carried findings (FS-02.R47's stale two-column clause and `ContextBar`'s overloaded `data-variant`)
  are confirmed still open. All other required checks pass. No product code or specifications
  changed.

- **2026-09-01 — feature design (FS-02.R61/A43 and the narrowed FS-02.R51, TS-08.R49; INV §1/§2):**
  Designed automatic pane opening and queued `open-waiting-approval-panes.md`. A chat agent newly
  entering `waiting_input` expands its own pane; this reverses one clause of R51, which had
  forbidden any automatic expansion, and every other event still opens nothing. The trigger is the
  durable `state` on `state_update`, not the `notification` event `bus.go` already emits for the
  same transition, because that stream is mute-filtered and is never replayed to a reconnecting
  tab — muting a toast must not silently change what the dashboard opens. Only a newly observed
  transition fires: `CardGrid` keeps one record of last observed states, reseeds it while
  `hydrating` is set, and drops ids `hydrateComplete` prunes, so reloads and reconnects cannot
  reopen panes a person collapsed. The stated cost is that a pane opens only while a grid is on
  screen. The user chose least-recently-used eviction over skipping the open when four panes are
  already up, so an automatic open can close a pane they had open; R48's draft retention makes that
  recoverable. Terminal agents, cross-project agents, and agents in a collapsed group section are
  excluded. TS-08 returns to **Partial**. This design was written while another session's card-grid
  implementation and FS-19 worktree design were live in the same tree: FS-02.R60/A42 were taken by
  the worktree design (since committed as `f25a2ba`), so these requirements are R61/A43. The
  review's broken-link **Must fix** was closed here — the handoff changelog entry is delinked and
  the finished change is off the waiting list. This design is left uncommitted because the card-grid
  implementation it sits beside is still uncommitted and unverified by this session; both need one
  commit from the session that finishes that work. No product code changed here.

- **2026-09-01 — design-feature (FS-19.R1–R11/A1–A7, FS-04.R45/A25, FS-02.R60/A42, TS-12.R1–R10,
  TS-01.R26, TS-02.R27, TS-03.R33; INV §2/§4/§5/§7/§12/§14/§15):** Specified worktree projects
  around the confirmed product decision that the project is the workspace. New FS-19 defines the
  fork action (branch off the effective base + owned worktree under
  `$AGENTDECK_HOME/worktrees/{project-id}/` + copied project, all-or-nothing before setup),
  optional per-project `base_branch`/`setup_command` (failure is a warning, never a block),
  disposable-checkout recreation reported on every start path, and explicit-consent-only deletion
  at project archive/delete with dirty state disclosed and the branch always kept; external
  checkouts are never deletable. New TS-12 pins the architecture: one argv-only Git boundary, a
  `project_worktrees` SQLite ownership row written after the checkout exists and removed after the
  checkout is gone (crash residue is always inert/external), one shared `ensureWorktreeCheckout`
  step replacing the two duplicated cwd stat checks across launch/resume/switch/ValidateStage,
  deletion gated inside `beginProjectArchive`, branch-ref creation as the fork's atomic claim, and
  a subprocess-free projects-list path. Ready change `worktree-projects.md` created; the idea was
  captured and removed in the same change. Documentation-only checks pass on the committed tree;
  the working-tree `make check-specs` failure on the card-grid change's dead link predates and
  survives this change. Only this design's files were committed; the uncommitted card-grid tree was
  left untouched.

- **2026-09-01 — review (`57b7154..180ea89` plus the uncommitted card-grid tree; FS-02.R55–R59/
  A37–A41, FS-12.R40/A16, TS-08.R45–R48, FS-03.R37/A20/A22, TS-08.R40, FS-18.A5, FS-04.A24; every
  INV class):** Reviewed the card-grid change, the Mermaid fix, the active-project fix, and the
  paused MCP design against their requirements in both directions. Six findings recorded, three
  **Must fix**: the react-markdown component map is memoized on `[text]`, so a settled diagram is
  still remounted on every streamed delta (reproduced against the shipped component — the SVG drops
  to source and `mermaid.render` runs again with a new id); the delivered tree fails
  `make check-specs` on a dead link to the deleted ready-change file, which the ready-change index
  also still lists as waiting to start; and the whole card-grid change, its spec updates, and its
  state entries are uncommitted. Three **Worth fixing**: FS-02.R47's two-column-footprint clause is
  now false but still normative, `ContextBar` collapses tone and density into one `data-variant` so
  the compact meter has no tone hook in the presentation contract, and an unattributed FS-14
  proposal-decline design sits in the same uncommitted tree with no ready-change file. The four
  MCP-migration design findings are closed by `3863e8b` and removed from the open list. Everything
  else in the range holds: `collapseAll` merge-preserves out-of-project ids and surfaces save
  failure, `ContextBar` remains the single context derivation, the refused-drag wildcard is the
  right shape for INV §2, and the embedded UI artifact matches a fresh build of this tree. All
  checks except `make check-specs` pass. No product code or specifications were changed.

- **2026-09-02 — work (FS-02.R55–R59/A37–A41, FS-12.R40/A16, TS-08.R45–R48; INV
  §1/§2/§8/§10/§13):** Stabilized and decluttered dashboard agent cards. Expanded panes now retain
  their original one-track grid cell, remain in their sortable block, and leave every other card's
  placement unchanged. A labelled per-card **Collapse** button and conditional whole-grid
  **Collapse all** share the existing expansion state and debounced layout save; the latter removes
  only current-grid ids and preserves retained ids from other projects. Names use a smaller
  three-line clamp with unbroken-token wrapping. `ContextBar` remains the sole context derivation
  and appears only as a compact expanded-header figure. The presentation contract and deterministic
  matrix cover the new hooks and long-name/context states. A real-browser pass caught and fixed an
  initial expanded-header overflow; Core and Sky & Grove then held at 1024px and 1440px. The 346-case
  UI suite, both Go variants, style/spec checks, UI/product/distributable builds, and `git diff
  --check` pass. The ready change is removed and FS-12 returns to Current; FS-02 and TS-08 stay
  Partial for their planned items. (Status sentence corrected by the 2026-09-02 review.)

- **2026-09-01 — fix (FS-03.R3/R37/A20/A22, TS-08.R40; INV §2/§10/§13):** Fixed both
  confirmed Mermaid display defects. `AssistantText` now keeps its react-markdown component map
  stable across parent-only transcript rerenders, so crossing the at-bottom scroll threshold no
  longer unmounts settled diagrams or reruns Mermaid. The sanitizer removes Mermaid's root-SVG
  intrinsic `max-width` cap, while the presentation stylesheet gives the figure its available
  message width and bounds tall diagrams to the viewport. The activated regression covers stable
  markup and render count, and sanitizer coverage prevents the intrinsic cap from returning. In the
  production browser the compact SVG grew from 124px to 735px in the dashboard pane and 768px in
  full chat; a wide graph stayed contained without horizontal overflow. Scrolling preserved all
  four SVG ids with no source fallback. Both AgentDeck Core and Sky & Grove passed the dashboard
  geometry check. `make check-specs`, `make build`, `make test`, `npm test`, `npm run build`, and
  `make dist` pass; the two Mermaid findings are closed.

- **2026-09-01 — feature design (FS-02.R55–R59/A37–A41, FS-12.R40/A16, TS-08.R45–R48 and the
  corrected TS-08.R43; INV §1/§2):** Designed the fix for the reported dashboard card-grid
  experience and queued
  `stabilize-and-declutter-agent-cards.md`, which has since been implemented and removed.
  The rearranging on expand/collapse is the two-track span in an auto-placed grid, which TS-08.R42
  accepted as a wrap-and-gap cost; the pane now spans one track, so every other card keeps its cell
  and only the rows below move. The rejected alternatives are recorded: moving panes out of the grid
  into a separate region, and filling the empty space beside a pane with `dense` or a fixed
  `grid-auto-rows` row span, which both reassign cells and reintroduce the movement. The empty space
  beside a pane's row is the deliberate accepted cost. Also specified a visible labelled collapse
  control (an open card previously had none — the header holds only a navigating name link and the
  state badge), **Collapse all** scoped to the grid's own ids so FS-02.R49's retained out-of-project
  ids survive, a smaller three-line-wrapping card name, and the context meter moving off the
  collapsed card onto the expanded one. The user declined enlarging cards, so minimum height, column
  count, gap, and the density control are unchanged. Reading the area found TS-08.R43 asserting that
  an expanded id leaves its `SortableContext` while FS-02.R47 and the shipped grid keep it; R43 is
  corrected to the shipped truth. FS-02, FS-12, and TS-08 move to **Partial**. No product code
  changed, and the unrelated pipeline-proposal design already dirty in the tree was left untouched.

- **2026-09-01 — bug investigation (FS-03.R3/R37/A20/A22, TS-08.R40; INV §2/§10/§13):**
  Recorded the field report verbatim: "Mermaid looks like garbage, it's tiny and keeps spazzing out
  between display and source when scrolling." The report named no logs, environment, version, or
  commit; reproduction used current `3863e8b`, the production embedded UI, an isolated copied
  fixture, the deterministic `diagram_stream` fake ACP scenario, and the in-app Chromium browser.
  Both symptoms are confirmed code defects. Four settled diagrams became four raw Mermaid sources
  immediately after one transcript scroll and returned as newly generated SVGs with different ids.
  `TranscriptView` changes `atBottom` state on scroll, while `AssistantText` creates a new inline
  react-markdown `code` component on every render; React therefore remounts every `MermaidDiagram`,
  resetting its markup to null until asynchronous rendering completes. The same browser measured a
  761px transcript, a 150px figure including padding, and a 124px SVG: `width: fit-content`
  shrink-wraps Mermaid's `width="100%"` SVG to its narrow intrinsic width. Both paths originated in
  the Mermaid feature commit `9ae64ba`. A focused parent-only-rerender test fails on the source flash
  and is committed skipped for the fix session to activate. No product code or specification
  changed; the existing unrelated dirty specification and idea files were untouched.

- **2026-09-01 — feature design (FS-06 §6, FS-15 §6, FS-16 §6, FS-17.R13–R20/A7–A11;
  TS-01.R25, TS-03.R32, TS-04.R32–R40, TS-05.R18–R19, TS-06.R23, TS-11.R11–R12):** Resolved
  the MCP-migration design findings and paused the change at a new safe-transport gate. The current
  packaged Codex/ACP path cannot reach the proposed loopback action route under its default sandbox
  without enabling shell networking more broadly than the feature needs. Broad networking,
  filesystem IPC, and a provider-specific MCP fallback are rejected; the shipped internal MCP path
  remains authoritative until Codex supports a narrow direct channel and all four chat providers
  prove it. The future contract now separates non-secret generation, hook credentials, and action
  credentials; makes `action describe` a local compiled-registry operation; and records exactly how
  FS-06, FS-15, and FS-16 transport wording is superseded after cutover. No product code changed.

- **2026-08-31 — fix (FS-02.R53/A35, FS-12.R39/A15, TS-08.R44, FS-18.A1/A5, FS-04.A24,
  TS-11.R4/R6; INV §6, §8, §10, §13):** Closed all five shipped-code findings from the
  `52d01c4..da2db77` review. **INV §10/§13** — the refused cross-block drag marked only the group
  stack, so the card under the pointer kept its own `cursor: pointer` and the promised in-flight
  refusal never reached the pointer; the refused state now covers every descendant of the stack with
  one wildcard rule rather than an opt-out list that would drift as card controls are added, and it
  outranks the expanded card's own header cursor. jsdom evaluates no CSS, so the new case reads the
  stylesheet the way `ProjectDashboard.test.tsx` already does and the browser half stays an
  acceptance gate. **INV §10** — the active-project overflow handled Escape only on its `+n` button,
  so the normal keyboard path (Tab into a project link, press Escape) left the disclosure open; the
  handler moved to the disclosure container and the regression now focuses a menu item first.
  **INV §10** — the ready-change index still linked the change file the implementation commit
  deleted, presenting finished work as selectable; the waiting list is corrected. **INV §8/§10** —
  the migration's "read failure" fixture exercised JSON decode corruption, not an `os.ReadFile`
  error, and one root guard skipped that independent case along with the permission-based write
  case; the three cases are now separate, corruption and a genuine read-I/O failure (a directory at
  the role path) always run, and only the write fixture skips as root. FS-18.A5 names the decode and
  I/O cases separately to match. **INV §6/§10** — the operating-knowledge lifecycle matrix resumed
  and switched only a chat agent, so a terminal-specific regression in either composer could stay
  green; terminal resume and terminal runtime switch rows joined it for both the available and
  unavailable package. One new finding is recorded below rather than fixed: the task-cancel case
  asserts a completed release the specification only guarantees through recovery, and it failed once
  under full-package load. `make check-specs`, both Go test variants, `make build`, the 340-case UI
  suite, the UI production build, and `git diff --check` pass.

- **2026-08-31 — design review (`migrate-internal-actions-from-mcp.md`; FS-06.R1/R2/A1/A3, FS-15,
  FS-16, FS-17.R13–R19/A7–A11, TS-01.R25, TS-03.R32, TS-04.R32–R40, TS-05.R18–R19, TS-06.R23,
  TS-11.R11–R12; INV §2/§4/§10/§12/§14):** Reviewed the waiting MCP-migration design through the
  over-engineering, extension, and research lenses. Four findings recorded, two of them Must fix. The
  transport assumption fails a real check: Codex 0.142.5 under the default `workspace-write` sandbox
  cannot open a loopback TCP connection from a spawned command, and AgentDeck mirrors the user's
  `config.toml` into its private `CODEX_HOME` unchanged, so `agentdeck action` would be unreachable
  for every Codex agent — while today's MCP path works because the unsandboxed CLI process makes the
  call. Separately, `generation` defaults to the launch token and is persisted and served as
  `agent_generation`, so merging hooks and actions onto that one secret publishes full action
  authority. The remaining two cover FS-06/FS-15/FS-16 still mandating the internal MCP with no
  supersession note, and `describe` routing compiled-in registry data through an authenticated HTTP
  round trip. Two consistency notes recorded. The change stays Waiting to start; no specification,
  change file, or product code was modified.

- **2026-08-31 — feature design (FS-17.R13–R19/A7–A11; TS-01.R25; TS-03.R32;
  TS-04.R32–R40; TS-05.R18–R19; TS-06.R23; TS-11.R11–R12; INV §§1–6, 8–15):** Validated that
  AgentDeck's in-process internal MCP adds model-visible schema and protocol coupling without an
  interoperability benefit, while MCP remains appropriate as a supported provider/user extension
  protocol. Specified one packaged `agentdeck action` CLI over a private loopback action adapter and
  provider-neutral typed registry, reusing the existing generation-scoped launch credential. The
  fifteen actions, structured results, authority, domain behavior, and external MCP federation stay
  unchanged. The implementation freezes parity first, validates Claude/Codex/OpenCode/OpenHands,
  and removes the internal MCP path before release with no shipped fallback. The ready change is
  `migrate-internal-actions-from-mcp.md`; its phased implementation plan is under `docs/plans/`.

- **2026-08-31 — review (FS-02.R53/R54/A35/A36, FS-12.R39/A15, FS-18.A1/A5,
  FS-04.A24, TS-08.R44, TS-11.R4/R6; INV §1–§15):** Reviewed the continuous
  `52d01c4..da2db77` range in both directions. Five findings are open: Escape closes the project
  overflow only from its trigger; the refused-drag state never overrides the card under the
  pointer; the ready-change index retains a dead link to the finished change; the migration
  I/O fixture substitutes corrupt JSON for a real read failure and skips all cases as root; and the
  knowledge-overlay matrix never resumes or switches a terminal agent. The code otherwise matches
  the active-project membership, ordering, current-context, presentation, pipeline action, layout
  error, Mermaid sanitization, and runtime-overlay requirements. The skipped pre-implementation
  design review was evaluated as a local workflow choice: this pass found no excess abstraction,
  parallel mechanism, or contradicted seam to turn into a product finding, but the independent
  before-build sequence cannot be reconstructed after implementation. INV §§1–3, 6–11, and 13 had
  applicable surfaces; §§6, 8, and 10 produced the findings above and the others produced none.
  §§4, 5, 12, 14, and 15 had no applicable surface. Specification checks, production build, the
  focused 61-test UI set, both Go test variants, the full UI suite, and the distributable build
  pass; the first sandboxed Go run failed only because it could not use the host cache or bind
  loopback test ports and passed unchanged with those permissions.

- **2026-08-31 — work (FS-02.R54/A36, FS-12.R39/A15, TS-08.R44; INV §1/§2/§8/§10/§13):**
  Added compact active-project navigation to the persistent header. One pure derivation combines
  the project catalog, hydrated agent projection, and current project/agent route; it filters
  stopped, archived, and unavailable entries, retains current context, alphabetizes with the id
  tie-breaker, and produces the five-link visible/overflow split without retained state. The
  feature-owned component supplies full accessible titles, accent identity, a structural selected
  marker, and an Escape/outside/selection-closing `+n` disclosure. The shell now uses a four-region
  non-wrapping grid. Focused regressions, the zero/one/overflow visual matrix, all UI tests, both Go
  variants, production and distributable builds, specification/style checks, and `git diff --check`
  pass. Real-browser inspection at 1024 and 1440 confirmed no shell overflow in Core or Sky & Grove.

- **2026-08-30 — feature design (FS-02.R54/A36, FS-12.R39/A15, TS-08.R44; INV §1, §2,
  §8, §10, §13):** Specified compact active-project navigation immediately after the shell's
  primary tabs. Configured non-archived projects qualify only while they have a non-archived
  `running` agent, except that the current project remains visible across its last agent stopping
  until the operator leaves its project or agent route. Title/id alphabetical order supplies five
  direct links; when the selected project falls later it replaces the fifth, and `+n` contains the
  alphabetized remainder. The visual contract uses smaller restrained rounded tabs, project accent
  plus a non-color selection cue, full accessible names for truncated labels, no motion, and a
  four-region header that holds at the 1024px floor in Core and Sky & Grove. The design reuses
  `useProjects`, the hydrated agent store, current route matching, and `--ad-project-accent`; it adds
  no measurement, recency state, persistence, API/server shape, dependency, token, public hook, or
  second row. `active-project-navigation-tabs.md` is Waiting to start.

- **2026-08-30 — fix (FS-03.R38/A21, FS-02.R45/R53/A28/A35, FS-14.R48/A26, FS-12.R37/A13,
  FS-18.A1/A5, FS-04.A24; INV §6, §7, §8, §10, §13):** Closed all seven open findings from the
  2026-08-30 code review and usability run. **INV §8** — the diagram sanitizer judged CSS before
  decoding it, so `u\72 l(...)`/`@\69 mport` survived the literal-text strip; style elements and
  style attributes are now decoded first and dropped whole when a URL-bearing token remains, with a
  renderer case that fails against the old regex. **INV §7/§8** — a failed `GET /api/layout` left
  `loaded` false forever, silently disabling layout persistence for the session; the failure now
  surfaces through `pushError` and still arms saving. **INV §10** — `launch_failed` and
  `resume_failed` pauses also leave no running stage agent, so R48's withholding of **Open agent**
  widened to them with wording that names the failure; the spec item and A26 widened with it.
  **INV §13** — `.pipeline-state-launch_failed`, `-resume_failed`, `-restart_recovery`, and
  `-restart_awaiting_quiescence` had no selector, so interrupted attempts fell back to the neutral
  badge; failures now read at error salience and interruptions at waiting salience. The refused
  cross-block drag now states the refusal in flight through the pointer instead of an unexplained
  snap-back, specified as new FS-02.R53/A35. **INV §6/§10** — FS-18.A1's lifecycle-composition
  matrix now covers all three composers across seven lifecycles with the package available and
  unavailable, asserting the effective process parameters once and their absence from frozen session
  metadata; removing the overlay call from `resume.go` fails it. **INV §10** — the exact migration
  gained corrupt-read and read-only-directory write-failure fixtures that compare the role bytes
  before and after. `make check-specs`, `make build`, both Go test variants, the UI suite, and the
  UI build pass.

- **2026-08-30 — review (FS-18, FS-04.R44/A24, FS-03.R38/A21, TS-11, INV
  §1–§15):** Reviewed the continuous `43e5feb..52d01c4` range end to end. The package installer,
  conditional runtime-only overlay, exact migration, lifecycle wiring, and release/workflow state
  match their specifications, but three findings are open below. One **Must fix**: Mermaid CSS is
  scrubbed with a literal `url`/`@import` regex, so a CSS-escaped identifier survives sanitization
  and can still resolve as a remote request in the browser. Two **Worth fixing** acceptance gaps:
  FS-18.A1's lifecycle matrix is tested only at the overlay helper and terminal argument layer, and
  FS-18.A5's read/write-error migration fixtures are absent. Every invariant class was swept:
  §§6, 8 and 10 produced these findings; applicable surfaces under §§1–5, 7, 9 and 11–15 produced no
  new finding. `make check-specs`, `make build`, the full default and `sqlite_fts5` test variants,
  and `make dist` pass; the first sandboxed test attempt could not use the Go cache or bind a
  loopback test port, then passed unchanged with those required permissions.

- **2026-08-30 — usability review (FS-14.R43/R45/R46/R48, A14/A18–A22/A24/A26; FS-02.R45–R52,
  A28–A34; FS-12.A13):** Drove the new Pipelines pages and the changed dashboard grid through a real
  Chromium against a `make dist` build of the shipped tree. Twenty-nine steps passed, including all
  three fixes that had never been in a browser (attempt badge on the timeline rule, run rail
  stacking after the timeline at the 1024 floor, one-shot entrance on an appended entry with
  reduced motion removing it) and every J5 item that was owed for running-first placement and the
  chat panes. Four findings are open: two Must fix (a failed layout read silently disables layout
  persistence for the session; a launch- or resume-failed pause still offers a dead-end **Open
  agent**) and two Worth fixing (failure states render in the neutral badge; a refused cross-block
  drag gives no reason). No product code, specification, or journey matrix changed. Coverage gaps
  between the J5/J14 charters and FS-02/FS-14 acceptance items are recorded in the run file.

- **2026-08-30 — release (FS-10, TS-06.R13–R22, TS-11.R1/R8):** Cut `v0.3.0` for the 55-commit
  `v0.2.3..HEAD` range under the new §16 role. No open findings blocked it; the user accepted the
  20-commit unreviewed range (`43e5feb..HEAD`) and chose a minor version for the bundled operating
  skill, split Pipelines surface, expandable chat panes, Mermaid chat rendering, and running-first
  card placement. Step 2 found the embedded package already correct for the range and recorded that
  as a result: the one agent-facing change after the design froze, FS-14.R47's stage participation
  boundary, is already stated in `build-and-run-pipelines.md`, and the mail budget of 15 and CLI
  launch forms still match the code. The README `~/.agentdeck/` layout was missing the
  `cache/agent-skills/` root TS-11.R2 introduced and is fixed. Both Go variants, 50 UI test files,
  `make dist VERSION=0.3.0`, and `git diff --check` pass; the binary reports `0.3.0` built with
  `sqlite_fts5`. The user authorized the push: `main` went out through `89014a5` (17 commits, which
  swept in another session's usability-review state commit), the tag followed, and release run
  `33296747765` published a verified archive, checksum manifest, and installer.

- **2026-08-30 — workflow (TS-11 §5, FS-10, TS-06.R13–R22):** Added the release role the shared
  operating skill was waiting on. Workflow §16 fixes the release range at the previous `vX.Y.Z` tag,
  blocks on open **Must fix** findings, requires the range to be read for agent-facing change and
  the embedded `internal/agentknowledge/operating-agentdeck/**` source to be refreshed under
  TS-11.R1/R8 ownership and exclusions, extends the same test to the README and the pinned
  `install.sh`/`assemble.sh` component versions, confirms the version with the user, verifies with
  the §2 checks plus `make dist`, and leaves assembly and publication to release CI behind explicit
  push authorization with the credentialed provider gates named as owed. Byte-identical `/release`
  launchers were added to `.claude/skills` and `.agents/skills`, and AGENTS.md now routes the role.
  Documentation-only: `make check-specs`, the twin comparison, and `git diff --check` pass.

- **2026-08-30 — work (FS-18, FS-04.R44/A24, TS-11, INV §1/§2/§4/§6/§8/§10/§15):** Shipped the
  release-matched `operating-agentdeck` package and thin AgentDecker role. Startup stages, syncs,
  verifies, and atomically publishes owner-only `.agents` and `.claude` views; a failure logs a
  warning, suppresses every availability signal, and leaves exact migration for a later retry.
  `applyKnowledgeOverlay` is the one fresh/resume/switch seam, using runtime-only add-dir, prompt
  suffix, and environment fields so no managed value enters the frozen session snapshot. Chat and
  every terminal driver consume the same effective parameters; iTerm transfers secret environment
  values through a bounded owner-only FIFO so its visible command and filesystem carry no secret
  data. Only the production SHA-256 of the immediately preceding AgentDecker seed migrates, with all
  other role fields preserved; fresh PM/teammate prompts and tool descriptions no longer duplicate
  cross-workflow guidance. Package, exact-fixture, failed-install retry, persistence, primer, and
  terminal regressions are green with both Go test variants, specification checks, production and
  distributable builds, and `git diff --check`. An independent audit closed its iTerm secret and
  migration-fixture findings; its old-cache note was rejected because FS-18 requires verified
  replacement of an older cache. Pinned adapters 0.59.0/1.1.2 are installed, but logged-in live
  provider discovery remains a manual gate.

- **2026-08-29 — feature design follow-up (FS-18.R2/R7/R11, FS-04.R44, TS-11.R4–R6/R8/R10):**
  Closed the shared-skill design review on the user's classification. Package verification now
  precedes exact AgentDecker migration; an unavailable package leaves the legacy prompt untouched
  for a later retry, and the thin seed prompt refers only to current operating guidance rather than
  claiming the bundled skill exists. The other two review items remain small required alignment
  cleanups: overlay directory/prompt additions use runtime-only fields that session persistence
  cannot see, and fresh PM/teammate seeds drop duplicated coordination mechanics and the numeric
  budget without migrating user-owned roles. Acceptance coverage now joins install failure to the
  exact-role fixture and later successful retry. The impossible comparison-error case and unrelated
  INV §11 citation were removed. The change is Waiting to start and ready for implementation.

- **2026-08-29 — design review (FS-18, FS-04, TS-11, INV §1/§2/§6/§8/§10/§15):** Three Must-fix
  findings block the waiting shared-skill change. The proposed helper adds the overlay to
  `LaunchSpec.AddDirs` and its prompt, but those are the frozen fields `runtimeMeta` writes back to
  the session; after one successful start, a later install-failed process can therefore resume with
  the stale directory/pointer that TS-11.R10 says must be absent. The exact AgentDecker migration is
  independent of package availability and its target prompt unconditionally tells the role to use
  the bundled skill, so an install failure can both remove the legacy manual and direct the agent to
  unavailable knowledge. Finally, the seeded PM and teammate prompts still own messaging tool names,
  wake behavior, recipient addressing, and the numeric budget assigned exclusively to
  `coordinate-work.md`, leaving two stale release-mismatched sources. Pinned `codex-acp` 1.1.2 and
  Claude 2.1.238 support skill discovery from additional roots, so no discovery finding remains.
  Two consistency notes record an impossible comparison-error fixture and a misapplied INV §11
  citation. No product code, specification, or change file was edited; the change remains Waiting
  to start.

- **2026-08-29 — feature design revision (FS-18, TS-11):** Tightened the shared operator skill to
  three runtime-operation references. Messaging budgets now belong only to coordination guidance;
  blocked/Continue/proposal behavior belongs to pipeline guidance; release maintenance is excluded
  from the product skill. Secure installation remains atomic and owner-only, but failure now logs a
  warning, starts AgentDeck, and suppresses every discovery/pointer claim for that dashboard
  process. The ready change remains waiting to start.

- **2026-08-29 — fix (INV §1/§2/§4/§5/§7/§8/§10):** Closed every open finding: the twelve from the
  2026-08-29 review of `6a16126..43e5feb`, the three from the 2026-08-28 bug investigation, and the
  seven remaining from the 2026-08-28 review of `790c01c`.

  The Must fix needed a product call and got one — state the blocked-stage boundary rather than
  build an in-chat continuation route, and withhold the dead-end action rather than widen Continue.
  New FS-14.R47 puts the boundary in the assignment, the accepted result, and the refusal; new
  FS-14.R48 withholds **Open agent** on a restart pause and names Retry. Refusals are now logged at
  Warn with the run, attempt, caller and attempt agent/generation, pending action and code — the set
  that separates the conditions the report conflated — and `OnTurnEnd`/`OnExit` re-read the run
  under the run lock as `Report` already did (INV §5).

  Two amplification defects in the shared SSE transport (INV §1/§4/§8): a joining tab restarted the
  one shared stream, costing every other tab a full re-hydration and dropping their deltas, and the
  worker never removed a port. The worker now replays a bounded retained snapshot to the joining
  port alone and drops a port that says goodbye, closing the stream with the last one (TS-03.R7).

  Two artifacts built twice were unified (INV §2): the card preview kept opposite ends of a message
  on the client and the server, now both the tail clipped by code point; and Retry eligibility, hand-
  duplicated across the language boundary and already drifted once, is now projected by the store as
  `retry_eligible` (new TS-10.R22).

  In the grid, each running/stopped block gained its own sortable context so an in-drag preview
  cannot cross the boundary FS-02.A28 promises it will not, an expanded card stays in its block's
  set so neighbours see its two-column footprint (FS-02.R47), and pane focus cycling binds once at
  the grid container so it can leave a group section (FS-02.R50).

  Smaller: Mermaid's scratch node is torn down on a draw failure; the run page keys `RunDetail` by
  run id; the run-list projection no longer claims a run-detail read it never made; an agent-target
  task row no longer opens with a stray separator; the reconcile regression now proves its own name;
  the tracked build caches and the emitted worker chunk are untracked; and TS-06 §6 names the stress
  fixture. Coverage that acceptance items named but no test provided was added for the pane change,
  the delegated-agent cards, the looping timeline, and the proposal counts, and FS-02.A27 was
  narrowed to what its suite proves. Both Go test variants, the 329-case UI suite,
  `make check-specs`, the UI typecheck, `make build` and `git diff --check` are green. Two browser
  checks remain owed and are recorded as acceptance gates.

- **2026-08-29 — feature design (FS-18, TS-11):** Specified a thin AgentDecker role backed by one
  product-managed `operating-agentdeck` skill available to every launched role. The ready change
  covers exact-only migration of the historical prompt, job-oriented progressive references,
  native and direct-path discovery, one lifecycle composition seam, and a four-way release
  maintenance classification. It remains waiting to start; no product code changed.

- **2026-08-29 — fix (INV §8):** Closed the false `stale_assignment` Must-fix finding. A report from
  the run's own current attempt after its prior result was accepted now returns the shared
  `already_reported` code and points to the human Continue boundary; genuine caller or generation
  mismatches still return `stale_assignment`. The previously skipped field-bug reproduction is now
  an active regression, and both Go test variants plus the product build are green.

- **2026-08-29 — fix (INV §1/§4):** Closed the pane transcript-retention Must-fix finding. Raw
  events now live only in a constant-time append tail while an authoritative transcript request is
  in flight; the tail is cleared when the newest request settles, the last surface unregisters, or
  the agent is removed. The permanent folded transcript still preserves streamed deltas and resolved
  permissions across refetches. Both Go test variants, the 302-case UI suite, specification checks,
  and production builds are green.

- **2026-08-29 — fix (INV §2/§8):** Closed the template deep-link Must-fix finding. A failed
  template-library request now renders a load failure and the transport message instead of claiming
  the template was deleted; a missing record after a successful read keeps the deletion guidance.
  The new acceptance regression and the full 301-case UI suite are green with both Go test variants
  and production builds.

- **2026-08-29 — fix (INV §8):** Closed the diagram sanitizer Must-fix finding. Renderer-produced
  inline `style` attributes now have remote `url(...)` and `@import` references removed at the same
  DOMPurify seam that already scrubs style-element text. The safety regression covers both the
  generated attribute and a zero-request assertion; both Go test variants, the 300-case UI suite,
  the product build, and the UI production build are green.

- **2026-08-29 — review:** Read the whole unreviewed range `6a16126..43e5feb` against the
  specifications and every invariant class, clearing the backlog of three commits that had only
  ever had design and usability reviews. Fifteen findings, three of them Must fix. The diagram
  sanitizer forbids URL attributes but not `style=""`, and DOMPurify treats `style` as URI-safe by
  default, so an author `classDef` carrying `url(https://…)` defeats FS-03.R38's no-network
  guarantee at the configuration level. A failed `/api/pipelines` fetch makes the template page
  claim the template was deleted, where its own twin `RunDetail` distinguishes 404 from any other
  read failure — the two paths drifted inside one commit series. The new pane store keeps a second,
  unfolded copy of every transcript event with no cleanup on any lifecycle boundary and a full array
  copy per streamed delta, on exactly the four-pane streaming workload the feature creates. The
  remainder are smaller: pane keyboard cycling cannot leave a group section, expanded and
  cross-block drags compute in-drag geometry over a list that does not match what is rendered,
  a draw-stage diagram failure leaks a node onto `document.body`, the delegated-agent and
  proposal-count UI has no frontend test behind the acceptance items that name one, and several
  traceability and tracked-artifact gaps. Both Go test variants, the 289-case UI suite,
  `make check-specs`, `git diff --check` and all nine skill twins are green at `43e5feb`; every new
  `rows.Next()` loop in the range checks `rows.Err()`. One earlier finding is corrected: the tracked
  `internal/server/ui/dist/index.html` is covered by a `.gitignore` negation that has existed since
  the first commit, so only the emitted worker chunk is genuinely tracked outside the rule.

- **2026-08-29 — work:** Shipped expandable chat panes on project dashboards. Up to four chat cards
  can expand in place, persist across reloads, retain per-agent drafts, cycle by keyboard, and keep
  their transcript scrolling isolated from the page. The shared SSE client now registers multiple
  open agents with reference-counted teardown, reconnect and gap recovery fan out only to registered
  agents, and stale transcript responses cannot erase newer streamed events or resolved permissions.
  Expanded cards leave dnd-kit sorting, keep activation on the header rather than pane controls, and
  silently evict the least-recently-used pane at the cap. Server layout validation and round-trip
  coverage, 300 UI tests, `make check-specs`, `make build`, `make test`, `make dist`, and whitespace
  checks are green. A real Chromium pass at 1280×800 verified fixed 640px pane geometry, two-column
  span, stable neighboring cards and page position during streaming, internal long-transcript
  scrolling, focus cycling, persistence after reload, and rendering under Core and Sky & Grove.

- **2026-08-29 — design-feature:** Resolved the 2026-08-29 design review of the expandable chat
  pane change. All five findings and all three consistency notes are closed in the specifications;
  none was rejected, because each named a real consequence that survived checking against the tree.
  Two changed the design rather than its wording. `AgentCard` carries `onClick`/`onContextMenu` on
  its outer `<article>` and the pane composes inside it, so an ordinary Send, permission decision,
  or autocomplete accept would have collapsed the pane or opened the card menu; new FS-02.R52 and
  A34 move both handlers to the card header for an expanded card, and TS-08.R43 records why that
  structural boundary was chosen over per-control `stopPropagation` — an opt-out list is the INV
  §2/§10 drift shape, since every control added later must remember to join it and a miss fails
  silently. And `setTranscript` replaces an agent's whole slice, so a refetch resolving after newer
  deltas drops them or reverts a resolved permission chip; TS-03.R31 now specifies the two repairs
  FS-03's own §6 deviation already named — a per-agent request token (INV §1's canonical pattern,
  already used by `FilesTab`/`CommandsTab`) and seq reconciliation that retains events newer than
  the response's maximum — so the four-fold fan-out closes that advisory instead of multiplying it,
  for the agent screen as well as the panes. The three lower-severity findings tightened evidence
  and definitions: FS-02.A29's geometry claims moved to J5 because jsdom evaluates no grid sizing,
  stretch, overflow, or scroll position (INV §13); R48 now names the three events that mark a pane
  as used, including a pointer press, because the transcript's scroll region is not focusable and a
  reader would otherwise lose the pane they were reading; and A30 gained the running→stopped
  transition, which every previously named case would have passed while closing the pane. The
  consistency notes closed as FS-02.R46's corrected host, TS-08.R42's narrowed scroll-region claim
  (`.annotation-tray-body` and `.composer-picker` also scroll), and new FS-12.R38/A14 scoping
  R15's no-new-shortcut clause to the presentation change it was written for.

  One defect the review did not raise is also fixed, and it was the more damaging one. R49 said an
  expanded id outside the grid's current scope is dropped from the next save — but `CardGrid` mounts
  only at `/project/:project`, so *every* id belongs to some other project the moment the operator
  opens a different one, and the debounced `PUT` would have wiped the first project's arrangement on
  arrival at the second. R49 and A32 now retain an out-of-scope id unrendered and write it back
  unchanged; only an unknown or archived agent is pruned. That same correction disposes of the
  original wrong claim that the projects home hosts an agent grid: it renders project cards, and
  after FS-02.R29's project-first split the project dashboard is the only agent-card surface.
  FS-12 joins FS-02, FS-03, TS-03, and TS-08 as Partial. No product code changed. `make
  check-specs`, the skill-twin comparison, and `git diff --check` are green.

- **2026-08-29 — design review correction:** Removed the projects-home Must-fix finding after the
  user clarified that "projects page" means the agent-card dashboard reached after selecting a
  project (`/project/:project-id`), not the root project-card catalog. That intended host is the
  existing `CardGrid` mount and is buildable. The misleading "projects-home and scoped project
  grid" wording remains a consistency note for follow-up design cleanup. Two Must-fix and three
  Worth-fixing findings remain; no product code, specification, or change file was edited.

- **2026-08-29 — design review:** Reviewed the waiting expandable-chat-pane change against FS-02,
  FS-03, FS-12, TS-03, TS-08, every invariant class, the affected grid/chat/SSE/layout code, and the
  incumbent Core visual matrix at 1280×800. Three Must-fix and three Worth-fixing findings are
  recorded below. The promised projects-home host does not exist after FS-02.R29's project-first
  split; an expanded pane composed inside `AgentCard` inherits root click/context-menu handlers that
  would collapse it or open the card menu during ordinary chat interaction; and concurrent
  transcript refetches can replace newer SSE deltas because `setTranscript` is an unguarded replace.
  Lower-severity gaps leave actual pane geometry without browser evidence, least-recent focus
  undefined, and the stopped-pane promise without acceptance coverage. Two page-only consistency
  notes record the `only scroll region` overstatement and FS-12.R15's stale no-shortcut clause. No
  product code, specification, or change file was edited; the change remains Waiting to start and
  is blocked from implementation until follow-up feature design resolves the Must-fix items.

- **2026-08-29 — design-feature:** Specified expandable chat panes on the agent card grid. A
  chat-interface card now expands in place into a transcript-plus-composer pane spanning two grid
  columns, up to four at once, so an operator can read one agent, answer a blocked one, and keep the
  rest of the grid's state on screen instead of paying a route round-trip per agent. Card-body click
  toggles expansion, which supersedes FS-02.R8's navigate clause; right-click, the context menu's
  **Open chat**, and `/agent/:id` itself are untouched, and the pane's name links to that page. The
  `/ux` trigger passed — this changes an established, many-times-per-hour task — and its walkthrough
  changed the design twice: a card expanding inside `repeat(perRow, 1fr)` would be *one column* wide
  and so would worsen the space complaint it was meant to fix, and a pane that grows the page rather
  than scrolling internally would let one agent's stream move another agent's reader. Both are now
  pinned by FS-02.R47 and TS-08.R42, which also record that `.card-grid` must gain `align-items:
  start` (it declares only `display: grid` today, so the default `stretch` would inflate every
  collapsed card sharing a pane's row) and that `grid-auto-flow` must never become `dense`, since
  dense packing reorders items and FS-02.R47 forbids expansion changing order. Four decisions were
  the user's and are recorded as chosen: click-to-expand over a separate chevron; a four-pane cap
  that silently collapses the least-recently-focused pane, which loses nothing because drafts are
  already per-agent and browser-local (FS-03.R36); persistence in `layout.json` beside order,
  density, and group collapse; and `Ctrl+Alt+ArrowDown`/`ArrowUp` focus cycling. Two exclusions were
  proposed and accepted: no card ever auto-expands from a notification or state change, so the grid
  never reflows while it is being read, and expansion belongs to the card grid, so it works on the
  projects home as well as a scoped project. Terminal-interface cards keep navigating, because the
  pane deliberately carries no terminal. The load-bearing technical finding is that `ui/src/api/sse.ts`
  holds a single `openAgentId`: only that agent's deltas are appended and only its sequence gaps are
  refetched, so "several live chats" is precisely what today's client cannot do. TS-03.R31 turns it
  into a reference-counted set whose registration returns its own teardown (INV §4), keeps the set
  alive across reconnect while `lastPing`/`hydrationIds`/`lastAgentSeq` keep resetting on `onopen`
  (INV §1), and bounds the reconnect refetch fan-out to the same four panes so it cannot re-create
  the origin connection-pool exhaustion TS-03.R7 exists to prevent (INV §7). `layout.json` gains one
  additive `expanded` list; a file written before this change reads unchanged and needs no
  migration. TS-08.R41/R43 hold the pane to composing the shipped `TranscriptView` and `Composer` —
  both already `agent_id`-parameterized with per-instance scroll refs — rather than growing a second
  chat surface (INV §2), and reuse the existing collapsed-section filter to keep an expanded id out
  of dnd-kit's sortable list. FS-02 and FS-03 add R46–R51/A29–A33 and R39/A23; FS-02, FS-03, TS-03,
  and TS-08 move to Partial with every new item tagged `(planned)`. No product code changed.
  `make check-specs`, the skill-twin comparison, and `git diff --check` are green.

- **2026-08-28 — workflow:** Retuned `/ux` for AgentDeck's actual small, experienced internal
  operator set. Automatic use now starts with a no-artifact trigger check and runs a full pass only
  when work changes an established task or introduces an unfamiliar or consequential decision,
  ambiguous state, recovery path, long-running operation, or AI uncertainty; ordinary user-facing
  additions and familiar interactions stay in their normal workflow. The task frame now defaults to
  one primary repeated-use task, adds a second only for a materially different high-risk branch,
  and explicitly favors speed, density, keyboard flow, predictability, learned shortcuts, and
  control over hypothetical-novice simplification. Walkthrough questions use the operator's real
  knowledge and habits; onboarding and rediscoverability apply only when the task makes them real.
  Real-browser validation now runs only when rendered interaction, timing/state, recovery, or an
  unresolved design risk can change the judgment, plus explicit standalone critique; behavior fully
  established by acceptance tests does not earn the harness cost. The canonical workflow, routing
  summary, and Claude/Codex launchers agree; skill twins, YAML parsing, specification checks, and
  whitespace checks are green.

- **2026-08-28 — workflow:** Added the twinned first-party `/ux` skill and canonical workflow §15.
  It accompanies `/design-feature` automatically before meaningful user-facing behavior is
  confirmed, joins implementation only when acceptance materially depends on task flow, state,
  consequence, recovery, long-running work, or AI interaction, and remains available as a focused
  read-only critique. The workflow distills cognitive walkthroughs, NN/g heuristics and focused
  patterns, Microsoft HAX, CLI Guidelines, Good Services, and the non-duplicative PAIR caution
  against implying human understanding into task framing, a two-question walkthrough, behavioral
  contract hardening, and focused real-product validation. Findings must name the actual task,
  observed friction, consequence, evidence, and a proportionate repair. `/design` continues to own
  visual composition and polish while sharing task frames and browser passes where the lenses
  overlap; `/usability-review` continues to own the product-wide acceptance matrix. Independent
  forward tests against the Pipelines split, Mermaid rendering, and running-first card placement
  recovered the existing blocked-run chat/Continue failure, produced no Mermaid false positive,
  and isolated cross-boundary drag feedback as an unverified J5 risk rather than a finding. The
  internal SharedWorker fallback correctly did not trigger the skill. Specification checks, all
  skill-twin comparisons, YAML frontmatter parsing, and whitespace checks are green. The bundled
  skill validator itself could not start because the host Python lacks PyYAML; its covered
  frontmatter/name/placeholders were checked directly instead.

- **2026-08-28 — work:** Shipped running-first card placement. Inside every group section of an
  agent card grid, running agents now render before stopped ones and the persisted manual order
  survives inside each of those two blocks, so a card crosses the boundary the moment its agent
  starts or stops with no reload. `running` is the sole test: the live `state` values still express
  themselves through salience alone and never move a card, which is why FS-12.R10's "without
  changing order" clause is narrowed by FS-12.R37 instead of being contradicted. Nothing new is
  persisted — `layout.json` keeps exactly the order, density, and per-group collapse state it held
  before, and a same-block drag still commits the identical flat manual order it committed before
  the split existed. Two things the split forced: dnd-kit now receives the order the cards actually
  render in, skipping collapsed sections whose cards mount no sortable node, because its indices and
  rect transforms are derived from that list; and a drop onto the other running/stopped block
  returns before `arrayMove` and before any layout write, so manual drag cannot override the
  boundary or make a hidden change (INV §10). Seven new `CardGrid.test.tsx` cases cover placement,
  the live flip, the registry order under a collapsed section, salience-without-movement, Ungrouped
  staying last, the same-block payload, and the cross-block no-request; five fail against the
  pre-change component. FS-02 and FS-12 have no `(planned)` items left, so both move to Current in
  their headers and the spec index. `make test` (both Go variants), `make build`, `make embed`,
  `make check-specs`, `git diff --check`, `tsc --noEmit`, `npm test` (289 passing) and `npm run
  build` are green. J5's real-browser half is owed: no browser was available in this session.

- **2026-08-28 — docs:** Replaced `/design`'s rigid one-repair/one-confirmation stopping rule with a
  quality gate. Agents still batch rendered findings, but they continue repairing and re-rendering
  while material in-scope problems remain; they stop when the direction and material findings are
  satisfied, without drifting into subjective polish. The twinned launcher and canonical workflow
  use the same rule.

- **2026-08-28 — implementation:** Added the twinned first-party `/design` skill and canonical
  workflow §14 after researching Impeccable, Emil Kowalski's design/animation skills, Anthropic's
  frontend-design skill, and designer-skills' design-review flow. Automatic discovery is deliberately
  narrow: it applies to new screens, redesigns, meaningful composition/styling, motion, polish, and
  critique, while routine frontend wiring and style-preserving fixes stay in their normal role. The
  workflow records one compact direction, derives specificity from AgentDeck's operator and
  lifecycle/coordination states, gates motion by purpose and frequency, extends TS-08's tokens,
  primitives, hooks, appearances, and visual matrix, and requires a bounded real-browser critique
  that continues while material in-scope problems remain and stops before subjective polish. It adds
  no product code, design-system layer, vendored skill pack, detector, script, screenshot baseline,
  or runtime dependency. Standalone critique remains read-only, and automatic invocation never
  selects or enlarges work. Independent forward tests covered an open Pipelines redesign, an
  incidental API/UI field, and a read-only Dashboard critique; the skill twins, YAML frontmatter,
  specification checks, and whitespace checks are green.

- **2026-08-28 — fix:** Closed the Tasks Retry finding from the `790c01c` review (INV §10). The
  view now gates Retry on the same predicate the server uses — `interrupted`, or `dependency_failed`
  with no `unsatisfiable` arm — instead of on `interrupted` alone. A task parked because its three
  start attempts were spent, or because its target became ineligible, gets back the repair FS-16.R23
  names for it; a task parked by an arm that can never be satisfied still shows Re-arm and no Retry,
  so the view never offers a control that would be refused. No behavior beyond the specification
  changed, so this restores FS-16.R23/R25 rather than altering them; A11 gains the UI half of its
  verification, which had no home before. A new regression covers both parked kinds in one list and
  fails against the pre-fix gate. `make test`, `make build`, `make embed`, `make check-specs`,
  `git diff --check`, `tsc --noEmit`, `npm test` (282 passing) and `npm run build` are green.

- **2026-08-28 — fix:** Closed the one Must-fix finding from the `790c01c` review (INV §6/§8/§12).
  The shared-worker SSE transport now has a reachable failure path: construction runs inside
  `try`/`catch`, the `SharedWorker` object's `error` event is handled, and a stream that never opens
  within the liveness window is demoted rather than reconnected into. All three routes fall back to
  the direct `/api/events` stream for the rest of the session, so a browser that exposes
  `SharedWorker` but cannot run it — or a worker asset that fails to load — no longer leaves the
  dashboard on `connecting` with no live data, no error and no retry. TS-03.R7 now states that
  sharing is best-effort and names the fallback, which was previously unspecified. `sse.test.ts`
  gains a `SharedWorker` stub and four cases: port fan-out including a ping satisfying the liveness
  window, and one per fallback route; each fails against the pre-fix code. Bookkeeping while
  verifying: the FS-02.A27 finding is narrowed to the un-run six-tab browser check now that the
  transport has tests, and a new finding records that `index.html` and one worker asset are
  force-tracked under the ignored embed directory. `make test`, `make build`, `make embed`,
  `make check-specs`, `git diff --check`, `tsc --noEmit`, `npm test` (281 passing) and `npm run
  build` are green.

- **2026-08-28 — review:** Read the shipped diff of `790c01c` (the thirteen dashboard/SSE and
  usability fixes) against FS-02, FS-04, FS-08, FS-16, FS-17, TS-03, TS-07, TS-10 and every
  invariant class. Eight findings recorded below: one **Must fix** and seven **Worth fixing**. The
  headline fixes are real — tabs do share one SSE stream, reconciliation is debounced to the changed
  session, card previews no longer clone the whole transcript map, and the six usability items are
  closed — but the shared-stream transport ships with no failure path and no test, and the Tasks
  Retry gate hides the repair FS-16.R23 requires. Two findings changed after the user reviewed them
  on 2026-08-28: the narrowed config-source publication gate (`changedFields` suppressing every
  change outside model/effort/assets, so an open Settings → Sources panel keeps a superseded view
  until reload) is the user's accepted boundary and is removed, leaving the amended FS-08.R15 as
  written; and the Tasks Retry finding drops to **Worth fixing** because the blank Re-arm form does
  return the task to `ready`, so the defect is discoverability and FS-16.R23 conformance rather than
  unrecoverable work. Classes with no finding: §3 (the create-then-edit switch in `ProjectsEditor` is
  correct), §5 (the reconcile debounce timer's stop/drain/reset is race-free), §7
  (`lastAssistantPreview` returning empty on an unreadable path cannot blank a card, because
  `ApplyStaleCorrection` refuses an empty detail), §9, §11 (`changedFields` returns a non-nil slice),
  §13 (the diff ships no new className), §14 (no new route; the worker's `/api/events` request is
  same-origin and inherits `localOnly`) and §15. `make check-specs`, `git diff --check`, `go test`
  for `internal/{configsource,server,state}`, `tsc --noEmit` and the 277-test UI suite are all green.
  `Last reviewed code` stays at `6a16126`: this review read one named commit, not a continuous range.

- **2026-08-28 — docs:** Corrected the review-state bookkeeping. `Last reviewed code` moves from
  `895348e` to `6a16126`, the last code commit actually read — `bbbdc90` verified it and did not
  advance the pointer. Review state now names the four commits whose diffs have had no §7 code
  review (`790c01c`, `c35ff8c`, `9114df7`, `69c2f99`), so the stale pointer stops implying that
  the twenty-plus commits behind it are all unreviewed when most are review sessions themselves.

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

- **2026-08-28 — fix:** No open review findings. `## Review findings

From the 2026-09-02 bug investigation of a field report on one 23h24m pipeline run: it completed
with outcome `success`, but the person had to act as the orchestration control plane throughout —
approving control-plane calls, choosing between Continue and Retry, preserving partial work across
attempts, authorizing isolated workspaces, resuming an earlier session, waking a coordinator,
relaying an already-completed result, and finally asking where the run's declared report was
stored. No AgentDeck version, environment, or server log came with it, and this machine's
`~/.agentdeck` holds nothing newer than 2026-08-27, so it is not the incident home: the diagnosis
below is from the code path and two local reproductions, not from incident logs. Confidence is
labeled on each finding.

- **Must fix** (confirmed) (FS-14.R19/R47, FS-17.R1–R3, TS-09.R8; INV §2/§8) —
  `internal/pipeline/assignment.go:35-40` tells every stage agent to "call
  `report_pipeline_stage_result` exactly once", that "that one call ends your part in this
  assignment", and that "only then is another report accepted". None of it distinguishes a call
  AgentDeck **accepted** from a call it **refused**, while `internal/messaging/tools.go:50-61`
  classifies `validation_failed` as `after_change` and `pipeline_unavailable` as `transient` —
  refusals FS-17 exists to invite a second call for. Normal-use trigger: any retryable refusal of a
  stage report, of which the most reachable is `validation_failed` for a destination stage whose
  required inputs are not yet resolved (`internal/pipeline/actions.go:263`) — precisely the shape of
  a fan-out stage reporting while some children are still producing values. The attempt is untouched
  by the refusal, so it still owes a result and only that agent can supply it; the agent has been
  told not to. In the reported run this happened twice and cost about nine and about ten hours, and
  both times the same preserved result was accepted as soon as a person told the agent to try again.
  Neither the tool description (`internal/messaging/messaging.go:174`) nor any refusal message
  explains what a `retry` class means, so the instruction the agent re-reads every turn wins.
  Reproduced by the committed skipped test `internal/pipeline/refused_report_retry_test.go`
  (`TestRefusedStageReportMustBeRetryable`), which shows the refused report leaving the run at
  `await_result` and the corrected retry being accepted. Fix: state the boundary in terms of the
  accepted result rather than the tool call, in the assignment and in the refusal, the way R47
  already states the blocked boundary in all three places.

- **Must fix** (confirmed) (FS-14.R7/R29/R37, TS-09.R17; INV §8/§10) — nothing detects a stage
  attempt whose agent can no longer advance it. `Manager.OnTurnEnd`
  (`internal/pipeline/actions.go:304`) returns `nil` unless the attempt already has an accepted
  report, so a turn boundary reached with nothing reported is silently ignored; there is no ticker,
  timer, or sweep anywhere in `internal/pipeline`; `internal/server/reconcile.go`'s 30s sweep never
  touches pipelines; and `Manager.Startup` (`manager.go:312-361`) re-pauses an interrupted
  `await_result` only at process boot. FS-14.R29's two notification categories — blocked, approval
  gate, launch failure, crash, and completion — exclude this state by enumeration. Normal-use
  trigger: the finding above, a permission timeout on the report call, or any turn that ends without
  one. A run sits at `state=running, pending_action=await_result` producing no publish, no
  notification, and no log entry, and R37's attention reason stays empty, so the run page renders it
  identically to a run being actively worked — which is what left this run idle for about nineteen
  hours in total. This needs a product decision before it can be fixed, because a stage agent
  legitimately ends many turns while waiting on delegated children: the choice is what signal
  qualifies (an idle agent with no running delegated tasks, an elapsed-time threshold, or an
  explicit heartbeat) and whether it becomes an attention reason under R29 or a weaker disclosure
  under R40. Recorded under *Decisions needing your input* below.

- **Must fix** (confirmed) (FS-06.R4/R22, FS-15.R17, FS-16.R12/R20, TS-10.R13; INV §8) —
  `internal/messaging/task_tools.go:252` refuses a task aimed at a stopped pipeline agent with
  `"No agent matches %q."`, naming an agent that exists, is not archived, has a resumable session
  snapshot, and that the same caller can share context with in the same turn. The exclusion itself
  is specified and shared correctly with messaging through one resolver (`stoppedWakeGates`,
  `internal/state/messages.go:40`, FS-06.R22), and the context plane's looser set is deliberate
  (FS-15.R17), so the divergence is by design — the message is not. Normal-use trigger: a later
  pipeline stage delegates work back to the coordinator an earlier stage used, which is what the
  reported run did. `recipient_not_found` is classified `after_change`, which is the right class
  because resuming the agent does make the call succeed, but nothing names that change, so the
  person had to diagnose it and resume the session by hand. This is the same defect class as the
  2026-08-28 `stale_assignment` report. Reproduced by the committed skipped test
  `internal/messaging/pipeline_agent_task_target_test.go`
  (`TestTaskAimedAtAStoppedPipelineAgentNamesTheRealCondition`), which shares context with the agent
  and then gets told it does not exist. Fix: name the condition and the change in the refusal for
  both `create_task` and `send_message`. A second, product-level question rides on it: a
  `pipeline_attempts` row lives as long as its run record (`ON DELETE CASCADE`,
  `internal/state/schema.go:237`), so **any** agent that ever ran one stage is unaddressable while
  stopped, forever, long after that run ends — recorded below.

- **Worth fixing** (confirmed) (FS-03.R17, FS-14.R15; INV §8) — a permission prompt on a
  control-plane call auto-denies after 180s (`internal/runtime/permission.go:13`) and finishes the
  turn without executing the tool, and the pipeline never learns. The reported run recorded 10 such
  timeouts across its three coordinator sessions. FS-14.R15 correctly forbids a pipeline from
  treating a permission prompt as a failed or completed stage, and nothing does — but nothing
  records the converse either: the only trace that `report_pipeline_stage_result` never ran is a
  generic `permission timed out` error event inside that agent's own transcript. `dashboard.log`
  carries every HTTP request and every pipeline refusal (`refuseReport`, `actions.go:287`) but not
  one denied or timed-out control-plane call, so a run stalled this way is undiagnosable from the
  server log — this report could only be reconstructed because the person watched it happen. Log the
  denial and the timeout with the tool name, agent id, and decision, and treat a denied control-plane
  call as an input to whatever signal the finding above grows. The volume of these prompts is
  already the user's own 2026-09-02 entry in `docs/ideas.md`; what that entry does not yet say is
  that the 180s auto-deny turns the friction into lost pipeline progress.

- **Worth fixing** (confirmed) (FS-14.R20/R37/R46; INV §10) —
  `ui/src/features/pipelines/RunBrowser.tsx:140` disables **Continue** until the continuation
  textarea holds non-whitespace text, with no `title`, no adjacent message, and no required marker,
  and `:108-111` leaves **Continue** and **Retry stage** both valid at an ordinary `blocked` pause
  with nothing distinguishing them. The one piece of copy that explains Retry — "Retry the stage to
  run it again with a fresh agent" at `:137` — renders only on the `recovered` branch R48 added,
  where Continue is withheld and there is no choice to make. Normal-use trigger: the ordinary
  blocked pause, which is the pause a person meets most. R46 already requires exactly this pattern
  ("names the value it is still waiting for beside that control") for the start dialog, so the
  product has the answer and does not apply it where the user actually got stuck: in the reported
  run the person asked which action was correct, was advised Continue, then Retry, and had no
  statement anywhere that Retry means a fresh agent carrying only bounded prior summaries
  (FS-14.R12/R20). Fix: name the missing continuation input beside the disabled control, and state
  each action's consequence where both are offered. Test both in `RunBrowser.test.tsx`.

- **Worth fixing** (confirmed) (FS-14.R23/R38; INV §10) — a stage's declared outputs are shipped in
  the run-detail payload as `report_outputs` (`ui/src/schemas/pipeline.ts:117`) and rendered
  nowhere: `TimelineAttempt` (`RunBrowser.tsx:189-193`) draws only the summary, details, checks, and
  agent cards, and the only place an output's text appears is the **Named values** `<details>` at
  `:161`, which has no `open` attribute while **Frozen setup** beside it does. Normal-use trigger:
  any run whose last stage declares a substantial text output — the reported run's final review
  report. R23 is satisfied by the named-value list, and §6 is right that AgentDeck delivers an
  output nowhere, but the value is closed by default and decoupled from the attempt that produced
  it, so the person ended the run not knowing where their report was and had to ask the stage agent
  to reproduce it. A field that ships in the API and reaches no surface is INV §10's own case. Fix:
  render an attempt's declared outputs in its timeline entry, and open the named values disclosure
  on a completed run.

**Read as specified, not findings.** Recorded so the fix session does not chase them: an accepted
`blocked` result is terminal while delegated children keep running, and the run page's count of them
is disclosure only (FS-14.R40, TS-09.R28 — implemented, `pipeline_projection.go:66-80`); Retry
creates a fresh agent and carries forward only one bounded summary line per prior reported attempt
(FS-14.R12/R20, `assignment.go:57-68`), and no work-unit, checkpoint, or resumable-task-identity
concept exists in the template schema at all; a run claims no filesystem isolation and provisions no
workspace, and nothing wires FS-19's worktree fork to a stage or a task (FS-14.R21, TS-09.R18,
FS-19.R5); the coordinator's permission mode comes from its role and the global default with no
pipeline-level override, by FS-14.R15's explicit intent; attaching a context reference requires the
**creator** to be able to read it, never the assignee (FS-16.R20); the 15-message per-turn budget
refused the wake-up message exactly as FS-06.R11/R18 describe; and the run's outcome is the
reporting agent's own word against a template-authored success condition, so a review stage that
reports `success` with unresolved findings produces a `success` run (FS-14.R6/R34). Four of these —
partial-success checkpoints across a Retry, isolated workspace provisioning for delegated work, a
durable stage handoff artifact, and an honest terminal outcome for a completed-but-unclean review —
are real product wishes that no requirement covers and that `docs/ideas.md` does not yet record.
They are left for the user to place, because that file carries uncommitted work of theirs.

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
  which is the ordinary shape of a diagram reply. Reproduced against the shipped component —
  after the diagram settles, one appended delta drops the `<svg>` back to the source code block and
  re-invokes `mermaid.render` with a fresh id (`ad-diagram-1` then `ad-diagram-2`). That is exactly
  the reported "spazzing between display and source", and it contradicts R37's "the reader therefore
  never sees a diagram flicker or error mid-stream"; it also repeats uninterruptible main-thread
  Mermaid work per delta, which is the cost TS-08.R40 bounds the input to avoid. Note TS-08.R40's
  new sentence is scoped to "while message text is unchanged", so the technical spec currently
  ratifies the gap rather than closing it. Fix: hold `text` in a ref updated each render, read
  `textRef.current` inside the `code` component, and memoize with `[]`; then widen R40. Test: in
  `AssistantText.test.tsx`, rerender with `CLOSED + "\ntrailing prose"` after the diagram settles
  and assert the SVG is still mounted and `mermaid.render` was called once. Confirmed still open at
  `e46e66b`.

- **Must fix** (TS-06 §2, workflow §2/§5; INV §10) — `e46e66b` changed `ui/src`
  (`AgentCard.tsx`, `CardGrid.tsx`, `ContextBar.tsx`, `VisualMatrix.tsx`, `dashboard.css`) but did
  not commit the regenerated `internal/server/ui/dist/index.html`; the rebuilt file naming
  `index-MJrIrA5t.js` / `index-Ccp6fmj4.css` is sitting uncommitted in the working tree while `main`
  still carries the pre-change `index-DoXOpwRi.js` / `index-DYVxYPFB.css`. That file is the one
  tracked artifact of the generated embed directory (`.gitignore:10-11`), and every earlier
  UI-changing commit back through `180ea89`, `57b7154`, `82d1717` and `34348c8` updated it in the
  same commit. Normal-use trigger: `git pull` onto a working copy that already has a populated
  `internal/server/ui/dist/assets/`, then `make build` — which the Makefile documents as "assumes
  the embed dir is populated" — embeds an `index.html` pointing at the previous bundle, so the
  dashboard silently serves the pre-card-change UI with no error anywhere. Because the assets are
  gitignored, nothing in the tree reveals the mismatch. Fix: run `make embed` (or `make dist`) and
  commit the refreshed `internal/server/ui/dist/index.html` alongside the UI change; re-check that a
  fresh `ui/dist/index.html` is byte-identical to the tracked copy before closing any UI change.

- **Must fix** (FS-19.R7, TS-12.R4/R5, TS-01.R26; INV §2) — TS-12.R4 routes
  `ensureWorktreeCheckout` — a helper that creates a directory, runs `git worktree add`, and
  re-runs `setup_command` for up to ten minutes (TS-12.R5) — through pipeline `ValidateStage`
  as one of its four call sites, "replacing the two currently duplicated cwd stat checks
  (`internal/server/launch.go`, `internal/server/pipeline_lifecycle.go`)". `ValidateStage`
  (`internal/server/pipeline_lifecycle.go:31`) is documented as checking "without registering or
  starting a process", and `internal/pipeline/manager.go:215` calls it **once per stage** inside the
  run-start validation loop, folding any error into a per-assignment `"unavailable"` diagnostic
  rather than an operation error. Normal-use trigger: starting a run in a worktree project whose
  checkout was cleaned up — a 32-stage template calls the mutating helper 32 times inside one
  validation pass, the pre-flight blocks on a checkout recreation plus a setup command (the server
  sets no `WriteTimeout`, `internal/server/server.go:337`), and a recreation failure surfaces as a
  bogus "this stage's backend/model assignment is unavailable" diagnostic. FS-19.R7 itself scopes
  recreation to "a launch", so FS-19 and TS-12 also disagree on the call sites. Fix before
  implementation: keep `ValidateStage` read-only (an ownership-aware *check* that the checkout is
  present or recreatable), place the mutating recreation on the stage's actual start path with the
  other three call sites, and state in TS-12.R4/R5 what a start request does while a ten-minute
  setup runs.

- **Worth fixing** (TS-12.R3; INV §15) — TS-12.R3 fixes fork ordering as "create branch and
  worktree → insert the ownership row → write the new project file → run setup" and then claims "a
  crash inside the window can only leave a checkout that is *not* recorded as owned ... so every
  crash residue is inert and conservatively treated as external". That is false for the window its
  own ordering opens: a crash after the ownership row is inserted and before the project file is
  written leaves a checkout that **is** recorded as owned, belonging to a project that does not
  exist. Normal-use trigger: the server is stopped or killed during a fork. Because TS-12.R7 gates
  every deletion inside `beginProjectArchive` for an existing project, no path ever reaches that
  row, so the branch, the directory under `$AGENTDECK_HOME/worktrees/`, and the row leak with no
  reclaim path, and the safety argument the requirement rests on does not hold as written. Fix:
  insert the ownership row **last** (worktree → project file → row), which makes the residue an
  unowned checkout exactly as R3 claims and leaves a visible project the person can act on; or keep
  the ordering and specify a startup reconciliation that drops `project_worktrees` rows with no
  project. Test: inject a failure between the row insert and the project write and assert what
  survives.

- **Worth fixing** (FS-19.R7, TS-12.R4/R10; INV §5) — TS-12.R10 names branch-ref creation as the
  atomic claim for concurrent **forks**, but nothing claims the concurrent **recreation** R4
  performs. Normal-use trigger: a worktree project's checkout is gone and the person launches two
  agents in it, or a pipeline starts two stages, within the same moment — both callers stat the
  missing directory, both run `git worktree add` for the same branch and path, and the loser fails
  with a raw Git error ("already exists" / "already checked out") on a path AgentDeck owns and
  intended to recreate. This is the check-then-act shape INV §5 names, and the design says nothing
  about serializing it. Fix: give `ensureWorktreeCheckout` a per-project claim (take-and-hold under
  one critical section, re-stat after acquiring) and state it in TS-12.R4 the way R10 states the
  fork claim.

- **Worth fixing** (FS-19.R8/§3, TS-12.R7, FS-04.R35; INV §10) — FS-19 never says what a worktree
  project does after its owned checkout is deleted by consent and the project is then reactivated,
  and TS-12.R7's two paths answer it in opposite ways. FS-04.R35 makes reactivation ordinary —
  "either state can be changed without re-creating the project", and an archived project keeps every
  agent/session record for restore. Normal-use trigger: archive a finished worktree project,
  accept the checkout deletion, then change your mind and reactivate it to resume an agent. On the
  normal path R7 deletes the ownership row, so the project is no longer owned, `ensureWorktreeCheckout`
  does nothing, and the resume fails on a missing `cwd` with the ordinary error — even though the
  branch it needs still exists and FS-19's whole model is "the checkout is disposable, the branch is
  durable". On R7's own stated crash path the row survives, so the same action silently recreates
  the checkout instead. Fix: decide the behavior once and state it in FS-19 §3 — either keep the
  ownership row through a consented deletion so reactivation recreates from the recorded branch, or
  state plainly that consented deletion is one-way and offer the person a way back.

- **Worth fixing** (FS-19.R3/A2, TS-12.R5; INV §8) — TS-12.R5 captures the setup command's combined
  output with "a bounded tail (64 KiB)", stores it on the ownership row, and returns it on the fork
  and start responses; FS-19.R3 then says the failure "surfaces as a visible warning carrying the
  captured output". 64 KiB is a storage bound, not a display bound, and nothing states what the
  warning renders. Normal-use trigger: a `setup_command` such as `npm install` fails after tens of
  kilobytes of progress output, and the fork's warning surface receives the whole tail. INV §8
  requires human-facing text to be parsed, human-meaningful and length-clamped at the write
  boundary — this is the same shape as the raw-payload surface the FS-14 idea below exists to fix.
  Fix: state a display clamp and the full-output route (an expandable region or a log link) in
  FS-19.R3, keeping 64 KiB as the storage bound only.

- **Worth fixing** (FS-02.R47/R55; INV §10) — `docs/specs/features/FS-02-dashboard.md:329` still
  requires that a collapsed card dragged past a pane "must see the pane's **two-column** footprint",
  and R55 supersedes only "R47's `min(2, perRow)` span" while asserting "every other clause of R47
  stands", which makes the stale clause normative. The shipped pane spans one track. TS-08.R43 was
  corrected in the same change to "a wider-or-taller footprint" and `CardGrid.tsx:121` to "one
  column"; FS-02.R47 was not. Normal-use trigger: a later reader trusts R47, concludes the grid is
  wrong, and reintroduces the two-track span R55 exists to remove. Fix: correct R47's footprint
  clause in place the way TS-08.R43 was corrected — the reason to keep the expanded id in its
  `SortableContext` still holds, because the pane is taller than its neighbours. Confirmed still
  open at `e46e66b`.

- **Worth fixing** (TS-08.R14/R48, FS-02.R59; INV §2) — `ui/src/components/grid/ContextBar.tsx:6`
  emits `data-variant={compact ? "compact" : tone}`, so one attribute carries two orthogonal
  dimensions and the compact meter exposes no low/medium/high tone through the presentation
  contract. The tone survives only on the `context-bar high` className, which TS-08 §3.3 excludes
  from the skin surface. Normal-use trigger: a skin styles `[data-ui="context-meter"]
  [data-variant="high"]` red, and the expanded card's meter — the only context reading FS-02.R59
  leaves on the dashboard — silently keeps the default ramp. Nothing is visibly wrong today because
  no shipped skin reads this hook, which is why it is Worth fixing. TS-08.R48 says the compact form
  "differs from the full meter in presentation only", yet it drops a contract hook. Fix: keep
  `data-variant` as the tone and express density separately (a second `data-*` dimension registered
  in `contract.json`), and extend `ContextBar.test.tsx` to assert a compact meter still reports its
  tone through the contract attribute. Confirmed still open at `e46e66b`.

- **Worth fixing** (INV §10; `docs/specs/README.md` lifecycle) — an unrelated, unattributed FS-14
  design still sits uncommitted in the tree, unchanged since the last review: R49–R51 and A27/A28
  are added as `(planned)`, FS-14 is flipped to **Partial**, and `docs/ideas.md` gains the
  proposal-collapse idea, but there is no `docs/ready-changes/` file, no brief, and no handoff
  changelog entry for it. It is also incomplete in its own terms: A27 names
  `internal/state/pipeline_proposals_test.go` and `internal/server/pipeline_handlers_test.go`, so
  R49 needs a durable declined state and new endpoints, yet TS-09, TS-02 and TS-03 carry no planned
  requirement for either and FS-14 §7 gains no traceability entry. Normal-use trigger: nothing in
  the approved-work list points at these requirements, so either they are lost with the tree or an
  agent later finds planned product requirements with no owner and no architecture. Fix: either
  finish that design session — technical-spec coverage, ready-change file, brief, commit — or revert
  the FS-14 and `ideas.md` edits; do not leave planned requirements alive only in an uncommitted
  tree.

Closed by this review: the previous cycle's two **Must fix** findings are gone. `make check-specs`
now reports `spec check: ok` on the delivered tree, and `e46e66b` commits the whole card-grid change
— implementation, tests, the FS-02/FS-12/TS-08 updates, the spec-index status flips, the brief, and
the changelog entry — with `docs/ready-changes/README.md` and the handoff changelog delinked from
the deleted ready change.

Invariant sweep of the `180ea89..e46e66b` range and the uncommitted FS-14 design: §1 holds —
`collapseAll` republishes only the current grid's ids and merge-preserves the retained
out-of-project ones, and TS-08.R49 resets its last-observed-state record on hydration and
reconnection rather than trusting it across the boundary, which is this class handled correctly at
design time. §2 produced two findings (TS-12.R4's misapplied seam and the carried `ContextBar`
contract split); `ContextBar` otherwise stays the single context derivation and TS-12.R4 correctly
collapses two duplicated cwd checks into one helper. §3 holds: `collapseAll` writes a filtered list,
never its on-screen view. §4 has no shipped surface; TS-12 §4 declares an explicit opt-out for the
checkout with the FS-11 project-resources precedent and confines teardown to the consented deletion
flow, which is a stated deviation rather than a silent one. §5 produced the concurrent-recreation
finding; the fork's branch-ref claim (TS-12.R10) is right. §6 has no new runtime or driver. §7 holds
— TS-12.R6 isolates per-project Git failures so one unreadable repo degrades that project's fields
and not the list. §8 produced the setup-output finding; the collapse path flows through the one
debounced `putLayout` whose failure already reaches `pushError` (`CardGrid.tsx:84`), leaving panes
collapsed on screen as FS-02.A39 requires. §9 holds — TS-12.R7 verifies the canonical path, rejects
symlinks, and confirms the checkout is still registered to the recorded repository before any
`git worktree remove --force`. §10 produced the embedded-`index.html`, FS-02.R47 and FS-14 findings.
§11: TS-12 §3 adds a per-project `worktree: {owned, branch}` object to `GET /api/projects` that is
absent for ordinary projects — the null shape and the UI's `?? {}` are the implementation's to get
right, and the class binds them. §12 holds and is the design's strongest section: TS-12.R1 pins
argv-only invocation, plumbing commands over porcelain scraping, bounded timeouts, and
`GIT_TERMINAL_PROMPT=0`. §13 holds: `agent-card-header-actions` and
`.context-bar[data-variant="compact"]` are both defined, and stylelint, the presentation-contract
audit, and the stylesheet-reading `CardGrid` case all pass. §14 holds — the new routes inherit
`localOnly` with the whole mux and TS-12.R8 keeps the owned root slug-validated, 0700, and
symlink-rejecting; noted for implementation, not as a finding: FS-19.R7 makes `setup_command` a
project config field that executes `/bin/sh -c` on a *launch*, so the existing project-CRUD
authorization is what stands behind it. §15 produced the fork-ordering finding; TS-12.R7's
remove-then-delete-row ordering is correct.

Checks run on the delivered tree: `make check-specs` (`spec check: ok`), `go build ./...`,
`go test ./...`, `go test -tags sqlite_fts5 ./...`, `npm test` (346 cases, 51 files),
`npm run check:styles` including the presentation-contract audit, `npm run build`, and
`git diff --check` all pass. `ui/dist/index.html` rebuilt from that tree was **not** byte-identical
to the tracked `internal/server/ui/dist/index.html`; the 2026-09-02 worktree work committed the
regenerated file, so that finding is closed and is not listed above.

Browser-only evidence stays recorded as acceptance gates above, not as findings. The card change's
own real-browser pass at 1024px and 1440px in Core and Sky & Grove is recorded in its changelog
entry and is taken as the evidence for FS-02.A37/A38/A40/A41's browser halves.

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
- [x] **Closed 2026-08-27.** A real Chromium confirmed a right-click anywhere on the projects
  canvas opens **New project** (FS-02.A24): eight background points including the padding frame on
  every edge and corner, while a card right-click still opens the card menu, and the menu opens a
  styled create modal. Evidence in the J16 section of
  [`../reviews/usability-review-run-2026-08-27-release-delta.md`](../reviews/usability-review-run-2026-08-27-release-delta.md).
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
- [x] **Closed 2026-08-30.** A real Chromium run covered J5's running-first placement, live
  running/stopped boundary crossings in both directions, in-drag geometry within one block, refused
  cross-block drop, and the expanded pane's two-column drag footprint (FS-02.A28, FS-02.R47).
  Evidence is in the J5 section of
  [`../reviews/usability-review-run-2026-08-30-new-pages.md`](../reviews/usability-review-run-2026-08-30-new-pages.md).

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

The five findings this review recorded against the FS-19/TS-12 worktree design are closed by the
2026-09-02 implementation rather than left open here: three became in-place TS-12 corrections —
R3's ownership-row ordering, R4's misapplied `ValidateStage` seam and its `status.detail` promise,
and R9's base fallback — and two became implementation, the per-project recreation claim and the
display bound on setup output. Each correction is recorded in the requirement it changes.

The user resolved the `agentdeck-shared-skill` design review: verified installation now
precedes exact AgentDecker migration and the thin prompt no longer claims an unavailable skill.
Runtime-only overlay fields and fresh PM/teammate prompt cleanup remain included implementation
alignment, not review findings. Browser-only evidence is recorded as acceptance gates above, not as
findings.

## Design consistency notes

- The change file cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 both cite `TS-04.R32–R39` and
  omit R40, the direct-action redaction clause. One range is wrong; the three should agree.
- FS-17 §6 opens with "The contract is shipped. Live-provider compatibility remains tracked as
  acceptance gate A6," which reads as covering the whole section, but §6 now also carries the planned
  direct-cutover boundary for R13–R19. Scope the opening sentence to R1–R12.
