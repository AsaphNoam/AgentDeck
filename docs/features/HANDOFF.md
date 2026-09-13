# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in `../archive/state/`: the dated
[`HANDOFF-through-2026-09-12`](../archive/state/HANDOFF-through-2026-09-12.md),
[`-11`](../archive/state/HANDOFF-through-2026-09-11.md),
[`-10`](../archive/state/HANDOFF-through-2026-09-10.md),
[`-09`](../archive/state/HANDOFF-through-2026-09-09.md),
[`-07`](../archive/state/HANDOFF-through-2026-09-07.md),
[`-06`](../archive/state/HANDOFF-through-2026-09-06.md) and
[`-03`](../archive/state/HANDOFF-through-2026-09-03.md) files, plus
[`HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** None.
- **Release:** `v0.4.3` is tagged and published; **Release state** and the release record carry its
  contents. `v0.4.2` and earlier are in the state archive.
- **Review units:** none available. `persistent-pipeline-orchestration` was reviewed 2026-09-13 and
  closed the same day when its fixes landed; that fix commit is part of its closure and is not a new
  review unit. `stop-telling-agents-to-poll` shipped without entering this queue on
  the operator's explicit 2026-09-10 instruction; it can be added later.
- **Work units:** `rename-product-to-deckhand.md` is Waiting to start: the AgentDeck → Deckhand rename with its
  one-time state migration, role rename to FirstMate, and two named read-compatibility paths.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport
  blocker; the ACP wait-list in `docs/ideas.md` holds the rest behind an adapter contract.
  Queue hygiene: `bump-pinned-acp-adapters.md` reads `State: Finished` but is still in
  `docs/ready-changes/` and absent from that directory's index; per its README a finished change's
  file is removed. Left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume; `New ideas` entries are available.
  Persistent pipeline orchestration and its mail extension are implemented, reviewed and fixed.
  TS-01.R31–R33, TS-02.R34 and TS-04.R53 complete shared prompt preparation, bounded batches,
  transactional budget/read settlement, uncertain-delivery recovery and deferred retention.
  The design decisions for clean legacy reset, descendant cancellation, project boundaries,
  subordinate coordination and ordinary stop/resume remain confirmed; implementation gaps are
  recorded below. Streaming agent thinking stays part-decided (live-only decided; rendering default and whether
  `plan` ships still open). The permanently unaddressable pipeline agent is the newest `New ideas`
  entry and needs `/design-feature` before code. The Deckhand rename is fully specified and promoted
  to the work queue; no design decision remains open for it.
- **Open findings:** `persistent-pipeline-orchestration` is closed; all fourteen findings are fixed
  with regression tests. Still open: the
  injected-steer lifetime edge case, live-gate finding durability, provider-contract oracles, and the
  unverified OpenCode/OpenHands paths. See **Review findings**.
- **Bug reports:** BR-1, BR-2, and BR-3 are investigated and archived with this release; BR-3 is
  fixed and closed. BR-1's Codex model/effort defect is fixed and reviewed; BR-2 is fixed and closed.
  BR-1's still-open findings are listed above. Pinned Claude model delivery
  through `_meta` works; an ACP model `currentValue` can be stale and is no execution-model oracle.
- **State:** The file viewer's credentialed rendered forms are owed: journey J3 now carries the
  file-link steps (docked and transcript-width forms, the refusal branch, the dashboard-pane
  navigation), and none of them has been exercised against a real browser.
  Automated MCP contract verification is green. The full post-fix Claude/Codex acceptance
  matrix remains open and must not be called verified; the 2026-09-08/09 probes were limited contract
  and model-delivery checks, not that matrix. Real Claude and Codex steering is unexercised.
- **Branch:** `main`.

## Active change

**Change:** None.

**Changelog — 2026-09-13 (workflow):** Clarified verified slice commits versus final closure,
required checkpoint/resumption notes and stable delegation ownership, and added an actionable-work
check before ending a turn. This administrative update leaves the product work and role queues intact.
Diff checks pass; `make check-specs` reports 14 existing finding-label errors, also present in `HEAD`.
Skill frontmatter is unchanged; its validator could not run because the available Python lacks PyYAML.

**State:** `persistent-pipeline-orchestration` is closed; its fourteen findings were fixed on
2026-09-13, each with a regression test that fails without the fix.

**Changelog — 2026-09-13 (fix: persistent-pipeline-orchestration):** Cleanup gained one stage/run
convergence contract that pages and cancels scoped members, holds the cursor until every
release/yield settles, re-drives the run after a settled effect, and offers repair on `finishing`
too (INV §5/§15/§16). Inherited closure became one SQL fence applied by admission, retry, re-arm and
both wait paths; stage acceptance now fences on run state rather than a caller-read revision, so a
report after Stop cannot un-stop a run, and a failed startup reconcile keeps the stop (INV
§2/§5/§15). Stage reports require a matching execution handle and reject a standing yield (INV
§5/§11). Managed-work authority derives from the live standing-stage binding, not creation
provenance, and assignments now carry the managed child and prior accepted results (INV §2/§10).
Report sharing resolves the task-backed result directly (INV §10). The run projection separates
stage succession from delegated work, surfaces retained cleanup, and is bounded and batched (INV
§7/§8/§11/§16). Deferred-only mail no longer starts a turn (INV §5/§15). Also removed the dead v1
report/lifecycle engine and its skipped tests, shared one task-row insert, refcounted the per-run
lock, added one ordered run-cursor accessor, and moved the report branch to the control plane that
owns runs so it publishes the run update it commits (INV §1/§2/§16/§17). `make test`, `make build`,
the UI suite and spec checks pass; credentialed provider and browser gates remain unverified.

**Release state:** `v0.4.3` is published and verified on tag `8ad5261`. Release and CI runs passed,
the local distributable reports `0.4.3` with `sqlite_fts5`, and the GitHub Release carries the
darwin/arm64 archive, `install.sh`, and a manifest declaring `0.4.3` with its SHA-256.
The release shipped with five open Must-fix findings on the operator's explicit decision; all five
are now closed. The credentialed Claude and Codex journeys under
**Acceptance gates** are owed; real steering has never been exercised against a provider.

**Available by role:** `/review` has no unreviewed unit. `/work` may take
`rename-product-to-deckhand`; `/fix` may take BR-1;
`/design-feature` may choose an available or resumable idea. Queues are independent.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Acceptance gates

**Not blocking as of 2026-09-05.** These gates have not been run, and no agent may describe them as
verified, passed, or closed. The operator chose to let roles proceed with them open.

- [ ] Pinned real-provider stage-result/file-edit approval journey (FS-03.A26/J14).
- [ ] Post-fix credentialed Claude and Codex chat, MCP, resume, task, effective-model/effort, and
      reported-result checks. The historical 2026-07-26 run failed model precedence and is evidence,
      not closure for the current implementation.
- [ ] Pinned Claude terminal flags/hooks and live xterm journeys.
- [ ] Pinned OpenCode/OpenHands launch and credential checks.
- [ ] Real macOS native folder-panel checks (FS-04.A22/J2/J9/J16).
- [ ] Real-browser permission-pane and drag-refusal journeys (FS-02.A35/A43).
- [ ] Phase 7 federation matrix against real Claude and Codex installations.
- [ ] Real-browser worktree creation/launch and archive-with-uncommitted-work journeys (FS-19).
- [ ] Six-tab same-origin dashboard check against a `make dist` build (FS-02.A27).

## Blocked on human

- None.

## Review findings

`persistent-pipeline-orchestration` closed on 2026-09-13: all fourteen findings are fixed with
regression tests, and the unit is no longer open for review or fixes.

### BR-1 — **Fix model:** difficult — Codex Sol.

- **Must fix** — BR-1 finding state was not durable across concurrent roles (**confirmed**).
  **Where:** `docs/archive/reviews/live-provider-acceptance-2026-07-26.md` recorded the exact Codex
  model failure as a Must-fix on July 26, but the live acceptance session could not edit HANDOFF
  while another session owned the shared docs. The report and a contradictory effort design were
  then swept into `7d294fb` without the finding entering HANDOFF. **Why it matters:** the mandatory
  read order made the archived report invisible to every later role, so an already-detected critical
  defect was treated as an open gate for another six weeks. **Requirement:** workflow §§1.1/12,
  `INV §1`, `INV §10`. **Suggested fix/test:** a failed live gate must be recorded in HANDOFF before
  its role can close; if state-file ownership blocks that write, leave the role explicitly blocked
  rather than archiving the only finding. A commit/review that contains an acceptance report must
  reconcile every Must-fix in it with live state.
- **Must fix** — provider-contract claims can still be proved by a self-authored oracle
  (**confirmed**). **Where:** TS-04.R18 asserted a `model[effort]` request shape after inspecting
  `codex-acp`'s internal `ModelId` parser without tracing `session/new` to `threadStart`; FS-09.A15
  and `fakeacp` then checked only that AgentDeck emitted the asserted field. The fake accepts unknown
  request members that the pinned ACP decoder drops. The same oracle error recurred in the 2026-09-08
  finding-fix: it called ACP `configOptions.model.currentValue` an independent effective-model oracle
  and declared Claude `_meta` delivery broken without sending a prompt. A real prompt then showed
  stale `currentValue = opus` alongside requested Haiku/Sonnet in SDK init, assistant, and usage
  signals. **Why it matters:** design, implementation, review, and an adapter-local readback can all
  agree and remain wrong about the external provider; following the false Claude finding would add a
  redundant delivery path and change launch failure/order behavior. **Requirement:** `INV §11`,
  `INV §12`, `INV §17`. **Suggested fix/test:** require provider-behavior statements to cite a
  complete reachability trace or a credentialed prompt receipt; label ACP `currentValue` as adapter
  configuration evidence, not provider execution evidence; and add an independently derived contract
  oracle that rejects out-of-schema standard fields instead of mirroring `sessionNewParams`.
- **Worth fixing** — equivalent OpenCode/OpenHands fields remain unverified (**undetermined**).
  **Where:** neither CLI is installed. OpenHands model delivery has a separate `LLM_MODEL` env path,
  so it does not depend on the suspect ACP `model` member, but both adapters still receive an
  out-of-schema top-level `systemPrompt`; OpenCode also still depends on the top-level `model`.
  **Why it matters:** the same silent-ignore class may be live on surfaces explicitly advertised by
  AgentDeck. **Requirement:** FS-09.A6, `INV §12`. **Suggested fix/test:** keep the claims gated until
  each pinned CLI is installed and its model/prompt delivery is checked at the effective provider;
  remove any redundant unsupported top-level fields once their real mechanism is known.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
