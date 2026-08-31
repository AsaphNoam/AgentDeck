# FS-17 — Agent-facing action contract

**Status:** Partial
**Code:** `internal/messaging`, `internal/server`, `internal/cli`, `internal/agentknowledge` · **Journeys:** —
**Absorbed:** —

## 1. Purpose

AgentDeck's agent-facing MCP tools already answer in machine-readable JSON with stable outcome
codes, and already express waiting as durable prerequisite arms rather than polling
(FS-16.R5, FS-16.R12). Two gaps remain in how that surface presents itself to an agent.

A refused call does not say whether the refusal is permanent, argument-dependent, confined to the
caller's current turn, or transient, so a model has to infer retry behavior from an English
sentence — and gets it wrong in the one case that costs something, re-sending over an exhausted
per-turn message budget. Separately, every result is delivered as JSON inside a text block, so each
client re-parses a string that the protocol can carry as structured data.

This spec owns the cross-cutting shape shared by every agent-facing tool result. The individual
tools remain owned by FS-06 (messaging), FS-14 (pipeline reporting), FS-15 (context links), and
FS-16 (tasks); this spec adds no tool and changes no tool's arguments, authority, or effect.

The planned replacement in R13–R19 removes MCP only as AgentDeck's internal action-delivery
mechanism. It preserves the action vocabulary and domain behavior while moving exact invocation
detail to an on-demand AgentDeck command. User- and provider-configured MCP servers remain supported.

## 2. Behavior

### 2.1 Retry classification

- **R1 — Every refusal classifies its retry behavior.** An agent-facing MCP tool result
  that reports a refusal carries a `retry` object beside its existing `error` code and `message`.
  `retry.class` holds exactly one value from the closed vocabulary in R2. The classification is
  advisory: it never gates a call, never obligates the host, and never changes what the refused call
  did or did not do.

- **R2 — The four classes have fixed meanings.**
  - `never` — the identical call from the same caller will not succeed. Caller errors, closed
    states, and authorization refusals.
  - `after_change` — the same call can succeed with different arguments. Any repair fields the
    refusal already carries, such as `candidates`, say how.
  - `next_turn` — the call was well-formed and was refused only for the caller's current turn.
    Retrying it within that turn fails again.
  - `transient` — a host-side read or dependency failed. A later identical call may succeed, and
    there is nothing durable to wait on.

- **R3 — Every refusal code declares one class.** Each outcome code an agent-facing tool
  can emit maps to exactly one class:

  | Class | Codes |
  |---|---|
  | `never` | `validation`, `invalid_body`, `invalid_subject`, `invalid_outcome`, `invalid_state`, `invalid_cursor`, `dependency_cycle`, `target_ineligible`, `already_reported`, `not_assigned`, `not_creator`, `retry_requires_rearm`, `task_not_found`, `context_not_found`, `context_source_unavailable`, `proposal_forbidden`, `session_unknown`, `assignment_unknown`, `stale_assignment` |
  | `after_change` | `ambiguous_recipient`, `recipient_not_found`, `source_unavailable`, `validation_failed` |
  | `next_turn` | `message_budget_exceeded` |
  | `transient` | `internal`, `store_unavailable`, `context_unavailable`, `pipeline_unavailable` |

  A refusal keeps every field it carries today; classification only adds `retry`.

- **R4 — Classification never discloses other work and never asks an agent to poll.** No
  `retry` value names a task, pipeline run, agent, message, or context reference that the caller is
  not already a participant in, and no class directs an agent to re-call a tool on a schedule.
  Waiting on other work remains expressible only as a prerequisite arm at task creation
  (FS-16.R5), which AgentDeck resolves without a model in the loop (FS-16.R6). This spec adds no
  condition language, no wait tool, and no task-graph query.

- **R5 — A successful result carries no retry field.** `retry` appears only on a refusal.

### 2.2 Result delivery

- **R6 — Every tool result is delivered as structured content and as text.** Each
  agent-facing tool result, successful or refused, populates the MCP `structuredContent` field with
  the same value it renders into its text content block.

