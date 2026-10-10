# TS-13 — Remote control

**Status:** Partial
**Code:** `internal/remote/`, `internal/server/remote*.go`, `ui/remote.html`, `ui/src/remote/`
**Absorbed:** —

## 1. Scope

The architecture behind FS-20: Chuck's embedded Tailscale node, the second (tailnet) HTTP
listener and its guard, phone pairing and device credentials, the phone's API allowlist, Web Push
delivery, keep-awake, and the phone web-app bundle. Out of scope: any Chuck-operated relay,
cloud service, or account; Tailscale Funnel or any public exposure; native phone apps; changes to
the loopback desktop interface's trust model (TS-05.R1–R3 keep governing it).

Verified evidence (2026-09-25, `tailscale.com` v1.102.5 source): `tsnet.Server.ListenTLS` fetches
certificates on handshake through `LocalClient().GetCertificate` and refuses to start unless the
tailnet has MagicDNS and HTTPS certificates enabled (`tsnet/tsnet.go:1371-1394`);
`local.Client.WhoIs` returns `Node.StableID` and `UserProfile.LoginName` for a peer address;
`ipnstate.Status.CertDomains` names the node's certificate domain; the interactive sign-in URL is
readable as `StatusWithoutPeers().AuthURL` with `BackendState`; `Server.Close` is idempotent and
closes listeners before shutdown. The module requires Go ≥ 1.26.6 and adds roughly 15 MiB
(stripped) to 22 MiB (unstripped) to a binary. Apple documents Home Screen web apps as having
cookies and storage separate from Safari (WWDC23 session 10120), so pairing must complete inside
the installed app on iOS.

## 2. Design & constraints

- **R1 — One process, one remote subsystem.** `internal/remote` owns the tailnet node,
  pairing, device credentials, push delivery, and keep-awake. The existing server process hosts it;
  no helper process, daemon, or second binary is added. A remote-subsystem failure never stops the
  loopback server, agents, tasks, or pipelines; it surfaces only as FS-20.R2's **Unavailable** state
  with a stable reason code. Rationale: remote control is an optional channel, not a runtime.
- **R2 — The tailnet node is an embedded `tsnet.Server`.** Hostname is the product name
  (FS-00.R19), state lives in `$CHUCK_HOME/remote/tailscale/` (`0700`, TS-05.R5), and the node is
  not ephemeral so pairings and its address survive restarts. It starts when `remote_enabled` is
  true at startup or when the person turns it on, and is closed with `Server.Close` when turned off or
  at shutdown. Enable/disable transitions are generation-scoped so a late start from a superseded
  generation closes itself instead of publishing state (INV §4, INV §5), and a new generation starts
  only after the previous node has closed so two nodes never share the state directory. `tsnet` log
  output is routed through Chuck's logger with R12's redaction, and the node runs in Tailscale's
  no-logs mode (`TS_NO_LOGS_NO_SUPPORT`) so it uploads nothing to Tailscale's log service.
- **R3 — Connection state is derived, not guessed.** The subsystem maps
  `StatusWithoutPeers().BackendState`/`AuthURL` and `ListenTLS` prerequisite errors to exactly
  `off`, `needs_login` (with the auth URL), `starting`, `on` (with `https://<CertDomains[0]>`), or
  `unavailable` with reason `magicdns_disabled`, `https_disabled`, `node_error`, or
  `listener_error`. It publishes each change as one `remote_update` SSE event on the loopback stream
  only and answers `GET /api/remote` with the same state (TS-03.R46). The auth URL is shown to the
  desktop only; it is never sent to a phone.
- **R4 — The tailnet listener has its own guard and shares handlers.** `ListenTLS("tcp",
  ":443")` feeds a second `http.Server` whose chain is: remote guard → device authentication →
  allowlist → the same handler functions the loopback mux uses (INV §2). It never passes through
  `localOnly`, and the loopback mux never passes through the remote chain (TS-05.R23). The remote
  guard requires `Host` to equal the node's certificate domain, requires a present `Origin` to equal
  `https://<that domain>`, requires `Origin` on every non-GET request, and requires `WhoIs` to
  resolve the peer; anything else is `403`. `ListenFunnel` is never called.
