# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in `../archive/state/`: the dated
[`HANDOFF-through-2026-09-27`](../archive/state/HANDOFF-through-2026-09-27.md),
[`-25`](../archive/state/HANDOFF-through-2026-09-25.md),
[`-14`](../archive/state/HANDOFF-through-2026-09-14.md),
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
- **Release:** `v0.6.0` is tagged; **Release state** carries its contents. `v0.5.0` and earlier
  are in the state archive, as are the units, findings and bug reports it closed.
- **Review units:** `add-studio-skin` (finished 2026-09-23) and `complete-studio-composition`
  (finished 2026-09-25) await `/review`; the operator shipped them unreviewed in `v0.6.0`. Review
  `add-studio-skin` against `e474d8a..1d78e1d` and `complete-studio-composition` against
  `9ae10ee..4bb5b2e`; `71c2810` is the latter's evidence-only handoff follow-up, not another unit.
  FS-12.A19–A23 and TS-08.R68 remain planned despite the shipped requirements.
  `stop-telling-agents-to-poll` shipped outside this queue on the operator's explicit
  2026-09-10 instruction; it can be added later.
- **Work units:** `rename-product-to-deckhand.md` and `add-mobile-remote-control.md` are Waiting to
  start. `migrate-internal-actions-from-mcp.md` stays
  paused on its transport blocker. Queue hygiene: `bump-pinned-acp-adapters.md` reads
  `State: Finished` but is still in `docs/ready-changes/`; left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume (the operator deleted the
  uncommitted Cursor backend draft on 2026-09-23); `New ideas`
  entries are available; the permanently unaddressable pipeline agent needs `/design-feature`.
- **Open findings:** None recorded. The injected-steer lifetime edge case is still named in prose
  but was never recorded as a finding; it needs `/investigate-bug` before `/fix` can take it.
  FilesTab and CommandsTab still copy silently via bare `writeText`.
- **Bug reports:** None open.
- **State:** Automated MCP contract verification is green.
- **Branch:** `main`.

## Active change

**Change:** None.

**Owed from archived entries** ([`HANDOFF-through-2026-09-25`](../archive/state/HANDOFF-through-2026-09-25.md)):
A46's real-browser J14 pass; the credentialed Codex 1.12.0 receipt (TS-06.R26) gating
FS-03.A41/A42 and FS-01.A20; Sky & Grove unviewed for Codex capabilities. Pre-existing, not Studio:
Sky & Grove tints the whole user event row; `--ad-shadow-project-edge` resolves at `:root`.

**Acceptance remains planned, not shipped:** FS-12.A19–A23 and TS-08.R68 still need the complete
route/state comparison. In this pass, the available Chrome browser adapter reported 1280×1125 after
its 1024×900 viewport request, so the populated built-app review does not establish true-1024
behavior; the in-app browser was unavailable to that run. A faithful desaturated cross-skin
comparison beyond the dashboard was also unavailable. fakeACP produced active and paused pipeline
states but not a genuine finished-run timeline. No product code or specifications changed in this
evidence pass.

**Release state:** `v0.6.0` is tagged on the release commit (range `v0.5.0..main`, 49 commits).
`make test` (both tag variants, including `make check-specs`), the UI suite (58 files, 481 tests),
and `make dist VERSION=0.6.0` pass; the local distributable reports `0.6.0` with `sqlite_fts5`. The
range changed nothing an operating agent must know, so `operating-agentdeck` is unchanged; README,
`install.sh` and `scripts/release/assemble.sh` pins still match. Two Studio review units shipped
unreviewed on the operator's explicit decision. Owed: the credentialed Claude and Codex
login/chat gates (TS-06.R21) and every real-browser journey; none may be described as verified.

**Available by role:** `/review` may take `add-studio-skin` or `complete-studio-composition`.
`/work` may take `rename-product-to-deckhand` or `add-mobile-remote-control`;
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

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
