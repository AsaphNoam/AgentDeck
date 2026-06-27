# Phase 5 — Coordination: Implementation Tech Spec

**Mirrors:** `docs/phases/phase-5-coordination.md` (phase PRD)
**Master PRD refs:** §4.5 (MCP messaging server, nudger, message budget), F8, F11
**Status:** ready to implement after Phases 1 and 2
**Features:** F8 (agent-to-agent messaging), F11 (notifications)

This is the implementation-level companion to the Phase 5 PRD. It is prescriptive: real tool schemas, real message rows, exact config shapes, concrete intervals and counters. There should be no design decisions left to make at implementation time. The messaging MCP server is hosted **in-process inside the Go dashboard server** (official Go MCP SDK); the nudger, budgets, notification emission, SSE, and message storage all live in that same Go process, and the dashboard pieces in the React/TS UI. There is no separate runtime process and no second language at runtime.

---

## 1. Overview & scope recap

### In scope
- **In-process MCP messaging server** hosted inside the Go dashboard binary using `github.com/modelcontextprotocol/go-sdk`. One server, owned by the dashboard process, shared by all agents; tool handlers run inside the server and operate directly on `state.db`.
- **Three MCP tools:** `list_agents`, `send_message(to, body)`, `check_messages`.
- **Message rows in `state.db`** (the Go server is the sole writer) with read/flag/delete semantics and a retention policy.
- **Nudger:** a Go server loop that detects an idle agent (`status.state == "idle"`) with pending unread mail and calls `Runtime.CheckMessages(pid)` to wake it.
- **Chat-runtime `CheckMessages`** implementation (stubbed in Phase 1).
- **Per-turn message budget** (default 15) capping messages processed/sent per turn, with breach logging + SSE surfacing.
- **Dashboard message indicators** on sender/recipient cards (unread/outbound badge), driven via SSE.
- **Notifications (F11):** SSE `notification` events on `done`, `waiting_input`, `permission_required`; OS desktop notifications via the Web Notifications API when the dashboard tab is backgrounded; in-app toasts when foregrounded; per-type mute persisted to `config.json`.
- **Optional** `GET /api/sessions/{id}/messages` inbox endpoint.

### Out of scope (explicit non-goals)
- **Cross-turn loop detection.** Only the per-turn budget (15/turn) is built. Detecting a slow ping-pong that stays under budget every turn but never terminates across turns is an **open question, not built** (see §13). We keep the data a future phase would need (per-turn budget rows + full message history in `state.db`), but no cross-turn termination logic ships here.
- **Activity-map animation** (Phase 7). Phase 5 only *emits* the `new_message` / `notification` events Phase 7 will animate on (see §12).
- **Threaded conversations / reply chains.** Messages carry an optional `in_reply_to` field for future use, but no threading UI or semantics ship.
- **Terminal-runtime `CheckMessages`.** Only the chat runtime's `CheckMessages` is implemented here. Terminal runtime (Phase 6) wires its own; this phase returns a typed "not implemented" for `interface: "terminal"` (consistent with Phase 1's registry behavior).

---

## 2. Technology choices

### 2.1 MCP SDK & language

- **`github.com/modelcontextprotocol/go-sdk`, pinned to a v1.x release** (v1.0.0 is the compatibility-frozen stable line, maintained with Google; v1.5.0 is current as of this writing). It provides a typed `mcp.Server`, typed tool handlers, and both an HTTP/streamable server transport and a stdio transport — exactly the primitives a messaging server needs.
- The dashboard server **embeds** the MCP server: it constructs one `mcp.Server`, registers the three tools as Go handlers, and exposes it on a localhost transport. Tool handlers are ordinary Go closures that capture the server's `*Store` (the `state.db` handle), so a tool call is a direct function call into the same process that owns the database. This realizes the master PRD's "in-process … with no serialization boundary" requirement literally: no IPC, no subprocess, no JSON round-trip to a sidecar.
- **Why in-process Go, not a separate process or a second language:** the master PRD fixes the messaging server as in-process in the Go binary, and the architecture's storage rule is "the Go server is the sole writer to `state.db`." Running the tool handlers inside that server is what keeps the sole-writer invariant trivially true — `send_message` is a Go `INSERT`, not a write from some other process that would have to be serialized and trusted. For a prebuilt-binary distribution the user needs nothing but the binary plus their agent CLI.
- **Caller identity** comes from the **registered MCP session**, not from any tool argument or environment variable. When an agent's CLI opens its MCP session, the dashboard server knows which `agent_id` that session belongs to (it issued the registration). `send_message`'s `from` and `check_messages`'s mailbox are derived from the session, so no caller can spoof another agent's identity.

### 2.2 Transport: in-process host + stdio fallback (Task 1 spike resolves which per CLI)

The dashboard server hosts the MCP server in-process. The open mechanism is *how each agent CLI connects to it*. There are two supported shapes, and Phase 5 **Task 1 is the handshake spike** that determines, per CLI (Claude Code, Codex), which one each needs:

**(A) HTTP / streamable transport (preferred).** The dashboard server already binds `127.0.0.1` for the browser UI. It mounts the go-sdk **streamable HTTP** server transport on a localhost path (e.g. `http://127.0.0.1:{port}/mcp`). Each agent is registered with an HTTP MCP server entry carrying a **per-agent session token** in a header; the server maps that token → `agent_id`, so identity is bound at registration and the tool handlers run inside the dashboard process against `state.db` directly. This is the cleanest realization of "in-process, no serialization boundary": there is no second process at all, only a localhost socket the CLI already trusts.

**(B) Stdio-subcommand fallback.** If a given CLI only knows how to launch a *stdio* MCP server as a child process (the classic `command`/`args` MCP shape), we ship an `agentdeck mcp` subcommand on the **same Go binary** (not a separate program, not a second language). The CLI spawns `agentdeck mcp --agent <agent_id> --token <token>` over stdio; that subcommand is a **thin proxy** that forwards each tool call to the already-running dashboard server over the localhost transport from (A) and streams the result back. The proxy holds no state and never touches `state.db` itself — the real handlers still execute in-process in the dashboard server, preserving "server is sole writer." The per-agent token passed to the subcommand is what binds the proxied calls to the right `agent_id`.

