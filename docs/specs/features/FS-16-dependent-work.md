# FS-16 — Dependent work and armed starts

**Status:** Current
**Code:** `internal/state`, `internal/server`, `internal/messaging`, `ui/src/features/tasks` · **Journeys:** —
**Absorbed:** —

## 1. Purpose

AgentDeck understands dependencies between pieces of work natively. A piece of work can exist in a
durable "ready to run once these prerequisites are satisfied" state, and AgentDeck starts it when
those prerequisites are satisfied. No model polls another agent, waits, relays status, or sends a
conversational "I'm done" purely to advance orchestration. Dependencies attach to durable work and
its explicit reported outcome, never to an agent's lifecycle status.

## 2. Behavior

Requirements are user- and agent/API-observable. R-item numbering is continuous through §4.

### 2.1 Durable work and explicit outcomes

- **R1** — **A task is durable work with an explicit outcome.** A task is a durable,
  project-scoped record with a stable opaque id, a display name, an instruction, an optional set of
  prerequisites, a state, and — once finished — one outcome. It outlives the agent that executes it,
  survives restart, and is not deleted when its agent stops, crashes, or is archived. A task holds no
  copied transcript, no provider session, and no context payload.
- **R2** — **A task names how it will be executed.** A task targets either one existing
  chat agent or a launch specification (role, project, backend, model) from which AgentDeck creates a
  new agent when the task starts. A task has at most one assigned agent at a time, and an agent holds
  at most one active task — one that is `starting` or `running` — at a time. That exclusivity is a
  durable claim taken atomically when the task is admitted, not a consequence of scheduling order, so
  two tasks admitted at the same moment for the same agent cannot both become active; the loser stays
  ready. Terminal-interface agents cannot execute tasks, matching FS-07's messaging boundary.
- **R3** — **Outcomes are reported, never inferred.** A finished task carries exactly one
  outcome from the closed vocabulary `success`, `failure`, `blocked`, or `cancelled`. The assigned
  agent or a person may record `success`, `failure`, or `blocked`; `cancelled` is written only by
  AgentDeck when the task is cancelled or its agent's work is abandoned. An agent becoming `idle`,
  `done`, or `error` (FS-01.R17–R20) never sets, implies, or clears a task outcome, and no unread
  count, status transition, sweep, or restart records one. What an agent may report is deliberately the
  same set a pipeline stage report accepts — `success`, `failure`, `blocked` (FS-14.R6, R19) — so one
  vocabulary, one set of field limits, and one validation serve both. The wider set differs by design
  and is not merged: `cancelled` is task-only and host-written, and a pipeline *run*'s registered
  terminal outcome is `success`, `failure`, or `cancelled` (R13), which is a different event from a
  stage report and happens later.
- **R4** — **Finishing releases the task's claim on its runtime, and stops only the
  runtime the task itself brought up.** A task records at start time whether it created the runtime by
  launching a new agent, woke it by resuming a stopped one, or merely borrowed a runtime that was
  already up for its own reasons. When the task finishes it always releases its claim. AgentDeck then
  stops the agent through the shared stop seam (FS-01.R6, R34) only in the first two cases, returning
  it to the state the task found it in. A borrowed runtime is left alone, so completing a task never
  kills a conversation a person is in the middle of. Every terminal transition — an agent-reported
  result, a person-recorded result, and a cancel — commits the terminal state together with a durable
  intent to release, and only then performs the stop. When the agent reported the result itself the
  stop is deferred until that reporting turn ends, never issued inside the tool call, so the agent
  always receives its own tool response; the claim and its budget slot are held until then. In the
  other two cases the stop follows the commit immediately. Because the intent is durable, a crash or a
  failed stop between the commit and the effect never strands a live task-owned runtime: recovery
  finishes the release that was already promised (R17). A stopped agent becomes a stopped, non-archived
  agent (FS-01.R31): its card, identity, and transcript stay visible and it resumes like any other
  stopped conversation. Nothing is ever archived or deleted by finishing a task.

### 2.2 Arming, prerequisites, and starting

- **R5** — **Prerequisites are a conjunction of typed arms.** A task carries zero or more
  arms, and every arm must be satisfied before the task becomes ready. There is no OR, quorum,
  negation, expression language, or workflow DSL. An arm is one of:
  (a) **task** — a prerequisite task id plus the non-empty set of its outcomes that satisfy the arm;
  (b) **pipeline run** — a pipeline run id plus the satisfying set over that run's registered terminal
  outcome (R13); (c) **signal** — a project-scoped signal name, satisfied when that signal is fired
  (R9). A task with zero arms is ready as soon as it is created.
