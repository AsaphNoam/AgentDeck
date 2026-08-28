# Usability review — the split Pipelines surface — 2026-08-28

## Scope and setup

- **Scope:** the Pipelines redesign that is currently staged in the working tree — the split into
  Runs and Templates sub-destinations, the run page and its execution timeline, the attempt agent
  cards, the focused template editor, the three-step start dialog, legacy-link resolution, bounded
  run history, and the motion/reduced-motion behavior. Requirements exercised: FS-14.R35–R45 and
  FS-14.A14–A23, plus the R11/R12/R13/R20/R21 gating those requirements carry forward.
- **Browser rung:** 1 — Playwright driving the environment's cached Chromium (`chromium-1228`), with
  console errors, page errors, and failed requests captured on every run.
- **Build:** `make dist` (embed + `sqlite_fts5` binary at `bin/agentdeck`), built from the working
  tree so the staged redesign is what was driven. The embed step produced no change to
  `internal/server/ui/dist/**` beyond what was already staged.
- **Fixtures:** three isolated `AGENTDECK_HOME` copies under `.review/usability-20260828-pipelines/`,
  one port each. `empty` (port 4503) is an onboarded home with no templates and no runs; `pipe`
  (port 4501) is the same home used to start one real run through the UI; `rich` (port 4502) adds a
  seeded seven-attempt repair-loop run, three paused runs covering the approval / blocked-result /
  crashed gating branches, 121 retained runs, 26 delegated tasks on one attempt, a second-hop task,
  a task whose assigned agent record is absent, a 32-stage/64-declaration template, and one pending
  proposal of each kind. The deterministic `fakeacp` peer was exposed as `claude-agent-acp` /
  `codex-acp` through a PATH shim.
- **Evidence:** 56 screenshots under `.review/usability-20260828-pipelines/run/shots/`; the
  load-bearing ones are copied to
  [`usability-review-2026-08-28-pipelines-evidence/`](usability-review-2026-08-28-pipelines-evidence).

## Journey results — J14, split-surface portion

| Step | Requirement | Result | Observation |
|---|---|---|---|
| Land on Runs | A14 | PASS | `/pipelines` redirects to `/pipelines/runs`. |
| Templates shows no run history / no start form | A14 | PASS | Only the library, the two create actions, and the AgentDecker panel. |
| Runs shows no editor | A14 | PASS | Zero editor nodes after switching back. |
| Run page and template page reload from their own URL | A14 | PASS | Every check below entered by direct navigation. |
| Heading + Runs/Templates switcher spatially stable | R44 | PASS | Identical rects `[32,126,599,45]` / `[32,225,1216,48]` on all four routes. |
| Empty Runs ledger | R44, state variant | PASS | "No runs yet — Start a run to turn a reusable template into supervised work." |
| Empty Templates library | R41, state variant | PASS | "No templates yet — Start manually or ask AgentDecker to shape a reusable pipeline from a description." |
| Start dialog collects the full start surface | A15 | PASS | Setup (template, name, project, goal, named inputs), Runtimes (per-stage backend/model/effort), Review. |
| Back/Next preserve every value | R44 | PASS | All three fields returned verbatim after Review → Back → Back. |
| Valid start opens the new run's page | A15 | PASS | Landed on `/pipelines/runs/pr_7db26550b60c7bb9`. |
| Shared-workspace confirmation | R21 | PASS | Named the four conflicting runs and two agents; **Confirm shared workspace and start** completed the start. |
| Rejected start keeps values | A15 | PASS | Dialog stayed open and all three fields survived. |
| Missing required input | A15 | **FAIL** | Finding 3 — Next is disabled with no stated reason. |
| No templates saved | A15/R41 | **FAIL** | Finding 4 — Start run opens a dialog that cannot proceed and does not say why. |
| Repair loop as repeated entries in execution order | A16 | PASS | Seven entries: Work v1, Review v1, Validate v1 (failure), Fix v1, Review v2, Review v3 (codex · gpt-5.6-sol), Validate v2 — each naming stage, visit, runtime, outcome and result. |
| Unreported attempt | A16 | PASS | Entry 5 reads "No stage result was reported for this attempt." |
| Current/attention entry stays open, completed stay compact | R44 | PASS | Only entry 7 was open on load. |
| Three cards for a stage agent that created two tasks | A17 | PASS | Stage agent + both assigned agents on the same attempt. |
| Second-hop agent absent | A17 | PASS | The task created by a delegated agent does not appear. |
| Live vs archive link targets | A17/A23 | PASS | A running delegate links `/agent/a_8a2627`; stopped ones link `/archive/{id}`. |
| No lifecycle action on any card | A17 | PASS | Zero buttons inside every expanded entry. |
| Delegated cap and true totals | A17 | PASS | "1 delegated still running · Showing 20 of 26 delegated"; API totals 26/1. |
| Non-linking fallback card | A23 | PASS | A retained task whose agent record is gone renders one honest card named from its task, `route: unavailable`, no link. |
| Detail-response render before live hydration | A23 | PASS | Cards render from the detail payload on reload, then the live snapshot corrects state and route in place. |
| Proposals placed and counted | A18 | PASS | `start_run` actionable on Runs, `save_template` actionable on Templates, both tab badges visible from either side. |
| Legacy `?run=` link | A19 | PASS | `/pipelines?run=…` resolves to the run's page. |
| Deleted run / template | A19 | PASS (copy nit) | Both explain the absence and offer a return link. Finding 5 records the run page's raw API string. |
| 121 retained runs, bounded pages | A22 | PASS | 50 → 100 → 121 with no duplicates; "121 of 121 retained runs · Complete history loaded"; **More runs** removed. |
| Total stated on the first page | A22 | PASS | "50 of 121 retained runs" before any pagination; `X-Total-Count: 121`. |
| Frozen stage titles survive template deletion | A22 | PASS | Ledger still showed Work / Validate after the source template was deleted. |
| Editor mounts one stage form | A20 | PASS | One textarea and one stage card for a 32-stage template; navigator lists all 32. |
| Stage switching preserves the unsaved draft | R44 | PASS | An edit to stage 1 survived a trip to stage 6 and back, and updated the navigator label live. |
| Validation identifies stage and field | R44 | PASS | "stages.0.id — must be a lowercase slug up to 63 characters" plus a `!` marker on the affected navigator item. |
| Run actions under R11/R12/R13/R20 gating | R37 | PASS | Approval-paused → Approve and continue; blocked result → Continue (gated on the new-input box) + Retry stage; crashed → Retry stage only; completed → Delete run record. Approving advanced Review → Validate live with no reload. |
| Delete run record | R37 | PASS | Returned to Runs, row gone, revisiting the URL explains the absence. |
| No horizontal page overflow at the desktop floor | R44 | PASS | 1024, 1100, 1280, 1440 and 1728 all report `scrollWidth == innerWidth`. |
| 32-stage start dialog at the floor | A20 | PASS | 703 px tall in a 768 px viewport with the footer actions visible. |
| Primary run state/actions ahead of secondary detail | A20 | PASS at 1280 · **FAIL at the floor** | Finding 2. |
| Attempt-number badge placement | R44 | **FAIL** | Finding 1 — clipped off the left window edge at every width. |
| Restrained motion, normal mode | A21 | PASS | Route/page `pipeline-route-in` 180 ms; disclosures and dialog panes `pipeline-disclosure-in` 140 ms; chevron 120 ms. |
| Newly appended timeline entry motion | A21 | **FAIL (static evidence)** | Finding 6. |
| Background refetch retains content | A21/R45 | PASS | 50 rows held across a refetch with zero skeleton frames. |
| Reduced motion | A21 | PASS | Every entrance animation and transform transition computes to `none`; route, dialog, disclosure and action sequence remained fully usable and equally legible. |
| Sky & Grove appearance | TS-08 | PASS | All four routes render correctly skinned at the floor with no overflow and no console error. |
| Console cleanliness | J1 class | PASS | Zero console errors, page errors or failed requests on every surface. The only console output was the expected 404 for a deliberately deleted run and template. |

