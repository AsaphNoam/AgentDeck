# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in `../archive/state/`: the dated
[`HANDOFF-through-2026-09-14`](../archive/state/HANDOFF-through-2026-09-14.md),
[`-13`](../archive/state/HANDOFF-through-2026-09-13.md),
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
- **Review units:** `adopt-modern-codex-acp-capabilities` (finished 2026-09-23) is available. `persistent-pipeline-orchestration` was reviewed 2026-09-13 and
  closed the same day when its fixes landed; BR-1's three findings were fixed the same day. Both
  sets of fix commits are closure of their originating units and are not new review units.
  `stop-telling-agents-to-poll` shipped without entering this queue on
  the operator's explicit 2026-09-10 instruction; it can be added later.
- **Work units:** `rename-product-to-deckhand.md` is Waiting to start. `migrate-internal-actions-from-mcp.md` stays
  paused on its transport blocker. Queue hygiene: `bump-pinned-acp-adapters.md` reads
  `State: Finished` but is still in `docs/ready-changes/`; left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume (the Cursor backend draft has
  uncommitted spec edits in the tree from another session — not this change's); `New ideas`
  entries are available; the permanently unaddressable pipeline agent needs `/design-feature`.
- **Open findings:** none. `persistent-pipeline-orchestration`, BR-1, and BR-4 are all closed.
  The injected-steer lifetime edge case is still named in prose but was never recorded
  as a finding; it needs `/investigate-bug` before `/fix` can take it.
- **Bug reports:** BR-1, BR-2, BR-3, and BR-4 are investigated, fixed and closed. Pinned Claude model
  delivery through `_meta` works; an ACP model `currentValue` is adapter configuration evidence and
  no execution-model oracle (TS-04.R54). BR-4 (2026-09-22, "the main project page looks off, the
  cards are stretched and stuck to the bottom") is fixed the same day; see **Review findings**.
- **State:** Automated MCP contract verification is green.
- **Branch:** `main`.

## Active change

**Change:** None. `adopt-modern-codex-acp-capabilities` finished 2026-09-23 and awaits `/review`.

**Changelog — 2026-09-23 (work: modern Codex ACP capabilities):** Eight slices shipped: Codex ACP
1.12.0/CLI 0.154.0 with the rebased steering patch; bilateral capability negotiation frozen on the
session; canonical tool names; live-only reasoning; nested native child sessions; background tasks
with targeted Stop; Clone as a native `session/fork`; file-change reports as tracking supplements.
Closure matrix passed (`make test` both variants, focused `-race`, `make build`, UI 453 tests + build).
A fake-ACP real-browser pass (Core, desktop and the one-column dashboard pane) confirmed Thinking,
child nesting, the task list with Stop, and Clone's copied history and marker; it led to keeping the
task list open after Stop. **Still owed before release:** the credentialed Codex 1.12.0 receipt
(TS-06.R26, stays `(planned)`), which also gates FS-03.A41/A42 and FS-01.A20 (J7); Sky & Grove was
not viewed. The in-app browser pane cannot run the shared-worker SSE stream (its worker reports an
error and the app never falls back to a direct stream) — not reproduced elsewhere, not recorded as a
finding. Review notes (reversible choices): MCP calls carry no `name`, so auto-approve identity
stays title-based; unknown child session ids drop only once subagents are negotiated; a task still
running at a later resume or clone boundary shows as ended there without Stop, because resume/fork
replay is dropped; the clone process forks the source thread because all Codex agents share one
AgentDeck Codex profile, and a source turn begun after the busy check is not excluded from the fork.
Settled 2026-09-14 entries are in
[`HANDOFF-through-2026-09-14`](../archive/state/HANDOFF-through-2026-09-14.md).

**Changelog — 2026-09-22 (design: modern Codex ACP capabilities):** Re-evaluated the ACP wishlist
against `codex-acp` 1.12.0 and promoted `adopt-modern-codex-acp-capabilities.md` to Waiting to start.
FS-01.R36, FS-03.R57–R61, FS-05.R38 and their acceptance items make Clone a native conversation
fork, stream reasoning live-only, nest negotiated child sessions, expose background-task lifecycle
and targeted stop, and consume canonical tool/file metadata without merging them into AgentDeck's
durable task plane. TS-01.R35, TS-02.R35, TS-03.R43–R44, TS-04.R61–R66, TS-06.R26 and TS-08.R59
keep the work inside normalized Runtime, pin the 1.12.0/0.154.0 pair, and retain the steering patch
because upstream still lacks its idle no-consumption behavior. Plans, provider recommendations and
session goals are excluded. No product code changed.

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

**Available by role:** `/review` may take `adopt-modern-codex-acp-capabilities`. `/work` may take
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

BR-4 closed on 2026-09-22: its one Must-fix finding (`.project-dashboard`/`.project-card-grid`
missing the `align-content`/`align-items: start` that `.card-grid` already carried, `INV §2`,
`ui/src/styles/features/dashboard.css`) is fixed with the committed regression test un-skipped, and
the unit is no longer open for review or fixes.

No review or bug-report unit has open findings.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