**Task 1 (the spike, ~1h, do first):** register the go-sdk server with **Claude Code** and with **Codex** and determine, for each, whether the HTTP-transport registration in (A) is accepted, or whether that CLI requires the stdio-subcommand fallback (B). Both CLIs consume standard MCP servers, so this swaps *which transport entry we write at registration*, not the server logic — the handlers are identical either way. Record the per-CLI outcome and wire `RegisterMessagingMCP` (§3.6) to emit the right entry per backend.

In both shapes the dashboard server is the single owner of the MCP server and of `state.db`; the only difference is whether the CLI talks to it directly over HTTP or through a stdio proxy that the same binary provides.

### 2.3 Where messaging state lives

- **Messages:** rows in the `messages` table in `state.db` (§4.1). `list_agents` reads `state.db`; `send_message` inserts a row; `check_messages` selects, flags, or deletes the caller's rows.
- **Per-turn budget:** a `turn_budget` table in `state.db` keyed by `(agent_id, turn_id)` (§6). The runtime resets it at turn start; the MCP tool handlers read and increment it inside the same process.
- **No file mailboxes, no per-agent state files for messaging.** Everything messaging touches is a transactional read/write of `state.db` by the one process allowed to write it.

### 2.4 Desktop notification approach

**Decision: Web Notifications API (browser-native), not an OS-level/native notifier.**

- The dashboard is a React/Vite app running in the user's local browser (master PRD §4, §7). The browser already provides the [Web Notifications API](https://developer.mozilla.org/en-US/docs/Web/API/Notifications_API) (`Notification.requestPermission()` + `new Notification(...)`), which renders true OS desktop notifications on macOS and Linux through the browser.
- **Rationale for Web Notifications over OS-level (e.g. a Go-side `terminal-notifier`/`notify-send` shell-out):**
  - **Zero new dependencies / no platform binaries.** OS-level would require `terminal-notifier` (macOS) and `notify-send`/libnotify (Linux) — extra prereqs the PRD doesn't list, plus per-platform branching.
  - **Backgrounded-tab detection is a browser concern.** We only fire desktop notifications when `document.visibilityState === "hidden"` (tab backgrounded/minimized); when visible we show an in-app toast instead. The browser owns that signal natively.
  - **Local-first alignment.** Everything stays in the browser/server pair; no shelling out to system tools the user must install.
  - **User-gesture permission flow** fits the onboarding/settings UX naturally.
- **Fallback:** if `Notification.permission === "denied"` or the API is unavailable, degrade gracefully to in-app toasts only (always available). Mute settings apply to both channels.

---

## 3. MCP messaging server design

The MCP server is constructed once during dashboard startup and lives for the life of the process. It is built in the Go server package (e.g. `internal/messaging/`), alongside the nudger and janitor, and shares the server's `*Store` (the `state.db` handle).

### 3.1 Server construction & session identity

At startup the dashboard server:

1. Builds an `mcp.Server` from the go-sdk and registers the three tools (`list_agents`, `send_message`, `check_messages`) as typed Go handlers, each closing over `*Store`.
2. Mounts the server's streamable HTTP transport on the existing localhost listener at `/mcp` (transport (A), §2.2).
3. Maintains a `map[sessionToken]agentID` (the **session registry**). `RegisterMessagingMCP` (§3.6) mints a per-agent token at launch and records the mapping; teardown removes it on Stop.

Every tool handler resolves the **calling `agent_id` from the session** (the token presented on the HTTP transport, or threaded through by the stdio proxy in transport (B)). This is the linchpin: `send_message`'s `from` and `check_messages`'s mailbox are unambiguous **without trusting any tool argument**, because identity is bound to the registered session, not passed by the caller.

If a tool call arrives on a session whose token is unknown (e.g. after the agent was stopped and its token revoked), the handler returns a structured `session_unknown` error and the call is rejected.

### 3.2 Store access helpers

A `Store` type (the same `state.db` handle the rest of the server uses) gains messaging methods. All run on the Go server's single writer connection (SQLite in WAL mode, `github.com/mattn/go-sqlite3`):

- `LiveAgents() ([]LiveAgent, error)` — agents currently running (a row in the running registry), joined with identity + latest status.
- `InsertMessage(m Message) error` — insert one message row (within a transaction; sets `created_at`, `read=false`, `delivered_via="pending"`).
- `ListMessages(recipientID string, unreadOnly bool, limit int) ([]Message, error)` — the caller's mailbox, ordered by `created_at` ascending.
- `MarkRead(ids []string) error` / `DeleteMessages(ids []string) error` — flag or remove returned rows.
- `UnreadCount(agentID string) (int, error)` — drives the card badge and the nudger.
- `ResolveRecipient(to string) (agentID string, candidates []AgentRef, err error)` — `to`-resolution (§3.4).

Because the Go server is the sole writer and these run in one process, concurrent tool calls from different agents are serialized by the DB connection/transaction layer — there is no cross-process write contention to reason about.

### 3.3 Tool: `list_agents`

Discover other live agents. **Excludes the caller** by default (identity from the session).

**Input schema (params):**
```jsonc
{
  "include_self": false,                 // boolean, optional — include the caller in results
  "state": "idle"                        // optional enum: "busy"|"idle"|"waiting_input"|"done"|"error"
}
```

**Returns** (MCP tool result `content[0].text` = JSON string of):
```jsonc
{
  "agents": [
    {
      "agent_id": "a_8f3c12",
      "name": "Atlas",
      "role": "implementer",
      "project": "my-app",
      "address": "implementer@my-app",   // canonical addressable form
      "state": "idle",                     // latest status (or "unknown")
      "detail": "Idle",
      "context_pct": 0.42
    }
  ]
}
```

**Sourcing:** `Store.LiveAgents()` — only agents with a row in the running registry are listed (stopped/archived agents are not addressable). `state`/`detail`/`context_pct` come from the agent's latest status in `state.db`; if absent, `state: "unknown"`. This is the "live state from `state.db`" the PRD asks for, read in-process — no file scan, no call out to another process.

### 3.4 Tool: `send_message(to, body)`

Insert a message row for the recipient.

**Input schema (params):**
```jsonc
{
  "to": "reviewer@my-app",               // string, required — "role@project" OR agent name OR agent_id
  "body": "Please review the diff.",     // string, required, 1..8000 chars
  "subject": "Review request",           // string, optional, <=200 chars
  "in_reply_to": "m_1a2b3c"              // string, optional — message_id being replied to (forward-compat; not threaded yet)
}
```

