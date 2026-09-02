# Open a conversation automatically when it stops for an approval

**State:** Waiting to start
**Why:** Direct human request on 2026-09-01: "make conversations waiting for approval open
automatically." It reverses one clause of FS-02.R51, which forbade any automatic expansion.
**Relevant requirements:** FS-02.R61 / A43 (planned), the narrowed FS-02.R51, TS-08.R49 (planned),
INV §1, §2, §10

## Outcome

A chat agent that stops for a permission decision puts its conversation in front of the person
looking at the dashboard, instead of only raising a badge and a toast they have to act on.

## Included work

- A chat agent newly entering `waiting_input` expands its own pane on the grid rendering it
  (FS-02.R61). No other event opens a pane, and R51's rule still holds for all of them.
- Only a newly observed transition opens a pane: a reload, a reconnect, a completed hydration
  burst, an agent first seen already waiting, and returning to the dashboard with an agent already
  waiting all open nothing (FS-02.R61, TS-08.R49).
- Collapsing an auto-opened pane keeps it collapsed until the agent asks again; answering the
  request leaves the pane open. **Collapse all** (FS-02.R57) clears an accumulation.
- With four panes open, an automatic expansion evicts the least-recently-used pane exactly as a
  fifth manual expansion does, preserving that pane's unsent draft (FS-02.R48).
- Excluded from expanding: terminal-interface agents, agents outside the grid's project, and agents
  inside a collapsed group section — the section is not expanded either.
- Detection reads the durable `state` on `state_update` inside `CardGrid`, reusing the existing
  expansion path, cap, and recency; it does not read the `notification` stream, which the mute list
  filters and the server never replays (TS-08.R49).

Not included: no preference to disable the behavior, no automatic collapse after the request is
answered, no sound or focus change, no automatic expansion of a collapsed group section, no
automatic open on `done`, `error`, mail, budget, or pipeline events, and no server, SSE, endpoint,
or `layout.json` change.

## How we will know it works

- FS-02.A43 — a `busy` → `waiting_input` transition expands the pane; the same agent already
  waiting across a reload, a hydration burst, and a reconnect expands nothing; an agent first
  observed as `waiting_input` expands nothing; collapsing keeps it collapsed until the next
  transition; answering leaves it open; a fifth waiting agent evicts the least-recently-used pane
  and its draft returns; five at once leave four open; terminal, cross-project, and
  collapsed-section agents expand nothing; a muted `waiting_input` notification does not suppress
  the expansion. — `ui/src/components/grid/CardGrid.test.tsx`,
  `ui/src/store/agentStore.test.ts`, J5.
- A real-browser check that an opening pane moves only the rows below it (FS-02.R55) and that the
  eviction is visible rather than silent.
- `npm run check:styles`, `npm test`, and `make check-specs` per TS-06 and workflow §2.

## Waiting on

Nothing.