- **R6** — **Ready work starts without a model in the loop.** AgentDeck observes its own
  durable records, and when a task's last arm is satisfied it starts that task itself. A task
  targeting a launch specification is started by launching its agent with the task's instruction as
  the intentional assignment input permitted by FS-00.R14–R15. A task targeting an existing agent
  crosses into that agent's existing conversation through a host-owned activation (FS-00.R15,
  TS-01.R19); a running agent is given one bounded turn and a stopped agent is resumed first on the
  same wake terms as mail (FS-01.R33). A task is `running` only once its assignment has crossed into
  a confirmed runtime; until then it is `starting` and its start is still owned by AgentDeck, not by
  the agent. No agent is asked to poll, wait, check whether a prerequisite finished, or announce its
  own completion to release other work.
- **R7** — **Readiness is logical; running processes are budgeted.** Every task whose
  arms are satisfied becomes ready, however many that is: a graph that fans out to fifteen tasks has
  fifteen ready tasks, and nothing about the dependency model is narrowed to fit a machine. Physical
  concurrency is separate. A configurable install-wide budget, default ten, limits how many agent
  runtimes AgentDeck itself brings up for tasks at one time. Ready tasks are admitted in the order
  they became ready as capacity frees, and a task whose start borrows a runtime that is already up
  (R4) consumes no budget because it creates no process. A ready task waiting for capacity is
  presented as ready and waiting, not as failed or blocked. The existing exclusive per-agent lifecycle
  transition (FS-01.R34) still serializes two operations aimed at the same agent.
- **R21** — **The budget is visible and adjustable.** The concurrency budget is a single
  install-wide setting with a default of ten, readable and writable through settings (FS-04.R43). It
  counts only runtimes this feature created or woke and still holds a claim on, so agents a person
  launched or resumed themselves are never counted and never blocked. Lowering the budget below the
  number of currently running task runtimes stops nothing; those runtimes finish and release normally
  and no new one starts until the count is back under the budget.
- **R8** — **A prerequisite that can never be satisfied parks the dependent.** When a
  prerequisite reaches a terminal outcome outside its arm's satisfying set, the dependent task moves
  to the durable `dependency_failed` state, records which arm failed and why, and is presented as
  needing attention. The same happens when a prerequisite task itself reaches `dependency_failed`, or
  when an unsatisfied arm's prerequisite is deleted (R18). A parked task never starts on its own.
  Because a recorded result is immutable, retrying a task parked this way would park it again at once:
  the repair is re-arming it with a different arm set, or cancelling it (R23). Nothing is silently
  cancelled, silently dropped, or left waiting on an outcome that can never arrive.
- **R9** — **Signals are named releases, not stored objects.** A person or an API client
  fires a project-scoped signal by name. Firing satisfies every signal arm waiting on that name at
  that moment and is recorded on the arms it satisfied. A signal is not a durable object with its own
  identity, CRUD surface, history, or retention: an arm may name a signal that has never been fired,
  and firing a name no arm is waiting on succeeds and changes nothing. Signals exist so work can be
  armed on something AgentDeck cannot observe, such as a CI result or an external approval.

### 2.3 Assignment context

- **R10** — **A task carries its own context attachments.** A task may attach canonical
  context reference ids (FS-15.R1) with an attachment-specific label and description (FS-15.R3). The
  attachment belongs to the task, not to the reference: attaching does not copy content, does not
  duplicate the reference, and does not create or modify a direct grant. The assigned agent may read
  an attached reference through a work-derived route for as long as it is that task's assignee.
  A terminal outcome does not by itself revoke that route; deleting the task removes only that task's
  attachments and work-derived route, leaving the canonical reference and every direct grant intact.
- **R11** — **An assigned agent receives its assignment directly.** An agent started for a
  task can read that task's instruction and its attached reference ids with presentation metadata in
  one bounded call. It never scans the global direct-share list to work out which context belongs to
  its current work, and personal hidden/visible state on that list (FS-15.R6) cannot detach or revoke
  context an active task requires.

### 2.4 Authoring, control, and presentation

