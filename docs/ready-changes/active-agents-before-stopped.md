# Place running agents before stopped ones in a project grid

**State:** Waiting to start
**Why:** Direct human request on 2026-08-27 — "put inactive agents after active ones in the project view by default."
**Relevant requirements:** FS-02.R45, FS-02.A28, FS-02.R6, FS-02.R12, FS-02.R14, FS-02.R18, FS-02.R36, FS-12.R37, FS-12.A13, FS-12.R10, INV §8, INV §10

## Outcome

Opening a project dashboard puts the agents that are actually running at the top of each group, so
supervision starts with live work instead of with whatever position a card happens to hold in a
manual order. Stopped agents stay visible and reachable, just after the running ones.

## Included work

Included: a running-before-stopped split applied inside each group section of an agent card grid,
preserving the persisted manual order within each block; live movement across the boundary when an
agent starts or stops; unchanged group section ordering with Ungrouped last; and one derived display
order shared by the rendered cards and dnd-kit's sortable registry. A completed drag inside a block
still translates its active and target ids against the existing flat manual order before persistence.
Dropping onto the other running/stopped block is a no-op and issues no layout write, so drag cannot
override the running-first boundary or make a hidden manual change.

Also included: narrowing FS-12.R10. That requirement currently says waiting-input and error states
receive higher salience "without changing order, grouping, or action behavior," which as written
forbids status driving order. FS-12.R37 narrows the clause to the five live `state` values it was
written about and leaves `running` free to order cards. Shipping the sort without this would leave
two specs contradicting each other.

Not included: any new persisted preference or control — `layout.json` keeps exactly the order,
density, and per-group collapse state it holds today; per-project or per-status ordering, which
FS-02.R36 and its confirmed project-first boundary exclude; ordering by `AgentStatus` (`busy`,
`idle`, `waiting_input`, `done`, `error`, or the `unknown` fallback), so a `busy` and an `idle`
running agent keep their relative manual order; and the separate cross-group and whole-card
drag-and-drop usability problems recorded in `docs/ideas.md`. Those broader problems are unchanged.

## How we will know it works

- **FS-02.A28** — new cases in `ui/src/components/grid/CardGrid.test.tsx` beside the existing
  scoped-reorder case: a group with interleaved running and stopped agents renders every running
  agent first while preserving manual order inside each block; a `state_update` flipping one agent's
  `running` moves only that card; the sortable registry receives the exact rendered id order; a drag
  inside a block shifts no card from the other block and still produces the same `PUT /api/layout`
  payload as today; a cross-block drop produces no layout request; the Ungrouped section stays last.
- **FS-12.A13** — a `waiting_input` or `error` card raises its salience while holding position, so
  position changes only with `running`.
- **J5** in `docs/features/USABILITY-REVIEW.md` — in a real browser, stop a running agent and watch
  only that card cross the boundary with no reload, drag inside a block and confirm that the other
  block holds position and the order survives a reload, then drop across the boundary and confirm
  that nothing moves or persists.
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
- `running` is a boolean computed separately from the six-value `AgentStatus` union, including its
  `unknown` fallback, and is already what the card treats as active — `AgentCard` keys its dimmed
  `stopped` class and marker off it. This is why R45 uses `running` and not `state`.
- dnd-kit derives sortable indices and measured-rectangle transforms from the order passed to
  `SortableContext`. The implementation must therefore flatten the visible, expanded grouped
  running-first projection into that registry order instead of passing the differently ordered flat
  manual ids or ids hidden by collapsed groups. The existing manual ids remain the source for
  `arrayMove` and `mergeScopedOrder` after a valid same-block drop. A `running` mismatch between the
  active and target agents returns before either helper or the layout write runs; cross-group
  behavior otherwise remains the existing separate concern.
- There is no existing test scaffolding for status-based placement, so A28 adds new cases rather
  than extending one.

## Waiting on

Nothing.