- **R5 — The phone API is an explicit allowlist.** One table in `internal/server` names
  every method and path pattern reachable on the tailnet listener; a route not in it answers `404`
  with code `remote_route_not_available`, whether or not the loopback mux serves it. The table
  contains only what FS-20 needs: health, capabilities, and the SSE stream; session list, detail, and
  transcript reads; `prompt` (POST/GET/DELETE), `steer`, `cancel`, `stop`, `resume`, and
  `permission`; `POST /api/sessions` for Ask Chucky; task list/detail/create and
  `cancel`/`result`/`retry`/`rearm`; pipeline run list/detail/start and
  `continue`/`retry`/`replace`/`repair-cleanup`/`stop`; read-only project, role,
  pipeline-template, and configuration (`GET /api/config`, for notification preferences) reads; and
  the phone routes of R8/R9/R10. It never contains `/mcp`, `/api/hook`, the terminal WebSocket, the
  directory picker, any config or federation write, archive, annotations, clone, switch runtime,
  rename, identity, group release, signals, layout, worktrees, any `/api/remote` management route,
  or the backend catalog (its backend and model `env` may hold keys): the phone omits runtime fields
  and the Mac resolves the defaults the desktop New Agent form preselects; for a pipeline start the
  tailnet listener fills the standing owner and every dedicated coordinator with that default
  before the shared handler validates the run. A test enumerates the
  loopback route inventory and fails when a route is neither allowlisted nor explicitly denied
  (INV §10). Unauthenticated GETs outside `/api/` reach only the phone app's static files (R14).
  R15–R17 supersede only the annotation, pipeline-replacement-runtime, and re-arm-value
  restrictions in this item; the full backend catalog and its secrets remain denied.
- **R6 — Remote requests cannot widen permission policy.** A launch or task-creation body
  arriving on the tailnet listener may not carry a per-launch permission-bypass override or any
  field the phone's FS-20.R15 forms do not set; such a body is rejected with
  `remote_field_not_allowed` before any process work. Every authenticated non-GET tailnet body is
  buffered to at most 1 MiB before any route filter or handler runs; a larger one is
  `413 remote_body_too_large` (INV §16). Effective permission policy still composes
  global and role policy exactly as TS-05.R9 defines. Every accepted remote mutation is attributed to
  its device id in the server log.
- **R7 — A device credential is a hashed, sliding, node-bound cookie.** Pairing mints a
  32-byte random token; SQLite stores only its SHA-256 hash with the phone's `Node.StableID` and
  login from `WhoIs` (TS-02.R37). The phone receives it only as the cookie
  `__Host-remote_device` (`Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`, `Max-Age` 400 days).
  Authentication hashes the presented cookie, finds the device, and requires the request's `WhoIs`
  `StableID` to equal the stored one; a mismatch is `401 remote_device_mismatch` and is not treated
  as a revocation. A valid cookie older than one day is re-issued on the response so an active phone
  never reaches the browser's cookie cap (FS-20: pairings do not expire). `last_seen_at` is written
  at most once a minute per device. Tokens never appear in URLs, SSE, API bodies, or logs.
- **R8 — Pairing is a desktop-issued, single-use code claimed atomically.** A loopback-only
  `POST /api/remote/pairings` creates the only outstanding code (a new one invalidates the previous):
  8 characters from an unambiguous alphabet, valid for five minutes, held in memory only. The QR
  encodes `https://<domain>/pair#<code>` so the code stays in the fragment and never reaches request
  logs. The phone posts `{code, name}` to `POST /api/remote/pair` (the only unauthenticated
  non-static tailnet route); a match atomically claims the code (INV §5) and creates a pending
  request surfaced to the desktop through `remote_update`; at most one request waits, and a newer
  claim declines an older one. Five failed attempts invalidate the current code, and ten failures
  from one peer node within five minutes answer `429 remote_rate_limited`. Failure tracking prunes
  expired windows on every claim and remembers at most 256 peer nodes, evicting the oldest window
  when full. The desktop answers
  through loopback-only `POST /api/remote/pairings/{id}/allow|decline`; allow commits the device row
  first (INV §15). The phone long-polls `GET /api/remote/pair/{pending_id}` (up to 20 seconds per
  request) with the pending id as its bearer of that wait, from the same peer node; the response
  that reports `allowed` sets R7's cookie exactly once. Declined, expired, or reused codes return
  `remote_pairing_invalid` without revealing which. The tailnet request log redacts the pending id.