**`to` resolution order** (`Store.ResolveRecipient`, first match wins):
1. **Exact `agent_id`** — if `to` matches a live agent's `agent_id`.
2. **`role@project`** — split on `@`; match a live agent whose `role` and `project` both equal the parts. If multiple live agents share the same `role@project` → **ambiguous**: return an error listing the candidate `agent_id`s and names, instructing the caller to address by name or id.
3. **Name (case-insensitive)** — match a live agent whose `name` equals `to`. Same ambiguity rule on duplicate names.
4. **No match** → error (see §9; recipient nonexistent/stopped).

Resolution only ever targets **live** agents (a row in the running registry). Sending to a stopped agent is an error, surfaced to the sender (§9).

**On success**, within one transaction: check + increment the sender's **outbound** turn budget (§6); if the budget would be exceeded, the row is **not** inserted and a budget-breach error is returned (§6). Otherwise `Store.InsertMessage` writes the row with `from` = the session's `agent_id` (never an argument), `to` = the resolved recipient `agent_id`, a freshly generated `message_id`, and `delivered_via="pending"`.

**Returns:**
```jsonc
{ "ok": true, "message_id": "m_1a2b3c", "to": "a_reviewer_id", "to_address": "reviewer@my-app" }
```

### 3.5 Tool: `check_messages`

Read + flag/delete the caller's pending messages. Caller is the session's `agent_id`.

**Input schema (params):**
```jsonc
{
  "mark_read": true,                     // boolean, optional, default true — flag returned messages read
  "delete_after": false,                 // boolean, optional, default false — delete returned messages after reading
  "unread_only": true,                   // boolean, optional, default true
  "limit": 15                            // integer, optional, default 15, range 1..50 — hard-capped at the per-turn budget
}
```

**Behavior** (one transaction):
1. `Store.ListMessages(self_id, unread_only, ...)` ordered by `created_at` ascending.
2. Take up to `min(limit, remaining_inbound_budget)` (§6). Reading is itself budget-governed: messages processed per turn count against the inbound budget.
3. If `mark_read`, set `read=true` + `read_at` on each returned row. If `delete_after`, delete instead (delete wins over mark_read).
4. Increment the **inbound** turn-budget counter by the number returned.

**Returns:**
```jsonc
{
  "messages": [
    {
      "message_id": "m_1a2b3c",
      "from": "a_impl_id",
      "from_address": "implementer@my-app",
      "from_name": "Atlas",
      "subject": "Review request",
      "body": "Please review the diff on src/auth.ts",
      "created_at": "2026-06-22T10:05:00Z",
      "in_reply_to": null
    }
  ],
  "remaining": 3,            // unread messages left after this read
  "budget_remaining": 12     // inbound budget left this turn
}
```

### 3.6 Registration with each agent CLI at launch (added to Phase 1 launch composition)

Phase 1's launch composition gains an **MCP registration step** between "compose config" and "Runtime.Start". The Go server, owning the running MCP server, mints a **per-agent session token**, records `token → agent_id` in the session registry (§3.1), and writes the CLI-appropriate MCP server entry pointing at the dashboard's transport.

**HTTP-transport entry (transport (A), preferred).** The Go server writes a per-agent MCP config naming the localhost HTTP endpoint with the token as a header:
```jsonc
{
  "mcpServers": {
    "agentdeck-messaging": {
      "type": "http",
      "url": "http://127.0.0.1:4317/mcp",
      "headers": { "X-AgentDeck-Token": "<per-agent token>" }
    }
  }
}
```

**Stdio-subcommand entry (transport (B), fallback for a CLI that only spawns stdio servers).** Same binary, no second language:
```jsonc
{
  "mcpServers": {
    "agentdeck-messaging": {
      "command": "agentdeck",
      "args": ["mcp", "--agent", "a_8f3c12", "--token", "<per-agent token>"]
    }
  }
}
```
The spawned `agentdeck mcp` subcommand proxies tool calls to the running dashboard server over the localhost transport (§2.2 (B)); the real handlers still run in-process against `state.db`.

**Per CLI:** Task 1 (§2.2) determines whether **Claude Code** and **Codex** each accept the HTTP entry or require the stdio entry. The Go server writes the per-agent entry to `~/.agentdeck/mcp/{agent_id}.mcp.json` and passes the CLI's config flag (e.g. `--mcp-config <file>` for Claude Code; Codex's equivalent `mcp_servers` config). Either way the *destination* is the one in-process server.

**Abstraction in Go:** add `RegisterMessagingMCP(agent, backendType) (launchArgs []string, cleanup func())` in the launch path. It mints the token, records the session mapping, emits the per-agent `.mcp.json` (HTTP or stdio per the Task 1 result for that backend), and returns the extra CLI args plus a cleanup that removes the config file **and revokes the token from the session registry** on Stop. This keeps backend-specific registration wiring in one place; the Phase 1 launch composition just calls it.

> **Phase 1 amendment note:** Phase 1's spec says "registers the MCP messaging server" in the launch flow (F4 step 2) but stubs the actual tool. Phase 5 fills in (a) the real in-process MCP server, and (b) the `RegisterMessagingMCP` helper that mints the token and produces the launch args. If Phase 1 already added a placeholder registration, Phase 5 replaces its body with the above.

---

## 4. Message storage + delivery design

### 4.1 `messages` table schema

Messages are rows in `state.db`; the Go server is the sole writer (architecture decision D3). One row per message.

```sql
CREATE TABLE IF NOT EXISTS messages (
  message_id     TEXT PRIMARY KEY,            -- "m_" + 6 hex (regenerated on the rare collision)
  from_agent     TEXT NOT NULL,               -- sender agent_id (from the session, never spoofable)
  from_address   TEXT NOT NULL,               -- resolved at send time, for display
  from_name      TEXT NOT NULL,
  to_agent       TEXT NOT NULL,               -- recipient agent_id (the mailbox owner)
  subject        TEXT NOT NULL DEFAULT '',
  body           TEXT NOT NULL,
  created_at     TEXT NOT NULL,               -- RFC3339 UTC
  read           INTEGER NOT NULL DEFAULT 0,  -- 0/1; flipped by check_messages mark_read
  read_at        TEXT,                        -- RFC3339 when read, else NULL
  delivered_via  TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'nudge' | 'poll'
  in_reply_to    TEXT                         -- message_id or NULL (forward-compat)
);
CREATE INDEX IF NOT EXISTS idx_messages_to_unread  ON messages(to_agent, read);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
```

