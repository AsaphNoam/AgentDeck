# FS-16 — Dependent work and armed starts

**Status:** Partial
**Code:** `internal/state`, `internal/server`, `internal/messaging`, `ui/src/features/tasks` · **Journeys:** —
**Absorbed:** —

## 1. Purpose

AgentDeck understands dependencies between pieces of work natively. A piece of work can exist in a
durable "ready to run once these prerequisites are satisfied" state, and AgentDeck starts it when
those prerequisites are satisfied. No model polls another agent, waits, relays status, or sends a
conversational "I'm done" purely to advance orchestration. Dependencies attach to durable work and
its explicit reported outcome, never to an agent's lifecycle status.

Every requirement and acceptance item in this specification is `(planned)`; none has shipped.

## 2. Behavior

Requirements are user- and agent/API-observable. R-item numbering is continuous through §4.

### 2.1 Durable work and explicit outcomes

- **R1** `(planned)` — **A task is durable work with an explicit outcome.** A task is a durable,
  project-scoped record with a stable opaque id, a display name, an instruction, an optional set of
  prerequisites, a state, and — once finished — one outcome. It outlives the agent that executes it,
  survives restart, and is not deleted when its agent stops, crashes, or is archived. A task holds no
  copied transcript, no provider session, and no context payload.
- **R2** `(planned)` — **A task names how it will be executed.** A task targets either one existing
  chat agent or a launch specification (role, project, backend, model) from which AgentDeck creates a
  new agent when the task starts. A task has at most one assigned agent at a time, and an agent has at
  most one active task at a time. Terminal-interface agents cannot execute tasks, matching FS-07's
  messaging boundary.
- **R3** `(planned)` — **Outcomes are reported, never inferred.** A finished task carries exactly one
  outcome from the closed vocabulary `success`, `failure`, `blocked`, or `cancelled`. The assigned
  agent or a person may record `success`, `failure`, or `blocked`; `cancelled` is written only by
  AgentDeck when the task is cancelled or its agent's work is abandoned. An agent becoming `idle`,
  `done`, or `error` (FS-01.R17–R20) never sets, implies, or clears a task outcome, and no unread
  count, status transition, sweep, or restart records one. The vocabulary is deliberately identical to
  the pipeline stage-result vocabulary (FS-14.R19) so one result concept serves both.
- **R4** `(planned)` — **Finishing releases the task's claim on its runtime, and stops only the
  runtime the task itself brought up.** A task records at start time whether it created the runtime by
  launching a new agent, woke it by resuming a stopped one, or merely borrowed a runtime that was
  already up for its own reasons. When the task finishes it always releases its claim. AgentDeck then
  stops the agent through the shared stop seam (FS-01.R6, R34) only in the first two cases, returning
  it to the state the task found it in. A borrowed runtime is left alone, so completing a task never
  kills a conversation a person is in the middle of. A stopped agent becomes a stopped, non-archived
  agent (FS-01.R31): its card, identity, and transcript stay visible and it resumes like any other
  stopped conversation. Nothing is ever archived or deleted by finishing a task.

### 2.2 Arming, prerequisites, and starting

- **R5** `(planned)` — **Prerequisites are a conjunction of typed arms.** A task carries zero or more
  arms, and every arm must be satisfied before the task becomes ready. There is no OR, quorum,
  negation, expression language, or workflow DSL. An arm is one of:
  (a) **task** — a prerequisite task id plus the non-empty set of its outcomes that satisfy the arm;
  (b) **pipeline run** — a pipeline run id plus the satisfying set over that run's registered terminal
  outcome (R13); (c) **signal** — a project-scoped signal name, satisfied when that signal is fired
  (R9). A task with zero arms is ready as soon as it is created.
