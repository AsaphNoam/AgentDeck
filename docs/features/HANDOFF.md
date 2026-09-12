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
- **Review units:** `fix-model-recommendations` and `file-read-nonregular-kind` await independent
  review. `open-a-file-from-chat` is fixed and closed. Every other unit through this release is closed,
  including `queue-a-follow-up-while-busy`,
  its `steering-host-owned-fallback` continuation, and `usability-20260907`.
  `stop-telling-agents-to-poll` shipped without entering
  this queue on the operator's explicit 2026-09-10 instruction; it can be added later.
- **Work units:** None waiting to start. `migrate-internal-actions-from-mcp.md` stays paused on its transport
  blocker; the ACP wait-list in `docs/ideas.md` holds the rest behind an adapter contract.
  Queue hygiene: `bump-pinned-acp-adapters.md` reads `State: Finished` but is still in
  `docs/ready-changes/` and absent from that directory's index; per its README a finished change's
  file is removed. Left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume; `New ideas` entries are available.
  Persistent pipeline orchestration has feature and technical drafts in FS-14.R60–R74/A35–A42,
  FS-16.R30–R36/A20–A22, TS-09.R35–R48, TS-10.R25–R34 and TS-05.R22. The operator approved a
  clean legacy reset, cancellation of all run descendants, and existing project boundaries.
  One technical choice remains in FS-14 §6: same identity/conversation with task stop/resume
  (recommended), versus a continuously live process. Draft assumes the former; no ready work unit.
  Streaming agent thinking stays part-decided (live-only decided; rendering default and whether
  `plan` ships still open). The permanently unaddressable pipeline agent is the newest `New ideas`
  entry and needs `/design-feature` before code.
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

**Changelog — 2026-09-12 (fix):** Hardened the New Agent modal tests to wait for the Launch
button to become enabled before clicking it, closing a CI timing race around asynchronously loaded
role and project state. All 450 UI tests pass; product behavior is unchanged.

**Changelog — 2026-09-11 (design-feature):** Designed pipeline progression over the shared task
dispatcher and result transaction, dynamic lineage, scoped inspection/repair, durable child waiting,
run-wide cancellation fences, same-agent recovery, and the authorized legacy reset. Reconciled
context, lifecycle, protocol, persistence and security contracts. A focused design check caught and
resolved dedicated-agent identity promotion, borrowed-turn cancellation races, and wake/release
ownership edges. Spec checks, twin skills and diff checks pass. No code changed; the sole pending
runtime-lifetime choice above prevents ready-change promotion.

**Changelog — 2026-09-11 (design-feature):** Drafted persistent pipeline orchestration as durable
stage assignments, with dynamic child work and explicit stage outcomes. Added feature acceptance
for continuity, dedicated-stage exceptions, review/fix loops, and recovery. Recorded the need for
agent work inspection/repair and child-result delivery during an active assignment. Awaiting product
decisions in FS-14 §6 before technical design; no product code or ready change. The pre-existing
pipeline idea was preserved and extended rather than duplicated.

**Changelog — 2026-09-11 (fix):** Fixed the CI failure in the chat file viewer's kind check
(FS-03.A37, TS-05.R21; `INV §14`, `INV §17`). The read classified its target by opening it, so a
non-regular file's verdict followed the platform: Linux refuses the open of a Unix socket and was
told the path was outside the workspace, while macOS accepted it and reported `not_a_file`; a FIFO
would have blocked the handler until a writer appeared. The read now opens the root once and
classifies the target through that handle before opening it, so non-regular files refuse as
`not_a_file` everywhere. Containment is unchanged: the root handle still refuses symlinks that leave
the workspace, and the post-open check on the descriptor still guards replacement. The full Go
matrix, tagged build, focused file-read race tests, a Linux-target vet, and spec checks pass; the
Linux behavior itself is covered by the CI rerun, not locally.

**Changelog — 2026-09-11 (fix):** Closed BR-2's remaining Codex discovery-versus-execution
authority finding (FS-09.R59/A29, TS-06.R22; `INV §8`, `INV §10`, `INV §12`). The packaged wrapper
now reports the exact private Codex version it selected; model autosync compares that authority with
the cache's existing `client_version` and skips a mismatched personal cache instead of importing
models the packaged CLI may not understand. New Agent shows the effective packaged path/version and
an actionable mismatch before launch, while a backend/model executable override is identified as
unverified. Matching caches, source launches, and explicit process overrides retain their prior
behavior. The full Go matrix, tagged build, all 450 UI tests, UI production build, spec checks, and
diff check pass. The BR-2 unit is closed; BR-1's three findings remain open.

**Changelog — 2026-09-11 (fix):** Closed **keep steering inside AgentDeck's turn lifecycle**
(FS-03.R56/A38, TS-01.R30, TS-03.R41, TS-04.R51, TS-06.R14; `INV §5`, `INV §10`, `INV §12`,
`INV §15`, `INV §17`). Steer now reserves its possible host-owned successor before the adapter call;
turn settlement transfers the gate to that reservation without firing it, and the adapter result
atomically commits `promptRequired` or unwinds injected, legacy, and refusal outcomes. The wire-level
regression holds the steering reply after the old prompt settles, accepts a concurrent Send, and
proves provider order remains old prompt, correction, then held Send. Release assembly now checks
exact pre- and post-patch SHA-256 fingerprints and disables patch fuzz, and shipped traceability no
longer says planned. The originating `queue-a-follow-up-while-busy` unit and its fallback continuation
are closed. The full Go matrix, tagged build, focused race tests, spec lint, shell syntax, patch
applicability, and diff check pass.

**Changelog — 2026-09-11 (review):** Reviewed **keep steering inside AgentDeck's turn lifecycle**.
The host-owned fallback works in the covered lifecycle, but the reservation is created only after
the adapter returns; a Send accepted during that wait can therefore become the next turn first.
Release assembly also allows patch fuzz, and three technical-spec traceability entries still call
the shipped lifecycle planned. The unit remains open with one Must-fix and two Worth-fixing findings.
**Fix model:** medium — Codex Terra or Claude Opus. The full Go matrix, tagged build, focused race
tests, spec lint, patch applicability, shell syntax, and diff check pass. Invariant classes 1, 2, 4,
5, 8, 10, 11, 12, 15, 16, and 17 apply; classes 3, 6, 7, 9, 13, and 14 have no applicable surface.

**Release state:** `v0.4.3` is published and verified on tag `8ad5261`. Release and CI runs passed,
the local distributable reports `0.4.3` with `sqlite_fts5`, and the GitHub Release carries the
darwin/arm64 archive, `install.sh`, and a manifest declaring `0.4.3` with its SHA-256.
The release shipped with five open Must-fix findings on the operator's explicit decision; all five
are now closed. The credentialed Claude and Codex journeys under
**Acceptance gates** are owed; real steering has never been exercised against a provider.

**Available by role:** `/review` may take `fix-model-recommendations`; `/work` has no waiting unit;
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