- **R9 — Device management is desktop-only; self-service is phone-only.** Loopback-only
  routes list devices (id, name, paired/last-seen times, notification state — never token hashes,
  node ids, or push endpoints), rename, and revoke. Tailnet routes under `/api/remote/self` let a
  paired phone read its own name, notification state, and the VAPID public key, rename itself,
  unpair itself, and set or clear its push subscription. Revoke and
  unpair delete the device row and cancel that device's open requests and SSE streams through a
  per-device context registry, closing them within one second (INV §4).
- **R10 — One server-side attention definition feeds Home and push.** A single helper
  classifies FS-20.R11's **Needs you**, **Moving**, and completion items from the agent snapshot,
  pending permissions, tasks, and pipeline runs. `GET /api/remote/home?since=<time>` returns the
  three lists; the phone refetches it on relevant SSE events (debounced) and keeps `since` in its own
  storage. The push sender uses the same helper's transitions, so Home and notifications cannot
  disagree about what needs the person (INV §2).
- **R11 — Push is standard Web Push sent from the Mac.** Delivery uses RFC 8030 with
  RFC 8291 `aes128gcm` payload encryption and RFC 8292 VAPID. The VAPID key pair is generated on
  first need into `$CHUCK_HOME/remote/vapid.json` (`0600`). A subscription's endpoint must be
  `https` on a known browser push-service host (Google FCM, Apple `*.push.apple.com`, Mozilla
  autopush, Microsoft WNS) or it is rejected, so a phone cannot make the Mac post to arbitrary URLs.
  Sends leave over the Mac's ordinary internet connection, not the tailnet. The payload is at most
  4 KiB of `{title, body, tag, url}` built only from FS-20.R19's fields; `tag` is the project or run
  so the service worker replaces rather than stacks (FS-20.R20), and it also sets the RFC 8030
  `Topic`. The first new item for a tag sends at once and opens a 10-second window; items arriving
  inside it send once as a summary when it closes. Only transitions notify: the first evaluation
  after start records what already needs the person without sending, and nothing sends while remote
  control is off. Remote enablement, mutes (FS-02.R24), and the per-device switch are reread before
  every send attempt, including each retry; a queued note keeps its items' mute types for that check,
  and a summary is dropped only when every item it covers is muted. A queue of 64
  with one worker sends; a `404`/`410` marks that endpoint's subscription expired (FS-20.R28), other
  failures retry after 2, 4, and 8 seconds and are then dropped and logged without payload or
  endpoint. The VAPID `sub` claim is the project's https URL, which Apple requires and which does
  not disclose the person's tailnet name. Verified against `webpush-go` v1.4.0.
- **R12 — Remote secrets follow TS-05.R7.** Device tokens and hashes, pairing codes,
  pending ids, push endpoints and keys, the VAPID private key, and `tsnet` node keys never appear in
  logs, error bodies, SSE, or API responses.
- **R13 — Keep-awake is a supervised `caffeinate` child.** When `keep_awake` is true and
  the work-active predicate holds (any agent `busy`, any pending permission request, or any pipeline
  run `queued` or `running`), one owner goroutine runs `/usr/bin/caffeinate -i -w <server pid>` and
  kills it when the predicate clears, the setting turns off, or the server stops; `-w` ends it if the
  server dies. It never uses `-d`, `-s`, or display assertions. Off macOS the setting reports
  unavailable.
- **R14 — The phone app is a second embedded UI entry.** `ui/remote.html` and
  `ui/src/remote/` build in the same Vite project and embed with the desktop bundle (TS-08.R73). The
  tailnet listener serves the phone entry for every app path, its web-app manifest, and its service
  worker at `/sw.js`; it never serves the desktop bundle, and the loopback server never serves the
  phone's files. The phone
  app reuses `ui/src/api` schemas and client (TS-02.R11 lockstep) and uses a plain `EventSource`
  with its cookie; it does not use the desktop SharedWorker.
- **R15 — Runtime choices use a secret-free phone projection.** A paired phone may read
  `GET /api/remote/runtime-options`, which projects only configured backend ids/names and model
  ids/names with their allowed efforts, default effort, and fast-mode support (true only where the
  shared fast validator would accept it). It never projects
  backend type, environment, credentials, executable paths, credential-check detail, or federation
  configuration. The phone may submit one of those assignments to pipeline orchestrator
  replacement, whose tailnet body admits only `revision` and `orchestrator`; the shared launch
  validator re-reads current configuration and rejects an unknown or unsupported
  backend/model/effort/fast combination before changing the run (FS-20.R31).
