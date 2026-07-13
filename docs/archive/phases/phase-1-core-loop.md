# Phase 1 — Core loop: ACP chat runtime, launch, streaming chat

**Status:** ready to build after Phase 0
**Features:** F4 (launch), F3 (streaming chat — minimal)
**Depends on:** Phase 0
**Enables:** Phases 2–6 (everything needs a running agent producing a transcript)

---

## 1. Goal

Make one agent actually run. Launch a single agent via CLI and REST, wrap a real agent CLI (Claude Code first) through the **ACP chat runtime** over stdio, send it a prompt, and stream the response — assistant text, tool calls, tool results, diffs, and permission prompts — back to the caller. This is the vertical spine; the dashboard, persistence, and coordination phases all decorate this loop.

This phase proves the hardest integration risk (ACP wire format + permission gating) before any UI investment.

---

## 2. Scope

### In scope
- The `Runtime` interface and the **Chat runtime** implementation (ACP JSON-RPC / NDJSON over stdio).
- Config composition at launch: `project.cwd` + `project.context_prompt` + `role.system_prompt` + `backend/model` → CLI invocation.
- Launch flow (REST + CLI): insert identity + running rows and an initial status row in `state.db`; mint a per-launch hook token; register the in-process MCP server (stdio) with the agent.
- Send prompt, stream response, cancel turn, stop session.
- Inline permission request/response over the wire.
- One backend end to end (Claude Code / `claude-acp`).

### Out of scope
- Dashboard UI (Phase 2) — verify this phase via a thin test page or `curl`/SSE client.
- Hook ingest endpoint / multiplexed SSE event bus (Phase 2). This phase may stream over a single direct SSE/long-poll connection for the launching client; the multiplexed bus is Phase 2.
- Persistence/archive (Phase 4), messaging (Phase 5), terminal runtime & switching (Phase 6).
- Second backend (Codex) — design the interface for it but only Claude need work.

---

## 3. Runtime abstraction

Define the interface the server programs against (master PRD §4.1):

```
Runtime:
  Start(agent)                  // spawn CLI, register session, insert running + status rows
  SendPrompt(agent, text)       // submit a user turn
  Cancel(agent)                 // interrupt current turn (/cancel)
  Stop(agent)                   // terminate process group, remove running row
  Resume(agent, session_id)     // (stub this phase; full in Phase 4)
  CheckMessages(pid)            // (stub this phase; full in Phase 5)
```

A **runtime registry** dispatches by `agent.interface`. Only `interface: "chat"` is implemented here; `"terminal"` returns "not implemented" (Phase 6).

### 3.1 Chat runtime details
- Spawn the CLI as a child process group (so Cancel/Stop can signal the whole group).
- Speak ACP over stdio: send prompts, parse the streamed NDJSON for these event kinds → normalize into AgentDeck's internal transcript event types:
  - `assistant_text` (markdown delta)
  - `tool_call` (name + arguments)
  - `tool_result`
  - `diff` (file path + patch)
  - `permission_request` (tool + reason → expects approve/deny)
  - `turn_end` / `error`
- Map ACP permission requests to a pending state; the launching client approves/denies; relay the decision back over the ACP channel and unblock execution.
- On each lifecycle transition, update the agent's status in `state.db` (`state`, `detail`, `last_trace`, `busy_since`, `context_pct`). Chat agents derive status from the ACP stream directly; the `POST /api/hook` ingest path (Phase 2) carries lifecycle for terminal agents.

---

## 4. Launch flow (F4)

Composition order at launch: `project.cwd` (working dir) + `project.context_prompt` + `role.system_prompt` + selected `backend`/`model` → CLI invocation. Apply backend-level `env` then per-model `env` overrides.

**REST:** `POST /api/sessions {role, project, backend, model, interface}` →
1. Generate `agent_id`, insert the identity row in `state.db`.
2. Compose config, mint the per-launch hook token, register the in-process MCP server, `Runtime.Start`.
3. Insert the running row (pid, session_id, interface, started_at) and an initial status row (`state: "idle"`) in `state.db`.
4. Return the agent identity + status.

**CLI:** `agentdeck implementer@my-app`, `agentdeck reviewer@my-app --backend codex --model gpt-5.5 --interface chat`. The CLI form calls the same REST endpoint so both paths produce an identical agent. Name is auto-suggested when omitted.

---

## 5. REST/SSE surface added this phase

```
POST /api/sessions                 launch {role, project, backend, model, interface}
GET  /api/sessions/{id}            agent detail + status
POST /api/sessions/{id}/prompt     {text}
POST /api/sessions/{id}/cancel     interrupt current turn
POST /api/sessions/{id}/stop       stop session
POST /api/sessions/{id}/permission {tool_call_id, decision: "approve"|"deny"}
GET  /api/sessions/{id}/events     transcript stream for this agent (interim, single-agent SSE)
```

`/sessions/{id}/events` emits normalized transcript events (`new_message`-style). The fully multiplexed `/api/events` bus is Phase 2; keep the event payload shape forward-compatible.

---

## 6. Acceptance criteria

- [ ] `agentdeck implementer@my-app` and `POST /api/sessions` with the same params produce an identical running agent (matching identity in `state.db`, both startable/stoppable).
- [ ] Sending a prompt streams the response incrementally (token/delta level), not all-at-once.
- [ ] A tool call requiring permission surfaces a permission_request event and **gates execution** until approve/deny is sent; deny prevents the tool from running.
- [ ] Tool calls, tool results, and file diffs appear in the stream with their arguments/patches.
- [ ] Cancel interrupts an in-progress turn; Stop terminates the process group and removes the running row.
- [ ] The agent's status in `state.db` transitions idle → busy → idle across a turn, including `context_pct`.

---

## 7. Open questions (from master PRD §9)
- Exact ACP message schema for the target Claude Code version (tool-call, diff, permission-request shapes) — pin a version.
- Permission granularity: global skip vs per-role vs per-tool — Phase 1 honors `skip_permissions` (config/role) and per-call prompting; finer policy deferred.
- Concurrency: this phase runs one agent; no launch queueing yet.
