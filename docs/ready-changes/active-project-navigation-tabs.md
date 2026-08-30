# Active-project navigation tabs

**State:** Waiting to start
**Why:** Direct human request on 2026-08-30 to switch between projects with running agents without
returning to the projects home.
**Relevant requirements:** FS-02.R54/A36, FS-12.R39/A15, TS-08.R44, INV §1, §2, §8, §10, §13

## Outcome

The shell will show compact, project-colored navigation for configured projects with running
agents, keep the current project visible, and place projects beyond the five-link cap under `+n`.

## Included work

Derive membership from the existing project query and hydrated agent store; select the project on
project and agent routes; use title/id alphabetical ordering; fit five smaller rounded links plus
overflow in the existing header at the desktop floor; and cover Core and Sky & Grove. The operator
is an experienced frequent user, so the strip stays secondary to primary navigation, updates
without motion, and uses running state plus project accent as AgentDeck-native information.

No unavailable-project link, pinning, recency or activity ranking, counts, measurement,
persistence, second row, new API, server change, dependency, token, public presentation hook, or
skin mechanism is included. `Header.tsx`, `useProjects`, `useAgentStore`, and the existing
`--ad-project-accent` exception seam provide the required data and composition directly.

## How we will know it works

FS-02.A36 proves live membership, current-context retention, alphabetical five-link overflow, and
hydration cleanup. FS-12.A15 proves keyboard access, selected and project-color semantics, title
truncation, both appearances, and non-overlap at 1024px and a wider desktop viewport. J5 supplies
the real-browser switching and lifecycle pass.

## Waiting on

Nothing.