- **R16 — Annotation delivery is allowlisted for paired phones.** The tailnet route table
  includes the existing `POST /api/sessions/{id}/annotations` handler for FS-20.R32. The remote
  chain bounds its body and admits only the shared request's top-level fields (`annotations`,
  `overall_instruction`, `target`) before the shared FS-13 validation and commit-before-delivery
  path; it adds no phone-only annotation representation or delivery mechanism.
- **R17 — Remote Re-arm has the desktop's value authority.** A paired phone may submit
  any complete arm set that the existing `POST /api/tasks/{id}/rearm` handler accepts under
  FS-16.R15/R23, not only the current set or an empty set. The remote chain bounds and
  schema-validates the request, while the shared handler remains the one graph/state validator
  (FS-20.R30).
- **R18 — Phone transcript reads are windowed.** `GET /api/sessions/{id}/transcript` accepts
  `limit` (clamped to 750) and `before_seq`; a windowed read streams the file and retains at most
  that many events and about 1 MiB of event payload, answering `{agent_id, events, has_more,
  pending_permission, latest_assistant}`. The newest unresolved permission request and the latest
  top-level assistant reply are derived from the whole session in constant space, so they survive
  the window boundary. A tailnet read is always windowed (750 events by default); a loopback read
  without `limit` or `before_seq` keeps the unwindowed `{agent_id, events}` shape (FS-20.R13,
  INV §16). The phone serializes older and gap-recovery reads, bounds a gap recovery to three
  windows, and rebases visibly to the latest window if that bound cannot reconnect retained history.
- **R19 (shipped 2026-10-02) — The phone mirrors the desktop dashboard from existing data.** Phone routes
  are `/` (project dashboard), `/project/{id}`, `/agent/{id}`, `/run/{id}`, `/phone`, and `/pair`;
  `/new`, `/task/{id}`, and any other path render Home. Home and the project page are derived in the
  browser from the SSE agent snapshot the phone already hydrates, `GET /api/projects`, and R20's
  home response; no new server read is added. The desktop's project-card derivation in
  `ui/src/features/dashboard/ProjectDashboard.tsx` — active-project filtering, unavailable-project
  detection, agent counts, per-state summary, and project ordering — and the card grid's agent
  ordering (FS-02.R45) move into one pure helper under `ui/src/` used by both entries, so the phone
  cannot drift from the desktop's rules (INV §2). Agent rows reuse `AgentState` fields
  (`name`, `role`, `backend`, `model`, `state`, `detail`, `running`, `archived`, `clone`,
  `fast_available`); an agent or project that becomes archived or removed in that snapshot drives
  FS-20.R39's state on an open screen (FS-20.R33/R34/R39).
- **R20 (shipped 2026-10-02) — Home attention narrows to agents and runs.** R10's single helper keeps
  classifying agent and pipeline-run attention but no longer reads tasks, and stops producing the
  agent/task **Moving** and every **Since you last looked** item. `GET /api/remote/home` takes no
  `since` and returns `{needs_you, active_runs}`: `needs_you` as before minus task items, and
  `active_runs` the non-terminal runs R10 classified as moving, each with its project and stage
  position, which the project page filters by project. The push sender keeps using the same
  helper, so task `interrupted`/`dependency_failed` transitions stop producing pushes and no push
  URL names `/task/`; agent and run pushes are unchanged (FS-20.R18/R33/R40, INV §2). The phone
  drops its stored `homeSeenAt` key.
