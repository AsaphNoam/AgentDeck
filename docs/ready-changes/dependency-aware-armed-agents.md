# Dependency-aware work that starts itself

**State:** In progress
**Why:** Human idea "Dependency-aware / armed agents" in `docs/ideas.md`, defined with the human on
2026-08-23. It is the third of the three follow-ons the orchestration-plane separation was built for,
after mail activation and context links.
**Relevant requirements:** FS-16.R1–R26, FS-16.A1–A17, TS-10.R1–R21, FS-02.R44, FS-02.A26,
FS-04.R43, FS-04.A23, FS-14.R34, TS-01.R24, TS-02.R25, TS-03.R28, TS-04.R29, TS-05.R17, TS-09.R27,
INV §1, §2, §4, §5, §7, §8, §9, §15

## Outcome

A piece of work can wait on other work and then start by itself. Someone models "start B when A
finishes", "start D after A, B, and C have all completed", or "unblock the reviewer once the
implementation reaches a terminal state", and AgentDeck starts each piece when its prerequisites are
satisfied. No agent polls another agent, waits, relays status, or sends a conversational "I'm done"
to release work — those become durable state transitions the host observes.

This fills the gap that made the idea impossible today: AgentDeck has no durable work object, and an
agent's status is process liveness, not a result. `done` is written by an ordinary Stop and by a
terminal shell exiting, so it says nothing about whether work succeeded.

## Included work

Included:

- A durable, project-scoped **task** with an instruction, a target (an existing agent or a launch
  spec), and one explicit outcome from `success`, `failure`, `blocked`, `cancelled` (FS-16.R1–R3).
- **Arms**: an AND-conjunction of prerequisites over another task's outcome, a pipeline run's
  registered outcome, or a fired project-scoped signal (FS-16.R5, R9). No OR, quorum, negation, or
  expression language.
- Host-driven starting through `ready → starting → running`, where `running` means the assignment
  crossed into a confirmed live runtime, an exclusive assignment claim is taken in the same statement
  that grants capacity, and a task whose assignee goes away becomes `interrupted` rather than staying
  `running` (FS-16.R2, R6, R16, TS-10.R4, R18).
- Recovery built on the shipped non-adoption boundary: a restart never claims a pre-crash runtime
  survived, so it abandons and reaps `starting` attempts, marks `running` tasks `interrupted`, and
  finishes any release a reporting turn never completed (FS-16.R17, TS-10.R15, R19).
- A reported result is released at the reporting turn's end through a turn-end fan-out shared with
  pipelines, so the reporting agent always receives its tool response before any stop (TS-10.R19,
  TS-09.R27, TS-01.R24).
- Person-recorded results, and re-arm and retry as distinct repairs, so parked and interrupted work
  can actually make progress (FS-16.R22–R23).
- Durable server-derived creator provenance behind agent cancel authority (FS-16.R24, TS-10.R20).
- A bounded start-attempt policy where only real failures spend an attempt and contention never does,
  and where re-arm and retry each say plainly what they repair (FS-16.R23, R25).
- A code-owned dependency activation instruction and status, and the kind-parameterized activation
  statements it needs, so an activated agent is told it has a task rather than told to check its mail
  (FS-16.R26, TS-10.R5).
- Deletion refused while a task still owns a runtime or an unfinished release, and a restart that
  never stops a runtime a task merely borrowed (FS-16.R4, R18, TS-10.R15, R16).
- **Logical fan-out with a physical budget**: every satisfied task becomes ready however many that
  is, while a configurable install-wide budget (default ten) limits how many runtimes AgentDeck
  brings up at once, admitting ready work in order as capacity frees (FS-16.R7, R21, TS-10.R17,
  FS-04.R43).
- Finishing **releases the task's runtime claim** and stops the agent, without archiving it, only
  when the task created or woke that runtime; a borrowed runtime a person is using is left alone
  (FS-16.R4).
- An unsatisfiable prerequisite parks the dependent as `dependency_failed` and surfaces it; nothing
  is silently cancelled or left waiting forever (FS-16.R8).
