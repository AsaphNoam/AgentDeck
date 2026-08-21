# Separate orchestration state from model conversations

**State:** Waiting to start
**Why:** The human requested an explicit control/context/conversation architecture, selected
autonomous at-most-once mail activation, confirmed its failure/backlog/retention consequences, and
wants context links, dependency-aware agents, and a semantic orchestration API built immediately
afterward for the same release.
**Relevant requirements:** FS-00.R13–R15, FS-06.R24–R27/A13–A15, TS-01.R18–R21,
TS-02.R23, TS-04.R27, INV §1/§2/§4/§5/§6/§11/§15

## Outcome

Control facts no longer cause incidental model turns. New mail commits one durable, coalesced,
payload-free activation and may produce one intentional reasoning turn; unread state, polling,
restart, and failed/ignored prior work cannot repeat it. Mail remains durable and pull-based through
`check_messages`.

## Included work

- Add the payload-free activation migration/state API, atomic message-plus-activation writes,
  conservative legacy-pending backfill, mail-specific coalescing/at-most-once transitions, and
  startup/live cleanup. Activation remains a neutral opportunity concept; this change does not make
  mail's uniqueness or retry policy universal.
- Replace the unread-driven nudger and `Runtime.CheckMessages(pid)` with a server activation
  executor and an agent-id-keyed runtime turn gate that commits attempted/local turn state before
  wake/provider effects.
- Reuse the existing stopped-agent wake gates, exclusive lifecycle claim, frozen resume composition,
  scoped MCP identity, unread indicators, message retention, and turn budget.
- Add deterministic migration, coalescing, restart, provider-failure, manual-turn, lifecycle-race,
  mid-flight-mail, and no-payload fake-ACP coverage; update descriptive architecture docs.
- Do not add context items/links, dependencies, armed agents, new orchestration MCP tools, generic
  activation CRUD, an activation UI/history, parallel pipelines, or a workflow DSL. Those remain
  separately specified follow-on features even though they are planned for the same release. A
  future kind must define its durable source/work identity, coalescing key, typed start/outcome, and
  retry/completion policy before extending the schema or executor.

## How we will know it works

FS-06.A13–A15 pass against fake ACP, including one prompt for a coalesced mail batch, no mail replay
after an attempt or restart, later mail re-arming, stopped-agent race/failure behavior, and message
content appearing only through `check_messages`. State migration tests prove that partial
uniqueness, claim recovery/cleanup, and legacy backfill are explicitly mail-scoped rather than a
generic activation contract. The completed change passes
`make check-specs`, `make build`, `make test`, and `make dist`.

## Waiting on

Nothing.
