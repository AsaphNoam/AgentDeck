# Tell agents whether a refused tool call is worth retrying

**State:** Waiting to start
**Why:** From the "Richer agent-facing orchestration API" idea in `docs/ideas.md`. The user scoped it
down to the two gaps that survived investigation: typed retry classification on refusals, and
structured result delivery. Agent-side re-arm/retry, task/agent state inspection, and agent-callable
lifecycle control were explicitly deselected and remain in `docs/ideas.md`.
**Relevant requirements:** FS-17.R1–R12, FS-17.A1–A6, TS-04.R30–R31, INV §2, INV §7, INV §8

## Outcome

An agent that gets refused by an AgentDeck MCP tool can tell, without reading English, whether the
refusal is permanent, fixable by changing its arguments, confined to the current turn, or transient.
The concrete win is the per-turn message budget: today an agent that exhausts it is told "This
message was not sent" and re-sends inside the same turn, which cannot succeed. Every tool result also
arrives in MCP's `structuredContent` field, so clients stop re-parsing JSON out of a text block.

## Included work

Included:

- A `retry` object on every agent-facing MCP refusal, with a closed four-value `class`
  (`never`, `after_change`, `next_turn`, `transient`) — FS-17.R1–R3, R5.
- One classifier table in `internal/messaging` owning the code-to-class mapping, applied by the
  shared refusal helper, with a guard test that derives the code set from the emitting call sites —
  TS-04.R30, FS-17.R9.
- `mcp.CallToolResult.StructuredContent` populated by `jsonResult`/`errResult` with the same value
  they marshal into the text block — TS-04.R31, FS-17.R6–R8.

Not included:

- No new tool, no changed tool argument, no changed tool authority or effect.
- No condition language and no armable `retry_when`. Investigation found no refusal a durable wait
  would repair: `create_task` does not refuse a busy target (it records the task and the dispatcher
  parks it, FS-16.R6), and `target_ineligible` is a permanent property of the target
  (`internal/server/task_handlers.go:570-577`), not a busy signal. The shipped way to wait on other
  work is a prerequisite arm.
- No task-graph query or state inspection. FS-16 §6 and TS-04.R29's anti-polling exclusion is
  upheld by FS-17.R4.
- No agent-callable re-arm, retry, stop, resume, or launch. `POST /api/tasks/{id}/rearm` and
  `/retry` stay HTTP-only.
- No declared tool `outputSchema`. Because every handler's typed output parameter is the empty
  interface, the pinned go-sdk derives no output schema and `StructuredContent` is set on the result
  directly — verified against `mcp.ToolHandlerFor` and `mcp.CallToolResult` in the pinned SDK. Adding
  schemas later is additive and waits on live-adapter verification.
- No HTTP, SSE, UI, persistence, or migration change.

## How we will know it works

- FS-17.A1 — table-driven MCP tests: every refusal code in FS-17.R3, produced by a real call to the
  tool that emits it, returns its declared class; successful calls carry no `retry`.
- FS-17.A2 — guard test fails if an agent-facing refusal code is unclassified; an injected unlisted
  code classifies `transient`.
- FS-17.A3 — no refusal payload names work the caller is not a participant in, and none carries a
  count, delay, or deadline.
- FS-17.A4 — golden-encoding tests: the text block stays byte-identical, `structuredContent`
  unmarshals to the identical value, `isError` is unchanged, and `tools/list` advertises no output
  schema.
- FS-17.A5 — the enumerations are derived from tool registration rather than hand-written;
  `session_unknown` classifies `never`; an unencodable result omits `structuredContent` and still
  answers.
- FS-17.A6 — manual gate: a pinned Claude adapter and a pinned Codex adapter each complete one
  successful and one refused tool call without rejecting the result or losing the text block. Runs
  with the other pinned live-provider checks recorded in `../features/HANDOFF.md`.

## Waiting on

Nothing. FS-17.A6 is a manual gate at the end of the change, not a precondition, and it joins the
live-provider acceptance already owed for FS-16.

FS-16/TS-10's implementation review findings are closed. The closing fix commits still need an
independent review, but this waiting change has no dependency on unresolved dependent-work findings.