- **R12** — **People and agents both create and control work.** Tasks, arms, and
  attachments are created over the local HTTP API and its UI, and by a token-bound chat agent through
  scoped MCP tools that create a task, read its own assignment, report its result, and cancel a task
  it created. Caller identity is always server-derived (TS-04.R7); no tool argument names another
  agent as the reporter or as a task's creator. Every task durably records who created it (R24).
  An agent-created task may target a launch specification, itself, or another chat agent named the
  same way a message names its recipient — a friendly selector that AgentDeck resolves server-side
  against durable identities, returning the same unknown and ambiguous recipient errors coordination
  already returns (FS-06). A raw agent id in a tool argument is not a target and is never authority;
  resolution, not the caller, decides which agent a task points at. This deliberately opens what
  FS-14 keeps closed for pipelines: an agent can cause new work to start without a person in the loop,
  which is the point of expressing orchestration as control state rather than prose.
- **R13** — **Pipeline results register in the shared result layer.** When a pipeline run
  reaches a terminal state, AgentDeck registers its outcome in the same result vocabulary as R3:
  `success` when the run completes with the final outcome `success`, `failure` when it completes with
  any other template-defined final outcome, and `cancelled` when the run was stopped. The raw
  template-defined label is retained as a display detail. That registration is what makes a pipeline
  run usable as a prerequisite (R5b). Pipeline runs keep their own template routing, revisits, and
  recovery; this requirement adds no cross-run join, parallel branch, or child pipeline to FS-14.
- **R14** — **Dependent work has its own view.** A Tasks view lists tasks for a project
  with their state, outcome, what each armed task is waiting on, and which parked task needs
  attention, and it offers create, record a result (R22), re-arm, retry, cancel, and delete (R18,
  R23). The dashboard is unchanged apart from an indicator of how many tasks need attention
  (FS-02.R44).
- **R22** — **A person can record a result when no agent will.** A person records
  `success`, `failure`, or `blocked` on a `running` or `interrupted` task over the local HTTP API and
  its Tasks view. This is the only non-cancelling way to resolve work whose agent went away, and it is
  the counterpart to the agent's report tool rather than an override of it: recording a result on a
  task that is already `finished`, or on one that is `armed`, `ready`, `starting`, or
  `dependency_failed`, is rejected and changes nothing. A person-recorded result is accepted, is
  immutable, and releases the runtime claim immediately (R4), because there is no reporting turn to
  wait for. It is marked as person-recorded so it is never mistaken for an agent's own report.
- **R23** — **Repairing arms and retrying execution are different operations.** *Re-arm*
  replaces a task's arm set atomically, revalidating the whole graph (R15), and returns the task to
  `armed`, or straight to `ready` when the new arms are already satisfied. It is accepted only on an
  `armed`, `ready`, or `dependency_failed` task and is rejected with a typed error on a `starting`,
  `running`, `interrupted`, or `finished` one, whose arms have already been passed or are moot. It is
  the only repair for a task parked because an arm can never be satisfied, since a recorded result is
  immutable and retrying the same arms parks it again immediately. *Retry* changes no arms and
  re-attempts execution: it returns an `interrupted` task, or one parked because its start attempts
  were exhausted, to `ready` with a fresh attempt allowance (R25). Retry is rejected on a task parked
  by an unsatisfiable arm, with a typed error naming re-arm as the repair, so neither operation
  silently fails to make progress.
  Retry acts on the assignee the task already has rather than inventing a second one. A task targeting
  an existing agent retries against that same target. A launch-spec task that never confirmed an
  assignee launches fresh, because nothing was ever confirmed. A launch-spec task that did confirm one
  resumes that same agent, which keeps its transcript continuous and its attached-context membership
  valid; minting a second agent would fork both and would require the reassignment this feature
  excludes. If that assignee has since been deleted or archived, retry is rejected with a typed error
  naming the reason, and the work is restarted by creating a new task.
- **R25** — **Start attempts are bounded, and only real failures spend them.** A task gets
  three start attempts. An attempt is spent only when bringing the work up genuinely fails: a launch
  that does not start, a resume that does not complete, or a target that has become ineligible. Being
  deferred spends nothing and is not a failure — losing a race for the exclusive assignment claim,
  finding the agent's lifecycle busy, or waiting for capacity all return the task to `ready` with its
  allowance intact and its place in the admission order kept, so a busy machine can never exhaust a
  task's attempts. There is no timed backoff; re-admission happens on the next dispatch, and the
  attempt count is the bound. When the three are spent the task parks as `dependency_failed` recording
  the last failure, and an explicit Retry (R23) restores the full allowance. If AgentDeck cannot stop
  or reap a runtime after assignment delivery or restart recovery fails, the task deliberately remains
  `starting`: that reservation is its only durable ownership record, so clearing or settling it could
  admit duplicate work. A later server restart retries the reap before the task can advance.
