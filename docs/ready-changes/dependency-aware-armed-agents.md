# Dependency-aware work that starts itself

**State:** Waiting to start
**Why:** Human idea "Dependency-aware / armed agents" in `docs/ideas.md`, defined with the human on
2026-08-23. It is the third of the three follow-ons the orchestration-plane separation was built for,
after mail activation and context links.
**Relevant requirements:** FS-16.R1–R20, FS-16.A1–A8, TS-10.R1–R16, FS-02.R44, FS-02.A26,
FS-14.R34, TS-01.R24, TS-02.R25, TS-03.R28, TS-04.R29, TS-05.R17, TS-09.R27, INV §1, §2, §4, §5,
§7, §8, §9, §15

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
- Host-driven starting: full fan-out, launch-spec tasks launch, existing-agent tasks cross through
  the new `dependency` activation kind on the shared activation primitive (FS-16.R6–R7, TS-01.R24).
- Finishing **stops the assigned agent without archiving it**, so the card stays visible and
  resumable while the process is freed (FS-16.R4).
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

FS-16.A1–A8 and FS-02.A26 are the acceptance set: scheduler and state tests for arming, restart, and
the lifecycle-versus-outcome separation; fake-ACP integration tests proving fan-in starts once,
simultaneous fan-out all starts, and that no prompt, mail row, or transcript event is produced to
poll or announce completion; lifecycle tests for stop-without-archive and for an agent that exits
without reporting; scheduler and state tests for parking, signals, and cycle rejection; MCP and
`internal/contextref` tests for attachment reads and their independence from the global share list;
HTTP and MCP protocol tests for typed atomic rejections; `internal/pipeline` tests for outcome
registration; and UI plus restart and deletion integration tests.

## Waiting on

Nothing. Every product and architectural decision is recorded in FS-16 and TS-10.

Two things the implementing change must also do, since they are true only once this ships: update
FS-15 §6, which currently lists work objects, dependency evaluation, and assignment APIs as
deliberately excluded, and flip the `(planned)` tags and statuses across FS-02, FS-14, FS-16, TS-01,
TS-02, TS-03, TS-04, TS-05, TS-09, and TS-10.

Noted for the reviewer, not blocking: FS-14 is marked `Partial` but carries no `(planned)` item — it
satisfies the lint only through a prose mention of the tag. That predates this change; either its
status is wrong or an unshipped requirement is untagged.
