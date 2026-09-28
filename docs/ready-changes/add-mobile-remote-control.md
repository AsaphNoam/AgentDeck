# Add mobile remote control

**State:** In progress
**Why:** 2026-09-25 `/design-feature` request: keep orchestrating from a phone while the Mac keeps
working — a remote control like Claude Code Remote Control or Codex in the ChatGPT app, using
Nodeterm's mobile companion (https://www.nodeterm.dev/docs/remote/mobile-companion/) as a reference
model rather than a specification.
**Relevant requirements:** FS-20.R1–R29, FS-00.R18, TS-13.R1–R14, TS-02.R37, TS-03.R46, TS-05.R23,
TS-06.R27, TS-08.R73, INV §2, §4, §5, §8, §10, §14, §15, §16

## Outcome

A person pairs an Android phone or iPhone with their Mac, then — anywhere their phone has Tailscale —
sees what needs them, answers permissions and questions with full context, continues and steers
conversations, redirects tasks and pipeline runs, starts new work against configured projects, and
gets attention-only push notifications. The Mac stays the only runtime and authority; AgentDeck adds
no cloud service or account.

## Included work

Settled decisions (user, 2026-09-25): Tailscale instead of a built relay, embedded as a `tsnet` node
(accepting the measured ~15–22 MiB binary growth and a Go ≥ 1.26.6 toolchain bump); AgentDeck QR
pairing on top of tailnet identity; an installable web app served by the Mac rather than an Expo or
native app; an allowlist of existing `/api` routes rather than a dedicated phone API; a hashed,
sliding, node-bound `HttpOnly` cookie credential; approvals only inside the app; optional
keep-awake; pairings that do not expire and no phone limit.

Included: the Remote settings section, embedded node lifecycle, tailnet listener guard and
allowlist, pairing/revocation, the phone web app (Home, decision cards, conversations, task/run
actions, New work), Web Push, and keep-awake. Excluded: any relay or public exposure, native apps,
biometric app lock, terminal on the phone, and every desktop-only surface in FS-20.R16.

