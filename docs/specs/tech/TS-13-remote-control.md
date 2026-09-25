# TS-13 — Remote control

**Status:** Partial
**Code:** — (planned: `internal/remote/`, `internal/server/remote*.go`, `ui/remote.html`, `ui/src/remote/`)
**Absorbed:** —

## 1. Scope

The architecture behind FS-20: AgentDeck's embedded Tailscale node, the second (tailnet) HTTP
listener and its guard, phone pairing and device credentials, the phone's API allowlist, Web Push
delivery, keep-awake, and the phone web-app bundle. Out of scope: any AgentDeck-operated relay,
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

- **R1 (planned) — One process, one remote subsystem.** `internal/remote` owns the tailnet node,
  pairing, device credentials, push delivery, and keep-awake. The existing server process hosts it;
  no helper process, daemon, or second binary is added. A remote-subsystem failure never stops the
  loopback server, agents, tasks, or pipelines; it surfaces only as FS-20.R2's **Unavailable** state
  with a stable reason code. Rationale: remote control is an optional channel, not a runtime.
- **R2 (planned) — The tailnet node is an embedded `tsnet.Server`.** Hostname is the product name
  (FS-00.R16), state lives in `$AGENTDECK_HOME/remote/tailscale/` (`0700`, TS-05.R5), and the node is
  not ephemeral so pairings and its address survive restarts. It starts when `remote_enabled` is
  true at startup or when the person turns it on, and is closed with `Server.Close` when turned off or
  at shutdown. Enable/disable transitions are generation-scoped so a late start from a superseded
  generation closes itself instead of publishing state (INV §4, INV §5). `tsnet` log output is routed
  through AgentDeck's logger with R12's redaction.
- **R3 (planned) — Connection state is derived, not guessed.** The subsystem maps
  `StatusWithoutPeers().BackendState`/`AuthURL` and `ListenTLS` prerequisite errors to exactly
  `off`, `needs_login` (with the auth URL), `starting`, `on` (with `https://<CertDomains[0]>`), or
  `unavailable` with reason `magicdns_disabled`, `https_disabled`, `node_error`, or
  `listener_error`. It publishes each change as one `remote_update` SSE event on the loopback stream
  only and answers `GET /api/remote` with the same state (TS-03.R46). The auth URL is shown to the
  desktop only; it is never sent to a phone.
- **R4 (planned) — The tailnet listener has its own guard and shares handlers.** `ListenTLS("tcp",
  ":443")` feeds a second `http.Server` whose chain is: remote guard → device authentication →
  allowlist → the same handler functions the loopback mux uses (INV §2). It never passes through
  `localOnly`, and the loopback mux never passes through the remote chain (TS-05.R23). The remote
  guard requires `Host` to equal the node's certificate domain, requires a present `Origin` to equal
  `https://<that domain>`, requires `Origin` on every non-GET request, and requires `WhoIs` to
  resolve the peer; anything else is `403`. `ListenFunnel` is never called.
- **R5 (planned) — The phone API is an explicit allowlist.** One table in `internal/server` names
  every method and path pattern reachable on the tailnet listener; a route not in it answers `404`
  with code `remote_route_not_available`, whether or not the loopback mux serves it. The table
  contains only what FS-20 needs: health, capabilities, and the SSE stream; session list, detail, and
  transcript reads; `prompt` (POST/GET/DELETE), `steer`, `cancel`, `stop`, `resume`, and
  `permission`; `POST /api/sessions` for Ask AgentDecker; task list/detail/create and
  `cancel`/`result`/`retry`/`rearm`; pipeline run list/detail/start and
  `continue`/`retry`/`replace`/`repair-cleanup`/`stop`; read-only project, role, backend-catalog,
  pipeline-template, and notification-preference reads; and the phone routes of R8/R9/R10. It never
  contains `/mcp`, `/api/hook`, the terminal WebSocket, the directory picker, any config or
  federation write, archive, annotations, clone, switch runtime, rename, identity, group release,
  signals, layout, worktrees, or any `/api/remote` management route. A test enumerates the loopback
  route inventory and fails when a route is neither allowlisted nor explicitly denied (INV §10).
- **R6 (planned) — Remote requests cannot widen permission policy.** A launch or task-creation body
  arriving on the tailnet listener may not carry a per-launch permission-bypass override or any
  field the phone's FS-20.R15 forms do not set; such a body is rejected with
  `remote_field_not_allowed` before any process work. Effective permission policy still composes
  global and role policy exactly as TS-05.R9 defines. Every accepted remote mutation is attributed to
  its device id in the server log.
- **R7 (planned) — A device credential is a hashed, sliding, node-bound cookie.** Pairing mints a
  32-byte random token; SQLite stores only its SHA-256 hash with the phone's `Node.StableID` and
  login from `WhoIs` (TS-02.R37). The phone receives it only as the cookie
  `__Host-remote_device` (`Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`, `Max-Age` 400 days).
  Authentication hashes the presented cookie, finds the device, and requires the request's `WhoIs`
  `StableID` to equal the stored one; a mismatch is `401 remote_device_mismatch` and is not treated
  as a revocation. A valid cookie older than one day is re-issued on the response so an active phone
  never reaches the browser's cookie cap (FS-20: pairings do not expire). `last_seen_at` is written
  at most once a minute per device. Tokens never appear in URLs, SSE, API bodies, or logs.
