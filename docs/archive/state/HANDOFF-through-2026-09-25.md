# AgentDeck — archived handoff state through 2026-09-25

Settled changelog entries moved out of [`../../features/HANDOFF.md`](../../features/HANDOFF.md) to
keep its session-start header inside budget. Nothing here is live state; the handoff carries the
resumable position, including any debt these entries still owe.

## Changelog

**Changelog — 2026-09-25 (design: simplify agent and automation setup):** User-approved suggestions
7–11 are one Waiting-to-start change in `docs/ready-changes/simplify-agent-and-automation-setup.md`:
compact New Agent, removal of unavailable config actions, optional provider-aware config linking,
named task dependencies and collapsed pipeline runtime setup. Planned FS-01.R37, FS-04.R49,
FS-08.R35–R36, FS-12.R50, FS-14.R80, FS-16.R39–R40 and TS-08.R69–R72 define behavior and gates.
No product code changed; implementation owns browser acceptance. `make check-specs`, twin-skill
comparison and `git diff --check` pass. Additional expert-workflow
suggestions were researched separately and are not part of this approved unit.

**Changelog — 2026-09-25 (work: Studio composition correction, FS-12.R46–R49/
TS-08.R65–R67):** Repaired the real dashboard and full-screen agent-workspace composition: Studio
now removes the Core grid gap and makes the header, tabs, transcript, and dashboard composer one
clipped surface without changing their scroll or focus ownership. Tasks, Pipelines, Archive, and
Settings now use their required Studio hierarchies (attention strip/rows, ledger and authoring
surfaces, search-first records, and a quiet navigation spine), with the same bounded treatment for
onboarding and overlays. Contract v4 exposes neutral presentation hooks for the shell, dashboard
composer, pipeline workspace/sections, and appearance preview. Production skin CSS is now checked
to reject implementation-class selectors; Sky & Grove was migrated to those preview hooks too.

Rendered evidence at this correction checkpoint: a Visual Matrix Studio view with a 1024px viewport
request; built-app Settings and empty Tasks, Pipelines, and Archive under viewport requests; and a
wider Tasks authoring view without horizontal overflow. The later acceptance pass above established
populated states but found the browser's reported viewport did not honor the requested 1024px floor.
`ui/npm run check:styles` (37 tests), `ui/npm test` (458 tests), `ui/npm run build`,
`make embed && make build`, and `make test` (both Go tag variants) pass.

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
not palette. **Still owed at this earlier checkpoint** (A21–A23, TS-08.R68 stayed `(planned)`):
the exhaustive per-surface state matrix (task attention rows, populated ledger/timeline, template
editor, archived view, every Settings section, onboarding) and a desaturated pass beyond the
dashboard. The later acceptance pass above covers some of those states but does not close the gate.

**Changelog — 2026-09-23 (work: Studio skin):** Shipped FS-12.R42–R45/A18, TS-02.R36, TS-03.R45,
TS-08.R61–R64: `studio` skin id, `contract.json` v3, `styles/skins/studio.css`, Settings/matrix
options. **Implementation gap corrected 2026-09-25:** the composition issue underlying A19/A20 —
see the correction entry above. Acceptance remains planned. Pre-existing, not Studio: Sky & Grove
tints the whole user event row;
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
entries are in [`HANDOFF-through-2026-09-14`](HANDOFF-through-2026-09-14.md).

