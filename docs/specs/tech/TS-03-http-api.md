# TS-03 — HTTP, SSE & WebSocket API

**Status:** Partial
**Code:** `internal/server`, `ui/src/api`
**Absorbed:** [`agent-dashboard-prd.md`](../../archive/agent-dashboard-prd.md) API sections and the [phase archive manifest](../../archive/phases/README.md)

## 1. Scope

This spec owns the local HTTP surface: routing, JSON conventions, status/error policy, the global
Server-Sent Events (SSE) stream, and the terminal WebSocket upgrade. Feature specs own the meaning
of each operation; protocol-specific authentication and payload details are in TS-04 and TS-05.

## 2. Design & constraints

**R1 — One loopback API serves UI and integrations.** The Go server owns `/api/*`, `/mcp`, the
terminal WebSocket, and the embedded single-page application on the same loopback listener. API
routes never fall through to the SPA; unknown `/api/*` paths return JSON `404`.

**R2 — Routes are method-specific.** Unsupported methods produce `405`; successful creates use
`201`, successful reads/updates use `200`, and successful deletes may use `204`. JSON input is
decoded once and validated before disk/process work. A shared request-body byte limit is not yet
installed across handlers (see §5).

**R3 — Structured errors are stable at new boundaries.** New or changed endpoints return
`{"error":{"code":"<stable_code>","message":"<safe text>","details":...}}` through the shared
API-error writer. Field validation may return the established
`{"error":"validation_failed","errors":[...]}` shape. Internal errors and secrets are not echoed.

**R4 — Legacy envelopes remain accepted behavior.** Some early read/config endpoints still return
flat `{"error":"message"}` bodies. They may be standardized only with an explicit compatibility
delta; clients must not assume every existing endpoint already uses R3.

**R5 — The route inventory is authoritative and complete.** The current families are:

| Family | Routes |
|---|---|
| Health/live state | `GET /api/health`, `GET /api/sessions`, `GET /api/sessions/{id}`, `GET /api/events`, `GET /api/capabilities` |
| Lifecycle/chat | `POST /api/sessions`, `prompt`, `cancel`, `stop`, `rename`, `identity`, `permission`, `resume`, `switch-runtime`; transcript read |
| Config | role/project CRUD; `GET/PUT /api/backends`, `/api/config`, `/api/layout` |
| Archive/tracking | `GET /api/archive`, session files/commands/messages |
| Coordination | `POST /api/groups/{group}/release`, `/mcp` GET/POST/DELETE |
| Federation | config-source list, preview, bind, refresh, delete |
| Producers/terminal | `POST /api/hook`, terminal WebSocket |

Adding, removing, or changing a route requires a TS-03 delta plus the owning FS/TS delta.

**R6 — Collection responses use arrays, never null.** Empty sessions, archive results, transcript
events, tracked files/commands, messages, bindings, candidates, and validation errors serialize as
`[]` where their schema is a list.

**R7 — The global SSE stream is snapshot-then-live.** `GET /api/events` atomically subscribes and
captures the current agent snapshot, emits the hydration burst, then live events. A `hydrated`
boundary lets the client prune absent agents. Periodic `ping` supports liveness; reconnect starts a
new hydration generation.

**R8 — SSE event types are versioned by payload contract.** Current types include `state_update`,
`new_message`, `notification`, `config_source_update`, `hydrated`, and `ping`. Unknown event types
are ignored by clients. Producers publish only after authoritative state is committed.

**R9 — Slow subscribers cannot block the server.** Bus/subscription buffers are bounded; overflow
uses the documented drop/resnapshot strategy. Shutdown cancels request contexts so open SSE streams
do not hold the server past its grace period.

**R10 — WebSocket upgrade is route-specific.** Only
`GET /api/sessions/{id}/terminal/ws` upgrades. Pre-upgrade errors are JSON; an agent without an
xterm PTY bridge receives a normal not-found response rather than a half-open socket.

**R12** `(planned)` — Existing project read/create/update response shapes gain the server-computed,
read-only `resource_dir` absolute-path string. It is computed from the response project's immutable
id, is not stored in `projects/{id}.json`, and any client-supplied value is ignored. `DELETE
/api/projects/{id}` remains `204`; before issuing it, Settings uses the read-only value to state
that the resource directory will be retained. The server schema, TypeScript schema, mocks, and
Settings copy stay in lockstep; no resource-content route or SSE event is added.

## 3. Interfaces & data shapes

Feature-owned request/response fields are specified in FS-01 through FS-09. Cross-cutting shapes:

```json
{"error":{"code":"validation","message":"...","details":{}}}
```

```text
event: state_update
data: {"agent":{...}}

event: hydrated
data: {}
```

Pagination uses `limit` and `offset` where exposed. Query parsing rejects malformed booleans and
integers instead of silently applying defaults.

## 4. Invariants

- **INV §1:** reconnect/hydration resets connection-scoped derived state.
- **INV §8:** snapshot + subscribe is atomic; publish follows the authoritative write.
- **INV §9:** cancellation and shutdown primitives reach long-lived HTTP handlers.
- **INV §14:** Host/Origin validation wraps the entire mux, including raw `/mcp` and WebSocket paths.
- **R11 — UI/API lockstep.** A payload field changed in the server is changed in `ui/src/api` schemas
  and tests in the same completed change; permissive client parsing is not a substitute for a specification update.
- **INV §10:** the Settings surface, HTTP responses, client schema, and lifecycle composition ship
  together; no API field or directory is left unreachable or undocumented.

## 5. Deviations & open decisions

- Error envelopes remain mixed as described by R3–R4. Standardization is a compatibility change,
  not cleanup that may be done opportunistically.
- Archive UI pagination is incomplete even though the API supports `limit`/`offset` (FS-05).
- JSON handlers do not yet apply a shared maximum request-body size. Field-level limits protect
  several operations, but uniform pre-decode bounding is a security/API backlog item.

## 6. Traceability

- Route inventory: `internal/server/routes.go`.
- Errors/middleware: `internal/server/apierror.go`, `middleware.go`, `security.go`.
- SSE/bus: `internal/server/sse.go`, `internal/bus/bus.go`, `ui/src/api/sse.ts`.
- Terminal upgrade: `internal/server/terminal.go`.
- Regression anchors: `TestUnknownAPIPath404`, `TestStartShutsDownWithOpenSSEClient`,
  `TestDNSRebindingHostRejected`, `TestCrossOriginRequestRejected`, SSE reconnect tests in
  `ui/src/api/sse.test.ts`.
