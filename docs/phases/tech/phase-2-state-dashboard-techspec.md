# Phase 2 — Implementation Tech Spec: State manager + SSE event bus + Dashboard card grid + Chat panel

**Mirrors:** `docs/phases/phase-2-state-dashboard.md`
**Features:** F1 (multi-agent dashboard, full), F3 (streaming chat panel, full)
**Depends on:** Phase 0 (file store, server skeleton), Phase 1 (Runtime, transcript events, hook reporting, per-agent SSE)
**Enables:** Phases 3–7 (the UI shell + multiplexed event bus they all plug into)
**Audience:** implementing engineer. This is prescriptive — pin every version, name every component, leave no design decisions open.

---

## 1. Overview & scope recap

### 1.1 What this phase delivers

Phase 1 produced a single agent that runs, streams a transcript, and gates permissions over an **interim per-agent SSE stream** (`GET /api/sessions/{id}/events`). Phase 2 turns that single-agent vertical into a **multi-agent supervisable dashboard**:

1. **State manager** (Go): a **SQLite-backed** store (`state.db`) of which the Go server is the sole writer. Status arrives over HTTP via **`POST /api/hook`** (token-authed); the manager applies each update to `state.db`, recomputes an *effective* `AgentState` (identity ⊕ running ⊕ status, joined from `state.db` rows), and pushes it into the event bus.
2. **SSE event bus** (Go): one multiplexed stream per browser client at `GET /api/events`, with a bounded per-client buffer (drop-oldest backpressure) and a ~10s keepalive ping. This is the dashboard's event channel. Transcript deltas flow as `new_message` events keyed by `agent_id`.
3. **React dashboard shell** (React + Vite + TS): app shell, global store, SSE client with reconnect, and routing between the card grid and the chat panel.
4. **Card grid (F1)**: a live card per running agent (name, role, project, backend/model, color-coded state badge, context-usage indicator, last-output-line preview), drag-reorder + density persisted to `layout.json`, right-click context menu.
5. **Chat panel (F3, full)**: assistant markdown, tool calls with args, tool results, file diffs, inline Approve/Deny gating, prompt send with streaming, cancel, context/model display.

### 1.2 In scope

- SQLite `state.db` store (server sole writer) + the `POST /api/hook` ingest endpoint + effective-state recompute (join identity+running+status rows).
- A reconciliation sweep over `sessions/` (fsnotify-backed) as a fallback for out-of-band transcript files.
- Multiplexed `GET /api/events` bus; removal of `GET /api/sessions/{id}/events`.
- `GET /api/layout` and `PUT /api/layout`.
- Full React app (store, SSE client, router, grid, chat panel).
- Drag-reorder + density (cards/row + gap) persistence.
- Right-click context menu (wiring per §1.3).

### 1.3 Card-menu actions: wired vs stubbed this phase

| Action | This phase | Backed by |
|---|---|---|
| **Open chat** | **Wired** | client-side route to `/agent/:id` |
| **Rename** | **Wired** | `POST /api/sessions/{id}/rename` (exists since Phase 1 surface) |
| **Stop** | **Wired** | `POST /api/sessions/{id}/stop` (Phase 1) |
| **Switch runtime** | **Stubbed** (visible, disabled, tooltip "Phase 6") | Phase 6 (F7) |
| **Clone** | **Stubbed** (visible, disabled, tooltip "Phase 3") | needs launch modal (Phase 3) |
| **Move to group** | **Stubbed** (visible, disabled, tooltip "Phase 6") | Phase 6 (F2 groups) |

Stubbed items render as disabled menu rows with a tooltip, **not** hidden — this keeps the menu layout stable across phases and signals the roadmap to the user.

### 1.4 Explicitly out of scope

Config-editing UI (Phase 3), launch modal (Phase 3 — Phase 2 ships only a minimal "New Agent" trigger that calls `POST /api/sessions` with defaults), archive/search/resume (Phase 4), message indicators (Phase 5), task-group collapsible sections (Phase 6), activity map (Phase 7).

---

## 2. Technology choices

### 2.1 Backend (Go)

| Concern | Choice | Version | Rationale |
|---|---|---|---|
| State store | SQLite via `github.com/mattn/go-sqlite3` (WAL mode) | `1.14.x` | `state.db` holds identity/running/status rows; the server is the sole writer, so there is no multi-process contention. The Phase 0 substrate already opens this driver; reuse the same handle. |
| HTTP ingest | **stdlib `net/http`** | Go 1.22+ | `POST /api/hook` is an ordinary JSON handler on the server that already serves the UI — no new transport. |
| SSE | **stdlib `net/http` only** — no SSE framework | Go 1.22+ | SSE is trivially a `text/event-stream` response with `http.Flusher`. A dependency adds nothing. Per-client goroutine + buffered channel. |
| JSON | stdlib `encoding/json` | — | Payloads are tiny; no need for a faster codec. |
| File watching (reconciliation only) | `github.com/fsnotify/fsnotify` | `v1.7.0` | Used solely to notice out-of-band transcript files under `sessions/` (§3.8). Cross-platform (kqueue on macOS, inotify on Linux), mature, no cgo. |

