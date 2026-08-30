# Usability review — the new Pipelines pages and the changed dashboard grid — 2026-08-30

## Scope and setup

- **Scope:** the recently added pages and the UI changes stacked on them — the split Pipelines
  surface (Runs, Templates, the per-run page, the focused template editor), the three fixes made to
  it after the 2026-08-28 run that had never been driven in a browser (attempt badge placement, run
  rail order at the desktop floor, appended-timeline motion), running-first card placement, and the
  expandable dashboard chat panes. Requirements exercised: FS-14.R35–R48 / A14–A26,
  FS-02.R45–R52 / A28–A34, FS-12.A13–A14.
- **Browser rung:** 1 — Playwright driving the environment's cached Chromium (`chromium-1228`),
  capturing console errors, page errors, and failed requests on every run.
- **Build:** `make dist` from the working tree (embed + `sqlite_fts5` binary at `bin/agentdeck`).
  The embed step produced no tracked change.
- **Fixtures:** three isolated `AGENTDECK_HOME` copies under `.review/usability-20260830-newpages/`,
  one port each. `pipe` (4601) is an onboarded home with one saved template and no runs; `rich`
  (4602) is a freshly re-seeded home with a seven-attempt repair-loop run, a paused/blocked run, a
  paused/restart-recovery run, 121 retained runs, 26 delegated tasks on one attempt, a
  32-stage/64-declaration template, and one pending proposal of each kind; `dash` (4603) has no
  templates and six real agents (five chat, one terminal) launched through the API, three of them
  running, with a manual layout order written before the run. The deterministic `fakeacp` peer was
  exposed as `claude-agent-acp` / `codex-acp` through a PATH shim.
- **Evidence:** screenshots under `.review/usability-20260830-newpages/run/shots/`; the load-bearing
  ones are copied to
  [`usability-review-2026-08-30-new-pages-evidence/`](usability-review-2026-08-30-new-pages-evidence).

## Journey results — J14, the new Pipelines pages

| Step | Requirement | Result | Observation |
|---|---|---|---|
| `/pipelines` lands on Runs | A14 | PASS | Redirects to `/pipelines/runs`; run list mounted, no editor. |
| Templates shows no run history | A14 | PASS | Zero ledger rows, two template rows, no start form. |
| Runs shows no editor | A14 | PASS | Zero editor nodes and zero template rows after switching back. |
| Attempt badge sits on the timeline rule | R43 (fix) | PASS | Badge centre `x=60` equals the rule centre `x=60` at 1280 and 1024; badge stays inside the list (`left 44 ≥ list left 32`) and never overlaps the disclosure (`right 76 = details left 76`). |
| Run rail stacks after the timeline at the floor | R43 (fix) | PASS | 1280: side by side (`timeline left 32`, `rail left 907`, same top). 1024: `rail top 1703` below `timeline top 550`, both at left 32. |
| Template stage navigator hoists above the form | R43 | PASS | 1024: navigator top 614 above stage form top 845 (1280: both 633, side by side). |
| Focused editor mounts one stage form | A20 | PASS | 32-stage template at 1024: 32 navigator entries, exactly 1 stage form, no horizontal overflow. |
| Appended timeline entry plays one entrance | A21 (fix) | PASS | Continue/Retry appended entry 2 with `pipeline-timeline-appended`; entry 1 never gained it, and a later background refetch neither replayed nor re-marked anything. |
| Reduced motion removes the entrance | A21 | PASS | Computed `animation-name` on the appended entry, run page, and route: `pipeline-disclosure-in` / `pipeline-route-in` normally, `none` under `prefers-reduced-motion: reduce`; chevron transition `transform` → `none`. |
| Bounded run history reaches an exact end | A22 | PASS | 50 → 100 → 122 of 122, no duplicate rows, "Complete history loaded" replaces **More runs**. |
| Start run refused with no valid template | A24 | PASS | **Start run** disabled beside "No template is ready to run yet. Create one in Templates." linking to `/pipelines/templates`; no dialog opens. |
| Disabled step control names what it waits for | A24 | PASS | **Next** disabled beside "Select a template to continue." |
| Deleted run / template deep links | A19 | PASS | "This run is gone." / "This template is gone." with **Return to Runs** / **Return to Templates**. |
| Legacy `?run=` link resolves | A19 | PASS | `/pipelines?run=pr_blocked000001` → `/pipelines/runs/pr_blocked000001`. |
| Restart pause withholds a dead-end chat | A26 | PASS | Restart-recovery pause: **Retry stage**, **Stop run**, no **Open agent**, plus the sentence naming why. Ordinary blocked pause: **Open agent**, **Continue** (disabled until input), **Retry stage**, **Stop run**. |
| Pending proposals counted on both destinations | A18 (partial) | PASS | Runs and Templates each carry a `1` count. Consumption on approval was not exercised. |
| Console/page errors across the Pipelines pages | — | PASS | Zero on every route driven. |

## Journey results — J5, the changed dashboard grid