The message-row JSON returned by the tools mirrors the row:
```jsonc
{
  "message_id": "m_1a2b3c",
  "from": "a_impl_id",                   // from_agent
  "from_address": "implementer@my-app",
  "from_name": "Atlas",
  "to": "a_reviewer_id",                 // to_agent
  "subject": "Review request",
  "body": "Please review the diff on src/auth.ts",
  "created_at": "2026-06-22T10:05:00Z",
  "read": false,
  "read_at": null,
  "delivered_via": "pending",            // "pending" | "nudge" | "poll"
  "in_reply_to": null
}
```

- **`message_id`** generated by the server: `m_` + crypto-random 6 hex; on the (rare) PK collision, regenerate and retry the insert.
- All writes go through the single writer connection inside a transaction; readers (nudger, indicators, inbox endpoint) use the same `Store`. SQLite WAL mode lets reads proceed concurrently with the writer.

### 4.2 Read / flag / delete semantics

- **Unread:** `read = 0`. Drives the recipient card's unread badge and the nudger's "pending mail" check (`idx_messages_to_unread` makes this a cheap lookup).
- **Flag read:** `check_messages` with `mark_read: true` (default) sets `read = 1`, `read_at`. The row remains (so the inbox endpoint can show recent history) until cleanup.
- **Delete:** `check_messages` with `delete_after: true` deletes the row immediately after returning it. Agents that don't want history pass this.
- **`delivered_via`:** the nudger sets `delivered_via = 'nudge'` on unread rows when it wakes the recipient (audit trail); a poll-driven `check_messages` sets `'poll'` on rows it returns that were still `'pending'`. Purely diagnostic.

### 4.3 Retention / cleanup policy (adopted)

A **Go-side janitor** (runs alongside the nudger loop, every 60s) issues bounded `DELETE`s on the writer connection:
- Delete `read = 1` messages whose `read_at` is older than **24h** (read messages are transient by default).
- Delete **any** message (read or not) older than **7 days** (`created_at`) — a hard cap so an offline/stopped recipient's mailbox can't grow unbounded.
- When an agent is **stopped/archived** (its running-registry row is removed), leave its message rows in place until the 7-day cap (so a resumed agent still sees recent mail), but the nudger no longer acts on it (no live pid to nudge).
- Cleanup is logged at debug level with row counts. Thresholds live as Go constants (`MailReadTTL = 24h`, `MailHardTTL = 168h`, `JanitorInterval = 60s`); promoting them to `config.json` is optional and not required for acceptance.

---

## 5. Nudger design

The nudger is a **Go server loop**. It closes the loop so an idle recipient processes mail without user action (F8 acceptance).

### 5.1 Detection

- The nudger maintains a ticker (default **2s** interval, `NudgeInterval`). On each tick (and additionally, event-driven, when a `send_message` insert signals new mail):
  1. Enumerate live agents (a row in the running registry).
  2. For each, check its latest status: act only if `state == "idle"` (per PRD: idle agents with pending mail). `done` is **not** nudged (it has finished; the user decides next step). `waiting_input`/`busy`/`error` are skipped.
  3. `Store.UnreadCount(agent_id)`: if ≥1 and the agent is idle → it's a nudge candidate.
- Combine ticker + insert-driven checks so latency is low (a new `send_message` can signal the loop directly) but a missed signal is still caught within `NudgeInterval`.

### 5.2 Waking

For a candidate, call `Runtime.CheckMessages(pid)` via the runtime registry (dispatched by `agent.interface`), passing the pid from the running registry.

**Chat-runtime `CheckMessages(pid)` implementation** (the Phase 1 stub, now real):
- The chat runtime holds the live ACP/stdio session for the agent (established at `Start`). `CheckMessages` injects a **system-level nudge turn** into that session instructing the agent to call the `check_messages` MCP tool and act on its mail. Concretely, it sends a minimal user/system turn over ACP:
  > `You have new messages. Call the check_messages tool and handle them.`
- This is a normal turn from the runtime's perspective: status transitions `idle → busy`, the agent calls `check_messages`, processes mail, possibly calls `send_message`, and returns to `idle`. The transcript deltas stream as usual (`new_message` SSE), and the user sees the agent "wake up."
- Before injecting, the runtime **re-checks** the agent is still `idle` (guards against a race where the user just sent a prompt — see §5.4). If not idle, it no-ops and returns; the nudger will retry next tick.
- Mark the triggering messages `delivered_via = 'nudge'` (the nudger updates these rows before/after the wake for the audit trail).

### 5.3 Scheduling / avoiding double-wakes

- **In-flight set:** the nudger keeps an in-memory `map[agent_id]nudgeState` with `{lastNudgeAt, inFlight bool}`. When it wakes an agent, set `inFlight = true`. Clear `inFlight` only when the agent returns to `idle` **and** has no unread mail, OR after a **timeout** (`NudgeInFlightTimeout = 60s`) to avoid a permanently stuck flag if a turn hangs.
- **Cooldown:** even after `inFlight` clears, enforce a minimum `lastNudgeAt + NudgeCooldown (3s)` before re-nudging the same agent. This prevents a tight nudge→idle→nudge spin.
- **Single nudge per idle window:** an agent that is woken processes *all* available mail (up to budget) in one turn, so one nudge per idle→busy→idle cycle suffices; the `inFlight` flag enforces exactly one outstanding wake per agent.
- The nudger is **single-goroutine** (the loop) dispatching wakes; `CheckMessages` calls run in a goroutine with the in-flight guard and a timeout, so a slow runtime can't stall the loop indefinitely (the loop itself never blocks > the tick).

### 5.4 Interaction with user turns

