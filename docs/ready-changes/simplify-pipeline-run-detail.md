# Simplify pipeline run detail

**State:** Waiting to start
**Why:** The operator reported on 2026-09-23 that Frozen setup and Named values add no useful run-page context, the value grid is unreadable, and the rail can overlap an opened stage. The operator confirmed removal of both panels after a task-focused UX review.
**Relevant requirements:** FS-14.R79, FS-14.A46, TS-08.R60, INV §8, INV §10, INV §13, INV §17

## Outcome

A person can supervise a pipeline from its live state, current and next stage, recovery actions, and full-width attempt timeline without setup/value panels competing with or covering stage work.

## Included work

Replace the two-column run page with a full-width timeline; show stage position and upcoming stage in the live summary; use human stage titles and keep each attempt's result, runtime, agents, and outputs together. Remove the setup/value rail, its obsolete presentation hooks, and rail-only styles. Preserve existing run/API data, retention, agent behavior, start flow, and template editor.

## How we will know it works

FS-14.A46 and J14: active, paused, and finished runs remain readable at the supported desktop floor and a wider viewport, including a long expanded attempt, long outputs, and Core/Sky & Grove. Focused run-page checks verify stage-title/position fallback, next-stage behavior, recovery context, and no duplicate panels. Presentation-contract checks verify removed slots and defined selectors.

## Waiting on

None.
