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

- **Active change:** `complete-studio-composition` (in progress) — Studio-scoped spatial/typographic
  composition, FS-12.R46–R49/A21–A23, TS-08.R65–R68.
- **Release:** `v0.5.0` is tagged and published; **Release state** carries its contents. `v0.4.3` and
  earlier are in the state archive.
- **Review units:** `add-studio-skin` finished 2026-09-23 and awaits `/review`. `simplify-pipeline-run-detail`
  (reviewed 2026-09-25, no findings), `adopt-modern-codex-acp-capabilities` (reviewed and fixed
  2026-09-23), and `persistent-pipeline-orchestration` (2026-09-13) are closed; fix commits are not
  new units. `stop-telling-agents-to-poll` shipped outside this queue on the operator's explicit
  2026-09-10 instruction; it can be added later.
- **Work units:** `complete-studio-composition` is in progress (see Active change). `rename-product-to-deckhand.md` is Waiting to start. `migrate-internal-actions-from-mcp.md` stays
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

**Change:** `complete-studio-composition` (in progress). FS-12.R46–R49/A21–A23, TS-08.R65–R68.

**Direction (§14.1, terse):** agent state + live chat are the scan target; chrome/metadata stay
quiet. No motion. Skin-scoped CSS on existing `data-ui`/`data-slot` hooks; new slot only if R66
forces it. Verify each slice: `check:styles` + affected `npm test` + a Playwright render at
1024/1440px against the real dev UI (seeded temp `AGENTDECK_HOME`); final slice runs the full
closure matrix + A21–A23 review.

**Slices:** 1 shell/R46 **done** (compact de-boxed header/nav; route/section titles → 26-32px) ·
2 dashboard-cards/R47 · 3 expanded-card+workspace/R48 · 4 tasks-pipelines-archive-settings/R49 ·
5 overlays+closure.

**Changelog — 2026-09-25 (design: Studio composition):** The operator rejected the shipped Studio
slice as a palette change and confirmed a composition-only correction. Planned FS-12.R46–R49/A21–A23
and TS-08.R65–R68 authorize Studio-scoped spatial and typographic design across the existing
surfaces while preserving all interactions, the real dashboard chat, grid stability, and Core/Sky &
Grove. `complete-studio-composition.md` is Waiting to start; no product code changed.

**Changelog — 2026-09-23 (work: Studio skin):** Shipped FS-12.R42–R45/A18, TS-02.R36, TS-03.R45,
TS-08.R61–R64 in one slice: `studio` in `config.BuiltInAppearanceSkins`, `BUILT_IN_SKINS` and
`contract.json` v3 (new Go test ties Go to the manifest); `styles/skins/studio.css`; Settings and
matrix options. Reversible review notes: `--ad-action-secondary` is teal-blue, since forest made Info
badges match Success; forest is `--ad-border-strong`; the user-bubble rule is compound so only the
message tints. Browser pass (fake ACP, 1024/1440): dashboard, expanded pane send, agent screen,
Settings switch + reload, Archive, Pipelines, matrix; no external requests. **Still owed:** A19/A20
stay `(planned)` — live stream, permissions and real terminal unobserved (in-app SSE limit).
Pre-existing, not Studio: Sky & Grove tints the whole user event row; `--ad-shadow-project-edge`
resolves at `:root`, so card edges show fallback grey, not the project accent, in every skin.

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

**Available by role:** `/review` may take `add-studio-skin`. `/work` may take
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

None open.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
