# FS-17 — Agent-facing tool result contract

**Status:** Current
**Code:** `internal/messaging` · **Journeys:** —
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
  | `never` | `validation`, `invalid_body`, `invalid_subject`, `invalid_outcome`, `invalid_state`, `invalid_cursor`, `dependency_cycle`, `target_ineligible`, `already_reported`, `not_assigned`, `not_creator`, `retry_requires_rearm`, `task_not_found`, `context_not_found`, `context_source_unavailable`, `proposal_forbidden`, `session_unknown` |
  | `after_change` | `ambiguous_recipient`, `recipient_not_found`, `source_unavailable` |
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

- **R7 — Structured delivery is additive and never the only channel.** The text content
  block keeps the encoding it has today, byte for byte, including on refusals and including
  `isError`. `structuredContent` never carries a field the text block lacks and never omits one it
  has, so the two can never disagree and a client that reads only text behaves exactly as before.
  No tool declares an output schema in this change.

- **R8 — The contract covers the whole agent-facing surface.** R1–R7 apply to every tool
  registered on AgentDeck's single MCP authority — the messaging, pipeline, context, and task tools
  named by TS-04.R6, R17, R28, and R29 — and to any tool added to it later.

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

- **A1** (R1–R3, R5) — Every refusal code in R3's table, produced by a real call to the tool that
  emits it, returns that code's declared class, while every successful call returns no `retry`
  field: table-driven MCP tests under `internal/messaging` that enumerate the emitted code set.
- **A2** (R3, R9) — A guard test fails if an agent-facing refusal code exists that R3's table does
  not classify, and an injected unlisted code classifies `transient`: classifier unit test under
  `internal/messaging`.
- **A3** (R4, R11) — No refusal payload in A1's enumeration contains a task, run, agent, message, or
  context reference identifier the caller is not a participant in, and none contains a count, delay,
  or deadline: assertions over the same table-driven enumeration.
- **A4** (R6–R7) — For one successful and one refused call per tool, the text content block is
  byte-identical to the encoding produced before this change, `structuredContent` unmarshals to the
  identical value, `isError` is unchanged, and no tool advertises an output schema in `tools/list`:
  golden-encoding and protocol tests under `internal/messaging`.
- **A5** (R8, R10, R12) — Every tool registered on the MCP server appears in A1's and A4's
  enumerations rather than a hand-written list, `session_unknown` classifies `never`, and an
  injected unencodable result returns its text block and code with `structuredContent` absent:
  registration-derived tests under `internal/messaging`.
- **A6** (R6–R7) — A pinned Claude and a pinned Codex adapter each complete a normal tool call and a
  refused tool call against a live AgentDeck with structured content enabled, and neither rejects
  the result nor loses the text block: manual gate, run with the other pinned live-provider checks.

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
- No HTTP surface changes. The REST error envelope and its mixed legacy forms (TS-03.R3) are
  untouched; this contract is the MCP tool surface only.

## 7. Traceability

Anchors: the single MCP authority and its shared result helpers in `internal/messaging`
(`tools.go` `jsonResult`/`errResult`); tool registration in `internal/messaging/messaging.go`;
contract coverage in `internal/messaging/tool_result_contract_test.go`;
refusal sources in `internal/messaging/task_tools.go`, `context_tools.go`, and `pipeline_tools.go`,
plus the control-plane `ToolError` codes raised in `internal/server/task_handlers.go`. The
protocol-level structured-content field is `CallToolResult.StructuredContent` in the pinned
`github.com/modelcontextprotocol/go-sdk/mcp`; it is set on the result directly, so no handler
signature and no output schema changes. Governing technical requirements: TS-04.R30–R31.
