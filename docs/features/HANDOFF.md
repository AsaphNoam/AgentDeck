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

- **Active change:** None. `complete-studio-composition` was corrected 2026-09-25 and awaits `/review`.
- **Release:** `v0.5.0` is tagged and published; **Release state** carries its contents. `v0.4.3` and
  earlier are in the state archive.
- **Review units:** `add-studio-skin` (finished 2026-09-23) and `complete-studio-composition`
  (finished 2026-09-25) both await `/review`. `simplify-pipeline-run-detail`
  (reviewed 2026-09-25, no findings), `adopt-modern-codex-acp-capabilities` (reviewed and fixed
  2026-09-23), and `persistent-pipeline-orchestration` (2026-09-13) are closed; fix commits are not
  new units. `stop-telling-agents-to-poll` shipped outside this queue on the operator's explicit
  2026-09-10 instruction; it can be added later.
- **Work units:** `rename-product-to-deckhand.md` is Waiting to start. `migrate-internal-actions-from-mcp.md` stays
  paused on its transport blocker. Queue hygiene: `bump-pinned-acp-adapters.md` reads
  `State: Finished` but is still in `docs/ready-changes/`; left in place rather than deleted unasked.
- **Design units:** `Ideas being defined` entries may resume (the operator deleted the
  uncommitted Cursor backend draft on 2026-09-23); `New ideas`
  entries are available; the permanently unaddressable pipeline agent needs `/design-feature`.
- **Open findings:** None. `adopt-modern-codex-acp-capabilities`, `persistent-pipeline-orchestration`, BR-1, and BR-4 are all closed.
  The injected-steer lifetime edge case is still named in prose but was never recorded
  as a finding; it needs `/investigate-bug` before `/fix` can take it.
