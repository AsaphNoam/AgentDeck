# Usability review run — Create with AgentDecker (2026-08-01)

**Journey:** J14, "Create with AgentDecker" slice only (FS-14 §4.2 R25–R30, A10).
**Trigger:** human reported "the pipeline creation with AgentDecker flow is broken."
**Method:** static trace of the full flow plus the ACP ingestion that feeds it. The credentialed
round-trip was **not** run — see coverage gap below.

## Coverage gap (why green tests miss this)

The one journey that would prove the proposal surface end-to-end (launch builder → AgentDecker calls
`propose_pipeline_template` → panel renders → Save/Start approval) is **not exercisable with the
shipped fixtures**: `fakeacp` (`internal/runtime/testdata/fakeacp/main.go`) can only emit *display*
`tool_call` updates; it cannot issue a real `propose_pipeline_template`/`propose_pipeline_run` MCP
call back to AgentDeck's messaging server. So neither the UI unit tests (MSW-mocked transcripts) nor
the server integration tests (`fakeacp`) ever observe a real MCP-tool-named transcript event.
Confirming finding 1 needs a live `claude-agent-acp`/`codex-acp` adapter.

## Path traced

`AgentDeckerBuilder.tsx` (setup form → `launchAgent(role: "agentdecker", interface: "chat")` →
`sendPrompt` → `navigate(/agent/{id})`) → transcript store → `extractPipelineProposals(events)` →
"Pending exact proposals" panel → `onTemplateProposal`/`onRunProposal` (in `PipelinesPage.tsx`) →
`TemplateEditor` / `RunStartForm`. Server side: `internal/messaging/messaging.go:137-144` registers
the bare tool names; `internal/messaging/pipeline_tools.go` returns `{ok, proposal}`. Ingestion:
`internal/runtime/acpmap.go` maps ACP `tool_call`/`tool_call_update` to transcript events.

## Findings (severity-mapped)

### Finding 1 — MAJOR / Must fix (confirmed root cause; human reproduced "didn't see it")

The display path discards the proposal because it gates on the tool-call **name** before inspecting
the result. `extractPipelineProposals` (`AgentDeckerBuilder.tsx:17-40`) builds a `toolCallID → name`
map from `tool_call` events and then, for a `tool_result`, `continue`s unless the mapped name is
exactly `propose_pipeline_template`/`propose_pipeline_run`. Two facts make that gate fail in the real
app:

1. `event.name` is never the tool name — ingestion sets it via `toolName(u) = FirstNonEmpty(kind,
   title, "tool")` (`acpmap.go:218`), and ACP `tool_call` has no machine tool-name field, only
   `title`/`kind`/`toolCallId` (`acpmap.go:29-31`). For an MCP tool `kind` is a category (`"other"`),
   so `name` = `"other"`.
2. Detection therefore depends entirely on the adapter setting `title` to the literal
   `propose_pipeline_template` — which it does not guarantee. Any other label → gate rejects the
   result unread → panel stays empty. This is exactly the human's observation.

The gate is also unnecessary. The result content is self-identifying: `jsonResult` emits
`{"ok":true,"proposal":{proposal_id,kind,digest,payload}}` (`internal/messaging/tools.go:25-31`,
`internal/pipeline/proposals.go:9-14`), and `pipelineProposalSchema` already validates that exact
shape. No other tool returns `{ok, proposal}`, so parsing every `tool_result` content against the
schema is sufficient and safe — no tool-name label needed. The store already flattens the wire
`{type, data:{...}}` envelope into top-level fields via `normalizeEvent`
(`ui/src/store/transcriptStore.ts`), so `event.content`/`event.tool_call_id` reads are correct; the
name-gate is the sole defect. The unit test hard-codes `title: "propose_pipeline_template"`
(`AgentDeckerBuilder.test.ts:29`), so it stays green regardless.

**Fix direction:** remove the `toolNames` name-gate; iterate `tool_result` events, parse `content`
via the existing `jsonCandidates`, accept any satisfying `parsed.ok === true &&
pipelineProposalSchema.safeParse(parsed.proposal).success`. **Verify secondary dependency:** the fix
only helps if the adapter forwards the MCP result JSON into `tool_call_update.content` — confirm
against the real `claude-agent-acp`/`codex-acp` adapter and add a J14 fixture that issues a real MCP
call.

### Finding 2 — MAJOR / Must fix (confirmed statically)

After launch the flow navigates to the chat and strands the user: the proposal review/approval
surface exists only on `/pipelines`. `launchBuilder` ends with `navigate(/agent/${agentID})`
(`AgentDeckerBuilder.tsx:148`), but the "Pending exact proposals" panel and the
`onTemplateProposal`/`onRunProposal` → `TemplateEditor`/`RunStartForm` handoff live only inside
`PipelinesPage.tsx`. No chat/transcript component references pipelines or proposals
(`ui/src/components/chat/ChatPanel.tsx` has none); the only routes to `/pipelines` are the global
header NavLink (`ui/src/components/shell/Header.tsx:13`) and the agent-card run link. A first-time
user has no in-context path from the chat back to their pending proposal.

**Fix direction:** surface a "proposal ready — review in Pipelines" affordance in the builder
session/chat (deep-link to the pending proposal), or don't navigate away from Pipelines on launch.

### Finding 3 — MINOR / Worth fixing

A proposal emitted just before the builder stops becomes unreachable. The transcript refetch effect
is gated on `builderRunning` (`AgentDeckerBuilder.tsx:112-118`) and a stopped builder clears its
persisted id and drops the proposals panel (`:120-124`), so a proposal made right before the process
ends disappears with no record. Compounds finding 2.

**Fix direction:** persist/refetch the last transcript for a recently-stopped builder, or list
pending proposals from durable server state rather than only the live transcript.

## Not run

- Real-browser J14 Create-with-AgentDecker round-trip (no fixture can issue the MCP call).
- Manual/Save/Start confirmation-after-edit sub-journey (blocked by the same fixture gap).