- **R7 — Structured delivery is additive and never the only channel.** The delivery change adds
  nothing to the text content block after R1's `retry` field is applied, and does not change
  `isError`. `structuredContent` never carries a field the text block lacks and never omits one it
  has, so the two can never disagree and a client that reads only text behaves exactly as before.
  No tool declares an output schema in this change.

- **R8 — The contract covers the whole agent-facing surface.** R1–R7 apply to every tool
  registered on AgentDeck's single MCP authority — the messaging, pipeline, context, and task tools
  named by TS-04.R6, R17, R28, and R29 — and to any tool added to it later.

### 2.3 Direct AgentDeck actions

- **R13 (planned) — AgentDeck actions use one packaged command instead of an internal MCP
  server.** Every fresh, resumed, or switched chat agent can invoke the existing action identifiers
  through one `agentdeck action` command family: `list_agents`, `send_message`, `check_messages`,
  `report_pipeline_stage_result`, `propose_pipeline_template`, `propose_pipeline_run`,
  `get_assigned_task`, `create_task`, `cancel_task`, `report_task_result`, `share_context`,
  `list_context_links`, `read_context_link`, `set_context_link_visibility`, and
  `revoke_context_grant`. AgentDeck no longer registers or advertises its own
  `agentdeck-messaging` MCP server to those agents. This changes delivery only: each action keeps the
  arguments, authority, effects, limits, and domain outcomes its owning FS defines.

- **R14 (planned) — Invocation and results are machine-composable.**
  `agentdeck action <action-name> --input -` accepts exactly one JSON object from standard input and
  writes exactly one JSON object to standard output; no-input actions may omit `--input -`, and
  `agentdeck action describe <action-name>` returns that action's exact contract on demand. A
  successful action exits successfully; a refused, invalid, unauthenticated, or unavailable action
  exits non-zero while still returning its structured result when AgentDeck can form one. Progress,
  diagnostics, provider output, and logs never mix into standard output. Empty collections remain
  arrays, and shell quoting is not part of any action's data model.

- **R15 (planned) — Exact mechanics are disclosed only when needed.** The shared
  `operating-agentdeck` skill explains when to choose messaging, tasks, context links, or pipelines
  and directs an agent to action-specific command help for exact input fields, limits, effects, and
  result fields. AgentDeck does not inject the complete action catalog and all fifteen input schemas
  into every conversation. A mail activation names the action that reads mail; a task activation
  names the action that reads the assignment; a pipeline assignment names the result action. An
  autonomously activated agent therefore has an exact next step without loading unrelated action
  definitions.

- **R16 (planned) — Caller authority remains runtime-derived.** AgentDeck derives the stable agent
  id and current launch generation from a fresh runtime credential, never from an action name,
  input field, environment claim, working directory, role text, or provider session id. Stop,
  crash, switch, and dashboard shutdown revoke that generation's authority. An earlier generation
  cannot send mail, create or cancel work, read context, propose a pipeline, or report a task or
  stage result after a later generation starts. Every FS-06, FS-14, FS-15, and FS-16 authorization,
  atomicity, wake, budget, lifecycle, retention, and recovery rule remains unchanged.

- **R17 (planned) — The result contract becomes transport-neutral.** R1–R5 and R9–R11 apply to
  direct action results unchanged. The JSON object on standard output replaces R6–R8's MCP text and
  `structuredContent` duplication and is the one authoritative representation. Invalid JSON,
  unknown fields, missing required fields, and wrong field types are rejected by AgentDeck with a
  stable error code, message, and retry classification rather than a provider- or protocol-owned
  plain-text schema error. A result that cannot be encoded is an internal refusal; it is never
  replaced by prose or reported as success.

- **R18 (planned) — The replacement is one portable chat capability.** Claude, Codex, OpenCode,
  and OpenHands chat agents use the same command, action identifiers, input objects, output objects,
  and authority rules. AgentDeck does not add provider-specific custom functions or retain MCP for
  one provider in released behavior. The internal MCP removal cannot ship until every supported
  chat adapter can invoke the packaged command and consume its structured success and refusal
  results across fresh launch, resume, and switch. Terminal agents remain outside the action surface,
  matching their current coordination boundary.

