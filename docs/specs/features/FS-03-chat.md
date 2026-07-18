# FS-03 — Live chat & permission flow

**Status:** Current
**Code:** `internal/runtime/` (`chat.go`, `permission.go`, `event.go`), `internal/server/sessions.go`, `internal/transcript/`, `ui/src/components/chat/`, `ui/src/store/transcriptStore.ts`, `ui/src/api/sse.ts` · **Journeys:** J3, J4, J7
**Absorbed:** exact source mapping in the [phase archive manifest](../../archive/phases/README.md)

## 1. Purpose

AgentDeck gives each chat-interface agent a live conversation surface over the Agent Client
Protocol (ACP). A user can send a prompt, watch assistant text and tool activity stream, decide
permission requests, cancel work, and reopen the durable transcript after a reload, restart, stop,
or resume. This spec governs the user-visible chat panel, normalized transcript events, prompt and
cancel controls, and permission decisions. Agent lifecycle and runtime switching are governed by
FS-01; archive/search and inactive-session viewing by FS-05; terminal input by FS-07.

## 2. Behavior

Requirements are user- and API-observable. R-item numbering is continuous through §4.

### 2.1 Opening and reading a chat

- **R1.** Opening a chat-interface agent shows its name, backend, model, context usage, and a
  Transcript tab. The same surface also exposes the session's Files and Commands tabs (FS-05).
- **R2.** The Transcript renders normalized events in sequence: user prompts; assistant text as sanitized
  GitHub-flavored Markdown with syntax-highlighted fenced code; tool calls with expandable JSON
  arguments; tool results with error styling and expandable content after the first 600 characters;
  unified file diffs; permission prompts; turn errors; turn boundaries; and backend-switch dividers.
  An unknown event remains inspectable as formatted JSON rather than disappearing.
- **R3.** The transcript follows new events while the reader remains at the bottom. Scrolling away
  suspends auto-follow and exposes **Jump to latest**, so streaming output does not take control of
  the reader's scroll position.
- **R4.** Consecutive `assistant_text` deltas are folded into one rendered response both while they
  arrive live and when a durable transcript is replayed. A `permission_resolved` event is folded
  into its matching `permission_request`, which then renders an Approved or Denied chip instead of
  active decision buttons.
- **R5.** One malformed or unrenderable event is isolated by an event-level error boundary; the
  remainder of the transcript stays usable and the failed item displays a fallback.

### 2.2 Prompting and turn state

- **R6.** On an idle chat agent, submitting non-whitespace text through the composer calls
  `POST /api/sessions/{id}/prompt` with `{text}`. Enter submits and Shift+Enter inserts a newline.
  The accepted response is `202` with `{accepted:true, agent_id}`.
- **R7.** The composer immediately displays the submitted user text for the current browser view
  and clears its draft. On acceptance, the runtime emits a sequenced `user_text` event that replaces
  the optimistic bubble and joins the durable transcript. If delivery fails, it shows an actionable
  error and restores the draft so the user can retry; the optimistic bubble remains visible and is
  not presented as server-acknowledged.
- **R8.** A prompt moves the agent synchronously to `busy`. ACP output is emitted as ordered
  transcript events and a final `turn_end`; successful completion returns the agent to `idle`,
  clears `busy_since`, and updates context usage. A runtime/protocol failure emits an error and
  turn end, marks fatal crashes `error`, and removes the dead running process.
- **R9.** While the agent is `busy` or `waiting_input`, the Send control becomes Cancel.
  `POST /api/sessions/{id}/cancel` returns `202 {cancelled:true}` when it claims an active turn or
  pending permission, and `202 {cancelled:false}` when the agent is already idle. Cancellation
  resolves a pending permission without executing its tool and terminates the turn with reason
  `cancelled`; if the cooperative ACP cancel does not finish, the runtime escalates to process
  interrupt.

### 2.3 Streaming and recovery