- **R6** `(planned)` — **Ready work starts without a model in the loop.** AgentDeck observes its own
  durable records, and when a task's last arm is satisfied it starts that task itself. A task
  targeting a launch specification is started by launching its agent with the task's instruction as
  the intentional assignment input permitted by FS-00.R14–R15. A task targeting an existing agent
  crosses into that agent's existing conversation through a host-owned activation (FS-00.R15,
  TS-01.R19); a running agent is given one bounded turn and a stopped agent is resumed first on the
  same wake terms as mail (FS-01.R33). A task is `running` only once its assignment has crossed into
  a confirmed runtime; until then it is `starting` and its start is still owned by AgentDeck, not by
  the agent. No agent is asked to poll, wait, check whether a prerequisite finished, or announce its
  own completion to release other work.
- **R7** `(planned)` — **Readiness is logical; running processes are budgeted.** Every task whose
  arms are satisfied becomes ready, however many that is: a graph that fans out to fifteen tasks has
  fifteen ready tasks, and nothing about the dependency model is narrowed to fit a machine. Physical
  concurrency is separate. A configurable install-wide budget, default four, limits how many agent
  runtimes AgentDeck itself brings up for tasks at one time. Ready tasks are admitted in the order
  they became ready as capacity frees, and a task whose start borrows a runtime that is already up
  (R4) consumes no budget because it creates no process. A ready task waiting for capacity is
  presented as ready and waiting, not as failed or blocked. The existing exclusive per-agent lifecycle
  transition (FS-01.R34) still serializes two operations aimed at the same agent.
- **R21** `(planned)` — **The budget is visible and adjustable.** The concurrency budget is a single
  install-wide setting with a default of four, readable and writable through settings (FS-04.R43). It
  counts only runtimes this feature created or woke and still holds a claim on, so agents a person
  launched or resumed themselves are never counted and never blocked. Lowering the budget below the
  number of currently running task runtimes stops nothing; those runtimes finish and release normally
  and no new one starts until the count is back under the budget.
- **R8** `(planned)` — **A prerequisite that can never be satisfied parks the dependent.** When a
  prerequisite reaches a terminal outcome outside its arm's satisfying set, the dependent task moves
  to the durable `dependency_failed` state, records which arm failed and why, and is presented as
  needing attention. The same happens when a prerequisite task itself reaches `dependency_failed`, or
  when a prerequisite task or pipeline run is deleted. A parked task never starts on its own; a person
  may re-arm, retry, or cancel it. Nothing is silently cancelled, silently dropped, or left waiting on
  an outcome that can never arrive.
- **R9** `(planned)` — **Signals are named releases, not stored objects.** A person or an API client
  fires a project-scoped signal by name. Firing satisfies every signal arm waiting on that name at
  that moment and is recorded on the arms it satisfied. A signal is not a durable object with its own
  identity, CRUD surface, history, or retention: an arm may name a signal that has never been fired,
  and firing a name no arm is waiting on succeeds and changes nothing. Signals exist so work can be
  armed on something AgentDeck cannot observe, such as a CI result or an external approval.

### 2.3 Assignment context

- **R10** `(planned)` — **A task carries its own context attachments.** A task may attach canonical
  context reference ids (FS-15.R1) with an attachment-specific label and description (FS-15.R3). The
  attachment belongs to the task, not to the reference: attaching does not copy content, does not
  duplicate the reference, and does not create or modify a direct grant. The assigned agent may read
  an attached reference through a work-derived route for as long as it is that task's assignee.
  A terminal outcome does not by itself revoke that route; deleting the task removes only that task's
  attachments and work-derived route, leaving the canonical reference and every direct grant intact.
- **R11** `(planned)` — **An assigned agent receives its assignment directly.** An agent started for a
  task can read that task's instruction and its attached reference ids with presentation metadata in
  one bounded call. It never scans the global direct-share list to work out which context belongs to
  its current work, and personal hidden/visible state on that list (FS-15.R6) cannot detach or revoke
  context an active task requires.

### 2.4 Authoring, control, and presentation