- **R26** — **An agent started for a task is told it has a task.** The activation that
  crosses into an existing conversation carries one short, code-owned instruction to read its
  assignment and act on it, and its own status text while that turn runs. It does not reuse mail's
  instruction or mail's status, because an agent told to check its messages will do exactly that and
  never find its task. The instruction carries no task id, arm set, context reference, or assignment
  text: the agent reads all of that through R11, which keeps the activation payload-free.
- **R24** — **Every task records who created it.** A task durably records whether a person
  or an agent created it and, for an agent, that agent's stable id, captured server-side at creation
  from the caller's token. An agent may cancel only a task whose recorded creator is that same agent
  id; the recorded launch generation is provenance only, so an agent that was stopped and resumed
  keeps the right to cancel work it created earlier, and a task created by a person can never be
  cancelled by an agent. A creator id supplied in a tool argument is never authority (TS-05.R17).

## 3. States & transitions

- **Task:** `armed` (at least one arm unsatisfied) → `ready` (every arm satisfied; eligible, and
  waiting for capacity if the budget is full) → `starting` (capacity and an exclusive assignment claim
  granted, a start attempt under way, naming the agent and generation it targets) → `running` (the
  assignment has crossed into a confirmed runtime) → `finished` (one outcome recorded). `running`
  means the agent is genuinely up and holding the assignment; nothing that is not running is
  `running`.
  `starting` → `ready` when a start attempt is abandoned without confirmation and may be retried, and
  `starting` → `dependency_failed` when its bounded start attempts are exhausted or its target became
  ineligible. `running` → `interrupted` when the assignee goes away before recording a result (R16),
  which is also where a `running` task lands after a restart (R17). `interrupted` → `ready` on retry
  and `interrupted` → `finished` when a person records a result (R22). `armed` or `ready` →
  `dependency_failed` when an arm becomes unsatisfiable (R8), and `dependency_failed` → `armed` or
  `ready` on re-arm, or → `ready` on retry only when it was parked by exhausted start attempts (R23).
  Any state → `finished` with outcome `cancelled` on an explicit cancel. `finished` is terminal; a
  finished task is re-run only by creating a new task.
- **Arm:** `unsatisfied` → `satisfied` when its prerequisite reaches an outcome inside its satisfying
  set or its signal is fired; `unsatisfied` → `unsatisfiable` when its prerequisite reaches any other
  terminal outcome, is parked, or is deleted. An arm never returns from `satisfied` to `unsatisfied`,
  and deleting a prerequisite whose result already satisfied an arm leaves that arm satisfied (R18).
  An `unsatisfiable` arm is never repaired in place; re-arming replaces the whole set (R23).
- **Outcome:** absent in every non-terminal state — `armed`, `ready`, `starting`, `running`,
  `interrupted`, and `dependency_failed` — and exactly one value once `finished`. There is no state in
  which an outcome is partially recorded. An accepted outcome is immutable, and it records whether an
  agent or a person supplied it (R22).
- **Assignment and runtime claim — reserved first, confirmed second.** A task *reserves* its target
  agent when it is admitted to `starting`, recording the agent id, the generation it intends to act
  on, and whether that start will create, wake, or borrow the runtime (R4). The reservation is what
  the exclusive assignment claim (R2) holds and what recovery reads, and it exists before any effect
  precisely so a crash mid-start is identifiable. It is *confirmed* into durable assignee membership
  only when the assignment crosses into a live runtime and the task becomes `running`; membership is
  what authorizes reading attached context (R10). An abandoned reservation releases the claim and
  authorizes nothing. A confirmed assignee is retained as durable provenance afterwards, including
  after that agent stops or is archived, and its claim is held until the task's result is recorded
  and, for an agent-reported result, that reporting turn has ended. Reassignment does not exist in
  this feature, so the assignee is written once. That membership is separate from the per-launch MCP
  registration, which is torn down whenever the runtime exits and re-established on the next one.
- **Attachment:** absent → attached → removed with its task. Attachment state is independent of task
  state, of the canonical reference, and of every direct grant.

## 4. Edge cases & errors

