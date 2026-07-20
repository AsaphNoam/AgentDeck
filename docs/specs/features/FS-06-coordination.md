# FS-06 — Agent coordination & notifications

**Status:** Partial
**Code:** `internal/messaging/`, `internal/state/messages.go`, `internal/server/` (`messaging_registration.go`, `messaging_loops.go`, `sessions.go`), `internal/bus/`, `ui/src/api/sse.ts`, `ui/src/components/grid/AgentCard.tsx`, `ui/src/components/shell/NotificationCenter.tsx`, `ui/src/features/settings/NotificationsEditor.tsx` · **Journeys:** J10, J11, J12
**Absorbed:** [`agent-dashboard-prd.md`](../../archive/agent-dashboard-prd.md) F8/F11 and the [phase archive manifest](../../archive/phases/README.md)

## 1. Purpose

Running chat agents can discover and message one another without the human relaying text. AgentDeck
hosts a Model Context Protocol (MCP) server in the dashboard process, binds each caller to its
registered agent identity, stores mail durably, wakes idle recipients, and caps per-turn messaging
so a tight loop cannot run away. This spec owns unread/sent indicators and production of the
coordination-specific `budget_exceeded` event. FS-02 owns notification preferences and presentation
for all notification types. The underlying data/protocol shapes belong to TS-02, TS-03, and TS-04,
and terminal agents' messaging boundary belongs to FS-07.

## 2. Behavior

Requirements are user-, agent-, and API-observable. R-item numbering is continuous through §4.

### 2.1 Messaging session and discovery

- **R1.** Every launched or resumed chat agent receives a reserved `agentdeck-messaging` MCP server
  entry pointing to the dashboard's loopback `/mcp` endpoint with a freshly-minted per-agent token
  in `X-AgentDeck-Token`. Stop, switch teardown, crash teardown, and server shutdown revoke the
  token and remove its owner-only generated MCP config.
- **R2.** The MCP server exposes exactly three coordination tools: `list_agents`, `send_message`,
  and `check_messages`. Calls with a missing, unknown, or revoked token fail as
  `session_unknown`; the caller identity is always derived from the token-bound session and cannot
  be supplied or spoofed through tool arguments.
- **R3.** `list_agents` returns currently-running agents with stable `agent_id`, display name,
  `role@project` address, interface, state, detail, and context usage. It excludes the caller by
  default, accepts `include_self`, and can filter by exact state. Terminal agents may appear for
  discovery but are not valid `send_message` recipients (R17).

### 2.2 Sending and reading mail

- **R4.** `send_message(to, body, subject?, in_reply_to?)` resolves a recipient in this order:
  exact `agent_id`, exact `role@project`, then case-insensitive display name. Only running
  chat-interface agents are addressable. No match returns `recipient_not_found`; multiple matches
  return `ambiguous_recipient` with candidates so the sender can retry by `agent_id`.
- **R5.** A message body is required and limited to 8,000 characters; an optional subject is
  limited to 200. On success the response returns `message_id`, resolved `to` agent id, and
  canonical `to_address`. The durable row records the session-derived sender id/address/name,
  recipient, subject/body, creation time, unread state, delivery marker, and optional reply id.
- **R6.** `check_messages` reads the caller's mailbox. Defaults are unread-only, mark returned
  messages read, do not delete, and limit 15. Callers may include read mail, retain unread state,
  delete returned rows, and request 1–50 messages. Results include sender identity, subject/body,
  timestamp and reply id plus unread `remaining` and turn-budget status.
- **R7.** When a mailbox query is limited, AgentDeck returns the newest N matching messages in
  newest-first order, so old mail is discarded from the page before recent mail. The dashboard's
  `GET /api/sessions/{id}/messages` view follows the same order, defaults to 50, caps at 200,
  accepts `unread_only=true`, and returns `unread_count`; unknown agents return `404` and invalid
  limits return `422 validation`.
- **R8.** Messages survive agent stop and dashboard restart in `state.db`. Read messages are
  deleted after 24 hours; all messages, including unread mail for stopped agents, are deleted after
  seven days. Stopped agents retain recent mail but are not addressable or nudged.

### 2.3 Nudging and per-turn budget

- **R9.** Inserting a message signals the recipient immediately and a two-second sweep is the
  fallback. When a running chat recipient is `idle` with unread mail, AgentDeck marks the pending
  rows `delivered_via="nudge"` and injects a new ACP turn instructing it to call
  `check_messages`, without a human prompt. Busy, waiting, done, error, stopped, terminal, mail-free,
  in-flight, and cooldown recipients are not nudged.
- **R10.** A nudge re-checks the recipient state at the runtime boundary before injection, resets
  that agent's per-turn coordination budget, and completes like an ordinary chat turn. One nudge
  may be in flight per agent; retries are separated by a three-second cooldown and a stuck
  in-flight marker expires after 60 seconds.
- **R11.** Each chat turn has a combined inbound-plus-outbound messaging budget of 15. Sending and
  reading consume it transactionally with the message mutation. The action that would exceed the
  budget does not occur: an outbound message is not inserted, and an inbound check returns only as
  many messages as remain. Responses expose remaining/exhausted state and a breach produces
  `message_budget_exceeded`, a warning, and a `budget_exceeded` notification.