- **R8 (planned) — Pairing is a desktop-issued, single-use code claimed atomically.** A loopback-only
  `POST /api/remote/pairings` creates the only outstanding code (a new one invalidates the previous):
  8 characters from an unambiguous alphabet, valid for five minutes, held in memory only. The QR
  encodes `https://<domain>/pair#<code>` so the code stays in the fragment and never reaches request
  logs. The phone posts `{code, name}` to `POST /api/remote/pair` (the only unauthenticated
  non-static tailnet route); a match atomically claims the code (INV §5) and creates a pending
  request surfaced to the desktop through `remote_update`. Five failed attempts invalidate the
  current code, and failures are rate-limited per peer node. The desktop answers through loopback-only
  `POST /api/remote/pairings/{id}/allow|decline`; the phone waits on `GET
  /api/remote/pair/{pending_id}` with the pending id as its bearer of that wait, and the allow
  response sets R7's cookie. Declined, expired, or reused codes return
  `remote_pairing_invalid` without revealing which.
- **R9 (planned) — Device management is desktop-only; self-service is phone-only.** Loopback-only
  routes list devices (id, name, paired/last-seen times, notification state — never token hashes,
  node ids, or push endpoints), rename, and revoke. Tailnet routes under `/api/remote/self` let a
  paired phone rename itself, unpair itself, and set or clear its push subscription. Revoke and
  unpair delete the device row and cancel that device's open requests and SSE streams through a
  per-device context registry, closing them within one second (INV §4).
- **R10 (planned) — One server-side attention definition feeds Home and push.** A single helper
  classifies FS-20.R11's **Needs you**, **Moving**, and completion items from the agent snapshot,
  pending permissions, tasks, and pipeline runs. `GET /api/remote/home?since=<time>` returns the
  three lists; the phone refetches it on relevant SSE events (debounced) and keeps `since` in its own
  storage. The push sender uses the same helper's transitions, so Home and notifications cannot
  disagree about what needs the person (INV §2).
- **R11 (planned) — Push is standard Web Push sent from the Mac.** Delivery uses RFC 8030 with
  RFC 8291 `aes128gcm` payload encryption and RFC 8292 VAPID. The VAPID key pair is generated on
  first need into `$AGENTDECK_HOME/remote/vapid.json` (`0600`). A subscription's endpoint must be
  `https` on a known browser push-service host (Google FCM, Apple `*.push.apple.com`, Mozilla
  autopush, Microsoft WNS) or it is rejected, so a phone cannot make the Mac post to arbitrary URLs.
  Sends leave over the Mac's ordinary internet connection, not the tailnet. The payload is at most
  4 KiB of `{title, body, tag, url}` built only from FS-20.R19's fields; `tag` is the project or run
  so the service worker replaces rather than stacks (FS-20.R20), and a 10-second per-tag coalescing
  window bounds bursts. Mutes (FS-02.R24) and the per-device switch are read at send time. A bounded
  queue with one worker sends; a `404`/`410` marks the subscription expired (FS-20.R28), other
  failures retry with bounded backoff and are then dropped and logged without payload.
- **R12 (planned) — Remote secrets follow TS-05.R7.** Device tokens and hashes, pairing codes,
  pending ids, push endpoints and keys, the VAPID private key, and `tsnet` node keys never appear in
  logs, error bodies, SSE, or API responses.
- **R13 (planned) — Keep-awake is a supervised `caffeinate` child.** When `keep_awake` is true and
  the work-active predicate holds (any agent `busy`, any pending permission request, or any pipeline
  run `queued` or `running`), one owner goroutine runs `/usr/bin/caffeinate -i -w <server pid>` and
  kills it when the predicate clears, the setting turns off, or the server stops; `-w` ends it if the
  server dies. It never uses `-d`, `-s`, or display assertions. Off macOS the setting reports
  unavailable.
- **R14 (planned) — The phone app is a second embedded UI entry.** `ui/remote.html` and
  `ui/src/remote/` build in the same Vite project and embed with the desktop bundle (TS-08.R73). The
  tailnet listener serves the phone entry, its web-app manifest, and its service worker at `/`; it
  never serves the desktop bundle, and the loopback server never serves the phone entry. The phone
  app reuses `ui/src/api` schemas and client (TS-02.R11 lockstep) and uses a plain `EventSource`
  with its cookie; it does not use the desktop SharedWorker.

## 3. Interfaces & data shapes

Loopback-only: `GET /api/remote`, `PUT /api/remote` `{enabled, keep_awake}`, `POST
/api/remote/pairings`, `POST /api/remote/pairings/{id}/allow|decline`, `GET /api/remote/devices`,
`PATCH|DELETE /api/remote/devices/{id}`. Tailnet-only: `POST /api/remote/pair`, `GET
/api/remote/pair/{pending_id}`, `GET /api/remote/home`, `PATCH|DELETE /api/remote/self`,
`PUT|DELETE /api/remote/self/push`. SSE `remote_update` (loopback) carries the R3 state, the pending
pairing, and the device list summary.

`GET /api/remote` → `{state, reason?, auth_url?, address?, keep_awake, keep_awake_available,
pending_pairing?}`. `POST /api/remote/pairings` → `{id, code, qr_url, expires_at}`.

Persistence is defined in TS-02.R37; route inventory in TS-03.R46; security boundary in TS-05.R23;
build and dependency pinning in TS-06.R27; the phone bundle in TS-08.R73.

## 4. Invariants

INV §2 (one attention helper; shared handlers across listeners), INV §4 (node and device-stream
teardown), INV §5 (pairing claim and enable/disable generations), INV §8 (bounded, in-vocabulary
phone and push data), INV §10 (allowlist wiring test), INV §14 (every route guarded — the tailnet
listener by R4's guard), INV §15 (commit device rows before releasing cookies or push sends),
INV §16 (bounded pairing attempts, push queue, and coalescing).

## 5. Deviations & open decisions

- Tests use a fake listener and fake `WhoIs`; the real tailnet, certificate issuance, and real push
  services are covered only by FS-20's manual gates.
- Face ID or fingerprint app lock and native apps are out of scope (FS-20 §6).

## 6. Traceability

— (planned)
