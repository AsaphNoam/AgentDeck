# TS-03 — HTTP, SSE & WebSocket API

**Status:** Current
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
| Lifecycle/chat | `POST /api/sessions`, `prompt`, `cancel`, `stop`, `archive`, `restore`, `rename`, `identity`, `permission`, `resume`, `switch-runtime`, `annotations`; transcript read |
| Config | role/project CRUD and project `archive`/`restore`; `GET/PUT /api/backends`, `/api/config`, `/api/layout` |
| Archive/tracking | `GET /api/archive`, `GET /api/archive/projects/{project}`, session files/commands/messages |
| Composer autocomplete | session-scoped file search and available-command snapshot reads |
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

**R12** — Existing project read/create/update response shapes gain the server-computed,
read-only `resource_dir` absolute-path string. It is computed from the response project's immutable
id, is not stored in `projects/{id}.json`, and any client-supplied value is ignored. `DELETE
/api/projects/{id}` remains `204`; before issuing it, Settings uses the read-only value to state
that the resource directory will be retained. The server schema, TypeScript schema, mocks, and
Settings copy stay in lockstep; no resource-content route or SSE event is added.

**R14 — Annotation batch endpoint.** `POST /api/sessions/{id}/annotations` accepts one
batch — annotations, optional overall instruction, and a target of kind `self` or `agent` (with
`agent_id`); the new-task flow is UI composition over the existing launch route plus an `agent`
target. Input is validated per FS-13.R11 before any disk or process work; the whole target is then
validated and its delivery payload composed before the durable source annotation event is appended,
and only then is the batch delivered (INV §15, FS-13.R5) — a `500` therefore never reports a failure
the retry would deliver twice. Success returns `202`
with the appended annotation event's `seq`, and a `self` target additionally mirrors the prompt
route's acceptance. Errors use the R3 structured envelope; list fields serialize as arrays (R6).
One shared annotation-block renderer composes the agent-visible content for both prompt-turn and
mail delivery (INV §2). No new SSE event type is added: the annotation event reaches clients as an
ordinary `new_message` and mail indicators reuse existing `state_update` publication (R8). The UI
schema ships in lockstep (R11).

**R15 — Onboarding readiness reuses the backend-save contract.** **Check again** submits
the existing normalized backend document to `PUT /api/backends` and consumes its existing bounded
`credentials` result. It adds no auth/login route, WebSocket, SSE event, long-lived HTTP request, or
server-started sign-in process.

**R16 — Pipelines add one conventional local REST family.** The route inventory gains
template list/create/validate and item read/update/delete under `/api/pipelines`; run list/start and
item read/delete under `/api/pipeline-runs`; a pending-proposal list at
`GET /api/pipeline-proposals`; and method-specific `continue`, `retry`, and `stop` actions on a run.
Start accepts a caller-generated request id and returns the original run for an exact idempotent
replay; reuse with different content is `409`. Invalid templates/run values are field-addressed R3
errors, stale run revisions are `409`, active-run deletion is `409`, creates are `201`, and successful
deletes are `204`. Collection/detail payloads, TypeScript schemas, and mocks ship together under
R6/R11; TS-09 owns their feature-specific fields.

**R17 — Pipeline live state uses summary SSE plus refetch.** `pipeline_update` is added to
the versioned SSE vocabulary after its authoritative SQLite commit. Its bounded payload contains run
id, monotonic revision, state, current stage/agent, and attention reason; clients ignore stale
revisions and refetch run detail for attempts/results/values. Existing `notification` events carry
pipeline needs-attention/completed categories through the ordinary mute path. Reconnect hydrates
pipeline lists through REST rather than replaying an unbounded event log.

**R18 — Archive responses expose effective search capability.** Every `GET /api/archive` and
`GET /api/archive/projects/{project}` response includes
`search_mode:"full_text"|"metadata"`, derived from the current SQLite connection rather than the
build command or a UI assumption. This field is present for listing, successful search, and empty
pages so the Archive UI can keep its search promise honest (FS-05.R30/R36).

**R19 — Effort is an optional field on existing routes, never a new one.**
`POST /api/sessions` and `POST /api/sessions/{id}/switch-runtime` accept an optional `effort`
alongside `backend`/`model`; omitting it preserves today's request and response bytes exactly, so no
existing client changes. `GET /api/backends` and `PUT /api/backends` carry each model's `efforts` and
`default_effort`; `efforts` marshals as `[]` and never `null` for a model that declares none, and the
UI defends with `?? []` at first touch (INV §11 — the exact shape that twice broke this surface).
Session and agent responses report the resolved `effort` as a plain string, empty when none resolved.
Validation failures use the existing shared field-error envelope: an undeclared level, a
`default_effort` outside `efforts`, a declared level on a backend type with no effort mechanism, and a
bracketed provider model string each name their field and reason rather than returning a bare 400
(INV §8 — the class the onboarding `HTTP 400` findings came from). No new route is added, so every
path continues to inherit the `localOnly` guard unchanged (INV §14).

