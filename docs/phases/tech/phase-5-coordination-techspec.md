# Phase 5 — Coordination: Implementation Tech Spec

**Mirrors:** `docs/phases/phase-5-coordination.md` (phase PRD)
**Master PRD refs:** §4.5 (MCP messaging server, nudger, message budget), F8, F11
**Status:** ready to implement after Phases 1 and 2
**Features:** F8 (agent-to-agent messaging), F11 (notifications)

This is the implementation-level companion to the Phase 5 PRD. It is prescriptive: real tool schemas, real message JSON, exact config shapes, concrete intervals and counters. There should be no design decisions left to make at implementation time. This is the only phase with a substantial Node.js component (the MCP messaging server); everything else (nudger, budgets, notification emission, SSE) lives in the Go server, and the dashboard pieces in the React/TS UI.

---

## 1. Overview & scope recap

### In scope
- **Node.js MCP messaging server** (`mcp-messaging/`), spawned and supervised by the Go server. One long-lived process shared by all agents (not one per agent).
- **Three MCP tools:** `list_agents`, `send_message(to, body)`, `check_messages`.
- **File-based mailbox** delivery to `~/.agentdeck/messages/{recipient_id}/`, one `.json` per message, with read/flag/delete semantics and a retention policy.
- **Nudger:** a Go server loop that detects an idle agent (`status.state == "idle"`) with pending unread mail and calls `Runtime.CheckMessages(pid)` to wake it.
- **Chat-runtime `CheckMessages`** implementation (stubbed in Phase 1).
- **Per-turn message budget** (default 15) capping messages processed/sent per turn, with breach logging + SSE surfacing.
- **Dashboard message indicators** on sender/recipient cards (unread/outbound badge), driven via SSE.
- **Notifications (F11):** SSE `notification` events on `done`, `waiting_input`, `permission_required`; OS desktop notifications when the dashboard tab is backgrounded; in-app toasts when foregrounded; per-type mute persisted to `config.json`.
- **Optional** `GET /api/sessions/{id}/messages` inbox endpoint.

### Out of scope (explicit non-goals)
- **Cross-turn loop detection.** Only the per-turn budget (15/turn) is built. Detecting a slow ping-pong that stays under budget every turn but never terminates across turns is an **open question, not built** (see §13). We add hooks (counters in `running/`) that a future phase can read, but no cross-turn termination logic ships here.
- **Activity-map animation** (Phase 7). Phase 5 only *emits* the `new_message` / `notification` events Phase 7 will animate on (see §12).
- **Threaded conversations / reply chains.** Messages carry an optional `in_reply_to` field for future use, but no threading UI or semantics ship.
- **Terminal-runtime `CheckMessages`.** Only the chat runtime's `CheckMessages` is implemented here. Terminal runtime (Phase 6) wires its own; this phase returns a typed "not implemented" for `interface: "terminal"` (consistent with Phase 1's registry behavior).

---

## 2. Technology choices

### 2.1 MCP SDK & language

- **`@modelcontextprotocol/sdk` (TypeScript), pinned to `^1.29.0`.** This is the official MCP TypeScript SDK; it provides `McpServer` and `StdioServerTransport`, the exact primitives we need. Pin in `mcp-messaging/package.json` with an exact lockfile (`package-lock.json` committed) so the supervised binary is reproducible.
- **Peer dependency:** `zod` (`^3.25.0` or `^4`; the SDK imports `zod/v4` internally but is back-compatible to Zod 3.25+). We author tool input schemas in Zod; the SDK derives the JSON Schema advertised to the CLI from them.
- **Runtime:** Node 18+ (matches the master PRD prereq). Use ESM (`"type": "module"`).
- **Why Node, not Go:** the master PRD fixes the MCP server as Node.js (§4.5, §7), and the official, best-supported MCP server SDK is the TypeScript one. The Go server stays the orchestrator; the Node process is a thin, stateless tool host over the same file store.
- **Why one shared process, not per-agent:** all tools operate on the shared `~/.agentdeck/` file store and need a global view (`list_agents`). A single process is cheaper to supervise and lets every agent's CLI connect to the same stdio server instance via its own MCP client transport. The CLI multiplexes; the server is stateless between calls (it reads the file store fresh each call), so concurrency is naturally safe.

