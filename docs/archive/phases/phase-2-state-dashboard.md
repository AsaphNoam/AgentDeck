# Phase 2 — State manager, SSE event bus, dashboard card grid

**Status:** ready to build after Phase 1
**Features:** F1 (multi-agent dashboard)
**Depends on:** Phase 1
**Enables:** Phases 3–7 (the UI shell + live event bus they all plug into)

---

## 1. Goal

Turn the single-agent loop into a supervisable, multi-agent dashboard. Add the SQLite-backed **state manager** with its **`POST /api/hook`** ingest endpoint and the multiplexed **SSE event bus**, then build the React card-grid home view that renders every agent live. After this phase you can run several agents at once and watch their status change in real time, reorder cards, and open a chat panel.

---

## 2. Scope

### In scope
- State manager: SQLite-backed (`state.db`), server is sole writer. Primary status ingest is `POST /api/hook` (token-authed); a reconciliation sweep over `sessions/` is a fallback for out-of-band transcript files.
- SSE event bus: one stream per browser client, bounded per-client buffer with drop-oldest backpressure, ~10s keepalive ping.
- `GET /api/events` (multiplexed) replacing the interim per-agent stream from Phase 1.
- React app shell: routing, SSE client, global agent store.
- Card grid (F1): name, role, project, backend/model, color-coded state badge, context-usage indicator, last-output-line preview.
- Drag-reorder + density controls persisted to `layout.json`.
- Card right-click context menu (Open chat, Rename, Stop wired now; Switch runtime / Clone / Move to group stubbed → enabled in Phases 6/2-groups).
- Chat panel UI consuming the Phase 1 streaming surface (the full F3 view).

### Out of scope
- Config editing UI (Phase 3), archive (Phase 4), messaging indicators (Phase 5), task-group sections (Phase 6/F2), future-phase candidates such as the activity map.

---

## 3. Detailed requirements

### 3.1 State manager (master PRD §4.2)
- `POST /api/hook` (token-authed) is the primary status ingest: validate the per-launch token, apply the update to `state.db` (identity/running/status), and emit a `state_update`. The chat runtime also writes status to `state.db` directly from the ACP stream.
- Compute the agent's effective state by joining identity + running + status rows; emit `state_update` on every applied change.
- A reconciliation sweep over `sessions/` (fsnotify or periodic) is a **fallback only**, to pick up transcript files written out-of-band by the agent CLI.
- On startup, read `state.db` so the dashboard reflects already-running agents.

### 3.2 SSE event bus (master PRD §4.3)
- One event stream per connected client; per-client bounded buffer; drop-oldest under backpressure.
- Keepalive `ping` ~every 10s.
- Event types: `state_update`, `new_message`, `notification` (emitted from later phases), `ping`.
- Resilient reconnect on the client (last-event tracking; full state resync on reconnect since `state.db` is the source of truth).

### 3.3 Dashboard card grid (F1)
- A card per active agent showing all fields above; badge colors map to `busy | idle | waiting_input | done | error`.
- Cards update live from `state_update` within ~1s of the underlying state change.
- Drag-reorder; density (cards per row, gap) adjustable. Order + density persist to `layout.json` via `GET/PUT /api/layout`.
- Right-click context menu with the actions listed in scope.
- Empty state when no agents are running (links to launch — full launch modal is Phase 3, but a minimal launch trigger should exist).

### 3.4 Chat panel (F3, full)
- Opens from a card. Renders assistant markdown, tool calls with arguments, tool results, file diffs, and inline Approve/Deny permission prompts from the stream.
- Send a prompt; stream tokens via SSE. Cancel button (`/cancel`). Shows context-window usage and current model.

---

## 4. REST/SSE surface added/changed

```
POST /api/hook          hook lifecycle ingest (per-launch token) → applied to state.db
GET  /api/events        multiplexed SSE (state_update, new_message, notification, ping)
GET  /api/layout        card order + density
PUT  /api/layout        persist card order + density
```

The Phase 1 per-agent `/sessions/{id}/events` is superseded by the multiplexed bus; transcript deltas now flow as `new_message` events keyed by `agent_id`.

---

## 5. Acceptance criteria

- [ ] Launching an agent adds a card within 1s without a manual refresh.
- [ ] A status badge reflects busy/idle/done live as the agent works.
- [ ] Running 3+ agents shows 3+ cards all updating independently from the single SSE stream.
- [ ] Dragging cards to reorder and changing density persists across a full page reload.
- [ ] Closing and reopening the dashboard resyncs current state from `state.db` (no stale cards).
- [ ] Opening a card shows the full streaming chat with working Approve/Deny gating, cancel, and context-usage display.
- [ ] SSE survives a dropped connection and resyncs.

---

## 6. Open questions
- Backpressure policy tuning: buffer size and drop-oldest threshold under many fast-streaming agents.
- "Last output line" preview source: derive from the latest `assistant_text`/`status.detail`? (Recommend `status.detail` for cheapness, fall back to last transcript line.)