Evidence already gathered is recorded in TS-13 §1 (tsnet v1.102.5 source lines, toolchain and size
measurements, Apple's separate Home Screen storage). The Go Web Push library is an implementation
choice; verify its `aes128gcm` and Apple push-service behavior before adopting it.

Sequencing note: if `rename-product-to-deckhand` ships first, the node hostname, cookie-facing copy,
and the **Ask AgentDecker** label follow the renamed product and resident-operator role (FS-00.R16,
FS-18.R14).

## How we will know it works

FS-20.A1–A7 through server tests with a fake listener and fake `WhoIs`, UI tests, and a fakeACP
browser pass at a 390px phone viewport; FS-20.A1, A5, A6, and A8 manual gates on a real tailnet with
a real Android phone and a real iPhone; the TS-13.R5 route-inventory allowlist test; the closure
matrix in AGENT-WORKFLOW §2, including both Go test variants and focused `-race` coverage of pairing
claims, device revocation, and node enable/disable generations.

## Waiting on

Nothing.

## Progress

Slices (each commits verified with its spec and handoff update; tests use a fake node, listener
and `WhoIs`):

1. **Persistence + subsystem skeleton** — `remote_enabled`/`keep_awake` config fields,
   `remote_devices` migration and store, `internal/remote` manager over a `Node` interface with
   generation-scoped enable/disable and TS-13.R3 state mapping, loopback `GET/PUT /api/remote`,
   `remote_update` SSE. Focused `-race`.
2. **Tailnet chain** — one route table for both muxes, TS-13.R5 allowlist plus inventory test,
   remote guard (Host/Origin/`WhoIs`), cookie device auth, R6 field restriction, tailnet SSE
   without `remote_update`, per-device context registry.
3. **Pairing + devices** — code issue/claim/rate limit, allow/decline, wait route issuing the
   cookie, device list/rename/revoke, `/api/remote/self`; revoke closes streams. Focused `-race`.
4. **Real `tsnet` node** — pin `tailscale.com` v1.102.5 and `go 1.26.6` (the toolchain downloads
   through `GOTOOLCHAIN`), CI/release/docs toolchains, log redaction.
5. **Attention helper** and `GET /api/remote/home`.
6. **Desktop Remote settings section** — state, disclosure, QR/code, allow/decline, devices,
   keep-awake.
7. **Phone app shell** — second Vite entry, manifest, pairing flow, Home, connection/stale state.
8. **Phone actions** — decision cards, conversation, task/run actions, New work.
9. **Web Push** — VAPID, subscriptions, bounded sender, service worker.
10. **Keep-awake** — supervised `caffeinate`.

Closure: 390px fakeACP browser pass, full matrix, spec reconciliation; FS-20's manual gates stay
owed. INV §6 checklist for the remote listener: persistence, LaunchSpec, messaging, turn
boundaries, reconcile and hooks lines are N/A (no new agent runtime); fan-out reuses the bus
subscriber that drops slow readers; capabilities honesty means the phone never offers an action
the Mac cannot perform; teardown is the node close plus per-device stream cancellation.

Done:

- **Slice 1** (2026-09-28) — `internal/remote.Manager` (fake-node tests incl. rapid-toggle
  `-race`), `remote_devices` migration 34 and store, config fields, loopback `GET/PUT /api/remote`
  with `remote_update`, shared `configMu` for both config writers. Until slice 4 the node factory
  returns `errRemoteUnsupported`, so turning remote on reports Unavailable/`node_error`. Specs stay
  `(planned)` until their user-visible surface lands. Test: `go test -race ./internal/remote
  ./internal/server -run Remote`.

- **Slice 2** (2026-09-28) — `routeTable()` feeds both muxes; `remote_routes.go` holds
  `remoteAllowed` (with per-route body field lists, TS-13.R6) and `remoteDenied`, guard, cookie
  auth (hash lookup, node binding, daily re-issue, minute-throttled `last_seen_at`), per-device
  request registry (`remoteDevices.end/endAll`), and `serveRemote`. The phone SSE drops
  `remote_update`. Review note (reversible choice): `GET /api/backends` is denied because the
  catalog carries backend/model env that may hold keys; slice 8 must resolve the New-work
  runtime preselection server-side or via a redacted read, and TS-13.R5 must record it.

- **Slice 3** (2026-09-28) — `remote_pairing.go`: one outstanding code, one pending request
  (a newer claim declines the older), constant-time compare, five failures burn a code, ten per
  node per five minutes rate-limit (429 `remote_rate_limited`), desktop allow commits the device
  row before the phone's 20 s long-poll wait collects the cookie once. `remoteView` now also
  carries `pending_pairing` and `devices` (TS-13 §3 to be reconciled). Tailnet request log
  redacts the wait token. Disable resets pairing.

- **Slice 4** (2026-09-28) — `tailscale.com` v1.102.5 pinned, `go 1.26.6` (local 1.25 switches
  through `GOTOOLCHAIN=auto`; CI/release read `go-version-file`), README updated.
  `remote.NewTSNetNode` pre-checks MagicDNS/cert domains so refusals map to stable reasons,
  routes `Logf`→Debug and `UserLogf`→Info through `RedactTailscaleLog`. Review note (reversible
  privacy choice): `envknob.SetNoLogsNoSupport()` stops the node uploading logs to Tailscale.
  `testServer` injects a failing node factory so no suite reaches a real tailnet. Measured
  stripped `sqlite_fts5` binary: 15.9 MB → 39.8 MB (+22.8 MiB, top of the accepted range);
  TS-06.R27 must record it. Observed unrelated flake under full `./...` load:
  `TestContextSharingStartsNoModelTurn` (baseline sampled before the held turn's prompt lands);
  passes alone and in the server package.

Next: slice 5 (attention helper + `GET /api/remote/home`).