- **R15** — **Cyclic and invalid graphs are rejected atomically.** Creating or re-arming a
  task whose arms would introduce a cycle, name itself, name an unknown or cross-project prerequisite,
  name an empty satisfying set, name a terminal-interface or archived target agent, or exceed the
  bounded arm and attachment counts returns a typed error and creates or changes no task, arm,
  attachment, or agent. A task graph is always acyclic.
- **R16** — **An agent that goes away without reporting leaves the task interrupted.**
  If an assigned agent exits, crashes, is stopped by a person, or has its runtime switched before
  recording an outcome, the task moves to `interrupted`, releases its runtime claim and its budget
  slot, records why its agent went away, and is presented as needing attention. It does not stay
  `running`, because nothing is running. AgentDeck never converts that exit into `success` or
  `failure`, and dependents keep waiting rather than advancing on a guess. A person may record an
  outcome (R22), retry the task (R23), or cancel it.
- **R17** — **A restart ends every task runtime rather than adopting it.** AgentDeck
  deliberately does not re-adopt an agent process that outlived a previous server (FS-01.R20), so
  after a restart no task runtime is owned and none can be trusted to still hold its assignment.
  Recovery therefore never claims one survived. Armed and ready tasks are re-evaluated from their
  durable rows and satisfied arms stay satisfied. A task that was `starting` is resolved by the
  reservation it recorded. If that start would have created or woken the runtime, the leftover agent
  is reaped through the ordinary orphan path (FS-01.R21) and the task returns to `ready` to be started
  once more within its attempt limit (R25). If it would have borrowed a runtime that was already up
  for someone else's reasons, that runtime is never touched — R4's promise does not lapse because
  AgentDeck restarted — and because it cannot be known whether the assignment reached that
  conversation, the task becomes `interrupted` for a person to resolve rather than being silently
  delivered twice. A task that was `running` becomes `interrupted` under R16, because its agent is now
  an unowned orphan. A task whose result was already recorded stays finished, and any stop and
  release its reporting turn never completed is finished during recovery. Budget slots are recomputed from surviving claims. Nothing is resumed on a guess
  and no unit of work gets two agents, matching FS-14.R14.
- **R18** — **Deletion has narrow effects, decided per dependent, and never abandons a
  runtime.** Deletion is rejected with a typed error while a task still owns something live: while it
  is `starting` or `running`, and while a finished or cancelled task's stop and release have not
  completed. Cancel it first and let its cleanup finish, then delete. This keeps the task row — the
  only record of the runtime claim, the budget slot, and the pending release — alive until nothing
  depends on it, so deleting can never strand a running agent or leak a slot. Deleting an otherwise
  eligible task removes its arms, its attachments, and its work-derived context route. Its recorded
  result is not removed: a result is keyed to its source and outlives the task that produced it, so a dependent
  whose arm that result already satisfied is completely unaffected, whatever state that dependent is
  in. Only an arm still waiting on the deleted task becomes unsatisfiable, which parks its `armed` or
  `ready` dependent under R8. A dependent that is already `starting`, `running`, `interrupted`, or
  `finished` has passed its arms and is never reopened or parked. Deletion does not delete the
  assigned agent, its transcript, its archive entry, the canonical context reference, any direct
  grant, or any pipeline run. Deleting or archiving an agent does not delete a task; the task retains
  the agent id as provenance and, if it was still `running`, becomes `interrupted` under R16.
- **R19** — **Existing lifecycle and archive gates still apply.** Starting a task obeys the
  same rules as any other start: an archived agent or archived project cannot be started into, a
  stopped agent is resumed only when it satisfies FS-01.R33's wake gates, a pipeline-associated agent
  is not woken for a task, and a task-started turn resets the messaging turn budget (FS-06.R11–R12)
  exactly as an activation turn does. A target that has become ineligible parks the task under R8
  rather than failing silently or retrying forever. Waiting for capacity (R7) is not a lifecycle gate:
  it delays a start, never fails one.
- **R20** — **Invalid task operations are typed and atomic.** Reporting an outcome for a
  task the caller is not assigned to, reporting twice, reporting an outcome outside the vocabulary,
  reporting an over-limit summary, cancelling a finished task, cancelling a task the caller did not
  create (R24), recording a person result on a task that is neither `running` nor `interrupted` (R22),
  retrying a task parked as `arm_unsatisfiable` (R23), re-arming a task outside `armed`, `ready`, or
  `dependency_failed` (R23), re-arming into a cycle (R15), deleting a task that still owns a runtime
  or an unfinished release (R18), firing a signal outside the caller's project, or attaching a
  reference the caller cannot read returns a stable typed error and mutates nothing.