**R20.** Project and agent archive use explicit action routes rather than overloading
ordinary project replacement or Stop: `POST /api/projects/{project}/archive`, `POST
/api/projects/{project}/restore`, `POST /api/sessions/{id}/archive`, and `POST
/api/sessions/{id}/restore`. Project archive returns the updated project plus stopped/archived agent
ids only after its warning-confirmed stop/archive work completes; individual agent archive stops a
running agent and archives it without a confirmation field. All actions return R3 errors: unknown
ids are `404`; Resume of an archived agent under an active project is `409 agent_archived`; a launch,
resume, agent restore, pipeline start/control, or builder launch targeting an archived project is
`409 project_archived`; a process-start request arriving while project archival holds its exclusive
claim is retryable `409 project_archiving`; and an invalid competing agent transition is `409
agent_archiving` or the existing `409 conflict` where no more specific code applies. Resume, Switch
runtime, agent restore, and pipeline start/control read the project definition to detect the archived
state; a read failure other than `ErrNotFound` (a corrupt file or an I/O/permission error) fails
closed with `500 internal` rather than proceeding as active, because the unreadable definition may
record the project archived, while `ErrNotFound` is treated as an unavailable-but-active project and
agent archive proceeds regardless. `GET
/api/projects` and project create/update responses add `archived`; agent/session responses and every
`state_update` include `archived`. `GET /api/archive` changes from a flat list to paginated project
groups, and `GET /api/archive/projects/{project}` pages that group's agent rows. FS-05.R36 owns the
group-versus-agent totals, ordering, `q`/`active` corpus, and pagination semantics. A group contains
its durable project id, current title/color when configured, `project_status` (`active`, `archived`,
or `missing`), and `archived_agent_count`; its agent rows retain the existing archive metadata/search
fields plus `archived`. No new SSE type is added: committed agent archive changes publish ordinary
full `state_update` payloads, and project mutations invalidate/refetch the existing project catalog.

**R21 — Appearance extends the existing config route only.** `GET /api/config`
adds optional string `appearance_skin` and optional `appearance_skin_warning`; `PUT /api/config`
accepts `appearance_skin` in its existing partial-merge body. Omission preserves the stored choice;
empty string selects Core and `sky-grove` selects the built-in skin, while another submitted id
returns the existing field-validation envelope naming `appearance_skin` and writes nothing. The
Core write omits the optional field from the rewritten document. `GET` returns the raw stored id and
omits `appearance_skin_warning` for empty/known values; an unknown hand-edited id is returned with
`appearance_skin_warning:"unsupported"`, and the existing corrupt-config fallback returns Core with
`appearance_skin_warning:"config_unreadable"`. An ordinary read failure remains the existing API
error and the client renders Core while Settings surfaces that query error. No appearance route,
SSE event, cache-control rule, or skin-content response is added. The TypeScript schema accepts
unknown read strings long enough to fall back safely, recognizes only the two warning codes, and the
Go/frontend/manifest supported-id sets have a lockstep regression (R11).

**R22** — Configuration-source routes describe backend-global bindings. `GET
/api/config-sources` has no required project query and returns candidates/bindings for the global
surface; `project` becomes optional on preview/refresh/delete, where omission means user-level
preview or refresh and an explicit project retains the existing resolver-compatible behavior. A
persisted binding is never project-scoped: a later launch still resolves with its actual project
under TS-07.R2/R4. `PUT /api/config-sources/{backend_id}` adds optional
`enable_model_sync:boolean`; the normal Settings connection sends `true`, while existing preview,
override, and compatibility callers retain their current bytes and behavior when it is omitted.
On a successful enabled bind, the response adds non-secret `model_sync_enabled:true` and
`models_added:<non-negative integer>` alongside the binding view; the `GET /api/backends` refetch
is the authoritative resulting catalog. The request/response schemas, mocks, query keys, and source
SSE invalidation change in lockstep under R11/R13. All routes remain under the existing `localOnly`
guard; no new configuration-source endpoint or provider credential transport is introduced.

**R23** — `POST /api/backends` is an item-scoped create operation and the only route
added by this change. Its body is
`{backend_id:<valid-id>,name:<display-name>,type:<backend-type>,connect_native_configuration?:boolean}`.
The server builds the type's canonical starter backend/model from the same authority as fresh-home
seeding, applies the submitted display name, validates the id and complete prospective catalog, and
inserts only that entry into the current durable catalog. It never accepts or writes the browser's
whole-catalog draft. An empty catalog makes the new entry default; otherwise the existing default is
preserved. Invalid input, unsupported native connection for OpenCode/OpenHands, or the initial
catalog write failing returns R3/field errors and creates nothing. Reusing an id with a different
requested name or type is `409 backend_exists`; replaying the same id/name/type is idempotent even if
an earlier connection attempt already added models or enabled autosync. It returns the same backend
and may safely continue/return the requested connection result. The first create returns `201`; an
exact replay may return `200`.