If the user sends a prompt to an agent that the nudger is about to wake, the user turn wins (the runtime's `idle` re-check in §5.2 fails and the nudge no-ops). The pending mail stays unread and will be picked up on the next idle window (or the agent may call `check_messages` itself mid-turn). No mail is lost.

---

## 6. Per-turn budget

Cap the messages an agent processes/sends **per turn** to prevent runaway agent-to-agent loops. **Default 15**, constant `MessageBudgetPerTurn`.

### 6.1 Where the counter lives

The budget is **per agent, per turn**, and is read/incremented by the MCP tool handlers (in-process) and reset by the runtime at turn start. Both run inside the Go server, so the counter lives in a `turn_budget` table in `state.db`:

```sql
CREATE TABLE IF NOT EXISTS turn_budget (
  agent_id  TEXT NOT NULL,
  turn_id   TEXT NOT NULL,                -- opaque id of the current turn (see 6.2)
  inbound   INTEGER NOT NULL DEFAULT 0,   -- messages read this turn via check_messages
  outbound  INTEGER NOT NULL DEFAULT 0,   -- messages sent this turn via send_message
  breached  INTEGER NOT NULL DEFAULT 0,   -- 1 on first breach this turn
  PRIMARY KEY (agent_id, turn_id)
);
```

- **`inbound + outbound` together count against `MessageBudgetPerTurn`** (a single shared cap of 15 covers both reading and sending in one turn — the strictest reading of "messages processed/sent per turn," and the most aggressive at killing loops).
- The MCP handlers read and increment the current `(agent_id, turn_id)` row inside the same transaction as the message insert/select. Because everything runs in the one Go writer process, the read-modify-write is naturally single-writer; there is no cross-process contention.

### 6.2 Turn boundary (`turn_id`)

A "turn" = one user/nudge prompt → agent response cycle. The **Go runtime** owns the turn boundary: when the chat runtime begins a new turn (user prompt or nudge injection), it sets a fresh `turn_id` (`t_` + counter) for that agent and **upserts a `turn_budget` row with `inbound=0, outbound=0, breached=0`**. The MCP handlers, on each tool call, read the agent's current `turn_id` and scope their increments to that row. (The handlers never reset counters — only the runtime does, at turn start. Runtime defines turns; the MCP handlers enforce the cap within them. The runtime tracks each agent's current `turn_id` in memory and writes it to the budget row; the handlers look it up via `state.db`.)

### 6.3 Breach handling

When a tool call would push `inbound + outbound` past `MessageBudgetPerTurn`:
- **`send_message`:** refuse to insert the row. Return an error result:
  ```jsonc
  { "ok": false, "error": "message_budget_exceeded",
    "message": "Per-turn message budget (15) reached. This message was not sent.",
    "budget": 15, "used": 15 }
  ```
- **`check_messages`:** return only up to the remaining budget; if zero remaining, return `messages: []` with `"budget_remaining": 0` and an explanatory note so the agent understands why it got nothing.
- **Set `breached = 1`** on the turn's budget row on first breach. The Go server **emits an SSE `notification`** of type `budget_exceeded` (see §8) plus logs `WARN budget exceeded agent=<id> turn=<turn_id>`. The dashboard shows a non-fatal toast/badge ("Atlas hit its message budget this turn"). This is the "logged and surfaced" requirement.
- Breach is **not** fatal: the agent's turn continues normally; only further messaging is blocked until the next turn resets the counter.

### 6.4 Why this caps loops

Two agents pinging each other: each ping is an outbound (sender) + an inbound (recipient) within their respective turns. After 15 messaging actions in a single turn an agent is blocked from sending/reading more, so an in-turn flood is hard-capped. A slower cross-turn ping-pong is **not** stopped by this (see §13).

---

## 7. Dashboard indicators + notifications (F11)

### 7.1 Card message indicator