- **R21 (shipped 2026-10-02) — The tailnet allowlist follows the new phone surface.** R5's table changes
  as follows; every change keeps the existing guard, device authentication, 1 MiB body bound, and
  shared handler, and the inventory test continues to fail on an unclassified route (INV §10):
  - `POST /api/sessions` admits `role`, `project`, `name`, `backend`, `model`, `effort`, and
    `fast`; `interface`, `driver`, `group`, permission-bypass, resume, and every other field stay
    `remote_field_not_allowed`. The shared launch validator resolves omitted runtime fields to the
    desktop defaults and rejects an unknown or unsupported backend/model/effort/fast combination
    before any process work (FS-20.R35, R6);
  - added: `POST /api/sessions/{id}/rename` admitting `name`; `POST
    /api/sessions/{id}/session-config` admitting `effort` and `fast`; `POST
    /api/sessions/{id}/switch-runtime` admitting `backend`, `model`, and `effort` (never `interface`
    or `driver`); bodiless `POST /api/sessions/{id}/clone` and `POST /api/sessions/{id}/archive`
    with no field filter, like `stop`; `GET /api/sessions/{id}/files` and `GET
    /api/sessions/{id}/commands`; and `GET /api/sessions/{id}/file` behind R22 (FS-20.R36/R37);
  - moved to the denied set: every `/api/tasks` route (`GET` list/detail and `POST` create,
    `cancel`, `result`, `retry`, `rearm`), superseding R17 and R5's task entries (FS-20.R40).
    Annotation's new-task target keeps working because the shared annotation handler creates its
    task server-side;
  - still denied: `POST /api/sessions/{id}/restore`, `GET /api/sessions/{id}/messages`,
    `file-search`, `available-commands`, archive reads, worktree routes, and the backend catalog
    (FS-20.R16/R38).
  Each accepted remote mutation stays attributed to its device id in the server log (R6).
- **R22 (shipped 2026-10-02) — Remote file reads are limited to the agent's tracked paths.** The tailnet
  chain wraps the shared `handleFileRead` with a filter that answers `404
  remote_file_not_tracked` unless the `path` query is byte-equal to the `path` column of a
  `tracked_files` row for that `{id}` — the value `GET /api/sessions/{id}/files` returned. The
  filter marks the request no-follow; the shared reader then opens the final path component with
  `O_NOFOLLOW` and classifies with `Lstat`, so a symbolic link at the tracked path is refused as
  `remote_file_not_tracked` without reading, while loopback reads keep FS-03.R64's link-following
  behaviour. Directory symlinks above the tracked file (for example `/var` → `/private/var`) still
  resolve. All other refusals, the chat-interface gate, size limit, UTF-8 check, and response shape
  stay those of TS-03.R48/TS-05.R24. The filter reads only the tracked-path index, never the
  filesystem, before deciding; it adds no durable access record (FS-20.R37).
- **R23 (shipped 2026-10-02) — Runtime options carry the desktop defaults.** R15's projection adds
  `default` (true on the one backend the launch resolver treats as default, FS-01.R5) and
  `default_model` on each backend, still projecting no type, environment, credential, path, or
  federation data. The phone's New agent and Switch runtime pickers preselect from these values and
  the agent's current runtime respectively; effort defaults to the model's `default_effort`
  (FS-20.R35/R36).

- **R24 (planned) — Paired phones gain only the confirmed group controls.** Extend R21's
  launch field allowlist with `group`. Allow existing `POST /api/sessions/{id}/identity` only
  with a required string `group`; reject name, project, role, runtime and all other fields with
  `remote_field_not_allowed`. Allow TS-03.R57's project-scoped group stop/archive routes with
  exactly the required string `group`, rejecting all other fields; keep global
  `/api/groups/{group}/release` denied. All use R4's guard,
  device authentication, R6's 1 MiB body bound/device attribution and the shared desktop services
  and group validator. No phone-only persistence or broad identity/config access is added.
  Archive reads, Restore and layout writes remain denied. Inventory tests independently cover
  the new route producers and rejection of forbidden fields, revoked/unpaired devices and global
  group release. This narrowly supersedes R5/R21's group and identity exclusions.

## 3. Interfaces & data shapes

Loopback-only: `GET /api/remote`, `PUT /api/remote` `{enabled, keep_awake}`, `POST
/api/remote/pairings`, `POST /api/remote/pairings/{id}/allow|decline`, `GET /api/remote/devices`,
`PATCH|DELETE /api/remote/devices/{id}`. Tailnet-only: `POST /api/remote/pair`, `GET
/api/remote/pair/{pending_id}`, `GET /api/remote/home`, `GET /api/remote/runtime-options`,
`GET|PATCH|DELETE /api/remote/self`,
`PUT|DELETE /api/remote/self/push`. SSE `remote_update` (loopback) signals a change to the
`GET /api/remote` view; the desktop refetches it.

