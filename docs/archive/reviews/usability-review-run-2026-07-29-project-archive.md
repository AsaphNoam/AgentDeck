# Usability Review Run — 2026-07-29 project dashboard and grouped Archive

**Scope:** browser and real-binary review of the finished project-dashboard and project-grouped
Archive change at `852ad9a`. The change names J2, J5, J7, J8, J12, and J14 as its acceptance
journeys. This run used the release-style `sqlite_fts5` binary, a copied isolated J5 home, and the
repository fake ACP peer; no real provider session or credential was used.

**Review surface:** browser ladder rung 1 until a native `confirm()` dialog stopped all input and
then all subsequent tab operations. Completed browser work has DOM, screenshot, and console evidence
under `.review/usability-20260729-project-archive/evidence/`. The remaining browser steps are
explicitly blocked, not inferred from supporting tests. Loopback API observations and focused
regressions were used only to check backing state after the browser block. Product code and
specifications were not changed.

## Executive summary

1. No new usability finding was confirmed in the completed browser paths.
2. The project home rendered configured empty and populated project cards; scoped dashboards showed
   only their project's agents, including state summaries and visibly labelled zero context usage.
3. A stopped agent exposed Resume, resumed to idle, then no longer offered Resume. Individual
   archive stopped and removed only that agent; grouped Archive showed it beneath an active project,
   its transcript exposed Restore, and Restore returned it stopped to its project dashboard.
4. The project archive warning invoked the browser's native confirmation dialog, but the in-app
   browser timed out before it could dispatch a confirmation response. It then could not create or
   use another tab. Project archive/restore containment and restart persistence were verified at the
   loopback API boundary, not as completed browser journeys.
5. Focused server/pipeline regressions, 44 relevant UI tests, style/presentation checks, the release
   binary build, and the specification check passed.

## Journey results

### J2 — Onboarding and archived-project eligibility

**BLOCKED(browser automation after the native confirmation dialog).** The archived-default selector
and onboarding presentation could not be rendered in this run. Supporting component tests passed;
this is not browser evidence.

### J5 — Project dashboard, scoped dashboard, and actions

| Step | Result | Evidence |
|---|---|---|
| Project-card home renders configured empty and populated projects | **PASS** | `evidence/J5-project-home.png`; zero console errors |
| Selecting the populated project renders only its three agents, project heading, state summaries, and `0% context used` labels | **PASS** | `evidence/J5-scoped-stopped-agents.png` |
| A stopped card shows Resume; after resume the card becomes idle and its menu omits Resume | **PASS** | browser DOM before and after action |
| Archiving one running agent removes only that agent from the active project dashboard | **PASS** | browser DOM after action |
| Project-card actions render Rename, Change color, and Archive | **PASS** | browser DOM |
| Confirming project Archive in the browser | **BLOCKED(native dialog input timeout)** | dialog appeared after Archive; no completed browser action |

### J7 — Stop, resume, and switch

**PARTIAL PASS.** The stopped-card Resume path completed in J5 with the fake ACP peer: the agent
returned as idle using the same durable id. Switch was not driven after the browser block.

### J8 — Grouped Archive and restore

| Step | Result | Evidence |
|---|---|---|
| An individually archived agent appears under its active project with archive count and transcript metadata | **PASS** | `evidence/J8-grouped-active-project.png` |
| Archived transcript is read-only and offers Restore rather than Resume | **PASS** | `evidence/J8-agent-restore.png` |
| Restoring that agent returns it stopped to the active project dashboard | **PASS** | browser URL and DOM after Restore |
| Project archive/restore keeps all agents archived until individual restore, and Resume under an archived project returns `project_archived` | **PASS (API evidence)** | isolated loopback responses recorded during run |
| Browser paging across project groups and per-project rows | **BLOCKED(browser automation)** | focused API/UI regressions pass but are not browser evidence |

### J12 — Restart durability

**PARTIAL PASS (API evidence).** After restarting the isolated server, the active project, its two
remaining archived agents, restored stopped agent, and persisted shared layout were unchanged. The
post-restart browser render was blocked by the native-dialog limitation.

### J14 — Pipeline containment

**BLOCKED(browser automation).** The active-pipeline project-archive interaction was not rendered.
`TestStartRejectsProjectArchiveClaimBeforeDurableMutation` passed as supporting evidence only.

## Findings

No confirmed Must-fix or Worth-fixing usability finding.

## Static sweeps and supporting verification

- **S2 CSS wiring:** PASS — stylelint and the presentation contract/audit suite passed (25 checks).
- **S1/S3/S4/S5:** limited read-only scans produced no confirmed usability finding; the browser block
  prevented promotion or dismissal through the affected rendered journeys.
- Release-style `make build`: PASS (the Go module-cache warning is a sandbox permission diagnostic,
  not a build failure).
- Focused tagged Go checks: PASS — archive action lists, >200-session grouping, archive transition
  claims, and pipeline archive/start containment.
- Relevant UI checks: PASS — 8 files / 44 tests covering project cards, grouped Archive, archive
  transcript controls, archive eligibility, launch selectors, layout, and card menus.
- `make check-specs`: PASS.

## Coverage and next run

Run the remaining browser paths with a browser capable of accepting native confirmation dialogs:
J2 archived-default eligibility, J5 confirmed project archive and page reload, J7 switch, J8
two-level paging, J12 post-restart render, and J14 active-pipeline project archive. Credentialed
provider and terminal gates remain separate and were not attempted.