- **R19 (planned) — Cutover is complete before release.** MCP and the direct command may coexist
  only inside the unreleased implementation while their behavior is compared. The completed change
  exposes only the direct action command to AgentDeck agents and retains no released compatibility
  mode, preference, feature flag, or per-provider fallback for the internal MCP. Existing AgentDeck
  mail, tasks, context references, grants, pipeline runs, proposals, transcripts, and sessions need
  no migration and keep their current retention. User- and provider-configured MCP servers,
  provider MCP configuration federation and inventory, and MCPs unrelated to AgentDeck's internal
  actions are explicitly unchanged.

## 3. States & transitions

None. `retry` is derived per call from the refusal reason and is never persisted, replayed, or
counted. No row, migration, notification, activation, transcript event, or SSE payload changes.

## 4. Edge cases & errors

- **R9 — An unclassified refusal is `transient`.** A refusal whose code is not in R3's
  table classifies as `transient`, because the realistic unmapped case is an internal or storage
  failure. Shipping an agent-facing refusal code that R3 does not list is a defect, not a
  supported state.

- **R10 — An unknown session is `never`, not `transient`.** `session_unknown` means the
  token was never issued or its agent generation ended. Neither is repaired by retrying within that
  session, so it does not invite one.

- **R11 — Classification carries no retry budget.** `retry` never includes a count, a
  delay, a backoff, a deadline, or a timestamp. AgentDeck does not track how many times a caller
  retried and does not refuse a call for having retried.

- **R12 — A result that cannot be structured still answers.** If a result value cannot be
  encoded as a JSON object, the call still returns its text content block and its outcome code
  unchanged, and omits `structuredContent` rather than failing the call.

## 5. Acceptance criteria

Each names the verification that demonstrates it.

- **A1** (R1–R3, R5) — The classifier table maps every code in R3 to its declared class; a
  source-derived guard rejects unclassified handler refusal literals, and a real call to every
  registered tool verifies the shared `session_unknown` refusal contract. Successful-result helper
  coverage verifies that success adds no `retry`: tests under `internal/messaging`.
- **A2** (R3, R9) — A guard test fails if an agent-facing refusal code exists that R3's table does
  not classify, and an injected unlisted code classifies `transient`: classifier unit test under
  `internal/messaging`.
- **A3** (R4, R11) — The shared refusal helper adds exactly `retry: {class}` and no identifier,
  count, delay, backoff, deadline, or timestamp; representative next-turn and pipeline-control
  refusals preserve only their pre-existing payload fields: exact-object helper tests under
  `internal/messaging`.
- **A4** (R6–R7) — Shared delivery-helper golden tests cover one successful and one refused result;
  registration-derived real refused calls verify identical text and structured values with
  unchanged `isError` for every tool, and `tools/list` verifies that none advertises an output
  schema: protocol tests under `internal/messaging`.
- **A5** (R8, R10, R12) — Every tool registered on the MCP server appears in A4's refused-call
  enumeration rather than a hand-written surface list, `session_unknown` classifies `never`, and an
  injected unencodable result returns its text block and code with `structuredContent` absent:
  registration-derived tests under `internal/messaging`.
- **A6** (R6–R7) — A pinned Claude and a pinned Codex adapter each complete a normal tool call and a
  refused tool call against a live AgentDeck with structured content enabled, and neither rejects
  the result nor loses the text block: manual gate, run with the other pinned live-provider checks.

- **A7 (planned)** (R13–R17) — Every one of R13's fifteen identifiers is invoked through the
  packaged command with a representative success and refusal. A transport-neutral golden matrix
  proves the command reaches the same domain operation and produces the same durable rows, state
  transitions, events, bounded result fields, stable error codes, and retry classes as the frozen
  pre-migration MCP baseline. Invalid JSON and schema-invalid input return AgentDeck-owned structured
  refusals and mutate nothing. *Verify:* command, protocol, and domain integration tests.

- **A8 (planned)** (R15–R16) — Fresh launch, ordinary resume, stopped-agent mail wake, task
  activation, pipeline stage launch/Continue, and same- and cross-backend switch each receive one
  working current-generation action identity. An old, missing, malformed, or revoked identity is
  rejected without mutation; every failed composition and every stop/crash/switch cleanup leaves no
  usable authority behind. Captured process parameters, frozen session rows, generated provider
  configuration, logs, and transcripts contain no credential. *Verify:* lifecycle composition,
  teardown, restart, and adversarial authorization matrix.

