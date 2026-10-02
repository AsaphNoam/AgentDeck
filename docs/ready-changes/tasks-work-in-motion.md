# Tasks as project-grouped work in motion

**State:** Waiting to start
**Why:** Direct `/design-feature` request, 2026-10-02; user confirmed connected task rows grouped
first by project, then recorded relationships, with creator as supporting context.
**Relevant requirements:** FS-16.R41–R45/A27–A29, FS-16.R3/R14/R22–R24/R33–R40,
FS-12.R49/R52/R55, TS-08.R82–R85, TS-10.R11/R22/R25–R35,
INV §1, §2, §7, §8, §10, §11, §13, §16, §17.

## Outcome

An experienced operator opens Tasks to see what is running, why work waits, what recorded work can
run next, and where intervention is needed across projects. Human task creation is exceptional and
collapsed at the bottom. Completed work supplies context without dominating the page.

## Included work

- All-project default and existing project deep links; compact project sections and relationship
  groups with typed dependency/delegation links, branches, joins and creator/assignee metadata.
- Active groups retain completed context; entirely finished, settled groups enter collapsed
  project history. Waiting, continuation and cleanup states stay accurate, including during updates.
- Inline task detail with existing results, links and eligible controls; preserve pipeline ownership
  and draft/error behavior. Bottom manual creation and secondary signal firing use explicit projects.
- Parse existing server fields currently omitted by the UI; reuse existing task/run queries,
  invalidate through the current SSE/cache paths, and bound concurrent project reads.

Exclude execution/scheduler changes, new APIs, database migrations, authority or retention changes,
creator-grouping modes, graph editing, predicted future tasks, a graph canvas, new dependencies or
skins, phone changes, and unrelated frontend refactors.

## Verified seams and boundaries

- `internal/server/task_handlers.go:handleTasks` requires a project. Use the existing project
  catalog and project task queries; do not invent an all-project endpoint. HTTP task lists currently
  return a project's full retained set (`internal/state/tasks.go:ListTasks`); TS-10.R33's bounded
  agent inspection surface is not an HTTP pagination implementation. This design does not fix or
  expand that existing list contract. Project requests are capped at four in flight, and graph
  projection is snapshot-based without recursive fetching or persistent duplicate history.
- `state.Task` and `hydrateTask` already expose parent/run/stage lineage, outputs, waiting flags,
  cleanup, and timestamps. `ui/src/schemas/task.ts` omits these and even rejects `waiting`; close
  that frontend contract gap as part of the view. No new durable state is needed.
- Lineage is provenance (TS-10.R25), not an arm. Combined delegation and dependency links cannot
  be assumed acyclic. Missing/deleted task details have no named tombstone in the list response;
  show unavailable references rather than reconstructing history or requesting every ancestor.
- Creator names are absent from task JSON; use available agent identity and a stable-id fallback.
  Run detail already exposes `stage_tasks[].task_id` for distinguishing authoritative stage work
  from ordinary descendants when task controls are opened. A run id alone is insufficient.
- `ui/src/api/sse.ts` already invalidates the task query family on task updates and reconnect;
  preserve that shared path. Reuse existing forms and their project-specific mutation hooks.

## Design and experience direction

Reading order: project and compact work summary → related task rows → inline selected detail →
collapsed project history → bottom authoring disclosures. Task title/state and the immediate wait
reason lead; completed rows, creator and runtime metadata stay quiet. Shallow connectors and named
references show branches and joins; do not flatten them into a misleading sequential checklist or
duplicate tasks. Real task lineage and explicit outcomes make this an AgentDeck work view.

The primary journey is scan a project, locate current work, follow its actual predecessor/successor
conditions, then open the agent only when useful. The consequential branch is inspect attention,
understand the failed prerequisite or interruption, and use the existing valid repair without losing
chain context. Preserve task focus and drafts when live updates settle or regroup work. No new
motion; immediate state changes and static connector structure keep the reading position stable.

Incumbent source inspection confirms top-of-page Create/Signal forms and a flat project list.
The existing browser tab at `http://localhost:5173/tasks` returned HTTP 504 on 2026-10-02, so
incumbent visual inspection and redesigned rendered acceptance remain implementation gates.

## How we will know it works

- FS-16.A27: two projects, A → B → C, branching/joining and delegation; exact links, distinct
  creator/assignee, missing references and no accidental grouping by creator/signal/run.
- FS-16.A28: wait/resume, interruption, prerequisite failure, terminal cleanup and completed history;
  valid recovery, pipeline restrictions, stable open detail and preserved rejected drafts.
- FS-16.A29: all-project/focused entry, empty/error/stale/reconnect, archived or unavailable identity,
  long chains, bottom disclosures and concrete project submission. Use Go-shaped fixtures rather
  than defining expected wire data from the frontend's own schema.
- Render the actual incumbent and resulting Tasks route at 1024px and wider in all three skins with
  representative states, keyboard navigation and long names. Record evidence; source inspection
  alone is not acceptance. Run focused component/projection checks, then the applicable TS-06 and
  workflow §2 closure matrix, style checks and generated embed once after final implementation.

## Waiting on

Nothing. Feature scope confirmed; no technical tradeoff requires a new product or protocol decision.
