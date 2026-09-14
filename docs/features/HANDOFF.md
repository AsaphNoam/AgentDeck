# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in `../archive/state/`: the dated
[`HANDOFF-through-2026-09-13`](../archive/state/HANDOFF-through-2026-09-13.md),
[`-12`](../archive/state/HANDOFF-through-2026-09-12.md),
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
- **Release:** `v0.5.0` is tagged and published; **Release state** carries its contents. `v0.4.3` and
  earlier are in the state archive.
- **Review units:** none available. `persistent-pipeline-orchestration` was reviewed 2026-09-13 and
  closed the same day when its fixes landed; BR-1's three findings were fixed the same day. Both
  sets of fix commits are closure of their originating units and are not new review units.
  `stop-telling-agents-to-poll` shipped without entering this queue on
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
- **Open findings:** none. `persistent-pipeline-orchestration` and BR-1 are both closed as of
  2026-09-13. The injected-steer lifetime edge case is still named in prose but was never recorded
  as a finding; it needs `/investigate-bug` before `/fix` can take it.
- **Bug reports:** BR-1, BR-2, and BR-3 are investigated, fixed and closed. Pinned Claude model
  delivery through `_meta` works; an ACP model `currentValue` is adapter configuration evidence and
  no execution-model oracle (TS-04.R54).
- **State:** Automated MCP contract verification is green.
- **Branch:** `main`.

## Active change

**Change:** None. `v0.5.0` closed the epoch: every settled 2026-09-13 changelog entry, the `v0.4.3`
release record, and the retired acceptance-gate checklist moved to
[`HANDOFF-through-2026-09-13`](../archive/state/HANDOFF-through-2026-09-13.md).

**Changelog — 2026-09-14 (CI flake):** Fixed the intermittent `LaunchStep` onboarding test that
reddened CI on `7162f59`. `LaunchStep` disables Launch until both the roles and the projects query
resolve, but both tests awaited only the project option before clicking; when the roles response
landed second, the click hit a disabled button, no `POST /api/sessions` was sent, and the
`launchBody` wait timed out. A shared `clickLaunch()` helper now waits for the button to be enabled.
Delaying the roles handler by 300ms reproduced the CI failure verbatim and both tests pass under
that delay with the fix. Test-only: no product code changed, so the shipped `v0.5.0` artifact is
unaffected and no re-release is required. CI also warns that `actions/checkout@v4`,
`setup-go@v5` and `setup-node@v4` are being forced off deprecated Node 20; not yet breaking, not
addressed here.

**Changelog — 2026-09-14 (release: `v0.5.0`):** Refreshed the shipped `operating-agentdeck` package
for the range's agent-facing changes (FS-18.R4–R5, TS-11.R1/R8).
`references/build-and-run-pipelines.md` was rewritten onto the standing-orchestrator model: stages
are bounded durable assignments to one standing run orchestrator that alone reports each stage
outcome, run setup picks one backend/model for it while only dedicated stages select a
sub-orchestrator runtime, templates carry no conditional routing, accepted stage completion cancels
unfinished descendants before cleanup fences the next stage, Stop cancels nested delegated work, and
Continue/Retry/Replace are distinguished (FS-14.R61–R78). `references/coordinate-work.md` gained
durable waiting — `wait_for_tasks` holds an assignment open, yields its capacity slot, and resumes on
a watched revision change instead of polling — plus creator authority to inspect, retry, re-arm,
cancel, and replace work it created (FS-16.R30–R38). `SKILL.md` names both in its routing bullets.
No product code changed. README, `install.sh`, and `scripts/release/assemble.sh` were re-checked
against the range and none of their release-matched claims is falsified: the CLI install/update/auth
surface, the config schema version, and the Node and adapter pins are unchanged, and the in-range
`assemble.sh` edit already carries its own `+agentdeck.1` component suffix.

**Release state:** `v0.5.0` is published and verified on tag `8ab84d3`. `make test` (both tag
variants, including `make check-specs`), the UI suite (54 files, 437 tests), and
`make dist VERSION=0.5.0` pass; the local distributable reports `0.5.0` with `sqlite_fts5`. The CI
and Release macOS installer runs both succeeded, and the GitHub Release carries the darwin/arm64
archive, `install.sh`, and a manifest declaring `0.5.0` whose SHA-256 and size match the uploaded
archive. No credentialed or real-browser journey was run for this release, and none may be described
as verified. The standing acceptance-gate checklist
was retired from this file on the operator's explicit decision during this release; the underlying
verification debt is unchanged and is recorded in
[`HANDOFF-through-2026-09-13`](../archive/state/HANDOFF-through-2026-09-13.md).

**Available by role:** `/review` has no unreviewed unit. `/work` may take
`rename-product-to-deckhand`; `/fix` has no open findings;
`/design-feature` may choose an available or resumable idea. Queues are independent.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Blocked on human

- None.

## Review findings

`persistent-pipeline-orchestration` closed on 2026-09-13: all fourteen findings are fixed with
regression tests, and the unit is no longer open for review or fixes. BR-1 closed on 2026-09-13:
all three findings are fixed, and the OpenCode/OpenHands delivery it left undetermined is now
documented as compatibility evidence rather than an open finding.

No review or bug-report unit has open findings.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