- **R10.** Live transcript events arrive on the multiplexed `/api/events` Server-Sent Events (SSE)
  stream as `new_message` envelopes keyed by `agent_id`. Each runtime event carries a per-agent
  monotonic `seq`, event `type`, timestamp, and type-specific `data`; the UI normalizes that nested
  wire shape before rendering.
- **R11.** `GET /api/sessions/{id}/transcript` returns `{agent_id, events}` from the durable
  transcript. `since_seq` returns only later events; `include_meta=true` includes session metadata.
  `events` is always an array. Unknown agents return `404`; a non-integer `since_seq` returns
  `422 validation`.
- **R12.** The chat panel fetches the transcript when opened and whenever its SSE connection
  reopens. If it detects a sequence gap for the currently-open agent, it refetches the authoritative
  transcript rather than appending a possibly incomplete delta stream. Permission decisions fold
  identically on live append and full replay.
- **R13.** Every accepted user prompt and runtime event delivered before a crash is appended to the
  per-agent transcript before it is published to the browser. A mid-turn crash therefore preserves
  both sides of the conversation plus already-delivered assistant/tool output for reload, archive,
  and later resume.

### 2.4 Permission decisions

- **R14.** When an ACP tool call requires approval and skip-permissions is false, AgentDeck emits a
  `permission_request`, sets the agent to `waiting_input`, and withholds the tool until the user
  chooses Approve or Deny, cancellation claims the turn, or the permission timeout expires.
- **R15.** Approve and Deny call `POST /api/sessions/{id}/permission` with `tool_call_id` and
  `decision`. Approve permits the selected ACP option and the tool may execute; Deny selects the
  rejection option and the tool does not execute. A successful response records
  `permission_resolved` in the transcript and returns the prompt to its resolved state.
- **R16.** Permission resolution is single-winner. Concurrent approve, deny, cancel, or timeout
  attempts cannot execute both outcomes. Re-deciding an already-resolved request returns `409`;
  an unknown/no-longer-pending tool call returns a typed failure rather than fabricating success.
- **R17.** An unanswered permission request auto-denies after the configured timeout, emits the
  error `permission timed out`, and finishes the turn without executing the tool.
- **R18.** When the frozen launch policy enables skip-permissions, a permission request is recorded
  as auto-approved, the agent never enters `waiting_input`, and the tool proceeds without a user
  click. Resume and switch retain that frozen policy under FS-01.

## 3. States & transitions

- **Open/reload:** panel fetches durable events → normalizes/folds them → subscribes to live SSE
  deltas. SSE reconnect or a detected seq gap repeats the authoritative fetch.
- **idle → busy:** a prompt or coordination nudge starts a turn and resets that turn's coordination
  budget (FS-06).
- **busy → waiting_input → busy:** a permission request pauses the ACP tool call; approve/deny
  atomically resolves it and lets the turn continue. Timeout or Cancel ends the turn instead.
- **busy → idle:** normal `turn_end`; context percentage and status detail are updated.
- **busy/waiting_input → error:** fatal transport/process failure emits the delivered error/turn-end
  events, removes the running row, and preserves the transcript.
- **stopped/resumed:** the durable transcript stays under the same `agent_id`; FS-01 and FS-05
  govern restoring the runtime and upstream ACP session.

## 4. Edge cases & errors

- **R19.** Empty or whitespace-only prompts are not sent. Invalid JSON or empty prompt bodies at
  the API boundary return `422 validation`; an unknown/non-running agent returns the applicable
  `404`/typed runtime error.
- **R20.** The UI sanitizes assistant Markdown before inserting it into the DOM; transcript content
  cannot inject arbitrary HTML or script through Markdown rendering.
- **R21.** A permission decision must be exactly `approve` or `deny`; invalid JSON or any other
  decision returns `422 validation`. A failed UI decision leaves the buttons available and shows an
  error instead of optimistically resolving the prompt.