## 5. Acceptance criteria

Each names the verification that demonstrates it.

- **A1** (R1–R3, R5) — A task armed on another task's `success` stays `armed` until that
  prerequisite records `success`, survives a server restart in each state, and is unaffected by the
  prerequisite agent going `idle`, `done`, or `error` without a reported outcome: state and scheduler
  tests under `internal/state` and `internal/server`.
- **A2** (R6–R7) — A fan-in task armed on three prerequisites starts exactly once, only
  after the third records a satisfying outcome; a task targeting a stopped existing agent resumes it
  through the shared resume seam and a running one receives one bounded turn without consuming budget;
  a task reaches `running` only after its runtime is confirmed; and no provider prompt, mail row, or
  transcript event is produced to poll, wait, or announce completion: fake-ACP integration tests under
  `internal/server`.
- **A3** (R4, R16) — A task that launched its agent and a task that woke a stopped agent
  each stop that agent on finish, leaving it non-archived, resumable, and visible with status `done`;
  a task that borrowed a runtime already up for its own reasons releases its claim and leaves that
  agent running with its conversation intact; and an agent that exits without reporting leaves its
  task `interrupted` and needing attention, releases its claim and slot, and leaves dependents
  waiting; and the reporting agent always receives its tool response before any stop, with the claim
  and slot released at that turn's end rather than inside the call: lifecycle integration tests.
- **A4** (R8, R9, R15) — A prerequisite finishing outside its satisfying set parks the
  dependent as `dependency_failed` and propagates to its own dependents; a fired signal releases every
  arm waiting on that name and firing an unwatched name changes nothing; and a cyclic, self-naming,
  cross-project, or over-limit arm set is rejected without mutation: scheduler and state tests.
- **A5** (R10–R11) — An agent started for a task reads its instruction and attached
  reference ids with per-attachment labels in one call, reads an attached reference through its
  work-derived route, retains that route after the task finishes, loses it when the task is deleted,
  and is unaffected by hiding the same reference on the global direct-share list; the canonical
  reference and an unrelated direct grant are untouched throughout: MCP and `internal/contextref`
  integration tests.
- **A6** (R12, R20) — Tasks, arms, and attachments are created over HTTP and by a
  token-bound agent; a caller cannot report for a task it is not assigned to, report twice, or name
  another agent as reporter; every rejection is a stable typed error that mutates nothing: HTTP and
  MCP protocol tests.
- **A7** (R13) — A pipeline run completing with final outcome `success` registers
  `success`, one completing with another template-defined final outcome registers `failure`, and a
  stopped run registers `cancelled`, each releasing or parking a task armed on that run; the run's own
  routing, revisits, and recovery are unchanged: `internal/pipeline` and scheduler integration tests.
- **A8** (R14, R17–R19) — The Tasks view shows armed, ready-and-waiting, starting,
  running, interrupted, parked, and finished work with what each armed task waits on; a restart
  abandons a `starting` attempt, reaps its recorded agent, and starts the task exactly once more,
  turns a `running` task into `interrupted`, and completes a stop and release its reporting turn never
  finished; deleting a task parks only dependents whose arm was still waiting, while
  leaving its agent, transcript, canonical reference, and direct grants intact; and an archived target
  parks rather than retries: UI tests plus restart and deletion integration tests.
- **A9** (R7, R21) — Fifteen tasks becoming ready with the budget set to four produce
  fifteen ready tasks and four running runtimes; each finish admits exactly one more in the order they
  became ready; a borrowed-runtime task starts regardless of the budget; lowering the budget below the
  running count stops nothing and admits nothing until the count falls; and the setting round-trips
  through settings with its default of ten: scheduler and settings tests.
- **A10** (R16, R22–R23) — An assignee that crashes, is stopped, or is switched away leaves
  its task `interrupted` with its claim and slot released; retry returns it to `ready` and it runs to
  a real result on the same assignee it already had, asserted by agent id and by a new generation,
  while a launch-spec task that never confirmed an assignee launches a fresh one and a retry whose
  prior assignee was archived is rejected with a typed error; a person-recorded result finishes it and releases the claim at once; recording a
  result on an `armed` or `finished` task is rejected without mutation: lifecycle, scheduler, and HTTP
  tests.
