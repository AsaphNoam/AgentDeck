# Phase 5 — Coordination: MCP messaging, nudger, budgets, notifications

**Status:** ready to build after Phase 2
**Features:** F8 (agent-to-agent messaging), F11 (notifications)
**Depends on:** Phases 1, 2
**Enables:** Phase 7 (activity-map message animations); parallelizable with Phases 3 and 4

---

## 1. Goal

Let agents coordinate with each other without manual relay, and keep the human informed of the moments that matter. Build the **in-process MCP messaging server** (hosted in the Go binary, registered with every launched agent), the **nudger** that wakes idle recipients with pending mail, per-turn message budgets to prevent runaway loops, and desktop/in-app notifications on significant state transitions.

Begin with the **SDK handshake spike** (~1h): confirm `modelcontextprotocol/go-sdk` registers cleanly over stdio with **both** Claude Code and Codex before building on it.

---

## 2. Scope

### In scope
- In-process MCP messaging server (Go, `modelcontextprotocol/go-sdk`, stdio), hosted by the server and registered with each agent at launch.
- Three MCP tools: `list_agents`, `send_message(to, body)`, `check_messages`.
- Message delivery as **rows in `state.db`** (server is sole writer); one row per message keyed to the recipient.
- Nudger: server loop detecting an idle agent with pending mail and waking it via `Runtime.CheckMessages(pid)`.
- Per-turn message budget (default 15/turn) to cap agent-to-agent loops.
- Dashboard message indicators on sender/recipient cards.
- Notifications (F11): desktop/in-app on `done`, `waiting_input`, permission-required; per-type mute.

### Out of scope
- Cross-turn loop detection beyond the per-turn budget (flagged as open question).
- Activity-map animation (Phase 7 consumes the `new_message`/notification events this phase emits).

---

## 3. Detailed requirements

### 3.1 MCP messaging server (master PRD §4.5)
- In-process Go MCP server (`modelcontextprotocol/go-sdk`, stdio), registered with each agent CLI at launch (the Phase 1 launch composition already includes the MCP registration step). Caller identity is the registered stdio session, not a spoofable argument.
- Tools (in-process operations on `state.db`):
  - `list_agents` → live agents (name, role, project, state) read from `state.db`.
  - `send_message(to, body)` → insert a message row for the recipient.
  - `check_messages` → read + flag/delete the caller's pending message rows.
- Resolve `to` by `role@project` and/or agent name → `agent_id`.

### 3.2 Delivery + nudger
- Default delivery: recipient agent polls `check_messages`.
- Nudger: a server loop detects an idle agent (`status.state == idle`) with pending mail and calls `Runtime.CheckMessages(pid)` to wake it so it processes the message without user action. Implement the chat-runtime `CheckMessages` (stubbed in Phase 1).
- Per-turn budget: cap messages processed/sent per turn (default 15) to prevent runaway loops; budget breach is logged and surfaced.

### 3.3 Dashboard indicators
- Sender and recipient cards show a message indicator (e.g. unread count / "mail" badge) driven by message state (`state.db`) via SSE.

### 3.4 Notifications (F11)
- Emit SSE `notification` events on: task complete (`done`), `waiting_input`, and permission-required.
- Surface as desktop notifications (when the dashboard is backgrounded) and in-app toasts.
- Per-type mute controls persisted (in `config.json` or layout/settings).

---

## 4. REST/SSE surface added

```
SSE notification        (done | waiting_input | permission_required)
```

Messaging itself flows through the in-process MCP server + `state.db` message rows, not REST; the dashboard reads message/indicator state via `state_update`/`new_message` on the existing `/api/events` bus. Optionally add `GET /api/sessions/{id}/messages` for an inbox view.

---

## 5. Acceptance criteria

- [ ] An implementer agent can `send_message` a review request to a reviewer agent.
- [ ] If the reviewer is idle, the nudger wakes it and it processes the message **without user action**.
- [ ] The per-turn budget caps a deliberate message loop (two agents pinging each other) rather than running away.
- [ ] Sender and recipient cards show a message indicator when mail is pending.
- [ ] Backgrounding the dashboard and letting an agent finish produces a "task complete" desktop notification.
- [ ] Muting a notification type suppresses only that type.

---

## 6. Open questions (master PRD §9)
- Is a 15/turn budget enough, or is cross-turn loop detection also needed?
- Retention policy for read message rows in `state.db`.
- **SDK handshake spike (do first):** confirm `modelcontextprotocol/go-sdk` (stdio) registers cleanly with both Claude Code and Codex.