- **R22.** A terminal-interface agent does not use this composer or ACP permission relay. Its panel
  opens on Terminal and directs input/permission handling to the live terminal (FS-07).

## 5. Acceptance criteria

- **A1** (R6, R8, R10) — Prompting streams ordered assistant deltas, finishes with `turn_end`, and
  transitions busy→idle with context usage: `internal/runtime/chat_test.go::TestChatStreamText`.
- **A2** (R2, R8) — Tool call, correlated result, and file diff survive ACP normalization:
  `internal/runtime/chat_test.go::TestChatToolFlow`.
- **A3** (R14–R16) — End-to-end HTTP prompt → permission request → approval → tool execution →
  durable transcript: `internal/server/integration_test.go::TestLaunchPromptPermissionFlow`.
- **A4** (R14–R18, R21) — Approve, deny, timeout, auto-approve, unknown call, and single-winner
  resolution: `internal/runtime/permission_test.go::TestPermissionApprove`,
  `TestPermissionDeny`, `TestPermissionTimeout`, `TestPermissionSkip`,
  `TestPermissionUnknownToolCall`, `TestTakePendingSingleWinner`, and
  `TestTakePendingReportsAlreadyResolved`; server mapping in
  `internal/server/server_test.go::TestPermissionErrorAlreadyResolved`.
- **A5** (R9) — Cancel claims a pending permission, prevents tool execution, records a cancelled
  turn, and becomes a no-op once idle: `internal/runtime/permission_test.go::TestCancelDuringPendingPermission`.
- **A6** (R4, R10–R12) — Nested wire events normalize, assistant deltas fold on live append and
  replay, and permission resolutions fold on both paths: `ui/src/store/transcriptStore.test.ts`.
- **A7** (R7, R13) — Accepted user prompts and delivered partial output remain in both the transcript
  endpoint and NDJSON after a mid-turn process crash:
  `internal/server/integration_test.go::TestCrashMidTurnPersistsDeliveredTranscript`.
- **A8** (R1–R18) — A user launches a fake-ACP chat agent, sends a prompt, observes streaming and
  status transitions, and completes approve/deny/timeout without a stuck prompt: journeys **J3**
  and **J4** in `docs/features/USABILITY-REVIEW.md`.

## 6. Deviations & open decisions

- **Concurrent transcript refetches have no ordering token.** Open, reconnect, and gap-repair
  fetches can overlap; the last promise to resolve replaces the store even if it contains an older
  maximum seq. The next live event/refetch can self-heal, but a slow stale response can temporarily
  regress the visible transcript. Tracked advisory; compare max seq or generation before applying.
- **Transcript-load failure is silent in the panel.** The initial `getTranscript` rejection is
  swallowed, leaving an empty transcript until a later SSE event/refetch. Prompt, cancel, and
  permission mutation failures are surfaced as required above; initial history-load diagnostics are
  an open UX gap.

## 7. Traceability

- **Runtime/events:** `internal/runtime/chat.go` (`Start`, `Resume`, `SendPrompt`, `Cancel`, `emit`,
  ACP update normalization), `internal/runtime/event.go`, `internal/runtime/permission.go`.
- **HTTP/persistence:** `internal/server/sessions.go` (`handlePrompt`, `handleTranscript`,
  `handleCancel`, `handlePermission`), `internal/transcript/writer.go`, `reader.go`.
- **UI:** `ui/src/components/chat/ChatPanel.tsx`, `Composer.tsx`, `TranscriptView.tsx`,
  `renderers/`, `ui/src/store/transcriptStore.ts`, `ui/src/api/sse.ts`.
- **Key regression tests:** `TestChatStreamText`, `TestChatToolFlow`,
  `TestLaunchPromptPermissionFlow`, `TestPermissionApprove`, `TestPermissionTimeout`,
  `TestCancelDuringPendingPermission`, `TestCrashMidTurnPersistsDeliveredTranscript`, and
  `ui/src/store/transcriptStore.test.ts`.