- **A11** (R8, R23) — A task parked by an unsatisfiable arm is rejected for retry with an
  error naming re-arm, and re-arming it onto a different, already-satisfied prerequisite moves it to
  `ready` and it starts — so the parked state is demonstrably repairable rather than permanent:
  scheduler and HTTP tests. The Tasks view offers each park reason only the repair that can succeed:
  Retry on a task parked by exhausted start attempts, and Re-arm without Retry on one parked by an
  unsatisfiable arm: UI test.
- **A12** (R18) — Deleting a prerequisite parks a dependent whose arm was still waiting,
  and leaves untouched a dependent whose arm that prerequisite had already satisfied, in each of the
  `ready`, `starting`, `running`, `interrupted`, and `finished` dependent states: state tests covering
  every case.
- **A14** (R5, R13, R18) — A pipeline run cannot be deleted before it is terminal, and a
  run that reaches a terminal state registers its outcome in the same commit, so deleting it
  afterwards leaves every arm that waited on it already resolved — satisfied arms stay satisfied and
  non-matching ones are already parked — with no arm left waiting on a run that no longer exists:
  `internal/pipeline` and scheduler deletion tests.
- **A15** (R6, R11, R26) — A fake provider agent activated for a task receives the task
  instruction rather than the mail instruction, shows the task status text, and calls the assignment
  route to obtain its instruction and attached references; a mail activation for the same agent is
  unchanged: fake-ACP and activation tests.
- **A16** (R25) — Losing the assignment claim, finding the lifecycle busy, and waiting for
  capacity each return a task to `ready` without spending an attempt or losing its place; three real
  launch or resume failures park it as `dependency_failed` recording the last one; and an explicit
  Retry restores the full allowance: scheduler tests.
- **A17** (R4, R17–R18, R22) — Deleting a `starting`, `running`, or not-yet-released task
  is refused with a typed error and mutates nothing; a restart never stops a runtime a task borrowed
  and leaves that task `interrupted`; and a crash between committing a person-recorded result or a
  cancel and its stop leaves a durable release that recovery completes exactly once: deletion,
  restart, and lifecycle integration tests.
- **A13** (R2, R24) — Two tasks admitted concurrently for the same agent leave exactly one
  `starting` or `running` and the other `ready`, under repeated concurrent execution; and an agent can
  cancel a task it created, still can after being stopped and resumed, and cannot cancel a task
  created by another agent, by a person, or named by a spoofed argument: concurrency and MCP
  authorization tests.

## 6. Deviations & open decisions

- **Planned transport supersession.** If and only if FS-17.R20 passes and the direct-action
  migration ships, FS-17.R13–R19 replace only this specification's internal-MCP transport wording.
  Task authority, prerequisite arms, activation, lifecycle, outcomes, cancellation, recovery, and
  retention remain authoritative. Until then the shipped MCP requirements and acceptance criteria
  are current.

- **No live-provider pass yet.** Every behavior here is proven against the fake ACP adapter and the
  durable rows. The pinned Claude and Codex adapters have not been driven through a task start, an
  assignment turn, or a reported result.
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

Durable task, arm, attachment, and result rows with their forward-only migration:
`internal/state/tasks.go`. Arm evaluation and the shared work-report rules: `EvaluateSource`,
`FireSignal`, and `internal/state/work_report.go`. Admission, starting, recovery, and the turn-end
release: `internal/server/task_dispatcher.go`. The HTTP surface and the agent-facing control plane:
`internal/server/task_handlers.go`. The `dependency` activation kind on the existing activation
primitive: `internal/runtime/activation_kinds.go` with `runActivationTurn`/`wakeForActivation` in
`internal/server/messaging_loops.go`. Scoped tools: `internal/messaging/task_tools.go`.
Attached-reference reads: `ContextReadAuthorized` in `internal/state/context_links.go`. The Tasks
view and the dashboard count: `ui/src/features/tasks/TasksPage.tsx` and
`ui/src/components/grid/CardGrid.tsx`.

Key regressions: `internal/state/tasks_test.go` (arms, admission, attempts, deletion, concurrency),
`internal/server/task_dispatch_test.go` (starting, budget, activation, recovery, release),
`internal/server/task_http_test.go` (authoring, control, authority),
`internal/messaging/task_tools_test.go` (tool identity, reporting, attached reads), and
`internal/state/pipelines_test.go` (run outcome registration).