## Findings

Severity counts: **0 BLOCKER, 0 MAJOR, 5 MINOR, 1 POLISH.**

### 1 — Attempt-number badges render off the left edge of the window

```
SEVERITY: MINOR
WHERE: J14 run page (fixture rich, port 4502), any run
REPRO: open /pipelines/runs/{id} at any viewport width
EXPECTED: each attempt's number sits in the timeline gutter beside its entry
OBSERVED: every .pipeline-stage-number is absolutely positioned at x = -4 px, clipped by the
  window edge and visually detached from the timeline it labels
EVIDENCE: run/shots/q-1024-run-detail.png, run/shots/e2-run-page.png
```

`ui/src/styles/features/pipelines.css:305` sets
`.pipeline-timeline-item .pipeline-stage-number { left: calc(var(--ad-space-8) * -1.5) }`. The
positioning context `.pipeline-timeline-item` starts 44 px from the viewport edge and carries a
32 px left padding intended to hold the badge, so the offset overshoots by exactly one and a half
badge widths. Measured `left: -4, right: 28` at 1024, 1280, 1440 and 1728; at the desktop floor all
seven entries of the repair-loop run stack into a column of half-cut numbers against the window
frame. `left: 0` places the 32 px badge centred on the 15 px timeline rule.

### 2 — At the desktop floor the run page puts frozen setup and named values above the timeline

```
SEVERITY: MINOR
WHERE: J14 run page, viewport width 1024–1100
REPRO: open /pipelines/runs/pr_loop00000000001 at 1024x768
EXPECTED: the execution timeline keeps the main reading column; setup and values stay secondary
OBSERVED: the secondary rail renders at y = 550 and the timeline at y = 1125 — the first attempt
  is a full viewport below the fold, behind Frozen setup and Named values
EVIDENCE: run/shots/q-1024-run-detail.png
```

