# Put each pipeline stage's agents in their own dashboard group

**State:** Waiting to start
**Why:** Direct request on 2026-09-04 — "pipeline generated agents should be grouped by stage automatically in the project dashboard."
**Relevant requirements:** FS-14.R16, R58, A33; FS-02.R18, R19, R20; TS-09.R6, R33; INV §2

## Outcome

An agent a pipeline run launches arrives with its stage's label already set as its ordinary task
group, so a stage's work reads as one collapsible section on the project dashboard instead of
scattering through Ungrouped among unrelated cards. A retried or loop-revisited stage collects its
later agents in the same section. Everything a group already does comes with it and needs no new
code: the member count, the per-state summary, collapse persisted in `layout.json`, **Release
group**, and **Move to group** to take a card back out.

The group is a visual label and nothing more. The run writes it once, when it creates the stage
agent, and never re-imposes or cleans it up; the immutable run/stage association stays the only
authority for what belongs to a run (FS-14.R16, TS-09.R6).

## Included work

- In `stageExecution` (`internal/pipeline/reconcile.go`), compute `stage.Title + " — " +
  run.DisplayName` once and use it for both the stage agent's name and its group label, so the
  convention has one home and the two cannot drift (INV §2). No new `StageExecution` field.
- Pass that value as the existing `launchRequest.Group` at the one stage-launch site in
  `internal/server/pipeline_lifecycle.go`.
- When it ships: flip the `(planned)` tags on FS-14.R58/A33 and TS-09.R33.

That is the whole change. Notably **not** needed, each checked against the code rather than assumed:

- No guard for a reused agent id. `reconcile.go` mints a fresh id unless the attempt is a blocked
  continuation, and continuations reconcile to `resume_blocked` → `ContinueStage`, which resumes the
  stored agent and never composes a launch request. `LaunchStage` only ever sees a fresh id.
- No guard for an empty or reserved label. A stage title is validated required, an omitted run name
  defaults to the template title, and `CardGrid` renders an empty or literal `_ungrouped` label as
  the Ungrouped section anyway.
- No dashboard, `layout.json`, `state.Agent`, or API change, and no migration.
- No pipeline-owned group concept: no group kind or badge, no regrouping as a run advances or ends,
  no label cleanup when a run is deleted, no section derived from the run/stage join.
- No change to what **Release group** does. It stops members through the ordinary lifecycle path, so
  the run then pauses under FS-14.R12 with its recovery actions rather than being stopped as **Stop
  run** stops it (FS-14.R13). The user chose this over hiding the control.

## How we will know it works

- **FS-14.A33** — a fake run whose second stage is retried stores each stage's label on its agents,
  putting the retried stage's two agents in one section and each other stage's agent in its own;
  those sections carry the count, summary, persisted collapse, and **Release group**; a member moved
  out with **Move to group** keeps its run/stage badge and its place on the run's page; releasing a
  stage's group stops its members and leaves the run paused with recovery actions rather than
  finished. A pipeline launch test asserting the composed label plus
  `ui/src/components/grid/CardGrid.test.tsx`.
- Existing **FS-14.A2** and **A5** must stay green: one agent at a time, and restart recovery
  unchanged.

## Waiting on

Nothing.
