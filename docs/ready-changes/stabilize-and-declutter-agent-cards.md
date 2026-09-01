# Stop the card grid moving when a pane opens, and make cards readable

**State:** Waiting to start
**Why:** Direct human request on 2026-09-01 after using the shipped expandable dashboard chat panes:
expanding and collapsing a card "moves around and is a terrible experience", there is no way to
collapse everything at once, an open card has almost no clickable area that collapses it, long agent
names are cut off with an ellipsis, and the context meter is not wanted on a collapsed card.
**Relevant requirements:** FS-02.R55–R59 / A37–A41 (planned), FS-12.R40 / A16 (planned),
TS-08.R45–R48 (planned) and the corrected TS-08.R43, INV §1, §2, §8, §10, §13

## Outcome

Opening or closing an agent's chat pane on the project dashboard leaves every other card exactly
where it was, one toolbar action closes all open panes, an open card has an obvious labelled control
that closes it, and an agent's full name is readable on its card instead of ending in an ellipsis.

## Included work

- An expanded pane spans one grid track instead of `min(2, perRow)`, so no card changes column or
  row when a pane opens or closes; only the rows below the pane move (FS-02.R55, TS-08.R45).
- A visible, labelled, keyboard-operable collapse control in the expanded card's header, alongside
  the existing whole-header target from FS-02.R52 (FS-02.R56, TS-08.R46).
- **Collapse all** in the card-grid toolbar, shown only while a pane on that grid is open, spanning
  every group section, and preserving the expanded ids FS-02.R49 retains for other projects
  (FS-02.R57, TS-08.R46).
- Card and expanded-card names set smaller and wrapped to at most three lines instead of truncated
  (FS-02.R58, FS-12.R40, TS-08.R47).
- The context meter leaves the collapsed card and appears as a compact figure on the expanded card,
  reusing `ContextBar`'s existing clamping, rounding, ramp, and label (FS-02.R59, TS-08.R48).
- The two `contract.json` additions the above need: one `agent-card` slot for the collapse control
  and one compact `context-meter` variant.
- Correcting TS-08.R43's stale claim that an expanded id leaves its `SortableContext`; the shipped
  grid keeps it, as FS-02.R47 requires.

Not included: card minimum height, column count, gap, and the density control keep their shipped
values — making cards larger was proposed and declined. Panes are not moved out of the grid into a
separate region; there is no expand-all, no confirmation, and no new keyboard binding. Grouping,
running-first placement, drag behavior, the four-pane cap and its LRU eviction, `Ctrl+Alt+Arrow`
pane cycling, per-agent composer drafts, `layout.json`'s shape, and every server surface are
unchanged.

## How we will know it works

- FS-02.A37 — expanding a card in a three-column grid, at `perRow` 1, and from a middle position
  leaves every other card's column and row index unchanged, with no wrap and no empty track;
  `CardGrid.test.tsx` plus a real-browser check at the 1024px desktop floor and at a wide viewport,
  in Core and Sky & Grove.
- FS-02.A38 — the collapse control's accessible name, pointer and keyboard activation, hover and
  focus feedback, the name link still navigating without collapsing, A34's pane interactions still
  collapsing nothing, and draft retention through either collapse route; `AgentCard.test.tsx`, J5.
- FS-02.A39 — **Collapse all** absent with no pane open, closing panes across two group sections in
  one press, leaving group collapse/order/density untouched, surviving a reload, keeping an
  out-of-project expanded id in the saved layout, and reporting a refused save; `CardGrid.test.tsx`.
- FS-02.A40 and FS-12.A16 — a three-line name wraps without truncation, overlap, or escape; a longer
  name clips at the third line; an unbroken token breaks; short-named cards keep their shipped
  height; `AgentCard.test.tsx`, the deterministic visual matrix, and the real-browser review in
  FS-12.A1 across both skins.
- FS-02.A41 — no context meter on a collapsed card; the expanded card's compact figure tracks the
  live value mid-turn and labels zero or absent values; the agent screen's meter is unchanged.
- `npm run check:styles`, `npm test`, and `make check-specs` per TS-06 and workflow §2.

## Waiting on

Nothing.
