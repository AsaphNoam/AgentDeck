# Project dashboard and project-grouped archive

**State:** Waiting to start
**Why:** the human requested a project-first dashboard with project-grouped agent archive
**Relevant requirements:** FS-01.R31/A15, FS-02.R29–R36/A15–A20, FS-04.R35–R36/A15–A16,
FS-05.R32–R36/A16–A19, FS-14.R32/A12, TS-01.R13, TS-02.R20, TS-03.R18/R20, TS-09.R25,
INV §1, §2, §3, §4, §5, §8, §10, §11

## Outcome

The dashboard opens on active project cards. Each project opens a scoped agent dashboard; stopped
agents remain visible and resumable there. Agent Archive is grouped under projects, including an
active-project indicator and archived-agent count. Project and agent archive/restore work as the
confirmed lifecycle actions.

## Included work

Persist project and agent archive state; add the archive/restore REST actions and independently paged
group/agent Archive queries; preserve all-session search and active filtering; publish archive state
through existing agent updates; add project-card context actions and routes; gate every process-start
path during archival; stop affected pipelines before project archival; and update UI, tests, journeys,
and embedded frontend output through the normal source build. When the replacement ships, retire
FS-05.R1–R3/R10/R28 and A1/A3/A10/A12 as superseded by R36/A19; retain R4/A2's non-null guarantee.

Intentionally excluded: per-project layout persistence or migration, project deletion changes,
cross-project agent moves, and a new SSE channel or client state authority.

## How we will know it works

FS-02.A15–A20 cover project navigation, unavailable/archived routes, context actions, archive
grouping, and shared layouts. FS-04.A15–A16 cover persistence and dormant archived defaults.
FS-05.A16–A19 cover atomic containment, restore-before-Resume, grouped pagination, and preservation
of all-session search. FS-14.A12 covers pipeline containment. Run J2, J5, J7, J8, J12, and J14 plus
the normal specification, Go, UI, source, and distribution checks.

## Waiting on

Nothing. All product and technical decisions are resolved.