- **A9 (planned)** (R13–R15, R18) — Credentialed pinned Claude, Codex, OpenCode, and OpenHands chat
  sessions each discover and invoke one success and one refusal through the packaged command after a
  fresh launch and resume; Claude↔Codex switching retains the same stable agent identity with a fresh
  generation. Mail and task activations follow their named action without human repair, and a
  pipeline agent reports its result once. No provider receives an internal AgentDeck MCP
  registration. *Verify:* release-blocking live-provider gate plus the fake-ACP lifecycle matrix.

- **A10 (planned)** (R18–R19) — The completed build has no internal `/mcp` route, generated
  AgentDeck MCP registration/config artifact, `agentdeck-messaging` reserved-name collision, or MCP
  SDK dependency, while configured external MCP definitions still appear in federation/inventory
  and still reach their provider-owned launch configuration unchanged. Terminal behavior is
  byte-for-byte unchanged. *Verify:* route, dependency, configuration-source, launch-parameter, and
  distributable-package tests.

- **A11 (planned)** (R15, R19) — A deterministic measurement records the serialized internal MCP
  catalog and input schemas used by the pre-migration baseline, then proves the completed launch and
  resume send none of those definitions. The change records command invocation latency and provider
  success for the same representative actions, so the claimed context reduction is measured and no
  material execution regression is hidden. *Verify:* checked-in measurement fixture and the A9 live
  gate.

## 6. Deviations & open decisions

The contract is shipped. Live-provider compatibility remains tracked as acceptance gate A6.

- The idea that prompted this work asked for a structured `retry_when` condition an agent could
  wait on. Tracing every refusal the MCP layer emits found none that a durable wait would repair:
  `create_task` does not refuse a busy target — it records the task and lets the dispatcher park it
  (FS-16.R6) — and `target_ineligible` is a permanent property of the target, not a busy signal. The
  shipped way to wait for other work is already a prerequisite arm. `retry` is therefore an object
  rather than a flat string, so a genuine condition can be added later as a sibling field without a
  breaking rename, but no such condition is specified here.
- Agent-callable re-arm and retry, task or agent state inspection, and agent-callable stop, resume,
  or launch remain out of scope and remain recorded in `../../ideas.md`. The exclusion of a
  task-graph query (FS-16 §6, TS-04.R29) is deliberately upheld by R4.
- Output schemas are not declared. The pinned Claude and Codex adapters' handling of a tool
  `outputSchema` is unverified, and declaring one later is additive.
- Argument shapes rejected by the pinned MCP SDK before an AgentDeck handler runs are outside
  R1–R8. The SDK returns its own plain-text `isError` result for schema-validation and decoding
  failures, so those results have neither AgentDeck's stable error code and retry class nor
  `structuredContent`. Handler-produced validation refusals remain fully covered by this contract.
- No HTTP surface changes. The REST error envelope and its mixed legacy forms (TS-03.R3) are
  untouched; this contract is the MCP tool surface only.
- **Confirmed direct-cutover boundary (planned).** R13–R19 replace only AgentDeck's internal action
  MCP with the packaged command and its private local transport. There is no released dual-transport
  window because AgentDeck currently has one active operator who can validate the cutover directly;
  parity and rollback exist during implementation instead. General MCP support, provider-native MCP
  configuration, terminal capability, domain data, and the public local API remain unchanged.

## 7. Traceability

Anchors: the single MCP authority and its shared result helpers in `internal/messaging`
(`tools.go` `jsonResult`/`errResult`); tool registration in `internal/messaging/messaging.go`;
contract coverage in `internal/messaging/tool_result_contract_test.go`;
refusal sources in `internal/messaging/task_tools.go`, `context_tools.go`, and `pipeline_tools.go`,
plus the control-plane `ToolError` codes raised in `internal/server/task_handlers.go`. The
protocol-level structured-content field is `CallToolResult.StructuredContent` in the pinned
`github.com/modelcontextprotocol/go-sdk/mcp`; it is set on the result directly, so no handler
signature and no output schema changes. Governing technical requirements: TS-04.R30–R31.
