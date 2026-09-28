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

- **Active change:** `add-mobile-remote-control` — in progress (slice plan under Active change).
- **Release:** `v0.6.0` is tagged and published; **Release state** carries its contents. `v0.5.0` and earlier
  are in the state archive, as are the units, findings and bug reports it closed.
- **Review units:** `add-studio-skin` (finished 2026-09-23) and `complete-studio-composition`
  (finished 2026-09-25) await `/review`; the operator shipped them unreviewed in `v0.6.0`. Review
  `add-studio-skin` against `e474d8a..1d78e1d` and `complete-studio-composition` against
  `9ae10ee..4bb5b2e`; `71c2810` is the latter's evidence-only handoff follow-up, not another unit.
  FS-12.A19–A23 and TS-08.R68 remain planned despite the shipped requirements.
  `share-creative-workspace-layout` finished 2026-09-28 and awaits `/review`; its substantive
  commits are `0b31c9a`, `ddab692` and the closure commit named
  `Finish shared workspace acceptance and reduced-motion fallback` (base `f79fb97`).
  Evidence: `docs/archive/reviews/implementation-share-creative-workspace-2026-09-28.md`.
  `stop-telling-agents-to-poll` shipped outside this queue on the operator's explicit
  2026-09-10 instruction; it can be added later.
- **Work units:** `add-mobile-remote-control.md` is in progress (see Active change).
  `share-creative-workspace-layout.md` is Finished. `rename-product-to-deckhand.md` and
  `drop-pipeline-recipient-refusal.md` are Waiting to start. `migrate-internal-actions-from-mcp.md` stays
  paused on its transport blocker.
- **Design units:** `Ideas being defined` entries may resume (the operator deleted the
  uncommitted Cursor backend draft on 2026-09-23); `New ideas`
  entries are available. The unaddressable-pipeline-agent idea was already shipped by FS-14.R74
  (`8d8ca6e`); on 2026-09-28 its stale FS-01/03/06/14/16 and TS-04 text was reconciled and the
  leftover refusal became `drop-pipeline-recipient-refusal.md` (FS-06.R37/A26, TS-04.R67).
  `docs/ideas.md` was pruned 2026-09-28: shipped agent re-arm/retry/inspection, fixed chat-reload,
  pagination and ACP-readiness items, and nudge-era liveness items were removed; small related
  entries were merged.
- **Open findings:** BR-6's investigation unit (four findings), in
  **Review findings**. The injected-steer lifetime edge case is still named in prose
  but was never recorded as a finding; it needs `/investigate-bug` before `/fix` can take it.
  FilesTab and CommandsTab still copy silently via bare `writeText`.
- **Bug reports:** BR-6 investigated; findings await `/fix` (**Review findings**). BR-7 closed
  2026-09-28: live shared workers retain the shared reconnect path during server outages.
- **State:** Automated MCP contract verification is green.
- **Branch:** `main`.

## Active change

**Change:** `add-mobile-remote-control` — in progress (2026-09-28). Slice plan and progress live in
the change file's **Progress** section. Slices 1–7, 9 (server) and 10 committed; next: 8 (phone actions).

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

**Release state:** `v0.6.0` is published on tag `24ab07b` (range `v0.5.0..main`, 49 commits). The
CI and Release macOS installer runs both succeeded; the GitHub Release carries the darwin/arm64
archive, `install.sh`, and a manifest declaring `0.6.0` whose SHA-256 and size match the uploaded
archive.
`make test` (both tag variants, including `make check-specs`), the UI suite (58 files, 481 tests),
and `make dist VERSION=0.6.0` pass; the local distributable reports `0.6.0` with `sqlite_fts5`. The
range changed nothing an operating agent must know, so `operating-agentdeck` is unchanged; README,
`install.sh` and `scripts/release/assemble.sh` pins still match. Two Studio review units shipped
unreviewed on the operator's explicit decision. Owed: the credentialed Claude and Codex
login/chat gates (TS-06.R21) and every real-browser journey; none may be described as verified.

**Available by role:** `/review` may take `add-studio-skin`, `complete-studio-composition` or
`share-creative-workspace-layout`.
`/work` may take `rename-product-to-deckhand`, `add-mobile-remote-control`
or `drop-pipeline-recipient-refusal`;
`/design-feature` may choose an available or resumable idea. Queues are independent.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Blocked on human

None for shared creative-workspace implementation. Its approved system check passed after
correcting reduced-motion selector priority, and Reduce Motion was restored off.

