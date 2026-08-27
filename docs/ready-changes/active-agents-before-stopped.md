# Place running agents before stopped ones in a project grid

**State:** Waiting to start
**Why:** Direct human request on 2026-08-27 — "put inactive agents after active ones in the project view by default."
**Relevant requirements:** FS-02.R45, FS-02.A28, FS-02.R6, FS-02.R12, FS-02.R14, FS-02.R18, FS-02.R36, FS-12.R37, FS-12.A13, FS-12.R10, INV §8

## Outcome

Opening a project dashboard puts the agents that are actually running at the top of each group, so
supervision starts with live work instead of with whatever position a card happens to hold in a
manual order. Stopped agents stay visible and reachable, just after the running ones.

## Included work

Included: a running-before-stopped split applied inside each group section of an agent card grid,
preserving the persisted manual order within each block; live movement across the boundary when an
agent starts or stops; and unchanged group section ordering with Ungrouped last.

Also included: narrowing FS-12.R10. That requirement currently says waiting-input and error states
receive higher salience "without changing order, grouping, or action behavior," which as written
forbids status driving order. FS-12.R37 narrows the clause to the five live `state` values it was
written about and leaves `running` free to order cards. Shipping the sort without this would leave
two specs contradicting each other.

Not included: any new persisted preference or control — `layout.json` keeps exactly the order,
density, and per-group collapse state it holds today; per-project or per-status ordering, which
FS-02.R36 and its confirmed project-first boundary exclude; ordering by the five `state` values, so
a `busy` and an `idle` running agent keep their relative manual order; and the separate
drag-and-drop usability problems recorded in `docs/ideas.md`, which this change neither fixes nor
worsens.

## How we will know it works

- **FS-02.A28** — new cases in `ui/src/components/grid/CardGrid.test.tsx` beside the existing
  scoped-reorder case: a group with interleaved running and stopped agents renders every running
  agent first while preserving manual order inside each block; a `state_update` flipping one agent's
  `running` moves only that card; a drag inside a block still produces the same `PUT /api/layout`
  payload as today; the Ungrouped section stays last.
- **FS-12.A13** — a `waiting_input` or `error` card raises its salience while holding position, so
  position changes only with `running`.
- **J5** in `docs/features/USABILITY-REVIEW.md` — in a real browser, stop a running agent and watch
  only that card cross the boundary with no reload, then drag inside a block and confirm the order
  survives a reload.
- The existing `TestLayoutDefault` and `TestPutLayoutValidatesAndPersists` stay green, since the
  server contract is untouched.

## Verified evidence behind the design

Checked against the current code:

- "The project view" is `/project/:id` → `ScopedProjectDashboard` → `CardGrid`. The projects home
  renders project cards, not agent cards.
- Ordering today is a single flat, global, server-persisted `order` array of agent ids in
  `layout.json`, with no status input at all; group sections are sorted alphabetically with
  `_ungrouped` forced last, and within a section cards fall in whatever position they hold in that
  one global array.
- `running` is a boolean computed separately from the five-value `state` enum and is already what
  the card treats as active — `AgentCard` keys its dimmed `stopped` class and marker off it. This is
  why R45 uses `running` and not `state`.
- There is no existing test scaffolding for status-based placement, so A28 adds new cases rather
  than extending one.

## Waiting on

Nothing.
