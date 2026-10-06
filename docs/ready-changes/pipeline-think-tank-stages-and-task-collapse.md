# Think Tank pipeline stages, readable workspace consent and collapsible tasks

**State:** Waiting to start
**Why:** Direct human request on 2026-10-06; proposed scope confirmed in the same conversation.
**Relevant requirements:** FS-14.R81–R86/A48–A52; FS-21.R41–R42/A31–A32;
FS-16.R46–R48/A30–A32; TS-09.R51–R56; TS-14.R19–R21; TS-10.R38; TS-08.R92–R95;
INV §1–§5, §7–§11, §13–§17.

## Outcome

A pipeline can deliberate through fresh same-project Think Tank participants and pass the judge's
published synthesis directly to the next stage. The workspace acknowledgement stays readable, and
Tasks can show a stage or nested parent without every delegated task beneath it.

## Included work

- Model-neutral Think Tank template slots and run runtime assignments; required fresh end-only judge,
  one synthesis output, room-backed stage task, exact-once result acceptance and existing human gate.
- Durable stage/room binding and context; explicit room recovery, acceptance-only retry, scoped Stop,
  retained history and run pinning. Preserve ordinary templates/runs and standalone room behavior.
- Shared-workspace warning contrast in modal/inline start and all appearances, with existing consent.
- Separate parent-lineage collapse at every Tasks level, session choices, counts/attention and
  reachable cross-branch dependency links, preserving detail drafts and active interactions.
- Ship API/CLI/proposal, editor/start, run/task/room, phone start assignments, fixtures and operator
  knowledge together. Room inspection/recovery remains desktop-only.

No existing/cross-project agents in pipeline rooms, parallel pipeline branches, automatic quality
judgment, provider bump, new room tools, phone room workspace, destructive reset or general graph UI.

## How we will know it works

FS-14.A48–A52 cover mixed-stage progression, exact output, failure/restart/Stop races, retained history,
output-acceptance replay and real rendered warning contrast. FS-21.A31–A32 protect standalone versus
owned-room boundaries. FS-16.A30–A32 cover nested collapse, cross-branch reachability and managed task
authority. Use serialized producer-derived fixtures and state/fault/race tests; final TS-06 closure
checks once after the last relevant edit. Render the actual UI at 1024px and wider in Core, Sky &
Grove and Studio. Record the bounded packaged Claude/Codex probe separately from fake-provider proof.

## Design direction

Experienced operators create pipelines and scan active stages. Keep goal, phase, attention and the
useful next action first. A Think Tank stage opens the existing room workspace; its judge contribution
is the concrete source of the output, rather than another coordinator evaluation. In Tasks, keep the
parent's title/state visible with a quiet child disclosure and counted attention beneath it. Retain
existing dense rows, capped indentation, independent detail disclosure and semantic tokens. All
collapse updates are instant; add no motion. Exercise setup, discussion/private wait, pause/failure,
judge, output acceptance, human gate, Stop and retained states. Warn about shared workspace through
readable paired colors rather than a new dialog or consent step.

## Verified seams and implementation cautions

Read-only code investigation confirmed `Stage`/canonical validation in `internal/pipeline/{types,
validate}.go`, stage creation/advance in `{manager,actions}.go`, and atomic result/closure writes in
`internal/state/pipeline_tasks.go`. `CreateThinkTank` reserves setup identities; the finalization path
in `internal/state/think_tank_turns.go` publishes synthesis before judge completion is visible. Reuse
these transaction helpers and `server/think_tanks.go`'s launch/resume/dispatch, plus the existing guarded
turn cancellation. No provider limitation or workaround is needed.

`StageOutput` has no `required` field: exactly one output is mandatory by this stage kind's validator.
Repeated participant roles are allowed. Do not add a second stage report/result store or dispatch a
standing-agent task for room work. The room's 64 KiB reply limit and pipeline's 64,000-rune value limit
both apply before judge staging. Stage context has its own bounded room record/read path rather than
being squeezed into the smaller ordinary room goal. Collapse uses `parent_task_id`, never `depth` or
prerequisites; the existing projection in `ui/src/features/tasks/taskWork.ts` remains authoritative.
The warning's inherited text on a technical background (`pipelines.css`) is a source hypothesis,
not rendered proof; reproduce and verify it in implementation.

## Waiting on

None. All requirements remain planned; product implementation has not started.