- **Recipient indicator (unread badge):** driven by message state. When a `messages` row is inserted, updated, or deleted for an agent, the state manager recomputes that agent's unread count (`Store.UnreadCount`) and folds it into that agent's `state_update` SSE payload as `unread_messages: N`. The card renders a mail badge with the count when `N > 0`.
- **Sender (outbound) indicator:** a brief "sent" pulse. When `send_message` inserts a row, the state manager emits a `state_update` for the sender carrying `last_sent_at` (timestamp). The card shows a transient outbound icon for ~2s. (This doubles as the signal Phase 7's activity map animates on — see §12.)
- No new SSE event type is needed for indicators; they ride on the existing `state_update` (per PRD §4: the dashboard reads message/indicator state via `state_update`/`new_message`).

### 7.2 Notification SSE events

The **Go server** emits `notification` SSE events on significant transitions, detected by the state manager when an agent's status changes:
- `state` transitions to **`done`** → `notification{type:"done"}`.
- `state` transitions to **`waiting_input`** → `notification{type:"waiting_input"}`.
- A **permission_request** transcript event arrives (the chat runtime already surfaces these in Phase 1) → `notification{type:"permission_required"}`.
- Budget breach (§6.3) → `notification{type:"budget_exceeded"}` (informational; mutable like the others).

Emit **on transition only** (edge-triggered), not on every status write, to avoid duplicate notifications. The state manager compares previous vs new `state` per agent (it already holds the prior state for `state_update` diffing).

### 7.3 Delivery: desktop vs in-app

Client logic (React):
- Subscribe to `notification` events on `/api/events`.
- For each event, check the per-type mute (§7.4). If muted, drop it.
- **If `document.visibilityState === "hidden"`** (tab backgrounded/minimized) **and** `Notification.permission === "granted"`: fire a Web Notification (`new Notification(title, { body, tag: agent_id })`). Use `tag: agent_id` so repeated notifications for the same agent collapse rather than stack.
- **Else** (tab visible): show an in-app toast (existing toast system / a lightweight one if none exists).
- Permission is requested once, on first run or from Settings, via a user gesture (`Notification.requestPermission()`); never auto-prompt on page load without a gesture.

Notification copy (examples): `done` → "Atlas finished" / detail; `waiting_input` → "Atlas needs input"; `permission_required` → "Atlas requests permission: Edit src/auth.ts"; `budget_exceeded` → "Atlas hit its message budget."

### 7.4 Per-type mute persistence — exact shape

Persisted in **`config.json`** under a `notifications` key:

```jsonc
// ~/.agentdeck/config.json
{
  "port": 4317,
  "default_project": "my-app",
  "default_role": "implementer",
  "skip_permissions": false,
  "notifications": {
    "desktop_enabled": true,          // master switch for OS-level (Web Notifications)
    "muted": {
      "done": false,
      "waiting_input": false,
      "permission_required": false,
      "budget_exceeded": false
    }
  }
}
```

- **Read/write via REST:** reuse the config endpoint. If Phase 3 owns `config.json`, add a focused pair here so Phase 5 is self-contained: `GET /api/settings/notifications` → returns the `notifications` block; `PUT /api/settings/notifications` → merges and persists it. (If Phase 3's `GET/PUT /api/config` already exists, prefer extending it and skip the dedicated endpoint.)
- The client loads the block on startup and on settings change; mute toggles in Settings write back via the endpoint. Muting a type sets its `muted.{type} = true`; the client then drops both desktop and in-app for that type.

### 7.5 Optional inbox endpoint

`GET /api/sessions/{id}/messages` — read-only inbox view for the chat panel / a future inbox UI. JSON in §8.2. Implemented in **Go** as a `state.db` read (`SELECT ... FROM messages WHERE to_agent = ?`); does not touch the MCP tool path or mark anything read. Marked optional in the PRD; we include it because the chat panel benefits and it's cheap. Query params: `?unread_only=true&limit=50`.

---

## 8. API / SSE contracts

### 8.1 `notification` SSE event payload

Sent on the existing `/api/events` multiplexed bus (Phase 2). SSE `event: notification`, `data:` =

```jsonc
{
  "type": "notification",
  "notification_type": "done",        // "done" | "waiting_input" | "permission_required" | "budget_exceeded"
  "agent_id": "a_8f3c12",
  "agent_name": "Atlas",
  "address": "implementer@my-app",
  "title": "Atlas finished",          // server-composed default copy
  "body": "Editing src/auth.ts — done",
  "detail": { "tool": "Edit", "path": "src/auth.ts" }, // type-specific extras; may be {}
  "ts": "2026-06-22T10:08:00Z"
}
```

`state_update` events (existing) gain two optional fields this phase:
```jsonc
{ "...existing state_update fields...": "...",
  "unread_messages": 2,          // recipient indicator
  "last_sent_at": "2026-06-22T10:05:00Z" // sender outbound pulse (optional)
}
```

### 8.2 Optional `GET /api/sessions/{id}/messages` response

```jsonc
{
  "agent_id": "a_8f3c12",
  "unread_count": 1,
  "messages": [
    {
      "message_id": "m_1a2b3c",
      "from": "a_impl_id",
      "from_address": "implementer@my-app",
      "from_name": "Atlas",
      "subject": "Review request",
      "body": "Please review the diff on src/auth.ts",
      "created_at": "2026-06-22T10:05:00Z",
      "read": false,
      "read_at": null,
      "in_reply_to": null
    }
  ]
}
```

Sorted `created_at` descending (newest first) for display. Read-only; this endpoint does **not** mark messages read (only the agent via `check_messages` does, to keep "read" meaning "the agent saw it").

---

## 9. Concurrency, edge cases & error handling

| Case | Handling |
|---|---|
| **Runaway loop (two agents pinging)** | Per-turn budget (§6) hard-caps in-turn floods at 15 combined inbound+outbound; breach blocks further messaging that turn, logs WARN, emits `budget_exceeded`. Cross-turn slow loops are **not** stopped (open question, §13) — but the per-turn budget rows + full message history in `state.db` give a future phase the data to detect them. |
| **MCP tool call on an unknown session** | The session registry (§3.1) has no `token → agent_id` mapping (e.g. agent stopped, token revoked). Handler returns `{"ok":false,"error":"session_unknown",...}`; the call is rejected, identity is never inferred from arguments. |
| **`state.db` unavailable / write error** | The handler's transaction fails; it returns a structured `{"ok":false,"error":"store_unavailable","message":...}`. Because the handler runs in the dashboard process, a DB outage is the same outage the rest of the server already surfaces; the agent sees the error and reports it. |
| **Message to nonexistent recipient** | `send_message` resolution fails → `{"ok":false,"error":"recipient_not_found","message":"No live agent matches 'reviewer@my-app'.","candidates":[...]}` (candidates empty here). The sender's agent reads the error and can `list_agents` to find a valid target. |
| **Message to stopped/archived recipient** | Same `recipient_not_found` (resolution only targets live agents). The sender is told the recipient isn't running. (We deliberately do not silently insert into a dead mailbox — it would never be delivered.) |
| **Ambiguous `to`** (duplicate `role@project` or name) | `{"ok":false,"error":"ambiguous_recipient","candidates":[{"agent_id","name","address"}...]}`; sender re-sends by `agent_id`. |
| **Nudge while agent transitions to busy** | Nudger's pre-injection `idle` re-check (§5.2) no-ops if not idle. `inFlight` flag (§5.3) prevents a second concurrent wake. User turns win (§5.4). |
| **Double-wake / nudge storm** | `inFlight` map + `NudgeCooldown (3s)` + one-nudge-per-idle-window (§5.3). In-flight timeout (60s) prevents a stuck flag. |
| **Budget counter under concurrent tool calls** | Single writer process; the read-increment-check of the `turn_budget` row happens inside the same transaction as the message insert/select, so it is atomic. The runtime (turn reset) and the MCP handlers (increments) touch disjoint moments — runtime only at turn boundary, handlers only between boundaries — coordinated via `turn_id`. |
| **`status` missing for a live agent** | Nudger treats unknown state as not-idle (won't nudge); `list_agents` returns `state:"unknown"`. Conservative: never nudge on uncertainty. |
| **Recipient has no messages** | `check_messages` returns `messages: []`; `UnreadCount` returns 0; no badge. |

---

## Subphase plan (incremental / quota-limited implementation)

Every subphase ends at a GREEN checkpoint — `go build ./...` passes (and `npm run build` for UI subphases) and all existing tests pass — so work is never half-done and a fresh agent can resume cold at the next subphase without inheriting half-finished state.

### Subphase 5.1 — Go-MCP-SDK handshake spike
- **Goal:** prove `github.com/modelcontextprotocol/go-sdk` registers with BOTH Claude Code and Codex, and decide the transport per CLI.
- **Deliverables:** a throwaway-or-keep spike that constructs an `mcp.Server` (no real tools needed — a trivial ping/echo handler suffices), exposes it over the streamable HTTP transport on the localhost listener (§2.2 (A)) and, if needed, via the stdio-subcommand shape (§2.2 (B)); a recorded per-CLI decision (HTTP-in-process vs stdio-subcommand fallback) wired toward `RegisterMessagingMCP` (§3.6). Maps onto Task 1 (§10).
- **Depends on:** Phase 1 launch composition + dashboard localhost listener (Phase 2); the pinned go-sdk v1.x dependency added to `go.mod`.
- **Done when (checkpoint):** registration confirmed for both CLIs — each CLI's MCP client connects to the spike server and a trivial tool call round-trips; the HTTP-vs-stdio outcome is recorded per CLI in the spec/notes. `go build ./...` passes and existing tests pass.
- **Resume note:** at start only the Phase 1/2 server exists with a stubbed MCP registration; begin by adding the go-sdk dependency and standing up the spike server against the existing localhost listener.
- **Size:** S.

### Subphase 5.2 — Message store + the three MCP tools
- **Goal:** persist messages in `state.db` and wire `list_agents`/`send_message`/`check_messages` to the in-process server.
- **Deliverables:** `messages` + `turn_budget` tables/indexes in the `state.db` migration (§4.1, §6.1); `Store` messaging methods (§3.2: `LiveAgents`, `InsertMessage`, `ListMessages`, `MarkRead`, `DeleteMessages`, `UnreadCount`, `ResolveRecipient`); the in-process MCP server skeleton with `token → agent_id` session registry (§3.1); the three typed tool handlers (§3.3–§3.5) closing over `*Store`, identity from the session. `to`-resolution + error shapes (§9). Maps onto Tasks 2–6 (§10).
- **Depends on:** Subphase 5.1 (transport decision + server construction pattern).
- **Done when (checkpoint):** `go build ./...` passes; unit tests for `to`-resolution (agent_id / role@project / name / ambiguous / not-found) and session identity (`from` always = session agent_id; unknown token → `session_unknown`) pass alongside all existing tests. Budget is read but enforcement-at-turn-boundary lands in 5.3.
- **Resume note:** at start the spike server exists; begin by adding the migration + `Store` methods, then attach the three handlers to the server from 5.1.
- **Size:** M.

### Subphase 5.3 — Registration, nudger, turn budget, janitor
- **Goal:** close the loop — register messaging per agent, reset budgets per turn, and wake idle recipients automatically within the 15/turn cap.
- **Deliverables:** `RegisterMessagingMCP(agent, backendType)` minting per-agent token + emitting `.mcp.json` (HTTP or stdio per 5.1), wired into Phase 1 launch composition for `claude-acp` and `codex-acp` (§3.6, Task 8); chat-runtime `CheckMessages(pid)` replacing the Phase 1 stub with idle re-check (§5.2, Task 9); runtime turn-boundary `turn_id` reset of the `turn_budget` row (§6.2, Task 10); budget breach handling in the handlers (§6.3); the nudger loop with in-flight/cooldown guards (§5, Task 11); the janitor retention `DELETE`s (§4.3, Task 12). Locked constants per §13.
- **Depends on:** Subphase 5.2 (tools + store + budget table).
- **Done when (checkpoint):** `go build ./...` passes; the F8 send→nudge→process-without-user-action integration test and the budget-caps-a-loop test (16th `send_message` → `message_budget_exceeded`, `breached=1`) pass, plus janitor retention + registration unit tests, alongside all existing tests.
- **Resume note:** at start the three tools work but nothing wakes a recipient or resets budgets; begin with `RegisterMessagingMCP` + the runtime turn boundary, then the nudger and janitor loops.
- **Size:** M.

### Subphase 5.4 — Notifications + dashboard message indicators (UI)
- **Goal:** surface notifications and message state to the user.
- **Deliverables:** state-manager extensions — `unread_messages` recompute on message-row change + `last_sent_at` outbound pulse field on `state_update`, edge-triggered `notification` SSE emission on done/waiting_input/permission_required/budget_exceeded (§7.1–§7.2, §8.1, Task 13); notification settings endpoint + optional inbox endpoint (§7.4–§7.5, Task 14); UI notification client — SSE handler, Web Notifications API permission flow, `visibilityState`-based desktop-vs-toast, per-type mute filtering (§7.3, Task 15); Settings notifications panel (Task 16); card unread badge + outbound pulse (Task 17); optional inbox view (Task 18). Maps onto Tasks 13–18 (§10).
- **Depends on:** Subphase 5.3 (messages flowing + budget breaches emitting).
- **Done when (checkpoint):** `go build ./...` AND `npm run build` pass; the mute-suppresses-one-type and backgrounded-`done`→Web Notification client tests pass, alongside all existing Go and UI tests.
- **Resume note:** at start messaging + nudger work end-to-end server-side but nothing is shown in the UI; begin with the state-manager `notification`/indicator emission, then the React notification client and Settings panel.
- **Size:** M.

## 10. Implementation task breakdown (ordered)

1. **MCP handshake spike (Task 1, §2.2)** — register the go-sdk server with **Claude Code** and **Codex**; determine per CLI whether the HTTP-transport entry works or the stdio-subcommand fallback is needed. Record the per-backend outcome; it drives task 8.
2. **`messages` + `turn_budget` schema** (§4.1, §6.1) — add tables/indexes to the `state.db` migration; extend `Store` with the messaging methods (§3.2).
3. **In-process MCP server skeleton** — construct `mcp.Server`, mount the streamable HTTP transport on the existing localhost listener, stand up the `token → agent_id` session registry (§3.1).
4. **Implement `list_agents`** (§3.3) — handler over `Store.LiveAgents()`.
5. **Implement `send_message`** (§3.4) — `to` resolution, transactional insert, outbound budget check.
6. **Implement `check_messages`** (§3.5) — read/mark/delete, inbound budget check.
7. **Stdio-subcommand `agentdeck mcp`** (§2.2 (B)) — same binary; thin proxy to the running server over the localhost transport (build only if Task 1 shows a CLI needs it; otherwise stub + test).
8. **Go: `RegisterMessagingMCP` helper** (§3.6) — mint token, record session mapping, emit per-agent `.mcp.json` (HTTP or stdio per Task 1 result), return launch args + cleanup (removes config, revokes token); wire into Phase 1 launch composition for both `claude-acp` and `codex-acp`.
9. **Go: chat-runtime `CheckMessages(pid)`** (§5.2) — replace Phase 1 stub; inject nudge turn with idle re-check.
10. **Go: turn boundary + budget reset** (§6.2) — runtime upserts/resets the `turn_budget` row at each turn start.
11. **Go: nudger loop** (§5) — ticker + insert-driven detection, in-flight/cooldown guards, dispatch `CheckMessages`.
12. **Go: janitor** (§4.3) — retention `DELETE`s, every 60s.
13. **Go: state manager extensions** — recompute `unread_messages` on message-row change; edge-triggered `notification` emission on done/waiting_input/permission/budget; outbound pulse field.
14. **Go: notification settings endpoint** (§7.4) + optional inbox endpoint (§7.5).
15. **UI: notification client** — SSE `notification` handler, Web Notifications permission flow, visibility-based desktop vs toast, mute filtering.
16. **UI: Settings — notifications panel** — per-type mute toggles + desktop master switch, persisted via the endpoint.
17. **UI: card indicators** — unread mail badge + outbound pulse from `state_update`.
18. **UI: (optional) inbox view** in chat panel via the messages endpoint.
19. **Tests** (§11).

---

## 11. Testing strategy

Map each test to an acceptance criterion. Use a temp `state.db` for isolation.

- **send → nudge → process without user action (F8 core):**
  - Integration: launch two real (or stubbed-ACP) chat agents, implementer + reviewer; reviewer idle. Implementer calls `send_message("reviewer@<proj>", ...)`. Assert (a) a `messages` row appears with `to_agent == reviewer_id`, (b) within `NudgeInterval+timeout` the reviewer's `state` goes `idle→busy`, (c) reviewer's transcript shows a `check_messages` call, (d) the row becomes `read=1` with `delivered_via='nudge'` — **all with zero user prompts.**
  - Unit: nudger detection (idle + unread → candidate; busy/done/waiting → skip).
- **budget caps a deliberate loop:**
  - Drive a stub agent that, on each nudge, sends a reply (ping-pong). Assert that within a single turn no more than 15 combined inbound+outbound actions succeed, the 16th `send_message` returns `message_budget_exceeded`, the `turn_budget` row has `breached=1`, a WARN is logged, and a `budget_exceeded` SSE fires.
  - Unit: budget read-increment-check across simulated concurrent tool calls stays correct (single-writer transaction).
- **mute suppresses one type:**
  - Set `notifications.muted.done = true`. Trigger a `done` and a `waiting_input`. Assert the client drops the `done` (no desktop call, no toast) and shows the `waiting_input`. Test the filter purely in the client with a mocked SSE stream + a stubbed `Notification`.
- **backgrounded done → notification:**
  - Mock `document.visibilityState = "hidden"` and `Notification.permission = "granted"`; emit `notification{type:"done"}`; assert `new Notification(...)` called once with `tag === agent_id`. With `visibilityState = "visible"`, assert a toast instead and no `Notification`.
- **resolution & errors:** unit-test `to` resolution for `agent_id` / `role@project` / name / ambiguous / not-found / stopped recipient → correct result/error shapes (§9).
- **session identity:** assert a tool call on an unknown/revoked session token returns `session_unknown` and that `from` always equals the session's `agent_id` regardless of any argument.
- **registration:** assert `RegisterMessagingMCP` mints a token, records the session mapping, emits a valid `.mcp.json` (HTTP or stdio per backend) with correct endpoint/args, and cleanup removes it **and revokes the token** on Stop, for `claude-acp` and `codex-acp`.
- **retention:** janitor deletes read>24h and any>7d; leaves fresh + recent-read rows.
- **transport fallback (if Task 1 requires it):** drive the `agentdeck mcp` stdio subcommand; assert it proxies a `list_agents`/`send_message`/`check_messages` call to the running server and returns the in-process result unchanged.

---

## 12. Interfaces produced for later phases

Phase 7 (activity map) consumes, with no new data needed:
- **`notification` SSE events** (§8.1) — markers can flash on `done`/`waiting_input`/`permission_required`.
- **Outbound message signal** — `state_update.last_sent_at` (§7.1) plus, if richer animation is wanted, the message-row inserts the state manager already observes. Phase 7 can animate a "message in flight" from sender→recipient using `from_agent`/`to_agent` on the row. The `messages` table (with `from_agent`/`to_agent`/`created_at`) is a stable, queryable record for any later visualization.
- **`unread_messages` per agent** (§7.1, §8.1) — for showing pending mail on map markers.
- **Per-turn budget data** in the `turn_budget` table — available to a future cross-turn loop-detection phase (§13).

No breaking changes to Phase 1/2 contracts: all additions are new SSE event types (`notification`) or new optional fields on existing events (`state_update`).

---

## 13. Resolved decisions

Answering the PRD §6 / master §9 open questions concretely (architecture rationale: D3).

1. **Is a 15/turn budget enough, or is cross-turn loop detection needed?**
   **Ship the per-turn budget (15, combined inbound+outbound) for Phase 5; do not build cross-turn detection now — track it as a follow-up.** Rationale: the per-turn cap deterministically kills the dangerous case (an agent flooding messages within one turn / a tight wake-reply-wake spin, throttled further by `NudgeCooldown` and one-nudge-per-idle-window). The residual risk — two agents trading one message per turn forever — is bounded in *rate* (gated by nudge interval + cooldown, so it's slow, visible on the dashboard, and human-interruptible) but not in *total*. We keep the data needed to address it later: per-turn budget rows in `state.db` and the full message history in the `messages` table. **Follow-up (post-Phase 5):** a cross-turn detector that flags an A↔B pair exchanging >N messages over a rolling window and auto-pauses one side. Not in this phase.

2. **Retention policy for read message rows.**
   **Adopted (§4.3):** Go janitor every 60s. Delete `read` messages older than 24h; delete any message older than 7 days (hard cap). Stopped/archived agents keep their rows until the 7-day cap (so a resumed agent sees recent mail) but are not nudged. Thresholds are Go constants (`MailReadTTL=24h`, `MailHardTTL=168h`); promoting them to `config.json` is optional.

3. **MCP registration mechanics per CLI (Claude Code vs Codex).**
   **Resolved by Task 1 (§2.2, §3.6).** The dashboard hosts one in-process server and registers each agent against it via a per-agent session token. For each CLI, Task 1 determines whether the **HTTP-transport** entry is accepted or the **stdio-subcommand** fallback (the same `agentdeck` binary's `mcp` subcommand, proxying to the running server) is needed. Either way the handlers run in-process against `state.db`, and identity is taken from the registered session token — never from a spoofable tool argument — so cross-agent spoofing is impossible regardless of backend.

### Locked constants (single reference)

| Constant | Value |
|---|---|
| `MessageBudgetPerTurn` | 15 (combined inbound + outbound) |
| `NudgeInterval` | 2s |
| `NudgeCooldown` | 3s |
| `NudgeInFlightTimeout` | 60s |
| `JanitorInterval` | 60s |
| `MailReadTTL` | 24h |
| `MailHardTTL` | 168h (7d) |
| MCP SDK | `github.com/modelcontextprotocol/go-sdk` (v1.x) |
| `message_id` format | `m_` + 6 hex |
| `turn_id` format | `t_` + counter |