- **R12** `(planned)` — **People and agents both create and control work.** Tasks, arms, and
  attachments are created over the local HTTP API and its UI, and by a token-bound chat agent through
  scoped MCP tools that create a task, read its own assignment, report its result, and cancel a task
  it created. Caller identity is always server-derived (TS-04.R7); no tool argument names another
  agent as the reporter. This deliberately opens what FS-14 keeps closed for pipelines: an agent can
  cause new work to start without a person in the loop, which is the point of expressing orchestration
  as control state rather than prose.
- **R13** `(planned)` — **Pipeline results register in the shared result layer.** When a pipeline run
  reaches a terminal state, AgentDeck registers its outcome in the same result vocabulary as R3:
  `success` when the run completes with the final outcome `success`, `failure` when it completes with
  any other template-defined final outcome, and `cancelled` when the run was stopped. The raw
  template-defined label is retained as a display detail. That registration is what makes a pipeline
  run usable as a prerequisite (R5b). Pipeline runs keep their own template routing, revisits, and
  recovery; this requirement adds no cross-run join, parallel branch, or child pipeline to FS-14.
- **R14** `(planned)` — **Dependent work has its own view.** A Tasks view lists tasks for a project
  with their state, outcome, what each armed task is waiting on, and which parked task needs
  attention, and it offers create, re-arm, retry, cancel, and delete. The dashboard is unchanged apart
  from an indicator of how many tasks need attention (FS-02.R44).

## 3. States & transitions

- **Task:** `armed` (at least one arm unsatisfied) → `ready` (every arm satisfied; eligible, and
  waiting for capacity if the budget is full) → `starting` (capacity granted and a start attempt is
  under way, naming the agent and generation it targets) → `running` (the assignment has crossed into
  a confirmed runtime) → `finished` (one outcome recorded). `running` means the agent is genuinely up
  and holding the assignment; a task whose start effect was never confirmed is `starting`, never
  `running`. `starting` → `ready` when a start attempt is abandoned without confirmation and may be
  retried. `armed` or `ready` → `dependency_failed` when an arm becomes unsatisfiable (R8), and
  `starting` → `dependency_failed` when its bounded start attempts are exhausted or its target became
  ineligible. `armed`, `ready`, `starting`, `running`, or `dependency_failed` → `finished` with
  outcome `cancelled` on an explicit cancel. `dependency_failed` → `armed` on an explicit re-arm or
  retry. `finished` is terminal; a finished task is re-run only by creating a new task.
- **Arm:** `unsatisfied` → `satisfied` when its prerequisite reaches an outcome inside its satisfying
  set or its signal is fired; `unsatisfied` → `unsatisfiable` when its prerequisite reaches any other
  terminal outcome, is parked, or is deleted. An arm never returns from `satisfied` to `unsatisfied`.
- **Outcome:** absent while `armed`, `ready`, `running`, or `dependency_failed`; exactly one value
  once `finished`. An accepted outcome is immutable.
- **Assignment and runtime claim:** a task acquires its agent id and generation when its start is
  confirmed and retains them as durable provenance afterwards, including after that agent stops or is
  archived. It records at the same moment whether it created, woke, or borrowed that runtime (R4), and
  holds a claim on it until the task finishes or is cancelled. Reassignment does not exist in this
  feature, so the assignee is written once.
- **Attachment:** absent → attached → removed with its task. Attachment state is independent of task
  state, of the canonical reference, and of every direct grant.

## 4. Edge cases & errors

- **R15** `(planned)` — **Cyclic and invalid graphs are rejected atomically.** Creating or re-arming a
  task whose arms would introduce a cycle, name itself, name an unknown or cross-project prerequisite,
  name an empty satisfying set, name a terminal-interface or archived target agent, or exceed the
  bounded arm and attachment counts returns a typed error and creates or changes no task, arm,
  attachment, or agent. A task graph is always acyclic.