`GET /api/remote` and `PUT /api/remote` → `{state, reason?, auth_url?, address?, keep_awake,
keep_awake_available, pending_pairing?: {id, name, login?, requested_at}, devices: [{id, name,
paired_at, last_seen_at, notifications: on|off|expired}]}`. `POST /api/remote/pairings` → `{id,
code, qr_url, qr_svg, expires_at}`; `qr_svg` is a self-contained SVG rendering of `qr_url`, so the
desktop needs no QR library. `GET /api/remote/home` → `{needs_you, moving, since_last}` of `{kind:
agent|task|run, id, title, project, state, reason, agent_id?, stage?, outcome?, since}`; run
summaries carry `stage_number`/`stage_count` for the stage position. `GET /api/remote/self` →
`{id, name, notifications, vapid_public_key}`.

Shipped (R20): `GET /api/remote/home` → `{needs_you, active_runs}` with the same item shape and no
`since` parameter.

`GET /api/remote/runtime-options` → `{backends: [{id, name, default, default_model (R23), models:
[{id, name, efforts, default_effort?, fast}]}]}`, sorted by id; the existing annotation and
pipeline Replace request/response shapes remain shared with loopback and gain only the R4 guard,
device authentication, allowlist, and bounded-body enforcement on the tailnet listener.

Persistence is defined in TS-02.R37; route inventory in TS-03.R46; security boundary in TS-05.R23;
build and dependency pinning in TS-06.R27; the phone bundle in TS-08.R73.

## 4. Invariants

INV §2 (one attention helper; shared handlers across listeners), INV §4 (node and device-stream
teardown), INV §5 (pairing claim and enable/disable generations), INV §8 (bounded, in-vocabulary
phone and push data), INV §10 (allowlist wiring test), INV §14 (every route guarded — the tailnet
listener by R4's guard), INV §15 (commit device rows before releasing cookies or push sends),
INV §16 (bounded pairing attempts, push queue, and coalescing).

## 5. Deviations & open decisions

- R24's narrow paired-phone group mutation access awaits human confirmation with TS-03.R57.

- Tests use a fake listener and fake `WhoIs`; the real tailnet, certificate issuance, and real push
  services are covered only by FS-20's manual gates.
- Face ID or fingerprint app lock and native apps are out of scope (FS-20 §6).
- Local browser passes use `CHUCK_DEV_FAKE_TAILNET=localhost:<port>` with a `dev`-tagged build
  (`internal/server/remote_dev.go`): a self-signed HTTPS listener with one fixed peer standing in
  for the node. Release builds never contain it.

## 6. Traceability

- Node and state: `internal/remote/remote.go`, `tsnet.go` (fake-node tests in `remote_test.go`,
  including rapid-toggle `-race`).
- Listener chain and allowlist: `internal/server/remote_routes.go`, `routes.go` (`routeTable`),
  `spa.go`; tests `remote_routes_test.go`, `spa_test.go`. Pipeline-start defaults (R5):
  `withDefaultPipelineRuntimes`; test `TestRemotePipelineStartUsesMacDefaults`.
- Pairing and devices: `internal/server/remote_pairing.go`, `internal/state/remote_devices.go`.
- Attention and Home: `internal/server/remote_home.go`, `state.ListAttentionTasks`.
- Runtime options (R15): `handleRemoteRuntimeOptions` in `internal/server/remote_home.go`; tests
  `TestRemoteRuntimeOptionsAreSecretFree`, `TestRemoteReplaceValidatesChosenRuntime`. Annotation
  (R16): `TestRemoteAnnotationUsesSharedDelivery`. R17 is superseded by R21; task denial:
  `TestRemoteDeniesEveryTaskRoute`. Allowlist and tracked-file reads (R21/R22):
  `TestRemoteAllowlistAndFieldFilter`, `TestRemoteTrackedFileRead`. Transcript window (R18):
  `internal/server/transcript_window.go`; tests in `transcript_window_test.go`.
- Push: `internal/remote/push.go`, `internal/server/remote_push.go`.
- Keep-awake: `internal/remote/keepawake.go`, `internal/server/keepawake.go`.
- Phone app: `ui/remote.html`, `ui/public/remote-*`, `ui/src/remote/`, `ui/src/styles/remote.css`.