**SSE implementation approach.** Each `GET /api/events` request:
1. Sets `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `X-Accel-Buffering: no`.
2. Registers a `*client` (with a buffered Go channel) in the `Bus`.
3. Runs a loop: `select` over (the client channel → write+flush frame) and (a `time.Ticker` at 10s → write a `ping` frame) and (`r.Context().Done()` → deregister and return).
4. Uses `http.Flusher.Flush()` after every frame so bytes reach the browser immediately.

No `Server-Sent-Events` library; the entire bus is ~150 lines of stdlib Go.

### 2.2 Frontend

| Concern | Choice | Version | Rationale |
|---|---|---|---|
| Framework / build | React + Vite + TypeScript | React `18.3.x`, Vite `5.4.x`, TS `5.5.x` | Mandated by PRD §7. Vite dev server proxies `/api` to the Go server. |
| **State management** | **Zustand** | `4.5.x` | **Chosen.** See rationale below. |
| SSE client | **native `EventSource`** (browser built-in) | — | We only consume server→client events; `EventSource` gives auto-reconnect, named events, and `event:`/`data:` parsing for free. No `fetch`-stream lib needed because we never need custom headers on the stream (no auth — local only). |
| Routing | `react-router-dom` | `6.26.x` | Grid (`/`) ↔ chat panel (`/agent/:id`). Standard, well-known. |
| Drag-and-drop | `@dnd-kit/core` + `@dnd-kit/sortable` | core `6.1.x`, sortable `8.0.x` | Lightweight, accessible, pointer + keyboard, no legacy `react-dnd` HTML5-backend friction. `SortableContext` + `rectSortingStrategy` fits a card grid exactly. |
| Markdown | `react-markdown` + `remark-gfm` | react-markdown `9.0.x`, remark-gfm `4.0.x` | Renders assistant markdown (tables, code fences, lists). GFM for task lists/tables. |
| Syntax highlight (code fences) | `react-syntax-highlighter` (Prism build) | `15.5.x` | Plugged into react-markdown's `code` renderer. |
| **Diff rendering** | `react-diff-viewer-continued` | `3.4.x` | Maintained fork of `react-diff-viewer`; renders unified/split diffs from old/new strings or a parsed patch. Used for `diff` transcript events. |
| HTTP (commands) | native `fetch` | — | REST commands are simple JSON POSTs; no axios. |
| Sanitization | `rehype-sanitize` | `6.0.x` | Defense-in-depth on assistant markdown even though content is local-trusted. |

**Why Zustand over Redux Toolkit / Jotai / Context.**
- The store is a single flat map of agents plus a few UI slices; Redux Toolkit's reducer/action/slice ceremony is overhead for this shape.
- SSE pushes ~10–100 updates/sec across many fast agents. Zustand lets the SSE client call `useAgentStore.getState().applyStateUpdate(...)` **outside React's render cycle** and mutate via `set`, with components subscribing to **narrow selector slices** (`useAgentStore(s => s.agents[id])`) so a single agent's update re-renders only its card. This is the cheapest path to the "3+ cards updating independently" acceptance criterion.
- No `<Provider>` wrapper, transcript buffers can live in the store as plain `Map`s, and the SSE event handler is a non-React module that imports the store. Jotai's atom-per-agent model would mean dynamic atom creation/teardown per agent — more moving parts for no win here.

---

## 3. State manager design

Package: `internal/state` (Go). Type: `Manager`. Backing store: `state.db` (SQLite). The Go server is the **sole writer** to `state.db`; nothing else opens it for writing.

### 3.1 `state.db` schema

`state.db` lives under `~/.agentdeck/` (resolve `AGENTDECK_HOME` first). Three tables carry the live state the dashboard needs; the effective `AgentState` is a join across them.

```sql
-- identity: written at launch and on rename
CREATE TABLE IF NOT EXISTS agents (
    agent_id   TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL,
    project    TEXT NOT NULL,
    backend    TEXT NOT NULL,
    model      TEXT NOT NULL,
    interface  TEXT NOT NULL,
    grp        TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

-- running registry: a row exists only while the agent is live
CREATE TABLE IF NOT EXISTS running (
    agent_id   TEXT PRIMARY KEY REFERENCES agents(agent_id),
    pid        INTEGER NOT NULL,
    session_id TEXT NOT NULL,
    started_at TEXT NOT NULL
);

-- live status: last reported status per agent
CREATE TABLE IF NOT EXISTS status (
    agent_id    TEXT PRIMARY KEY REFERENCES agents(agent_id),
    state       TEXT NOT NULL,          -- busy|idle|waiting_input|done|error
    detail      TEXT NOT NULL DEFAULT '', -- last-output-line source (see §13)
    last_trace  TEXT NOT NULL DEFAULT '',
    busy_since  TEXT NOT NULL DEFAULT '',
    context_pct REAL NOT NULL DEFAULT 0, -- 0..1
    updated_at  INTEGER NOT NULL         -- unix ms
);
```

Open the DB with `PRAGMA journal_mode=WAL` and `PRAGMA busy_timeout=5000`. Because there is a single writer process, a single serialized writer connection (plus a read pool) is sufficient; wrap each applied update in a transaction.

### 3.2 Effective AgentState (the merge)

The dashboard renders **one** object per agent, computed by joining the `agents`, `running`, and `status` rows for that id. Producers (hooks, runtime) and the consumer (UI) stay decoupled through `state.db`.

```go
// internal/state/types.go
type AgentState struct {
    // identity (from agents row)
    AgentID   string `json:"agent_id"`
    Name      string `json:"name"`
    Role      string `json:"role"`
    Project   string `json:"project"`
    Backend   string `json:"backend"`
    Model     string `json:"model"`
    Interface string `json:"interface"`
    Group     string `json:"group,omitempty"`
    CreatedAt string `json:"created_at"`

    // running (from running row) — present only while live
    Running   bool   `json:"running"`
    PID       int    `json:"pid,omitempty"`
    SessionID string `json:"session_id,omitempty"`
    StartedAt string `json:"started_at,omitempty"`

    // status (from status row)
    State      string  `json:"state"`       // busy|idle|waiting_input|done|error; "unknown" if no status row
    Detail     string  `json:"detail"`      // last-output-line source (see §13)
    LastTrace  string  `json:"last_trace,omitempty"`
    BusySince  string  `json:"busy_since,omitempty"`
    ContextPct float64 `json:"context_pct"` // 0..1

    // derived
    UpdatedAt int64 `json:"updated_at"` // unix ms, set at recompute time (monotonic ordering aid)
}
```

**Recompute rule for `recompute(agentID)`** — a single `LEFT JOIN` query over `state.db`:
1. Select the `agents` row → identity. If missing, the agent does not exist → emit nothing (or a removal if it was previously known; see §3.5).
2. `LEFT JOIN running` → if a row exists set `Running=true` + pid/session/started; else `Running=false`.
3. `LEFT JOIN status` → state/detail/etc. If no status row → `State="unknown"`, `ContextPct=0`.
4. `UpdatedAt = time.Now().UnixMilli()`.
5. Build the `AgentState`, hand to the bus as a `state_update` event (§4).

Because the join runs inside the single-writer process against a transactionally consistent `state.db`, there are no partial-write races to defend against here — an update is applied in one transaction, then the recompute reads a committed, consistent snapshot.

### 3.3 `POST /api/hook` — the status ingest

This is the primary channel by which agents report status. Lifecycle hooks (a thin shell `curl`) POST to the server, which validates the token, applies the update to `state.db`, and emits a `state_update`. The chat runtime also writes status to `state.db` directly from the ACP stream (in-process, not over HTTP), through the same `Manager.Apply*` methods.

**Token.** Each launched agent's hooks receive a **one-time per-launch token** at launch time (issued by the launch path, stored alongside the `running` row). Every hook POST carries it. The server rejects any POST whose token does not match a live launch — this closes the gap that any local process could otherwise spoof status. The token is sent in the **`X-AgentDeck-Token` header** (a body field `token` is accepted as a fallback for hook environments that cannot set headers; if both are present the header wins).

**Request.**

```
POST /api/hook
Content-Type: application/json
X-AgentDeck-Token: <per-launch token>
```

```jsonc
{
  "agent_id": "a_8f3c12",        // required
  "event": "status",             // status | running | stopped   (lifecycle phase)
  "state": "busy",               // for event=status: busy|idle|waiting_input|done|error
  "detail": "Editing src/auth.ts",
  "last_trace": "PostToolUse: Edit",
  "context_pct": 0.42,           // 0..1, optional
  "pid": 48213,                  // for event=running
  "session_id": "claude-sess-xyz", // for event=running
  "ts": 1750579200123            // optional client unix ms; server stamps its own updated_at regardless
}
```

**Per-event effect on `state.db`:**

| `event` | Effect (single transaction) |
|---|---|
| `running` | Upsert the `running` row (`pid`, `session_id`, `started_at=now`). Mark the agent live. |
| `status` | Upsert the `status` row (`state`, `detail`, `last_trace`, `busy_since` set when transitioning into `busy`, `context_pct`, `updated_at=now`). |
| `stopped` | Delete the `running` row. Leaves identity + last status intact (card stays as `done`/`idle`, dimmed — see §3.5). |

After the transaction commits, the handler calls `recompute(agent_id)` and publishes the resulting `state_update` (§3.7).

**Responses.**

| Code | When | Body |
|---|---|---|
| `204 No Content` | Update applied and emitted. | empty |
| `400 Bad Request` | Malformed JSON, missing `agent_id`, unknown `event`, or out-of-range `state`/`context_pct`. | `{"error":"bad_request","message":"…"}` |
| `401 Unauthorized` | Missing or unparseable token. | `{"error":"unauthorized","message":"missing token"}` |
| `403 Forbidden` | Token does not match a live launch for this `agent_id`. | `{"error":"forbidden","message":"token mismatch"}` |
| `404 Not Found` | `agent_id` has no identity row in `state.db`. | `{"error":"not_found","message":"unknown agent"}` |
| `500 Internal Server Error` | `state.db` write failed. | `{"error":"internal","message":"…"}` |

Error envelope shape is fixed: `{ "error": <code-string>, "message": <human string> }`. Hooks are fire-and-forget shell `curl`s; they do not retry on `4xx` (a `4xx` means the request is wrong, not transient), and may retry once on `5xx`.

### 3.4 Apply path & emit

`Manager.ApplyHook(token, payload)` (called by the HTTP handler) and the runtime's in-process `Manager.ApplyStatus(...)` both funnel into one serialized writer:

```go
// pseudo
func (m *Manager) apply(agentID string, mutate func(tx *sql.Tx) error) error {
    m.writeMu.Lock()
    defer m.writeMu.Unlock()
    tx, _ := m.db.Begin()
    if err := mutate(tx); err != nil { tx.Rollback(); return err }
    if err := tx.Commit(); err != nil { return err }
    m.recomputeAndEmit(agentID) // read committed snapshot, publish state_update
    return nil
}
```

A single serialized writer keeps `state.db` simple and correct under the single-writer invariant. Recompute reads can use a separate read connection.

### 3.5 Removal semantics

The manager keeps a `known map[string]bool` of agent ids it has emitted. When a recompute finds **no `agents` row** (identity gone — only happens on a hard delete, not on stop) it emits a `state_update` with a tombstone form: `{"agent_id": id, "removed": true}` and drops the id from `known`. The frontend deletes the card on `removed:true`.

A **stop** (`event:"stopped"` → `running` row deleted, `status` may show `done`) is *not* a removal — the agent still has its identity row and stays as a card with `Running=false` until the user clears it or this phase's scope ends. (Archive/cleanup is Phase 4.) In practice Phase 1's stop emits a `stopped` hook (or the runtime applies it directly) and a final `status` of `done`/`idle`; the card stays visible showing `done`.

### 3.6 Startup full scan + rename

- **Startup:** `Manager.Start()` queries `SELECT agent_id FROM agents`, and for each id calls `recomputeAndEmit`. This seeds the bus's *current snapshot* (§4.4) so a client connecting at any time gets every already-known agent — including any still running from a prior server process, since `state.db` survived the restart. The scan must complete before the HTTP server starts accepting `/api/events` (or the snapshot must be marked ready) to avoid an empty first frame.
- **Rename / direct pokes:** REST handlers that write identity (`/rename`) update the `agents` row in the same serialized writer, then call `Manager.Touch(agentID)`, which schedules a `recomputeAndEmit`. Renames go live immediately.

### 3.7 How it feeds the bus

The manager holds a reference to the `Bus`. `recomputeAndEmit`:
1. runs the join query against `state.db` to build `AgentState`,
2. updates the bus's **snapshot map** (`bus.SetSnapshot(state)`) so future connections resync,
3. calls `bus.Publish(Event{Type: "state_update", AgentID: id, Data: state})`.

Transcript `new_message` events do **not** flow through the state manager — they come from the runtime (Phase 1) which publishes directly to the same bus (§4.3).

### 3.8 Reconciliation sweep (fallback)

`state.db` is the authoritative state store and `POST /api/hook` is how status reaches it. As a **fallback only**, a reconciliation sweep watches the agent CLI's own transcript directory:

- The agent CLI (Claude Code / Codex) writes its transcript files under `~/.agentdeck/sessions/`, outside our control. An fsnotify watch over `sessions/` (plus a periodic timer, e.g. every 30s) notices new or grown transcript files.
- When the sweep observes activity for a session whose `state.db` status looks stale (e.g. a `running` row with no recent `updated_at`, suggesting a missed hook), it derives a minimal status correction (at most: the agent is alive, and the last transcript line for the last-output-line fallback) and applies it through the same serialized writer, then recompute emits as usual.
- The sweep never *competes* with hooks: hook updates are immediate and authoritative; the sweep only fills gaps when a hook was missed. It watches `sessions/` exclusively — it does not watch any state directory, because state lives in `state.db`, not on loose files.

This keeps the dashboard correct even if a hook is dropped, without making file-watching the status channel.

---

## 4. SSE event bus design

Package: `internal/bus`. Types: `Bus`, `client`, `Event`.

### 4.1 Event envelope (wire schema)

Every SSE frame is one named event whose `data:` is a JSON envelope:

```jsonc
{
  "type": "state_update",          // state_update | new_message | notification | ping
  "seq": 10428,                    // server-global monotonic sequence (uint64), for client ordering/debug
  "ts": 1750579200123,             // unix ms server time
  "agent_id": "a_8f3c12",          // present for state_update & new_message; null for ping; optional for notification
  "data": { /* type-specific payload, see §8 */ }
}
```

On the wire (note the SSE `event:` line mirrors `type` so the client can use `addEventListener`):

```
event: state_update
id: 10428
data: {"type":"state_update","seq":10428,"ts":1750579200123,"agent_id":"a_8f3c12","data":{...AgentState...}}

```

(Blank line terminates the frame. `id:` carries `seq` so the browser sends `Last-Event-ID` on reconnect — we accept it but, since `state.db` is the source of truth, reconnect always triggers a **full resync** rather than replay; see §4.5.)

### 4.2 Per-client bounded buffer + drop-oldest

```go
type client struct {
    ch     chan Event   // buffered, capacity = bufSize
    id     string       // uuid per connection
}

const bufSize = 256   // see §9 / §13 for tuning rationale
```

`Bus.Publish(ev)`:
```go
b.mu.RLock()
for _, c := range b.clients {
    select {
    case c.ch <- ev:           // fast path
    default:                   // buffer full → DROP OLDEST, then enqueue
        select { case <-c.ch: default: }   // pop one
        select { case c.ch <- ev: default: } // push new (best-effort)
        c.dropped++             // metric
    }
}
b.mu.RUnlock()
```

Drop-oldest (not drop-newest) because the **newest** state is the truest — a slow client that fell behind should converge to current reality, not replay stale frames. For `state_update` this is self-correcting (each carries the full `AgentState`). For `new_message` deltas, dropping is lossier (a transcript gap) — mitigated by the client requesting a transcript refetch when it detects a `seq` gap on a `new_message` for an agent whose chat panel is open (§7.4 / §9).

`bufSize = 256` envelopes per client (see §13).

### 4.3 How transcript deltas flow (replacing Phase 1's per-agent stream)

Phase 1 streamed transcript events on `GET /api/sessions/{id}/events` (single agent, one connection per agent). **Phase 2 removes that endpoint.** The chat runtime, which already normalizes ACP into transcript events (`assistant_text`, `tool_call`, `tool_result`, `diff`, `permission_request`, `turn_end`, `error`), now calls `bus.Publish(Event{Type:"new_message", AgentID: id, Data: <transcript event>})` instead.

Every connected client receives every `new_message` (keyed by `agent_id`). The frontend filters: the open chat panel for agent X consumes `new_message` where `agent_id == X`; cards may peek at the latest `assistant_text` only if `status.detail` is empty (last-output-line fallback, §13). This is acceptable because traffic is local and per-message payloads are small; if it ever matters, a future phase can add server-side subscription filtering — **not** this phase.

`permission_request` and `turn_end`/`error` also ride `new_message` (they are transcript events). `notification` events (Phase 5/F11) are a separate type for desktop/in-app alerts and are **not** emitted this phase except the bus type must exist.

### 4.4 Snapshot + connect semantics

The bus owns `snapshot map[string]AgentState` (written by the state manager, §3.7). On a new `/api/events` connection, **before** entering the live loop, the server writes one `state_update` frame per agent in the snapshot (a "hydration burst"), then streams live. This guarantees: connect at any time → see all current agents immediately, then live deltas. No separate "GET all agents then subscribe" race.

(There is also `GET /api/sessions` from Phase 1 for a plain REST list; the frontend uses the SSE hydration burst as the primary path and may use `GET /api/sessions` only as a fallback if SSE is unavailable.)

### 4.5 Reconnect + full resync

- The browser `EventSource` auto-reconnects on drop (default ~3s; we set the server `retry:` hint to `2000`).
- On reconnect the server treats it as a brand-new connection: **replays the full snapshot** (hydration burst) then live. We do **not** replay missed `new_message` deltas from a ring — instead, the client, on the `open` event of a reconnect, **refetches the transcript** for any chat panel currently open (`GET /api/sessions/{id}/transcript` — see note) and clears stale agents not present in the new hydration burst.
- The snapshot is rebuilt from `state.db` (the source of truth), so a client reconnecting after any downtime resyncs to current committed state. "No stale cards on reconnect" (acceptance) is satisfied because the hydration burst is the authoritative current set; the client replaces its agent map with it (any agent id present in the store but absent from a *completed* hydration burst is removed). To know when the burst is complete, the bus sends a synthetic `state_update` with `agent_id: "__hydrated__"` and `data: {"hydrated": true}` as the final hydration frame; the client uses it as the "snapshot complete" marker.

> **Transcript refetch endpoint.** Phase 1 streamed transcript but the persisted history endpoint is Phase 4. For Phase 2, add a minimal read-only `GET /api/sessions/{id}/transcript` that returns the **in-memory** transcript buffer the runtime already holds for the live turn(s) this session (bounded to the current process's retained events). This is enough to repaint an open chat panel after reconnect without waiting on Phase 4 persistence. If the runtime retains nothing, it returns an empty array and the panel shows only live deltas going forward.

### 4.6 Keepalive

A 10s `time.Ticker` per connection writes a `ping` frame (`event: ping\ndata: {"type":"ping","seq":<n>,"ts":<ms>,"agent_id":null,"data":{}}\n\n`). Keeps intermediaries and the browser from idling the connection and gives the client a liveness signal (if no ping for ~25s, the client force-reconnects, §5.4).

---

## 5. Frontend architecture

### 5.1 Folder structure

```
ui/                                  # Vite root
  index.html
  vite.config.ts                     # proxy /api → http://127.0.0.1:4317
  package.json
  tsconfig.json
  src/
    main.tsx                         # ReactDOM root + RouterProvider
    App.tsx                          # layout shell (header + <Outlet/>)
    routes.tsx                       # react-router route table
    api/
      client.ts                      # fetch wrappers for REST commands
      sse.ts                         # EventSource manager (connect, reconnect, dispatch)
      types.ts                       # AgentState, Event envelope, transcript event types (mirror Go)
    store/
      agentStore.ts                  # Zustand: agents map + applyStateUpdate / hydrate / removeAgent
      transcriptStore.ts            # Zustand: per-agent transcript event arrays + appendMessage
      uiStore.ts                     # Zustand: density, layout order, connection status, context menu
    components/
      shell/
        Header.tsx                   # title, connection indicator, "New Agent" trigger
        ConnectionDot.tsx            # green/amber/red from uiStore.connection
      grid/
        CardGrid.tsx                 # dnd-kit SortableContext, density CSS grid
        AgentCard.tsx                # one card (badge, context bar, last-output line)
        StateBadge.tsx               # color mapping
        ContextBar.tsx               # context_pct meter
        CardContextMenu.tsx          # right-click menu (wired + stubbed items)
        DensityControl.tsx           # cards/row + gap controls
        EmptyState.tsx               # no agents → minimal New Agent trigger
      chat/
        ChatPanel.tsx                # route /agent/:id; header + transcript + composer
        TranscriptView.tsx           # maps transcript events → renderers, autoscroll
        renderers/
          AssistantText.tsx          # react-markdown + gfm + sanitize + code highlight
          ToolCall.tsx               # tool name + collapsible args
          ToolResult.tsx             # result (collapsible)
          DiffBlock.tsx              # react-diff-viewer-continued
          PermissionPrompt.tsx       # Approve / Deny gating control
          TurnError.tsx
        Composer.tsx                 # textarea + send + cancel
        ChatHeader.tsx               # name, model, context bar, back button
    lib/
      classNames.ts
      time.ts
    styles/
      tokens.css                     # color tokens incl. state-badge palette
      global.css
```

### 5.2 Global store shape

```ts
// store/agentStore.ts
interface AgentStoreState {
  agents: Record<string, AgentState>;     // keyed by agent_id
  order: string[];                         // display order (mirrors layout.json; falls back to created_at)
  hydrating: boolean;                      // true while a hydration burst is in progress
  applyStateUpdate(s: AgentState): void;   // upsert one agent; if not in order, append
  hydrateBegin(): void;                    // mark start; collect ids
  hydrateComplete(seenIds: string[]): void;// remove agents not in seenIds (no stale cards)
  removeAgent(id: string): void;           // tombstone (removed:true)
  setOrder(order: string[]): void;         // from drag-reorder / layout load
}

// store/transcriptStore.ts
interface TranscriptStoreState {
  byAgent: Record<string, TranscriptEvent[]>;     // append-only per agent
  pending: Record<string, PendingPermission | null>; // open permission gate per agent
  appendMessage(agentId: string, ev: TranscriptEvent): void;
  setTranscript(agentId: string, evs: TranscriptEvent[]): void; // refetch on reconnect
  resolvePermission(agentId: string, toolCallId: string): void;
}

// store/uiStore.ts
interface UiStoreState {
  density: { perRow: number; gap: number };        // mirrors layout.json
  connection: 'connecting' | 'open' | 'reconnecting' | 'down';
  contextMenu: { agentId: string; x: number; y: number } | null;
  setDensity(d): void;
  setConnection(c): void;
  openContextMenu(agentId, x, y): void;
  closeContextMenu(): void;
}
```

`order` + `density` are the persisted `layout.json` shape (§8.3). They load once at boot (`GET /api/layout`) and save on change (`PUT /api/layout`, debounced 400ms).

### 5.3 SSE client + dispatch (`api/sse.ts`)

A singleton, created once in `main.tsx`, **not** inside a component (so it survives route changes and re-renders):

```ts
class SseClient {
  private es: EventSource | null = null;
  private lastPing = Date.now();
  private hydrationIds: string[] = [];

  connect() {
    setConnection('connecting');
    this.es = new EventSource('/api/events');     // same-origin via Vite proxy / served by Go
    this.es.onopen = () => { setConnection('open'); this.onReconnect(); };
    this.es.addEventListener('state_update', e => this.onStateUpdate(JSON.parse(e.data)));
    this.es.addEventListener('new_message',  e => this.onNewMessage(JSON.parse(e.data)));
    this.es.addEventListener('notification', e => this.onNotification(JSON.parse(e.data)));
    this.es.addEventListener('ping',         () => { this.lastPing = Date.now(); });
    this.es.onerror = () => { setConnection('reconnecting'); };  // EventSource retries itself
    this.startWatchdog();
  }
  // watchdog: if Date.now()-lastPing > 25000 → es.close(); connect();
}
```

Dispatch:
- `state_update` with `data.hydrated === true` → mark hydration complete (`hydrateComplete(hydrationIds)`), reset `hydrationIds`.
- `state_update` with `data.removed === true` → `removeAgent(agent_id)`.
- other `state_update` → `applyStateUpdate(data)`; if hydrating, push `agent_id` to `hydrationIds`.
- `new_message` → `transcriptStore.appendMessage(agent_id, data)`; if it is a `permission_request`, set `pending[agent_id]`; also feed last-output-line fallback (§13). Detect `seq` gap → if a chat panel for that agent is open, trigger transcript refetch.
- `ping` → bump `lastPing`.

The SSE client lives **outside React**; it calls Zustand `set` functions directly via `getState()`. Components subscribe to slices and re-render only on their slice's change.

### 5.4 Reconnect on client

- `EventSource` auto-reconnects; we surface state via `connection`.
- On every `onopen`, run `onReconnect()`: set `hydrating=true`, expect a fresh hydration burst, and refetch transcript for any open chat panel.
- A **watchdog** (25s without ping or message) force-closes and reconnects, covering half-open TCP that `EventSource` won't notice.

### 5.5 Routing

`react-router-dom` with `createBrowserRouter`:

```
/                 → CardGrid (inside App shell)
/agent/:id        → ChatPanel (inside App shell)
*                 → redirect to /
```

The shell (`App.tsx`) renders `Header` + `<Outlet/>`. Opening a card navigates to `/agent/:id`; the chat panel reads `:id`, subscribes to that agent's slice + transcript, and renders. Back button (and browser back) returns to the grid. Both views share the same live store, so the grid keeps updating while a chat panel is open (state is global, not route-scoped).

---

## 6. Card grid (F1)

### 6.1 `AgentCard` fields

Rendered from `agents[id]`:

- **Name** (`name`) — bold, editable inline on Rename.
- **Role · Project** (`role`, `project`) — subtitle; project accent color from `projects/{p}.color` (fetched lazily / cached; if unavailable use neutral).
- **Backend / model** (`backend` / `model`) — small mono pill, e.g. `claude · sonnet-4-6`.
- **State badge** (`state`) — color-coded (`StateBadge`, §6.2).
- **Context-usage indicator** (`context_pct`) — `ContextBar` (§6.3).
- **Last-output-line preview** — single truncated line (§13).
- **Running indicator** — if `running === false`, dim the card and show a small "stopped" marker.

Clicking the card body → `navigate('/agent/'+id)`. Right-click → context menu (§6.5).

### 6.2 Badge color mapping (`StateBadge` + `tokens.css`)

| state | label | token | color (light) |
|---|---|---|---|
| `busy` | Busy | `--badge-busy` | amber `#D97706` (pulsing dot) |
| `idle` | Idle | `--badge-idle` | slate `#64748B` |
| `waiting_input` | Waiting | `--badge-waiting` | blue `#2563EB` (attention) |
| `done` | Done | `--badge-done` | green `#16A34A` |
| `error` | Error | `--badge-error` | red `#DC2626` |
| `unknown` | — | `--badge-unknown` | gray `#9CA3AF` (no status row yet) |

`busy` gets an animated pulsing dot; `waiting_input` and `error` get a subtle highlighted card border to draw the eye (these are the actionable states). Colors are CSS variables so Phase 3 theming can override.

### 6.3 Context-usage indicator (`ContextBar`)

A thin horizontal meter, width = `context_pct * 100%`, with a numeric `Math.round(pct*100)%` label. Color ramps: green `< 0.6`, amber `0.6–0.85`, red `> 0.85` (approaching context limit). `context_pct` of `0`/missing renders an empty bar with no label.

### 6.4 Last-output-line source

Per §13: **`status.detail`** is the primary source (cheap, already on every card's `AgentState`). Fallback: the latest `assistant_text` delta seen for that agent (maintained as `lastLine[id]` in the SSE client from `new_message`). If both empty → render nothing (no placeholder text). Truncate to one line with ellipsis.

### 6.5 Drag-reorder + density (persisted)

- **Drag-reorder:** `CardGrid` wraps cards in `@dnd-kit` `<DndContext>` + `<SortableContext items={order} strategy={rectSortingStrategy}>`. Each `AgentCard` is a `useSortable` item. On `onDragEnd`, compute the new `order` via `arrayMove`, call `setOrder`, and `PUT /api/layout` (debounced 400ms).
- **Density:** `DensityControl` adjusts `perRow` (e.g. 2–6) and `gap` (e.g. 8–24px). The grid is `display:grid; grid-template-columns: repeat(perRow, 1fr); gap: gap`. Changes call `setDensity` + persist.
- **Persistence:** `order` + `density` are saved to `layout.json` via `PUT /api/layout` and loaded at boot via `GET /api/layout`. New agents not yet in `order` append at the end (and a save captures them). Acceptance: reload preserves order + density.

### 6.6 Right-click context menu (`CardContextMenu`)

Opens at cursor on `contextmenu` event; `uiStore.openContextMenu(id,x,y)`. Items:

| Item | Enabled | Handler |
|---|---|---|
| Open chat | yes | `navigate('/agent/'+id)` |
| Rename | yes | inline edit → `POST /api/sessions/{id}/rename {name}` |
| Stop | yes (if running) | `POST /api/sessions/{id}/stop` (confirm dialog) |
| — divider — | | |
| Switch runtime | **disabled** | tooltip "Available in Phase 6" |
| Clone | **disabled** | tooltip "Available in Phase 3" |
| Move to group | **disabled** | tooltip "Available in Phase 6" |

Click-outside or Escape closes. Menu is a portal so it isn't clipped by card overflow.

### 6.7 Empty state

When `agents` is empty, `EmptyState` shows a message + a **minimal "New Agent" trigger** (full modal is Phase 3). The trigger posts `POST /api/sessions` with `config.json` defaults (`default_role`, `default_project`, default backend/model, `interface:"chat"`). On success the SSE `state_update` adds the card automatically.

---

## 7. Chat panel (full F3)

Route `/agent/:id` → `ChatPanel`. Consumes `transcriptStore.byAgent[id]` (live `new_message` deltas) + `agentStore.agents[id]` (header/context/model).

### 7.1 Transcript rendering (`TranscriptView` + renderers)

`TranscriptView` maps each `TranscriptEvent` to a renderer by `kind`:

| transcript `kind` | renderer | behavior |
|---|---|---|
| `assistant_text` | `AssistantText` | react-markdown + remark-gfm + rehype-sanitize; code fences via react-syntax-highlighter. **Deltas with the same `message_id` concatenate** into one growing bubble (streaming). |
| `tool_call` | `ToolCall` | shows tool `name`; args rendered as collapsible pretty-printed JSON. |
| `tool_result` | `ToolResult` | collapsible; truncates very long output with "show more". |
| `diff` | `DiffBlock` | `react-diff-viewer-continued` from `old`/`new` (or parsed patch); file path header; split/unified toggle. |
| `permission_request` | `PermissionPrompt` | Approve / Deny buttons; see §7.3. |
| `turn_end` | — | marks turn boundary (subtle separator); re-enables composer. |
| `error` | `TurnError` | red inline error block. |

**Autoscroll:** stick to bottom while at bottom; if the user scrolls up, stop autoscrolling and show a "jump to latest" affordance. Streaming `assistant_text` deltas append to the in-progress bubble (keyed by `message_id`) so tokens appear incrementally (acceptance: "streams output incrementally").

### 7.2 Streaming send + cancel (`Composer`)

- **Send:** textarea + Send (Enter to send, Shift+Enter newline). On send: `POST /api/sessions/{id}/prompt {text}`, optimistically append a user bubble, disable Send while the turn is busy. The response streams back as `new_message` events over the shared SSE bus (no per-request stream). Composer re-enables on `turn_end`.
- **Cancel:** while busy, Send becomes/exposes a Cancel button → `POST /api/sessions/{id}/cancel`. The runtime interrupts the turn and emits `turn_end`/`error`.

### 7.3 Inline Approve / Deny gating (`PermissionPrompt`)

When a `permission_request` transcript event arrives (`{kind:"permission_request", tool_call_id, tool, reason, args}`):
1. `transcriptStore.pending[agentId]` is set; the request renders inline with the tool name, reason, and args, plus **Approve** / **Deny**.
2. Execution is gated server-side (Phase 1 holds the tool until a decision arrives) — the UI simply must collect the decision.
3. Click → `POST /api/sessions/{id}/permission {tool_call_id, decision:"approve"|"deny"}`; clear `pending`; the prompt collapses to a resolved chip ("Approved"/"Denied").
Acceptance: a tool needing permission surfaces an Approve/Deny that gates execution; Deny prevents the tool from running.

### 7.4 Context / model display + reconnect repaint (`ChatHeader`)

- Shows `name`, `backend · model`, and the same `ContextBar` (`context_pct`) as the card, live-updating from `state_update`.
- Back button → `/`.
- **Reconnect:** when SSE reconnects while this panel is open, `sse.ts` calls `GET /api/sessions/{id}/transcript` and `transcriptStore.setTranscript(id, evs)` to repaint, preventing a gap from any dropped `new_message` deltas (§4.5, §9).

---

## 8. API contracts

All under `http://127.0.0.1:4317/api` (port from `config.json`, default placeholder `4317`).

### 8.1 `POST /api/hook` (status ingest) — added

The primary status channel (§3.3). Token-authed (`X-AgentDeck-Token` header, body `token` fallback). Applies the update to `state.db`, then recompute emits a `state_update`.

**Request:**
```jsonc
{ "agent_id":"a_8f3c12", "event":"status",
  "state":"busy", "detail":"Editing src/auth.ts", "last_trace":"PostToolUse: Edit",
  "context_pct":0.42 }
// event=running:  { "agent_id":"...", "event":"running", "pid":48213, "session_id":"claude-sess-xyz" }
// event=stopped:  { "agent_id":"...", "event":"stopped" }
```

**Responses:** `204` applied · `400` bad request · `401` missing token · `403` token mismatch · `404` unknown agent · `500` write failed. Error body: `{ "error": <code>, "message": <string> }` (§3.3).

### 8.2 `GET /api/events` (multiplexed SSE) — added; replaces Phase 1 per-agent stream

**Response headers:** `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `X-Accel-Buffering: no`.

**Frame format:** `event: <type>\nid: <seq>\ndata: <envelope JSON>\n\n`. Envelope:

```jsonc
{ "type": string, "seq": number, "ts": number, "agent_id": string|null, "data": object }
```

**Connect sequence:**
1. `retry: 2000` line.
2. Hydration burst: one `state_update` per current agent (snapshot, built from `state.db`).
3. Final hydration marker: `state_update` with `agent_id:"__hydrated__"`, `data:{ "hydrated": true }`.
4. Live frames + `ping` every 10s.

**Payloads per `type`:**

`state_update` — `data` is `AgentState` (§3.2), or a control form:
```jsonc
// normal
{ "type":"state_update","seq":N,"ts":T,"agent_id":"a_8f3c12",
  "data":{ "agent_id":"a_8f3c12","name":"Atlas","role":"implementer","project":"my-app",
           "backend":"claude","model":"sonnet-4-6","interface":"chat","group":"",
           "created_at":"...","running":true,"pid":48213,"session_id":"claude-sess-xyz",
           "started_at":"...","state":"busy","detail":"Editing src/auth.ts",
           "last_trace":"PostToolUse: Edit","busy_since":"...","context_pct":0.42,"updated_at":1750579200123 } }
// removal tombstone
{ ..., "agent_id":"a_8f3c12", "data":{ "agent_id":"a_8f3c12", "removed": true } }
// hydration complete marker
{ ..., "agent_id":"__hydrated__", "data":{ "hydrated": true } }
```

`new_message` — `data` is one normalized transcript event (shapes defined Phase 1):
```jsonc
{ "type":"new_message","seq":N,"ts":T,"agent_id":"a_8f3c12",
  "data":{ "kind":"assistant_text", "message_id":"m_12", "text":"…delta…" } }
// kinds: assistant_text | tool_call | tool_result | diff | permission_request | turn_end | error
// tool_call:        { "kind":"tool_call","tool_call_id":"tc_3","name":"Edit","args":{...} }
// tool_result:      { "kind":"tool_result","tool_call_id":"tc_3","result":"…","is_error":false }
// diff:             { "kind":"diff","path":"src/auth.ts","old":"…","new":"…" }   // or {"patch":"…unified…"}
// permission_request:{ "kind":"permission_request","tool_call_id":"tc_4","tool":"Bash","reason":"run tests","args":{...} }
// turn_end:         { "kind":"turn_end","message_id":"m_12" }
// error:            { "kind":"error","message":"…" }
```

`notification` — **type reserved this phase** (emitted in Phase 5/F11). Shape pinned now:
```jsonc
{ "type":"notification","seq":N,"ts":T,"agent_id":"a_8f3c12",
  "data":{ "level":"info|warn|action", "kind":"done|waiting_input|permission_required","title":"…","body":"…" } }
```

`ping`:
```jsonc
{ "type":"ping","seq":N,"ts":T,"agent_id":null,"data":{} }
```

### 8.3 `GET /api/layout` — added

```jsonc
// 200 OK ; if layout.json missing, return defaults (do not 404)
{ "order": ["a_8f3c12","a_99aa01"], "density": { "perRow": 4, "gap": 16 } }
```

### 8.4 `PUT /api/layout` — added

Request body = same shape. Server validates (`perRow` 1–8, `gap` 0–48; `order` is string[]) and writes `~/.agentdeck/layout.json` atomically (temp+rename). Returns `200` with the stored object. Unknown agent ids in `order` are kept (harmless; they're filtered against live agents on the client).

### 8.5 `GET /api/sessions/{id}/transcript` — added (minimal, for reconnect repaint)

```jsonc
// 200 OK : the in-memory transcript events the runtime currently retains for this live session
{ "agent_id":"a_8f3c12", "events":[ { "kind":"assistant_text", ... }, ... ] }
// if nothing retained → { "agent_id":"...", "events":[] }
// 404 if no such agent
```

### 8.6 Removed

`GET /api/sessions/{id}/events` (Phase 1 interim per-agent stream) — **deleted**. All transcript flow moves to `new_message` on `/api/events`.

### 8.7 Reused unchanged from Phase 1

`POST /api/sessions`, `GET /api/sessions`, `GET /api/sessions/{id}`, `POST /api/sessions/{id}/prompt`, `/cancel`, `/stop`, `/rename`, `/permission`.

---

## 9. Concurrency, edge cases & error handling

**Backpressure tuning.** Per-client buffer = **256 envelopes**. Reasoning: a fast turn emits assistant-text deltas at perhaps 20–80/s; with, say, 10 concurrent busy agents that's up to ~800 envelopes/s peak. A correctly-behaving browser drains far faster than that; 256 absorbs a multi-hundred-ms render stall without dropping. If the buffer does fill (a tab throttled in the background), drop-oldest keeps the connection alive and `state_update`s self-heal (full state each time). `dropped` counter per client is exposed for debugging.

**Many fast agents.** Hook POSTs are applied to `state.db` one transaction at a time through the single serialized writer; the recompute after each commit emits at most one `state_update` per applied change. `new_message` is not throttled (deltas must stream), but each is small. The bus's `Publish` holds only an `RLock` and does non-blocking sends, so one slow client never blocks publishing to others or the manager. If hooks ever fire faster than is useful, the handler can coalesce same-`agent_id` status updates within a short window before committing — not needed this phase.

**`POST /api/hook` validation.** Bad/missing token → `401`/`403`; unknown agent → `404`; malformed body or out-of-range fields → `400` (§3.3). The write is one transaction; a failed write returns `500` and emits nothing (the prior committed state stands), so the dashboard never shows a half-applied update.

**Missed hook / status drift.** If a hook POST is lost, `state.db` simply doesn't change for that update. The reconciliation sweep over `sessions/` (§3.8) is the safety net: it notices transcript activity for an agent whose status looks stale and applies a minimal correction, which recompute then emits. This is a fallback — the steady-state channel is `POST /api/hook`.

**Dropped SSE connection.** `EventSource` auto-reconnects; the watchdog (25s no-ping) force-reconnects half-open sockets. On reconnect: full hydration burst (rebuilt from `state.db`) → client replaces its agent set (`hydrateComplete` removes stale) → open chat panel refetches transcript. No manual refresh needed (acceptance).

**Stale cards on reconnect.** Solved by hydration-complete marker (`__hydrated__`): any agent in the store but absent from the just-completed burst is removed. Because the burst comes from `state.db`, a stopped-then-restarted agent reappears with a fresh `AgentState`.

**`new_message` gap after a drop.** Client tracks last `seq` per type; if a `new_message` arrives with a `seq` gap **and** that agent's chat panel is open, it refetches `GET /api/sessions/{id}/transcript`. If the panel is closed, the gap is ignored (transcript will be correct next time it's opened or on next reconnect). Cards are unaffected (they rely on `state_update`).

**Reconciliation watcher death** (rare: too many open files, `sessions/` removed). The sweep logs an error, attempts to re-add the watch with backoff, and on success does a fresh pass. State correctness does not depend on it — `state.db` + hooks remain authoritative.

**Slow consumer detection.** If a client's `dropped` exceeds a threshold (e.g. 1000) the server may close that connection to force a clean reconnect/resync; the client reconnects and rehydrates.

**Layout write race.** `PUT /api/layout` writes atomically (temp+rename); concurrent PUTs are last-writer-wins (single user, acceptable). The 400ms client debounce minimizes thrash from drag/density changes.

---

## 10. Implementation task breakdown (ordered)

**Backend — store + ingest, bus, then wiring:**
1. `internal/state`: open/migrate `state.db` (`agents`/`running`/`status` tables, WAL); serialized writer; `recompute` (join identity+running+status); `known` map + removal tombstones; startup scan; `Touch`.
2. `internal/bus`: `Event`, `Bus`, `client`; `Subscribe/Unsubscribe`, `Publish` (drop-oldest), `SetSnapshot`, global `seq` counter. Unit-testable with no HTTP.
3. `internal/api`: `POST /api/hook` — token validation, body validation, apply to `state.db`, recompute+publish; the response-code matrix in §3.3.
4. `internal/api`: `GET /api/events` handler — headers, hydration burst + `__hydrated__` marker, live loop, 10s ping ticker, context-cancel cleanup.
5. Re-point the chat runtime's transcript emission from the Phase 1 per-agent stream to `bus.Publish("new_message", …)`; have the runtime apply status to `state.db` in-process; **delete** `GET /api/sessions/{id}/events`.
6. Reconciliation sweep: fsnotify + periodic timer over `sessions/`; apply minimal corrections for missed hooks (fallback only).
7. `GET/PUT /api/layout` (read defaults if missing; validate; atomic write).
8. `GET /api/sessions/{id}/transcript` (in-memory retained events).

**Frontend — store, then SSE, then grid, then chat:**
9. Vite + TS scaffold; `vite.config.ts` proxy to `:4317`; tokens/global CSS; router skeleton (`/`, `/agent/:id`).
10. `api/types.ts` (mirror Go); Zustand stores (`agentStore`, `transcriptStore`, `uiStore`).
11. `api/sse.ts` singleton: connect, named-event listeners, hydration handling, watchdog, dispatch to stores; `api/client.ts` REST wrappers.
12. Grid: `CardGrid` (CSS grid + density), `AgentCard`, `StateBadge`, `ContextBar`, last-output-line; `EmptyState` with minimal New Agent trigger.
13. Drag-reorder (`@dnd-kit`) + `DensityControl`; load/save `layout.json` (debounced).
14. `CardContextMenu` (wired: Open/Rename/Stop; stubbed: Switch/Clone/Move, disabled + tooltip).
15. Chat: `ChatPanel`, `ChatHeader` (context/model), `TranscriptView` + renderers (`AssistantText`, `ToolCall`, `ToolResult`, `DiffBlock`, `PermissionPrompt`, `TurnError`), autoscroll.
16. `Composer` (send/cancel streaming) + Approve/Deny wiring + reconnect transcript refetch.
17. Polish: connection indicator, error toasts, empty/loading states.

---

## 11. Testing strategy

**Backend (Go, `testing`):**
- *State store unit:* point at a temp `AGENTDECK_HOME`; open `state.db`; apply an identity insert + `running` + `status` and assert the recomputed `AgentState` reflects the join; delete the `running` row (`event:"stopped"`) → `running:false`; delete the `agents` row → `removed` tombstone; startup scan emits one event per existing agent.
- *Hook endpoint (httptest):* valid token + `event:"status"` → `204` and a `state_update` is published with the merged state; missing token → `401`; wrong token → `403`; unknown `agent_id` → `404`; malformed body / bad `context_pct` → `400`; forced `state.db` write error → `500` with the error envelope and nothing published.
- *Bus unit:* publish to N subscribers; assert all receive in order. Fill one client's buffer past 256 and assert drop-oldest (newest retained, oldest gone, others unaffected, connection alive).
- *Bus concurrency:* `go test -race` with concurrent publishers + subscribe/unsubscribe churn; no deadlock, no data race.
- *SSE handler (httptest):* connect, assert hydration burst + `__hydrated__` marker, then a published event arrives as a well-formed frame; ping appears (use a shortened ticker in test); client disconnect deregisters.
- *Reconciliation sweep:* with a stale `running` row and no recent hook, drop a grown transcript file under `sessions/` and assert the sweep applies a correction and a `state_update` is emitted; assert it does not override a fresh hook update.
- *Layout:* GET defaults when missing; PUT validates bounds and round-trips; atomic write leaves no temp file.

**Frontend (Vitest + React Testing Library):**
- *Store:* `applyStateUpdate` upserts + orders; `hydrateComplete` removes stale; `removeAgent` tombstone; `appendMessage` concatenates same-`message_id` assistant text.
- *Components:* `StateBadge` color per state; `ContextBar` ramp + label; `AgentCard` renders fields + last-output-line (detail vs fallback); `CardContextMenu` wired vs disabled items; `PermissionPrompt` calls the permission endpoint with correct decision; `DiffBlock` renders old/new.
- *SSE integration (mock EventSource):* feed a hydration burst + deltas → grid renders N cards; a `state_update` flips a single card's badge without re-rendering others (selector isolation); reconnect (re-fire `onopen` + new burst) removes a stale agent and refetches transcript.

**Multi-agent live-update test (integration / manual+scripted):**
- Launch 3 agents; drive status via `POST /api/hook` (or real turns) so they cycle idle→busy→done; assert all three cards update independently from the single `/api/events` connection within ~1s, and the chat panel for one streams while the others keep updating in the grid. Reload preserves order + density; close and reopen the dashboard and assert state resyncs from `state.db` with no stale cards; kill+restart the SSE connection and assert no stale cards.

---

## 12. Interfaces produced for later phases

**The SSE envelope (§4.1, §8.2) is the cross-phase contract.** Every later phase emits into / consumes from `/api/events`:
- **Phase 3 (config/onboarding):** consumes `state_update` for the dashboard it gates; the launch modal replaces this phase's minimal New Agent trigger (same `POST /api/sessions`). Reuses `tokens.css` theming hooks and the App shell + `Header`.
- **Phase 4 (persistence/archive):** adds REST archive/search/resume over `state.db` (FTS5 full-text index); `GET /api/sessions/{id}/transcript` (added here, in-memory) is upgraded to read persisted history; resume produces normal `state_update`/`new_message` flow — no bus change.
- **Phase 5 (coordination/notifications):** emits `notification` events (type + payload pinned in §8.2) and message indicators via `state_update` (e.g. a future `has_messages` field on `AgentState`). The in-process messaging MCP server reads/writes `state.db`; the Nudger does not touch the bus directly; status changes flow through the state manager.
- **Phase 6 (terminal runtime / switch-runtime / groups):** enables the **stubbed** menu items (Switch runtime, Move to group); F2 group sections extend `CardGrid` (the `AgentState.group` field already rides the envelope). Switch-runtime re-launch surfaces as ordinary `state_update`s.
- **Phase 7 (activity map):** pure consumer of existing `state_update` + `new_message` — no new server data.

**UI shell extension points:** `App.tsx` shell + `Header`, the Zustand store trio, the `api/sse.ts` singleton, `tokens.css` color variables, and the renderer registry in `TranscriptView` (later phases add renderers by `kind` without touching existing ones).

---

## 13. Resolved decisions

1. **Status channel:** **`POST /api/hook`** (token-authed) is the primary ingest; the server is the sole writer to `state.db` and emits `state_update` after each applied change. This adds zero new transport (the server already serves the UI over localhost HTTP) and keeps updates ordered and race-free. The reconciliation sweep over `sessions/` is a fallback for missed hooks and out-of-band transcript files only — never the channel. See the architecture rationale in `docs/architecture-decisions.md` (D1, D2).
2. **Per-client buffer size:** **256 envelopes.** Rationale in §9 — absorbs multi-hundred-ms render stalls under ~10 concurrent fast agents while keeping memory trivial (~tens of KB/client).
3. **Drop policy / threshold:** **drop-oldest**, applied as soon as the 256-slot buffer is full (no separate threshold). Optional hard cutoff: if cumulative `dropped > 1000` for a client, close it to force a clean rehydrate (§9). Newest frame always wins because `state_update` carries full state and is self-correcting.
4. **Keepalive:** **10s ping**; client watchdog force-reconnects after **25s** without ping/message.
5. **"Last output line" source:** **`status.detail`** is primary (cheap, already on `AgentState`); **fallback** to the latest `assistant_text` delta tracked client-side; render nothing if both empty (§6.4).
6. **Reconnect resync:** **full hydration burst** rebuilt from `state.db` (the source of truth) terminated by an `__hydrated__` marker; client removes any agent absent from the completed burst (no stale cards) and refetches transcript for the open chat panel. No server-side delta replay ring.
7. **Effective-state computation:** join the `agents`, `running`, and `status` rows in `state.db` per recompute; identity changes (rename) update the `agents` row and are pushed via `Manager.Touch` from the REST handler (§3.6).
8. **Per-agent SSE replacement:** Phase 1's `GET /api/sessions/{id}/events` is **deleted**; transcript deltas flow as `new_message` keyed by `agent_id` on the multiplexed bus (§4.3, §8.6).
9. **Stubbed menu items:** rendered **visible but disabled** with a phase tooltip, not hidden (§1.3, §6.6).