- **R16** `(planned)` — **An agent that stops without reporting leaves the task needing attention.**
  If an assigned agent exits, crashes, is stopped by a person, or has its runtime switched before
  recording an outcome, the task stays `running`, releases its runtime claim and its budget slot,
  records why its agent went away, and is presented as needing attention. AgentDeck never converts that exit into `success` or `failure`, and dependents
  keep waiting rather than advancing on a guess. A person may record an outcome, retry the task, or
  cancel it.
- **R17** `(planned)` — **Restart re-evaluates durable state and never double-starts.** After a server
  restart, armed and ready tasks are re-evaluated from their durable rows and satisfied arms stay
  satisfied. Each `starting` task names the agent and generation its attempt targeted, so recovery
  checks whether that exact runtime is live and holding the assignment: if it is, the task becomes
  `running`; if it is not, the attempt is abandoned and the task returns to `ready` to be started once
  more, within its bounded attempt limit. A confirmed `running` task is never started again, matching
  FS-14.R14's rule that recovery never guesses that interrupted work passed and never launches two
  agents for one unit of work. Budget slots are recomputed from surviving runtime claims rather than
  carried across the restart.
- **R18** `(planned)` — **Deletion has narrow effects.** Deleting a task removes its arms, its
  attachments, and its work-derived context route, and parks any task armed on it (R8). It does not
  delete the assigned agent, its transcript, its archive entry, the canonical context reference, any
  direct grant, or any pipeline run. Deleting or archiving an agent does not delete a task; the task
  retains the agent id as provenance and, if it was still `running`, needs attention under R16.
- **R19** `(planned)` — **Existing lifecycle and archive gates still apply.** Starting a task obeys the
  same rules as any other start: an archived agent or archived project cannot be started into, a
  stopped agent is resumed only when it satisfies FS-01.R33's wake gates, a pipeline-associated agent
  is not woken for a task, and a task-started turn resets the messaging turn budget (FS-06.R11–R12)
  exactly as an activation turn does. A target that has become ineligible parks the task under R8
  rather than failing silently or retrying forever. Waiting for capacity (R7) is not a lifecycle gate:
  it delays a start, never fails one.
- **R20** `(planned)` — **Invalid task operations are typed and atomic.** Reporting an outcome for a
  task the caller is not assigned to, reporting twice, reporting an outcome outside the vocabulary,
  reporting an over-limit summary, cancelling a finished task, firing a signal outside the caller's
  project, or attaching a reference the caller cannot read returns a stable typed error and mutates
  nothing.

## 5. Acceptance criteria

Each names the verification that demonstrates it.

- **A1** `(planned)` (R1–R3, R5) — A task armed on another task's `success` stays `armed` until that
  prerequisite records `success`, survives a server restart in each state, and is unaffected by the
  prerequisite agent going `idle`, `done`, or `error` without a reported outcome: state and scheduler
  tests under `internal/state` and `internal/server`.
- **A2** `(planned)` (R6–R7) — A fan-in task armed on three prerequisites starts exactly once, only
  after the third records a satisfying outcome; a task targeting a stopped existing agent resumes it
  through the shared resume seam and a running one receives one bounded turn without consuming budget;
  a task reaches `running` only after its runtime is confirmed; and no provider prompt, mail row, or
  transcript event is produced to poll, wait, or announce completion: fake-ACP integration tests under
  `internal/server`.
- **A3** `(planned)` (R4, R16) — A task that launched its agent and a task that woke a stopped agent
  each stop that agent on finish, leaving it non-archived, resumable, and visible with status `done`;
  a task that borrowed a runtime already up for its own reasons releases its claim and leaves that
  agent running with its conversation intact; and an agent that exits without reporting leaves its
  task `running` and needing attention, releases its claim and slot, and leaves dependents waiting:
  lifecycle integration tests.
- **A4** `(planned)` (R8, R9, R15) — A prerequisite finishing outside its satisfying set parks the
  dependent as `dependency_failed` and propagates to its own dependents; a fired signal releases every
  arm waiting on that name and firing an unwatched name changes nothing; and a cyclic, self-naming,
  cross-project, or over-limit arm set is rejected without mutation: scheduler and state tests.