`ui/src/styles/features/pipelines.css:425-429` collapses `.pipeline-run-workspace` to one column at
`max-width: 1100px` and gives `.pipeline-run-rail` `order: -1`, deliberately hoisting the secondary
rail above the primary content. FS-14.R44 makes the timeline the main reading column with setup and
values as secondary disclosures, and permits stacking the secondary rail — but not ahead of the
content the page exists to show. `order: -1` on `.pipeline-stage-navigator` in the same block is
appropriate (a navigator above its form is a normal stacked pattern); only the run rail inverts a
stated hierarchy.

### 3 — The start dialog disables Next without saying what is missing

```
SEVERITY: MINOR
WHERE: J14 start dialog, Setup step
REPRO: Runs → Start run → choose a template with a required named input → fill display name and
  goal → leave the named input empty
EXPECTED: the blocking field is named, per FS-14.A15's "named field error"
OBSERVED: Next is greyed with no message, no marking on the empty required field, and no title or
  aria hint; nothing on screen explains why the primary action is dead
EVIDENCE: run/shots/z2-missing-input.png
```

The client-side gate means the server rejection A15 describes is never reached, so the named field
error it promises is never rendered. A person who fills the two visually obvious fields has to
discover that the separate "Named inputs" block further down is what blocks them. Suggested fix:
mark the empty required input and state the reason next to the disabled control (the blocked-stage
Continue button already does exactly this, with its "New input for the blocked stage" box adjacent).

### 4 — With no templates saved, Start run opens a dialog that cannot proceed

```
SEVERITY: MINOR
WHERE: J14 Runs, fixture empty (port 4503) — the fresh-install state
REPRO: onboarded home with zero templates → Pipelines → Start run
EXPECTED: the action either is unavailable or explains that a template must exist first
OBSERVED: the dialog opens with an empty Template select ("Select a valid template", no options)
  and a permanently disabled Next; nothing points to Templates
EVIDENCE: run/shots/aa-start-no-templates.png
```

Same class as finding 3 and reached earlier: this is what a first-time user meets immediately after
onboarding. The empty Runs state one layer behind already carries the right sentence — "Start a run
to turn a reusable template into supervised work" — which the dialog does not repeat or act on.

### 5 — A deleted run's page shows the raw API error string

```
SEVERITY: POLISH
WHERE: J14, /pipelines/runs/{deleted or unknown id}
REPRO: delete a run, then revisit its URL
EXPECTED: human copy, as the template equivalent already has
OBSERVED: "RUN UNAVAILABLE / This run is gone. / pipeline resource not found / Return to Runs"
EVIDENCE: run/shots/l-deleted-template.png (the template page's correct copy, for contrast)
```

The template page reads "This template is gone. It may have been deleted in another tab. Existing
runs still retain their frozen setup." The run page substitutes the transport-layer message for the
equivalent sentence.

### 6 — A newly appended timeline entry has no continuity transition

```
SEVERITY: MINOR
WHERE: J14 run page, execution timeline
REPRO: inspect any rendered timeline entry's computed style
EXPECTED: FS-14.R45 and A21 name "a newly appended timeline entry" as one of the four continuity
  transitions the surface uses
OBSERVED: .pipeline-timeline-item and its <details> compute animationName: none in both normal and
  reduced-motion modes, and RunBrowser.tsx emits no append-time class; the only entrance animation
  on the surface is the page-level pipeline-route-in, which plays on route mount, not on append
EVIDENCE: measured computed styles, normal and reduced-motion runs
```

**Honesty note:** a live attempt append could not be produced. Advancing a stage requires the stage
agent to report a result through the MCP tool, which the deterministic fake ACP peer does not do,
and the run-detail query is invalidated by `pipeline_update` / `task_update` rather than by window
focus, so a direct fixture insert does not surface. The evidence above is the rendered app's
computed styles plus the component source, not an observed append. The correct resolution may be to
narrow the requirement rather than to add the motion.

## Notes that are not findings

- The `blocked` run state does not exist. Run states are `queued | running | paused | completed |
  stopped`; a blocked *result* pauses the run with `attention_reason = blocked`. A first fixture
  that invented a `blocked` run state made the Continue control look absent; corrected, the R37
  gating is exact. `.pipeline-state-blocked` and `.pipeline-state-crashed` exist in the stylesheet
  for values the run vocabulary never produces — harmless, mentioned only so a future reader does
  not repeat the same fixture mistake.
- Selecting stage 20 of a 32-stage template at 1024 px leaves the stage form barely at the fold
  because the stacked navigator is 825 px tall. Playwright's scroll-into-view on click makes this
  ambiguous to measure honestly, so it is recorded as an observation rather than a finding; it is
  the same stacking cause as finding 2 and a fix there likely resolves it.
- If the `/api/pipeline-proposals` query fails to parse, both tab badges silently read zero and no
  proposal panel renders, with no error surfaced. Reached here only by hand-writing a proposal row
  that the real MCP path could not produce, so it is a risk lead for the §8 code-review path, not a
  usability finding.

## Verification of the review itself

No BLOCKER was recorded, so no blocker replay was required. Findings 1, 2, 3 and 4 were each
reproduced from a reset fixture in a second browser session, and finding 1 was additionally
measured at five viewport widths. Finding 6 is explicitly labelled as static evidence above.