- **Result-layer convergence with pipelines**: one outcome vocabulary, one validation, one accept
  transaction shared by `report_pipeline_stage_result` and the task report tool, and a run's terminal
  outcome registered so runs can be prerequisites (TS-09.R27, FS-14.R34).
- **Context attachments** on a task with per-attachment label and description, read by the assignee
  through a work-derived route (FS-16.R10–R11).
- Four scoped MCP tools and the HTTP/SSE surface, plus a **Tasks view** and a dashboard
  needs-attention count (FS-16.R12, R14, FS-02.R44, TS-03.R28, TS-04.R29).

Deliberately not included: reassignment, participant membership, and explicit participant removal;
any agent-facing task-graph query or listing tool; time-based, recurring, or webhook triggers;
absorbing pipeline runs into tasks; parallel branches, joins, or child pipelines inside FS-14; and
any change to the same-machine trust model. Each exclusion and its reason is recorded in FS-16 §6 and
TS-10 §5.

## How we will know it works

FS-16.A1–A17, FS-02.A26, and FS-04.A23 are the acceptance set: scheduler and state tests for arming,
restart, and the lifecycle-versus-outcome separation; fake-ACP integration tests proving fan-in
starts once, that a task reaches `running` only on a confirmed runtime, and that no prompt, mail row,
or transcript event is produced to poll or announce completion; scheduler and settings tests proving
fifteen ready tasks run four at a time with the budget set to four, admitting in order as each
finishes, and that the shipped default is ten;
lifecycle tests for stopping a created or woken runtime, leaving a borrowed one alone, proving the
reporter gets its tool response before any stop, and leaving an interrupted task retryable to a real
result; concurrency tests proving two tasks admitted for one agent leave exactly one active;
authorization tests for creator-scoped cancel across a stop and resume; deletion tests covering every
dependent state; scheduler and state tests for parking, signals, and cycle rejection; MCP and
`internal/contextref` tests for attachment reads and their independence from the global share list;
HTTP and MCP protocol tests for typed atomic rejections; `internal/pipeline` tests for outcome
registration; and UI plus restart and deletion integration tests.

## Waiting on

Nothing. Every product and architectural decision is recorded in FS-16 and TS-10.

Both design reviews are resolved in the requirements: the first review's nine Must-fix findings and
the re-audit's twelve Must-fix plus two Worth-fixing findings. Five findings across the two rounds
were verified against shipped code before being accepted, and that verification changed two of the
re-audit's remedies. Deleting a paused pipeline run cannot strand an arm, because the shipped delete
path refuses any run that is not terminal and a terminal run registers its outcome in the same
commit — so no fan-out into the pipeline delete path is added (TS-10.R21, FS-16.A14). And
`AcceptPipelineReport` does not own routing as the review stated; it writes the attempt report,
declared stage outputs, and `await_quiescence`, so what is shared with tasks is the vocabulary,
limits, and staleness check, called inside each domain's own transaction (TS-10.R7, TS-09.R27).
From the first round: the runtime registry is in-process and starts empty and stale
reconciliation deliberately never re-adopts a live process (`internal/runtime/registry.go`,
`internal/runtime/reconcile.go`, FS-01.R20–R21), so recovery no longer asks whether a pre-crash
runtime survived; and pipelines really do hold an `await_quiescence` boundary between an accepted
report and stopping the reporter, through a turn-end dispatch with one hard-coded consumer, so that
call site becomes a shared fan-out instead of gaining a second path.

Two things the implementing change must also do, since they are true only once this ships: update
FS-15 §6, which currently lists work objects, dependency evaluation, and assignment APIs as
deliberately excluded, and flip the `(planned)` tags and statuses across FS-02, FS-14, FS-16, TS-01,
TS-02, TS-03, TS-04, TS-05, TS-09, and TS-10.

Noted for the reviewer, not blocking: FS-14 is marked `Partial` but carries no `(planned)` item — it
satisfies the lint only through a prose mention of the tag. That predates this change; either its
status is wrong or an unshipped requirement is untagged.
