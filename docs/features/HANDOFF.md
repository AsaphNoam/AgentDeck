# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in
[`../archive/state/HANDOFF-through-2026-09-11.md`](../archive/state/HANDOFF-through-2026-09-11.md),
[`../archive/state/HANDOFF-through-2026-09-10.md`](../archive/state/HANDOFF-through-2026-09-10.md),
[`../archive/state/HANDOFF-through-2026-09-09.md`](../archive/state/HANDOFF-through-2026-09-09.md),
[`../archive/state/HANDOFF-through-2026-09-07.md`](../archive/state/HANDOFF-through-2026-09-07.md),
[`../archive/state/HANDOFF-through-2026-09-06.md`](../archive/state/HANDOFF-through-2026-09-06.md),
[`../archive/state/HANDOFF-through-2026-09-03.md`](../archive/state/HANDOFF-through-2026-09-03.md),
and [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** None; the next role picks from the queues below.
- **Release:** `v0.4.3` is tagged and published; **Release state** and the release record carry its
  contents. `v0.4.2` and earlier are in the state archive.
- **Review units:** `file-read-nonregular-kind` awaits independent review.
  `fix-model-recommendations` and `open-a-file-from-chat` are closed. Every other unit through this
  release is closed,
  including `queue-a-follow-up-while-busy`,
  its `steering-host-owned-fallback` continuation, and `usability-20260907`.
  `stop-telling-agents-to-poll` shipped without entering
  this queue on the operator's explicit 2026-09-10 instruction; it can be added later.
- **Work units:** `persistent-pipeline-orchestration.md` is Waiting to start: standing stage ownership,
  managed subordinate coordinators, shared task execution/waiting and automatic cleanup. Its exact
  requirements and acceptance gates are in `docs/ready-changes/persistent-pipeline-orchestration.md`.
  `rename-product-to-deckhand.md` is Waiting to start: the AgentDeck → Deckhand rename with its
  one-time state migration, role rename to FirstMate, and two named read-compatibility paths.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport
  blocker; the ACP wait-list in `docs/ideas.md` holds the rest behind an adapter contract.
  Queue hygiene: `bump-pinned-acp-adapters.md` reads `State: Finished` but is still in
  `docs/ready-changes/` and absent from that directory's index; per its README a finished change's
  file is removed. Left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume; `New ideas` entries are available.
  Persistent pipeline orchestration is fully specified and promoted to the work queue; no design
  decision remains open for it. The clean legacy reset, descendant cancellation, project boundaries,
  subordinate stage coordination and ordinary stop/resume are confirmed.
  Streaming agent thinking stays part-decided (live-only decided; rendering default and whether
  `plan` ships still open). The permanently unaddressable pipeline agent is the newest `New ideas`
  entry and needs `/design-feature` before code. The Deckhand rename is fully specified and promoted
  to the work queue; no design decision remains open for it.
- **Open findings:** The separate injected-steer lifetime edge case remains outside the closed
  host-owned fallback unit. Also open: live-gate finding durability, provider-contract oracles, and
  the unverified OpenCode/OpenHands paths. See **Review findings**.
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

**Changelog — 2026-09-12 (design-feature):** Designed the **Deckhand** rename: one cut through every
identity, `~/.agentdeck` migrated by `os.Rename` at first start, `agentdecker` → **FirstMate**, clean
CLI swap, dual-read annotation prefix, browser-key copy-forward. Existing installs move over by
running the installer once, not `agentdeck update` — GitHub documents rename redirects only for web
links and git clone/fetch/push. MCP tool names were never branded and do not change. Added
FS-00.R16, FS-04.R48/A28, FS-10.R15–R19/A7–A9, FS-13.R24/A15, FS-18.R14/A10, TS-02.R32–R33,
TS-04.R52, TS-06.R24, TS-08.R58, TS-11.R14; seven specs moved Current → Partial with the index.
Promoted to `docs/ready-changes/rename-product-to-deckhand.md`. Checks pass; no product code changed.

**Changelog — 2026-09-12 (design-feature):** Revised persistent orchestration so the standing
agent always owns and reports the stage; configured dedicated coordinators are managed children and
the normal coordination contact, with durable awareness of material direct intervention. Cleanup
now automatically reconciles transient failures with persisted backoff and attention only for
persistent/unsafe conditions. Confirmed same-identity stop/resume and assignment re-delivery. Updated
FS-14.R61–R77/A35–A44, FS-16.R30–R38/A20–A24, TS-09.R35–R50, TS-10.R25–R37 and TS-05.R22;
R60 is superseded by R75. Promoted the source idea to the waiting ready change. Spec checks, twin
skills and diff checks pass; no product code changed and no implementation is active.

**Changelog — 2026-09-12 (review):** Reviewed `fix-model-recommendations` across its workflow,
mirrored role launchers, validation scripts, mutation tests, and build wiring. The change consistently
records one recommendation per grouped fix unit at the level of its most difficult open fix, and the
tests independently reject missing, duplicate, per-item, mismatched, or weakened routing. No findings;
the unit is closed. Focused finding-contract, launcher-contract, mutation, handoff-spec, and diff checks
pass. Invariant classes 2, 4, 7, 10, and 17 apply; classes 1, 3, 5, 6, 8, 9, and 11–16 have no applicable
surface.

**Changelog — 2026-09-12 (fix):** Hardened the New Agent modal tests to wait for the Launch
button to become enabled before clicking it, closing a CI timing race around asynchronously loaded
role and project state. All 450 UI tests pass; product behavior is unchanged.

**Release state:** `v0.4.3` is published and verified on tag `8ad5261`. Release and CI runs passed,
the local distributable reports `0.4.3` with `sqlite_fts5`, and the GitHub Release carries the
darwin/arm64 archive, `install.sh`, and a manifest declaring `0.4.3` with its SHA-256.
The release shipped with five open Must-fix findings on the operator's explicit decision; all five
are now closed. The credentialed Claude and Codex journeys under
**Acceptance gates** are owed; real steering has never been exercised against a provider.

**Available by role:** `/review` may take `file-read-nonregular-kind`; `/work` may take `persistent-pipeline-orchestration`;
`/fix` may take BR-1 (difficult, Sol);
`/design-feature` may choose an available or
resumable idea. Queues are independent.

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
