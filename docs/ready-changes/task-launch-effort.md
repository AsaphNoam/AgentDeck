# Let a task choose its agent's reasoning effort

**State:** Waiting to start
**Why:** Direct request on 2026-09-04 — "Create task has no effort argument."
**Relevant requirements:** FS-16.R2, R20, R25, R27, R28, A18; FS-09.R41, R42, R49, A22; TS-10.R23, §3; INV §2

## Outcome

A task that will launch its own agent can say what reasoning effort that agent should run at. Today
`create_task`, `POST /api/tasks`, and the Tasks create form accept `role`, `backend`, and `model`
but no effort, so every task-launched agent falls through to the model's `default_effort` or the
CLI's own level — even though New Agent, resume, switch runtime, and pipeline run start all offer
the choice. An agent scheduling later work can now ask for a cheap model at a low level or a hard
problem at a high one.

The same change stops a bad launch specification from being discovered three start attempts too
late: an unknown backend, an unknown model, or an undeclared effort is rejected when the task is
created, with the field named, instead of being accepted and parked as `dependency_failed`.

## Included work

Included:

- An optional `effort` on the durable task row (one forward-only migration, nullable column), on the
  `create_task` tool arguments, on the `POST /api/tasks` body, and as a field beside Backend and
  Model in the Tasks create form.
- Passing the stored value through as `launchRequest.Effort` on the call
  `internal/server/task_dispatcher.go` already makes, so `resolveEffort` and
  `config.ValidateModelEffort` remain the only precedence and validation code (TS-10.R23, INV §2).
- One shared helper that resolves a launch specification to a concrete backend/model/effort triple,
  called by both authoring paths for creation-time validation and by launch composition.
- Rejecting an effort supplied together with an existing-agent `to` target, as a typed `validation`
  error.
- Creation-time validation of `backend` and `model`, which are passed through unchecked today
  (`task_handlers.go:154` and `:591` validate only the role).
- When it ships: flip the `(planned)` tags, extend FS-16.R2's launch-specification parenthetical to
  name effort, and return FS-16 and TS-10 to **Current**.

Not included:

- Editing a task's effort after creation; a task's execution target is written once, like its
  assignee.
- Any effort override on a task that targets an existing agent, or any mid-task effort change.
- Changing the silently-ignored behavior of `role`/`backend`/`model` when `to` names an existing
  agent — only `effort` is rejected there. Revisiting the other three is a separate compatibility
  call.
- Replacing the Tasks form's free-text Backend/Model inputs with catalog-backed selects.
- Effort on arms or attachments.

## How we will know it works

- **FS-16.A18** — effort round-trips from both authoring paths onto the composed launch spec, beats
  a bound source's effort override, and resolves to `default_effort` when omitted; undeclared
  effort, unknown model, unknown backend, and effort-with-`to` are each rejected at creation with a
  typed field-named error that creates no task; a model whose levels are edited after creation fails
  the start attempt rather than launching at a substituted level. MCP, HTTP, and task-dispatch tests
  plus a Tasks-view create test.
- **FS-09.A22** — the creation-time rejection uses the same field-level error and levels list a
  launch returns, and the start-time check still runs.
- Existing **FS-16.A16** must stay green: a valid specification that becomes invalid before start
  spends a start attempt exactly as today, and deferrals still spend none.

## Waiting on

Nothing.
