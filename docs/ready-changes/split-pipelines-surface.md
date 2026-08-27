# Split the Pipelines surface into runs and templates

**State:** Waiting to start
**Why:** Direct human request on 2026-08-27 — "the pipeline view is terrible; the template editor
should be separate from run history or existing runs; running pipelines should be their own page
with a better UI showing which stage it's on, and results". Captured as the "Pipelines surface
refactor" idea and defined with the user in the same session.
**Relevant requirements:** FS-14.R35–R45, FS-14.A14–A23, FS-14.R25 (superseded), TS-02.R26,
TS-09.R28, TS-03.R29–R30, TS-08.R7/R8, INV §7, INV §8, INV §10

## Outcome

Authoring a pipeline template and supervising a live run stop sharing one scrolling screen. A person
opens Pipelines and lands on Runs; each run has its own addressable page that shows where the run
actually is — every attempt in the order it ran, each with its outcome, result, and the agents that
worked on it, including the agents its stage agent delegated to. Templates get their own
sub-destination and a focused full-width editor. The new pages use deliberate hierarchy,
progressive disclosure, stable loading and navigation geometry, and restrained motion rather than
moving the existing dense panels to different URLs.

## Included work

**In scope**

- Four routes replacing the single `/pipelines` page: Runs (default), a run page, Templates, and a
  template editor page, with a sub-destination switcher (FS-14.R35).
- A compact Runs ledger with frozen human stage titles, exact retained total, bounded **More runs**
  pagination, and a three-step start dialog that opens the started run (FS-14.R36, R44).
- Run page: goal/project/state/final outcome, attention reason, frozen setup with per-stage
  runtime, named values with provenance, and the existing controls under unchanged gating
  (FS-14.R37).
- A timeline-first run layout: live state/actions stay primary; attempts appear in execution order;
  completed results, frozen setup, and named values use intentional disclosure (FS-14.R38, R44).
- Compact, read-only agent cards per attempt — the stage agent plus one hop of delegated agents —
  linking to a live conversation or an archived transcript (FS-14.R39), and a count of delegated
  agents still running after the run advanced or ended (FS-14.R40).
- Templates library, focused one-stage-at-a-time editor, and the AgentDecker builder, with pending
  proposals placed on the sub-destination that can act on them and counted on the other
  (FS-14.R41, R42, R44).
- Legacy `?run=` links resolving to the run's page; a deleted run or template explaining its absence
  (FS-14.R43).
- Brief route/dialog/disclosure/live-entry motion, stable background-refetch content, and a complete
  reduced-motion equivalent using the existing CSS/Radix presentation seam (FS-14.R45).
- Server: `GET /api/pipeline-runs/{id}` gains the additive, capped attempt-agent block, composed at
  the HTTP boundary from targeted attempt-window task and bulk agent-state reads; the Runs response
  stays an array while adding frozen stage titles and an `X-Total-Count` header (TS-09.R28,
  TS-03.R29–R30).
- State: one forward-only composite task-read index supports the creator/generation/time-window
  projection; no task or pipeline authority changes (TS-02.R26).
- Client: an open run page invalidates its detail query on `task_update` as well as
  `pipeline_update` (TS-03.R29).
- Presentation hooks for the new pipeline surfaces registered in
  `ui/src/presentation/contract.json` — an existing obligation the current Pipelines page never met
  (TS-08.R7/R8).

**Not in scope**

- No change to the template model, stage semantics, routing, approval gates, loop bounds, recovery,
  or the run state machine. No new attention reason and no change to FS-14.R7's advance rule.
- No parallel stage execution or joins; a stage still runs exactly one agent of its own at a time.
- No graph canvas.
- No agent-facing or MCP change: no tool, refusal code, or agent payload is touched.
- No dashboard changes — surfacing active runs on the dashboard was considered and deliberately
  excluded as a separate FS-02 decision.
- No agent lifecycle mutations from the run page; stop/rename/clone stay on FS-01/FS-02 surfaces.
- No new database column or table; the one read-only index migration above is the complete durable
  schema delta.
- No FS-12 per-surface item: FS-12 carries no Pipelines entry today, and its shared page-frame,
  route-heading, dialog, and repeated-surface rules already govern the new pages.

## How we will know it works

- **FS-14.A14** — landing on Runs, Templates showing no run history, and each run and template page
  reloading from its own URL. Pipelines routing tests and J14.
- **FS-14.A15** — the start dialog collecting the full run-start surface, a valid start opening the
  new run's page, and a rejected start keeping values with named field errors and starting no
  process. Run-setup UI tests and J14.
- **FS-14.A16** — a looping run showing each visit as its own timeline entry in execution order, and
  an unreported attempt shown as unreported. Run-detail UI test over a fake looping run and J14.
- **FS-14.A17** — three cards for a stage agent that created two tasks, live versus archive link
  targets, a second-hop agent absent, no lifecycle action on any card, and the still-running count
  after a run finishes. Run-detail UI tests over a delegated-task fixture and J14.
- **FS-14.A18** — proposals actionable on their own sub-destination, counted on the other, and still
  consumed exactly once. `ui/src/features/pipelines/AgentDeckerBuilder.test.tsx` and J14.
- **FS-14.A19** — a legacy `?run=` link resolving, and a deleted run or template explaining its
  absence. Pipelines routing tests.
- **FS-14.A20–A21** — empty/populated/maximum-density visual fixtures, one-stage-at-a-time editing,
  a stable three-step start flow, restrained continuity motion, content retained during refetch, and
  the same usable sequence under reduced motion. Presentation checks and J14.
- **FS-14.A22** — 121 retained runs loading in bounded pages to an exact end state while frozen
  stage titles survive source-template edit/deletion. Run-list API/UI tests and J14.
- **FS-14.A23** — reload-safe stage/delegate cards with bounded previews, explicit routes, and a
  non-linking unavailable fallback before global live hydration. Run-detail API/UI tests and J14.
- Server/state tests that the delegated block caps at 20 per attempt, reports true totals and
  distinct running counts, attributes same-generation continuation tasks to one attempt, follows
  exactly one hop, reads no arms, holds query count fixed, and uses the new composite index
  (TS-02.R26, TS-09.R28, TS-03.R29).
- `make test`, `make build`, `cd ui && npm test && npm run build`, `make check-specs`,
  `git diff --check`, and the presentation-contract checker.

## Waiting on

Nothing. Every product and technical decision named during design is resolved: one nav entry with
sub-tabs, execution-order timeline, one-hop delegated agents shown read-only, still-running
delegates disclosed without a new attention reason, start-run in a dialog on Runs, agent-plus-task
summary payload, focused high-density authoring/supervision layouts, reduced-motion behavior,
paginated retained history, and a capped indexed projection with an honest overflow line.