## Review findings

### BR-6 investigation unit (2026-09-27) — **Fix model:** medium — Codex Terra or Claude Opus.

Report, verbatim: "links clicked on in the agents aren't opened (refused), even if it's in the
working directory". Version: `v0.6.0` era (the live dev instance runs `main`); macOS. No link, agent,
or log was supplied. Nothing was logged: the local instances' request logs contain no `/file`
request, and the dev instance logs to its terminal. Governing items: FS-03.R51–R55/A34–A37,
TS-03.R40, TS-05.R21. The server route was probed live against real agents: relative, `./`, and
absolute in-directory paths read correctly, so the failures are in which path reaches it.

- **Must fix** — confirmed (reproduced), spec gap plus defect; INV §11. Links inside a file shown in the
  viewer's **Rendered** form are sent verbatim and resolved against the agent's working directory,
  not the viewed file's directory. From `docs/features/HANDOFF.md`, `../archive/state/x.md` is
  refused as outside the working directory though the target is inside it, and a sibling
  `AGENT-WORKFLOW.md` reads as missing; this repository's own docs link this way throughout.
  `FileViewer.tsx:95` passes `onOpenFile` straight to `SanitizedMarkdown`. FS-03.R52 does not say
  what a rendered file's relative link is relative to; specify the file's own directory (ordinary
  Markdown semantics), resolve it client-side before `onOpenFile`, and let a result that climbs
  above the working directory still reach the server's refusal. The existing A35 test expects the
  current behavior (`other.go` from `docs/notes.md`) and must change. Regression: the skipped
  BR-6 test in `ui/src/components/chat/FileViewer.test.tsx`.
- **Worth fixing** — confirmed (reproduced); INV §11. A bare file name with a line suffix
  (a link whose target is `README.md:12`) matches `filePath.ts`'s scheme regex (`README.md:` reads as a
  scheme), so it is not a file link, the default URL transform strips it, and the click does
  nothing. FS-03.R51/A34. Fix: treat a target whose apparent scheme is followed only by a line
  suffix as a path. Regression: skipped test in `renderers/filePath.test.ts`.
- **Worth fixing** — confirmed (reproduced); INV §11. A `:start-end` line range, which Codex writes (18
  occurrences in this machine's recent Codex sessions, e.g. `dashboard.css:49-125`), stays part of
  the file name, so the viewer reports the file missing. FS-03.R51 names only `:line` and
  `:line:col`; extend it to a range cited by its start line. Regression: skipped test in
  `renderers/filePath.test.ts`.
- **Worth fixing** — probable (refusal reproduced live, trigger not observed); INV §14. On macOS's
  case-insensitive disk, an absolute link whose spelling differs from the recorded working
  directory only by case (`/users/…/projects/agentdeck/README.md` against
  `/Users/…/Projects/AgentDeck`) is refused as outside the directory by `rebaseAbsolute`'s lexical
  `filepath.Rel` (`internal/server/fileread.go:117`). An agent that takes its root from `git` or
  `realpath` while the directory was typed in another case produces this. Fix at that seam, e.g.
  by also accepting a base whose canonical on-disk spelling matches; keep the refusal on form
  alone (TS-05.R21). Regression: a `fileread_test.go` case on a case-insensitive temp volume.
- **Worth fixing** — observability, confirmed from the code path; INV §8. A refused read leaves no trace: `requestLog` records the path
  without the query and the status without the refusal code, so this report could not be tied to a
  link form. Log the refusal code and the requested `path` at `handleFileRead` (local-only log), so
  the next report answers "which link, which boundary".

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.

## Changelog

- **2026-09-28 — Fix BR-7 (INV §1 boundary-derived state; §16 bounded streams).** A worker port
  message now proves worker liveness for the current connection, preventing server outages from
  permanently multiplying per-tab streams. TS-03.R7 reconciled; silent-worker and failed-load
  fallbacks remain covered. The unskipped 90s-outage regression failed before the fix (two direct
  streams), then passed; initial-outage recovery and old-port teardown are also covered.
  Focused SSE tests: 26 passed; full UI: 484 passed/3 existing skips. `make dist`, both Go test
  variants and `make check-specs` pass. The first `make test` stopped on a concurrent FS-06 index
  status edit; after that session committed the correction, spec lint and both Go variants passed
  separately. BR-7 is closed; BR-6 remains open. FS-02.A27's six-tab real-browser check is still owed.