For Claude/Codex with `connect_native_configuration:true`, the server first makes the backend
durable, then orchestrates TS-07.R16's standard auto-root, user-level, Linked preview/token/bind with
`enable_model_sync:true`; the request accepts no project, root, profile, claims, or mode. Success
returns `{backend_id,backend,connection}` with `connection.status:"connected"`, the ordinary redacted
binding, `model_sync_enabled:true`, and `models_added`. If creation succeeds but discovery, preview,
consent, validation, catalog/source persistence, or bind fails, creation still returns success with
the saved backend and
`connection:{status:"unbound",error:{code,message,details?}}`; no binding, installed generation, or
source-update event is claimed. The nested error uses R3's safe vocabulary and tells the client that
the ordinary connection action is retryable. Create-only omits `connection`. Existing
`GET /api/backends` returns the current document's strong `ETag`; `PUT /api/backends` remains the
complete-document editing operation and requires that value in `If-Match`. A changed catalog returns
R3's `409 backend_catalog_changed` response and preserves the submitted browser draft for reload or
reconciliation. A successful POST/PUT response returns the resulting `ETag`. POST/PUT/source-bind
mutations share TS-07.R17's catalog lock; schemas, mocks, backend/source query invalidation, and
error display ship together under R11/R13.

**R24 — Composer autocomplete uses two session-scoped reads.** `GET
/api/sessions/{id}/file-search?q=<text>` resolves the known chat session's working directory on the
server and returns `{agent_id,files:[<relative-path>]}`. The query is decoded as text, never joined
as a caller-chosen path. Results are relative, slash-separated, deterministically ranked by simple
case-insensitive basename/path matches, and capped at 50. The inventory uses Git's tracked,
untracked, and effective-ignore view when the directory is in a Git worktree; otherwise a bounded
filesystem walk is used. Neither path follows directory symlinks or traverses `.git`, and canonical
containment is checked before a result is returned.

`GET /api/sessions/{id}/available-commands` returns
`{agent_id,commands:[{name,description,input_hint?}]}` from TS-04.R24's latest in-memory snapshot.
It does not ask the adapter to refresh and creates no persisted row or transcript event. Both lists
obey R6. An unknown agent is `404`; an existing stopped or non-chat agent uses the shared typed
runtime/conflict error. Search, Git, traversal, or command-source unavailability returns the safe
R3 error/empty shape required by FS-03.R32 and never affects the prompt route. The TypeScript client,
mocks, and component consumers ship with both handlers under R11.

**R25 — The prompt route wakes a stopped wakeable chat agent synchronously.** When
`POST /api/sessions/{id}/prompt` finds no live runtime handle for an existing chat agent that
FS-01.R33 can wake, the handler invokes the shared wake helper (TS-01.R16) inside the same request
— bounded by TS-04.R22's per-stage ACP deadlines — and then delivers the prompt to the woken
session. The route inventory (R5), request/response shapes, and success status are unchanged; the
request simply carries the resume latency. Failure mapping: an agent the wake gates exclude keeps
today's `404`/typed errors; a failed wake returns the typed resume error the explicit resume route
would return; losing TS-01.R16's shared exclusive wake claim returns the existing `409` conflict,
which the client may retry.

## 3. Interfaces & data shapes

Feature-owned request/response fields are specified in the owning FS, including FS-14 for pipeline
templates, run summaries/details, and controls. Cross-cutting shapes:

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
- **R13 — Config map normalization.** A config response with a map that the UI iterates, including
  `backends` and each backend's `models`, must never expose a nil map as JSON `null`. A malformed or
  structurally incomplete hand-edited document follows its feature-owned fallback/error path with a
  diagnostic that names the source file; the UI still uses a null-safe boundary guard.
- **INV §10:** the Settings surface, HTTP responses, client schema, and lifecycle composition ship
  together; no API field or directory is left unreachable or undocumented.

## 5. Deviations & open decisions

- Error envelopes remain mixed as described by R3–R4. Standardization is a compatibility change,
  not cleanup that may be done opportunistically.
- JSON handlers do not yet apply a shared maximum request-body size. Field-level limits protect
  several operations, but uniform pre-decode bounding is a security/API backlog item.

## 6. Traceability

- Route inventory: `internal/server/routes.go`.
- Errors/middleware: `internal/server/apierror.go`, `middleware.go`, `security.go`.
- SSE/bus: `internal/server/sse.go`, `internal/bus/bus.go`, `ui/src/api/sse.ts`.
- Terminal upgrade: `internal/server/terminal.go`.
- Archive action and grouped query routes: `internal/server/{archive,archive_actions}.go`,
  `ui/src/api/client.ts`; `TestArchiveProjectRespondsWithActionLists`.
- Composer autocomplete (R24): routes in `internal/server/routes.go`, handlers beside
  `internal/server/files_commands.go`, runtime command snapshots from TS-04.R24, and client shapes in
  `ui/src/api/{client,types}.ts`.
- Appearance config projection: `internal/server/config_handlers.go`,
  `internal/server/config_endpoint_test.go`, `ui/src/{api/config.ts,schemas/config.ts}`.
- Regression anchors: `TestUnknownAPIPath404`, `TestStartShutsDownWithOpenSSEClient`,
  `TestDNSRebindingHostRejected`, `TestCrossOriginRequestRejected`, SSE reconnect tests in
  `ui/src/api/sse.test.ts`.
