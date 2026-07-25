# Add configurable pipeline runs

**State:** Waiting to start
**Why:** The human asked for a native, reusable work/review/validation/fix pipeline whose stage
models can be swapped per run, then approved the resulting product and technical design. This
promotes the configurable-pipeline idea previously recorded in `docs/ideas.md`.
**Relevant requirements:** FS-14.R1–R30, TS-01.R11, TS-02.R17, TS-03.R16–R17,
TS-04.R17, TS-05.R14, TS-09.R1–R23, INV §2, INV §4–§11, INV §13–§15

## Outcome

A person can create a reusable model-neutral pipeline, assign a backend/model to each stage for a
specific run, and supervise durable sequential execution through explicit success, failure, blocked,
approval, retry, repair-loop, and restart states. AgentDecker can propose templates and run setups;
exact Save and Start actions remain separately human-approved.

## Included work

Add versioned template JSON and validation; SQLite run/attempt/value/idempotency state; the in-process
pipeline reconciler; shared lifecycle-service extraction; authenticated stage-result and
AgentDecker-proposal MCP tools; pipeline REST/SSE and thin CLI commands; pipeline associations on
ordinary agents; notifications; and the dedicated Pipelines page with manual and assisted creation,
run setup/history, approvals, controls, and transcript links.

Keep effort selection, terminal stages, hard API capability enforcement, parallel branches/joins,
child pipelines, typed/file artifacts, arbitrary conditions, worktree isolation, and automatic
AgentDecker Save/Start outside this change.

## How we will know it works

Implement and pass FS-14.A1–A10: template/run API and UI coverage; fake-runtime sequencing,
failure, identity, race, restart, and idempotency tests; persistence migrations with both SQLite test
variants; MCP spoof/stale-call tests; CLI contract tests; and the complete browser pipeline journey
including reversed Codex/Claude assignments, a blocked continuation, a repair loop, AgentDecker
proposal approval, shared-workspace warning, retained transcripts, and deletion behavior.

## Waiting on

None. The change is specified and approved, but has not been started.