> **Transport decision.** Each agent CLI spawns its **own** stdio connection to the MCP server. There are two viable shapes; we pick (B):
>
> **(A) Single shared process, multiple stdio clients** — not possible with one process over a single stdio pair (stdio is 1:1). Rejected.
>
> **(B) One MCP server *process per agent*, all running the same code.** The CLI launches the MCP server as its child over stdio (standard MCP stdio pattern). The Go server does **not** keep the Node process alive itself; instead the **registration** (see §3.5) tells each agent CLI to spawn `node mcp-messaging/dist/server.js` as its MCP stdio child. The "shared state" is the file store, not a shared process. This is the canonical MCP stdio model and avoids inventing a custom multiplexing transport.
>
> We adopt **(B)**. "Launched/managed by the Go server" (PRD §4.5) is satisfied by: the Go server (a) ensures the Node bundle is built/available and its path is correct, (b) writes the per-agent MCP registration that causes the CLI to spawn it, and (c) passes `AGENTDECK_HOME` + the caller's `agent_id` via env so each spawned MCP server knows who it is acting for. Supervision of the per-agent MCP child is delegated to the agent CLI's own MCP lifecycle (it restarts/kills it with the session). The Go server additionally **validates** the Node toolchain at startup (see §2.2) and surfaces a clear error if missing.

### 2.2 How the Go server launches & supervises Node

The Go server's responsibilities around the Node component:

1. **Build/availability check at server startup.** On `agentdeck dashboard start`, the Go server verifies `mcp-messaging/dist/server.js` exists and `node` is on `PATH` (resolve via `exec.LookPath("node")`). If the bundle is missing but `mcp-messaging/` source is present, run `npm ci && npm run build` once (logged); if `node` is absent, log a clear warning and **disable messaging** (agents still launch; messaging tools simply aren't registered). Messaging being unavailable is a degraded mode, never a launch blocker.
2. **No persistent Node process owned by Go.** Per the (B) decision, the Node MCP server runs as a child of each agent CLI. The Go server therefore does not `os/exec` the Node process directly for normal operation.
3. **Registration injection at agent launch** (§3.5): the Go server writes the MCP server config the CLI reads, pointing at `node {abs}/mcp-messaging/dist/server.js` with the right env.
4. **Health surfacing:** the MCP server writes a one-line heartbeat to `~/.agentdeck/messages/.mcp-health/{agent_id}.json` (`{"pid":..., "started_at":..., "sdk_version":...}`) on startup and deletes it on clean exit. The Go server's existing file watcher can optionally watch this dir to show "messaging connected" per card. (Nice-to-have; not required for acceptance.)

### 2.3 IPC / registration mechanics (summary; full detail in §3.5)

- **Agent → MCP server:** MCP JSON-RPC over **stdio** (the CLI is the MCP *client*, the Node process is the MCP *server*). This is the SDK's `StdioServerTransport`.
- **MCP server → file store:** direct synchronous file reads/writes under `AGENTDECK_HOME` (defaults to `~/.agentdeck/`). No network, no socket to the Go server.
- **MCP server → Go server:** **none required.** The nudger (Go) observes mailbox files via the file watcher; the MCP server observes agent state via the file store. The file store is the only IPC channel between the two. This keeps the Node process completely decoupled and crash-isolated.

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

Location: `mcp-messaging/` (sibling to the Go server and UI). Entry: `src/server.ts` → bundled to `dist/server.js`.

### 3.1 Process identity & environment

The MCP server is spawned per agent by that agent's CLI. It learns who it is acting for from env, injected by the Go server's registration step:

| Env var | Meaning | Source |
|---|---|---|
| `AGENTDECK_HOME` | Root of the file store | Go server (inherits its own resolved value) |
| `AGENTDECK_SELF_ID` | The `agent_id` of the agent this MCP instance serves | Go server, per-agent at registration |

`AGENTDECK_SELF_ID` is the linchpin: it makes `send_message`'s `from` and `check_messages`'s mailbox unambiguous **without trusting any tool argument**. The caller cannot spoof another agent's identity because identity comes from the spawn env, not the tool call.

### 3.2 File store access helpers

A small `store.ts` module wraps reads:
- `readAgents(): AgentIdentity[]` — read all `agents/*.json`.
- `readRunning(): Map<agent_id, Running>` — read all `running/*.json`.
- `readStatus(id): Status | null` — read `status/{id}.json`.
- `liveAgents(): LiveAgent[]` — agents that have a `running/{id}.json` (i.e. currently active), joined with identity + status.
- Mailbox helpers: `mailboxDir(id)`, `listMessages(id)`, `writeMessage(id, msg)`, `markRead(id, msgId)`, `deleteMessage(id, msgId)`.

All reads are fresh per tool call (no caching) — the file store is the source of truth and tool calls are infrequent relative to file I/O cost. Writes are atomic (write to `*.json.tmp`, `fsync`, `rename`) to avoid the Go file watcher seeing partial files.

### 3.3 Tool: `list_agents`

Discover other live agents. **Excludes the caller** (`AGENTDECK_SELF_ID`).

**Input schema (Zod → JSON Schema):**
```ts
list_agents: {
  include_self: z.boolean().optional().default(false), // include the caller in results
  state: z.enum(["busy","idle","waiting_input","done","error"]).optional() // filter
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
      "state": "idle",                     // from status/{id}.json (or "unknown")
      "detail": "Idle",
      "context_pct": 0.42
    }
  ]
}
```

**Sourcing:** `liveAgents()` — only agents with a `running/{id}.json` are listed (stopped/archived agents are not addressable). `state`/`detail`/`context_pct` come from `status/{id}.json`; if missing, `state: "unknown"`. This is exactly the "live state from the file store / state manager" the PRD asks for, read directly from disk (the state manager is a Go consumer of the same files; the MCP server reads the files itself rather than calling Go).

### 3.4 Tool: `send_message(to, body)`

Drop a message into the recipient's mailbox.

**Input schema:**
```ts
send_message: {
  to: z.string().min(1),       // "role@project" OR agent name OR agent_id
  body: z.string().min(1).max(8000),
  subject: z.string().max(200).optional(),
  in_reply_to: z.string().optional()   // message_id being replied to (forward-compat; not threaded yet)
}
```

**`to` resolution order** (first match wins):
1. **Exact `agent_id`** — if `to` matches a live agent's `agent_id`.
2. **`role@project`** — split on `@`; match a live agent whose `role` and `project` both equal the parts. If multiple live agents share the same `role@project` → **ambiguous**: return an error listing the candidate `agent_id`s and their names, instructing the caller to address by name or id.
3. **Name (case-insensitive)** — match a live agent whose `name` equals `to`. Same ambiguity rule on duplicate names.
4. **No match** → error (see §9 for the exact error shape; recipient nonexistent/stopped).

Resolution only ever targets **live** agents (those with `running/{id}.json`). Sending to a stopped agent is an error, surfaced to the sender (§9).

**On success**, write `messages/{recipient_id}/{message_id}.json` (schema in §4.1), where `from` = `AGENTDECK_SELF_ID` (env, not argument). Increment the sender's **outbound** per-turn counter (§6); if the budget is exceeded, the message is **not** written and a budget-breach error is returned (§6).

**Returns:**
```jsonc
{ "ok": true, "message_id": "m_1a2b3c", "to": "a_reviewer_id", "to_address": "reviewer@my-app" }
```

### 3.5 Tool: `check_messages`

Read + flag/delete the caller's pending messages. Caller is always `AGENTDECK_SELF_ID`.

**Input schema:**
```ts
check_messages: {
  mark_read: z.boolean().optional().default(true),   // flag returned messages as read
  delete_after: z.boolean().optional().default(false), // delete returned messages after reading
  unread_only: z.boolean().optional().default(true),
  limit: z.number().int().min(1).max(50).optional().default(15) // hard-capped at the per-turn budget
}
```

**Behavior:**
1. List `messages/{self_id}/*.json`, sorted by `created_at` ascending.
2. Filter to `unread === false`-pending if `unread_only`.
3. Take up to `min(limit, remaining_inbound_budget)` (§6). Reading is itself budget-governed: messages processed per turn count against the inbound budget.
4. If `mark_read`, set `read: true` + `read_at` on each returned message (atomic rewrite). If `delete_after`, delete instead (delete wins over mark_read).
5. Increment the **inbound** per-turn counter by the number returned.

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

Phase 1's launch composition gains an **MCP registration step** between "compose config" and "Runtime.Start". The Go server, having resolved the absolute path to `node` and `mcp-messaging/dist/server.js`, registers the MCP server **per agent** with the env from §3.1.

**Claude Code (`claude-acp`):** Claude Code reads MCP servers from a JSON config (`.mcp.json` / `--mcp-config <file>` / settings). The Go server writes a per-agent MCP config file at `~/.agentdeck/mcp/{agent_id}.mcp.json`:
```jsonc
{
  "mcpServers": {
    "agentdeck-messaging": {
      "command": "node",
      "args": ["/abs/path/to/mcp-messaging/dist/server.js"],
      "env": {
        "AGENTDECK_HOME": "/Users/me/.agentdeck",
        "AGENTDECK_SELF_ID": "a_8f3c12"
      }
    }
  }
}
```
and passes `--mcp-config /Users/me/.agentdeck/mcp/a_8f3c12.mcp.json` (or the CLI's equivalent flag) in the launch invocation.

**Codex (`codex-acp`):** Codex registers MCP servers via its own config (e.g. a `mcp_servers` table in its TOML/CLI). The Go server emits the equivalent registration with the same `command`/`args`/`env` triple. Because both CLIs spawn the identical `node dist/server.js` with the same env, **the same server binary serves both backends cleanly** (confirms PRD §6 open question — see §13).

**Abstraction in Go:** add a `RegisterMessagingMCP(agent, backendType) (launchArgs []string, cleanup func())` helper in the launch path. It returns the extra CLI args and a cleanup that removes `~/.agentdeck/mcp/{agent_id}.mcp.json` on Stop. This keeps backend-specific registration wiring in one place; the Phase 1 launch composition just calls it.

> **Phase 1 amendment note:** Phase 1's spec says "registers the MCP messaging server" in the launch flow (F4 step 2) but stubs the actual tool. Phase 5 fills in (a) the real MCP server, and (b) the `RegisterMessagingMCP` helper that produces the launch args. If Phase 1 already added a placeholder registration, Phase 5 replaces its body with the above.

---

## 4. Mailbox + delivery design

### 4.1 Message file schema

Path: `~/.agentdeck/messages/{recipient_id}/{message_id}.json`. One file per message.

```jsonc
{
  "message_id": "m_1a2b3c",            // unique: "m_" + 6 hex (collision-checked at write)
  "from": "a_impl_id",                  // sender agent_id (from AGENTDECK_SELF_ID, never spoofable)
  "from_address": "implementer@my-app", // resolved at send time for display convenience
  "from_name": "Atlas",
  "to": "a_reviewer_id",                // recipient agent_id (== the mailbox dir owner)
  "subject": "Review request",          // optional, may be ""
  "body": "Please review the diff on src/auth.ts",
  "created_at": "2026-06-22T10:05:00Z", // RFC3339 UTC
  "read": false,                        // flipped true by check_messages mark_read
  "read_at": null,                      // RFC3339 when read, else null
  "delivered_via": "pending",           // "pending" | "nudge" | "poll" (set by nudger/check)
  "in_reply_to": null                   // message_id or null (forward-compat)
}
```

- **`message_id`** generated by the MCP server: `m_` + crypto-random 6 hex; on collision in the target dir, regenerate.
- **Atomic write:** `{message_id}.json.tmp` → `fsync` → `rename` to `{message_id}.json`, so the Go file watcher never reads a partial file.
- The recipient mailbox dir is created on first send (`mkdir -p messages/{recipient_id}/`).

### 4.2 Read / flag / delete semantics

- **Unread:** `read === false`. Drives the recipient card's unread badge and the nudger's "pending mail" check.
- **Flag read:** `check_messages` with `mark_read: true` (default) sets `read: true`, `read_at`. The file remains on disk (so the inbox endpoint can show recent history) until cleanup.
- **Delete:** `check_messages` with `delete_after: true` removes the file immediately after returning it. Agents that don't want history pass this.
- **`delivered_via`:** the nudger sets `delivered_via: "nudge"` on unread messages when it wakes the recipient (audit trail); `check_messages` invoked by polling sets `"poll"` on messages it returns that were still `"pending"`. Purely diagnostic.

### 4.3 Retention / cleanup policy (proposal — adopted)

A **Go-side janitor** (runs alongside the nudger loop, every 60s):
- Delete `read === true` messages whose `read_at` is older than **24h** (read messages are transient by default).
- Delete **any** message (read or not) older than **7 days** (`created_at`) — a hard cap so an offline/stopped recipient's mailbox can't grow unbounded.
- When an agent is **stopped/archived** (its `running/{id}.json` is removed), leave its mailbox in place until the 7-day cap (so a resumed agent still sees recent mail), but **stop the nudger** from acting on it (no live `running/` → no pid to nudge).
- Cleanup is logged at debug level with counts. These thresholds live as constants in the Go config (`MailReadTTL = 24h`, `MailHardTTL = 168h`, `JanitorInterval = 60s`); exposing them in `config.json` is optional and not required for acceptance.

---

## 5. Nudger design

The nudger is a **Go server loop** (not Node). It closes the loop so an idle recipient processes mail without user action (F8 acceptance).

### 5.1 Detection

- The nudger maintains a ticker (default **2s** interval, `NudgeInterval`). On each tick (and additionally, event-driven, when the file watcher reports a write under `messages/`):
  1. Enumerate live agents (those with `running/{id}.json`).
  2. For each, check `status/{id}.json`: act only if `state == "idle"` (per PRD: idle agents with pending mail). `done` is **not** nudged (it has finished; the user decides next step). `waiting_input`/`busy`/`error` are skipped.
  3. Check the agent's mailbox for **unread** messages (`read === false`). If ≥1 and the agent is idle → it's a nudge candidate.
- Combine ticker + watcher-driven checks so latency is low (watcher fires on new mail) but a missed event is still caught within `NudgeInterval`.

### 5.2 Waking

For a candidate, call `Runtime.CheckMessages(pid)` via the runtime registry (dispatched by `agent.interface`), passing the pid from `running/{id}.json`.

**Chat-runtime `CheckMessages(pid)` implementation** (the Phase 1 stub, now real):
- The chat runtime holds the live ACP/stdio session for the agent (established at `Start`). `CheckMessages` injects a **system-level nudge turn** into that session instructing the agent to call the `check_messages` MCP tool and act on its mail. Concretely, it sends a minimal user/system turn over ACP:
  > `You have new messages. Call the check_messages tool and handle them.`
- This is a normal turn from the runtime's perspective: status transitions `idle → busy`, the agent calls `check_messages`, processes mail, possibly calls `send_message`, and returns to `idle`. The transcript deltas stream as usual (`new_message` SSE), and the user sees the agent "wake up."
- Before injecting, the runtime **re-checks** the agent is still `idle` (guards against a race where the user just sent a prompt — see §5.4). If not idle, it no-ops and returns; the nudger will retry next tick.
- Mark the triggering messages `delivered_via: "nudge"` (the nudger does this on the files before/after the wake for the audit trail).

### 5.3 Scheduling / avoiding double-wakes

- **In-flight set:** the nudger keeps an in-memory `map[agent_id]nudgeState` with `{lastNudgeAt, inFlight bool}`. When it wakes an agent, set `inFlight = true`. Clear `inFlight` only when the agent returns to `idle` **and** has no unread mail, OR after a **timeout** (`NudgeInFlightTimeout = 60s`) to avoid a permanently stuck flag if a turn hangs.
- **Cooldown:** even after `inFlight` clears, enforce a minimum `lastNudgeAt + NudgeCooldown (3s)` before re-nudging the same agent. This prevents a tight nudge→idle→nudge spin.
- **Single nudge per idle window:** an agent that is woken processes *all* available mail (up to budget) in one turn, so one nudge per idle→busy→idle cycle suffices; the `inFlight` flag enforces exactly one outstanding wake per agent.
- The nudger is **single-goroutine** (the loop) dispatching wakes; `CheckMessages` calls are synchronous-with-timeout so a slow runtime can't stall the loop indefinitely (run each wake in a goroutine with the in-flight guard; the loop itself never blocks > the tick).

### 5.4 Interaction with user turns

If the user sends a prompt to an agent that the nudger is about to wake, the user turn wins (the runtime's `idle` re-check in §5.2 fails and the nudge no-ops). The pending mail stays unread and will be picked up on the next idle window (or the agent may call `check_messages` itself mid-turn). No mail is lost.

---

## 6. Per-turn budget

Cap the messages an agent processes/sends **per turn** to prevent runaway agent-to-agent loops. **Default 15**, constant `MessageBudgetPerTurn`.

### 6.1 Where the counter lives

The budget is **per agent, per turn**, and must be visible to the MCP server (which enforces it on `send_message`/`check_messages`). Since the MCP server is per-agent and stateless between calls, the counter is persisted in the **`running/{agent_id}.json`** file (already watched, already per-agent) under a `message_budget` block:

```jsonc
// running/{agent_id}.json (Phase 5 additions)
{
  "agent_id": "a_8f3c12",
  "pid": 48213,
  "session_id": "...",
  "interface": "chat",
  "started_at": "...",
  "message_budget": {
    "turn_id": "t_77",        // opaque id of the current turn (see 6.2)
    "inbound": 0,             // messages read this turn via check_messages
    "outbound": 0,            // messages sent this turn via send_message
    "breached": false         // set true on first breach this turn
  }
}
```

- **`inbound + outbound` together count against `MessageBudgetPerTurn`** (a single shared cap of 15 covers both reading and sending in one turn — this is the strictest reading of "messages processed/sent per turn" and most aggressively kills loops).
- The MCP server reads/increments this block on each tool call (atomic read-modify-write of `running/{id}.json`). Because each agent has its own MCP server instance and its own `running/` file, there's no cross-agent contention on a single file; within one agent, tool calls are serialized by the CLI's MCP client, so the read-modify-write is effectively single-writer.

### 6.2 Turn boundary (`turn_id`)

A "turn" = one user/nudge prompt → agent response cycle. The **Go runtime** owns the turn boundary: when the chat runtime begins a new turn (user prompt or nudge injection), it sets a fresh `turn_id` (`t_` + counter) in `running/{id}.json` and **resets `inbound=0, outbound=0, breached=false`**. The MCP server, on each tool call, reads the current `turn_id`; the counters it increments are scoped to that turn. (The MCP server never resets counters — only the runtime does, at turn start. This cleanly separates concerns: runtime defines turns, MCP server enforces the cap within them.)

### 6.3 Breach handling

When a tool call would push `inbound + outbound` past `MessageBudgetPerTurn`:
- **`send_message`:** refuse to write the file. Return an error result:
  ```jsonc
  { "ok": false, "error": "message_budget_exceeded",
    "message": "Per-turn message budget (15) reached. This message was not sent.",
    "budget": 15, "used": 15 }
  ```
- **`check_messages`:** return only up to the remaining budget; if zero remaining, return `messages: []` with `"budget_remaining": 0` and an explanatory note so the agent understands why it got nothing.
- **Set `breached: true`** in `running/{id}.json` on first breach in the turn. The Go file watcher sees the change and the Go server **emits an SSE `notification`** of type `budget_exceeded` (see §8) plus logs `WARN budget exceeded agent=<id> turn=<turn_id>`. The dashboard shows a non-fatal toast/badge ("Atlas hit its message budget this turn"). This is the "logged and surfaced" requirement.
- Breach is **not** fatal: the agent's turn continues normally; only further messaging is blocked until the next turn resets the counter.

### 6.4 Why this caps loops

Two agents pinging each other: each ping is an outbound (sender) + an inbound (recipient) within their respective turns. After 15 messaging actions in a single turn an agent is blocked from sending/reading more, so an in-turn flood is hard-capped. A slower cross-turn ping-pong is **not** stopped by this (see §13).

---

## 7. Dashboard indicators + notifications (F11)

### 7.1 Card message indicator

- **Recipient indicator (unread badge):** driven by mailbox state. The Go file watcher already watches `~/.agentdeck/`; extend it to also watch `messages/`. On any change under `messages/{id}/`, recompute the agent's unread count (`count of read === false`) and fold it into that agent's `state_update` SSE payload as `unread_messages: N`. The card renders a mail badge with the count when `N > 0`.
- **Sender (outbound) indicator:** a brief "sent" pulse. When `running/{id}.json`'s `message_budget.outbound` increments, the `state_update` for the sender carries `last_sent_at` (timestamp). The card shows a transient outbound icon for ~2s. (This doubles as the signal Phase 7's activity map animates on — see §12.)
- No new SSE event type is needed for indicators; they ride on the existing `state_update` (per PRD §4: "the dashboard reads mailbox/indicator state via `state_update`/`new_message`").

### 7.2 Notification SSE events

The **Go server** emits `notification` SSE events on significant transitions, detected by the state manager when `status/{id}.json` changes:
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

Persisted in **`config.json`** under a new `notifications` key:

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
- The client loads the block on startup and on `state_update`-style settings change; mute toggles in Settings write back via the endpoint. Muting a type sets its `muted.{type} = true`; the client then drops both desktop and in-app for that type.

### 7.5 Optional inbox endpoint

`GET /api/sessions/{id}/messages` — read-only inbox view for the chat panel / a future inbox UI. JSON in §8.2. Implemented in **Go** (reads `messages/{id}/*.json` directly; does not touch the MCP server). Marked optional in the PRD; we include it because the chat panel benefits and it's cheap. Query params: `?unread_only=true&limit=50`.

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
| **Runaway loop (two agents pinging)** | Per-turn budget (§6) hard-caps in-turn floods at 15 combined inbound+outbound; breach blocks further messaging that turn, logs WARN, emits `budget_exceeded`. Cross-turn slow loops are **not** stopped (open question, §13) — but counters in `running/` give a future phase the data to detect them. |
| **Node MCP process crash** | Isolated: it's a child of the agent CLI. The CLI's MCP client surfaces the tool as unavailable; the agent's turn continues (tool calls fail with an MCP error the agent can read). On the agent's next turn the CLI re-spawns its MCP child (standard MCP stdio lifecycle). No Go-side restart logic needed. The Go server's startup toolchain check (§2.2) catches the "node missing" case up front. |
| **MCP server can't read file store** (`AGENTDECK_HOME` bad / perms) | Each tool returns a structured error (`{"ok":false,"error":"store_unavailable","message":...}`); the agent sees it and reports. Heartbeat file (§2.2) absent → dashboard can show "messaging unavailable." |
| **Message to nonexistent recipient** | `send_message` resolution fails → `{"ok":false,"error":"recipient_not_found","message":"No live agent matches 'reviewer@my-app'.","candidates":[...]}` (candidates empty here). Sender's agent reads the error and can `list_agents` to find a valid target. |
| **Message to stopped/archived recipient** | Same `recipient_not_found` (resolution only targets live agents). The sender is told the recipient isn't running. (We deliberately do not silently queue to a dead mailbox — it would never be delivered.) |
| **Ambiguous `to`** (duplicate `role@project` or name) | `{"ok":false,"error":"ambiguous_recipient","candidates":[{"agent_id","name","address"}...]}`; sender re-sends by `agent_id`. |
| **Nudge while agent transitions to busy** | Nudger's pre-injection `idle` re-check (§5.2) no-ops if not idle. `inFlight` flag (§5.3) prevents a second concurrent wake. User turns win (§5.4). |
| **Double-wake / nudge storm** | `inFlight` map + `NudgeCooldown (3s)` + one-nudge-per-idle-window (§5.3). In-flight timeout (60s) prevents a stuck flag. |
| **Budget counter / `running.json` write race** | Per-agent file, single CLI serializing that agent's tool calls → effectively single-writer. Atomic tmp+rename writes. The runtime (turn reset) and MCP server (increments) both write `running/{id}.json`; they coordinate via the `turn_id` (runtime only writes at turn boundary; MCP server only between boundaries) so they don't clobber each other's fields — each does a read-merge-write touching disjoint fields under `message_budget`. |
| **Partial file read by watcher/MCP** | All writers use tmp+fsync+rename; readers that hit a transient missing file (mid-rename) retry once. |
| **Recipient mailbox dir missing** | `send_message` `mkdir -p`s it; `check_messages` treats missing dir as empty inbox. |
| **`status/{id}.json` missing for a live agent** | Nudger treats unknown state as not-idle (won't nudge); `list_agents` returns `state:"unknown"`. Conservative: never nudge on uncertainty. |

---

## 10. Implementation task breakdown (ordered)

1. **Scaffold `mcp-messaging/`** — `package.json` (ESM, deps: `@modelcontextprotocol/sdk@^1.29.0`, `zod`), `tsconfig`, bundler (esbuild) → `dist/server.js`. Commit lockfile.
2. **`store.ts`** — file-store read/write helpers + atomic write + `liveAgents()`.
3. **MCP server skeleton** — `McpServer` + `StdioServerTransport`, read `AGENTDECK_HOME` / `AGENTDECK_SELF_ID`, write heartbeat file on start.
4. **Implement `list_agents`** (§3.3) with Zod schema.
5. **Implement `send_message`** (§3.4): `to` resolution, atomic message write, outbound budget check.
6. **Implement `check_messages`** (§3.5): read/mark/delete, inbound budget check.
7. **Go: `RegisterMessagingMCP` helper** (§3.6) — emit per-agent `.mcp.json`, return launch args + cleanup; wire into Phase 1 launch composition for both `claude-acp` and `codex-acp`.
8. **Go: startup toolchain check** (§2.2) — node + bundle presence; build-if-needed; degrade gracefully.
9. **Go: chat-runtime `CheckMessages(pid)`** (§5.2) — replace Phase 1 stub; inject nudge turn with idle re-check.
10. **Go: turn boundary + budget reset** (§6.2) — runtime sets/resets `message_budget` in `running/{id}.json` at each turn start.
11. **Go: nudger loop** (§5) — ticker + watcher-driven detection, in-flight/cooldown guards, dispatch `CheckMessages`.
12. **Go: janitor** (§4.3) — retention cleanup, every 60s.
13. **Go: state manager extensions** — watch `messages/`; compute `unread_messages`; edge-triggered `notification` emission on done/waiting_input/permission/budget; outbound pulse field.
14. **Go: notification settings endpoint** (§7.4) + optional inbox endpoint (§7.5).
15. **UI: notification client** — SSE `notification` handler, Web Notifications permission flow, visibility-based desktop vs toast, mute filtering.
16. **UI: Settings — notifications panel** — per-type mute toggles + desktop master switch, persisted via the endpoint.
17. **UI: card indicators** — unread mail badge + outbound pulse from `state_update`.
18. **UI: (optional) inbox view** in chat panel via the messages endpoint.
19. **Tests** (§11).

---

## 11. Testing strategy

Map each test to an acceptance criterion. Use `AGENTDECK_HOME` pointed at a temp dir for isolation.

- **send → nudge → process without user action (F8 core):**
  - Integration: launch two real (or stubbed-ACP) chat agents, implementer + reviewer; reviewer idle. Implementer calls `send_message("reviewer@<proj>", ...)`. Assert (a) a file appears in `messages/{reviewer_id}/`, (b) within `NudgeInterval+timeout` the reviewer's `status.state` goes `idle→busy`, (c) reviewer's transcript shows a `check_messages` call, (d) message becomes `read:true` with `delivered_via:"nudge"` — **all with zero user prompts.**
  - Unit: nudger detection (idle + unread → candidate; busy/done/waiting → skip).
- **budget caps a deliberate loop:**
  - Drive a stub agent that, on each nudge, sends a reply (ping-pong). Assert that within a single turn no more than 15 combined inbound+outbound actions succeed, the 16th `send_message` returns `message_budget_exceeded`, `running.json.message_budget.breached === true`, a WARN is logged, and a `budget_exceeded` SSE fires.
  - Unit: budget read-modify-write across simulated concurrent tool calls stays correct.
- **mute suppresses one type:**
  - Set `notifications.muted.done = true`. Trigger a `done` and a `waiting_input`. Assert client drops the `done` (no desktop call, no toast) and shows the `waiting_input`. Test the filter purely in the client with a mocked SSE stream + a stubbed `Notification`.
- **backgrounded done → notification:**
  - Mock `document.visibilityState = "hidden"` and `Notification.permission = "granted"`; emit `notification{type:"done"}`; assert `new Notification(...)` called once with `tag === agent_id`. With `visibilityState = "visible"`, assert a toast instead and no `Notification`.
- **resolution & errors:** unit-test `to` resolution for `agent_id` / `role@project` / name / ambiguous / not-found / stopped recipient → correct result/error shapes (§9).
- **registration:** assert `RegisterMessagingMCP` emits a valid `.mcp.json` with correct abs paths + env for `claude-acp` and `codex-acp`, and cleanup removes it on Stop.
- **retention:** janitor deletes read>24h and any>7d; leaves fresh + recent-read.
- **crash isolation:** kill the MCP child mid-session; assert the agent's turn continues and a later turn still has working tools (re-spawn).

---

## 12. Interfaces produced for later phases

Phase 7 (activity map) consumes, with no new data needed:
- **`notification` SSE events** (§8.1) — markers can flash on `done`/`waiting_input`/`permission_required`.
- **Outbound message signal** — `state_update.last_sent_at` (§7.1) plus, if richer animation is wanted, the `new_message`/message-file write events the file watcher already emits. Phase 7 can animate a "message in flight" from sender→recipient using `from`/`to` on the message file. The mailbox files themselves (`messages/{id}/*.json` with `from`/`to`/`created_at`) are a stable, queryable record for any later visualization.
- **`unread_messages` per agent** (§7.1, §8.1) — for showing pending mail on map markers.
- **Budget/turn data** in `running/{id}.json.message_budget` — available to a future cross-turn loop-detection phase (§13).

No breaking changes to Phase 1/2 contracts: all additions are new SSE event types (`notification`) or new optional fields on existing events (`state_update`).

---

## 13. Resolved decisions

Answering the PRD §6 / master §9 open questions concretely:

1. **Is a 15/turn budget enough, or is cross-turn loop detection needed?**
   **Decision: ship the per-turn budget (15, combined inbound+outbound) for Phase 5; do NOT build cross-turn detection now — track it as a follow-up.** Rationale: the per-turn cap deterministically kills the dangerous case (an agent flooding messages within one turn / a tight wake-reply-wake spin, throttled further by `NudgeCooldown` and one-nudge-per-idle-window). The residual risk — two agents trading one message per turn forever — is bounded in *rate* (gated by nudge interval + cooldown, so it's slow, visible on the dashboard, and human-interruptible) but not in *total*. We deliberately keep the data needed to address it later: per-turn counters in `running/{id}.json` and the full message history in `messages/`. **Follow-up (post-Phase 5):** a cross-turn detector that flags an A↔B pair exchanging >N messages over a rolling window and auto-pauses one side. Not in this phase.

2. **Mailbox cleanup / retention policy.**
   **Decision (adopted, §4.3):** Go janitor every 60s. Delete `read` messages older than 24h; delete any message older than 7 days (hard cap). Stopped/archived agents keep their mailbox until the 7-day cap (so a resumed agent sees recent mail) but are not nudged. Thresholds are Go constants (`MailReadTTL=24h`, `MailHardTTL=168h`); promoting them to `config.json` is optional.

3. **MCP registration mechanics per CLI (Claude Code vs Codex) — do both register the same server cleanly?**
   **Decision: yes, confirmed (§3.6).** Both CLIs register MCP servers via a `command`/`args`/`env` triple (Claude Code via `--mcp-config` JSON; Codex via its `mcp_servers` config). The Go `RegisterMessagingMCP` helper emits the backend-appropriate config but always points at the **same** `node mcp-messaging/dist/server.js` with the same env (`AGENTDECK_HOME`, `AGENTDECK_SELF_ID`). One server binary, two backends, identical behavior. Identity is taken from spawn env (`AGENTDECK_SELF_ID`), never from a spoofable tool argument, so cross-agent spoofing is impossible regardless of backend.

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
| MCP SDK | `@modelcontextprotocol/sdk@^1.29.0` |
| `message_id` format | `m_` + 6 hex |
| `turn_id` format | `t_` + counter |