- **Bug reports:** BR-1, BR-2, BR-3, and BR-4 are investigated, fixed and closed. Pinned Claude model
  delivery through `_meta` works; an ACP model `currentValue` is adapter configuration evidence and
  no execution-model oracle (TS-04.R54). BR-4 (2026-09-22, "the main project page looks off, the
  cards are stretched and stuck to the bottom") is fixed the same day; see **Review findings**.
- **State:** Automated MCP contract verification is green.
- **Branch:** `main`.

## Active change

**Change:** None. `complete-studio-composition` was corrected 2026-09-25 and awaits `/review`.

**Changelog — 2026-09-25 (work: Studio composition correction, FS-12.R46–R49/
TS-08.R65–R67):** Repaired the real dashboard and full-screen agent-workspace composition: Studio
now removes the Core grid gap and makes the header, tabs, transcript, and dashboard composer one
clipped surface without changing their scroll or focus ownership. Tasks, Pipelines, Archive, and
Settings now use their required Studio hierarchies (attention strip/rows, ledger and authoring
surfaces, search-first records, and a quiet navigation spine), with the same bounded treatment for
onboarding and overlays. Contract v4 exposes neutral presentation hooks for the shell, dashboard
composer, pipeline workspace/sections, and appearance preview. Production skin CSS is now checked
to reject implementation-class selectors; Sky & Grove was migrated to those preview hooks too.

Rendered evidence: Visual Matrix Studio at 1024px; the built application at 1024px for Settings,
empty Tasks, Pipelines, and Archive; and the Tasks authoring layout at 1440px with no horizontal
overflow. `ui/npm run check:styles` (37 tests), `ui/npm test` (458 tests), `ui/npm run build`,
`make embed && make build`, and `make test` (both Go tag variants) pass. **Still owed:** A21–A23
and TS-08.R68 remain `(planned)`: populated long/dense task, pipeline, template, and archive
states; every Settings/onboarding state; and desaturated review beyond the dashboard were not
available as deterministic evidence in this correction.

**Changelog — 2026-09-25 (design: complete Studio composition, FS-12.R46–R49/TS-08.R65–R67
shipped):** The operator rejected the first Studio slice as a palette change; this shipped the
compositional upgrade in five skin-scoped-CSS-only slices (no TSX changes): shell (compact
de-boxed header/nav, 26–32px titles, including the shared `[data-ui="page-header"]` hook Tasks/
Pipelines use — missed in slice 1, still 51px); dashboard cards (accents bounded to a spine/strip
instead of a full-card wash, preview promoted, metadata quieted); expanded card + full agent
screen (header/tabs/transcript/composer joined into one surface instead of three boxed panels,
`TranscriptView`/`Composer` unchanged); Tasks/Pipelines/Archive/Settings (attention-first rows,
ledger/authoring chrome, search-first Archive, quiet nav spine); overlays (same bounded-accent
language — a border-ordering bug that let a quieter default override the toast/permission-prompt
state accent was caught and fixed before commit).

Verified every slice: `check:styles`, full `npm test` (458 pass), Playwright renders against a
live fakeacp dev UI. Closure: `make test` (both tags), `make build`, `ui/npm run build` pass. A
desaturated Core/Sky & Grove/Studio triptych of the dashboard confirms the difference is layout,
not palette. **Still owed** (A21–A23, TS-08.R68 stay `(planned)`): the exhaustive per-surface
state matrix (task attention rows, populated ledger/timeline, template editor, archived view,
every Settings section, onboarding) and a desaturated pass beyond the dashboard — same posture
the first slice left for A19/A20.

**Changelog — 2026-09-23 (work: Studio skin):** Shipped FS-12.R42–R45/A18, TS-02.R36, TS-03.R45,
TS-08.R61–R64: `studio` skin id, `contract.json` v3, `styles/skins/studio.css`, Settings/matrix
options. **Still owed then, closed 2026-09-25:** A19/A20's composition gap — see the 2026-09-25
entry below. Pre-existing, not Studio: Sky & Grove tints the whole user event row;
`--ad-shadow-project-edge` resolves at `:root`, so card edges show fallback grey in every skin.

**Changelog — 2026-09-23 (work: simplify pipeline run detail):** Shipped FS-14.R79 and TS-08.R60 in
one slice. `RunBrowser.tsx` drops the setup/value rail for a full-width timeline; the live summary
shows "Stage N of M", "Next: <title>" while active, and a compact project · template line; attempts
use the frozen template's stage title with the stage id as fallback. The `setup`/`values` slots left
`presentation/contract.json` (schema stays version 2); rail-only CSS was removed (`.pipeline-disclosure` stays for
the template editor). API and stored data are unchanged. Closure matrix passed. **Still owed:** A46's
real-browser J14 pass (Core and Sky & Grove, desktop floor and wider, long expanded attempt) — A46
stays `(planned)`. Review note (reversible): the run id kicker stays in the hero as run identity;
the per-task id line was removed as an opaque id.

**Changelog — 2026-09-23 (work+fix: modern Codex ACP capabilities):** Eight slices shipped Codex ACP
1.12.0/CLI 0.154.0 (rebased steering patch, negotiated capabilities, canonical tool names, live-only
reasoning, nested children, background tasks with Stop, Clone as `session/fork`, file-change tracking
supplements); the follow-up fix closed all three review findings (INV §2/§3/§15 clone builder/discard,
INV §11 permission scope, INV §11/§17 malformed report). Closure matrix and a fake-ACP browser pass
(Core) passed. **Still owed before release:** the credentialed Codex 1.12.0 receipt (TS-06.R26,
`(planned)`), gating FS-03.A41/A42 and FS-01.A20 (J7); Sky & Grove unviewed. Settled 2026-09-14
entries are in [`HANDOFF-through-2026-09-14`](../archive/state/HANDOFF-through-2026-09-14.md).

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

**Available by role:** `/review` may take `add-studio-skin` or `complete-studio-composition`.
`/work` may take `rename-product-to-deckhand`; `/fix` has no open findings;
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

None open.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