- **A5** `(planned)` (R10–R11) — An agent started for a task reads its instruction and attached
  reference ids with per-attachment labels in one call, reads an attached reference through its
  work-derived route, retains that route after the task finishes, loses it when the task is deleted,
  and is unaffected by hiding the same reference on the global direct-share list; the canonical
  reference and an unrelated direct grant are untouched throughout: MCP and `internal/contextref`
  integration tests.
- **A6** `(planned)` (R12, R20) — Tasks, arms, and attachments are created over HTTP and by a
  token-bound agent; a caller cannot report for a task it is not assigned to, report twice, or name
  another agent as reporter; every rejection is a stable typed error that mutates nothing: HTTP and
  MCP protocol tests.
- **A7** `(planned)` (R13) — A pipeline run completing with final outcome `success` registers
  `success`, one completing with another template-defined final outcome registers `failure`, and a
  stopped run registers `cancelled`, each releasing or parking a task armed on that run; the run's own
  routing, revisits, and recovery are unchanged: `internal/pipeline` and scheduler integration tests.
- **A8** `(planned)` (R14, R17–R19) — The Tasks view shows armed, ready-and-waiting, starting,
  running, parked, and finished work with what each armed task waits on; a restart with a `starting`
  row whose runtime is live promotes it to `running`, and one whose runtime is gone returns it to
  `ready` and starts it exactly once more, never twice; deleting a task parks its dependents while
  leaving its agent, transcript, canonical reference, and direct grants intact; and an archived target
  parks rather than retries: UI tests plus restart and deletion integration tests.
- **A9** `(planned)` (R7, R21) — Fifteen tasks becoming ready under a budget of four produce fifteen
  ready tasks and four running runtimes; each finish admits exactly one more in the order they became
  ready; a borrowed-runtime task starts regardless of the budget; lowering the budget below the
  running count stops nothing and admits nothing until the count falls; and the setting round-trips
  through settings with its default of four: scheduler and settings tests.

## 6. Deviations & open decisions

- Nothing in this specification has shipped.
- **Reassignment and participant membership are deliberately excluded.** A task's assignee is written
  once when it starts, so this feature needs no participant set, no reassignment transition, and no
  explicit participant removal. The integration rule those additions must follow is already recorded
  in FS-15 §3 and TS-01.R23; they belong to the semantic orchestration API rather than here.
- **Agents cannot list or inspect the task graph.** The scoped tools deliberately expose only creating
  a task, reading the caller's own assignment, reporting a result, and cancelling a task the caller
  created. A general query surface would reintroduce exactly the polling this feature exists to
  remove; an agent that wants to act on an outcome arms a task on it instead.
- **No time-based, recurring, or externally triggered starts.** There is no cron, timer, delay, or
  webhook. Firing a signal (R9) is the only way an external system advances work, and it is a
  deliberate API call rather than an observed condition.
- **Pipelines converge only at the result layer.** FS-14 keeps its own template snapshot, sequential
  routing, revisits, and recovery. Absorbing pipeline runs into tasks outright would require the task
  graph to grow revisit and back-edge semantics it does not otherwise need, which is the cyclic
  workflow engine both this specification and FS-14 decline to build. Revisit only with evidence that
  maintaining two run layers costs more than that.
- **The local trust model is unchanged.** Task tools add no authentication boundary; they reuse the
  existing per-launch token and server-derived identity (TS-05.R14, R16) on the same-machine trust
  basis described in TS-05.

## 7. Traceability

Anchors (planned): durable task, arm, and attachment rows plus their forward-only migration in
`internal/state`; arm evaluation and start scheduling alongside the existing activation executor in
`internal/server`; the `dependency` activation kind on the existing activation primitive
(TS-01.R19, TS-02.R23); the shared result-acceptance path with `internal/pipeline`; scoped tools on
the existing MCP server in `internal/messaging`; attached-reference reads through
`internal/contextref`; the Tasks view in `ui/src/features/tasks`.