- **R12.** A fresh user-prompt or nudge turn resets the budget under a new turn id. Restart/resume
  also resets cleanly and retains only one current budget row per agent, so a stale higher turn id
  cannot make the first post-restart turn appear exhausted.

### 2.4 Dashboard indicators and budget notification

- **R13.** A successful send causes the recipient's next `state_update` to carry its recomputed
  `unread_messages` count and causes the sender to carry a transient `last_sent_at` pulse. Agent
  cards render **Mail N** for unread mail and **Sent** for the outbound pulse; the pulse clears from
  the browser after two seconds.
- **R14.** Reading or deleting mail through `check_messages` immediately recomputes and publishes
  the recipient state so **Mail N** clears or decreases without unrelated activity.
- **R15.** A budget breach emits a `budget_exceeded` `notification` SSE event naming the agent and
  carrying a title/body suitable for a person returning attention to the dashboard. Lifecycle and
  permission event production belongs to FS-01/FS-03.
- **R16.** The `budget_exceeded` event participates in the notification mute/delivery pipeline
  governed by FS-02.R22–R24; FS-02 owns persisted preferences, toast/desktop choice, and dismissal.

## 3. States & transitions

- **Registration:** chat launch/resume mints token → registers token→agent → composes the reserved
  MCP server into the live session. Stop/switch/crash cleanup revokes it.
- **pending → nudged → read:** send inserts unread mail and publishes indicators; an idle recipient
  is nudged; `check_messages` marks/deletes returned rows and republishes its unread count.
- **pending/read → expired:** the janitor removes read mail older than 24 hours and any mail older
  than seven days. Expiration is permanent.
- **turn budget:** prompt/nudge resets to 15 → each send/read decrements remaining atomically → an
  over-limit attempt marks breached and notifies → the next turn resets it.
- **budget notification:** breach emits SSE → FS-02's shared presentation pipeline applies mute and
  delivery preferences.

## 4. Edge cases & errors

- **R17.** Terminal-interface agents are absent from recipient resolution and the interactive CLI
  does not receive usable coordination tools. Addressing one by id, address, or name returns
  `recipient_not_found` rather than queuing mail it cannot read. Server-side launch composition may
  still mint short-lived registration artifacts that terminal execution ignores; teardown removes
  them under the same lifecycle rules as chat registration.
- **R18.** `send_message` never inserts a row on invalid body/subject, missing/ambiguous recipient,
  store failure, unknown session, or exhausted budget. Tool failures return structured JSON in an
  MCP result marked as an error.
- **R19.** Empty agent and message lists serialize as arrays, not `null`. A mailbox read that
  returns nothing does not emit a false read-state update.
- **R20.** Multiple concurrent messaging calls cannot overrun the budget: budget consumption and
  message insert/read mutation share one SQLite transaction.
- **R21 (planned).** Annotate-and-assign (FS-13) inserts mail from a reserved **user sender**:
  sender id `user`, address `user@dashboard`, display name `Dashboard user`. That identity is
  minted only server-side by the annotation delivery path — it can never be supplied through MCP
  tool arguments and cannot collide with an agent id — and its sends consume no agent's per-turn
  budget while still updating unread/indicator state transactionally. Messages from the user sender
  follow the normal R6–R10 reading, nudging, indicator, and retention rules.

## 5. Acceptance criteria

- **A1** (R2–R6) — Two token-bound agents discover, send, and read over the real HTTP MCP
  transport, with sender identity derived from session:
  `internal/messaging/messaging_test.go::TestSendAndCheckRoundTrip` and
  `TestSendIdentityNotSpoofable`.
- **A2** (R4–R5, R17–R18) — Address resolution, ambiguity/not-found errors, validation, and terminal
  exclusion: `internal/messaging/messaging_test.go::TestSendErrors`;
  `internal/state/messages_test.go::TestResolveRecipient`, `TestResolveRecipientAmbiguous`, and
  `TestResolveRecipientExcludesTerminalAgents`.
- **A3** (R1–R2) — Launch registration writes an HTTP MCP entry with token and cleanup revokes it:
  `internal/server/messaging_registration_test.go::TestRegisterMessagingMCPWritesHTTPConfigAndCleanup`.
- **A4** (R9–R10) — An idle recipient with unread mail receives a no-user-action nudge and the row
  records `delivered_via=nudge`:
  `internal/server/messaging_registration_test.go::TestNudgeOnceWakesIdleAgentAndMarksDelivered`
  and `internal/runtime/chat_test.go::TestCheckMessagesInjectsNudgeTurn`.
- **A5** (R11–R12, R20) — The 16th outbound action is rejected without insertion, inbound reads cap
  to remaining budget, and restart/reset reuses one current row:
  `internal/messaging/messaging_test.go::TestSendMessageBudgetExceeded`,
  `TestCheckMessagesCapsAtRemainingBudget`; `internal/state/state_test.go::TestTurnBudgetConsumeAndBreach`
  and `TestResetTurnBudgetReusesSingleRow`.