| Step | Requirement | Result | Observation |
|---|---|---|---|
| Running cards precede stopped ones | A28 | PASS | Manual order 1,2,3,4,5 with 1 and 5 running renders 1, 5, 2, 3, 4 — manual order kept inside each block. |
| Live start/stop crosses the boundary without reload | A28 | PASS | Resume moves Agent 5 into the running block; stop returns it to its manual slot at the end. No reload, no scroll change, other cards fixed. |
| In-drag preview stays inside one block | A28 | PASS | Dragging Agent 4 over Agent 2 moved only stopped-block cards; the running card held `32,292` throughout. |
| Same-block order persists | A28 | PASS | Committed order written to `/api/layout` and identical after reload. |
| Cross-block drop refused | A28 | PASS | Displayed order, persisted order, and post-reload order all unchanged; the other block never moved (the 2 px shift on the hovered card is the ordinary `:hover` lift). |
| Four-pane cap | A31 | PASS | Expanding a fifth pane leaves exactly four, with no confirmation dialog. |
| Least-recently-used pane is the one evicted | A31 | PASS | Touching a pane protects it; the untouched oldest pane is the one that collapses. |
| Evicted pane restores its unsent draft | A31 | PASS | "unsent draft in the evicted pane" returned verbatim on re-expand. |
| Expanded panes persist across reload | A32 | PASS | Server layout `expanded` matched the four open panes before and after reload. |
| Terminal card navigates instead of expanding | A29 | PASS | Click on the terminal agent's card → `/agent/a_0db92b`. |
| In-pane gestures do not collapse the pane | A34 | PASS | Composer click and transcript right-click left the pane expanded and opened no card menu. |
| Pane name link navigates without toggling | A34 | PASS | → `/agent/a_48026a`; the pane was still expanded on return. |

## Findings

```
SEVERITY: MAJOR
WHERE: J5, dashboard grid (fixture dash/, port 4603) — ui/src/components/grid/CardGrid.tsx:61
REPRO: fail one GET /api/layout at page load, then expand a card / reorder / change density
EXPECTED: the read failure is surfaced, and later layout changes still persist
OBSERVED: uncaught page error, no message to the user, the saved order and open panes vanish from
  the display, and every later layout change issues no PUT at all for the rest of the session
  (control run with a healthy read issues the PUTs and persists)
EVIDENCE: n21-failed-get.png, n21-control.png
```

```
SEVERITY: MAJOR
WHERE: J14, run page (fixture rich/, port 4602) — ui/src/features/pipelines/RunBrowser.tsx:116,137
REPRO: pause a run with a stage-agent launch or resume failure, open the run page, click Open agent
EXPECTED: R48's principle — a pause no chat can resolve does not offer a chat
OBSERVED: attention `resume_failed` (equally `launch_failed`) keeps Open agent, which lands on the
  stopped stage agent's chat and composer; only `restart_*` reasons withhold it
EVIDENCE: n16-open-agent-after-resume-failed.png
```

```
SEVERITY: MINOR
WHERE: J14, run page (fixture rich/, port 4602) — ui/src/styles/features/pipelines.css
REPRO: open a run whose attempt state is launch_failed / resume_failed / restart_recovery
EXPECTED: a failure state reads at higher salience than an ordinary one (FS-12.A13)
OBSERVED: no `.pipeline-state-<reason>` rule exists for those states, so "resume failed" renders in
  the neutral grey badge (`rgb(105,126,123)`) while paused/blocked render in the accent colour
EVIDENCE: n8-resume-failed-badge.png
```

```
SEVERITY: MINOR
WHERE: J5, dashboard grid (fixture dash/, port 4603) — ui/src/components/grid/CardGrid.tsx:146
REPRO: drag a running card across the running/stopped boundary and keep the pointer moving
EXPECTED: some statement that the drop is refused
OBSERVED: the card follows the pointer, then snaps back to its home slot mid-drag with no message;
  the refusal itself is correct. No requirement covers in-drag feedback for the refused drop.
EVIDENCE: n12-crossblock-indrag.png
```

## Coverage gaps recorded (USABILITY-REVIEW §7)

The J5 and J14 charters do not tell a reviewer to exercise: the four-pane cap, its least-recently-used
eviction, or the evicted pane's draft (A31); expanded-pane persistence and its unknown-agent and
cross-project id cases (A32); the terminal-card exemption (A29); the pane name link (A34); the
cross-destination proposal count and its consumption on approval (A18); deleted run/template links
(A19); the start-run refusal and the named awaited value (A24); the stage-boundary wording in the
assignment, accepted result, and refusal (A25); and the restart-pause versus blocked-pause action
difference (A26). Every one of those except A18's consumption, A32's id edge cases, and A25 was
driven in this run.

## Not exercised

- A25 (stage-boundary wording) needs a live stage assignment from a real report cycle; `fakeacp` has
  no scenario that calls the stage-result tool, so no assignment text was rendered.
- A16/A17 (repair-loop timeline entries, delegated agent cards) were left as they passed on
  2026-08-28; nothing in the range since then touched them.
- The untagged no-FTS5 build was not driven; nothing in scope touches archive search.

## Notes (not findings)

- `ui/src/styles/features/pipelines.css` ends with a stray extra `}` on line 458. It is the last
  line, so no rule is lost and nothing is user-visible.