- **A6** (R7–R8) — Mail listing keeps newest N and retention removes read-after-24h/all-after-7d:
  `internal/state/messages_test.go::TestListMessagesOrderingAndLimit`,
  `internal/state/state_test.go::TestDeleteExpiredMessagesRetention`, and
  `internal/server/server_test.go::TestSessionMessagesEndpoint`.
- **A7** (R13–R14) — Send publishes unread state and a default read clears it immediately:
  `internal/server/server_test.go::TestTouchRecipientPublishesUnread` and
  `internal/messaging/messaging_test.go::TestCheckMessagesFiresReadSink`.
- **A8** (R15–R16) — Budget notification integrates with the shared mute/delivery pipeline:
  `ui/src/api/sse.test.ts`,
  `ui/src/features/settings/NotificationsEditor.test.tsx`, and
  `internal/server/config_endpoint_test.go::TestPutConfigPersistsNotificationSettings`.
- **A9** (R1–R16) — Two fake-ACP chat agents send, nudge, read, and clear unread state without human
  relay, and state survives restart: journeys **J10** and **J12** in
  `docs/features/USABILITY-REVIEW.md`.
- **A10 (planned)** (R21) — A reserved-sender insert raises the recipient's unread badge, nudges an
  idle recipient, consumes no turn budget, and the user identity cannot be produced through MCP tool
  arguments: planned tests in `internal/messaging` and `internal/state` created with the FS-13
  implementation.

## 6. Deviations & open decisions

- **HTTP-only agent messaging.** The shipped registration
  always supplies a streamable HTTP `/mcp` entry. The legacy plan's `agentdeck mcp` stdio proxy was
  never implemented. A CLI that rejects HTTP MCP registration cannot use coordination until a
  working proxy or supported transport is added.
- **Real-CLI MCP acceptance is credential-gated.** Tests prove the real HTTP protocol with the Go
  MCP client and fake ACP sessions, but real Claude Code and Codex acceptance of the generated
  per-session HTTP registration and a live `ping`/tool call remains a manual gate. Do not claim
  compatibility for a CLI until that gate passes; implement a stdio proxy if either rejects HTTP.
- **No cross-turn loop detector.** R11 caps a tight loop within a turn, and the nudge cooldown
  bounds its rate, but two agents can continue a slow one-message-per-turn ping-pong indefinitely.
  Message history and budget rows provide the data for a rolling-window detector; none ships now.
- **Budget notifications repeat after the first breach.** The intended notification is the first
  breach of a turn, but each over-limit retry currently calls the budget sink again even though the
  budget row was already marked breached. This can produce repeated toasts until the next turn;
  tracked advisory to gate emission on the prior breach flag.
- **A permission request can notify twice.** Entering `waiting_input` emits the state-edge
  notification and the following `permission_request` transcript event emits the permission-specific
  notification, so one ACP prompt can produce two stacked toasts. The mute settings treat them as
  different types. Tracked advisory to make one notification authoritative for this transition.
- **Janitor expiry can leave an unread badge stale.** `check_messages` republishes after read/delete
  (R14), but the retention janitor deletes expired rows without touching affected agents. A
  seven-day unread expiration may therefore leave **Mail N** visible until another state update.
  Tracked advisory to publish for every affected recipient.
- **Nudge cooldown is keyed only by stable agent id.** A stop/relaunch under the same `agent_id`
  can inherit the old in-memory in-flight/cooldown entry and delay new mail by up to the cooldown
  (or until the in-flight timeout in the worst stale case). Key by launch generation or clear state
  when the running row changes.
- **Notification mutes depend on a populated config cache.** The SSE client reads preferences from
  React Query cache without fetching them itself. On a deep route that has not loaded config, a
  muted type can fall back to unmuted behavior. Tracked advisory to prefetch config at app start.

## 7. Traceability

- **MCP tools/session identity:** `internal/messaging/messaging.go`, `tools.go`;
  `internal/server/messaging_registration.go`.
- **Durable mail/budget:** `internal/state/messages.go`, `internal/state/migrate.go` (`messages`,
  `turn_budget`), `internal/messaging/constants.go`.
- **Nudger/retention:** `internal/server/messaging_loops.go`; chat injection in
  `internal/runtime/chat.go::CheckMessages`.
- **Indicators/budget event:** sinks in `internal/server/server.go`, notification publication in
  `internal/bus/`, `ui/src/api/sse.ts`, `ui/src/components/grid/AgentCard.tsx`,
  `ui/src/components/shell/NotificationCenter.tsx`,
  `ui/src/features/settings/NotificationsEditor.tsx`.
- **Key regression tests:** `TestSendAndCheckRoundTrip`, `TestSendIdentityNotSpoofable`,
  `TestNudgeOnceWakesIdleAgentAndMarksDelivered`, `TestSendMessageBudgetExceeded`,
  `TestResetTurnBudgetReusesSingleRow`, `TestCheckMessagesFiresReadSink`,
  `TestResolveRecipientExcludesTerminalAgents`, and `ui/src/api/sse.test.ts`.
